// Package events provides vault event publishing helpers.
package events

import (
	"context"

	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
)

// Event subjects for vault operations.
const (
	SubjBarIngested     = "gold.vault.bar_ingested.v1"
	SubjBarTransferred  = "gold.vault.bar_transferred.v1"
	SubjAuditCompleted  = "gold.vault.audit_completed.v1"
)

// Publisher wraps the event bus for vault-specific publishing.
type Publisher struct {
	bus *pkgevents.Bus
	log *zap.Logger
}

// NewPublisher constructs a vault event publisher.
func NewPublisher(bus *pkgevents.Bus, log *zap.Logger) *Publisher {
	return &Publisher{bus: bus, log: log}
}

// Publish emits a vault event. Silently logs errors (non-fatal).
func (p *Publisher) Publish(ctx context.Context, subj, aggregateID string, data map[string]any) {
	if p.bus == nil {
		return
	}
	env := pkgevents.Envelope[map[string]any]{
		EventType:   subj,
		AggregateID: aggregateID,
		Data:        data,
	}
	if err := pkgevents.Publish(ctx, p.bus, env); err != nil {
		p.log.Warn("vault event publish failed", zap.String("subject", subj), zap.Error(err))
	}
}
