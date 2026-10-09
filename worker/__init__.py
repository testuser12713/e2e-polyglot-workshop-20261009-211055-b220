"""Invoice worker for the workshop portal.

It consumes invoice jobs from a Valkey list and writes invoices plus outbox
notifications into PostgreSQL.
"""
