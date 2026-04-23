// Package chain provides on-chain read/write operations for the PoR service.
//
// OracleReader calls view functions on ReserveOracle via eth_call (no signing).
// OracleWriter calls publish() on ReserveOracle using a TxSigner for gas.
//
// ABI JSON is embedded directly; no abigen codegen step required.
package chain

import (
	"context"
	"encoding/hex"
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
// ABI
// ──────────────────────────────────────────────────────────────────────────────

const reserveOracleABIJSON = `[
  {
    "type": "function",
    "name": "latest",
    "inputs": [],
    "outputs": [{"name":"","type":"tuple","components":[
      {"name":"timestamp",  "type":"uint64"},
      {"name":"asOf",       "type":"uint64"},
      {"name":"totalGrams", "type":"uint256"},
      {"name":"merkleRoot", "type":"bytes32"},
      {"name":"ipfsCid",    "type":"bytes32"},
      {"name":"auditor",    "type":"address"}
    ]}],
    "stateMutability": "view"
  },
  {
    "type": "function",
    "name": "attestationAt",
    "inputs": [{"name":"index","type":"uint256"}],
    "outputs": [{"name":"","type":"tuple","components":[
      {"name":"timestamp",  "type":"uint64"},
      {"name":"asOf",       "type":"uint64"},
      {"name":"totalGrams", "type":"uint256"},
      {"name":"merkleRoot", "type":"bytes32"},
      {"name":"ipfsCid",    "type":"bytes32"},
      {"name":"auditor",    "type":"address"}
    ]}],
    "stateMutability": "view"
  },
  {
    "type": "function",
    "name": "attestationCount",
    "inputs": [],
    "outputs": [{"name":"","type":"uint256"}],
    "stateMutability": "view"
  },
  {
    "type": "function",
    "name": "publish",
    "inputs": [
      {"name":"a","type":"tuple","components":[
        {"name":"timestamp",  "type":"uint64"},
        {"name":"asOf",       "type":"uint64"},
        {"name":"totalGrams", "type":"uint256"},
        {"name":"merkleRoot", "type":"bytes32"},
        {"name":"ipfsCid",    "type":"bytes32"},
        {"name":"auditor",    "type":"address"}
      ]},
      {"name":"signature","type":"bytes"}
    ],
    "outputs": [],
    "stateMutability": "nonpayable"
  }
]`

// ──────────────────────────────────────────────────────────────────────────────
// Go structs mirroring Solidity tuple layout
// ──────────────────────────────────────────────────────────────────────────────

// attestationArgs mirrors IReserveOracle.Attestation for ABI packing.
type attestationArgs struct {
	Timestamp  uint64
	AsOf       uint64
	TotalGrams *big.Int
	MerkleRoot [32]byte
	IPFSCid    [32]byte //nolint:revive
	Auditor    gethcommon.Address
}

// OnChainAttestation is the decoded on-chain Attestation struct.
type OnChainAttestation struct {
	Timestamp  uint64
	AsOf       uint64
	TotalGrams *big.Int
	MerkleRoot [32]byte
	IPFSCid    [32]byte
	Auditor    gethcommon.Address
}

// PublishRequest holds everything needed to call publish() on-chain.
type PublishRequest struct {
	Timestamp   uint64
	AsOf        uint64
	TotalGrams  *big.Int  // wei; 1 gram = 1e18
	MerkleRoot  [32]byte
	IPFSCid     [32]byte
	Auditor     gethcommon.Address
	Signature   []byte    // EIP-712 signature by the auditor key
}

// ──────────────────────────────────────────────────────────────────────────────
// OracleReader — view-only, no signer needed
// ──────────────────────────────────────────────────────────────────────────────

// OracleReader calls view functions on the ReserveOracle contract via eth_call.
type OracleReader struct {
	rpc          *ethclient.Client
	contractABI  abi.ABI
	contractAddr gethcommon.Address
}

// NewOracleReader dials the chain and targets the ReserveOracle contract.
// contractAddr may be empty; reads will return zero-value attestations.
func NewOracleReader(rpcURL, contractAddr string) (*OracleReader, error) {
	rpc, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("oracle reader: dial %s: %w", rpcURL, err)
	}
	parsed, err := abi.JSON(strings.NewReader(reserveOracleABIJSON))
	if err != nil {
		return nil, fmt.Errorf("oracle reader: parse ABI: %w", err)
	}
	var addr gethcommon.Address
	if contractAddr != "" {
		if !gethcommon.IsHexAddress(contractAddr) {
			return nil, fmt.Errorf("oracle reader: invalid contract address %q", contractAddr)
		}
		addr = gethcommon.HexToAddress(contractAddr)
	}
	return &OracleReader{rpc: rpc, contractABI: parsed, contractAddr: addr}, nil
}

// Latest returns the most recent on-chain attestation.
// Returns a zero-value attestation (Timestamp==0) when no attestations exist yet.
func (r *OracleReader) Latest(ctx context.Context) (OnChainAttestation, error) {
	if (r.contractAddr == gethcommon.Address{}) {
		return OnChainAttestation{}, nil
	}
	calldata, err := r.contractABI.Pack("latest")
	if err != nil {
		return OnChainAttestation{}, fmt.Errorf("oracle latest: pack: %w", err)
	}
	out, err := r.call(ctx, calldata)
	if err != nil {
		return OnChainAttestation{}, fmt.Errorf("oracle latest: eth_call: %w", err)
	}
	return r.unpackAttestation("latest", out)
}

// AttestationAt returns the attestation at the given index.
func (r *OracleReader) AttestationAt(ctx context.Context, index uint64) (OnChainAttestation, error) {
	if (r.contractAddr == gethcommon.Address{}) {
		return OnChainAttestation{}, nil
	}
	calldata, err := r.contractABI.Pack("attestationAt", new(big.Int).SetUint64(index))
	if err != nil {
		return OnChainAttestation{}, fmt.Errorf("oracle attestationAt: pack: %w", err)
	}
	out, err := r.call(ctx, calldata)
	if err != nil {
		return OnChainAttestation{}, fmt.Errorf("oracle attestationAt: eth_call: %w", err)
	}
	return r.unpackAttestation("attestationAt", out)
}

// AttestationCount returns the total number of on-chain attestations.
func (r *OracleReader) AttestationCount(ctx context.Context) (uint64, error) {
	if (r.contractAddr == gethcommon.Address{}) {
		return 0, nil
	}
	calldata, err := r.contractABI.Pack("attestationCount")
	if err != nil {
		return 0, fmt.Errorf("oracle attestationCount: pack: %w", err)
	}
	out, err := r.call(ctx, calldata)
	if err != nil {
		return 0, fmt.Errorf("oracle attestationCount: eth_call: %w", err)
	}
	result := make(map[string]interface{})
	if err := r.contractABI.UnpackIntoMap(result, "attestationCount", out); err != nil {
		return 0, fmt.Errorf("oracle attestationCount: unpack: %w", err)
	}
	n, ok := result["0"].(*big.Int)
	if !ok {
		return 0, fmt.Errorf("oracle attestationCount: unexpected type %T", result["0"])
	}
	return n.Uint64(), nil
}

func (r *OracleReader) call(ctx context.Context, calldata []byte) ([]byte, error) {
	msg := ethereum.CallMsg{To: &r.contractAddr, Data: calldata}
	return r.rpc.CallContract(ctx, msg, nil)
}

func (r *OracleReader) unpackAttestation(fn string, out []byte) (OnChainAttestation, error) {
	result := make(map[string]interface{})
	if err := r.contractABI.UnpackIntoMap(result, fn, out); err != nil {
		return OnChainAttestation{}, fmt.Errorf("unpack %s: %w", fn, err)
	}
	raw, ok := result["0"]
	if !ok {
		return OnChainAttestation{}, fmt.Errorf("unpack %s: missing output key 0", fn)
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return OnChainAttestation{}, fmt.Errorf("unpack %s: unexpected type %T", fn, raw)
	}

	var a OnChainAttestation
	if v, ok := m["timestamp"].(uint64); ok {
		a.Timestamp = v
	}
	if v, ok := m["asOf"].(uint64); ok {
		a.AsOf = v
	}
	if v, ok := m["totalGrams"].(*big.Int); ok && v != nil {
		a.TotalGrams = new(big.Int).Set(v)
	} else {
		a.TotalGrams = new(big.Int)
	}
	if v, ok := m["merkleRoot"].([32]byte); ok {
		a.MerkleRoot = v
	}
	if v, ok := m["ipfsCid"].([32]byte); ok {
		a.IPFSCid = v
	}
	if v, ok := m["auditor"].(gethcommon.Address); ok {
		a.Auditor = v
	}
	return a, nil
}

// Close releases the underlying RPC connection.
func (r *OracleReader) Close() { r.rpc.Close() }

// ──────────────────────────────────────────────────────────────────────────────
// OracleWriter — publishes attestations via signed transactions
// ──────────────────────────────────────────────────────────────────────────────

// OracleWriter calls publish() on the ReserveOracle contract.
type OracleWriter struct {
	contractABI  abi.ABI
	contractAddr gethcommon.Address
	signer       pkgchain.Signer
	rpc          *ethclient.Client
	chain        pkgchain.Client
}

// NewOracleWriter wires up the on-chain writer.
func NewOracleWriter(
	contractAddr string,
	signer pkgchain.Signer,
	ethClient *pkgchain.EthClient,
) (*OracleWriter, error) {
	parsed, err := abi.JSON(strings.NewReader(reserveOracleABIJSON))
	if err != nil {
		return nil, fmt.Errorf("oracle writer: parse ABI: %w", err)
	}
	if !gethcommon.IsHexAddress(contractAddr) {
		return nil, fmt.Errorf("oracle writer: invalid contract address %q", contractAddr)
	}
	return &OracleWriter{
		contractABI:  parsed,
		contractAddr: gethcommon.HexToAddress(contractAddr),
		signer:       signer,
		rpc:          ethClient.Inner(),
		chain:        ethClient,
	}, nil
}

// Publish submits a new attestation to the ReserveOracle contract.
// The req.Signature must be a valid EIP-712 signature from req.Auditor.
// Returns the transaction hash.
func (w *OracleWriter) Publish(ctx context.Context, req PublishRequest) (string, error) {
	calldata, err := w.contractABI.Pack("publish",
		attestationArgs{
			Timestamp:  req.Timestamp,
			AsOf:       req.AsOf,
			TotalGrams: req.TotalGrams,
			MerkleRoot: req.MerkleRoot,
			IPFSCid:    req.IPFSCid,
			Auditor:    req.Auditor,
		},
		req.Signature,
	)
	if err != nil {
		return "", fmt.Errorf("publish: abi pack: %w", err)
	}

	to := pkgchain.Address(w.contractAddr)
	raw, err := w.signer.Sign(ctx, to, calldata, new(big.Int))
	if err != nil {
		return "", fmt.Errorf("publish: sign: %w", err)
	}

	txHash, err := w.chain.SendTx(ctx, raw)
	if err != nil {
		return "", fmt.Errorf("publish: send tx: %w", err)
	}

	rec, err := w.chain.WaitReceipt(ctx, txHash)
	if err != nil {
		return "", fmt.Errorf("publish: wait receipt: %w", err)
	}
	if rec.Status == 0 {
		return "", fmt.Errorf("publish: transaction reverted")
	}

	return gethcommon.Hash(txHash).Hex(), nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

// Bytes32ToHex converts a [32]byte to a 0x-prefixed hex string.
func Bytes32ToHex(b [32]byte) string {
	return "0x" + hex.EncodeToString(b[:])
}

// HexToBytes32 parses a 0x-prefixed hex string into [32]byte.
func HexToBytes32(s string) ([32]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	b, err := hex.DecodeString(s)
	if err != nil {
		return [32]byte{}, fmt.Errorf("hex decode: %w", err)
	}
	var out [32]byte
	copy(out[32-len(b):], b)
	return out, nil
}
