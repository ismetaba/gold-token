package provider

import (
	"context"
	"math/rand"
	"time"
)

// Stub returns a realistic-looking simulated price for local development.
// It drifts slightly on each call so the history graph has some shape.
type Stub struct {
	rng   *rand.Rand
	price float64 // USD per gram; starts near real-world value
}

// NewStub creates a stub provider seeded from the current time.
func NewStub() *Stub {
	//nolint:gosec // non-cryptographic seed is fine for a dev stub
	return &Stub{
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
		price: 60.00, // ~$1865/oz at time of writing; adjust as needed
	}
}

func (s *Stub) Name() string { return "stub" }

func (s *Stub) FetchXAUUSD(_ context.Context) (float64, error) {
	// Random walk: ±0.5% each tick
	delta := (s.rng.Float64() - 0.5) * 0.01 * s.price
	s.price += delta
	if s.price < 50 {
		s.price = 50 // floor
	}
	return s.price, nil
}
