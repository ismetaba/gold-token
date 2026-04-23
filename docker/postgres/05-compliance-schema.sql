-- Compliance service schema (docker dev init)
-- Mirrors backend/migrations/0007_compliance_initial.sql

CREATE SCHEMA IF NOT EXISTS compliance;

CREATE TABLE IF NOT EXISTS compliance.screening_results (
    id           UUID        PRIMARY KEY,
    user_id      UUID        NOT NULL,
    order_id     UUID,
    status       TEXT        NOT NULL CHECK (status IN ('approved', 'rejected', 'pending')),
    match_type   TEXT        NOT NULL CHECK (match_type IN ('none', 'exact', 'fuzzy')),
    matched_name TEXT        NOT NULL DEFAULT '',
    provider     TEXT        NOT NULL DEFAULT 'local',
    screened_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_screening_user_screened ON compliance.screening_results (user_id, screened_at DESC);
CREATE INDEX IF NOT EXISTS idx_screening_order         ON compliance.screening_results (order_id) WHERE order_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS compliance.user_status (
    user_id    UUID        PRIMARY KEY,
    status     TEXT        NOT NULL CHECK (status IN ('clear', 'flagged', 'blocked')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
