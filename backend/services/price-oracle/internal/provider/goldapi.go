package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// GoldAPI implements Provider using goldapi.io.
// API docs: https://www.goldapi.io/dashboard
// The endpoint pattern is: GET https://www.goldapi.io/api/{metal}/{currency}
// where metal="XAU" and currency is e.g. "USD", "TRY", "EUR", "CHF".
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
	Price        float64 `json:"price"` // per troy ounce fallback
}

const troyOzToGrams = 31.1034768

// FetchPrice returns the price per gram for the given pair (e.g. "XAU/USD").
func (g *GoldAPI) FetchPrice(ctx context.Context, pair string) (float64, error) {
	metal, currency, err := splitPair(pair)
	if err != nil {
		return 0, fmt.Errorf("goldapi: %w", err)
	}

	url := fmt.Sprintf("https://www.goldapi.io/api/%s/%s", metal, currency)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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

	if body.PriceGram24k > 0 {
		return body.PriceGram24k, nil
	}
	if body.Price > 0 {
		return body.Price / troyOzToGrams, nil
	}
	return 0, fmt.Errorf("goldapi: no price in response")
}

// splitPair splits "XAU/USD" into ("XAU", "USD").
func splitPair(pair string) (metal, currency string, err error) {
	parts := strings.SplitN(pair, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid pair %q: expected METAL/CURRENCY", pair)
	}
	return parts[0], parts[1], nil
}
