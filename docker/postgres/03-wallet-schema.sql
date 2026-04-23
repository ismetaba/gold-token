-- Wallet service schema — applied automatically on first postgres container start.
-- Mirrors backend/migrations/0004_wallet_initial.sql without goose directives.

CREATE SCHEMA IF NOT EXISTS wallet;

CREATE TABLE IF NOT EXISTS wallet.wallets (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL,
    address     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_wallet_user    UNIQUE (user_id),
    CONSTRAINT uq_wallet_address UNIQUE (lower(address))
);

CREATE INDEX IF NOT EXISTS idx_wallet_address ON wallet.wallets (lower(address));

CREATE TABLE IF NOT EXISTS wallet.transaction_log (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL,
    address     TEXT NOT NULL,
    tx_hash     TEXT NOT NULL,
    event_type  TEXT NOT NULL
                    CHECK (event_type IN ('mint','burn','transfer_in','transfer_out')),
    amount_wei  TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_tx_log_hash_type UNIQUE (tx_hash, event_type)
);

CREATE INDEX IF NOT EXISTS idx_tx_log_user_time ON wallet.transaction_log (user_id, occurred_at DESC);
