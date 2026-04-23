// Package domain defines the KYC application entity and related types.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// Status represents the lifecycle state of a KYC application.
type Status string

const (
	StatusPending     Status = "pending"
	StatusUnderReview Status = "under_review"
	StatusApproved    Status = "approved"
	StatusRejected    Status = "rejected"
)

// Application is the core KYC entity.
type Application struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Status       Status
	DocumentPath string // relative path within the storage root
	FirstName    string
	LastName     string
	DateOfBirth  string // ISO 8601 date: YYYY-MM-DD
	Nationality  string
	ReviewerNote string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ReviewedAt   *time.Time
}

// NATS event subjects.
const (
	SubjKYCSubmitted = "kyc.submitted"
	SubjKYCApproved  = "kyc.approved"
	SubjKYCRejected  = "kyc.rejected"
)
