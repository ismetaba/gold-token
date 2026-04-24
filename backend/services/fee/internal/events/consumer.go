// Package events wires the fee service into the NATS event bus.
package events

import (
	"context"
	"encoding/json"
	"math/big"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/services/fee/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/fee/internal/repo"
)

const (
	SubjFeeCalculated = "gold.fee.calculated.v1"
	SubjFeeCollected  = "gold.fee.collected.v1"
)

type orderCreatedPayload struct {
	OrderID   string `json:"order_id"`
	Type      string `json:"type"`      // "buy" or "sell"
	AmountWei string `json:"amount_wei"`
	Arena     string `json:"arena"`
	UserID    string `json:"user_id"`
}

// Consumer handles fee-related NATS events.
type Consumer struct {
	bus       *pkgevents.Bus
	schedules repo.ScheduleRepo
	ledger    repo.LedgerRepo
	log       *zap.Logger
	stream    string
}

func NewConsumer(bus *pkgevents.Bus, schedules repo.ScheduleRepo, ledger repo.LedgerRepo, log *zap.Logger, stream string) *Consumer {
	return &Consumer{bus: bus, schedules: schedules, ledger: ledger, log: log, stream: stream}
}

func (c *Consumer) Start(ctx context.Context) error {
	return c.bus.Subscribe(ctx, c.stream, "fee_order_created", "gold.order.created.v1", c.handleOrderCreated)
}

func (c *Consumer) handleOrderCreated(ctx context.Context, data []byte) error {
	var env pkgevents.Envelope[orderCreatedPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		return nil // don't retry unparseable
	}

	amountWei, ok := new(big.Int).SetString(env.Data.AmountWei, 10)
	if !ok || amountWei.Sign() <= 0 {
		c.log.Warn("order created: invalid amount_wei", zap.String("raw", env.Data.AmountWei))
		return nil
	}

	opType := "mint"
	if env.Data.Type == "sell" {
		opType = "burn"
	}

	arena := env.Data.Arena
	if arena == "" {
		arena = "global"
	}

	tier, err := c.schedules.FindTier(ctx, opType, arena, amountWei)
	if err != nil {
		c.log.Warn("no fee tier found", zap.String("op", opType), zap.String("arena", arena), zap.Error(err))
		return nil // no fee tier = no fee
	}

	// Calculate fee: (amount * bps) / 10000
	feeWei := new(big.Int).Mul(amountWei, big.NewInt(int64(tier.FeeBPS)))
	feeWei.Div(feeWei, big.NewInt(10000))

	// Enforce minimum fee.
	if tier.MinFeeWei != nil && feeWei.Cmp(tier.MinFeeWei) < 0 {
		feeWei = new(big.Int).Set(tier.MinFeeWei)
	}

	orderID, _ := uuid.Parse(env.Data.OrderID)
	now := time.Now().UTC()
	entry := domain.LedgerEntry{
		ID:            uuid.Must(uuid.NewV7()),
		OrderID:       orderID,
		OperationType: opType,
		AmountWei:     amountWei,
		FeeWei:        feeWei,
		FeeBPS:        tier.FeeBPS,
		Arena:         arena,
		Status:        "calculated",
		CreatedAt:     now,
	}

	if err := c.ledger.Create(ctx, entry); err != nil {
		c.log.Error("create fee ledger entry", zap.Error(err))
		return err
	}

	// Publish fee calculated event.
	_ = pkgevents.Publish(ctx, c.bus, pkgevents.Envelope[map[string]any]{
		EventType:     SubjFeeCalculated,
		AggregateID:   orderID.String(),
		CausationID:   env.EventID.String(),
		CorrelationID: env.CorrelationID,
		Data: map[string]any{
			"order_id":       orderID.String(),
			"fee_wei":        feeWei.String(),
			"fee_bps":        tier.FeeBPS,
			"operation_type": opType,
			"arena":          arena,
		},
	})

	c.log.Info("fee calculated",
		zap.String("order_id", orderID.String()),
		zap.String("fee_wei", feeWei.String()),
		zap.Int("fee_bps", tier.FeeBPS),
	)
	return nil
}
