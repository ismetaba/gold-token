// Package oracle drives the periodic multi-pair price fetch, in-memory cache,
// PostgreSQL persistence, NATS publishing, OHLCV candle computation, and
// WebSocket broadcast.
package oracle

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/provider"
	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/repo"
)

// Oracle fetches gold prices from multiple providers, aggregates by median,
// and makes the latest prices for all supported pairs available in memory.
type Oracle struct {
	providers []provider.Provider
	pairs     []string
	repo      repo.PriceRepo   // may be nil in local mode
	bus       *pkgevents.Bus   // may be nil in local mode
	interval  time.Duration
	log       *zap.Logger

	mu      sync.RWMutex
	current map[string]*domain.Price // pair -> latest price

	// WebSocket hub — broadcast updates to all connected clients.
	hub *Hub
}

// New creates an Oracle with the given providers and pairs.
// repo and bus may be nil; the oracle will still serve in-memory prices.
func New(
	providers []provider.Provider,
	pairs []string,
	r repo.PriceRepo,
	bus *pkgevents.Bus,
	interval time.Duration,
	log *zap.Logger,
) *Oracle {
	o := &Oracle{
		providers: providers,
		pairs:     pairs,
		repo:      r,
		bus:       bus,
		interval:  interval,
		log:       log,
		current:   make(map[string]*domain.Price),
		hub:       NewHub(),
	}
	return o
}

// Hub returns the WebSocket hub so HTTP handlers can subscribe clients.
func (o *Oracle) Hub() *Hub { return o.hub }

// Current returns the latest cached price for the given pair, or nil.
func (o *Oracle) Current(pair string) *domain.Price {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.current[pair]
}

// CurrentAll returns a snapshot of all latest prices.
func (o *Oracle) CurrentAll() map[string]*domain.Price {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make(map[string]*domain.Price, len(o.current))
	for k, v := range o.current {
		cp := *v
		out[k] = &cp
	}
	return out
}

// Run starts the periodic fetch loop and the WebSocket hub. Blocks until ctx is cancelled.
func (o *Oracle) Run(ctx context.Context) {
	go o.hub.Run(ctx)

	// Immediate fetch before the first tick.
	o.fetchAll(ctx)

	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			o.fetchAll(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// fetchAll fetches prices for every configured pair from all providers and
// updates the in-memory cache, persists to DB, publishes to NATS, builds
// candles, and broadcasts over WebSocket.
func (o *Oracle) fetchAll(ctx context.Context) {
	now := time.Now().UTC()
	for _, pair := range o.pairs {
		p := o.fetchMedian(ctx, pair, now)
		if p == nil {
			continue
		}

		o.mu.Lock()
		o.current[pair] = p
		o.mu.Unlock()

		o.log.Info("price updated",
			zap.String("pair", pair),
			zap.Float64("price_per_gram", p.PricePerGram),
			zap.String("provider", p.Provider),
		)

		if o.repo != nil {
			if err := o.repo.Save(ctx, *p); err != nil {
				o.log.Warn("price persist failed", zap.String("pair", pair), zap.Error(err))
			}
			// Upsert candles for each interval.
			for _, interval := range []string{"1h", "4h", "1d"} {
				if err := o.repo.UpsertCandle(ctx, *p, interval); err != nil {
					o.log.Warn("candle upsert failed",
						zap.String("pair", pair),
						zap.String("interval", interval),
						zap.Error(err),
					)
				}
			}
		}

		if o.bus != nil {
			env := pkgevents.Envelope[domain.Price]{
				EventType:   pkgevents.SubjPriceUpdated,
				AggregateID: p.ID.String(),
				Data:        *p,
			}
			if err := pkgevents.Publish(ctx, o.bus, env); err != nil {
				o.log.Warn("price publish failed", zap.String("pair", pair), zap.Error(err))
			}
		}

		// Broadcast to WebSocket clients.
		update := domain.PriceUpdate{
			Pair:         p.Pair,
			PricePerGram: p.PricePerGram,
			Provider:     p.Provider,
			FetchedAt:    p.FetchedAt,
		}
		if msg, err := json.Marshal(update); err == nil {
			o.hub.Broadcast(msg)
		}
	}
}

// fetchMedian fetches the price for pair from all providers concurrently and
// returns a Price whose value is the median of all successful responses.
// Returns nil if no provider succeeds.
func (o *Oracle) fetchMedian(ctx context.Context, pair string, now time.Time) *domain.Price {
	type result struct {
		value    float64
		provider string
	}

	ch := make(chan result, len(o.providers))

	for _, p := range o.providers {
		p := p
		go func() {
			val, err := p.FetchPrice(ctx, pair)
			if err != nil {
				o.log.Warn("provider fetch failed",
					zap.String("provider", p.Name()),
					zap.String("pair", pair),
					zap.Error(err),
				)
				ch <- result{}
				return
			}
			ch <- result{value: val, provider: p.Name()}
		}()
	}

	var values []float64
	var providerNames []string
	for range o.providers {
		r := <-ch
		if r.value > 0 {
			values = append(values, r.value)
			providerNames = append(providerNames, r.provider)
		}
	}

	if len(values) == 0 {
		o.log.Warn("all providers failed", zap.String("pair", pair))
		return nil
	}

	sort.Float64s(values)
	median := values[len(values)/2]

	// Composite provider label lists all contributing sources.
	providerLabel := joinProviders(providerNames)

	return &domain.Price{
		ID:           uuid.Must(uuid.NewV7()),
		Pair:         pair,
		PricePerGram: median,
		Provider:     providerLabel,
		FetchedAt:    now,
	}
}

func joinProviders(names []string) string {
	if len(names) == 0 {
		return ""
	}
	out := names[0]
	for _, n := range names[1:] {
		out += "," + n
	}
	return out
}
