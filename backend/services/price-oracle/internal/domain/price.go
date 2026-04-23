// Package domain holds types for the price oracle service.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// Price is a single XAU/USD sample expressed in USD per gram (24-karat).
type Price struct {
	ID         uuid.UUID `json:"id"`
	PriceUSDg  float64   `json:"price_usd_per_gram"` // USD per gram
	Provider   string    `json:"provider"`
	FetchedAt  time.Time `json:"fetched_at"`
}
