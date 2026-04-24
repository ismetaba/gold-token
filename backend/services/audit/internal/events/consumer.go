// Package events wires the audit service into the NATS event bus as a wildcard consumer.
package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/services/audit/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/audit/internal/repo"
)

// genericEnvelope captures just enough of the envelope to extract audit metadata.
type genericEnvelope struct {
	EventID       uuid.UUID       `json:"event_id"`
	EventType     string          `json:"event_type"`
	OccurredAt    time.Time       `json:"occurred_at"`
	AggregateID   string          `json:"aggregate_id"`
	CausationID   string          `json:"causation_id"`
	CorrelationID string          `json:"correlation_id"`
	Data          json.RawMessage `json:"data"`
}

// Consumer persists every domain event into the audit log.
type Consumer struct {
	bus    *pkgevents.Bus
	repo   repo.EntryRepo
	log    *zap.Logger
	stream string
}

// NewConsumer constructs the audit event consumer.
func NewConsumer(bus *pkgevents.Bus, entryRepo repo.EntryRepo, log *zap.Logger, stream string) *Consumer {
	return &Consumer{bus: bus, repo: entryRepo, log: log, stream: stream}
}

// Start registers subscriptions for all domain event subjects. Non-blocking.
func (c *Consumer) Start(ctx context.Context) error {
	subjects := []struct {
		durable string
		subject string
	}{
		{"audit_gold_all", "gold.>"},
		{"audit_kyc_all", "kyc.>"},
		{"audit_price_all", "price.>"},
	}

	for _, s := range subjects {
		if err := c.bus.Subscribe(ctx, c.stream, s.durable, s.subject, c.handleEvent); err != nil {
			return err
		}
	}
	return nil
}

func (c *Consumer) handleEvent(ctx context.Context, data []byte) error {
	var env genericEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		c.log.Warn("unmarshal event for audit", zap.Error(err))
		return nil // don't retry unparseable events
	}

	if env.EventID == uuid.Nil {
		env.EventID = uuid.Must(uuid.NewV7())
	}
	if env.OccurredAt.IsZero() {
		env.OccurredAt = time.Now().UTC()
	}

	// Derive entity info from the event type.
	entityType, action := classifyEvent(env.EventType)

	// Try to extract actor/entity IDs from data payload.
	actorID, entityID := extractIDs(env)

	entry := domain.Entry{
		ID:         uuid.Must(uuid.NewV7()),
		EventID:    env.EventID,
		EventType:  env.EventType,
		ActorID:    actorID,
		ActorType:  "system",
		EntityID:   entityID,
		EntityType: entityType,
		Action:     action,
		Metadata:   data,
		OccurredAt: env.OccurredAt,
		IngestedAt: time.Now().UTC(),
	}

	if err := c.repo.Insert(ctx, entry); err != nil {
		c.log.Error("persist audit entry", zap.String("event_type", env.EventType), zap.Error(err))
		return err
	}

	return nil
}

// classifyEvent maps event subjects to entity types and action descriptions.
func classifyEvent(eventType string) (entityType, action string) {
	mapping := map[string][2]string{
		"gold.mint.proposed.v1":              {"saga", "mint_proposed"},
		"gold.mint.approved.v1":              {"saga", "mint_approved"},
		"gold.mint.executed.v1":              {"saga", "mint_executed"},
		"gold.mint.failed.v1":               {"saga", "mint_failed"},
		"gold.burn.requested.v1":             {"saga", "burn_requested"},
		"gold.burn.executed.v1":              {"saga", "burn_executed"},
		"gold.burn.failed.v1":                {"saga", "burn_failed"},
		"gold.order.created.v1":              {"order", "order_created"},
		"gold.order.ready_to_mint.v1":        {"order", "order_ready_to_mint"},
		"gold.order.cancelled.v1":            {"order", "order_cancelled"},
		"gold.compliance.approved.v1":        {"compliance", "compliance_approved"},
		"gold.compliance.rejected.v1":        {"compliance", "compliance_rejected"},
		"gold.compliance.alert.v1":           {"compliance", "compliance_alert"},
		"gold.compliance.freeze.v1":          {"compliance", "compliance_freeze"},
		"gold.treasury.settlement.v1":        {"treasury", "settlement_recorded"},
		"gold.treasury.reconciled.v1":        {"treasury", "reconciliation_completed"},
		"gold.reserve.snapshot.v1":           {"reserve", "reserve_snapshot"},
		"gold.reserve.attestation.v1":        {"reserve", "reserve_attestation"},
		"gold.vault.bar_ingested.v1":         {"vault", "bar_ingested"},
		"gold.vault.bar_transferred.v1":      {"vault", "bar_transferred"},
		"gold.vault.audit_completed.v1":      {"vault", "vault_audit_completed"},
		"gold.fee.calculated.v1":             {"fee", "fee_calculated"},
		"gold.fee.collected.v1":              {"fee", "fee_collected"},
		"gold.user.registered.v1":            {"user", "user_registered"},
		"kyc.submitted":                      {"kyc", "kyc_submitted"},
		"kyc.approved":                       {"kyc", "kyc_approved"},
		"kyc.rejected":                       {"kyc", "kyc_rejected"},
		"price.updated":                      {"price", "price_updated"},
	}

	if m, ok := mapping[eventType]; ok {
		return m[0], m[1]
	}
	return "unknown", eventType
}

// extractIDs attempts to pull actor and entity identifiers from the envelope.
func extractIDs(env genericEnvelope) (actorID, entityID string) {
	entityID = env.AggregateID
	actorID = "system"

	// Try to extract user_id from data payload.
	var data map[string]any
	if err := json.Unmarshal(env.Data, &data); err == nil {
		if uid, ok := data["user_id"].(string); ok && uid != "" {
			actorID = uid
		}
		if eid, ok := data["order_id"].(string); ok && eid != "" && entityID == "" {
			entityID = eid
		}
	}
	return actorID, entityID
}
