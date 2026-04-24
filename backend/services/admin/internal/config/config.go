package config

import (
	"fmt"
	"os"
)

type Config struct {
	Env                string
	HTTPAddr           string
	DatabaseURL        string
	JWTPrivateKeyFile  string
	JWTPublicKeyFile   string

	// Downstream service admin secrets.
	KYCAdminSecret        string
	TreasuryAdminSecret   string
	VaultAdminSecret      string
	FeeAdminSecret        string
	AuditAdminSecret      string
	ComplianceAdminSecret string
}

func FromEnv() (*Config, error) {
	c := &Config{
		Env:                   getenv("GOLD_ENV", "local"),
		HTTPAddr:              getenv("GOLD_ADMIN_HTTP_ADDR", ":8094"),
		JWTPrivateKeyFile:     os.Getenv("ADMIN_JWT_PRIVATE_KEY_FILE"),
		JWTPublicKeyFile:      os.Getenv("ADMIN_JWT_PUBLIC_KEY_FILE"),
		KYCAdminSecret:        getenv("GOLD_ADMIN_SECRET", "local-admin-secret"),
		TreasuryAdminSecret:   getenv("TREASURY_ADMIN_SECRET", "local-treasury-secret"),
		VaultAdminSecret:      getenv("VAULT_ADMIN_SECRET", "local-vault-secret"),
		FeeAdminSecret:        getenv("FEE_ADMIN_SECRET", "local-fee-secret"),
		AuditAdminSecret:      getenv("AUDIT_ADMIN_SECRET", "local-audit-secret"),
		ComplianceAdminSecret: getenv("COMPLIANCE_ADMIN_SECRET", "local-compliance-secret"),
	}
	c.DatabaseURL = os.Getenv("DATABASE_URL")

	if c.Env != "local" {
		if c.DatabaseURL == "" {
			return nil, fmt.Errorf("missing required env: DATABASE_URL")
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
