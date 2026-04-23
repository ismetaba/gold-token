-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS orders;

-- orders: buy/sell order flow
CREATE TABLE orders.orders (
    id              UUID PRIMARY KEY,
    user_id         UUID NOT NULL,
    type            TEXT NOT NULL CHECK (type IN ('buy', 'sell')),
    status          TEXT NOT NULL DEFAULT 'created'
                        CHECK (status IN ('created','confirmed','processing','completed','failed')),
    amount_grams    TEXT NOT NULL,   -- human-readable decimal string, e.g. "1.5"
    amount_wei      TEXT NOT NULL,   -- grams * 1e18 as decimal string
    user_address    TEXT NOT NULL,   -- 0x-prefixed Ethereum address (from wallet service)
    arena           TEXT NOT NULL DEFAULT 'TR',  -- ISO-3166 jurisdiction
    allocation_id   UUID,            -- set on confirm, passed to mint saga
    idempotency_key TEXT NOT NULL,
    failure_reason  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmed_at    TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,

    CONSTRAINT uq_order_idempotency UNIQUE (user_id, idempotency_key)
);

CREATE INDEX idx_orders_user_created ON orders.orders (user_id, created_at DESC);
CREATE INDEX idx_orders_status       ON orders.orders (status) WHERE status NOT IN ('completed','failed');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS orders.orders;
DROP SCHEMA IF EXISTS orders CASCADE;
-- +goose StatementEnd
