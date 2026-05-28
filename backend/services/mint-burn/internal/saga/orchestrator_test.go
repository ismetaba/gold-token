package saga

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/chain"
	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/repo"
)

// ── fakes ──────────────────────────────────────────────────────────────────

type fakeSagaRepo struct {
	lastState domain.SagaState
	lastEvent *repo.OutboxEvent
	enqueued  int
}

func (f *fakeSagaRepo) Create(context.Context, *domain.Saga) error          { return nil }
func (f *fakeSagaRepo) ByID(context.Context, uuid.UUID) (*domain.Saga, error) { return nil, nil }
func (f *fakeSagaRepo) NextPending(context.Context) (*domain.Saga, error)   { return nil, repo.ErrNoPending }
func (f *fakeSagaRepo) UpdateState(_ context.Context, s *domain.Saga) error {
	f.lastState = s.State
	return nil
}
func (f *fakeSagaRepo) UpdateStateAndEnqueue(_ context.Context, s *domain.Saga, evt *repo.OutboxEvent) error {
	f.lastState = s.State
	f.lastEvent = evt
	f.enqueued++
	return nil
}

type fakeBarRepo struct {
	reserveErr error
	released   bool
}

func (f *fakeBarRepo) ReserveBars(context.Context, domain.Arena, *big.Int, uuid.UUID, uuid.UUID) ([]string, error) {
	return []string{"BAR-1"}, f.reserveErr
}
func (f *fakeBarRepo) ReleaseAllocation(context.Context, uuid.UUID) error {
	f.released = true
	return nil
}
func (f *fakeBarRepo) ListAllocations(context.Context, uuid.UUID) ([]domain.BarAllocation, error) {
	return nil, nil
}

type fakeMC struct {
	approvals uint8
	executeErr error
}

func (f *fakeMC) ProposeMint(context.Context, chain.MintRequest) (string, error) { return "0xpropose", nil }
func (f *fakeMC) ExecuteMint(context.Context, [32]byte) (string, error)          { return "0xexec", f.executeErr }
func (f *fakeMC) ProposalStatus(context.Context, [32]byte) (chain.ProposalStatus, error) {
	return chain.ProposalNone, nil
}
func (f *fakeMC) ApprovalCount(context.Context, [32]byte) (uint8, error) { return f.approvals, nil }

func newSaga(state domain.SagaState) *domain.Saga {
	alloc := uuid.New()
	return &domain.Saga{
		ID:        uuid.New(),
		Type:      domain.SagaMint,
		State:     state,
		OrderID:   uuid.New(),
		Arena:     "TR",
		StartedAt: time.Now().UTC(),
		Context: domain.SagaContext{
			AllocationID: &alloc,
			MintReq: &domain.MintRequest{
				AllocationID: alloc,
				OrderID:      uuid.New(),
				AmountWei:    big.NewInt(1e18),
				Arena:        "TR",
			},
		},
	}
}

// ── tests ──────────────────────────────────────────────────────────────────

func TestReservingBarsFailureCompensatesAndStagesFailedEvent(t *testing.T) {
	sagas := &fakeSagaRepo{}
	bars := &fakeBarRepo{reserveErr: errors.New("no stock")}
	o := NewOrchestrator(sagas, bars, &fakeMC{}, zap.NewNop(), Config{ApprovalThreshold: 3})

	s := newSaga(domain.StateReservingBars)
	if err := o.step(context.Background(), s); err != nil {
		t.Fatalf("step: %v", err)
	}

	if s.State != domain.StateFailedNoStock {
		t.Fatalf("state=%s want %s", s.State, domain.StateFailedNoStock)
	}
	if !bars.released {
		t.Fatal("expected bar allocation to be released during compensation")
	}
	if sagas.lastEvent == nil || sagas.lastEvent.Subject != events.SubjMintFailed {
		t.Fatalf("expected a staged %s event, got %+v", events.SubjMintFailed, sagas.lastEvent)
	}
}

func TestAwaitingApprovalsBelowThresholdDoesNotAdvance(t *testing.T) {
	sagas := &fakeSagaRepo{}
	o := NewOrchestrator(sagas, &fakeBarRepo{}, &fakeMC{approvals: 2}, zap.NewNop(), Config{
		ApprovalThreshold: 3,
		ApprovalTimeout:   time.Hour,
	})

	s := newSaga(domain.StateAwaitingApprovals)
	if err := o.step(context.Background(), s); err != nil {
		t.Fatalf("step: %v", err)
	}
	// Below threshold: it should "touch" (plain UpdateState), not enqueue or advance.
	if s.State != domain.StateAwaitingApprovals {
		t.Fatalf("state advanced prematurely to %s", s.State)
	}
	if sagas.enqueued != 0 {
		t.Fatal("should not stage an event before approvals are met")
	}
}

func TestExecutingSuccessStagesExecutedEvent(t *testing.T) {
	sagas := &fakeSagaRepo{}
	o := NewOrchestrator(sagas, &fakeBarRepo{}, &fakeMC{}, zap.NewNop(), Config{ApprovalThreshold: 3})

	s := newSaga(domain.StateExecuting)
	if err := o.step(context.Background(), s); err != nil {
		t.Fatalf("step: %v", err)
	}
	if s.State != domain.StateCompleted {
		t.Fatalf("state=%s want completed", s.State)
	}
	if sagas.lastEvent == nil || sagas.lastEvent.Subject != events.SubjMintExecuted {
		t.Fatalf("expected staged %s event, got %+v", events.SubjMintExecuted, sagas.lastEvent)
	}
}

func TestExecutingReserveInvariantCompensates(t *testing.T) {
	sagas := &fakeSagaRepo{}
	bars := &fakeBarRepo{}
	o := NewOrchestrator(sagas, bars, &fakeMC{executeErr: chain.ErrReserveInvariant}, zap.NewNop(), Config{ApprovalThreshold: 3})

	s := newSaga(domain.StateExecuting)
	if err := o.step(context.Background(), s); err != nil {
		t.Fatalf("step: %v", err)
	}
	if s.State != domain.StateFailedReserveInvariant {
		t.Fatalf("state=%s want failed_reserve_invariant", s.State)
	}
	if !bars.released {
		t.Fatal("expected compensation to release the allocation")
	}
}
