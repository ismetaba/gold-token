// Package events wires the compliance service into the NATS event bus.
package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	comphttp "github.com/ismetaba/gold-token/backend/services/compliance/internal/http"
)

// Consumer subscribes to gold.order.created.v1 and triggers sanctions screening.
type Consumer struct {
	bus      *pkgevents.Bus
	handlers *comphttp.Handlers
	stream   string
	log      *zap.Logger
}

func NewConsumer(bus *pkgevents.Bus, handlers *comphttp.Handlers, log *zap.Logger, stream string) *Consumer {
	return &Consumer{bus: bus, handlers: handlers, stream: stream, log: log}
}

// Start registers all subscriptions. Non-blocking.
func (c *Consumer) Start(ctx context.Context) error {
	if err := c.bus.Subscribe(
		ctx, c.stream, "compliance.order_created",
		pkgevents.SubjOrderCreated, c.handleOrderCreated,
	); err != nil {
		return fmt.Errorf("subscribe order.created: %w", err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// order.created handler
// ─────────────────────────────────────────────────────────────────────────────

type orderCreatedPayload struct {
	OrderID     string `json:"order_id"`
	UserID      string `json:"user_id"`
	Type        string `json:"type"`
	AmountWei   string `json:"amount_wei"`
	UserAddress string `json:"user_address"`
	Arena       string `json:"arena"`
}

func (c *Consumer) handleOrderCreated(ctx context.Context, data []byte) error {
	var env pkgevents.Envelope[orderCreatedPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("unmarshal order.created: %w", err)
	}

	userID, err := uuid.Parse(env.Data.UserID)
	if err != nil {
		return fmt.Errorf("parse user_id: %w", err)
	}
	orderID, err := uuid.Parse(env.Data.OrderID)
	if err != nil {
		return fmt.Errorf("parse order_id: %w", err)
	}

	// POC: no name/country in the order event — use arena as country and an
	// empty name so the screener falls through to "approved" unless a specific
	// user_id-based screening was already submitted via POST /compliance/screen.
	result, err := c.handlers.RunScreen(ctx, userID, "", env.Data.Arena, &orderID)
	if err != nil {
		c.log.Error("auto-screening failed",
			zap.String("order_id", env.Data.OrderID),
			zap.String("user_id", env.Data.UserID),
			zap.Error(err),
		)
		// Non-fatal: continue without blocking the saga
		return nil
	}

	// Publish compliance.approved or compliance.rejected
	subject := pkgevents.SubjComplianceApproved
	if result.Status == "rejected" {
		subject = pkgevents.SubjComplianceRejected
	}

	type complianceResultPayload struct {
		OrderID     string `json:"order_id"`
		UserID      string `json:"user_id"`
		ResultID    string `json:"result_id"`
		Status      string `json:"status"`
		MatchType   string `json:"match_type"`
		MatchedName string `json:"matched_name,omitempty"`
		Provider    string `json:"provider"`
	}

	if err := pkgevents.Publish(ctx, c.bus, pkgevents.Envelope[complianceResultPayload]{
		EventType:     subject,
		AggregateID:   env.Data.OrderID,
		CorrelationID: env.Data.OrderID,
		Data: complianceResultPayload{
			OrderID:     env.Data.OrderID,
			UserID:      env.Data.UserID,
			ResultID:    result.ID.String(),
			Status:      string(result.Status),
			MatchType:   string(result.MatchType),
			MatchedName: result.MatchedName,
			Provider:    result.Provider,
		},
	}); err != nil {
		c.log.Error("publish compliance result",
			zap.String("order_id", env.Data.OrderID),
			zap.Error(err),
		)
	}

	c.log.Info("order screened",
		zap.String("order_id", env.Data.OrderID),
		zap.String("user_id", env.Data.UserID),
		zap.String("status", string(result.Status)),
	)
	return nil
}
