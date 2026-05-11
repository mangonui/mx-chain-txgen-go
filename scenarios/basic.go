package scenarios

import (
	"context"
	"fmt"

	"github.com/mangonui/mx-chain-txgen-go/submit"
)

// BasicScenario emits native EGLD transfers between accounts in the pool.
// This is the canonical throughput scenario — every transaction is a
// minimal move-balance with the same shape as a wallet user sending coins.
type BasicScenario struct{}

// NewBasic constructs the basic scenario. It is stateless; one instance
// services every HTTP request.
func NewBasic() *BasicScenario { return &BasicScenario{} }

// Name implements Scenario.
func (b *BasicScenario) Name() string { return "basic" }

// Run implements Scenario.
//
// For each of req.NumOfTxs:
//
//  1. pick a sender uniformly from the pool
//  2. pick a receiver per req.Destination (same / cross / mixed) relative
//     to the sender's shard
//  3. assign nonce — refresh from chain first when req.RecallNonce is true
//  4. build a v1 frontend transaction with the supplied gas/value
//  5. queue the job for batched signing + submission
//
// The submitter handles batching internally (BunchSize per round-trip).
func (b *BasicScenario) Run(ctx context.Context, req Request, comp *Components) (*Result, error) {
	if req.NumOfTxs <= 0 {
		return nil, fmt.Errorf("numOfTxs must be > 0")
	}
	jobs := make([]submit.Job, 0, req.NumOfTxs)

	for i := 0; i < req.NumOfTxs; i++ {
		sender := comp.Pool.Random()
		if sender == nil {
			return nil, fmt.Errorf("pool is empty")
		}
		receiver := comp.Pool.PickReceiver(sender.ShardID, req.Destination, comp.Shards.NumShards())
		if receiver == nil {
			return nil, fmt.Errorf("could not pick receiver")
		}
		if req.RecallNonce {
			if err := comp.Nonces.Refresh(ctx, sender.AddressHandler, sender.Bech32); err != nil {
				return nil, err
			}
		}
		nonce := comp.Nonces.Next(sender.Bech32)
		tx := buildTx(req, comp, sender.Bech32, receiver.Bech32, nonce, req.Value, nil)
		jobs = append(jobs, submit.Job{Sender: sender, Tx: tx})
	}
	hashes, err := comp.Submitter.SignAndSubmit(ctx, jobs)
	if err != nil {
		return nil, err
	}
	return &Result{
		NumSent: len(hashes),
		Hashes:  hashes,
	}, nil
}
