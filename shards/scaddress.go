package shards

import (
	"fmt"

	sdkBlockchain "github.com/multiversx/mx-sdk-go/blockchain"
	sdkData "github.com/multiversx/mx-sdk-go/data"
)

// ComputeContractAddress derives the bech32 address that a wasm contract
// deployed by deployerPubKey at the given creation nonce will assume on
// chain. The derivation algorithm matches mx-chain-go's BlockChainHookImpl
// .NewAddress for the wasm VM (factory.WasmVirtualMachine), so the result
// is the exact address the chain will record post-execution.
//
// Constructing a fresh shardCoordinator + addressGenerator per call is a
// minor overhead vs. caching them, but it keeps this helper stateless and
// re-entrant; the deploy path is a one-shot per scenario invocation, not
// a hot path.
func ComputeContractAddress(deployerPubKey []byte, nonce uint64, numShards uint32) (string, error) {
	if len(deployerPubKey) == 0 {
		return "", fmt.Errorf("deployer public key is empty")
	}
	if numShards == 0 {
		return "", fmt.Errorf("numShards must be > 0")
	}
	coord, err := sdkBlockchain.NewShardCoordinator(numShards, 0)
	if err != nil {
		return "", fmt.Errorf("new shard coordinator: %w", err)
	}
	ag, err := sdkBlockchain.NewAddressGenerator(coord)
	if err != nil {
		return "", fmt.Errorf("new address generator: %w", err)
	}
	deployer := sdkData.NewAddressFromBytes(deployerPubKey)
	scAddr, err := ag.ComputeWasmVMScAddress(deployer, nonce)
	if err != nil {
		return "", fmt.Errorf("compute wasm vm sc address: %w", err)
	}
	return scAddr.AddressAsBech32String()
}
