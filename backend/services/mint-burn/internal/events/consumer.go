// Package events wires the mint-burn service into the NATS event bus.
package events

import (
	"context"
	"encoding/json"
	"math/big"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/saga"
)

// OrderReadyToMintPayload, Order Service'den gelen event şekli.
type OrderReadyToMintPayload struct {
	OrderID      string `json:"order_id"`
	AllocationID string `json:"allocation_id"`
	UserAddress  string `json:"user_address"` // 0x-prefixed hex
	AmountWei    string `json:"amount_wei"`   // decimal string
	Arena        string `json:"arena"`
}

// Consumer, NATS subscription'ları ve saga dispatching'i yönetir.
type Consumer struct {
	bus          *events.Bus
	orch         *saga.Orchestrator
	log          *zap.Logger
	stream       string
	maxAmountWei *big.Int // upper bound sanity check on a single mint amount
}

// defaultMaxAmountWei caps a single mint at 100 tonnes of gold (100_000_000 grams).
// Far above any legitimate single allocation, but blocks absurd/hostile values.
var defaultMaxAmountWei = new(big.Int).Mul(big.NewInt(100_000_000), big.NewInt(1e18))

func NewConsumer(bus *events.Bus, orch *saga.Orchestrator, log *zap.Logger, stream string) *Consumer {
	return &Consumer{bus: bus, orch: orch, log: log, stream: stream, maxAmountWei: defaultMaxAmountWei}
}

// Start, consumer'ları kaydeder. Non-blocking; mesajlar iç goroutine'de işlenir.
func (c *Consumer) Start(ctx context.Context) error {
	return c.bus.Subscribe(
		ctx, c.stream, "mintburn_order_ready_to_mint", events.SubjOrderReadyToMint,
		c.handleOrderReadyToMint,
	)
}

func (c *Consumer) handleOrderReadyToMint(ctx context.Context, data []byte) error {
	// Malformed payloads are POISON: they will never parse, so wrap the error as
	// permanent to terminate redelivery instead of NAK-looping forever. Only
	// transient failures (e.g. CreateMintSaga DB error) should be retried.
	var env events.Envelope[OrderReadyToMintPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		return events.Permanent(err)
	}

	orderID, err := uuid.Parse(env.Data.OrderID)
	if err != nil {
		return events.Permanent(err)
	}
	allocID, err := uuid.Parse(env.Data.AllocationID)
	if err != nil {
		return events.Permanent(err)
	}
	addr, err := parseAddress(env.Data.UserAddress)
	if err != nil {
		return events.Permanent(err)
	}
	amount, ok := new(big.Int).SetString(env.Data.AmountWei, 10)
	if !ok {
		return events.Permanent(ErrInvalidAmount)
	}
	// Bounds: amount must be strictly positive and within the configured ceiling so a
	// malformed/hostile value can never drive a zero or absurd mint proposal on-chain.
	if amount.Sign() <= 0 || (c.maxAmountWei != nil && amount.Cmp(c.maxAmountWei) > 0) {
		return events.Permanent(ErrInvalidAmount)
	}
	arena := domain.Arena(env.Data.Arena)
	if !arena.Valid() {
		return events.Permanent(ErrInvalidArena)
	}

	req := domain.MintRequest{
		AllocationID: allocID,
		OrderID:      orderID,
		To:           addr,
		AmountWei:    amount,
		Arena:        arena,
		RequestedAt:  time.Now().UTC(),
	}
	_, err = c.orch.CreateMintSaga(ctx, req)
	return err
}
