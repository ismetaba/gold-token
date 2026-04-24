// Package domain defines Audit Log service entity types.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// Entry is an immutable audit record.
type Entry struct {
	ID         uuid.UUID
	EventID    uuid.UUID // dedup key — maps to Envelope.EventID
	EventType  string    // e.g. "gold.mint.executed.v1"
	ActorID    string    // who performed the action (user ID, system, etc.)
	ActorType  string    // "user", "system", "admin"
	EntityID   string    // the entity being acted on
	EntityType string    // "order", "saga", "kyc_application", etc.
	Action     string    // human-readable action summary
	Metadata   []byte    // raw event JSON for forensic inspection
	OccurredAt time.Time // when the original event happened
	IngestedAt time.Time // when we persisted it
}

// ListFilter defines query filters for listing audit entries.
type ListFilter struct {
	EntityType *string
	EntityID   *string
	ActorID    *string
	Action     *string
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
}
