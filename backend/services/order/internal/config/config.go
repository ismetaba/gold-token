// Package config loads runtime configuration for orderd.
package config

import (
	"fmt"
	"os"
)

// Config holds runtime configuration for orderd.
type Config struct {
	Env      string // local | dev | staging | prod
	HTTPAddr string

	DatabaseURL string
	NATSURL     string
	NATSStream  string

	// JWT RS256 public key for verifying tokens issued by auth service.
	// In local mode, set to "" and auth is skipped.
	JWTPublicKeyFile string
}

func FromEnv() (*Config, error) {
	c := &Config{
		Env:              getenv("GOLD_ENV", "local"),
		HTTPAddr:         getenv("GOLD_ORDER_HTTP_ADDR", ":8085"),
		NATSStream:       getenv("GOLD_NATS_STREAM", "GOLD"),
		JWTPublicKeyFile: os.Getenv("JWT_PUBLIC_KEY_FILE"),
	}

	c.DatabaseURL = os.Getenv("DATABASE_URL")
	c.NATSURL = os.Getenv("NATS_URL")

	if c.Env != "local" {
		for k, v := range map[string]string{
			"DATABASE_URL":        c.DatabaseURL,
			"NATS_URL":            c.NATSURL,
			"JWT_PUBLIC_KEY_FILE": c.JWTPublicKeyFile,
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
