package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds runtime configuration for walletd.
type Config struct {
	Env      string // local | dev | staging | prod
	HTTPAddr string

	DatabaseURL string
	NATSURL     string
	NATSStream  string

	// Chain
	ChainRPCURL    string
	GoldTokenAddr  string // 0x… ERC-20 GOLD token contract address

	// JWT RS256 public key for verifying tokens issued by auth service.
	// In local mode, set to "" and auth is skipped.
	JWTPublicKeyFile string
}

func FromEnv() (*Config, error) {
	c := &Config{
		Env:              getenv("GOLD_ENV", "local"),
		HTTPAddr:         getenv("GOLD_WALLET_HTTP_ADDR", ":8084"),
		NATSStream:       getenv("GOLD_NATS_STREAM", "GOLD"),
		JWTPublicKeyFile: os.Getenv("JWT_PUBLIC_KEY_FILE"),
		GoldTokenAddr:    os.Getenv("GOLD_TOKEN_ADDR"),
	}

	c.DatabaseURL = os.Getenv("DATABASE_URL")
	c.NATSURL = os.Getenv("NATS_URL")
	c.ChainRPCURL = os.Getenv("CHAIN_RPC_URL")

	if c.Env != "local" {
		for k, v := range map[string]string{
			"DATABASE_URL":         c.DatabaseURL,
			"NATS_URL":             c.NATSURL,
			"CHAIN_RPC_URL":        c.ChainRPCURL,
			"GOLD_TOKEN_ADDR":      c.GoldTokenAddr,
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
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// suppress unused warning — may be used in future config fields
var _ = getenvInt
