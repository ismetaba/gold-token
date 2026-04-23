// Package config loads runtime configuration for the PoR service.
package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds runtime configuration for pord.
type Config struct {
	Env      string // local | dev | staging | prod
	HTTPAddr string

	DatabaseURL string
	NATSURL     string
	NATSStream  string

	// Chain
	ChainRPCURL         string
	ReserveOracleAddr   string // 0x… ReserveOracle contract address
	GoldTokenAddr       string // 0x… GoldToken ERC-20 address (for totalSupply)

	// Signer — used only for POST /por/attest (admin write path).
	// In local mode without CHAIN_RPC_URL, the write path is skipped.
	SignerType        string // stub | softhsm
	SignerPrivateKey  string // hex-encoded (stub mode)
	ChainID           int64

	// SoftHSM (staging/prod)
	SoftHSMLib        string
	SoftHSMTokenLabel string
	SoftHSMPin        string
	SoftHSMKeyLabel   string

	// Admin token for POST /por/attest.
	// Empty = admin endpoints disabled. In local dev a static token is used.
	AdminToken string

	// SyncInterval controls how often the service polls the chain for new
	// attestations to backfill the DB log.
	SyncInterval time.Duration
}

func FromEnv() (*Config, error) {
	c := &Config{
		Env:               getenv("GOLD_ENV", "local"),
		HTTPAddr:          getenv("GOLD_POR_HTTP_ADDR", ":8085"),
		NATSStream:        getenv("GOLD_NATS_STREAM", "GOLD"),
		ReserveOracleAddr: os.Getenv("RESERVE_ORACLE_ADDR"),
		GoldTokenAddr:     os.Getenv("GOLD_TOKEN_ADDR"),
		SignerType:        getenv("SIGNER_TYPE", "stub"),
		SignerPrivateKey:  os.Getenv("SIGNER_PRIVATE_KEY"),
		SoftHSMLib:        os.Getenv("SOFTHSM2_LIB"),
		SoftHSMTokenLabel: os.Getenv("SOFTHSM2_TOKEN_LABEL"),
		SoftHSMPin:        os.Getenv("SOFTHSM2_PIN"),
		SoftHSMKeyLabel:   os.Getenv("SOFTHSM2_KEY_LABEL"),
		AdminToken:        getenv("POR_ADMIN_TOKEN", "dev-admin-token"),
		SyncInterval:      getenvDuration("POR_SYNC_INTERVAL", 60*time.Second),
	}

	c.DatabaseURL = os.Getenv("DATABASE_URL")
	c.NATSURL = os.Getenv("NATS_URL")
	c.ChainRPCURL = os.Getenv("CHAIN_RPC_URL")

	if c.Env != "local" {
		for k, v := range map[string]string{
			"DATABASE_URL":        c.DatabaseURL,
			"NATS_URL":            c.NATSURL,
			"CHAIN_RPC_URL":       c.ChainRPCURL,
			"RESERVE_ORACLE_ADDR": c.ReserveOracleAddr,
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
