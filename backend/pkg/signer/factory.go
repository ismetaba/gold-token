package signer

import (
	"fmt"
	"os"
	"strings"
)

// Type identifies which Signer implementation to instantiate.
type Type string

const (
	// TypeStub selects StubSigner: an in-memory ECDSA key for dev / tests.
	TypeStub Type = "stub"
	// TypeSoftHSM selects SoftHSMSigner: PKCS#11 via SoftHSM2 or a real HSM.
	TypeSoftHSM Type = "softhsm"
)

// Config holds all parameters for any Signer implementation.
// Unused fields for a given Type are ignored.
type Config struct {
	Type Type

	// ── Stub fields ──────────────────────────────────────────────────────────
	// PrivateKeyHex is a hex-encoded secp256k1 private key (with or without 0x).
	PrivateKeyHex string

	// ── SoftHSM / PKCS#11 fields ─────────────────────────────────────────────
	// LibPath is the filesystem path to the PKCS#11 shared library.
	// Defaults to the value of SOFTHSM2_LIB env var if empty.
	LibPath string
	// TokenLabel is the label of the PKCS#11 token slot.
	TokenLabel string
	// PIN is the user PIN for the token.
	PIN string
	// KeyLabel is the CKA_LABEL of the private-key object to use.
	KeyLabel string
}

// ConfigFromEnv builds a Config from well-known environment variables:
//
//	SIGNER_TYPE          — "stub" (default) or "softhsm"
//	SIGNER_PRIVATE_KEY   — hex private key for stub signer
//	SOFTHSM2_LIB         — path to libsofthsm2.so
//	SOFTHSM2_TOKEN_LABEL — token label
//	SOFTHSM2_PIN         — user PIN
//	SOFTHSM2_KEY_LABEL   — key label
func ConfigFromEnv() Config {
	return Config{
		Type:          Type(strings.ToLower(getenvOr("SIGNER_TYPE", string(TypeStub)))),
		PrivateKeyHex: os.Getenv("SIGNER_PRIVATE_KEY"),
		LibPath:       getenvOr("SOFTHSM2_LIB", defaultSoftHSMLib()),
		TokenLabel:    os.Getenv("SOFTHSM2_TOKEN_LABEL"),
		PIN:           os.Getenv("SOFTHSM2_PIN"),
		KeyLabel:      os.Getenv("SOFTHSM2_KEY_LABEL"),
	}
}

// New creates a Signer according to cfg.Type.
// For TypeSoftHSM the caller is responsible for calling Close() when done.
func New(cfg Config) (Signer, error) {
	switch cfg.Type {
	case TypeStub:
		if cfg.PrivateKeyHex == "" {
			return nil, fmt.Errorf("signer: SIGNER_PRIVATE_KEY must be set for stub signer")
		}
		return NewStubSignerFromHex(cfg.PrivateKeyHex)

	case TypeSoftHSM:
		hsmCfg := SoftHSMConfig{
			LibPath:    cfg.LibPath,
			TokenLabel: cfg.TokenLabel,
			PIN:        cfg.PIN,
			KeyLabel:   cfg.KeyLabel,
		}
		if hsmCfg.LibPath == "" {
			return nil, fmt.Errorf("signer: SOFTHSM2_LIB must be set for softhsm signer")
		}
		if hsmCfg.TokenLabel == "" {
			return nil, fmt.Errorf("signer: SOFTHSM2_TOKEN_LABEL must be set for softhsm signer")
		}
		if hsmCfg.KeyLabel == "" {
			return nil, fmt.Errorf("signer: SOFTHSM2_KEY_LABEL must be set for softhsm signer")
		}
		return NewSoftHSMSigner(hsmCfg)

	default:
		return nil, fmt.Errorf("signer: unknown type %q (valid: stub, softhsm)", cfg.Type)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func getenvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// defaultSoftHSMLib returns the most common install path for libsofthsm2.so
// based on the host OS. Returns empty string if neither path exists.
func defaultSoftHSMLib() string {
	candidates := []string{
		"/usr/lib/softhsm/libsofthsm2.so",         // Debian/Ubuntu
		"/usr/local/lib/softhsm/libsofthsm2.so",   // macOS brew
		"/usr/lib64/pkcs11/libsofthsm2.so",         // RHEL/Fedora
		"/usr/lib/x86_64-linux-gnu/softhsm/libsofthsm2.so",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
