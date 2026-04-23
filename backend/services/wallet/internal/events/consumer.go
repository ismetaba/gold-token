// Package events subscribes to NATS events and populates the transaction log.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/services/wallet/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/wallet/internal/repo"
)

// Consumer subscribes to mint/burn executed events and records them in the
// transaction log so users can see their token history.
type Consumer struct {
	bus     *pkgevents.Bus
	wallets repo.WalletRepo
	txs     repo.TxRepo
	log     *zap.Logger
	stream  string
}

func NewConsumer(bus *pkgevents.Bus, wallets repo.WalletRepo, txs repo.TxRepo, log *zap.Logger, stream string) *Consumer {
	return &Consumer{bus: bus, wallets: wallets, txs: txs, log: log, stream: stream}
}

// Start registers all NATS consumers. Non-blocking — messages are processed
// by internal goroutines managed by the NATS JetStream library.
func (c *Consumer) Start(ctx context.Context) error {
	if err := c.bus.Subscribe(
		ctx, c.stream, "wallet_mint_executed", pkgevents.SubjMintExecuted,
		c.handleMintExecuted,
	); err != nil {
		return fmt.Errorf("subscribe mint executed: %w", err)
	}

	if err := c.bus.Subscribe(
		ctx, c.stream, "wallet_burn_executed", pkgevents.SubjBurnExecuted,
		c.handleBurnExecuted,
	); err != nil {
		return fmt.Errorf("subscribe burn executed: %w", err)
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// mint.executed.v1
// ─────────────────────────────────────────────────────────────────────────────

type mintExecutedPayload struct {
	SagaID       string `json:"saga_id"`
	OrderID      string `json:"order_id"`
	AmountWei    string `json:"amount_wei"`
	TxHash       string `json:"tx_hash"`
	AllocationID string `json:"allocation_id"`
	ToAddress    string `json:"to_address"` // 0x-prefixed recipient
}

func (c *Consumer) handleMintExecuted(ctx context.Context, data []byte) error {
	var env pkgevents.Envelope[mintExecutedPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("unmarshal mint executed: %w", err)
	}

	toAddr := env.Data.ToAddress
	if toAddr == "" {
		c.log.Warn("mint executed event missing to_address — skipping tx log",
			zap.String("saga_id", env.Data.SagaID))
		return nil
	}

	wallet, err := c.wallets.ByAddress(ctx, toAddr)
	if err != nil {
		// Wallet not registered in this service yet — skip gracefully.
		c.log.Debug("wallet not found for mint recipient",
			zap.String("address", toAddr),
			zap.String("saga_id", env.Data.SagaID))
		return nil
	}

	tx := domain.Transaction{
		ID:         uuid.Must(uuid.NewV7()),
		UserID:     wallet.UserID,
		Address:    wallet.Address,
		TxHash:     env.Data.TxHash,
		EventType:  "mint",
		AmountWei:  env.Data.AmountWei,
		OccurredAt: env.OccurredAt,
	}
	if tx.OccurredAt.IsZero() {
		tx.OccurredAt = time.Now().UTC()
	}

	if err := c.txs.Create(ctx, tx); err != nil {
		return fmt.Errorf("record mint tx: %w", err)
	}

	c.log.Info("mint transaction recorded",
		zap.String("user_id", wallet.UserID.String()),
		zap.String("tx_hash", tx.TxHash),
		zap.String("amount_wei", tx.AmountWei),
	)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// burn.executed.v1
// ─────────────────────────────────────────────────────────────────────────────

type burnExecutedPayload struct {
	SagaID      string `json:"saga_id"`
	OrderID     string `json:"order_id"`
	AmountWei   string `json:"amount_wei"`
	TxHash      string `json:"tx_hash"`
	FromAddress string `json:"from_address"` // 0x-prefixed burner
}

func (c *Consumer) handleBurnExecuted(ctx context.Context, data []byte) error {
	var env pkgevents.Envelope[burnExecutedPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("unmarshal burn executed: %w", err)
	}

	fromAddr := env.Data.FromAddress
	if fromAddr == "" {
		c.log.Warn("burn executed event missing from_address — skipping tx log",
			zap.String("saga_id", env.Data.SagaID))
		return nil
	}

	wallet, err := c.wallets.ByAddress(ctx, fromAddr)
	if err != nil {
		c.log.Debug("wallet not found for burn sender",
			zap.String("address", fromAddr),
			zap.String("saga_id", env.Data.SagaID))
		return nil
	}

	tx := domain.Transaction{
		ID:         uuid.Must(uuid.NewV7()),
		UserID:     wallet.UserID,
		Address:    wallet.Address,
		TxHash:     env.Data.TxHash,
		EventType:  "burn",
		AmountWei:  env.Data.AmountWei,
		OccurredAt: env.OccurredAt,
	}
	if tx.OccurredAt.IsZero() {
		tx.OccurredAt = time.Now().UTC()
	}

	if err := c.txs.Create(ctx, tx); err != nil {
		return fmt.Errorf("record burn tx: %w", err)
	}

	c.log.Info("burn transaction recorded",
		zap.String("user_id", wallet.UserID.String()),
		zap.String("tx_hash", tx.TxHash),
		zap.String("amount_wei", tx.AmountWei),
	)
	return nil
}
