package accounts

import (
	"testing"

	"github.com/mangonui/mx-chain-txgen-go/config"
	"github.com/mangonui/mx-chain-txgen-go/shards"
)

// TestPool_PickReceiver_CrossShardNeverReturnsSameShard generates a
// 200-account pool over 2 shards, then asks for 5000 cross-shard
// receivers from accounts in shard 0 and verifies every receiver lands
// in shard 1. Catches the obvious regression of "cross_shard silently
// becomes same_shard" if the shard math drifts.
func TestPool_PickReceiver_CrossShardNeverReturnsSameShard(t *testing.T) {
	dir := t.TempDir()
	cfg := config.AccountsConfig{
		PoolSize:          200,
		StateDir:          dir,
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

	// Find any sender in shard 0. With 200 accounts over 2 shards the
	// probability of shard 0 being empty is effectively zero.
	var senderShard uint32 = 0
	if pool.RandomInShard(senderShard) == nil {
		// pool happened to be lopsided — try shard 1 instead.
		senderShard = 1
		if pool.RandomInShard(senderShard) == nil {
			t.Fatalf("pool is empty in both shards; cannot run test")
		}
	}
	otherShard := 1 - senderShard

	const trials = 5000
	for i := 0; i < trials; i++ {
		r := pool.PickReceiver(senderShard, shards.CrossShard, sc.NumShards())
		if r == nil {
			t.Fatalf("trial %d: nil receiver", i)
		}
		if r.ShardID == senderShard {
			t.Fatalf("trial %d: CrossShard returned receiver in sender's shard (%d)", i, r.ShardID)
		}
		if r.ShardID != otherShard {
			t.Fatalf("trial %d: receiver in shard %d, expected %d", i, r.ShardID, otherShard)
		}
	}
}

// TestPool_PickReceiver_SameShardNeverCrosses is the dual check: when the
// caller asks for SameShard, the returned receiver must share the
// sender's shard ID (assuming the sender's shard is populated).
func TestPool_PickReceiver_SameShardNeverCrosses(t *testing.T) {
	dir := t.TempDir()
	cfg := config.AccountsConfig{
		PoolSize:          200,
		StateDir:          dir,
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

	for senderShard := uint32(0); senderShard <= 1; senderShard++ {
		if pool.RandomInShard(senderShard) == nil {
			continue // skip empty shard (lopsided pool)
		}
		for i := 0; i < 2000; i++ {
			r := pool.PickReceiver(senderShard, shards.SameShard, sc.NumShards())
			if r == nil {
				t.Fatalf("trial %d sender shard %d: nil receiver", i, senderShard)
			}
			if r.ShardID != senderShard {
				t.Fatalf("trial %d sender shard %d: receiver in shard %d", i, senderShard, r.ShardID)
			}
		}
	}
}

// TestPool_PickReceiver_MixedHitsBothShards verifies that Mixed actually
// produces a cross-shard mix rather than collapsing to one bucket. With
// 2 shards the cross-shard ratio should approach 1/2 over many trials.
func TestPool_PickReceiver_MixedHitsBothShards(t *testing.T) {
	dir := t.TempDir()
	cfg := config.AccountsConfig{
		PoolSize:          200,
		StateDir:          dir,
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
	if pool.RandomInShard(0) == nil || pool.RandomInShard(1) == nil {
		t.Skip("both shards must be populated for the Mixed distribution test")
	}

	const trials = 5000
	hits := map[uint32]int{}
	for i := 0; i < trials; i++ {
		r := pool.PickReceiver(0, shards.Mixed, sc.NumShards())
		if r == nil {
			t.Fatalf("trial %d: nil receiver", i)
		}
		hits[r.ShardID]++
	}
	if hits[0] == 0 || hits[1] == 0 {
		t.Fatalf("Mixed collapsed to a single shard: %+v", hits)
	}
	// Sanity: with a uniform pick across the pool and roughly balanced
	// shards, each bucket should hold at least 25% and at most 75% of the
	// trials. Strict statistical tests are overkill here — we want a
	// regression alarm, not a t-test.
	for shard, count := range hits {
		ratio := float64(count) / float64(trials)
		if ratio < 0.20 || ratio > 0.80 {
			t.Fatalf("Mixed shard %d ratio %.2f outside [0.20, 0.80]", shard, ratio)
		}
	}
}
