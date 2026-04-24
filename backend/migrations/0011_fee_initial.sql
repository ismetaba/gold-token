-- +goose Up

CREATE SCHEMA IF NOT EXISTS fee;

CREATE TABLE fee.schedules (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT        NOT NULL,
    operation_type    TEXT        NOT NULL,
    arena             TEXT        NOT NULL DEFAULT 'global',
    tier_min_grams_wei NUMERIC(78,0) NOT NULL DEFAULT 0,
    tier_max_grams_wei NUMERIC(78,0),
    fee_bps           INT         NOT NULL DEFAULT 50,
    min_fee_wei       NUMERIC(78,0) NOT NULL DEFAULT 0,
    active            BOOLEAN     NOT NULL DEFAULT true,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_fee_schedules_lookup ON fee.schedules (operation_type, arena, active);

-- Seed default fee schedules: 50bps (0.5%) for all operations globally.
INSERT INTO fee.schedules (name, operation_type, arena, tier_min_grams_wei, fee_bps, min_fee_wei)
VALUES
    ('Global Mint — Standard', 'mint', 'global', 0, 50, 0),
    ('Global Burn — Standard', 'burn', 'global', 0, 50, 0),
    ('Global Transfer — Standard', 'transfer', 'global', 0, 25, 0);

CREATE TABLE fee.ledger_entries (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID,
    operation_type  TEXT        NOT NULL,
    amount_wei      NUMERIC(78,0) NOT NULL,
    fee_wei         NUMERIC(78,0) NOT NULL,
    fee_bps         INT         NOT NULL,
    arena           TEXT        NOT NULL DEFAULT 'global',
    status          TEXT        NOT NULL DEFAULT 'calculated',
    collected_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_fee_ledger_order  ON fee.ledger_entries (order_id);
CREATE INDEX idx_fee_ledger_time   ON fee.ledger_entries (created_at DESC);

-- +goose Down

DROP TABLE IF EXISTS fee.ledger_entries;
DROP TABLE IF EXISTS fee.schedules;
DROP SCHEMA IF EXISTS fee;
