package accounts

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdkCore "github.com/multiversx/mx-sdk-go/core"

	"github.com/mangonui/mx-chain-txgen-go/config"
	"github.com/mangonui/mx-chain-txgen-go/shards"
)

// newSmallPool spins up a pool of n accounts for tests that care only
// about iteration, not about generated key material.
func newSmallPool(t *testing.T, n int) *Pool {
	t.Helper()
	cfg := config.AccountsConfig{
		PoolSize:          n,
		StateDir:          t.TempDir(),
		RegenerateOnStart: true,
		InitialBalance:    "1",
	}
	sc, err := shards.NewCoordinator(2)
	if err != nil {
		t.Fatalf("shard coordinator: %v", err)
	}
	pool, err := NewPool(cfg, sc)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	return pool
}

func TestSyncNonces_SerialFallback(t *testing.T) {
	pool := newSmallPool(t, 5)
	var (
		mu     sync.Mutex
		called []string
	)
	syncFn := func(_ context.Context, _ sdkCore.AddressHandler, bech32 string) error {
		mu.Lock()
		called = append(called, bech32)
		mu.Unlock()
		return nil
	}
	if err := pool.SyncNonces(context.Background(), syncFn, 0); err != nil {
		t.Fatalf("SyncNonces: %v", err)
	}
	if len(called) != 5 {
		t.Fatalf("calls: got %d, want 5", len(called))
	}
}

func TestSyncNonces_RespectsConcurrencyLimit(t *testing.T) {
	const n = 50
	const limit = 5
	pool := newSmallPool(t, n)

	var (
		inFlight   atomic.Int32
		maxObserved atomic.Int32
	)
	syncFn := func(_ context.Context, _ sdkCore.AddressHandler, _ string) error {
		now := inFlight.Add(1)
		// Track the high-water mark by CAS.
		for {
			peak := maxObserved.Load()
			if now <= peak || maxObserved.CompareAndSwap(peak, now) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond) // hold the slot long enough to observe
		inFlight.Add(-1)
		return nil
	}
	if err := pool.SyncNonces(context.Background(), syncFn, limit); err != nil {
		t.Fatalf("SyncNonces: %v", err)
	}
	peak := maxObserved.Load()
	if peak > limit {
		t.Fatalf("concurrency exceeded: peak=%d limit=%d", peak, limit)
	}
	// Sanity: the limit should be reached on a pool larger than the limit.
	if peak < 2 {
		t.Fatalf("expected some parallelism, peak=%d", peak)
	}
}

func TestSyncNonces_ReturnsFirstError(t *testing.T) {
	pool := newSmallPool(t, 10)
	target := pool.Index(3).Bech32
	syncFn := func(_ context.Context, _ sdkCore.AddressHandler, bech32 string) error {
		if bech32 == target {
			return errors.New("simulated proxy failure")
		}
		return nil
	}
	err := pool.SyncNonces(context.Background(), syncFn, 4)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !contains(err.Error(), "simulated proxy failure") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(err.Error(), target) {
		t.Fatalf("error should mention the failing address; got %v", err)
	}
}

func TestSyncNonces_EmptyPoolIsNoOp(t *testing.T) {
	// Construct an empty pool by hand (NewPool refuses size <= 0 via
	// the config validator at a higher layer; this exercises the
	// internal early-return).
	p := &Pool{}
	calls := 0
	syncFn := func(_ context.Context, _ sdkCore.AddressHandler, _ string) error {
		calls++
		return nil
	}
	if err := p.SyncNonces(context.Background(), syncFn, 4); err != nil {
		t.Fatalf("empty pool: %v", err)
	}
	if calls != 0 {
		t.Fatalf("syncFn called %d times on empty pool", calls)
	}
}

func TestSyncNonces_CancelledContextStopsScheduling(t *testing.T) {
	pool := newSmallPool(t, 100)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel — no iteration should start fresh work

	calls := atomic.Int32{}
	syncFn := func(_ context.Context, _ sdkCore.AddressHandler, _ string) error {
		calls.Add(1)
		return nil
	}
	// With pre-cancelled ctx the loop hits the ctx.Done() branch on the
	// first iteration before any goroutine is spawned.
	err := pool.SyncNonces(ctx, syncFn, 4)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls.Load() > 0 {
		t.Fatalf("syncFn called %d times despite cancelled ctx", calls.Load())
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
