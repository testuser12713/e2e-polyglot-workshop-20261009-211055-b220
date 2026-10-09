"""Shared fixtures for the invoice worker tests.

The tests run against a real PostgreSQL and a real Valkey, addressed by
``DATABASE_URL`` and ``VALKEY_URL``. The schema is created from the canonical
migration so the tests exercise the same tables the product uses.
"""

from __future__ import annotations

import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import psycopg
import pytest
from psycopg import ClientCursor

WORKER_DIR = Path(__file__).resolve().parents[1]
REPO_ROOT = WORKER_DIR.parent
MIGRATION = REPO_ROOT / "backend" / "migrations" / "0001_init.sql"

os.environ.setdefault("HOUR_RATE_CENTS", "6500")

DATABASE_URL = os.environ.get("DATABASE_URL")
VALKEY_URL = os.environ.get("VALKEY_URL")
INVOICE_QUEUE = os.environ.get("INVOICE_QUEUE", "invoices")


@pytest.fixture(scope="session")
def db_conn():
    if not DATABASE_URL:
        pytest.skip("DATABASE_URL is not set; the database is not available")
    conn = psycopg.connect(DATABASE_URL, cursor_factory=ClientCursor, autocommit=True)
    with conn.cursor() as cur:
        cur.execute(MIGRATION.read_text(encoding="utf-8"))
    yield conn
    conn.close()


@pytest.fixture(scope="session", autouse=True)
def _close_pool():
    yield
    from db import close_pool

    close_pool()


@pytest.fixture(autouse=True)
def clean_db(db_conn):
    """Reset only the rows these tests create, before each test."""
    with db_conn.cursor() as cur:
        cur.execute(
            "TRUNCATE outbox, invoices, order_items, orders, customers, counters "
            "RESTART IDENTITY CASCADE"
        )
        cur.execute(
            "INSERT INTO counters (name, value) VALUES ('re', 0) "
            "ON CONFLICT (name) DO UPDATE SET value = 0"
        )
    yield


@pytest.fixture
def seed_order(db_conn, clean_db):
    """Insert one finished order with a customer and two parts."""
    with db_conn.cursor() as cur:
        cur.execute(
            "INSERT INTO customers (name, email, phone) VALUES (%s, %s, %s) RETURNING id",
            ("Anna Beispiel", "anna@example.test", "+49 30 123456"),
        )
        customer_id = cur.fetchone()[0]
        cur.execute(
            """
            INSERT INTO orders (order_number, customer_id, status, problem, labor_minutes)
            VALUES (%s, %s, %s, %s, %s)
            RETURNING id
            """,
            ("AW-2026-0001", customer_id, "fertig", "Bremsen quietschen", 90),
        )
        order_id = cur.fetchone()[0]
        cur.execute(
            """
            INSERT INTO order_items (order_id, description, quantity, unit_price_cents)
            VALUES (%s, %s, %s, %s), (%s, %s, %s, %s)
            """,
            (order_id, "Bremsbelag", 2, 4500, order_id, "Arbeit", 1, 1000),
        )
    return {
        "order_id": order_id,
        "email": "anna@example.test",
        "labor_minutes": 90,
        "parts": [
            {"quantity": 2, "unit_price_cents": 4500},
            {"quantity": 1, "unit_price_cents": 1000},
        ],
    }


@pytest.fixture
def valkey_client():
    import redis

    if not VALKEY_URL:
        pytest.skip("VALKEY_URL is not set; Valkey is not available")
    client = redis.from_url(VALKEY_URL)
    client.delete(INVOICE_QUEUE)
    yield client
    client.delete(INVOICE_QUEUE)
    client.close()
