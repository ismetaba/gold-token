// Package saga implements the mint/burn orchestrator — the critical path
// of the GOLD off-chain system.
//
// Responsibilities:
//   1. Consume `gold.order.ready_to_mint` events, create saga instance.
//   2. Periodically advance pending sagas one step at a time.
//   3. On failure, run compensation (release bar allocations) and publish
//      `gold.mint.failed` for the Order Service to refund.
//
// Design: single-step-per-tick. Workers poll `NextPending` + FOR UPDATE SKIP LOCKED;
// ensures only one worker acts on a given saga at a time, horizontally scalable.
package saga

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/chain"
	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/repo"
)

// Config tunables. Loaded at service start.
type Config struct {
	ApprovalTimeout    time.Duration // e.g. 4h
	StepPollInterval   time.Duration // e.g. 2s
	MaxAttempts        int           // per-step retry cap (e.g. 5)
}

// Orchestrator is the saga worker. One instance per service process;
// multiple instances coordinate via DB row locks.
type Orchestrator struct {
	sagas  repo.SagaRepo
	bars   repo.BarRepo
	mc     chain.MintControllerClient
	bus    *events.Bus
	log    *zap.Logger
	cfg    Config
}

func NewOrchestrator(
	sagas repo.SagaRepo,
	bars repo.BarRepo,
	mc chain.MintControllerClient,
	bus *events.Bus,
	log *zap.Logger,
	cfg Config,
) *Orchestrator {
	return &Orchestrator{sagas: sagas, bars: bars, mc: mc, bus: bus, log: log, cfg: cfg}
}

// CreateMintSaga, Order Service'den gelen `order.ready_to_mint` event'ine yanıt.
// Yeni saga_instance oluşturur, context'e MintRequest yazar.
func (o *Orchestrator) CreateMintSaga(ctx context.Context, req domain.MintRequest) (uuid.UUID, error) {
	id := uuid.Must(uuid.NewV7())
	s := &domain.Saga{
		ID:         id,
		Type:       domain.SagaMint,
		State:      domain.StateCreated,
		OrderID:    req.OrderID,
		Arena:      req.Arena,
		StartedAt:  time.Now().UTC(),
		LastStepAt: time.Now().UTC(),
		Context: domain.SagaContext{
			MintReq:      &req,
			AllocationID: ptr(req.AllocationID),
		},
	}
	if err := o.sagas.Create(ctx, s); err != nil {
		return uuid.Nil, fmt.Errorf("create saga: %w", err)
	}
	o.log.Info("mint saga created",
		zap.String("saga_id", id.String()),
		zap.String("order_id", req.OrderID.String()),
	)
	return id, nil
}

// Run starts the polling loop. Blocks until ctx cancelled.
// Multiple Run goroutines in the same process are safe; DB skip-locked.
func (o *Orchestrator) Run(ctx context.Context) error {
	tick := time.NewTicker(o.cfg.StepPollInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			if _, err := o.tickOnce(ctx); err != nil {
				o.log.Warn("saga tick failed", zap.Error(err))
			}
		}
	}
}

func (o *Orchestrator) tickOnce(ctx context.Context) (advanced bool, err error) {
	s, err := o.sagas.NextPending(ctx)
	if err != nil {
		// "no rows" benzeri bir hata normal; "no pending saga" yolu Faz 1'de
		// repo'ya eklenir. Şimdilik log atla.
		return false, nil
	}
	return true, o.step(ctx, s)
}

// step, bir saga'yı bir adım ilerletir.
func (o *Orchestrator) step(ctx context.Context, s *domain.Saga) error {
	if s.Type != domain.SagaMint {
		return o.stepBurn(ctx, s)
	}
	switch s.State {
	case domain.StateCreated:
		return o.onCreated(ctx, s)
	case domain.StateReservingBars:
		return o.onReservingBars(ctx, s)
	case domain.StateProposing:
		return o.onProposing(ctx, s)
	case domain.StateAwaitingApprovals:
		return o.onAwaitingApprovals(ctx, s)
	case domain.StateExecuting:
		return o.onExecuting(ctx, s)
	default:
		return fmt.Errorf("no handler for state %s", s.State)
	}
}

// ──────────────────────────────────────────────────────────────────────
// Mint saga step handlers
// ──────────────────────────────────────────────────────────────────────

func (o *Orchestrator) onCreated(ctx context.Context, s *domain.Saga) error {
	// Compliance preflight: canMint? (on-chain'e sormak pahalı; Compliance Service
	// gRPC üzerinden sorulur. Skeleton: direkt ilerle.)
	// TODO(Faz1): compliance.Check(ctx, s.Context.MintReq.To, s.Arena)
	return o.advance(ctx, s, domain.StateReservingBars)
}

func (o *Orchestrator) onReservingBars(ctx context.Context, s *domain.Saga) error {
	req := s.Context.MintReq
	bars, err := o.bars.ReserveBars(ctx, req.Arena, req.AmountWei, s.ID, req.AllocationID)
	if err != nil {
		return o.compensate(ctx, s, domain.StateFailedNoStock, err, "bars_not_available")
	}
	s.Context.BarAllocations = bars
	return o.advance(ctx, s, domain.StateProposing)
}

func (o *Orchestrator) onProposing(ctx context.Context, s *domain.Saga) error {
	req := s.Context.MintReq
	cr := chain.MintRequest{
		AllocationID: chain.AllocationIDFromUUID(req.AllocationID),
		To:           [20]byte(req.To),
		Amount:       req.AmountWei,
		Jurisdiction: jurisdictionBytes(req.Arena),
		ProposedAt:   uint64(time.Now().Unix()),
	}
	cr.BarSerials = make([][32]byte, 0, len(s.Context.BarAllocations))
	for _, serial := range s.Context.BarAllocations {
		cr.BarSerials = append(cr.BarSerials, hashSerial(serial))
	}

	txHash, err := o.mc.ProposeMint(ctx, cr)
	if err != nil {
		return o.compensate(ctx, s, domain.StateFailed, err, "propose_failed")
	}
	s.Context.ProposalTxHash = txHash
	return o.advance(ctx, s, domain.StateAwaitingApprovals)
}

func (o *Orchestrator) onAwaitingApprovals(ctx context.Context, s *domain.Saga) error {
	pid := chain.AllocationIDFromUUID(s.Context.MintReq.AllocationID)

	// Timeout kontrolü
	if time.Since(s.StartedAt) > o.cfg.ApprovalTimeout {
		return o.compensate(
			ctx, s, domain.StateFailedApprovalTimeout,
			fmt.Errorf("approval timeout after %s", o.cfg.ApprovalTimeout),
			"approval_timeout",
		)
	}

	count, err := o.mc.ApprovalCount(ctx, pid)
	if err != nil {
		return fmt.Errorf("read approval count: %w", err)
	}
	if count < 3 {
		// Geri dön, sonraki tick'te tekrar bak
		return o.touch(ctx, s)
	}
	return o.advance(ctx, s, domain.StateExecuting)
}

func (o *Orchestrator) onExecuting(ctx context.Context, s *domain.Saga) error {
	pid := chain.AllocationIDFromUUID(s.Context.MintReq.AllocationID)
	txHash, err := o.mc.ExecuteMint(ctx, pid)
	if err != nil {
		// Reserve invariant ihlali ayrı state'e gitmeli.
		if err == chain.ErrReserveInvariant {
			return o.compensate(ctx, s, domain.StateFailedReserveInvariant, err, "reserve_invariant")
		}
		return o.compensate(ctx, s, domain.StateFailed, err, "execute_failed")
	}
	s.Context.ExecuteTxHash = txHash
	if err := o.advance(ctx, s, domain.StateCompleted); err != nil {
		return err
	}
	return o.publishMintExecuted(ctx, s)
}

// ──────────────────────────────────────────────────────────────────────
// Burn saga (stub; tam implementasyon Faz 1)
// ──────────────────────────────────────────────────────────────────────

func (o *Orchestrator) stepBurn(_ context.Context, s *domain.Saga) error {
	o.log.Warn("burn saga step not implemented",
		zap.String("saga_id", s.ID.String()),
		zap.String("state", string(s.State)),
	)
	return nil
}

// ──────────────────────────────────────────────────────────────────────
// Yardımcılar
// ──────────────────────────────────────────────────────────────────────

// advance, durum geçişini doğrular, DB'ye yazar, ilgili event'i yayınlar.
func (o *Orchestrator) advance(ctx context.Context, s *domain.Saga, to domain.SagaState) error {
	if err := domain.MintStateTransition(s.State, to); err != nil {
		return err
	}
	from := s.State
	s.State = to
	if err := o.sagas.UpdateState(ctx, s); err != nil {
		return fmt.Errorf("update state: %w", err)
	}
	o.log.Info("saga advanced",
		zap.String("saga_id", s.ID.String()),
		zap.String("from", string(from)),
		zap.String("to", string(to)),
	)
	return nil
}

// touch, saga'nın last_step_at'ini ileri alır (state değiştirmez) — polling backoff.
func (o *Orchestrator) touch(ctx context.Context, s *domain.Saga) error {
	return o.sagas.UpdateState(ctx, s)
}

// compensate: bar allocation'ları serbest bırak, state'i ilgili FAILED_* yap, event yay.
func (o *Orchestrator) compensate(
	ctx context.Context,
	s *domain.Saga,
	failureState domain.SagaState,
	cause error,
	errorCode string,
) error {
	if s.Context.AllocationID != nil {
		if err := o.bars.ReleaseAllocation(ctx, *s.Context.AllocationID); err != nil {
			o.log.Error("release allocation failed during compensate",
				zap.String("saga_id", s.ID.String()),
				zap.Error(err),
			)
			// Devam et — admin müdahalesi için alert eklenmeli
		}
	}
	s.State = failureState
	s.Context.LastError = cause.Error()
	s.Context.LastErrorCode = errorCode
	s.Context.LastErrorAt = time.Now().UTC()
	if err := o.sagas.UpdateState(ctx, s); err != nil {
		return fmt.Errorf("update compensate state: %w", err)
	}
	o.log.Warn("saga compensated",
		zap.String("saga_id", s.ID.String()),
		zap.String("failure_state", string(failureState)),
		zap.String("cause", cause.Error()),
	)
	return o.publishMintFailed(ctx, s, errorCode)
}

// publishMintExecuted → gold.mint.executed.v1
func (o *Orchestrator) publishMintExecuted(ctx context.Context, s *domain.Saga) error {
	type payload struct {
		SagaID       string `json:"saga_id"`
		OrderID      string `json:"order_id"`
		AmountWei    string `json:"amount_wei"`
		TxHash       string `json:"tx_hash"`
		AllocationID string `json:"allocation_id"`
		ToAddress    string `json:"to_address"` // 0x-prefixed; wallet service uses this to attribute the tx
	}
	to := s.Context.MintReq.To
	return events.Publish(ctx, o.bus, events.Envelope[payload]{
		EventType:     events.SubjMintExecuted,
		AggregateID:   s.ID.String(),
		CorrelationID: s.ID.String(),
		Data: payload{
			SagaID:       s.ID.String(),
			OrderID:      s.OrderID.String(),
			AmountWei:    s.Context.MintReq.AmountWei.String(),
			TxHash:       s.Context.ExecuteTxHash,
			AllocationID: s.Context.MintReq.AllocationID.String(),
			ToAddress:    fmt.Sprintf("0x%x", to[:]),
		},
	})
}

// publishMintFailed → gold.mint.failed.v1
func (o *Orchestrator) publishMintFailed(ctx context.Context, s *domain.Saga, code string) error {
	type payload struct {
		SagaID    string `json:"saga_id"`
		OrderID   string `json:"order_id"`
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	return events.Publish(ctx, o.bus, events.Envelope[payload]{
		EventType:     events.SubjMintFailed,
		AggregateID:   s.ID.String(),
		CorrelationID: s.ID.String(),
		Data: payload{
			SagaID:    s.ID.String(),
			OrderID:   s.OrderID.String(),
			ErrorCode: code,
			Message:   s.Context.LastError,
		},
	})
}

// ──────────────────────────────────────────────────────────────────────
// Tip dönüşüm yardımcıları
// ──────────────────────────────────────────────────────────────────────

func jurisdictionBytes(a domain.Arena) [2]byte {
	var out [2]byte
	copy(out[:], a)
	return out
}

func hashSerial(serial string) [32]byte {
	// TODO(chain): keccak256(abi.encode(serial, weight, purity, vault, lbmaId))
	// Skeleton: direkt byte kopyası (32-byte truncate/pad). Production'da tam ABI hash.
	var out [32]byte
	copy(out[:], []byte(serial))
	return out
}

func ptr[T any](v T) *T { return &v }
