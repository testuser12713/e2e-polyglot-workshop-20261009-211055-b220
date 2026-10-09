"""PostgreSQL access for the invoice worker.

A single lazily created connection pool is shared by the whole process. All
statements are parameterised; no query text is ever assembled from values.
"""

from __future__ import annotations

import logging
from collections.abc import Iterator
from contextlib import contextmanager

from config import load_config
from psycopg import Connection
from psycopg_pool import ConnectionPool

logger = logging.getLogger(__name__)

_pool: ConnectionPool | None = None


def get_pool() -> ConnectionPool:
    """Return the process-wide connection pool, creating it on first use.

    The pool is opened eagerly so a missing or unreachable database fails here,
    at startup, with a message that points at ``DATABASE_URL``.
    """
    global _pool
    if _pool is None:
        config = load_config()
        pool = ConnectionPool(conninfo=config.database_url, min_size=1, max_size=5, open=False)
        pool.open(wait=True, timeout=10.0)
        _pool = pool
        logger.info("database pool ready")
    return _pool


@contextmanager
def connection() -> Iterator[Connection]:
    """Yield a pooled connection; commit on success, roll back on error."""
    pool = get_pool()
    with pool.connection() as conn:
        yield conn


def close_pool() -> None:
    """Close the shared pool, if one was opened."""
    global _pool
    if _pool is not None:
        _pool.close()
        _pool = None
