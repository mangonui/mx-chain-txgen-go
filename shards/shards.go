package shards

import (
	"fmt"

	sdkBlockchain "github.com/multiversx/mx-sdk-go/blockchain"
	sdkCore "github.com/multiversx/mx-sdk-go/core"
)

// Destination is the receiver-selection mode driven by the HTTP request's
// "destination" field.
type Destination int

const (
	// SameShard picks a receiver in the same shard as the sender. Pure
	// intra-shard load.
	SameShard Destination = iota
	// CrossShard picks a receiver in a strictly different shard than the
	// sender. Exercises miniblock dispatching and cross-shard execution.
	CrossShard
	// Mixed picks proportionally so that the receiver lands in any shard
	// with equal probability (so for N shards the cross-shard ratio
	// approaches (N-1)/N).
	Mixed
)

// ParseDestination converts the HTTP-layer string into a typed Destination.
func ParseDestination(s string) (Destination, error) {
	switch s {
	case "same_shard", "same":
		return SameShard, nil
	case "cross_shard", "cross":
		return CrossShard, nil
	case "mixed", "":
		return Mixed, nil
	default:
		return 0, fmt.Errorf("unknown destination %q", s)
	}
}

// Coordinator wraps mx-sdk-go's shardCoordinator so the txgen computes
// shard IDs with the same algorithm the chain uses. Encapsulating it here
// keeps the rest of the codebase free of SDK plumbing.
type Coordinator struct {
	sdk       *sdkShardCoordinatorAdapter
	numShards uint32
}

// sdkShardCoordinatorAdapter is the local view of the SDK's
// shardCoordinator. Using an unexported type avoids leaking SDK types
// across package boundaries.
type sdkShardCoordinatorAdapter struct {
	inner interface {
		ComputeShardId(address sdkCore.AddressHandler) (uint32, error)
		IsInterfaceNil() bool
	}
}

// NewCoordinator constructs a coordinator pinned to a number of shards.
// currentShard is unused for txgen (we are not running inside a shard) but
// the SDK constructor requires it.
func NewCoordinator(numShards uint32) (*Coordinator, error) {
	if numShards == 0 {
		return nil, fmt.Errorf("numShards must be > 0")
	}
	inner, err := sdkBlockchain.NewShardCoordinator(numShards, 0)
	if err != nil {
		return nil, fmt.Errorf("new shard coordinator: %w", err)
	}
	return &Coordinator{
		sdk:       &sdkShardCoordinatorAdapter{inner: inner},
		numShards: numShards,
	}, nil
}

// NumShards returns the configured shard count.
func (c *Coordinator) NumShards() uint32 {
	return c.numShards
}

// ComputeShardID returns the shard ID for a given address handler.
func (c *Coordinator) ComputeShardID(addr sdkCore.AddressHandler) (uint32, error) {
	return c.sdk.inner.ComputeShardId(addr)
}
