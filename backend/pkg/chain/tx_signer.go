// Package chain — TxSigner wraps a pkg/signer.Signer (digest-level) into the
// chain.Signer (transaction-level) interface.
//
// This is the production path: the HSM signs only the 32-byte transaction hash
// while all EIP-1559 transaction construction happens in Go.
package chain

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	pkgsigner "github.com/ismetaba/gold-token/backend/pkg/signer"
)

// TxSigner implements chain.Signer by:
//  1. Building an unsigned EIP-1559 transaction (nonce, gas from RPC).
//  2. Hashing the transaction with the appropriate signer.
//  3. Delegating the cryptographic signing to a pkg/signer.Signer.
//  4. Attaching the signature and returning the RLP-encoded signed tx.
//
// Use NewTxSigner to construct one. LocalSigner provides the same functionality
// with an embedded key and is retained for backward compatibility.
type TxSigner struct {
	inner   pkgsigner.Signer
	address Address
	chainID *big.Int
	rpc     *ethclient.Client
}

// NewTxSigner wraps inner and connects to rpc for nonce / gas estimation.
// chainID must match the network chainID; if zero it is fetched from rpc.
func NewTxSigner(inner pkgsigner.Signer, chainID *big.Int, rpc *ethclient.Client) *TxSigner {
	addr := inner.Address()
	return &TxSigner{
		inner:   inner,
		address: addr,
		chainID: new(big.Int).Set(chainID),
		rpc:     rpc,
	}
}

func (s *TxSigner) Address() Address { return s.address }

// Sign builds and signs an EIP-1559 transaction to `to` with `calldata` and
// `value`. Returns an RLP-encoded signed transaction ready for broadcast.
func (s *TxSigner) Sign(ctx context.Context, to Address, calldata []byte, value *big.Int) (SignedTx, error) {
	from := gethcommon.Address(s.address)
	toAddr := gethcommon.Address(to)

	nonce, err := s.rpc.PendingNonceAt(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("tx signer: pending nonce: %w", err)
	}

	head, err := s.rpc.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("tx signer: fetch head: %w", err)
	}

	// EIP-1559 gas: tip = 1 gwei, feeCap = baseFee*2 + tip.
	tip := big.NewInt(1e9) // 1 gwei
	var baseFee *big.Int
	if head.BaseFee != nil {
		baseFee = new(big.Int).Set(head.BaseFee)
	} else {
		baseFee = big.NewInt(1e9) // fallback for Anvil / non-EIP-1559 chains
	}
	feeCap := new(big.Int).Add(new(big.Int).Mul(baseFee, big.NewInt(2)), tip)

	msg := ethereum.CallMsg{
		From:      from,
		To:        &toAddr,
		GasFeeCap: feeCap,
		GasTipCap: tip,
		Value:     value,
		Data:      calldata,
	}
	gasLimit, err := s.rpc.EstimateGas(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("tx signer: estimate gas: %w", err)
	}
	gasLimit = gasLimit * 12 / 10 // +20% headroom

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   s.chainID,
		Nonce:     nonce,
		GasTipCap: tip,
		GasFeeCap: feeCap,
		Gas:       gasLimit,
		To:        &toAddr,
		Value:     value,
		Data:      calldata,
	})

	gethSigner := types.LatestSignerForChainID(s.chainID)
	txHash := gethSigner.Hash(tx) // 32-byte digest to sign

	var digest [32]byte
	copy(digest[:], txHash[:])

	sig65, err := s.inner.Sign(ctx, digest)
	if err != nil {
		return nil, fmt.Errorf("tx signer: digest sign: %w", err)
	}

	signed, err := tx.WithSignature(gethSigner, sig65[:])
	if err != nil {
		return nil, fmt.Errorf("tx signer: attach signature: %w", err)
	}

	raw, err := signed.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("tx signer: marshal signed tx: %w", err)
	}
	return SignedTx(raw), nil
}
