"""Invoice computation and persistence.

``compute_invoice`` turns labour minutes and parts into net, VAT and gross in
whole cents (19 % VAT, commercial half-up rounding). ``store_invoice`` loads an
order with its items and, in one transaction, writes a consecutively numbered
invoice plus its outbox notification. It is idempotent: the
``invoices.order_id`` unique constraint plus ``ON CONFLICT DO NOTHING`` make a
second delivery of the same job a no-op.
"""

from __future__ import annotations

import logging
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from datetime import UTC, datetime
from decimal import ROUND_HALF_UP, Decimal
from typing import Any

from config import load_config
from db import connection

logger = logging.getLogger(__name__)

VAT_PERCENT = Decimal(19)
MINUTES_PER_HOUR = Decimal(60)
COUNTER_NAME = "re"


class OrderNotFoundError(LookupError):
    """The order referenced by a job does not exist."""

    def __init__(self, order_id: int) -> None:
        super().__init__(f"order {order_id} not found")
        self.order_id = order_id


@dataclass(frozen=True)
class InvoiceTotals:
    """Net, VAT and gross amount of an invoice, each in whole cents."""

    net_cents: int
    vat_cents: int
    gross_cents: int


@dataclass(frozen=True)
class StoredInvoice:
    """Result of storing (or finding) an invoice."""

    invoice_id: int
    invoice_number: str
    order_id: int
    net_cents: int
    vat_cents: int
    gross_cents: int
    created: bool


def _round_half_up(value: Decimal) -> int:
    """Round a decimal amount to whole cents, half away from zero."""
    return int(value.quantize(Decimal(1), rounding=ROUND_HALF_UP))


def compute_invoice(
    labor_minutes: int | float,
    parts: Sequence[Mapping[str, Any]],
    hour_rate_cents: int,
) -> InvoiceTotals:
    """Compute net, VAT and gross for an order in whole cents.

    Labour is charged as ``labor_minutes`` hours at ``hour_rate_cents``, parts
    add ``quantity * unit_price_cents`` each, and 19 % VAT is added on top.
    """
    labor_cents = _round_half_up(
        Decimal(labor_minutes) * Decimal(hour_rate_cents) / MINUTES_PER_HOUR
    )
    parts_cents = sum(int(part["quantity"]) * int(part["unit_price_cents"]) for part in parts)
    net_cents = labor_cents + parts_cents
    vat_cents = _round_half_up(Decimal(net_cents) * VAT_PERCENT / Decimal(100))
    return InvoiceTotals(
        net_cents=net_cents,
        vat_cents=vat_cents,
        gross_cents=net_cents + vat_cents,
    )


def _next_invoice_number(cursor) -> str:
    """Reserve the next invoice number using the shared ``counters`` table."""
    cursor.execute(
        """
        INSERT INTO counters (name, value) VALUES (%s, 1)
        ON CONFLICT (name) DO UPDATE SET value = counters.value + 1
        RETURNING value
        """,
        (COUNTER_NAME,),
    )
    row = cursor.fetchone()
    if row is None:
        raise RuntimeError("could not reserve an invoice number")
    sequence = int(row[0])
    year = datetime.now(UTC).year
    return f"RE-{year:04d}-{sequence:04d}"


def _load_existing(cursor, order_id: int) -> StoredInvoice | None:
    cursor.execute(
        """
        SELECT id, invoice_number, net_cents, vat_cents, gross_cents
        FROM invoices
        WHERE order_id = %s
        """,
        (order_id,),
    )
    row = cursor.fetchone()
    if row is None:
        return None
    return StoredInvoice(
        invoice_id=row[0],
        invoice_number=row[1],
        order_id=order_id,
        net_cents=row[2],
        vat_cents=row[3],
        gross_cents=row[4],
        created=False,
    )


def store_invoice(order_id: int) -> StoredInvoice:
    """Create the invoice and outbox notification for ``order_id``.

    Everything happens in one transaction. A duplicate job returns the existing
    invoice unchanged and writes no second invoice or outbox row.
    """
    config = load_config()
    with connection() as conn, conn.transaction(), conn.cursor() as cursor:
        # Serialise concurrent jobs for the same order: the lock is released
        # when this transaction ends.
        cursor.execute("SELECT pg_advisory_xact_lock(%s)", (order_id,))

        existing = _load_existing(cursor, order_id)
        if existing is not None:
            return existing

        cursor.execute(
            """
                SELECT o.order_number, o.labor_minutes, c.email
                FROM orders o
                LEFT JOIN customers c ON c.id = o.customer_id
                WHERE o.id = %s
                """,
            (order_id,),
        )
        order = cursor.fetchone()
        if order is None:
            raise OrderNotFoundError(order_id)
        order_number, labor_minutes, email = order

        cursor.execute(
            """
                SELECT quantity, unit_price_cents
                FROM order_items
                WHERE order_id = %s
                ORDER BY id
                """,
            (order_id,),
        )
        parts = [
            {"quantity": quantity, "unit_price_cents": unit_price}
            for quantity, unit_price in cursor.fetchall()
        ]

        totals = compute_invoice(labor_minutes or 0, parts, config.hour_rate_cents)
        invoice_number = _next_invoice_number(cursor)

        cursor.execute(
            """
                INSERT INTO invoices
                    (invoice_number, order_id, labor_minutes,
                     net_cents, vat_cents, gross_cents)
                VALUES (%s, %s, %s, %s, %s, %s)
                ON CONFLICT (order_id) DO NOTHING
                RETURNING id
                """,
            (
                invoice_number,
                order_id,
                labor_minutes or 0,
                totals.net_cents,
                totals.vat_cents,
                totals.gross_cents,
            ),
        )
        inserted = cursor.fetchone()
        if inserted is None:
            # Lost a race against a concurrent writer: reuse its invoice.
            existing = _load_existing(cursor, order_id)
            if existing is None:
                raise RuntimeError(f"invoice for order {order_id} could not be stored")
            return existing
        invoice_id = inserted[0]

        recipient = email or ""
        subject = f"Rechnung {invoice_number}"
        body = (
            f"Guten Tag,\n\n"
            f"zu Ihrem Auftrag {order_number} wurde die Rechnung "
            f"{invoice_number} erstellt.\n"
            f"Netto: {totals.net_cents / 100:.2f} EUR\n"
            f"Mehrwertsteuer (19 %): {totals.vat_cents / 100:.2f} EUR\n"
            f"Gesamt: {totals.gross_cents / 100:.2f} EUR\n\n"
            f"Diese Nachricht wurde automatisch erstellt.\n"
        )
        cursor.execute(
            """
                INSERT INTO outbox (invoice_id, recipient, subject, body)
                VALUES (%s, %s, %s, %s)
                """,
            (invoice_id, recipient, subject, body),
        )

    return StoredInvoice(
        invoice_id=invoice_id,
        invoice_number=invoice_number,
        order_id=order_id,
        net_cents=totals.net_cents,
        vat_cents=totals.vat_cents,
        gross_cents=totals.gross_cents,
        created=True,
    )
