-- PoR (Proof of Reserve) service schema
-- Mirrors backend/migrations/0006_por_initial.sql

CREATE SCHEMA IF NOT EXISTS por;

CREATE TABLE IF NOT EXISTS por.attestation_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    on_chain_idx    BIGINT,
    timestamp_sec   BIGINT NOT NULL,
    as_of_sec       BIGINT NOT NULL,
    total_grams_wei TEXT NOT NULL,
    merkle_root     TEXT NOT NULL,
    ipfs_cid        TEXT NOT NULL,
    auditor         TEXT NOT NULL,
    tx_hash         TEXT,
    recorded_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_por_timestamp UNIQUE (timestamp_sec)
);

CREATE INDEX IF NOT EXISTS idx_por_timestamp ON por.attestation_log (timestamp_sec DESC);
CREATE INDEX IF NOT EXISTS idx_por_on_chain_idx ON por.attestation_log (on_chain_idx)
    WHERE on_chain_idx IS NOT NULL;
