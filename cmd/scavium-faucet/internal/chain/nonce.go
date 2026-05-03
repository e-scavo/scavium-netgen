package chain

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// NonceManager tracks the next pending nonce for a single faucet address.
// On the first call (or after Reset) it fetches the pending nonce from the node
// via NonceAt; subsequent calls increment a local counter to avoid repeated RPC
// round-trips.  Safe for concurrent use.
type NonceManager struct {
	mu      sync.Mutex
	client  ChainClient
	address common.Address
	next    uint64
	seeded  bool
}

// NewNonceManager creates a NonceManager for address.  No RPC call is made
// until Next is first called.
func NewNonceManager(client ChainClient, address common.Address) *NonceManager {
	return &NonceManager{client: client, address: address}
}

// Next returns the nonce to use for the next transaction and pre-increments the
// local counter.  On the first call (or after Reset) it fetches the pending
// nonce from the node.
func (nm *NonceManager) Next(ctx context.Context) (uint64, error) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if !nm.seeded {
		nonce, err := nm.client.NonceAt(ctx, nm.address, nil)
		if err != nil {
			return 0, fmt.Errorf("fetch pending nonce: %w", err)
		}
		nm.next = nonce
		nm.seeded = true
	}

	v := nm.next
	nm.next++
	return v, nil
}

// Reset forces the next Next call to re-fetch the pending nonce from the node.
// Call this when SendTransaction fails so that a stale or skipped nonce does
// not block subsequent transactions.
func (nm *NonceManager) Reset() {
	nm.mu.Lock()
	nm.seeded = false
	nm.mu.Unlock()
}
