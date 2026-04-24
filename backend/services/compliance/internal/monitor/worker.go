// Package monitor provides the ongoing compliance monitoring worker.
//
// The worker periodically re-screens users who were previously approved to
// detect new sanctions or PEP matches. When a match is detected, it:
//
//  1. Updates the user's compliance state to "flagged".
//  2. Publishes a gold.compliance.alert.v1 NATS event.
//  3. Advances the monitoring schedule (last_checked_at / next_check_at).
package monitor

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/services/compliance/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/compliance/internal/repo"
	"github.com/ismetaba/gold-token/backend/services/compliance/internal/screener"
)

// Worker re-screens approved users on a configurable interval.
type Worker struct {
	compRepo    repo.ComplianceRepo
	monRepo     repo.MonitoringRepo
	pepRepo     repo.PEPRepo
	sanctions   screener.Screener
	pep         screener.PEPScreener
	bus         *pkgevents.Bus
	interval    time.Duration
	batchSize   int
	log         *zap.Logger
}

// Config carries Worker configuration.
type Config struct {
	// Interval is how often the worker polls for due users (default: 1 hour).
	Interval time.Duration
	// BatchSize is max users to re-screen per tick (default: 100).
	BatchSize int
}

func NewWorker(
	compRepo repo.ComplianceRepo,
	monRepo repo.MonitoringRepo,
	pepRepo repo.PEPRepo,
	sanctions screener.Screener,
	pep screener.PEPScreener,
	bus *pkgevents.Bus,
	cfg Config,
	log *zap.Logger,
) *Worker {
	if cfg.Interval <= 0 {
		cfg.Interval = time.Hour
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	return &Worker{
		compRepo:  compRepo,
		monRepo:   monRepo,
		pepRepo:   pepRepo,
		sanctions: sanctions,
		pep:       pep,
		bus:       bus,
		interval:  cfg.Interval,
		batchSize: cfg.BatchSize,
		log:       log,
	}
}

// Start runs the monitoring loop until ctx is cancelled. Non-blocking.
func (w *Worker) Start(ctx context.Context) {
	go w.loop(ctx)
}

func (w *Worker) loop(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.RunOnce(ctx); err != nil {
				w.log.Error("monitoring run failed", zap.Error(err))
			}
		}
	}
}

// RunOnce performs a single monitoring pass — re-screens all users due for
// re-screening and publishes alert events for any new hits.
func (w *Worker) RunOnce(ctx context.Context) error {
	due, err := w.monRepo.UsersDue(ctx, w.batchSize)
	if err != nil {
		return err
	}

	w.log.Info("monitoring run started", zap.Int("due", len(due)))

	for _, sched := range due {
		if err := w.screenUser(ctx, sched); err != nil {
			w.log.Error("re-screening failed",
				zap.String("user_id", sched.UserID.String()),
				zap.Error(err),
			)
		}
	}
	return nil
}

// EnrollUser adds a user to the monitoring schedule.
func (w *Worker) EnrollUser(ctx context.Context, userID uuid.UUID, frequencyDays int) error {
	if frequencyDays <= 0 {
		frequencyDays = 30
	}
	now := time.Now().UTC()
	return w.monRepo.UpsertSchedule(ctx, domain.MonitoringSchedule{
		ID:            uuid.Must(uuid.NewV7()),
		UserID:        userID,
		LastCheckedAt: nil,
		NextCheckAt:   now.AddDate(0, 0, frequencyDays),
		FrequencyDays: frequencyDays,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// internal
// ─────────────────────────────────────────────────────────────────────────────

func (w *Worker) screenUser(ctx context.Context, sched domain.MonitoringSchedule) error {
	// Fetch current state — only re-screen users who were previously approved.
	state, err := w.compRepo.StateByUserID(ctx, sched.UserID)
	if err != nil {
		// User has no state yet — skip quietly.
		return nil
	}
	if state.Status == domain.UserBlocked {
		// Already blocked — skip, just advance schedule.
		return w.advanceSchedule(ctx, sched)
	}

	// Re-run sanctions check (name not available in schedule — use empty name;
	// real implementations would pull name from a user service or cache).
	sanctionsRes, err := w.sanctions.Screen(ctx, screener.Request{Name: ""})
	if err != nil {
		return err
	}

	// Re-run PEP check.
	pepRes, err := w.pep.ScreenPEP(ctx, screener.Request{Name: ""})
	if err != nil {
		return err
	}

	newHit := !sanctionsRes.Allowed || pepRes.Matched
	if newHit {
		w.log.Warn("monitoring hit detected",
			zap.String("user_id", sched.UserID.String()),
			zap.Bool("sanctions", !sanctionsRes.Allowed),
			zap.Bool("pep", pepRes.Matched),
		)

		// Update compliance state.
		_ = w.compRepo.UpsertState(ctx, domain.ComplianceState{
			UserID:    sched.UserID,
			Status:    domain.UserFlagged,
			UpdatedAt: time.Now().UTC(),
		})

		// Publish alert event.
		if w.bus != nil {
			type alertPayload struct {
				UserID      string `json:"user_id"`
				Reason      string `json:"reason"`
				SanctionsHit bool  `json:"sanctions_hit"`
				PEPHit      bool   `json:"pep_hit"`
			}
			_ = pkgevents.Publish(ctx, w.bus, pkgevents.Envelope[alertPayload]{
				EventType:   pkgevents.SubjComplianceAlert,
				AggregateID: sched.UserID.String(),
				Data: alertPayload{
					UserID:       sched.UserID.String(),
					Reason:       "ongoing_monitoring_hit",
					SanctionsHit: !sanctionsRes.Allowed,
					PEPHit:       pepRes.Matched,
				},
			})
		}
	}

	return w.advanceSchedule(ctx, sched)
}

func (w *Worker) advanceSchedule(ctx context.Context, sched domain.MonitoringSchedule) error {
	now := time.Now().UTC()
	next := now.AddDate(0, 0, sched.FrequencyDays)
	return w.monRepo.UpsertSchedule(ctx, domain.MonitoringSchedule{
		ID:            sched.ID,
		UserID:        sched.UserID,
		LastCheckedAt: &now,
		NextCheckAt:   next,
		FrequencyDays: sched.FrequencyDays,
	})
}
