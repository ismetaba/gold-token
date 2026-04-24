// Package domain defines Fee Management service entity types.
package domain

import (
	"math/big"
	"time"

	"github.com/google/uuid"
)

// FeeSchedule defines a fee tier for a specific operation type and arena.
type FeeSchedule struct {
	ID              uuid.UUID
	Name            string
	OperationType   string   // "mint", "burn", "transfer"
	Arena           string   // jurisdiction code or "global"
	TierMinGramsWei *big.Int // minimum amount for this tier (inclusive)
	TierMaxGramsWei *big.Int // maximum amount for this tier (exclusive); nil = unlimited
	FeeBPS          int      // fee in basis points (100 bps = 1%)
	MinFeeWei       *big.Int // minimum fee regardless of calculation
	Active          bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// LedgerEntry records a fee calculation or collection event.
type LedgerEntry struct {
	ID            uuid.UUID
	OrderID       uuid.UUID
	OperationType string
	AmountWei     *big.Int // order amount
	FeeWei        *big.Int // calculated fee
	FeeBPS        int
	Arena         string
	Status        string // "calculated", "collected", "refunded"
	CollectedAt   *time.Time
	CreatedAt     time.Time
}

// CalculateRequest is the input for fee calculation.
type CalculateRequest struct {
	OperationType string
	AmountGrams   string // decimal string e.g. "1.5"
	Arena         string
}

// CalculateResponse is the result of a fee calculation.
type CalculateResponse struct {
	FeeWei        string `json:"fee_wei"`
	FeeBPS        int    `json:"fee_bps"`
	AmountWei     string `json:"amount_wei"`
	OperationType string `json:"operation_type"`
	Arena         string `json:"arena"`
}
