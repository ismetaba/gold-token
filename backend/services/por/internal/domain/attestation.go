// Package domain holds the core PoR business types.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// Attestation is the off-chain representation of a ReserveOracle on-chain record.
type Attestation struct {
	ID            uuid.UUID
	OnChainIdx    *int64  // nil when not yet synced from chain
	TimestampSec  int64   // Attestation.timestamp (block.timestamp unix)
	AsOfSec       int64   // Attestation.asOf (audit reference date unix)
	TotalGramsWei string  // decimal string; 1 gram = 1e18
	MerkleRoot    string  // 0x-prefixed bytes32 hex
	IPFSCid       string  // 0x-prefixed bytes32 hex
	Auditor       string  // 0x-prefixed Ethereum address
	TxHash        string  // on-chain tx hash (empty for read-synced entries)
	RecordedAt    time.Time
}
