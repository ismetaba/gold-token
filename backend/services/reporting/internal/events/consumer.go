// Package events wires the reporting service into the NATS event bus.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/services/reporting/internal/repo"
)

// Consumer manages NATS subscriptions for the reporting service.
// It listens to key domain events and incrementally updates materialized_reports.
type Consumer struct {
	bus    *pkgevents.Bus
	mat    repo.MaterializedRepo
	log    *zap.Logger
	stream string
}

func NewConsumer(bus *pkgevents.Bus, mat repo.MaterializedRepo, log *zap.Logger, stream string) *Consumer {
	return &Consumer{bus: bus, mat: mat, log: log, stream: stream}
}

// Start registers all subscriptions. Non-blocking.
func (c *Consumer) Start(ctx context.Context) error {
	subs := []struct {
		durable  string
		subject  string
		handler  func(ctx context.Context, data []byte) error
	}{
		{"reporting_mint_executed", pkgevents.SubjMintExecuted, c.handleMintExecuted},
		{"reporting_burn_executed", pkgevents.SubjBurnExecuted, c.handleBurnExecuted},
		{"reporting_treasury_settlement", pkgevents.SubjTreasurySettlement, c.handleTreasurySettlement},
		{"reporting_reserve_attestation", pkgevents.SubjReserveAttestation, c.handleReserveAttestation},
	}
	for _, s := range subs {
		if err := c.bus.Subscribe(ctx, c.stream, s.durable, s.subject, s.handler); err != nil {
			return fmt.Errorf("subscribe %s: %w", s.subject, err)
		}
	}
	return nil
}

// ── handlers ──────────────────────────────────────────────────────────────────

type mintBurnPayload struct {
	SagaID    string `json:"saga_id"`
	OrderID   string `json:"order_id"`
	AmountWei string `json:"amount_wei"`
	TxHash    string `json:"tx_hash"`
	Arena     string `json:"arena"`
}

type settlementPayload struct {
	SettlementID   string `json:"settlement_id"`
	AccountID      string `json:"account_id"`
	SettlementType string `json:"settlement_type"`
	AmountWei      string `json:"amount_wei"`
	ReferenceType  string `json:"reference_type"`
}

type attestationPayload struct {
	AttestationID string `json:"attestation_id"`
	ReserveWei    string `json:"reserve_wei"`
	TokenSupplyWei string `json:"token_supply_wei"`
	TxHash        string `json:"tx_hash"`
}

func (c *Consumer) handleMintExecuted(ctx context.Context, data []byte) error {
	var env pkgevents.Envelope[mintBurnPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	period := env.OccurredAt.UTC().Format("2006-01-02")
	return c.mat.IncrementTransactionCounter(ctx, period, "mint", env.Data.AmountWei)
}

func (c *Consumer) handleBurnExecuted(ctx context.Context, data []byte) error {
	var env pkgevents.Envelope[mintBurnPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	period := env.OccurredAt.UTC().Format("2006-01-02")
	return c.mat.IncrementTransactionCounter(ctx, period, "burn", env.Data.AmountWei)
}

func (c *Consumer) handleTreasurySettlement(ctx context.Context, data []byte) error {
	var env pkgevents.Envelope[settlementPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	period := env.OccurredAt.UTC().Format("2006-01-02")
	c.log.Debug("reserve snapshot event", zap.String("period", period), zap.String("settlement_type", env.Data.SettlementType))
	return nil
}

func (c *Consumer) handleReserveAttestation(ctx context.Context, data []byte) error {
	var env pkgevents.Envelope[attestationPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	period := env.OccurredAt.UTC().Format("2006-01-02")
	now := time.Now().UTC()
	return c.mat.UpsertReserveSnapshot(ctx, period, env.Data.ReserveWei, env.Data.TokenSupplyWei, now)
}
