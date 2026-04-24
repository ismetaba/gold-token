// Package config loads Vault Integration service configuration from environment variables.
package config

import (
	"fmt"
	"os"
)

// Config holds all runtime configuration for vaultd.
type Config struct {
	Env         string
	HTTPAddr    string
	DatabaseURL string
	NATSURL     string
	NATSStream  string
	AdminSecret string
}

// FromEnv loads config from environment variables.
func FromEnv() (*Config, error) {
	c := &Config{
		Env:         getenv("GOLD_ENV", "local"),
		HTTPAddr:    getenv("GOLD_VAULT_HTTP_ADDR", ":8091"),
		NATSStream:  getenv("GOLD_NATS_STREAM", "GOLD"),
		AdminSecret: os.Getenv("VAULT_ADMIN_SECRET"),
	}

	c.DatabaseURL = os.Getenv("DATABASE_URL")
	c.NATSURL = os.Getenv("NATS_URL")

	if c.Env != "local" {
		for k, v := range map[string]string{
			"DATABASE_URL":       c.DatabaseURL,
			"NATS_URL":           c.NATSURL,
			"VAULT_ADMIN_SECRET": c.AdminSecret,
		} {
			if v == "" {
				return nil, fmt.Errorf("missing required env: %s", k)
			}
		}
	} else if c.AdminSecret == "" {
		c.AdminSecret = "local-vault-secret"
	}

	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
