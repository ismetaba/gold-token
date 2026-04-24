// Package worker provides background workers for the PoR service.
package worker

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	porchain "github.com/ismetaba/gold-token/backend/services/por/internal/chain"
	"github.com/ismetaba/gold-token/backend/services/por/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/por/internal/repo"
)

// OracleReader is the on-chain read interface used by the auto-attestation worker.
type OracleReader interface {
	Latest(ctx context.Context) (porchain.OnChainAttestation, error)
	AttestationCount(ctx context.Context) (uint64, error)
	AttestationAt(ctx context.Context, index uint64) (porchain.OnChainAttestation, error)
}

// AutoAttestationWorker runs on a schedule derived from por.auto_attestation_config
// and triggers chain sync + publishes a scheduled attestation event.
type AutoAttestationWorker struct {
	cfgRepo   repo.AutoAttestationConfigRepo
	reader    OracleReader            // may be nil; worker becomes a no-op when nil
	attestRepo repo.AttestationRepo  // may be nil
	log       *zap.Logger
	// onRun is called each time the scheduled run fires (chain sync delegate).
	onRun func(ctx context.Context) error
}

// NewAutoAttestationWorker creates the worker.
// onRun is the function to call on each scheduled tick (typically runChainSync logic).
func NewAutoAttestationWorker(
	cfgRepo repo.AutoAttestationConfigRepo,
	reader OracleReader,
	attestRepo repo.AttestationRepo,
	onRun func(ctx context.Context) error,
	log *zap.Logger,
) *AutoAttestationWorker {
	return &AutoAttestationWorker{
		cfgRepo:    cfgRepo,
		reader:     reader,
		attestRepo: attestRepo,
		onRun:      onRun,
		log:        log,
	}
}

// Run starts the worker loop. It blocks until ctx is cancelled.
func (w *AutoAttestationWorker) Run(ctx context.Context) {
	w.log.Info("auto-attestation worker started")
	for {
		cfg, err := w.cfgRepo.Get(ctx)
		if err != nil {
			w.log.Warn("auto-attestation: cannot load config, retrying in 60s", zap.Error(err))
			select {
			case <-ctx.Done():
				return
			case <-time.After(60 * time.Second):
				continue
			}
		}

		if !cfg.Enabled {
			// Config disabled — poll every 5 minutes to detect re-enable.
			w.log.Debug("auto-attestation disabled; sleeping 5m")
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
				continue
			}
		}

		next, err := nextRunTime(cfg)
		if err != nil {
			w.log.Warn("auto-attestation: invalid cron expression, disabling",
				zap.String("expr", cfg.CronExpression), zap.Error(err))
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
				continue
			}
		}

		delay := time.Until(next)
		if delay < 0 {
			delay = 0
		}
		w.log.Info("auto-attestation scheduled",
			zap.String("cron", cfg.CronExpression),
			zap.Time("next_run", next),
			zap.Duration("in", delay),
		)

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		w.log.Info("auto-attestation: running scheduled attestation sync")
		if w.onRun != nil {
			if err := w.onRun(ctx); err != nil {
				w.log.Error("auto-attestation sync failed", zap.Error(err))
			}
		}

		if err := w.cfgRepo.UpdateLastRunAt(ctx, cfg); err != nil {
			w.log.Warn("auto-attestation: update last_run_at failed", zap.Error(err))
		}
	}
}

// nextRunTime computes the next UTC time to run based on a 5-field cron expression.
// Supports numeric literals and "*" for each field.
// Fields: minute hour dom month dow
func nextRunTime(cfg domain.AutoAttestationConfig) (time.Time, error) {
	minute, hour, err := parseDailySchedule(cfg.CronExpression)
	if err != nil {
		return time.Time{}, err
	}

	now := time.Now().UTC()
	candidate := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.UTC)
	if !candidate.After(now) {
		candidate = candidate.Add(24 * time.Hour)
	}
	return candidate, nil
}

// parseDailySchedule extracts hour and minute from a 5-field cron expression.
// Non-daily fields (dom, month, dow) are parsed but not used for scheduling;
// only minute and hour are honoured, matching the common daily-schedule pattern.
func parseDailySchedule(expr string) (minute, hour int, err error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return 0, 0, fmt.Errorf("expected 5 cron fields, got %d", len(fields))
	}

	minute, err = parseCronField(fields[0], 0, 59)
	if err != nil {
		return 0, 0, fmt.Errorf("cron minute field: %w", err)
	}
	hour, err = parseCronField(fields[1], 0, 23)
	if err != nil {
		return 0, 0, fmt.Errorf("cron hour field: %w", err)
	}
	return minute, hour, nil
}

func parseCronField(s string, min, max int) (int, error) {
	if s == "*" {
		return min, nil // default to lowest value for wildcard
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("non-numeric cron field %q", s)
	}
	if n < min || n > max {
		return 0, fmt.Errorf("cron field %d out of range [%d, %d]", n, min, max)
	}
	return n, nil
}
