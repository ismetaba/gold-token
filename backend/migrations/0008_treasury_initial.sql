-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS treasury;

-- reserve_accounts: issuer-controlled reserve balances (fiat + gold).
CREATE TABLE treasury.reserve_accounts (
    id           UUID        PRIMARY KEY,
    account_type TEXT        NOT NULL CHECK (account_type IN ('fiat', 'gold')),
    balance_wei  NUMERIC(78,0) NOT NULL DEFAULT 0,
    currency     TEXT        NOT NULL, -- e.g. 'USD', 'XAU'
    arena        TEXT        NOT NULL DEFAULT 'global',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_reserve_accounts_type_currency_arena
    ON treasury.reserve_accounts (account_type, currency, arena);

-- settlements: records of individual fund movements into/out of reserves.
CREATE TABLE treasury.settlements (
    id             UUID        PRIMARY KEY,
    settlement_type TEXT       NOT NULL CHECK (settlement_type IN ('credit', 'debit')),
    account_id     UUID        NOT NULL REFERENCES treasury.reserve_accounts(id),
    amount_wei     NUMERIC(78,0) NOT NULL,
    reference_id   UUID        NOT NULL, -- e.g. order_id, saga_id
    reference_type TEXT        NOT NULL, -- e.g. 'mint', 'burn', 'manual'
    tx_hash        TEXT        NOT NULL DEFAULT '',
    status         TEXT        NOT NULL CHECK (status IN ('pending', 'settled', 'failed')) DEFAULT 'pending',
    settled_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_settlements_account ON treasury.settlements (account_id, created_at DESC);
CREATE INDEX idx_settlements_reference ON treasury.settlements (reference_id, reference_type);
CREATE INDEX idx_settlements_status ON treasury.settlements (status) WHERE status != 'settled';

-- reconciliation_logs: periodic balance-verification records.
CREATE TABLE treasury.reconciliation_logs (
    id                   UUID        PRIMARY KEY,
    account_id           UUID        NOT NULL REFERENCES treasury.reserve_accounts(id),
    expected_balance_wei NUMERIC(78,0) NOT NULL,
    actual_balance_wei   NUMERIC(78,0) NOT NULL,
    discrepancy_wei      NUMERIC(78,0) NOT NULL GENERATED ALWAYS AS (actual_balance_wei - expected_balance_wei) STORED,
    status               TEXT        NOT NULL CHECK (status IN ('ok', 'discrepancy')) DEFAULT 'ok',
    reconciled_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_reconciliation_account ON treasury.reconciliation_logs (account_id, reconciled_at DESC);

-- Seed: one gold reserve account per major arena.
INSERT INTO treasury.reserve_accounts (id, account_type, balance_wei, currency, arena)
VALUES
    (gen_random_uuid(), 'gold', 0, 'XAU', 'global'),
    (gen_random_uuid(), 'fiat', 0, 'USD', 'global');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS treasury.reconciliation_logs;
DROP TABLE IF EXISTS treasury.settlements;
DROP TABLE IF EXISTS treasury.reserve_accounts;
DROP SCHEMA IF EXISTS treasury;
-- +goose StatementEnd
