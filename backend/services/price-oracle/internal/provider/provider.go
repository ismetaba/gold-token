// Package provider defines the price-feed provider interface and implementations.
package provider

import "context"

// Provider fetches the current XAU/USD price.
// All implementations must return USD per gram (24-karat gold).
type Provider interface {
	// FetchXAUUSD returns the current gold price in USD per gram.
	FetchXAUUSD(ctx context.Context) (float64, error)
	// Name returns a human-readable identifier for the provider.
	Name() string
}
