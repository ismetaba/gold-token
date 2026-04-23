// Package domain holds core compliance types.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// ScreeningStatus is the outcome of a single screening run.
type ScreeningStatus string

const (
	ScreeningApproved ScreeningStatus = "approved"
	ScreeningRejected ScreeningStatus = "rejected"
	ScreeningPending  ScreeningStatus = "pending"
)

// MatchType describes how a sanctions hit was found.
type MatchType string

const (
	MatchNone  MatchType = "none"
	MatchExact MatchType = "exact"
	MatchFuzzy MatchType = "fuzzy"
)

// ScreeningResult is a single sanctions-screening record.
type ScreeningResult struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	OrderID     *uuid.UUID // set when triggered by an order event
	Status      ScreeningStatus
	MatchType   MatchType
	MatchedName string // the list entry that matched, or ""
	Provider    string // "local" | "ofac" | "eu"
	ScreenedAt  time.Time
}

// UserComplianceStatus is the current aggregate status for a user.
type UserComplianceStatus string

const (
	UserClear   UserComplianceStatus = "clear"
	UserFlagged UserComplianceStatus = "flagged"
	UserBlocked UserComplianceStatus = "blocked"
)

// ComplianceState is the per-user compliance record.
type ComplianceState struct {
	UserID    uuid.UUID
	Status    UserComplianceStatus
	UpdatedAt time.Time
}
