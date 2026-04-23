-- Order service schema
CREATE SCHEMA IF NOT EXISTS orders;

CREATE TABLE IF NOT EXISTS orders.orders (
    id              UUID PRIMARY KEY,
    user_id         UUID NOT NULL,
    type            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'created',
    amount_grams    NUMERIC NOT NULL,
    amount_wei      TEXT NOT NULL DEFAULT '0',
    user_address    TEXT NOT NULL DEFAULT '',
    arena           TEXT NOT NULL DEFAULT '',
    allocation_id   TEXT DEFAULT '',
    idempotency_key TEXT NOT NULL,
    failure_reason  TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmed_at    TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_idempotency
    ON orders.orders (user_id, idempotency_key);
