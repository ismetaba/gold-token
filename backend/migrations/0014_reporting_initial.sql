-- +goose Up
CREATE SCHEMA IF NOT EXISTS reporting;

CREATE TABLE reporting.report_jobs (
    id            UUID          PRIMARY KEY,
    report_type   TEXT          NOT NULL CHECK (report_type IN ('transactions','reserves','compliance')),
    parameters    JSONB,
    status        TEXT          NOT NULL DEFAULT 'pending'
                                CHECK (status IN ('pending','running','completed','failed')),
    output_path   TEXT          NOT NULL DEFAULT '',
    error         TEXT          NOT NULL DEFAULT '',
    requested_by  TEXT          NOT NULL DEFAULT '',
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_report_jobs_status ON reporting.report_jobs(status);
CREATE INDEX idx_report_jobs_created ON reporting.report_jobs(created_at DESC);

CREATE TABLE reporting.materialized_reports (
    id            UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    report_type   TEXT          NOT NULL CHECK (report_type IN ('transactions','reserves','compliance')),
    period        TEXT          NOT NULL,  -- e.g. "2026-04" or "2026-04-24"
    data          JSONB         NOT NULL DEFAULT '{}',
    generated_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    UNIQUE (report_type, period)
);

CREATE INDEX idx_materialized_reports_type_period ON reporting.materialized_reports(report_type, period DESC);

-- +goose Down
DROP TABLE IF EXISTS reporting.materialized_reports;
DROP TABLE IF EXISTS reporting.report_jobs;
DROP SCHEMA IF EXISTS reporting;
