// Package domain contains the core types for the wallet service.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// Wallet is an Ethereum address assigned to a user account.
type Wallet struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Address   string    // 0x-prefixed checksummed Ethereum address
	CreatedAt time.Time
}

// Transaction records a mint, burn, or token transfer for a wallet.
type Transaction struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Address    string    // wallet address
	TxHash     string    // on-chain transaction hash
	EventType  string    // "mint" | "burn" | "transfer_in" | "transfer_out"
	AmountWei  string    // decimal string (avoids JSON precision issues with big.Int)
	OccurredAt time.Time
}
