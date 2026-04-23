// Package signer provides a low-level key-custody abstraction for Ethereum signing.
//
// This package operates at the digest level: the caller provides a 32-byte hash
// (e.g. an EIP-1559 transaction hash) and receives a 65-byte Ethereum-compact
// signature {r[32] || s[32] || v[1]}.
//
// Implementations:
//   - StubSigner    — in-memory ECDSA key (dev / unit tests)
//   - SoftHSMSigner — PKCS#11 via SoftHSM2 (staging) or any PKCS#11 HSM (prod)
//
// The transaction-building layer (pkg/chain.TxSigner) wraps this interface so
// that the upper services are shielded from key custody details.
//
// Fireblocks and AWS CloudHSM can be added as further implementations of Signer
// without touching service code.
package signer

import (
	"context"
	"crypto/ecdsa"
)

// Signer abstracts key custody at the digest level.
// Implementations MUST be safe for concurrent use.
type Signer interface {
	// Sign returns a 65-byte Ethereum-compact signature for the given 32-byte
	// digest. The layout is {r[0:32] || s[32:64] || v[64]} where v ∈ {0, 1}.
	Sign(ctx context.Context, digest [32]byte) ([65]byte, error)

	// PublicKey returns the ECDSA public key that corresponds to the signing key.
	PublicKey() *ecdsa.PublicKey

	// Address returns the 20-byte Ethereum address derived from PublicKey.
	Address() [20]byte
}
