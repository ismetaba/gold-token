package domain

import (
	"time"

	"github.com/google/uuid"
)

type ReportJob struct {
	ID          uuid.UUID
	ReportType  string // "transactions", "reserves", "compliance"
	Parameters  []byte // JSON
	Status      string // "pending", "running", "completed", "failed"
	OutputPath  string
	Error       string
	RequestedBy string
	StartedAt   *time.Time
	CompletedAt *time.Time
	CreatedAt   time.Time
}

type TransactionSummary struct {
	Date          string `json:"date"`
	MintCount     int    `json:"mint_count"`
	BurnCount     int    `json:"burn_count"`
	MintVolumeWei string `json:"mint_volume_wei"`
	BurnVolumeWei string `json:"burn_volume_wei"`
	FeeVolumeWei  string `json:"fee_volume_wei"`
}

type ReserveSummary struct {
	Date              string `json:"date"`
	GoldBalanceWei    string `json:"gold_balance_wei"`
	TokenSupplyWei    string `json:"token_supply_wei"`
	AttestationCount  int    `json:"attestation_count"`
}

type ComplianceSummary struct {
	TotalScreenings    int `json:"total_screenings"`
	ApprovedCount      int `json:"approved_count"`
	RejectedCount      int `json:"rejected_count"`
	PendingKYC         int `json:"pending_kyc"`
	ApprovedKYC        int `json:"approved_kyc"`
	RejectedKYC        int `json:"rejected_kyc"`
}
