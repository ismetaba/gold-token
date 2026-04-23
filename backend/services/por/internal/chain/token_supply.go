// Package chain — TokenSupplyReader reads the total supply of the GOLD ERC-20.
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

const erc20TotalSupplyABI = `[
  {
    "name": "totalSupply",
    "type": "function",
    "inputs": [],
    "outputs": [{"name":"","type":"uint256"}],
    "stateMutability": "view"
  }
]`

// TokenSupplyReader reads the total supply of an ERC-20 token via eth_call.
type TokenSupplyReader struct {
	rpc       *ethclient.Client
	tokenABI  abi.ABI
	tokenAddr gethcommon.Address
}

// NewTokenSupplyReader dials the given RPC URL and targets the given ERC-20 contract.
func NewTokenSupplyReader(rpcURL, tokenAddr string) (*TokenSupplyReader, error) {
	rpc, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("token supply reader: dial %s: %w", rpcURL, err)
	}
	parsed, err := abi.JSON(strings.NewReader(erc20TotalSupplyABI))
	if err != nil {
		return nil, fmt.Errorf("token supply reader: parse ABI: %w", err)
	}
	var addr gethcommon.Address
	if tokenAddr != "" {
		if !gethcommon.IsHexAddress(tokenAddr) {
			return nil, fmt.Errorf("token supply reader: invalid token address %q", tokenAddr)
		}
		addr = gethcommon.HexToAddress(tokenAddr)
	}
	return &TokenSupplyReader{rpc: rpc, tokenABI: parsed, tokenAddr: addr}, nil
}

// TotalSupply returns the total supply of the ERC-20 token in wei.
func (r *TokenSupplyReader) TotalSupply(ctx context.Context) (*big.Int, error) {
	if (r.tokenAddr == gethcommon.Address{}) {
		return big.NewInt(0), nil
	}
	calldata, err := r.tokenABI.Pack("totalSupply")
	if err != nil {
		return nil, fmt.Errorf("total supply: pack: %w", err)
	}

	msg := ethereum.CallMsg{To: &r.tokenAddr, Data: calldata}
	out, err := r.rpc.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("total supply: eth_call: %w", err)
	}

	result := make(map[string]interface{})
	if err := r.tokenABI.UnpackIntoMap(result, "totalSupply", out); err != nil {
		return nil, fmt.Errorf("total supply: unpack: %w", err)
	}

	v, ok := result["0"].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("total supply: unexpected type %T", result["0"])
	}
	return v, nil
}

// Close releases the underlying RPC connection.
func (r *TokenSupplyReader) Close() { r.rpc.Close() }
