-- +goose Up
-- +goose StatementBegin

-- auditor_verifications: external auditor verification records linked to an attestation.
CREATE TABLE por.auditor_verifications (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    attestation_id    UUID        NOT NULL REFERENCES por.attestation_log(id) ON DELETE CASCADE,
    auditor_name      TEXT        NOT NULL,
    auditor_id        TEXT        NOT NULL,   -- unique identifier for the auditing firm/individual
    verification_hash TEXT        NOT NULL,   -- hash the auditor computed over the attestation data
    verified_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_auditor_verifications_attestation
    ON por.auditor_verifications (attestation_id, verified_at DESC);

-- auto_attestation_config: singleton row that drives the scheduled auto-attestation worker.
-- The worker polls this table on startup and after each run to pick up config changes.
CREATE TABLE por.auto_attestation_config (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    cron_expression TEXT        NOT NULL DEFAULT '0 0 * * *',  -- daily midnight UTC
    enabled         BOOLEAN     NOT NULL DEFAULT true,
    last_run_at     TIMESTAMPTZ
);

-- Seed with default daily-midnight schedule (disabled until explicitly enabled).
INSERT INTO por.auto_attestation_config (cron_expression, enabled)
VALUES ('0 0 * * *', false);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS por.auto_attestation_config;
DROP INDEX  IF EXISTS por.idx_auditor_verifications_attestation;
DROP TABLE  IF EXISTS por.auditor_verifications;

-- +goose StatementEnd
