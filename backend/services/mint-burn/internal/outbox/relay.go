// Package outbox publishes events staged in the transactional outbox.
//
// Events are written to mint.outbox in the same DB transaction as the state
// change that produced them (see repo.SagaRepo.UpdateStateAndEnqueue). This
// relay then polls for unpublished rows, publishes each to NATS, and marks it
// published — guaranteeing at-least-once delivery even if the process crashes
// between the state commit and publication. The outbox row ID is used as the
// JetStream Msg-Id, so a redelivered row is de-duplicated downstream.
package outbox

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/repo"
)

// Publisher publishes a raw payload to a subject with a dedup key.
type Publisher interface {
	PublishRaw(ctx context.Context, subject, msgID string, payload []byte) error
}

// Relay drains the outbox to the event bus on an interval.
type Relay struct {
	repo     repo.OutboxRepo
	pub      Publisher
	log      *zap.Logger
	interval time.Duration
	batch    int
}

// NewRelay constructs a relay. If interval <= 0 it defaults to 1s.
func NewRelay(r repo.OutboxRepo, pub Publisher, log *zap.Logger, interval time.Duration) *Relay {
	if interval <= 0 {
		interval = time.Second
	}
	return &Relay{repo: r, pub: pub, log: log, interval: interval, batch: 100}
}

// Run drains the outbox until ctx is cancelled.
func (r *Relay) Run(ctx context.Context) error {
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if err := r.drain(ctx); err != nil {
				r.log.Warn("outbox drain failed", zap.Error(err))
			}
		}
	}
}

func (r *Relay) drain(ctx context.Context) error {
	rows, err := r.repo.FetchUnpublished(ctx, r.batch)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := r.pub.PublishRaw(ctx, row.Subject, row.ID.String(), row.Payload); err != nil {
			// Leave the row unpublished; it will be retried on the next tick.
			r.log.Warn("outbox publish failed; will retry",
				zap.String("subject", row.Subject),
				zap.String("event_id", row.ID.String()),
				zap.Error(err),
			)
			return nil
		}
		if err := r.repo.MarkPublished(ctx, row.ID); err != nil {
			// Publication succeeded but the ack failed; the row will be
			// re-published next tick and de-duplicated downstream via Msg-Id.
			r.log.Warn("outbox mark-published failed",
				zap.String("event_id", row.ID.String()),
				zap.Error(err),
			)
			return nil
		}
	}
	return nil
}
