// Package config loads Treasury service runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
)

// Config holds all runtime configuration for treasuryd.
type Config struct {
	Env         string // local | dev | staging | prod
	HTTPAddr    string
	DatabaseURL string
	NATSURL     string
	NATSStream  string

	// AdminSecret is checked against the X-Admin-Secret header for all treasury endpoints.
	AdminSecret string
}

// FromEnv loads config from environment variables.
func FromEnv() (*Config, error) {
	c := &Config{
		Env:        getenv("GOLD_ENV", "local"),
		HTTPAddr:   getenv("GOLD_TREASURY_HTTP_ADDR", ":8089"),
		NATSStream: getenv("GOLD_NATS_STREAM", "GOLD"),
		AdminSecret: os.Getenv("TREASURY_ADMIN_SECRET"),
	}

	c.DatabaseURL = os.Getenv("DATABASE_URL")
	c.NATSURL = os.Getenv("NATS_URL")

	if c.Env != "local" {
		for k, v := range map[string]string{
			"DATABASE_URL":            c.DatabaseURL,
			"NATS_URL":                c.NATSURL,
			"TREASURY_ADMIN_SECRET":   c.AdminSecret,
		} {
			if v == "" {
				return nil, fmt.Errorf("missing required env: %s", k)
			}
		}
	} else if c.AdminSecret == "" {
		// Use a predictable default only in local dev so curl commands work easily.
		c.AdminSecret = "local-treasury-secret"
	}

	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
