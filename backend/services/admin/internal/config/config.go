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
	c := &Config{
		Env:      getenv("GOLD_ENV", "local"),
		HTTPAddr: getenv("GOLD_ADMIN_HTTP_ADDR", ":8094"),

		JWTPrivateKeyFile: os.Getenv("ADMIN_JWT_PRIVATE_KEY_FILE"),
		JWTPublicKeyFile:  os.Getenv("ADMIN_JWT_PUBLIC_KEY_FILE"),

		MasterSecret: getenv("GOLD_ADMIN_SECRET", "local-admin-secret"),

		// Downstream base URLs.
		KYCServiceURL:        getenv("KYC_SERVICE_URL", "http://localhost:8083"),
		AuthServiceURL:       getenv("AUTH_SERVICE_URL", "http://localhost:8082"),
		OrderServiceURL:      getenv("ORDER_SERVICE_URL", "http://localhost:8085"),
		TreasuryServiceURL:   getenv("TREASURY_SERVICE_URL", "http://localhost:8089"),
		VaultServiceURL:      getenv("VAULT_SERVICE_URL", "http://localhost:8091"),
		FeeServiceURL:        getenv("FEE_SERVICE_URL", "http://localhost:8092"),
		AuditServiceURL:      getenv("AUDIT_SERVICE_URL", "http://localhost:8090"),
		ComplianceServiceURL: getenv("COMPLIANCE_SERVICE_URL", "http://localhost:8086"),

		// Downstream admin secrets.
		KYCAdminSecret:        getenv("KYC_ADMIN_SECRET", "local-kyc-secret"),
		AuthAdminSecret:       getenv("AUTH_ADMIN_SECRET", "local-auth-secret"),
		OrderAdminSecret:      getenv("ORDER_ADMIN_SECRET", "local-order-secret"),
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
