package accounts

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mangonui/mx-chain-txgen-go/config"
	"github.com/mangonui/mx-chain-txgen-go/shards"
)

func mustCoordinator(t *testing.T, n uint32) *shards.Coordinator {
	t.Helper()
	sc, err := shards.NewCoordinator(n)
	if err != nil {
		t.Fatalf("shard coordinator: %v", err)
	}
	return sc
}

func TestPool_Generate_AddressesAreValidBech32(t *testing.T) {
	dir := t.TempDir()
	cfg := config.AccountsConfig{
		PoolSize:          5,
		StateDir:          dir,
		RegenerateOnStart: true,
		InitialBalance:    "1000",
	}
	pool, err := NewPool(cfg, mustCoordinator(t, 2))
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	if pool.Len() != 5 {
		t.Fatalf("len: got %d, want 5", pool.Len())
	}
	for i, acc := range pool.All() {
		if !strings.HasPrefix(acc.Bech32, "erd1") {
			t.Fatalf("account %d bech32 missing erd1 prefix: %q", i, acc.Bech32)
		}
		if len(acc.PublicKey) != 32 {
			t.Fatalf("account %d pubkey len: got %d, want 32", i, len(acc.PublicKey))
		}
		if len(acc.PrivateKey) == 0 {
			t.Fatalf("account %d has empty private key", i)
		}
	}
}

func TestPool_RoundTripPersistence(t *testing.T) {
	dir := t.TempDir()
	cfg := config.AccountsConfig{
		PoolSize:          3,
		StateDir:          dir,
		RegenerateOnStart: true,
		InitialBalance:    "1",
	}
	sc := mustCoordinator(t, 2)

	first, err := NewPool(cfg, sc)
	if err != nil {
		t.Fatalf("first pool: %v", err)
	}

	// Second open with RegenerateOnStart=false should load the same keys.
	cfg.RegenerateOnStart = false
	second, err := NewPool(cfg, sc)
	if err != nil {
		t.Fatalf("second pool: %v", err)
	}
	if second.Len() != first.Len() {
		t.Fatalf("size mismatch: first=%d second=%d", first.Len(), second.Len())
	}
	for i := 0; i < first.Len(); i++ {
		if first.Index(i).Bech32 != second.Index(i).Bech32 {
			t.Fatalf("account %d bech32 drift: first=%s second=%s",
				i, first.Index(i).Bech32, second.Index(i).Bech32)
		}
	}
}

func TestPool_ShardBucketsArePopulated(t *testing.T) {
	dir := t.TempDir()
	cfg := config.AccountsConfig{
		PoolSize:          100,
		StateDir:          dir,
		RegenerateOnStart: true,
		InitialBalance:    "1",
	}
	pool, err := NewPool(cfg, mustCoordinator(t, 2))
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	// With 2 shards and 100 random accounts, both buckets should be
	// populated. If either bucket is empty something is wrong with the
	// shard coordinator wiring.
	if r := pool.RandomInShard(0); r == nil {
		t.Fatalf("shard 0 has no accounts in a 100-account pool")
	}
	if r := pool.RandomInShard(1); r == nil {
		t.Fatalf("shard 1 has no accounts in a 100-account pool")
	}
}

func TestPool_PersistedFileHasRestrictivePermissions(t *testing.T) {
	dir := t.TempDir()
	cfg := config.AccountsConfig{
		PoolSize:          2,
		StateDir:          dir,
		RegenerateOnStart: true,
		InitialBalance:    "1",
	}
	if _, err := NewPool(cfg, mustCoordinator(t, 2)); err != nil {
		t.Fatalf("new pool: %v", err)
	}
	info, err := osStat(filepath.Join(dir, "accounts.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	mode := info.Mode().Perm()
	// 0600 — only owner can read/write. Private keys must never be
	// world-readable.
	if mode != 0o600 {
		t.Fatalf("permissions: got %o, want 600", mode)
	}
}
