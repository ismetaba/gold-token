// Package domain defines the Treasury service entity types and business constants.
package domain

import (
	"math/big"
	"time"

	"github.com/google/uuid"
)

// AccountType distinguishes fiat from gold reserve accounts.
type AccountType string

const (
	AccountTypeFiat AccountType = "fiat"
	AccountTypeGold AccountType = "gold"
)

// SettlementType is the direction of a fund movement.
type SettlementType string

const (
	SettlementCredit SettlementType = "credit"
	SettlementDebit  SettlementType = "debit"
)

// SettlementStatus is the lifecycle state of a settlement record.
type SettlementStatus string

const (
	SettlementPending  SettlementStatus = "pending"
	SettlementSettled  SettlementStatus = "settled"
	SettlementFailed   SettlementStatus = "failed"
)

// ReconciliationStatus indicates whether a reconciliation run found a discrepancy.
type ReconciliationStatus string

const (
	ReconciliationOK          ReconciliationStatus = "ok"
	ReconciliationDiscrepancy ReconciliationStatus = "discrepancy"
)

// ReserveAccount is the issuer's tracked balance for a currency/arena pair.
type ReserveAccount struct {
	ID          uuid.UUID
	AccountType AccountType
	BalanceWei  *big.Int
	Currency    string // e.g. "XAU", "USD"
	Arena       string // e.g. "global"
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Settlement records a single credit or debit movement against a reserve account.
type Settlement struct {
	ID             uuid.UUID
	SettlementType SettlementType
	AccountID      uuid.UUID
	AmountWei      *big.Int
	ReferenceID    uuid.UUID
	ReferenceType  string // "mint", "burn", "manual"
	TxHash         string
	Status         SettlementStatus
	SettledAt      *time.Time
	CreatedAt      time.Time
}

// ReconciliationLog captures a snapshot comparison between expected and actual balance.
type ReconciliationLog struct {
	ID                  uuid.UUID
	AccountID           uuid.UUID
	ExpectedBalanceWei  *big.Int
	ActualBalanceWei    *big.Int
	DiscrepancyWei      *big.Int // actual - expected; DB-generated but stored here post-fetch
	Status              ReconciliationStatus
	ReconciledAt        time.Time
}

// ReconcileRequest is the input to a reconciliation run.
type ReconcileRequest struct {
	AccountID          uuid.UUID
	ActualBalanceWei   *big.Int
}

// SettlementRequest is the payload for recording a new settlement.
type SettlementRequest struct {
	SettlementType SettlementType
	AccountID      uuid.UUID
	AmountWei      *big.Int
	ReferenceID    uuid.UUID
	ReferenceType  string
	TxHash         string
}
