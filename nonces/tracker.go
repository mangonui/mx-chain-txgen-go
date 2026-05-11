package nonces

import (
	"context"
	"fmt"
	"sync"

	sdkCore "github.com/multiversx/mx-sdk-go/core"

	"github.com/mangonui/mx-chain-txgen-go/proxy"
)

// Tracker is the txgen's in-memory per-address nonce manager.
//
// At boot, Sync(addr) queries the proxy once and seeds the local map. After
// that, Next(addr) returns the current value and increments it locally so
// the high-throughput signing path never round-trips the proxy. When a
// caller requests recallNonce=true on a batch, Refresh(addr) re-syncs from
// the proxy and overwrites the local value — useful when local state has
// drifted (e.g. an external tx submitter on the same account).
type Tracker struct {
	mu       sync.Mutex
	nonces   map[string]uint64
	prx      *proxy.Client
}

// New returns a fresh tracker backed by the given proxy client.
func New(prx *proxy.Client) *Tracker {
	return &Tracker{
		nonces: make(map[string]uint64),
		prx:    prx,
	}
}

// Sync seeds the tracker with the on-chain nonce for the address. Safe to
// call concurrently for different addresses.
func (t *Tracker) Sync(ctx context.Context, addr sdkCore.AddressHandler, bech32 string) error {
	acc, err := t.prx.SDK.GetAccount(ctx, addr)
	if err != nil {
		return fmt.Errorf("sync nonce for %s: %w", bech32, err)
	}
	t.mu.Lock()
	t.nonces[bech32] = acc.Nonce
	t.mu.Unlock()
	return nil
}

// Refresh forces an on-chain re-query for the address and overwrites the
// local value. Called when a request sets recallNonce=true.
func (t *Tracker) Refresh(ctx context.Context, addr sdkCore.AddressHandler, bech32 string) error {
	return t.Sync(ctx, addr, bech32)
}

// Next returns the next nonce to use for the address and increments the
// in-memory value. Returns the value that should be assigned to the
// transaction's Nonce field.
func (t *Tracker) Next(bech32 string) uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := t.nonces[bech32]
	t.nonces[bech32] = n + 1
	return n
}

// Peek returns the current value without advancing it. Useful for logging
// or diagnostics.
func (t *Tracker) Peek(bech32 string) uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.nonces[bech32]
}

// Set overrides the value for an address. Primarily for tests; production
// callers should prefer Sync/Refresh.
func (t *Tracker) Set(bech32 string, n uint64) {
	t.mu.Lock()
	t.nonces[bech32] = n
	t.mu.Unlock()
}
