// Package chain wraps the on-chain ComplianceRegistry contract calls.
//
// ComplianceRegistryClient is the service's view of the whitelist contract.
// StubRegistryClient is a no-op implementation for local dev and POC; swap for
// the real abigen-generated binding when the contract is deployed.
package chain

import (
	"context"
	"fmt"
	"sync"
)

// ComplianceRegistryClient is the on-chain whitelist abstraction.
type ComplianceRegistryClient interface {
	// Whitelist records the user address as KYC/sanctions-cleared on-chain.
	Whitelist(ctx context.Context, userAddress string) (txHash string, err error)
	// IsWhitelisted queries the on-chain whitelist status of an address.
	IsWhitelisted(ctx context.Context, userAddress string) (bool, error)
}

// ─── Stub (local dev / POC) ───────────────────────────────────────────────────

// StubRegistryClient keeps state in memory; always succeeds.
type StubRegistryClient struct {
	mu        sync.RWMutex
	whitelist map[string]bool
	nextTx    int
}

func NewStubRegistryClient() *StubRegistryClient {
	return &StubRegistryClient{whitelist: make(map[string]bool)}
}

func (s *StubRegistryClient) Whitelist(_ context.Context, addr string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextTx++
	s.whitelist[addr] = true
	return fmt.Sprintf("0xstub_tx_%06d", s.nextTx), nil
}

func (s *StubRegistryClient) IsWhitelisted(_ context.Context, addr string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.whitelist[addr], nil
}
