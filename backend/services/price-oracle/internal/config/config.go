package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for priceoracle.
type Config struct {
	Env      string // local | dev | staging | prod
	HTTPAddr string

	DatabaseURL string
	NATSURL     string
	NATSStream  string

	// PriceAPIKey is the goldapi.io access token.
	// If empty (local mode), a stub provider is used instead.
	PriceAPIKey string

	// RefreshInterval controls how often the oracle polls the price feed.
	// Default: 60s.
	RefreshInterval time.Duration
}

// FromEnv loads configuration from environment variables.
func FromEnv() (*Config, error) {
	c := &Config{
		Env:             getenv("GOLD_ENV", "local"),
		HTTPAddr:        getenv("GOLD_PRICE_HTTP_ADDR", ":8083"),
		NATSStream:      getenv("GOLD_NATS_STREAM", "GOLD"),
		PriceAPIKey:     os.Getenv("GOLD_PRICE_API_KEY"),
		RefreshInterval: getenvDuration("GOLD_PRICE_REFRESH_INTERVAL", 60*time.Second),
	}

	c.DatabaseURL = os.Getenv("DATABASE_URL")
	c.NATSURL = os.Getenv("NATS_URL")

	if c.Env != "local" {
		for k, v := range map[string]string{
			"DATABASE_URL":       c.DatabaseURL,
			"NATS_URL":           c.NATSURL,
			"GOLD_PRICE_API_KEY": c.PriceAPIKey,
		} {
			if v == "" {
				return nil, fmt.Errorf("missing required env: %s", k)
			}
		}
	}
	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getenvDuration(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getenvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// suppress unused lint warning; getenvInt may be used by future config fields.
var _ = getenvInt
