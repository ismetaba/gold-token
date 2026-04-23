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
	bus    *events.Bus
	orch   *saga.Orchestrator
	log    *zap.Logger
	stream string
}

func NewConsumer(bus *events.Bus, orch *saga.Orchestrator, log *zap.Logger, stream string) *Consumer {
	return &Consumer{bus: bus, orch: orch, log: log, stream: stream}
}

// Start, consumer'ları kaydeder. Non-blocking; mesajlar iç goroutine'de işlenir.
func (c *Consumer) Start(ctx context.Context) error {
	return c.bus.Subscribe(
		ctx, c.stream, "mintburn.order_ready_to_mint", events.SubjOrderReadyToMint,
		c.handleOrderReadyToMint,
	)
}

func (c *Consumer) handleOrderReadyToMint(ctx context.Context, data []byte) error {
	var env events.Envelope[OrderReadyToMintPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}

	orderID, err := uuid.Parse(env.Data.OrderID)
	if err != nil {
		return err
	}
	allocID, err := uuid.Parse(env.Data.AllocationID)
	if err != nil {
		return err
	}
	addr, err := parseAddress(env.Data.UserAddress)
	if err != nil {
		return err
	}
	amount, ok := new(big.Int).SetString(env.Data.AmountWei, 10)
	if !ok {
		return ErrInvalidAmount
	}

	req := domain.MintRequest{
		AllocationID: allocID,
		OrderID:      orderID,
		To:           addr,
		AmountWei:    amount,
		Arena:        domain.Arena(env.Data.Arena),
		RequestedAt:  time.Now().UTC(),
	}
	_, err = c.orch.CreateMintSaga(ctx, req)
	return err
}
