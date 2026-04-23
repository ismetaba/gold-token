// Package chain provides on-chain read operations for the wallet service.
// It uses go-ethereum's eth_call to read ERC-20 balances without signing.
package chain

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const erc20ABI = `[
  {
    "name": "balanceOf",
    "type": "function",
    "inputs": [{"name": "account", "type": "address"}],
    "outputs": [{"name": "", "type": "uint256"}],
    "stateMutability": "view"
  }
]`

// BalanceReader reads ERC-20 token balances via eth_call.
type BalanceReader struct {
	rpc           *ethclient.Client
	tokenABI      abi.ABI
	tokenAddr     gethcommon.Address
}

// NewBalanceReader dials the given RPC URL and targets the given ERC-20 contract.
// tokenAddr may be empty (zero address) when no chain is available (local mode).
func NewBalanceReader(rpcURL, tokenAddr string) (*BalanceReader, error) {
	rpc, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("balance reader: dial %s: %w", rpcURL, err)
	}
	parsed, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return nil, fmt.Errorf("balance reader: parse ABI: %w", err)
	}
	var addr gethcommon.Address
	if tokenAddr != "" {
		if !gethcommon.IsHexAddress(tokenAddr) {
			return nil, fmt.Errorf("balance reader: invalid token address %q", tokenAddr)
		}
		addr = gethcommon.HexToAddress(tokenAddr)
	}
	return &BalanceReader{rpc: rpc, tokenABI: parsed, tokenAddr: addr}, nil
}

// BalanceOf returns the ERC-20 balance for the given address (0x-prefixed hex).
// Returns zero when the token contract address is not configured (local mode).
func (r *BalanceReader) BalanceOf(ctx context.Context, holder string) (*big.Int, error) {
	if (r.tokenAddr == gethcommon.Address{}) {
		return big.NewInt(0), nil
	}
	if !gethcommon.IsHexAddress(holder) {
		return nil, fmt.Errorf("balance reader: invalid address %q", holder)
	}
	holderAddr := gethcommon.HexToAddress(holder)

	calldata, err := r.tokenABI.Pack("balanceOf", holderAddr)
	if err != nil {
		return nil, fmt.Errorf("balance reader: pack: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &r.tokenAddr,
		Data: calldata,
	}
	out, err := r.rpc.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("balance reader: eth_call: %w", err)
	}

	result := make(map[string]interface{})
	if err := r.tokenABI.UnpackIntoMap(result, "balanceOf", out); err != nil {
		return nil, fmt.Errorf("balance reader: unpack: %w", err)
	}

	bal, ok := result["0"].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("balance reader: unexpected result type %T", result["0"])
	}
	return bal, nil
}

// Close closes the underlying RPC connection.
func (r *BalanceReader) Close() { r.rpc.Close() }
