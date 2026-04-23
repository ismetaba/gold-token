// Package chain — EthMintControllerClient and EthBurnControllerClient
// implement MintControllerClient and BurnControllerClient using go-ethereum
// ABI encoding to call the deployed Solidity contracts.
//
// ABI JSON is embedded directly; no abigen code-generation step required.
// The Signer abstraction (pkgchain.Signer) provides key custody:
//   - dev/test: pkgchain.LocalSigner (ECDSA in-process)
//   - production: SoftHSM2 signer (CAPA-18, same interface)
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

	pkgchain "github.com/ismetaba/gold-token/backend/pkg/chain"
)

// ──────────────────────────────────────────────────────────────────────────────
// ABI definitions (minimal subsets of contracts/out/*.json)
// ──────────────────────────────────────────────────────────────────────────────

const mintControllerABIJSON = `[
  {
    "type": "function",
    "name": "proposeMint",
    "inputs": [{"name":"req","type":"tuple","components":[
      {"name":"to",           "type":"address"},
      {"name":"amount",       "type":"uint256"},
      {"name":"allocationId", "type":"bytes32"},
      {"name":"barSerials",   "type":"bytes32[]"},
      {"name":"jurisdiction", "type":"bytes2"},
      {"name":"proposedAt",   "type":"uint64"}
    ]}],
    "outputs": [{"name":"proposalId","type":"bytes32"}],
    "stateMutability": "nonpayable"
  },
  {
    "type": "function",
    "name": "executeMint",
    "inputs": [{"name":"proposalId","type":"bytes32"}],
    "outputs": [],
    "stateMutability": "nonpayable"
  },
  {
    "type": "function",
    "name": "getProposal",
    "inputs": [{"name":"proposalId","type":"bytes32"}],
    "outputs": [{"name":"","type":"tuple","components":[
      {"name":"req","type":"tuple","components":[
        {"name":"to",           "type":"address"},
        {"name":"amount",       "type":"uint256"},
        {"name":"allocationId", "type":"bytes32"},
        {"name":"barSerials",   "type":"bytes32[]"},
        {"name":"jurisdiction", "type":"bytes2"},
        {"name":"proposedAt",   "type":"uint64"}
      ]},
      {"name":"status",    "type":"uint8"},
      {"name":"proposer",  "type":"address"},
      {"name":"approvers", "type":"address[]"}
    ]}],
    "stateMutability": "view"
  }
]`

const burnControllerABIJSON = `[
  {
    "type": "function",
    "name": "requestRedemption",
    "inputs": [{"name":"req","type":"tuple","components":[
      {"name":"from",            "type":"address"},
      {"name":"amount",          "type":"uint256"},
      {"name":"redemptionType",  "type":"uint8"},
      {"name":"offChainOrderId", "type":"bytes32"},
      {"name":"deliveryRef",     "type":"string"}
    ]}],
    "outputs": [{"name":"reqId","type":"bytes32"}],
    "stateMutability": "nonpayable"
  }
]`

// ──────────────────────────────────────────────────────────────────────────────
// Go structs that mirror Solidity tuple layouts for ABI packing.
// Field names must match the ABI component names (case-insensitive).
// ──────────────────────────────────────────────────────────────────────────────

// mintReqArgs matches IMintController.MintRequest.
type mintReqArgs struct {
	To           gethcommon.Address
	Amount       *big.Int
	AllocationId [32]byte   //nolint:revive // must match ABI name
	BarSerials   [][32]byte
	Jurisdiction [2]byte
	ProposedAt   uint64
}

// redemptionReqArgs matches IBurnController.RedemptionRequest.
type redemptionReqArgs struct {
	From            gethcommon.Address
	Amount          *big.Int
	RedemptionType  uint8
	OffChainOrderId [32]byte //nolint:revive // must match ABI name
	DeliveryRef     string
}

// ──────────────────────────────────────────────────────────────────────────────
// EthMintControllerClient
// ──────────────────────────────────────────────────────────────────────────────

// EthMintControllerClient is the production MintControllerClient.
type EthMintControllerClient struct {
	contractABI  abi.ABI
	contractAddr gethcommon.Address
	signer       pkgchain.Signer
	rpc          *ethclient.Client
	chain        pkgchain.Client // for WaitReceipt / SendTx
}

// NewEthMintControllerClient wires up the real on-chain mint client.
//
//   - contractAddr: deployed MintController proxy address (0x…)
//   - signer: key custody — LocalSigner for dev, HSM for prod (CAPA-18)
//   - ethClient: an *EthClient that implements both Client and exposes Inner()
func NewEthMintControllerClient(
	contractAddr string,
	signer pkgchain.Signer,
	ethClient *pkgchain.EthClient,
) (*EthMintControllerClient, error) {
	parsed, err := abi.JSON(strings.NewReader(mintControllerABIJSON))
	if err != nil {
		return nil, fmt.Errorf("mint controller: parse ABI: %w", err)
	}
	if !gethcommon.IsHexAddress(contractAddr) {
		return nil, fmt.Errorf("mint controller: invalid contract address %q", contractAddr)
	}
	return &EthMintControllerClient{
		contractABI:  parsed,
		contractAddr: gethcommon.HexToAddress(contractAddr),
		signer:       signer,
		rpc:          ethClient.Inner(),
		chain:        ethClient,
	}, nil
}

// ProposeMint calls proposeMint on-chain and returns the tx hash.
func (c *EthMintControllerClient) ProposeMint(ctx context.Context, req MintRequest) (string, error) {
	calldata, err := c.contractABI.Pack("proposeMint", mintReqArgs{
		To:           gethcommon.Address(req.To),
		Amount:       req.Amount,
		AllocationId: req.AllocationID,
		BarSerials:   req.BarSerials,
		Jurisdiction: req.Jurisdiction,
		ProposedAt:   req.ProposedAt,
	})
	if err != nil {
		return "", fmt.Errorf("proposeMint: abi pack: %w", err)
	}

	txHash, err := c.signBroadcastWait(ctx, calldata)
	if err != nil {
		return "", fmt.Errorf("proposeMint: %w", err)
	}
	return txHash, nil
}

// ExecuteMint calls executeMint(proposalId) on-chain.
func (c *EthMintControllerClient) ExecuteMint(ctx context.Context, proposalID [32]byte) (string, error) {
	calldata, err := c.contractABI.Pack("executeMint", proposalID)
	if err != nil {
		return "", fmt.Errorf("executeMint: abi pack: %w", err)
	}

	txHash, err := c.signBroadcastWait(ctx, calldata)
	if err != nil {
		return "", fmt.Errorf("executeMint: %w", err)
	}
	return txHash, nil
}

// ProposalStatus reads the on-chain proposal status via eth_call.
func (c *EthMintControllerClient) ProposalStatus(ctx context.Context, proposalID [32]byte) (ProposalStatus, error) {
	p, err := c.fetchProposal(ctx, proposalID)
	if err != nil {
		return ProposalNone, err
	}
	return ProposalStatus(p.status), nil
}

// ApprovalCount returns how many distinct approvers have signed on-chain.
func (c *EthMintControllerClient) ApprovalCount(ctx context.Context, proposalID [32]byte) (uint8, error) {
	p, err := c.fetchProposal(ctx, proposalID)
	if err != nil {
		return 0, err
	}
	n := len(p.approvers)
	if n > 255 {
		n = 255
	}
	return uint8(n), nil
}

// signBroadcastWait: sign → send → wait for receipt, returns hex tx hash.
func (c *EthMintControllerClient) signBroadcastWait(ctx context.Context, calldata []byte) (string, error) {
	to := pkgchain.Address(c.contractAddr)
	raw, err := c.signer.Sign(ctx, to, calldata, new(big.Int))
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}

	txHash, err := c.chain.SendTx(ctx, raw)
	if err != nil {
		return "", fmt.Errorf("send tx: %w", err)
	}

	rec, err := c.chain.WaitReceipt(ctx, txHash)
	if err != nil {
		return "", fmt.Errorf("wait receipt: %w", err)
	}
	if rec.Status == 0 {
		return "", ErrTxReverted
	}
	return gethcommon.Hash(txHash).Hex(), nil
}

// proposalData holds the decoded output of getProposal.
type proposalData struct {
	status    uint8
	approvers []gethcommon.Address
}

// fetchProposal calls getProposal via eth_call and decodes the result.
// go-ethereum UnpackIntoMap is used to avoid hard-coded anonymous struct casts.
func (c *EthMintControllerClient) fetchProposal(ctx context.Context, proposalID [32]byte) (*proposalData, error) {
	calldata, err := c.contractABI.Pack("getProposal", proposalID)
	if err != nil {
		return nil, fmt.Errorf("getProposal: pack: %w", err)
	}

	toAddr := c.contractAddr
	msg := ethereum.CallMsg{
		From: gethcommon.Address(c.signer.Address()),
		To:   &toAddr,
		Data: calldata,
	}
	out, err := c.rpc.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("getProposal: eth_call: %w", err)
	}

	results := make(map[string]interface{})
	if err := c.contractABI.UnpackIntoMap(results, "getProposal", out); err != nil {
		return nil, fmt.Errorf("getProposal: unpack: %w", err)
	}

	// The unnamed output maps to key "0" in UnpackIntoMap.
	raw, ok := results["0"]
	if !ok {
		return nil, ErrProposalNotFound
	}

	// go-ethereum decodes named tuple components into map[string]interface{}.
	proposal, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("getProposal: unexpected result type %T", raw)
	}

	status, _ := proposal["status"].(uint8)

	var approvers []gethcommon.Address
	if raw, ok := proposal["approvers"]; ok {
		approvers, _ = raw.([]gethcommon.Address)
	}

	return &proposalData{status: status, approvers: approvers}, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// EthBurnControllerClient
// ──────────────────────────────────────────────────────────────────────────────

// EthBurnControllerClient is the production BurnControllerClient.
type EthBurnControllerClient struct {
	contractABI  abi.ABI
	contractAddr gethcommon.Address
	signer       pkgchain.Signer
	chain        pkgchain.Client
}

// NewEthBurnControllerClient wires up the real on-chain burn client.
func NewEthBurnControllerClient(
	contractAddr string,
	signer pkgchain.Signer,
	ethClient *pkgchain.EthClient,
) (*EthBurnControllerClient, error) {
	parsed, err := abi.JSON(strings.NewReader(burnControllerABIJSON))
	if err != nil {
		return nil, fmt.Errorf("burn controller: parse ABI: %w", err)
	}
	if !gethcommon.IsHexAddress(contractAddr) {
		return nil, fmt.Errorf("burn controller: invalid contract address %q", contractAddr)
	}
	return &EthBurnControllerClient{
		contractABI:  parsed,
		contractAddr: gethcommon.HexToAddress(contractAddr),
		signer:       signer,
		chain:        ethClient,
	}, nil
}

// RequestRedemption calls requestRedemption on-chain.
func (c *EthBurnControllerClient) RequestRedemption(ctx context.Context, req RedemptionRequest) (string, error) {
	calldata, err := c.contractABI.Pack("requestRedemption", redemptionReqArgs{
		From:            gethcommon.Address(req.From),
		Amount:          req.Amount,
		RedemptionType:  req.RedemptionType,
		OffChainOrderId: req.OffChainOrderID,
		DeliveryRef:     req.DeliveryRef,
	})
	if err != nil {
		return "", fmt.Errorf("requestRedemption: abi pack: %w", err)
	}

	to := pkgchain.Address(c.contractAddr)
	raw, err := c.signer.Sign(ctx, to, calldata, new(big.Int))
	if err != nil {
		return "", fmt.Errorf("requestRedemption: sign: %w", err)
	}

	txHash, err := c.chain.SendTx(ctx, raw)
	if err != nil {
		return "", fmt.Errorf("requestRedemption: send tx: %w", err)
	}

	rec, err := c.chain.WaitReceipt(ctx, txHash)
	if err != nil {
		return "", fmt.Errorf("requestRedemption: wait receipt: %w", err)
	}
	if rec.Status == 0 {
		return "", ErrTxReverted
	}
	return gethcommon.Hash(txHash).Hex(), nil
}
