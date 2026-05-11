package accounts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	mrand "math/rand"
	"os"
	"path/filepath"
	"sync"

	"github.com/multiversx/mx-chain-crypto-go/signing"
	"github.com/multiversx/mx-chain-crypto-go/signing/ed25519"
	sdkCore "github.com/multiversx/mx-sdk-go/core"

	"github.com/mangonui/mx-chain-txgen-go/config"
	"github.com/mangonui/mx-chain-txgen-go/shards"
)

// Account is one synthetic load-test EOA. The crypto components holder is
// cached at boot so the high-throughput signing path never re-derives it.
type Account struct {
	Index      int
	PrivateKey []byte
	PublicKey  []byte
	Bech32     string
	ShardID    uint32

	AddressHandler sdkCore.AddressHandler
	CryptoHolder   sdkCore.CryptoComponentsHolder
}

// Pool is the bag of accounts the txgen owns. Layout is index-keyed plus a
// per-shard secondary index so cross-shard receiver picks are O(1).
type Pool struct {
	all     []*Account
	byShard map[uint32][]*Account

	rngMu sync.Mutex
	rng   *mrand.Rand
}

// NewPool either loads an existing pool from disk or generates a fresh one
// and persists it. When cfg.RegenerateOnStart is true the previous state
// is discarded.
func NewPool(cfg config.AccountsConfig, sc *shards.Coordinator) (*Pool, error) {
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", cfg.StateDir, err)
	}

	statePath := filepath.Join(cfg.StateDir, "accounts.json")
	if !cfg.RegenerateOnStart {
		if pool, err := load(statePath, sc); err == nil && pool.Len() == cfg.PoolSize {
			return pool, nil
		}
	}

	pool, err := generate(cfg.PoolSize, sc)
	if err != nil {
		return nil, err
	}
	if err := pool.save(statePath); err != nil {
		return nil, fmt.Errorf("persist accounts: %w", err)
	}
	return pool, nil
}

// Len returns the number of accounts in the pool.
func (p *Pool) Len() int { return len(p.all) }

// All returns the underlying slice (read-only — callers must not mutate).
func (p *Pool) All() []*Account { return p.all }

// Index returns the i-th account or nil if out of range.
func (p *Pool) Index(i int) *Account {
	if i < 0 || i >= len(p.all) {
		return nil
	}
	return p.all[i]
}

// Random picks an arbitrary account from the pool.
func (p *Pool) Random() *Account {
	p.rngMu.Lock()
	defer p.rngMu.Unlock()
	if len(p.all) == 0 {
		return nil
	}
	return p.all[p.rng.Intn(len(p.all))]
}

// RandomInShard picks an arbitrary account in the given shard.
func (p *Pool) RandomInShard(shardID uint32) *Account {
	bucket := p.byShard[shardID]
	if len(bucket) == 0 {
		return nil
	}
	p.rngMu.Lock()
	defer p.rngMu.Unlock()
	return bucket[p.rng.Intn(len(bucket))]
}

// PickReceiver returns a receiver for a transaction whose sender is in
// senderShard, honoring the requested destination policy.
//
// Mixed picks any account in the pool uniformly. The cross-shard ratio
// then approaches (numShards-1)/numShards for sufficiently uniform pool
// shard distribution.
//
// SameShard / CrossShard fall back to Random() if the desired bucket is
// empty (defensive — small pools may not populate every shard).
func (p *Pool) PickReceiver(senderShard uint32, dest shards.Destination, numShards uint32) *Account {
	switch dest {
	case shards.SameShard:
		if r := p.RandomInShard(senderShard); r != nil {
			return r
		}
		return p.Random()
	case shards.CrossShard:
		if numShards <= 1 {
			return p.Random()
		}
		p.rngMu.Lock()
		offset := uint32(p.rng.Intn(int(numShards-1))) + 1
		p.rngMu.Unlock()
		target := (senderShard + offset) % numShards
		if r := p.RandomInShard(target); r != nil {
			return r
		}
		return p.Random()
	default: // Mixed
		return p.Random()
	}
}

// generate creates a fresh pool of n accounts with random ed25519 keys.
func generate(n int, sc *shards.Coordinator) (*Pool, error) {
	suite := ed25519.NewEd25519()
	keyGen := signing.NewKeyGenerator(suite)

	pool := &Pool{
		all:     make([]*Account, 0, n),
		byShard: make(map[uint32][]*Account),
		rng:     mrand.New(mrand.NewSource(seedFromCryptoRand())),
	}

	for i := 0; i < n; i++ {
		sk, _ := keyGen.GeneratePair()
		skBytes, err := sk.ToByteArray()
		if err != nil {
			return nil, fmt.Errorf("export private key %d: %w", i, err)
		}
		acc, err := BuildAccount(skBytes, i, sc)
		if err != nil {
			return nil, fmt.Errorf("build account %d: %w", i, err)
		}
		pool.all = append(pool.all, acc)
		pool.byShard[acc.ShardID] = append(pool.byShard[acc.ShardID], acc)
	}
	return pool, nil
}

// persistRecord is the on-disk shape. We persist only the secret material;
// public keys, addresses, and shard IDs are derived on load so the file
// stays small and never goes stale relative to sharding changes.
type persistRecord struct {
	Index      int    `json:"index"`
	PrivateKey string `json:"privateKey"`
}

type persistFile struct {
	Version  int             `json:"version"`
	PoolSize int             `json:"poolSize"`
	Accounts []persistRecord `json:"accounts"`
}

func (p *Pool) save(path string) error {
	out := persistFile{
		Version:  1,
		PoolSize: len(p.all),
		Accounts: make([]persistRecord, 0, len(p.all)),
	}
	for _, a := range p.all {
		out.Accounts = append(out.Accounts, persistRecord{
			Index:      a.Index,
			PrivateKey: hex.EncodeToString(a.PrivateKey),
		})
	}
	buff, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, buff, 0o600)
}

func load(path string, sc *shards.Coordinator) (*Pool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f persistFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("unmarshal pool: %w", err)
	}
	if f.Version != 1 {
		return nil, fmt.Errorf("unsupported pool version %d", f.Version)
	}

	pool := &Pool{
		all:     make([]*Account, 0, len(f.Accounts)),
		byShard: make(map[uint32][]*Account),
		rng:     mrand.New(mrand.NewSource(seedFromCryptoRand())),
	}
	for _, rec := range f.Accounts {
		skBytes, err := hex.DecodeString(rec.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("decode privkey index %d: %w", rec.Index, err)
		}
		acc, err := BuildAccount(skBytes, rec.Index, sc)
		if err != nil {
			return nil, fmt.Errorf("rebuild account %d: %w", rec.Index, err)
		}
		pool.all = append(pool.all, acc)
		pool.byShard[acc.ShardID] = append(pool.byShard[acc.ShardID], acc)
	}
	return pool, nil
}

// SyncNonces seeds every account's nonce from the proxy at boot.
//
// Runs syncFn against up to concurrency accounts in parallel; bounded by
// a semaphore channel so a 10k-account pool against a single proxy
// doesn't open 10k simultaneous HTTP connections. Returns the first
// error encountered; later errors after the first failure are
// discarded but every in-flight call is allowed to drain.
//
// concurrency <= 0 falls back to serial execution.
func (p *Pool) SyncNonces(
	ctx context.Context,
	syncFn func(ctx context.Context, addr sdkCore.AddressHandler, bech32 string) error,
	concurrency int,
) error {
	if len(p.all) == 0 {
		return nil
	}
	if concurrency <= 0 {
		concurrency = 1
	}

	sem := make(chan struct{}, concurrency)
	var (
		mu       sync.Mutex
		firstErr error
		wg       sync.WaitGroup
	)

	for _, acc := range p.all {
		// Primary gate: if the context is already cancelled, do not
		// schedule any further work. A bare select { ctx.Done() | sem }
		// would race because Go picks pseudo-randomly between ready
		// cases, letting up to `concurrency` jobs slip in after
		// cancellation.
		if err := ctx.Err(); err != nil {
			wg.Wait()
			if firstErr != nil {
				return firstErr
			}
			return err
		}
		select {
		case <-ctx.Done():
			// Cancellation arrived while waiting on a semaphore slot.
			wg.Wait()
			if firstErr != nil {
				return firstErr
			}
			return ctx.Err()
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go func(a *Account) {
			defer wg.Done()
			defer func() { <-sem }()

			// Short-circuit if another goroutine already failed; saves
			// pointless work but is not a correctness requirement
			// (syncFn must be tolerant of in-flight cancellation
			// regardless).
			mu.Lock()
			alreadyFailed := firstErr != nil
			mu.Unlock()
			if alreadyFailed {
				return
			}
			if err := syncFn(ctx, a.AddressHandler, a.Bech32); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("sync nonce for %s (index %d): %w", a.Bech32, a.Index, err)
				}
				mu.Unlock()
			}
		}(acc)
	}
	wg.Wait()
	return firstErr
}

func seedFromCryptoRand() int64 {
	maxVal := new(big.Int).SetInt64(1<<62 - 1)
	n, err := rand.Int(rand.Reader, maxVal)
	if err != nil {
		// crypto/rand failure means the OS entropy pool is broken; fall
		// back to a deterministic seed so the binary still works. The
		// selection randomness is non-security-critical (it only
		// affects which accounts pair with which) so this is fine.
		return 1
	}
	return n.Int64()
}
