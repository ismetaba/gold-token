-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS mint;

-- saga_instances: her mint/burn akışı için bir satır
CREATE TABLE mint.saga_instances (
    id              UUID PRIMARY KEY,
    saga_type       TEXT NOT NULL CHECK (saga_type IN ('mint','burn')),
    state           TEXT NOT NULL,
    order_id        UUID NOT NULL,
    arena           CHAR(2) NOT NULL,
    context         JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_step_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    attempts        INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_saga_state ON mint.saga_instances(state) WHERE completed_at IS NULL;
CREATE INDEX idx_saga_order ON mint.saga_instances(order_id);
CREATE INDEX idx_saga_last_step ON mint.saga_instances(last_step_at) WHERE completed_at IS NULL;

-- vaults: fiziksel kasa konumları
CREATE TABLE mint.vaults (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code            CHAR(4) NOT NULL UNIQUE,        -- "TRCH", "TRIS", "CHZH", "AEDU"
    arena           CHAR(2) NOT NULL,
    operator        TEXT NOT NULL,                  -- "the Refinery","Brinks","BIST KMP"
    address         TEXT NOT NULL,
    country_code    CHAR(2) NOT NULL,
    lbma_approved   BOOLEAN NOT NULL DEFAULT FALSE,
    insured_by      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- gold_bars: ana envanter
CREATE TABLE mint.gold_bars (
    serial_no           TEXT PRIMARY KEY,
    vault_id            UUID NOT NULL REFERENCES mint.vaults(id),
    weight_grams_wei    NUMERIC(78,0) NOT NULL,      -- wei = gram * 1e18
    allocated_sum_wei   NUMERIC(78,0) NOT NULL DEFAULT 0,
    purity_9999         INTEGER NOT NULL,            -- 9999 = %99.99
    refiner_lbma_id     TEXT NOT NULL,
    cast_date           DATE,
    status              TEXT NOT NULL DEFAULT 'in_vault'
                            CHECK (status IN ('in_vault','allocated','in_transit','redeemed','burned')),
    ingested_at         TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- invariant: allocated_sum_wei ≤ weight_grams_wei
    CHECK (allocated_sum_wei <= weight_grams_wei)
);

CREATE INDEX idx_bars_vault_status ON mint.gold_bars(vault_id, status);
CREATE INDEX idx_bars_available ON mint.gold_bars(vault_id)
    WHERE status = 'in_vault' AND allocated_sum_wei < weight_grams_wei;

-- bar_allocations: hangi çubuktan ne kadar alındı
CREATE TABLE mint.bar_allocations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    allocation_id       UUID NOT NULL,                  -- saga allocation_id (== proposalId)
    saga_id             UUID NOT NULL REFERENCES mint.saga_instances(id),
    bar_serial          TEXT NOT NULL REFERENCES mint.gold_bars(serial_no),
    allocated_wei       NUMERIC(78,0) NOT NULL,
    allocated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    released_at         TIMESTAMPTZ
);

CREATE INDEX idx_alloc_allocation_id ON mint.bar_allocations(allocation_id);
CREATE INDEX idx_alloc_saga ON mint.bar_allocations(saga_id);
CREATE INDEX idx_alloc_bar ON mint.bar_allocations(bar_serial);

-- Idempotency: aynı allocation_id iki saga oluşturulamasın
CREATE UNIQUE INDEX uq_saga_allocation ON mint.saga_instances(
    ((context->>'allocation_id'))
) WHERE (context->>'allocation_id') IS NOT NULL;

-- Outbox (event publish tutarlılığı)
CREATE TABLE mint.outbox (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_id    TEXT NOT NULL,
    subject         TEXT NOT NULL,
    payload         JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at    TIMESTAMPTZ
);

CREATE INDEX idx_outbox_unpublished ON mint.outbox(created_at) WHERE published_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS mint.outbox;
DROP TABLE IF EXISTS mint.bar_allocations;
DROP TABLE IF EXISTS mint.gold_bars;
DROP TABLE IF EXISTS mint.vaults;
DROP TABLE IF EXISTS mint.saga_instances;
DROP SCHEMA IF EXISTS mint CASCADE;
-- +goose StatementEnd
