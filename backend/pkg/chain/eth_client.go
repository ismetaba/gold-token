// Package chain — EthClient wraps go-ethereum's ethclient.Client.
// It is the production implementation of the Client interface; LocalSigner
// (local_signer.go) is the dev/test Signer implementation.
package chain

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const defaultPollInterval = 500 * time.Millisecond

// EthClient implements Client using a live go-ethereum RPC connection.
type EthClient struct {
	inner        *ethclient.Client
	pollInterval time.Duration
}

// NewEthClient dials the given RPC URL and returns a ready Client.
func NewEthClient(rpcURL string) (*EthClient, error) {
	inner, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("chain: dial %s: %w", rpcURL, err)
	}
	return &EthClient{inner: inner, pollInterval: defaultPollInterval}, nil
}

// Inner returns the underlying go-ethereum client.
// Used by LocalSigner and EthMintControllerClient for nonce/gas fetching.
func (c *EthClient) Inner() *ethclient.Client { return c.inner }

func (c *EthClient) ChainID(ctx context.Context) (*big.Int, error) {
	return c.inner.ChainID(ctx)
}

func (c *EthClient) BlockNumber(ctx context.Context) (uint64, error) {
	return c.inner.BlockNumber(ctx)
}

// SendTx decodes a raw RLP-encoded signed transaction and submits it.
func (c *EthClient) SendTx(ctx context.Context, payload SignedTx) (Hash, error) {
	tx := new(gethtypes.Transaction)
	if err := tx.UnmarshalBinary(payload); err != nil {
		return Hash{}, fmt.Errorf("chain: unmarshal signed tx: %w", err)
	}
	if err := c.inner.SendTransaction(ctx, tx); err != nil {
		return Hash{}, fmt.Errorf("chain: send tx: %w", err)
	}
	return Hash(tx.Hash()), nil
}

// WaitReceipt polls until the tx is mined or ctx is cancelled.
func (c *EthClient) WaitReceipt(ctx context.Context, txHash Hash) (Receipt, error) {
	h := gethcommon.Hash(txHash)
	for {
		r, err := c.inner.TransactionReceipt(ctx, h)
		if err == ethereum.NotFound {
			select {
			case <-ctx.Done():
				return Receipt{}, ctx.Err()
			case <-time.After(c.pollInterval):
				continue
			}
		}
		if err != nil {
			return Receipt{}, fmt.Errorf("chain: receipt for %s: %w", h.Hex(), err)
		}
		return mapReceipt(r), nil
	}
}

func mapReceipt(r *gethtypes.Receipt) Receipt {
	rec := Receipt{
		TxHash:      Hash(r.TxHash),
		BlockNumber: r.BlockNumber.Uint64(),
		Status:      r.Status,
		GasUsed:     r.GasUsed,
	}
	for _, l := range r.Logs {
		entry := Log{Address: Address(l.Address)}
		for _, t := range l.Topics {
			entry.Topics = append(entry.Topics, Hash(t))
		}
		entry.Data = l.Data
		rec.Logs = append(rec.Logs, entry)
	}
	return rec
}
