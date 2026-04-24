// Package provider defines the price-feed provider interface and implementations.
package provider

import "context"

// Provider fetches the current price for a given currency pair.
// All implementations return the price in quote-currency units per gram (24-karat gold).
//
// Pair format: "XAU/USD", "XAU/TRY", "XAU/EUR", "XAU/CHF"
type Provider interface {
	// FetchPrice returns the current gold price per gram in the quote currency.
	FetchPrice(ctx context.Context, pair string) (float64, error)
	// Name returns a human-readable identifier for the provider.
	Name() string
}
