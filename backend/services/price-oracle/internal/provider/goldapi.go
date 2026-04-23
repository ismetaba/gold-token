package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GoldAPI implements Provider using goldapi.io.
// API docs: https://www.goldapi.io/dashboard
// Free tier: ~100 requests/month. Upgrade for higher limits.
type GoldAPI struct {
	apiKey     string
	httpClient *http.Client
}

// NewGoldAPI creates a GoldAPI provider with the given API key.
func NewGoldAPI(apiKey string) *GoldAPI {
	return &GoldAPI{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (g *GoldAPI) Name() string { return "goldapi.io" }

type goldAPIResponse struct {
	PriceGram24k float64 `json:"price_gram_24k"`
	Price        float64 `json:"price"` // USD per troy ounce (fallback)
}

const troyOzToGrams = 31.1034768

func (g *GoldAPI) FetchXAUUSD(ctx context.Context) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.goldapi.io/api/XAU/USD", nil)
	if err != nil {
		return 0, fmt.Errorf("goldapi: build request: %w", err)
	}
	req.Header.Set("x-access-token", g.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("goldapi: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("goldapi: unexpected status %d", resp.StatusCode)
	}

	var body goldAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, fmt.Errorf("goldapi: decode: %w", err)
	}

	// Prefer the pre-computed gram price; fall back to per-troy-oz conversion.
	if body.PriceGram24k > 0 {
		return body.PriceGram24k, nil
	}
	if body.Price > 0 {
		return body.Price / troyOzToGrams, nil
	}
	return 0, fmt.Errorf("goldapi: no price in response")
}
