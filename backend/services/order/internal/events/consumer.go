// Package events wires the order service into the NATS event bus.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/services/order/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/order/internal/repo"
)

// Consumer subscribes to downstream saga outcome events and updates order status.
type Consumer struct {
	bus    *pkgevents.Bus
	orders repo.OrderRepo
	log    *zap.Logger
	stream string
}

func NewConsumer(bus *pkgevents.Bus, orders repo.OrderRepo, log *zap.Logger, stream string) *Consumer {
	return &Consumer{bus: bus, orders: orders, log: log, stream: stream}
}

// Start registers all subscriptions. Non-blocking; messages processed in internal goroutines.
func (c *Consumer) Start(ctx context.Context) error {
	if err := c.bus.Subscribe(
		ctx, c.stream, "order_mint_executed",
		pkgevents.SubjMintExecuted, c.handleMintExecuted,
	); err != nil {
		return fmt.Errorf("subscribe mint_executed: %w", err)
	}
	if err := c.bus.Subscribe(
		ctx, c.stream, "order_mint_failed",
		pkgevents.SubjMintFailed, c.handleMintFailed,
	); err != nil {
		return fmt.Errorf("subscribe mint_failed: %w", err)
	}
	if err := c.bus.Subscribe(
		ctx, c.stream, "order_burn_executed",
		pkgevents.SubjBurnExecuted, c.handleBurnExecuted,
	); err != nil {
		return fmt.Errorf("subscribe burn_executed: %w", err)
	}
	if err := c.bus.Subscribe(
		ctx, c.stream, "order_burn_failed",
		pkgevents.SubjBurnFailed, c.handleBurnFailed,
	); err != nil {
		return fmt.Errorf("subscribe burn_failed: %w", err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers
// ─────────────────────────────────────────────────────────────────────────────

type mintExecutedPayload struct {
	OrderID string `json:"order_id"`
}

type mintFailedPayload struct {
	OrderID   string `json:"order_id"`
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

type burnExecutedPayload struct {
	OrderID string `json:"order_id"`
}

type burnFailedPayload struct {
	OrderID   string `json:"order_id"`
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

func (c *Consumer) handleMintExecuted(ctx context.Context, data []byte) error {
	var env pkgevents.Envelope[mintExecutedPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	return c.completeOrder(ctx, env.Data.OrderID)
}

func (c *Consumer) handleMintFailed(ctx context.Context, data []byte) error {
	var env pkgevents.Envelope[mintFailedPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	reason := fmt.Sprintf("%s: %s", env.Data.ErrorCode, env.Data.Message)
	return c.failOrder(ctx, env.Data.OrderID, reason)
}

func (c *Consumer) handleBurnExecuted(ctx context.Context, data []byte) error {
	var env pkgevents.Envelope[burnExecutedPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	return c.completeOrder(ctx, env.Data.OrderID)
}

func (c *Consumer) handleBurnFailed(ctx context.Context, data []byte) error {
	var env pkgevents.Envelope[burnFailedPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	reason := fmt.Sprintf("%s: %s", env.Data.ErrorCode, env.Data.Message)
	return c.failOrder(ctx, env.Data.OrderID, reason)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func (c *Consumer) completeOrder(ctx context.Context, orderIDStr string) error {
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		return fmt.Errorf("parse order_id: %w", err)
	}
	o, err := c.orders.ByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("fetch order: %w", err)
	}
	if o.Status.IsTerminal() {
		return nil // already settled, idempotent
	}
	now := time.Now().UTC()
	o.Status = domain.OrderCompleted
	o.CompletedAt = &now
	if err := c.orders.Update(ctx, o); err != nil {
		return fmt.Errorf("update order to completed: %w", err)
	}
	c.log.Info("order completed", zap.String("order_id", orderIDStr))
	return nil
}

func (c *Consumer) failOrder(ctx context.Context, orderIDStr, reason string) error {
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		return fmt.Errorf("parse order_id: %w", err)
	}
	o, err := c.orders.ByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("fetch order: %w", err)
	}
	if o.Status.IsTerminal() {
		return nil // already settled, idempotent
	}
	now := time.Now().UTC()
	o.Status = domain.OrderFailed
	o.FailureReason = reason
	o.CompletedAt = &now
	if err := c.orders.Update(ctx, o); err != nil {
		return fmt.Errorf("update order to failed: %w", err)
	}
	c.log.Warn("order failed", zap.String("order_id", orderIDStr), zap.String("reason", reason))
	return nil
}
