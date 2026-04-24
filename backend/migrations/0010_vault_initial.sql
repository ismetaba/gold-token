-- +goose Up

CREATE SCHEMA IF NOT EXISTS vault;

CREATE TABLE vault.bar_movements (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    bar_serial    TEXT        NOT NULL,
    from_vault_id UUID,
    to_vault_id   UUID,
    movement_type TEXT        NOT NULL,
    initiated_by  TEXT        NOT NULL DEFAULT 'system',
    reason        TEXT        NOT NULL DEFAULT '',
    moved_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_vault_movements_bar    ON vault.bar_movements (bar_serial);
CREATE INDEX idx_vault_movements_time   ON vault.bar_movements (moved_at DESC);

CREATE TABLE vault.audit_records (
    id                     UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    vault_id               UUID        NOT NULL,
    auditor                TEXT        NOT NULL,
    audit_type             TEXT        NOT NULL,
    bar_count              INT         NOT NULL DEFAULT 0,
    total_weight_grams_wei NUMERIC(78,0) NOT NULL DEFAULT 0,
    discrepancies          JSONB,
    status                 TEXT        NOT NULL DEFAULT 'passed',
    audited_at             TIMESTAMPTZ NOT NULL,
    recorded_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_vault_audits_vault ON vault.audit_records (vault_id);
CREATE INDEX idx_vault_audits_time  ON vault.audit_records (audited_at DESC);

-- +goose Down

DROP TABLE IF EXISTS vault.audit_records;
DROP TABLE IF EXISTS vault.bar_movements;
DROP SCHEMA IF EXISTS vault;
