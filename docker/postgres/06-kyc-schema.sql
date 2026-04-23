-- KYC service schema
CREATE SCHEMA IF NOT EXISTS kyc;

CREATE TABLE IF NOT EXISTS kyc.applications (
    id              UUID PRIMARY KEY,
    user_id         UUID NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    document_path   TEXT NOT NULL,
    first_name      TEXT NOT NULL,
    last_name       TEXT NOT NULL,
    date_of_birth   DATE NOT NULL,
    nationality     TEXT NOT NULL,
    reviewer_note   TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at     TIMESTAMPTZ
);

-- Only one active (non-rejected) application per user
CREATE UNIQUE INDEX IF NOT EXISTS idx_kyc_applications_user_active
    ON kyc.applications (user_id) WHERE status != 'rejected';
