-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS compliance;

-- screening_results: one row per sanctions-check run
CREATE TABLE compliance.screening_results (
    id           UUID        PRIMARY KEY,
    user_id      UUID        NOT NULL,
    order_id     UUID,                        -- NULL for manual / non-order checks
    status       TEXT        NOT NULL CHECK (status IN ('approved', 'rejected', 'pending')),
    match_type   TEXT        NOT NULL CHECK (match_type IN ('none', 'exact', 'fuzzy')),
    matched_name TEXT        NOT NULL DEFAULT '',
    provider     TEXT        NOT NULL DEFAULT 'local',
    screened_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_screening_user_screened ON compliance.screening_results (user_id, screened_at DESC);
CREATE INDEX idx_screening_order        ON compliance.screening_results (order_id) WHERE order_id IS NOT NULL;

-- user_status: aggregate per-user compliance state (derived from screening_results)
CREATE TABLE compliance.user_status (
    user_id    UUID        PRIMARY KEY,
    status     TEXT        NOT NULL CHECK (status IN ('clear', 'flagged', 'blocked')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS compliance.user_status;
DROP TABLE IF EXISTS compliance.screening_results;
DROP SCHEMA IF EXISTS compliance CASCADE;
-- +goose StatementEnd
