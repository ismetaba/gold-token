// Package config loads KYC service runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
)

// Config holds all runtime configuration for kycd.
type Config struct {
	Env      string // local | dev | staging | prod
	HTTPAddr string

	DatabaseURL string
	NATSURL     string

	// JWT RS256 public key for verifying tokens issued by the auth service.
	// In local mode the KYC service accepts tokens verified with an ephemeral key
	// shared via JWT_PUBLIC_KEY_FILE (or falls back to no-op verification).
	JWTPublicKeyFile string

	// StorageDir is the root directory for uploaded documents (POC: local FS).
	StorageDir string

	// AdminSecret is checked against the X-Admin-Secret header for review endpoints.
	AdminSecret string
}

// FromEnv loads config from environment variables.
func FromEnv() (*Config, error) {
	c := &Config{
		Env:              getenv("GOLD_ENV", "local"),
		HTTPAddr:         getenv("GOLD_KYC_HTTP_ADDR", ":8083"),
		JWTPublicKeyFile: os.Getenv("JWT_PUBLIC_KEY_FILE"),
		StorageDir:       getenv("KYC_STORAGE_DIR", "/tmp/gold-kyc-docs"),
		AdminSecret:      getenv("KYC_ADMIN_SECRET", "dev-admin-secret"),
	}

	c.DatabaseURL = os.Getenv("DATABASE_URL")
	c.NATSURL = os.Getenv("NATS_URL")

	if c.Env != "local" {
		for k, v := range map[string]string{
			"DATABASE_URL":       c.DatabaseURL,
			"NATS_URL":           c.NATSURL,
			"JWT_PUBLIC_KEY_FILE": c.JWTPublicKeyFile,
			"KYC_ADMIN_SECRET":   c.AdminSecret,
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
