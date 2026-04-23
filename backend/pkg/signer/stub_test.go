package signer_test

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ismetaba/gold-token/backend/pkg/signer"
)

// testKey is a well-known insecure dev key (Hardhat/Anvil account 0).
const testKeyHex = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

func TestStubSigner_RoundTrip(t *testing.T) {
	s, err := signer.NewStubSignerFromHex(testKeyHex)
	if err != nil {
		t.Fatalf("NewStubSignerFromHex: %v", err)
	}

	// Sign a known digest.
	var digest [32]byte
	copy(digest[:], crypto.Keccak256([]byte("hello gold-token")))

	sig, err := s.Sign(context.Background(), digest)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// Recover public key from signature and compare.
	recovered, err := crypto.SigToPub(digest[:], sig[:])
	if err != nil {
		t.Fatalf("SigToPub: %v", err)
	}
	if !pubKeyEqual(recovered, s.PublicKey()) {
		t.Errorf("recovered key does not match signer public key")
	}

	// Verify address derivation.
	expectedAddr := crypto.PubkeyToAddress(*s.PublicKey())
	if s.Address() != [20]byte(expectedAddr) {
		t.Errorf("Address() mismatch: got %x, want %x", s.Address(), expectedAddr)
	}
}

func TestStubSigner_0xPrefix(t *testing.T) {
	// Ensure "0x" prefix is accepted.
	_, err := signer.NewStubSignerFromHex("0x" + testKeyHex)
	if err != nil {
		t.Fatalf("0x-prefixed key rejected: %v", err)
	}
}

func TestStubSigner_InvalidKey(t *testing.T) {
	_, err := signer.NewStubSignerFromHex("notahex")
	if err == nil {
		t.Fatal("expected error for invalid hex, got nil")
	}
}

func TestNewFactory_Stub(t *testing.T) {
	s, err := signer.New(signer.Config{
		Type:          signer.TypeStub,
		PrivateKeyHex: testKeyHex,
	})
	if err != nil {
		t.Fatalf("factory New: %v", err)
	}
	if s == nil {
		t.Fatal("factory returned nil signer")
	}
}

func TestNewFactory_UnknownType(t *testing.T) {
	_, err := signer.New(signer.Config{Type: "fireblocks"})
	if err == nil {
		t.Fatal("expected error for unknown type, got nil")
	}
}

func TestNewFactory_StubMissingKey(t *testing.T) {
	_, err := signer.New(signer.Config{Type: signer.TypeStub})
	if err == nil {
		t.Fatal("expected error when PrivateKeyHex is empty, got nil")
	}
}

func TestConfigFromEnv_DefaultType(t *testing.T) {
	// Unset SIGNER_TYPE → default "stub".
	t.Setenv("SIGNER_TYPE", "")
	cfg := signer.ConfigFromEnv()
	if cfg.Type != signer.TypeStub {
		t.Errorf("default type: got %q, want %q", cfg.Type, signer.TypeStub)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func pubKeyEqual(a, b *ecdsa.PublicKey) bool {
	return a.X.Cmp(b.X) == 0 && a.Y.Cmp(b.Y) == 0
}

// keep hex import used.
var _ = hex.EncodeToString
