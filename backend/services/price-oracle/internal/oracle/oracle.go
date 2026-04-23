// Package oracle drives the periodic XAU/USD price fetch, in-memory cache,
// PostgreSQL persistence, and NATS publishing.
package oracle

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/provider"
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/repo"
)

// Oracle fetches gold prices on a configurable interval and makes the
// latest price available in memory.
type Oracle struct {
	provider provider.Provider
	repo     repo.PriceRepo // may be nil in local mode
	bus      *pkgevents.Bus // may be nil in local mode
	interval time.Duration
	log      *zap.Logger

	mu      sync.RWMutex
	current *domain.Price
}

// New creates an Oracle. repo and bus may be nil; the oracle will still
// serve in-memory prices and log fetches.
func New(
	p provider.Provider,
	r repo.PriceRepo,
	bus *pkgevents.Bus,
	interval time.Duration,
	log *zap.Logger,
) *Oracle {
	return &Oracle{
		provider: p,
		repo:     r,
		bus:      bus,
		interval: interval,
		log:      log,
	}
}

// Current returns the latest cached price, or nil if no fetch has completed yet.
func (o *Oracle) Current() *domain.Price {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.current
}

// Run starts the periodic fetch loop. It blocks until ctx is cancelled.
// An immediate fetch is performed before the first ticker fires.
func (o *Oracle) Run(ctx context.Context) {
	// Fetch immediately so we have a value before the first tick.
	o.fetch(ctx)

	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			o.fetch(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// fetch performs a single price fetch-store-publish cycle.
func (o *Oracle) fetch(ctx context.Context) {
	usdPerGram, err := o.provider.FetchXAUUSD(ctx)
	if err != nil {
		o.log.Warn("price fetch failed", zap.String("provider", o.provider.Name()), zap.Error(err))
		return
	}

	p := domain.Price{
		ID:        uuid.Must(uuid.NewV7()),
		PriceUSDg: usdPerGram,
		Provider:  o.provider.Name(),
		FetchedAt: time.Now().UTC(),
	}

	o.log.Info("price fetched",
		zap.Float64("usd_per_gram", usdPerGram),
		zap.String("provider", p.Provider),
	)

	// Update in-memory cache.
	o.mu.Lock()
	o.current = &p
	o.mu.Unlock()

	// Persist to PostgreSQL.
	if o.repo != nil {
		if err := o.repo.Save(ctx, p); err != nil {
			o.log.Warn("price persist failed", zap.Error(err))
		}
	}

	// Publish to NATS.
	if o.bus != nil {
		env := pkgevents.Envelope[domain.Price]{
			EventType:   pkgevents.SubjPriceUpdated,
			AggregateID: p.ID.String(),
			Data:        p,
		}
		if err := pkgevents.Publish(ctx, o.bus, env); err != nil {
			o.log.Warn("price publish failed", zap.Error(err))
		}
	}
}
