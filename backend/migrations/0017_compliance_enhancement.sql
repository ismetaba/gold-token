-- +goose Up
-- +goose StatementBegin

-- pep_checks: one row per Politically Exposed Persons screening run
CREATE TABLE compliance.pep_checks (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID        NOT NULL,
    matched        BOOLEAN     NOT NULL DEFAULT FALSE,
    match_details  JSONB,
    checked_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_pep_checks_user_checked ON compliance.pep_checks (user_id, checked_at DESC);

-- monitoring_schedule: per-user re-screening schedule
CREATE TABLE compliance.monitoring_schedule (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL UNIQUE,
    last_checked_at TIMESTAMPTZ,
    next_check_at   TIMESTAMPTZ NOT NULL,
    frequency_days  INT         NOT NULL DEFAULT 30
);

CREATE INDEX idx_monitoring_next_check ON compliance.monitoring_schedule (next_check_at)
    WHERE next_check_at IS NOT NULL;

-- jurisdiction_rules: configurable per-arena compliance rules
CREATE TABLE compliance.jurisdiction_rules (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    arena                 CHAR(2)     NOT NULL,
    rule_type             TEXT        NOT NULL,
    threshold_grams_wei   NUMERIC(78,0),
    action                TEXT        NOT NULL,
    active                BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_jurisdiction_rules_arena ON compliance.jurisdiction_rules (arena) WHERE active = TRUE;

-- seed a few starter jurisdiction rules
INSERT INTO compliance.jurisdiction_rules (arena, rule_type, threshold_grams_wei, action, active) VALUES
    ('TR', 'enhanced_due_diligence', 1000000000000000000, 'require_edd',      TRUE),
    ('CH', 'source_of_funds',        NULL,                 'require_sof_decl', TRUE);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS compliance.jurisdiction_rules;
DROP TABLE IF EXISTS compliance.monitoring_schedule;
DROP TABLE IF EXISTS compliance.pep_checks;
-- +goose StatementEnd
