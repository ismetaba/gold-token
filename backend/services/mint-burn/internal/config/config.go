package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config, mintburnd servisinin tüm runtime ayarları.
type Config struct {
	Env        string // local | dev | staging | prod
	HTTPAddr   string
	DatabaseURL string
	NATSURL    string
	NATSStream string

	ChainRPCURL  string
	ChainID      int64
	MintCtrlAddr string // 0x… MintController proxy address
	BurnCtrlAddr string // 0x… BurnController proxy address

	// Signer selection — see pkg/signer.
	SignerType string // "stub" (default) or "softhsm"

	// Stub signer (SIGNER_TYPE=stub).
	SignerPrivateKey string // hex ECDSA key; required when SignerType == "stub"

	// SoftHSM / PKCS#11 signer (SIGNER_TYPE=softhsm).
	SoftHSMLib        string // path to libsofthsm2.so
	SoftHSMTokenLabel string
	SoftHSMPin        string
	SoftHSMKeyLabel   string

	ApprovalTimeout  time.Duration
	StepPollInterval time.Duration
	MaxAttempts      int
}

// FromEnv, ortam değişkenlerinden config yükler. Zorunlu alanlar eksikse hata.
func FromEnv() (*Config, error) {
	c := &Config{
		Env:              getenv("GOLD_ENV", "local"),
		HTTPAddr:         getenv("GOLD_HTTP_ADDR", ":8081"),
		NATSStream:       getenv("GOLD_NATS_STREAM", "GOLD"),
		ApprovalTimeout:  getenvDuration("GOLD_APPROVAL_TIMEOUT", 4*time.Hour),
		StepPollInterval: getenvDuration("GOLD_STEP_POLL_INTERVAL", 2*time.Second),
		MaxAttempts:      getenvInt("GOLD_MAX_ATTEMPTS", 5),
	}

	c.DatabaseURL = os.Getenv("DATABASE_URL")
	c.NATSURL = os.Getenv("NATS_URL")
	c.ChainRPCURL = os.Getenv("CHAIN_RPC_URL")
	c.MintCtrlAddr = os.Getenv("MINT_CONTROLLER_ADDR")
	c.BurnCtrlAddr = os.Getenv("BURN_CONTROLLER_ADDR")
	c.ChainID = int64(getenvInt("CHAIN_ID", 1))

	// Signer
	c.SignerType = getenv("SIGNER_TYPE", "stub")
	c.SignerPrivateKey = os.Getenv("SIGNER_PRIVATE_KEY")
	c.SoftHSMLib = os.Getenv("SOFTHSM2_LIB")
	c.SoftHSMTokenLabel = os.Getenv("SOFTHSM2_TOKEN_LABEL")
	c.SoftHSMPin = os.Getenv("SOFTHSM2_PIN")
	c.SoftHSMKeyLabel = os.Getenv("SOFTHSM2_KEY_LABEL")

	if c.Env != "local" {
		for k, v := range map[string]string{
			"DATABASE_URL":         c.DatabaseURL,
			"NATS_URL":             c.NATSURL,
			"CHAIN_RPC_URL":        c.ChainRPCURL,
			"MINT_CONTROLLER_ADDR": c.MintCtrlAddr,
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
