package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SagaType ayırt eder — mint veya burn.
type SagaType string

const (
	SagaMint SagaType = "mint"
	SagaBurn SagaType = "burn"
)

// SagaState, mint/burn saga'sının makine durumu.
//
// Geçiş grafiği (mint):
//
//	CREATED → RESERVING_BARS → PROPOSING → AWAITING_APPROVALS → EXECUTING → COMPLETED
//	                        ↘                                 ↘
//	                         FAILED                           FAILED_APPROVAL_TIMEOUT
//
// Her FAILED_* durumu compensation (bar serbest bırakma + iade) tetikler ve terminaldir.
type SagaState string

const (
	StateCreated                SagaState = "created"
	StateReservingBars          SagaState = "reserving_bars"
	StateProposing              SagaState = "proposing"           // on-chain proposeMint bekleniyor
	StateAwaitingApprovals      SagaState = "awaiting_approvals"
	StateExecuting              SagaState = "executing"           // on-chain executeMint bekleniyor
	StateCompleted              SagaState = "completed"
	StateFailed                 SagaState = "failed"
	StateFailedNoStock          SagaState = "failed_no_stock"
	StateFailedApprovalTimeout  SagaState = "failed_approval_timeout"
	StateFailedReserveInvariant SagaState = "failed_reserve_invariant"

	// Burn saga states (daha sade)
	StateBurnRequested       SagaState = "burn_requested"
	StateBurnRequestingChain SagaState = "burn_requesting_chain"
	StateBurnExecuted        SagaState = "burn_executed"
)

// IsTerminal belirtir: state makinesi bu durumdan ileri gitmez.
func (s SagaState) IsTerminal() bool {
	switch s {
	case StateCompleted, StateBurnExecuted,
		StateFailed, StateFailedNoStock, StateFailedApprovalTimeout, StateFailedReserveInvariant:
		return true
	default:
		return false
	}
}

// Saga, DB'de saklanan bir saga örneği.
type Saga struct {
	ID          uuid.UUID
	Type        SagaType
	State       SagaState
	OrderID     uuid.UUID
	Arena       Arena
	Context     SagaContext
	StartedAt   time.Time
	LastStepAt  time.Time
	CompletedAt *time.Time
	Attempts    int
}

// SagaContext, adımlar arasında taşınan ara değişkenler.
// JSONB olarak saklanır — yeni alanlar ekleme upgrade-safe.
type SagaContext struct {
	// Girdi
	MintReq *MintRequest `json:"mint_req,omitempty"`
	BurnReq *BurnRequest `json:"burn_req,omitempty"`

	// Ara durumlar
	AllocationID   *uuid.UUID `json:"allocation_id,omitempty"`
	BarAllocations []string   `json:"bar_allocations,omitempty"` // seri numaraları
	ProposalTxHash string     `json:"proposal_tx_hash,omitempty"`
	ExecuteTxHash  string     `json:"execute_tx_hash,omitempty"`
	BurnTxHash     string     `json:"burn_tx_hash,omitempty"`

	// Durum açıklama
	LastError     string    `json:"last_error,omitempty"`
	LastErrorCode string    `json:"last_error_code,omitempty"`
	LastErrorAt   time.Time `json:"last_error_at,omitempty"`
}

// Marshal yardımcıları (pgx jsonb için)
func (c *SagaContext) MarshalJSON() ([]byte, error) {
	type alias SagaContext
	return json.Marshal((*alias)(c))
}

func (c *SagaContext) String() string {
	b, _ := json.Marshal(c)
	return string(b)
}

// MintStateTransition, izin verilen geçişleri doğrular.
// Yasadışı geçiş girişimi programlama hatasıdır — panic yerine error döner.
func MintStateTransition(from, to SagaState) error {
	allowed := map[SagaState][]SagaState{
		StateCreated:           {StateReservingBars, StateFailed},
		StateReservingBars:     {StateProposing, StateFailedNoStock, StateFailed},
		StateProposing:         {StateAwaitingApprovals, StateFailed},
		StateAwaitingApprovals: {StateExecuting, StateFailedApprovalTimeout, StateFailed},
		StateExecuting:         {StateCompleted, StateFailedReserveInvariant, StateFailed},
	}
	for _, next := range allowed[from] {
		if next == to {
			return nil
		}
	}
	return fmt.Errorf("invalid state transition: %s → %s", from, to)
}
