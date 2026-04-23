-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS por;

-- attestation_log: mirrors on-chain ReserveOracle attestation history
-- Populated by the PoR service after each successful on-chain publish.
CREATE TABLE por.attestation_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    on_chain_idx    BIGINT,                      -- ReserveOracle array index (NULL if unknown)
    timestamp_sec   BIGINT NOT NULL,             -- Attestation.timestamp (block.timestamp)
    as_of_sec       BIGINT NOT NULL,             -- Attestation.asOf (audit reference date)
    total_grams_wei TEXT NOT NULL,               -- Attestation.totalGrams, decimal string
    merkle_root     TEXT NOT NULL,               -- bytes32, 0x-prefixed hex
    ipfs_cid        TEXT NOT NULL,               -- bytes32, 0x-prefixed hex
    auditor         TEXT NOT NULL,               -- 0x-prefixed Ethereum address
    tx_hash         TEXT,                        -- on-chain tx hash (NULL for chain-synced entries)
    recorded_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_por_timestamp UNIQUE (timestamp_sec)
);

CREATE INDEX idx_por_timestamp ON por.attestation_log (timestamp_sec DESC);
CREATE INDEX idx_por_on_chain_idx ON por.attestation_log (on_chain_idx) WHERE on_chain_idx IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS por.attestation_log;
DROP SCHEMA IF EXISTS por CASCADE;
-- +goose StatementEnd
