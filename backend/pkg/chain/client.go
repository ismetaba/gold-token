// Package chain provides an abstraction over Ethereum RPC and signing.
//
// In production this is backed by go-ethereum + Fireblocks/HSM.
// For the skeleton we expose an interface so services can be wired up
// and tested against a fake.
package chain

import (
	"context"
	"math/big"
)

// Client is the thin Ethereum client surface we depend on.
type Client interface {
	ChainID(ctx context.Context) (*big.Int, error)
	BlockNumber(ctx context.Context) (uint64, error)
	// SendTx submits a signed transaction. In production this goes via Fireblocks.
	SendTx(ctx context.Context, payload SignedTx) (txHash Hash, err error)
	// WaitReceipt blocks until tx is confirmed (or ctx cancelled).
	WaitReceipt(ctx context.Context, txHash Hash) (Receipt, error)
}

// Address is a 20-byte Ethereum address.
type Address [20]byte

// Hash is a 32-byte keccak256 hash.
type Hash [32]byte

// SignedTx is a raw signed transaction payload (RLP-encoded).
type SignedTx []byte

// Receipt captures the outcome of a confirmed transaction.
type Receipt struct {
	TxHash      Hash
	BlockNumber uint64
	Status      uint64 // 1 = success, 0 = reverted
	GasUsed     uint64
	Logs        []Log
}

// Log is a structured event log entry.
type Log struct {
	Address Address
	Topics  []Hash
	Data    []byte
}

// Signer abstracts key custody. In production this calls Fireblocks MPC.
// For local dev: in-process key.
type Signer interface {
	Address() Address
	// Sign builds and signs a transaction to target with calldata.
	Sign(ctx context.Context, to Address, calldata []byte, value *big.Int) (SignedTx, error)
}
