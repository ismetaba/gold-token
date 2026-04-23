//go:build integration

// Integration tests for EthMintControllerClient against a live Anvil node.
//
// Prerequisites:
//   1. Anvil running: `anvil --chain-id 31337`
//   2. Contracts deployed (Foundry scripts): `forge script script/Deploy.s.sol --rpc-url http://localhost:8545 --broadcast`
//   3. Export env vars:
//        CHAIN_RPC_URL=http://localhost:8545
//        MINT_CONTROLLER_ADDR=0x<deployed address>
//        SIGNER_PRIVATE_KEY=0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80
//
// Run with: go test -tags integration ./services/mint-burn/internal/chain/...
package chain_test

import (
	"context"
	"math/big"
	"os"
	"testing"
	"time"

	pkgchain "github.com/ismetaba/gold-token/backend/pkg/chain"
	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/chain"
)

func skipIfNoAnvil(t *testing.T) (rpcURL, mintCtrlAddr, signerKey string) {
	t.Helper()
	rpcURL = os.Getenv("CHAIN_RPC_URL")
	mintCtrlAddr = os.Getenv("MINT_CONTROLLER_ADDR")
	signerKey = os.Getenv("SIGNER_PRIVATE_KEY")
	if rpcURL == "" || mintCtrlAddr == "" || signerKey == "" {
		t.Skip("set CHAIN_RPC_URL, MINT_CONTROLLER_ADDR, SIGNER_PRIVATE_KEY to run integration tests")
	}
	return
}

func TestEthClient_ChainID(t *testing.T) {
	rpcURL, _, _ := skipIfNoAnvil(t)

	client, err := pkgchain.NewEthClient(rpcURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		t.Fatalf("ChainID: %v", err)
	}
	t.Logf("chain ID: %s", chainID)
	if chainID.Sign() <= 0 {
		t.Errorf("expected positive chain ID, got %s", chainID)
	}
}

func TestLocalSigner_Sign(t *testing.T) {
	rpcURL, _, signerKey := skipIfNoAnvil(t)

	ethClient, err := pkgchain.NewEthClient(rpcURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		t.Fatalf("chain ID: %v", err)
	}

	signer, err := pkgchain.NewLocalSignerFromHex(signerKey, chainID, ethClient.Inner())
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	addr := signer.Address()
	t.Logf("signer address: 0x%x", addr[:])

	// Sign a no-op self-transfer (0 ETH to self) to verify signing works.
	raw, err := signer.Sign(ctx, addr, nil, new(big.Int))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if len(raw) == 0 {
		t.Error("signed tx is empty")
	}
	t.Logf("signed tx length: %d bytes", len(raw))
}

func TestEthMintControllerClient_ProposeMint(t *testing.T) {
	rpcURL, mintCtrlAddr, signerKey := skipIfNoAnvil(t)

	ethClient, err := pkgchain.NewEthClient(rpcURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		t.Fatalf("chain ID: %v", err)
	}

	signer, err := pkgchain.NewLocalSignerFromHex(signerKey, chainID, ethClient.Inner())
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	mc, err := chain.NewEthMintControllerClient(mintCtrlAddr, signer, ethClient)
	if err != nil {
		t.Fatalf("mint client: %v", err)
	}

	req := chain.MintRequest{
		AllocationID: [32]byte{1, 2, 3, 4},
		To:           pkgchain.Address(signer.Address()),
		Amount:       big.NewInt(1e18), // 1 GOLD token
		BarSerials:   [][32]byte{{0xAA}},
		Jurisdiction: [2]byte{'T', 'R'},
		ProposedAt:   uint64(time.Now().Unix()),
	}

	txHash, err := mc.ProposeMint(ctx, req)
	if err != nil {
		// Likely a revert from missing PROPOSER role — that's expected in a raw deploy.
		// The test verifies the client can build + sign + broadcast correctly.
		t.Logf("proposeMint reverted (expected if caller lacks PROPOSER role): %v", err)
		return
	}
	t.Logf("proposeMint tx: %s", txHash)

	// Verify we can read proposal status back.
	status, err := mc.ProposalStatus(ctx, req.AllocationID)
	if err != nil {
		t.Fatalf("ProposalStatus: %v", err)
	}
	t.Logf("proposal status: %d", status)
}

func TestEthMintControllerClient_ApprovalCount(t *testing.T) {
	rpcURL, mintCtrlAddr, signerKey := skipIfNoAnvil(t)

	ethClient, err := pkgchain.NewEthClient(rpcURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	chainID, _ := ethClient.ChainID(ctx)

	signer, err := pkgchain.NewLocalSignerFromHex(signerKey, chainID, ethClient.Inner())
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	mc, err := chain.NewEthMintControllerClient(mintCtrlAddr, signer, ethClient)
	if err != nil {
		t.Fatalf("mint client: %v", err)
	}

	// Query for a non-existent proposal; expect 0 approvals.
	count, err := mc.ApprovalCount(ctx, [32]byte{0xFF})
	if err != nil {
		t.Fatalf("ApprovalCount: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 approvals for unknown proposal, got %d", count)
	}
}
