-- +goose Up
-- +goose StatementBegin

CREATE SCHEMA IF NOT EXISTS audit;

-- Partitioned parent table (by month on occurred_at).
-- Primary key must include the partition key.
CREATE TABLE audit.entries (
    id          UUID        NOT NULL DEFAULT gen_random_uuid(),
    event_id    UUID        NOT NULL,
    event_type  TEXT        NOT NULL,
    actor_id    TEXT        NOT NULL DEFAULT 'system',
    actor_type  TEXT        NOT NULL DEFAULT 'system',
    entity_id   TEXT        NOT NULL DEFAULT '',
    entity_type TEXT        NOT NULL DEFAULT 'unknown',
    action      TEXT        NOT NULL DEFAULT '',
    metadata    JSONB,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ingested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, occurred_at)
) PARTITION BY RANGE (occurred_at);

-- Unique dedup index across the whole partitioned table.
CREATE UNIQUE INDEX idx_audit_entries_event_id ON audit.entries (event_id, occurred_at);

CREATE INDEX idx_audit_entries_entity ON audit.entries (entity_type, entity_id);
CREATE INDEX idx_audit_entries_actor  ON audit.entries (actor_id);
CREATE INDEX idx_audit_entries_time   ON audit.entries (occurred_at DESC);
CREATE INDEX idx_audit_entries_type   ON audit.entries (event_type);

COMMENT ON TABLE audit.entries IS 'Immutable, append-only audit trail. No UPDATE or DELETE.';

-- Seed default monthly partitions for the first year of operation.
-- New partitions should be created by a scheduled maintenance job.
CREATE TABLE audit.entries_2026_04 PARTITION OF audit.entries
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');

CREATE TABLE audit.entries_2026_05 PARTITION OF audit.entries
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

CREATE TABLE audit.entries_2026_06 PARTITION OF audit.entries
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

CREATE TABLE audit.entries_2026_07 PARTITION OF audit.entries
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

CREATE TABLE audit.entries_2026_08 PARTITION OF audit.entries
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');

CREATE TABLE audit.entries_2026_09 PARTITION OF audit.entries
    FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');

CREATE TABLE audit.entries_2026_10 PARTITION OF audit.entries
    FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');

CREATE TABLE audit.entries_2026_11 PARTITION OF audit.entries
    FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');

CREATE TABLE audit.entries_2026_12 PARTITION OF audit.entries
    FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');

CREATE TABLE audit.entries_2027_01 PARTITION OF audit.entries
    FOR VALUES FROM ('2027-01-01') TO ('2027-02-01');

CREATE TABLE audit.entries_2027_02 PARTITION OF audit.entries
    FOR VALUES FROM ('2027-02-01') TO ('2027-03-01');

CREATE TABLE audit.entries_2027_03 PARTITION OF audit.entries
    FOR VALUES FROM ('2027-03-01') TO ('2027-04-01');

-- Catch-all for events outside the pre-partitioned range.
CREATE TABLE audit.entries_default PARTITION OF audit.entries DEFAULT;

-- Revoke mutation privileges — enforce append-only at the DB level.
-- The application role (assumed to be the DATABASE_URL user) loses UPDATE/DELETE.
REVOKE UPDATE, DELETE ON audit.entries FROM PUBLIC;

-- Defense-in-depth: a REVOKE does NOT bind the table owner, and services frequently
-- connect as the owner of their schema. Enforce append-only with a trigger that raises
-- on any UPDATE/DELETE for EVERY role, owner included. Row-level triggers on a
-- partitioned table cascade to all current and future partitions (PostgreSQL 13+).
-- (Operators should additionally run services under a dedicated non-owner role.)
CREATE OR REPLACE FUNCTION audit.deny_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit.entries is append-only: % is not permitted', TG_OP;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_entries_no_update
    BEFORE UPDATE ON audit.entries
    FOR EACH ROW EXECUTE FUNCTION audit.deny_mutation();

CREATE TRIGGER audit_entries_no_delete
    BEFORE DELETE ON audit.entries
    FOR EACH ROW EXECUTE FUNCTION audit.deny_mutation();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS audit.entries CASCADE;
DROP FUNCTION IF EXISTS audit.deny_mutation() CASCADE;
DROP SCHEMA IF EXISTS audit CASCADE;

-- +goose StatementEnd
