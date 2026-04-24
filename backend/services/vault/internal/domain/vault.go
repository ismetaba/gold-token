// Package domain defines Vault Integration service entity types.
package domain

import (
	"math/big"
	"time"

	"github.com/google/uuid"
)

// BarStatus is the lifecycle state of a gold bar in the mint schema.
type BarStatus string

const (
	BarAvailable BarStatus = "available"
	BarAllocated BarStatus = "allocated"
	BarRetired   BarStatus = "retired"
)

// GoldBar represents a physical gold bar (reads from mint.gold_bars).
type GoldBar struct {
	SerialNo        string
	VaultID         uuid.UUID
	WeightGramsWei  *big.Int
	AllocatedSumWei *big.Int
	Purity9999      int
	RefinerLBMAID   string
	CastDate        time.Time
	Status          BarStatus
	IngestedAt      time.Time
}

// BarMovement tracks a bar transfer between vaults.
type BarMovement struct {
	ID          uuid.UUID
	BarSerial   string
	FromVaultID *uuid.UUID
	ToVaultID   *uuid.UUID
	Type        string // "ingestion", "transfer", "retirement"
	InitiatedBy string
	Reason      string
	MovedAt     time.Time
}

// AuditRecord captures a physical vault audit result.
type AuditRecord struct {
	ID                uuid.UUID
	VaultID           uuid.UUID
	Auditor           string
	AuditType         string // "routine", "regulatory", "ad_hoc"
	BarCount          int
	TotalWeightWei    *big.Int
	Discrepancies     []byte // JSON
	Status            string // "passed", "discrepancy"
	AuditedAt         time.Time
	RecordedAt        time.Time
}

// IngestBarRequest is the input for registering a new gold bar.
type IngestBarRequest struct {
	SerialNo       string
	VaultID        uuid.UUID
	WeightGramsWei *big.Int
	Purity9999     int
	RefinerLBMAID  string
	CastDate       time.Time
}

// TransferBarRequest is the input for inter-vault bar transfer.
type TransferBarRequest struct {
	ToVaultID uuid.UUID
	Reason    string
}

// Vault represents a physical vault location (from mint.vaults).
type Vault struct {
	ID          uuid.UUID
	Code        string
	Arena       string
	Operator    string
	Address     string
	CountryCode string
	LBMAApproved bool
	InsuredBy   string
}
