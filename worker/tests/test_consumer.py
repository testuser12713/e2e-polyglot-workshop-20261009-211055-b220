"""Tests for the Valkey invoice consumer."""

from __future__ import annotations

import json

import pytest
from consumer import _process_next, handle_message, parse_order_id


def test_parse_order_id_accepts_integer_object():
    assert parse_order_id(json.dumps({"order_id": 7})) == 7
    assert parse_order_id(b'{"order_id": 12}') == 12


@pytest.mark.parametrize(
    "payload",
    [
        "not json",
        json.dumps({"order_id": "7"}),
        json.dumps({"order_id": True}),
        json.dumps({"order_id": 7.5}),
        json.dumps({"nope": 7}),
        json.dumps([7]),
    ],
)
def test_parse_order_id_rejects_bad_payloads(payload):
    with pytest.raises(ValueError):
        parse_order_id(payload)


def test_handle_message_stores_invoice(seed_order, db_conn):
    stored = handle_message(json.dumps({"order_id": seed_order["order_id"]}))

    assert stored.created is True
    assert stored.order_id == seed_order["order_id"]
    with db_conn.cursor() as cur:
        cur.execute("SELECT count(*) FROM invoices WHERE order_id = %s", (seed_order["order_id"],))
        assert cur.fetchone()[0] == 1


def test_message_pushed_to_valkey_is_processed(seed_order, valkey_client, db_conn):
    valkey_client.lpush("invoices", json.dumps({"order_id": seed_order["order_id"]}))

    stored = _process_next(valkey_client, "invoices", timeout=2)

    assert stored is not None
    assert stored.created is True
    with db_conn.cursor() as cur:
        cur.execute("SELECT count(*) FROM invoices WHERE order_id = %s", (seed_order["order_id"],))
        assert cur.fetchone()[0] == 1


def test_duplicate_queued_message_creates_one_invoice(seed_order, valkey_client, db_conn):
    message = json.dumps({"order_id": seed_order["order_id"]})
    valkey_client.lpush("invoices", message, message)

    first = _process_next(valkey_client, "invoices", timeout=2)
    second = _process_next(valkey_client, "invoices", timeout=2)

    assert first is not None
    assert second is not None
    assert second.created is False
    assert second.invoice_id == first.invoice_id
    with db_conn.cursor() as cur:
        cur.execute("SELECT count(*) FROM invoices WHERE order_id = %s", (seed_order["order_id"],))
        assert cur.fetchone()[0] == 1
        cur.execute("SELECT count(*) FROM outbox")
        assert cur.fetchone()[0] == 1
