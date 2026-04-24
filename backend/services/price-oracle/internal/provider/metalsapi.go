package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// MetalsAPI implements Provider using metals-api.com.
// API docs: https://metals-api.com/documentation
// The endpoint fetches gold (XAU) in the requested currency.
// Free tier: 100 requests/month. API key required.
//
// Response shape (simplified):
//
//	{ "success": true, "rates": { "XAU": 0.000532 }, "base": "USD" }
//
// metals-api returns the number of troy ounces per base currency unit (i.e. XAU/USD rate
// expressed as ounces per dollar). We invert and convert to grams per quote unit.
type MetalsAPI struct {
	apiKey     string
	httpClient *http.Client
}

// NewMetalsAPI creates a MetalsAPI provider with the given API key.
func NewMetalsAPI(apiKey string) *MetalsAPI {
	return &MetalsAPI{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (m *MetalsAPI) Name() string { return "metals-api.com" }

type metalsAPIResponse struct {
	Success bool               `json:"success"`
	Rates   map[string]float64 `json:"rates"` // e.g. { "XAU": 0.000532 }
	Base    string             `json:"base"`  // e.g. "USD"
	Error   *struct {
		Code int    `json:"code"`
		Info string `json:"info"`
	} `json:"error,omitempty"`
}

// FetchPrice returns the price per gram for the given pair (e.g. "XAU/USD").
// metals-api.com endpoint: GET /latest?access_key=...&base={currency}&symbols=XAU
// This gives us XAU expressed as troy ounces per 1 unit of base currency.
// We invert to get currency units per troy ounce, then divide by grams/oz.
func (m *MetalsAPI) FetchPrice(ctx context.Context, pair string) (float64, error) {
	_, currency, err := splitPair(pair)
	if err != nil {
		return 0, fmt.Errorf("metalsapi: %w", err)
	}

	url := fmt.Sprintf(
		"https://metals-api.com/api/latest?access_key=%s&base=%s&symbols=XAU",
		m.apiKey, currency,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("metalsapi: build request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("metalsapi: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("metalsapi: unexpected status %d", resp.StatusCode)
	}

	var body metalsAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, fmt.Errorf("metalsapi: decode: %w", err)
	}
	if !body.Success {
		if body.Error != nil {
			return 0, fmt.Errorf("metalsapi: api error %d: %s", body.Error.Code, body.Error.Info)
		}
		return 0, fmt.Errorf("metalsapi: unsuccessful response")
	}

	// rates["XAU"] = troy ounces of gold per 1 unit of base currency.
	// Invert → currency units per troy ounce → divide by grams/oz → per gram.
	xauRate, ok := body.Rates["XAU"]
	if !ok || xauRate <= 0 {
		return 0, fmt.Errorf("metalsapi: XAU rate not present in response")
	}

	// xauRate is oz_per_currency_unit. Invert to get currency_per_oz, then per gram.
	pricePerGram := (1.0 / xauRate) / troyOzToGrams
	return pricePerGram, nil
}
