// Package domain holds types for the price oracle service.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// SupportedPairs is the canonical set of currency pairs this service handles.
var SupportedPairs = []string{
	"XAU/USD",
	"XAU/TRY",
	"XAU/EUR",
	"XAU/CHF",
}

// Price is a single price sample for a given pair, expressed as units of
// the quote currency per gram (24-karat gold).
type Price struct {
	ID          uuid.UUID `json:"id"`
	Pair        string    `json:"pair"`          // e.g. "XAU/USD"
	PricePerGram float64  `json:"price_per_gram"` // quote currency per gram
	Provider    string    `json:"provider"`
	FetchedAt   time.Time `json:"fetched_at"`
}

// Candle is an OHLCV candlestick for a given pair and interval.
type Candle struct {
	ID            uuid.UUID `json:"id"`
	Pair          string    `json:"pair"`
	Interval      string    `json:"interval"`       // "1h", "4h", "1d"
	OpenPerGram   float64   `json:"open_per_gram"`
	HighPerGram   float64   `json:"high_per_gram"`
	LowPerGram    float64   `json:"low_per_gram"`
	ClosePerGram  float64   `json:"close_per_gram"`
	Volume        float64   `json:"volume"`
	BucketStart   time.Time `json:"bucket_start"`
	BucketEnd     time.Time `json:"bucket_end"`
}

// PriceUpdate is the payload broadcast over WebSocket connections.
type PriceUpdate struct {
	Pair         string    `json:"pair"`
	PricePerGram float64   `json:"price_per_gram"`
	Provider     string    `json:"provider"`
	FetchedAt    time.Time `json:"fetched_at"`
}
