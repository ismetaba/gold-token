// Package events wires the treasury service into the NATS event bus.
package events

import (
	"context"
	"encoding/json"
	"math/big"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/services/treasury/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/treasury/internal/repo"
)

// MintExecutedPayload is the data portion of gold.mint.executed.v1.
type MintExecutedPayload struct {
	SagaID    string `json:"saga_id"`
	OrderID   string `json:"order_id"`
	AmountWei string `json:"amount_wei"`
	TxHash    string `json:"tx_hash"`
	Arena     string `json:"arena"`
}

// BurnExecutedPayload is the data portion of gold.burn.executed.v1.
type BurnExecutedPayload struct {
	SagaID    string `json:"saga_id"`
	OrderID   string `json:"order_id"`
	AmountWei string `json:"amount_wei"`
	TxHash    string `json:"tx_hash"`
	Arena     string `json:"arena"`
}

// Consumer manages NATS subscriptions for the treasury service.
type Consumer struct {
	bus      *pkgevents.Bus
	reserves repo.ReserveRepo
	settles  repo.SettlementRepo
	bus2     *pkgevents.Bus // same instance; alias kept for publish clarity
	log      *zap.Logger
	stream   string
}

func NewConsumer(
	bus *pkgevents.Bus,
	reserves repo.ReserveRepo,
	settles repo.SettlementRepo,
	log *zap.Logger,
	stream string,
) *Consumer {
	return &Consumer{
		bus:      bus,
		reserves: reserves,
		settles:  settles,
		bus2:     bus,
		log:      log,
		stream:   stream,
	}
}

// Start registers all subscriptions. Non-blocking.
func (c *Consumer) Start(ctx context.Context) error {
	if err := c.bus.Subscribe(
		ctx, c.stream, "treasury_mint_executed", pkgevents.SubjMintExecuted,
		c.handleMintExecuted,
	); err != nil {
		return err
	}
	return c.bus.Subscribe(
		ctx, c.stream, "treasury_burn_executed", pkgevents.SubjBurnExecuted,
		c.handleBurnExecuted,
	)
}

func (c *Consumer) handleMintExecuted(ctx context.Context, data []byte) error {
	var env pkgevents.Envelope[MintExecutedPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}

	amountWei, ok := new(big.Int).SetString(env.Data.AmountWei, 10)
	if !ok || amountWei.Sign() <= 0 {
		c.log.Warn("mint executed: invalid amount_wei, skipping", zap.String("raw", env.Data.AmountWei))
		return nil
	}

	// Find gold reserve account.
	acc, err := c.reserves.ByTypeAndCurrency(ctx, domain.AccountTypeGold, "XAU", env.Data.Arena)
	if err != nil {
		// If not seeded for this arena, fall back to global.
		acc, err = c.reserves.ByTypeAndCurrency(ctx, domain.AccountTypeGold, "XAU", "global")
		if err != nil {
			return err
		}
	}

	// Credit the reserve.
	if err := c.reserves.Credit(ctx, acc.ID, amountWei); err != nil {
		return err
	}

	// Record settlement.
	refID, _ := uuid.Parse(env.Data.SagaID)
	if refID == uuid.Nil {
		refID = uuid.New()
	}
	now := time.Now().UTC()
	s := domain.Settlement{
		ID:             uuid.New(),
		SettlementType: domain.SettlementCredit,
		AccountID:      acc.ID,
		AmountWei:      amountWei,
		ReferenceID:    refID,
		ReferenceType:  "mint",
		TxHash:         env.Data.TxHash,
		Status:         domain.SettlementSettled,
		SettledAt:      &now,
		CreatedAt:      now,
	}
	if err := c.settles.Create(ctx, s); err != nil {
		c.log.Error("create settlement failed", zap.Error(err))
		// Non-fatal: reserve already credited; settlement will be re-tried on replay.
	}

	// Publish treasury settlement event.
	_ = pkgevents.Publish(ctx, c.bus2, pkgevents.Envelope[map[string]any]{
		EventType:     pkgevents.SubjTreasurySettlement,
		AggregateID:   acc.ID.String(),
		CausationID:   env.EventID.String(),
		CorrelationID: env.CorrelationID,
		Data: map[string]any{
			"settlement_id":   s.ID.String(),
			"account_id":      acc.ID.String(),
			"settlement_type": "credit",
			"amount_wei":      amountWei.String(),
			"reference_id":    refID.String(),
			"reference_type":  "mint",
			"tx_hash":         env.Data.TxHash,
		},
	})

	c.log.Info("gold reserve credited (mint)",
		zap.String("account_id", acc.ID.String()),
		zap.String("amount_wei", amountWei.String()),
		zap.String("tx_hash", env.Data.TxHash),
	)
	return nil
}

func (c *Consumer) handleBurnExecuted(ctx context.Context, data []byte) error {
	var env pkgevents.Envelope[BurnExecutedPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}

	amountWei, ok := new(big.Int).SetString(env.Data.AmountWei, 10)
	if !ok || amountWei.Sign() <= 0 {
		c.log.Warn("burn executed: invalid amount_wei, skipping", zap.String("raw", env.Data.AmountWei))
		return nil
	}

	acc, err := c.reserves.ByTypeAndCurrency(ctx, domain.AccountTypeGold, "XAU", env.Data.Arena)
	if err != nil {
		acc, err = c.reserves.ByTypeAndCurrency(ctx, domain.AccountTypeGold, "XAU", "global")
		if err != nil {
			return err
		}
	}

	if err := c.reserves.Debit(ctx, acc.ID, amountWei); err != nil {
		return err
	}

	refID, _ := uuid.Parse(env.Data.SagaID)
	if refID == uuid.Nil {
		refID = uuid.New()
	}
	now := time.Now().UTC()
	s := domain.Settlement{
		ID:             uuid.New(),
		SettlementType: domain.SettlementDebit,
		AccountID:      acc.ID,
		AmountWei:      amountWei,
		ReferenceID:    refID,
		ReferenceType:  "burn",
		TxHash:         env.Data.TxHash,
		Status:         domain.SettlementSettled,
		SettledAt:      &now,
		CreatedAt:      now,
	}
	if err := c.settles.Create(ctx, s); err != nil {
		c.log.Error("create settlement failed", zap.Error(err))
	}

	_ = pkgevents.Publish(ctx, c.bus2, pkgevents.Envelope[map[string]any]{
		EventType:     pkgevents.SubjTreasurySettlement,
		AggregateID:   acc.ID.String(),
		CausationID:   env.EventID.String(),
		CorrelationID: env.CorrelationID,
		Data: map[string]any{
			"settlement_id":   s.ID.String(),
			"account_id":      acc.ID.String(),
			"settlement_type": "debit",
			"amount_wei":      amountWei.String(),
			"reference_id":    refID.String(),
			"reference_type":  "burn",
			"tx_hash":         env.Data.TxHash,
		},
	})

	c.log.Info("gold reserve debited (burn)",
		zap.String("account_id", acc.ID.String()),
		zap.String("amount_wei", amountWei.String()),
		zap.String("tx_hash", env.Data.TxHash),
	)
	return nil
}
