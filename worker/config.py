"""Environment-driven configuration for the invoice worker.

Every value is read lazily when the worker starts, never at import time. A
missing required variable raises a :class:`ConfigError` whose message names the
variable, instead of a bare traceback before the process can log anything.
"""

from __future__ import annotations

import os
from collections.abc import Mapping
from dataclasses import dataclass

DEFAULT_INVOICE_QUEUE = "invoices"


class ConfigError(RuntimeError):
    """A required configuration value is missing or invalid."""


@dataclass(frozen=True)
class Config:
    """The full worker configuration, read once at startup."""

    database_url: str
    valkey_url: str
    hour_rate_cents: int
    invoice_queue: str


def _required(env: Mapping[str, str], name: str) -> str:
    value = env.get(name)
    if value is None or value == "":
        raise ConfigError(
            f"missing required environment variable {name}; declare it in RUN.json or the environment"
        )
    return value


def load_config(env: Mapping[str, str] | None = None) -> Config:
    """Read and validate the worker configuration.

    ``env`` defaults to :data:`os.environ`; passing a mapping keeps the function
    testable without mutating the real environment.
    """
    source: Mapping[str, str] = os.environ if env is None else env
    database_url = _required(source, "DATABASE_URL")
    valkey_url = _required(source, "VALKEY_URL")
    raw_rate = _required(source, "HOUR_RATE_CENTS")
    try:
        hour_rate_cents = int(raw_rate)
    except ValueError as exc:
        raise ConfigError(f"HOUR_RATE_CENTS must be an integer, got {raw_rate!r}") from exc
    if hour_rate_cents < 0:
        raise ConfigError("HOUR_RATE_CENTS must not be negative")
    invoice_queue = source.get("INVOICE_QUEUE") or DEFAULT_INVOICE_QUEUE
    return Config(
        database_url=database_url,
        valkey_url=valkey_url,
        hour_rate_cents=hour_rate_cents,
        invoice_queue=invoice_queue,
    )


def database_url() -> str:
    """The PostgreSQL connection string (read lazily)."""
    return _required(os.environ, "DATABASE_URL")


def valkey_url() -> str:
    """The Valkey connection string (read lazily)."""
    return _required(os.environ, "VALKEY_URL")


def hour_rate_cents() -> int:
    """The hourly labour rate in whole cents (read lazily)."""
    return load_config().hour_rate_cents


def invoice_queue() -> str:
    """The Valkey list carrying invoice jobs (read lazily)."""
    return os.environ.get("INVOICE_QUEUE") or DEFAULT_INVOICE_QUEUE
