package config

import (
	"fmt"
	"os"
)

type Config struct {
	Env      string
	HTTPAddr string

	DatabaseURL string

	// Admin JWT key pair — if both are empty an ephemeral key is used (local dev).
	JWTPrivateKeyFile string
	JWTPublicKeyFile  string

	// Master bootstrap secret (used only for POST /admin/bootstrap).
	MasterSecret string

	// Downstream service base URLs.
	KYCServiceURL        string
	AuthServiceURL       string
	OrderServiceURL      string
	TreasuryServiceURL   string
	VaultServiceURL      string
	FeeServiceURL        string
	AuditServiceURL      string
	ComplianceServiceURL string

	// Secrets forwarded to each downstream service as X-Admin-Secret.
	KYCAdminSecret        string
	TreasuryAdminSecret   string
	VaultAdminSecret      string
	FeeAdminSecret        string
	AuditAdminSecret      string
	ComplianceAdminSecret string
	OrderAdminSecret      string
	AuthAdminSecret       string
}

func FromEnv() (*Config, error) {
	env := getenv("GOLD_ENV", "local")
	c := &Config{
		Env:      env,
		HTTPAddr: getenv("GOLD_ADMIN_HTTP_ADDR", ":8094"),

		JWTPrivateKeyFile: os.Getenv("ADMIN_JWT_PRIVATE_KEY_FILE"),
		JWTPublicKeyFile:  os.Getenv("ADMIN_JWT_PUBLIC_KEY_FILE"),

		MasterSecret: os.Getenv("GOLD_ADMIN_SECRET"),

		// Downstream base URLs (non-secret; localhost defaults are fine for local dev).
		KYCServiceURL:        getenv("KYC_SERVICE_URL", "http://localhost:8083"),
		AuthServiceURL:       getenv("AUTH_SERVICE_URL", "http://localhost:8082"),
		OrderServiceURL:      getenv("ORDER_SERVICE_URL", "http://localhost:8085"),
		TreasuryServiceURL:   getenv("TREASURY_SERVICE_URL", "http://localhost:8089"),
		VaultServiceURL:      getenv("VAULT_SERVICE_URL", "http://localhost:8091"),
		FeeServiceURL:        getenv("FEE_SERVICE_URL", "http://localhost:8092"),
		AuditServiceURL:      getenv("AUDIT_SERVICE_URL", "http://localhost:8090"),
		ComplianceServiceURL: getenv("COMPLIANCE_SERVICE_URL", "http://localhost:8086"),

		// Downstream admin secrets — read raw; validated / defaulted below.
		KYCAdminSecret:        os.Getenv("KYC_ADMIN_SECRET"),
		AuthAdminSecret:       os.Getenv("AUTH_ADMIN_SECRET"),
		OrderAdminSecret:      os.Getenv("ORDER_ADMIN_SECRET"),
		TreasuryAdminSecret:   os.Getenv("TREASURY_ADMIN_SECRET"),
		VaultAdminSecret:      os.Getenv("VAULT_ADMIN_SECRET"),
		FeeAdminSecret:        os.Getenv("FEE_ADMIN_SECRET"),
		AuditAdminSecret:      os.Getenv("AUDIT_ADMIN_SECRET"),
		ComplianceAdminSecret: os.Getenv("COMPLIANCE_ADMIN_SECRET"),
	}

	c.DatabaseURL = os.Getenv("DATABASE_URL")

	// Secrets that MUST be supplied via env in any non-local environment. Hardcoded
	// fallbacks here would be a catastrophic auth bypass (the master bootstrap secret
	// grants full admin control), so we fail closed instead.
	secretsByEnv := map[string]string{
		"GOLD_ADMIN_SECRET":       c.MasterSecret,
		"KYC_ADMIN_SECRET":        c.KYCAdminSecret,
		"AUTH_ADMIN_SECRET":       c.AuthAdminSecret,
		"ORDER_ADMIN_SECRET":      c.OrderAdminSecret,
		"TREASURY_ADMIN_SECRET":   c.TreasuryAdminSecret,
		"VAULT_ADMIN_SECRET":      c.VaultAdminSecret,
		"FEE_ADMIN_SECRET":        c.FeeAdminSecret,
		"AUDIT_ADMIN_SECRET":      c.AuditAdminSecret,
		"COMPLIANCE_ADMIN_SECRET": c.ComplianceAdminSecret,
	}

	if c.Env != "local" {
		if c.DatabaseURL == "" {
			return nil, fmt.Errorf("missing required env: DATABASE_URL")
		}
		for k, v := range secretsByEnv {
			if v == "" {
				return nil, fmt.Errorf("missing required env: %s", k)
			}
		}
	} else {
		// Local dev only: fall back to stable, clearly-non-production placeholders so
		// the whole stack still comes up with zero configuration.
		localDefaults := map[string]*string{
			"local-admin-secret":      &c.MasterSecret,
			"local-kyc-secret":        &c.KYCAdminSecret,
			"local-auth-secret":       &c.AuthAdminSecret,
			"local-order-secret":      &c.OrderAdminSecret,
			"local-treasury-secret":   &c.TreasuryAdminSecret,
			"local-vault-secret":      &c.VaultAdminSecret,
			"local-fee-secret":        &c.FeeAdminSecret,
			"local-audit-secret":      &c.AuditAdminSecret,
			"local-compliance-secret": &c.ComplianceAdminSecret,
		}
		for def, ptr := range localDefaults {
			if *ptr == "" {
				*ptr = def
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
