// Package config loads runtime configuration for complianced.
package config

import (
	"fmt"
	"os"
)

// Config holds runtime configuration for complianced.
type Config struct {
	Env      string // local | dev | staging | prod
	HTTPAddr string

	DatabaseURL string
	NATSURL     string
	NATSStream  string

	// SanctionsListFile is an optional path to a custom JSON sanctions list.
	// When empty, the embedded default list is used.
	SanctionsListFile string
}

func FromEnv() (*Config, error) {
	c := &Config{
		Env:               getenv("GOLD_ENV", "local"),
		HTTPAddr:          getenv("GOLD_COMPLIANCE_HTTP_ADDR", ":8086"),
		NATSStream:        getenv("GOLD_NATS_STREAM", "GOLD"),
		SanctionsListFile: os.Getenv("SANCTIONS_LIST_FILE"),
	}

	c.DatabaseURL = os.Getenv("DATABASE_URL")
	c.NATSURL = os.Getenv("NATS_URL")

	if c.Env != "local" {
		for k, v := range map[string]string{
			"DATABASE_URL": c.DatabaseURL,
			"NATS_URL":     c.NATSURL,
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
