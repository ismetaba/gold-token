package config

import (
	"fmt"
	"os"
)

// Config holds all runtime configuration for authd.
type Config struct {
	Env        string // local | dev | staging | prod
	HTTPAddr   string
	DatabaseURL string
	NATSURL    string
	NATSStream string

	// JWT RS256: exactly one of the pair must be set in non-local envs.
	// In local mode, a temporary RSA key is generated at startup if unset.
	JWTPrivateKeyFile string // path to PEM-encoded PKCS8 RSA private key
	JWTPublicKeyFile  string // path to PEM-encoded PKIX RSA public key

	// Access token TTL (seconds). Default 900 (15 min).
	AccessTokenTTL int
	// Refresh token TTL (seconds). Default 604800 (7 days).
	RefreshTokenTTL int
}

// FromEnv loads config from environment variables.
func FromEnv() (*Config, error) {
	c := &Config{
		Env:               getenv("GOLD_ENV", "local"),
		HTTPAddr:          getenv("GOLD_AUTH_HTTP_ADDR", ":8082"),
		NATSStream:        getenv("GOLD_NATS_STREAM", "GOLD"),
		JWTPrivateKeyFile: os.Getenv("JWT_PRIVATE_KEY_FILE"),
		JWTPublicKeyFile:  os.Getenv("JWT_PUBLIC_KEY_FILE"),
		AccessTokenTTL:    getenvInt("JWT_ACCESS_TTL_SECONDS", 900),
		RefreshTokenTTL:   getenvInt("JWT_REFRESH_TTL_SECONDS", 604800),
	}

	c.DatabaseURL = os.Getenv("DATABASE_URL")
	c.NATSURL = os.Getenv("NATS_URL")

	if c.Env != "local" {
		for k, v := range map[string]string{
			"DATABASE_URL":         c.DatabaseURL,
			"NATS_URL":             c.NATSURL,
			"JWT_PRIVATE_KEY_FILE": c.JWTPrivateKeyFile,
			"JWT_PUBLIC_KEY_FILE":  c.JWTPublicKeyFile,
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

func getenvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}
