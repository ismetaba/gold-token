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

// ─────────────────────────────────────────────────────────────────────────────
// PEP screening
// ─────────────────────────────────────────────────────────────────────────────

// PEPMatchDetail carries the details of a PEP list hit.
type PEPMatchDetail struct {
	Name     string `json:"name"`
	Position string `json:"position,omitempty"`
	Country  string `json:"country,omitempty"`
}

// PEPCheck is a single PEP-screening record.
type PEPCheck struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Matched      bool
	MatchDetails []PEPMatchDetail
	CheckedAt    time.Time
}

// ─────────────────────────────────────────────────────────────────────────────
// Ongoing monitoring
// ─────────────────────────────────────────────────────────────────────────────

// MonitoringSchedule tracks when a user is next due for re-screening.
type MonitoringSchedule struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	LastCheckedAt *time.Time
	NextCheckAt   time.Time
	FrequencyDays int
}

// ─────────────────────────────────────────────────────────────────────────────
// Jurisdiction rules
// ─────────────────────────────────────────────────────────────────────────────

// JurisdictionRule is a configurable per-arena compliance rule.
type JurisdictionRule struct {
	ID                  uuid.UUID
	Arena               string  // ISO 3166-1 alpha-2
	RuleType            string  // e.g. "enhanced_due_diligence", "source_of_funds"
	ThresholdGramsWei   *string // NUMERIC(78,0) as string; nil = no threshold
	Action              string  // e.g. "require_edd", "require_sof_decl"
	Active              bool
}

// RuleViolation describes a triggered jurisdiction rule.
type RuleViolation struct {
	Rule   JurisdictionRule
	Reason string
}
