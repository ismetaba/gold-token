package provider

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Stub returns realistic-looking simulated prices for local development.
// Each pair is tracked independently with a small random walk each call.
type Stub struct {
	mu     sync.Mutex
	rng    *rand.Rand
	prices map[string]float64 // pair -> current price per gram
}

// stubBasePrice holds realistic starting prices per gram in each currency.
var stubBasePrice = map[string]float64{
	"XAU/USD": 60.00,  // ~$1,865/oz
	"XAU/TRY": 1950.0, // approximate
	"XAU/EUR": 55.00,
	"XAU/CHF": 53.50,
}

// NewStub creates a stub provider seeded from the current time.
func NewStub() *Stub {
	prices := make(map[string]float64, len(stubBasePrice))
	for pair, base := range stubBasePrice {
		prices[pair] = base
	}
	//nolint:gosec // non-cryptographic seed is fine for a dev stub
	return &Stub{
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
		prices: prices,
	}
}

func (s *Stub) Name() string { return "stub" }

// FetchPrice returns a slowly drifting simulated price for the requested pair.
func (s *Stub) FetchPrice(_ context.Context, pair string) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	price, ok := s.prices[pair]
	if !ok {
		return 0, fmt.Errorf("stub: unsupported pair %q", pair)
	}

	// Random walk: ±0.5% each tick.
	delta := (s.rng.Float64() - 0.5) * 0.01 * price
	price += delta
	if price < 1 {
		price = 1 // floor
	}
	s.prices[pair] = price
	return price, nil
}
