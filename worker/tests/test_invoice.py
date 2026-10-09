"""Tests for invoice computation and persistence."""

from __future__ import annotations

import re

import pytest
from invoice import InvoiceTotals, OrderNotFoundError, compute_invoice, store_invoice

INVOICE_NUMBER = re.compile(r"^RE-\d{4}-\d{4,}$")


def test_compute_invoice_applies_rate_parts_and_vat():
    parts = [
        {"quantity": 2, "unit_price_cents": 4500},
        {"quantity": 1, "unit_price_cents": 1000},
    ]
    totals = compute_invoice(90, parts, hour_rate_cents=6500)
    # 90 min = 1.5 h * 6500 = 9750 cents labour; parts = 2*4500 + 1*1000 = 10000.
    assert totals == InvoiceTotals(net_cents=19750, vat_cents=3753, gross_cents=23503)
    assert totals.gross_cents == totals.net_cents + totals.vat_cents


def test_compute_invoice_rounds_half_up():
    # 30 min at 100 cents = 50 cents net; 19 % of 50 = 9.5 -> 10 (half up).
    totals = compute_invoice(30, [], hour_rate_cents=100)
    assert totals.net_cents == 50
    assert totals.vat_cents == 10
    assert totals.gross_cents == 60


def test_compute_invoice_rounds_labour_half_up():
    # 30 min at 5 cents = 2.5 cents -> 3 (half up).
    totals = compute_invoice(30, [], hour_rate_cents=5)
    assert totals.net_cents == 3


def test_store_invoice_writes_invoice_and_outbox(seed_order, db_conn):
    order_id = seed_order["order_id"]
    stored = store_invoice(order_id)

    assert stored.created is True
    assert INVOICE_NUMBER.match(stored.invoice_number)
    assert stored.net_cents == 19750
    assert stored.vat_cents == 3753
    assert stored.gross_cents == 23503

    with db_conn.cursor() as cur:
        cur.execute(
            """
            SELECT invoice_number, order_id, labor_minutes, net_cents, vat_cents, gross_cents
            FROM invoices WHERE order_id = %s
            """,
            (order_id,),
        )
        row = cur.fetchone()
        assert row == (
            stored.invoice_number,
            order_id,
            90,
            19750,
            3753,
            23503,
        )

        cur.execute(
            "SELECT invoice_id, recipient, subject, body, created_at "
            "FROM outbox WHERE invoice_id = %s",
            (stored.invoice_id,),
        )
        outbox = cur.fetchone()
        assert outbox is not None
        outbox_invoice_id, recipient, subject, body, created_at = outbox
        assert outbox_invoice_id == stored.invoice_id
        assert recipient == seed_order["email"]
        assert stored.invoice_number in subject
        assert body.strip() != ""
        assert created_at is not None


def test_store_invoice_numbers_are_consecutive(seed_order, db_conn):
    first = store_invoice(seed_order["order_id"])

    with db_conn.cursor() as cur:
        cur.execute("SELECT customer_id FROM orders WHERE id = %s", (seed_order["order_id"],))
        customer_id = cur.fetchone()[0]
        cur.execute(
            "INSERT INTO orders (order_number, customer_id, status, labor_minutes) "
            "VALUES (%s, %s, %s, %s) RETURNING id",
            ("AW-2026-0002", customer_id, "fertig", 0),
        )
        second_order_id = cur.fetchone()[0]

    second = store_invoice(second_order_id)

    first_seq = int(first.invoice_number.rsplit("-", 1)[1])
    second_seq = int(second.invoice_number.rsplit("-", 1)[1])
    assert second_seq == first_seq + 1


def test_duplicate_message_creates_no_second_invoice(seed_order, db_conn):
    first = store_invoice(seed_order["order_id"])
    second = store_invoice(seed_order["order_id"])

    assert second.created is False
    assert second.invoice_id == first.invoice_id
    assert second.invoice_number == first.invoice_number

    with db_conn.cursor() as cur:
        cur.execute("SELECT count(*) FROM invoices WHERE order_id = %s", (seed_order["order_id"],))
        assert cur.fetchone()[0] == 1
        cur.execute("SELECT count(*) FROM outbox WHERE invoice_id = %s", (first.invoice_id,))
        assert cur.fetchone()[0] == 1


def test_store_invoice_unknown_order_raises(db_conn):
    with pytest.raises(OrderNotFoundError):
        store_invoice(424242)
