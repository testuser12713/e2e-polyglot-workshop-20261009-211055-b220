"""Entry point of the invoice worker: ``python -m worker.main``.

Reads and validates the configuration, proves the database is reachable, then
consumes invoice jobs from Valkey forever.
"""

from __future__ import annotations

import logging
import sys

from config import ConfigError, load_config
from consumer import consume_forever
from db import close_pool, get_pool

logger = logging.getLogger("worker")


def main() -> int:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
    )
    try:
        load_config()
    except ConfigError as exc:
        print(f"configuration error: {exc}", file=sys.stderr)
        return 1
    try:
        get_pool()
    except Exception as exc:
        print(f"cannot reach the database: {type(exc).__name__}", file=sys.stderr)
        return 1
    logger.info("invoice worker started")
    try:
        consume_forever()
    except KeyboardInterrupt:
        logger.info("invoice worker stopped")
    finally:
        close_pool()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
