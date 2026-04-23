// LocalSigner implements Signer using an in-process ECDSA private key.
//
// Use this for local development and integration tests only.
// Production deployments must use the SoftHSM2 signer (CAPA-18) which
// implements the same Signer interface via a PKCS#11 session.
package chain

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// LocalSigner signs transactions with an in-process ECDSA private key.
// It fetches the pending nonce from an RPC endpoint before signing so that
// each call produces a correctly-sequenced signed transaction.
type LocalSigner struct {
	key     *ecdsa.PrivateKey
	address Address
	chainID *big.Int
	rpc     *ethclient.Client
}

// NewLocalSignerFromHex parses a hex-encoded private key (with or without 0x prefix)
// and returns a LocalSigner connected to rpc for nonce/gas estimation.
func NewLocalSignerFromHex(hexKey string, chainID *big.Int, rpc *ethclient.Client) (*LocalSigner, error) {
	hexKey = strings.TrimPrefix(hexKey, "0x")
	raw, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("local signer: decode private key: %w", err)
	}
	priv, err := crypto.ToECDSA(raw)
	if err != nil {
		return nil, fmt.Errorf("local signer: parse private key: %w", err)
	}
	addr := crypto.PubkeyToAddress(priv.PublicKey)
	return &LocalSigner{
		key:     priv,
		address: Address(addr),
		chainID: new(big.Int).Set(chainID),
		rpc:     rpc,
	}, nil
}

func (s *LocalSigner) Address() Address { return s.address }

// Sign builds a signed EIP-1559 transaction to `to` with `calldata` and `value`.
// The nonce and gas parameters are fetched live from the RPC node.
// Returns an RLP-encoded signed transaction ready for broadcast.
func (s *LocalSigner) Sign(ctx context.Context, to Address, calldata []byte, value *big.Int) (SignedTx, error) {
	from := gethcommon.Address(s.address)
	toAddr := gethcommon.Address(to)

	nonce, err := s.rpc.PendingNonceAt(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("local signer: pending nonce: %w", err)
	}

	head, err := s.rpc.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("local signer: fetch head: %w", err)
	}

	// EIP-1559 gas parameters: tip = 1 gwei, fee cap = baseFee*2 + tip.
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
		return nil, fmt.Errorf("local signer: estimate gas: %w", err)
	}
	// Add 20% headroom to avoid out-of-gas on borderline cases.
	gasLimit = gasLimit * 12 / 10

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

	signer := types.LatestSignerForChainID(s.chainID)
	signed, err := types.SignTx(tx, signer, s.key)
	if err != nil {
		return nil, fmt.Errorf("local signer: sign tx: %w", err)
	}

	raw, err := signed.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("local signer: marshal signed tx: %w", err)
	}
	return SignedTx(raw), nil
}
