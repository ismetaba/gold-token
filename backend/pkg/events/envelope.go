// Package events provides the NATS-backed event bus wrapper.
package events

import (
	"time"

	"github.com/google/uuid"
)

// Envelope is the canonical event envelope for GOLD.
//
// All events flow through NATS JetStream with Subject == EventType
// and Msg-Id == EventID for dedup. Payloads are JSON.
type Envelope[T any] struct {
	EventID       uuid.UUID `json:"event_id"`
	EventType     string    `json:"event_type"` // e.g. "gold.mint.executed.v1"
	OccurredAt    time.Time `json:"occurred_at"`
	AggregateID   string    `json:"aggregate_id"`
	CausationID   string    `json:"causation_id,omitempty"`
	CorrelationID string    `json:"correlation_id,omitempty"` // saga ID
	Version       int       `json:"version"`
	Data          T         `json:"data"`
}

// Subjects are the canonical NATS subjects.
const (
	// Order
	SubjOrderReadyToMint = "gold.order.ready_to_mint.v1"
	SubjOrderCancelled   = "gold.order.cancelled.v1"

	// Mint
	SubjMintProposed = "gold.mint.proposed.v1"
	SubjMintApproved = "gold.mint.approved.v1"
	SubjMintExecuted = "gold.mint.executed.v1"
	SubjMintFailed   = "gold.mint.failed.v1"

	// Burn
	SubjBurnRequested = "gold.burn.requested.v1"
	SubjBurnExecuted  = "gold.burn.executed.v1"
	SubjBurnFailed    = "gold.burn.failed.v1"

	// Reserve
	SubjReserveSnapshot     = "gold.reserve.snapshot.v1"
	SubjReserveAttestation  = "gold.reserve.attestation.v1"

	// Order (created — published by order service)
	SubjOrderCreated = "gold.order.created.v1"

	// Compliance
	SubjComplianceApproved = "gold.compliance.approved.v1"
	SubjComplianceRejected = "gold.compliance.rejected.v1"
	SubjComplianceAlert    = "gold.compliance.alert.v1"
	SubjComplianceFreeze   = "gold.compliance.freeze.v1"

	// KYC
	SubjKYCSubmitted = "kyc.submitted"
	SubjKYCApproved  = "kyc.approved"
	SubjKYCRejected  = "kyc.rejected"

	// Price oracle
	SubjPriceUpdated = "price.updated"
)
