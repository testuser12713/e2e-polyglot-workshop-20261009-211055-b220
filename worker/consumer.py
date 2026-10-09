"""Consume invoice jobs from the Valkey list.

The API pushes ``{"order_id": <int>}`` onto the list when an order becomes
``fertig``. The worker pops from the tail with ``BRPOP`` (the API pushes with
``LPUSH``, so the queue is FIFO), stores the invoice and keeps consuming.

Failures are logged by exception type only: no customer name, e-mail, phone,
plate or password ever reaches a log line.
"""

from __future__ import annotations

import json
import logging
import time
from typing import Any

import redis
from config import Config, load_config
from invoice import StoredInvoice, store_invoice

logger = logging.getLogger(__name__)

BRPOP_TIMEOUT_SECONDS = 5
RETRY_DELAY_SECONDS = 1.0


def parse_order_id(payload: str | bytes) -> int:
    """Extract and validate the integer ``order_id`` from a job payload."""
    data: Any = json.loads(payload)
    if not isinstance(data, dict):
        raise ValueError("job payload is not a JSON object")
    order_id = data.get("order_id")
    if isinstance(order_id, bool) or not isinstance(order_id, int):
        raise ValueError("job payload has no integer order_id")
    return order_id


def handle_message(payload: str | bytes) -> StoredInvoice:
    """Store the invoice for the order named by a raw job payload."""
    return store_invoice(parse_order_id(payload))


def _process_next(
    client: redis.Redis,
    queue: str,
    timeout: int = BRPOP_TIMEOUT_SECONDS,
) -> StoredInvoice | None:
    """Pop and process one job; return ``None`` when the queue stays empty."""
    item = client.brpop(queue, timeout=timeout)
    if item is None:
        return None
    _, payload = item
    result = handle_message(payload)
    logger.info("stored invoice %s for order %s", result.invoice_number, result.order_id)
    return result


def consume_forever(
    client: redis.Redis | None = None,
    config: Config | None = None,
) -> None:
    """Consume invoice jobs until the process is stopped."""
    config = config or load_config()
    if client is None:
        client = redis.from_url(config.valkey_url)
    logger.info("invoice worker listening on queue %s", config.invoice_queue)
    while True:
        try:
            _process_next(client, config.invoice_queue)
        except redis.RedisError as exc:
            logger.error("valkey error (%s); retrying", type(exc).__name__)
            time.sleep(RETRY_DELAY_SECONDS)
        except Exception as exc:
            logger.error("invoice job failed: %s", type(exc).__name__)
