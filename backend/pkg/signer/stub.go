package signer

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// StubSigner implements Signer using an in-memory ECDSA private key.
// It is intended for local development and unit / integration tests only.
// Never use in production — the private key is held in process memory.
type StubSigner struct {
	key  *ecdsa.PrivateKey
	pub  *ecdsa.PublicKey
	addr [20]byte
}

// NewStubSignerFromHex parses a hex-encoded secp256k1 private key (with or
// without a leading 0x) and returns a ready-to-use StubSigner.
func NewStubSignerFromHex(hexKey string) (*StubSigner, error) {
	hexKey = strings.TrimPrefix(hexKey, "0x")
	raw, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("stub signer: decode private key: %w", err)
	}
	key, err := crypto.ToECDSA(raw)
	if err != nil {
		return nil, fmt.Errorf("stub signer: parse private key: %w", err)
	}
	pub := &key.PublicKey
	ethAddr := crypto.PubkeyToAddress(*pub)
	return &StubSigner{
		key:  key,
		pub:  pub,
		addr: [20]byte(ethAddr),
	}, nil
}

// Sign signs the digest using the in-memory private key.
// The returned 65-byte signature has the Ethereum {r||s||v} layout (v ∈ {0, 1}).
func (s *StubSigner) Sign(_ context.Context, digest [32]byte) ([65]byte, error) {
	sig, err := crypto.Sign(digest[:], s.key)
	if err != nil {
		return [65]byte{}, fmt.Errorf("stub signer: sign: %w", err)
	}
	var out [65]byte
	copy(out[:], sig) // crypto.Sign already returns 65 bytes {r||s||v}
	return out, nil
}

// PublicKey returns the ECDSA public key.
func (s *StubSigner) PublicKey() *ecdsa.PublicKey { return s.pub }

// Address returns the Ethereum address derived from the public key.
func (s *StubSigner) Address() [20]byte { return s.addr }
