-- 0001_init.sql — initial schema of the workshop portal.
-- Applied by the API at startup (see cmd/api/main.go) and idempotently re-runnable.

CREATE TABLE IF NOT EXISTS customers (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT        NOT NULL,
    email      TEXT        NOT NULL,
    phone      TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS vehicles (
    id          BIGSERIAL PRIMARY KEY,
    plate       TEXT        NOT NULL UNIQUE,
    make        TEXT        NOT NULL DEFAULT '',
    model       TEXT        NOT NULL DEFAULT '',
    mileage     INTEGER     NOT NULL DEFAULT 0,
    customer_id BIGINT      REFERENCES customers (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS orders (
    id           BIGSERIAL PRIMARY KEY,
    order_number TEXT        NOT NULL UNIQUE,
    customer_id  BIGINT      REFERENCES customers (id) ON DELETE SET NULL,
    vehicle_id   BIGINT      REFERENCES vehicles (id) ON DELETE SET NULL,
    status       TEXT        NOT NULL DEFAULT 'angefragt',
    desired_date DATE,
    problem      TEXT        NOT NULL DEFAULT '',
    labor_minutes INTEGER    NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT orders_status_valid CHECK (
        status IN ('angefragt', 'bestätigt', 'in Arbeit', 'fertig', 'abgeholt')
    )
);

CREATE TABLE IF NOT EXISTS order_items (
    id               BIGSERIAL PRIMARY KEY,
    order_id         BIGINT      NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    description      TEXT        NOT NULL,
    quantity         INTEGER     NOT NULL DEFAULT 1,
    unit_price_cents BIGINT      NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS order_status_history (
    id         BIGSERIAL PRIMARY KEY,
    order_id   BIGINT      NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    status     TEXT        NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS invoices (
    id             BIGSERIAL PRIMARY KEY,
    invoice_number TEXT        NOT NULL UNIQUE,
    order_id       BIGINT      NOT NULL UNIQUE REFERENCES orders (id) ON DELETE CASCADE,
    issued_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    labor_minutes  INTEGER     NOT NULL DEFAULT 0,
    net_cents      BIGINT      NOT NULL DEFAULT 0,
    vat_cents      BIGINT      NOT NULL DEFAULT 0,
    gross_cents    BIGINT      NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS outbox (
    id         BIGSERIAL PRIMARY KEY,
    invoice_id BIGINT      REFERENCES invoices (id) ON DELETE SET NULL,
    recipient  TEXT        NOT NULL,
    subject    TEXT        NOT NULL,
    body       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS employees (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT        NOT NULL UNIQUE,
    name          TEXT        NOT NULL,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Sequence counters for the AW- (order) and RE- (invoice) numbers.
CREATE TABLE IF NOT EXISTS counters (
    name  TEXT    PRIMARY KEY,
    value BIGINT  NOT NULL DEFAULT 0
);

INSERT INTO counters (name, value) VALUES ('aw', 0), ('re', 0)
ON CONFLICT (name) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_orders_status ON orders (status);
CREATE INDEX IF NOT EXISTS idx_orders_vehicle ON orders (vehicle_id);
CREATE INDEX IF NOT EXISTS idx_vehicles_plate ON vehicles (plate);
CREATE INDEX IF NOT EXISTS idx_order_status_history_order ON order_status_history (order_id, changed_at);
