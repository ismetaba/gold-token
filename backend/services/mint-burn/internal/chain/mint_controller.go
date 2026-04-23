// Package chain wraps the on-chain MintController/BurnController calls.
//
// Production bindings are abigen-generated from contracts/out/*.abi.json.
// For the skeleton we define the interface the saga depends on and provide a
// stub implementation that can be swapped for the real one.
package chain

import (
	"context"
	"math/big"

	"github.com/google/uuid"
)

// MintControllerClient is the saga's view of the on-chain contract.
type MintControllerClient interface {
	// ProposeMint calls MintController.proposeMint(req). Returns tx hash.
	ProposeMint(ctx context.Context, req MintRequest) (txHash string, err error)

	// ExecuteMint calls MintController.executeMint(proposalId).
	ExecuteMint(ctx context.Context, proposalID [32]byte) (txHash string, err error)

	// ProposalStatus on-chain durumu döner.
	ProposalStatus(ctx context.Context, proposalID [32]byte) (ProposalStatus, error)

	// ApprovalCount kaç imza toplandı.
	ApprovalCount(ctx context.Context, proposalID [32]byte) (uint8, error)
}

// BurnControllerClient — benzer şekilde burn/redemption için.
type BurnControllerClient interface {
	RequestRedemption(ctx context.Context, req RedemptionRequest) (txHash string, err error)
}

// MintRequest, IMintController.MintRequest ABI'ının Go karşılığı.
type MintRequest struct {
	AllocationID [32]byte
	To           [20]byte
	Amount       *big.Int
	BarSerials   [][32]byte
	Jurisdiction [2]byte
	ProposedAt   uint64
}

// RedemptionRequest, IBurnController.RedemptionRequest ABI karşılığı.
type RedemptionRequest struct {
	OffChainOrderID [32]byte
	From            [20]byte
	Amount          *big.Int
	RedemptionType  uint8
	DeliveryRef     string
}

// ProposalStatus, IMintController.ProposalStatus enum.
type ProposalStatus uint8

const (
	ProposalNone ProposalStatus = iota
	ProposalProposed
	ProposalExecuted
	ProposalCancelled
)

// UUID'yi on-chain bytes32 allocationId'ye çevirir (UUID'in 16 byte'ı solda, 16 byte sıfır padding).
func AllocationIDFromUUID(id uuid.UUID) [32]byte {
	var out [32]byte
	copy(out[:16], id[:])
	return out
}

// ─── Stub implementation (test + local dev için) ───

// StubClient, gerçek zincir yerine in-memory state tutar.
// Saga orchestrator testlerinde kullanılır.
type StubClient struct {
	Proposals map[[32]byte]*StubProposal
	NextTx    int
}

type StubProposal struct {
	Status      ProposalStatus
	Approvals   uint8
	Req         MintRequest
	ExecutedTx  string
	ProposedTx  string
}

func NewStubClient() *StubClient {
	return &StubClient{Proposals: make(map[[32]byte]*StubProposal)}
}

func (s *StubClient) nextHash(prefix string) string {
	s.NextTx++
	return prefix + "_tx_" + itoa(s.NextTx)
}

func (s *StubClient) ProposeMint(_ context.Context, req MintRequest) (string, error) {
	s.Proposals[req.AllocationID] = &StubProposal{
		Status:     ProposalProposed,
		Req:        req,
		ProposedTx: s.nextHash("propose"),
	}
	return s.Proposals[req.AllocationID].ProposedTx, nil
}

func (s *StubClient) ExecuteMint(_ context.Context, id [32]byte) (string, error) {
	p, ok := s.Proposals[id]
	if !ok {
		return "", ErrProposalNotFound
	}
	if p.Approvals < 3 {
		return "", ErrInsufficientApprovals
	}
	p.Status = ProposalExecuted
	p.ExecutedTx = s.nextHash("execute")
	return p.ExecutedTx, nil
}

func (s *StubClient) ProposalStatus(_ context.Context, id [32]byte) (ProposalStatus, error) {
	if p, ok := s.Proposals[id]; ok {
		return p.Status, nil
	}
	return ProposalNone, nil
}

func (s *StubClient) ApprovalCount(_ context.Context, id [32]byte) (uint8, error) {
	if p, ok := s.Proposals[id]; ok {
		return p.Approvals, nil
	}
	return 0, nil
}

// Test helper — onay ekle.
func (s *StubClient) AddApproval(id [32]byte) { s.Proposals[id].Approvals++ }

// itoa — stdlib olmadan tek-amaçlı int to string.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
