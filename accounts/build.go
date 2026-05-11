package accounts

import (
	"fmt"

	"github.com/multiversx/mx-chain-crypto-go/signing"
	"github.com/multiversx/mx-chain-crypto-go/signing/ed25519"
	sdkBlockchainCrypto "github.com/multiversx/mx-sdk-go/blockchain/cryptoProvider"
	sdkData "github.com/multiversx/mx-sdk-go/data"

	"github.com/mangonui/mx-chain-txgen-go/shards"
)

// BuildAccount derives a fully-formed Account from a raw ed25519 private
// key, the desired pool index, and the shard coordinator. Used by:
//
//   - pool generation (one call per generated key)
//   - pool reload from disk (one call per persisted key)
//   - faucet PEM loading (one call for the single faucet key)
//
// Centralising the derivation keeps the (sk → pk → bech32 → shardID →
// CryptoComponentsHolder) chain in a single place; if a future
// MultiversX upgrade changes the address format or the crypto holder
// shape, every caller picks up the change for free.
//
// poolIndex < 0 is the convention for "this account is not a pool
// member" (the faucet uses index = -1).
func BuildAccount(skBytes []byte, poolIndex int, sc *shards.Coordinator) (*Account, error) {
	if len(skBytes) == 0 {
		return nil, fmt.Errorf("empty private key")
	}
	suite := ed25519.NewEd25519()
	keyGen := signing.NewKeyGenerator(suite)
	sk, err := keyGen.PrivateKeyFromByteArray(skBytes)
	if err != nil {
		return nil, fmt.Errorf("private key from bytes: %w", err)
	}
	pk := sk.GeneratePublic()
	pkBytes, err := pk.ToByteArray()
	if err != nil {
		return nil, fmt.Errorf("export public key: %w", err)
	}
	addr := sdkData.NewAddressFromBytes(pkBytes)
	bech32, err := addr.AddressAsBech32String()
	if err != nil {
		return nil, fmt.Errorf("bech32 encode: %w", err)
	}
	shardID, err := sc.ComputeShardID(addr)
	if err != nil {
		return nil, fmt.Errorf("compute shard: %w", err)
	}
	holder, err := sdkBlockchainCrypto.NewCryptoComponentsHolder(keyGen, skBytes)
	if err != nil {
		return nil, fmt.Errorf("crypto holder: %w", err)
	}
	return &Account{
		Index:          poolIndex,
		PrivateKey:     skBytes,
		PublicKey:      pkBytes,
		Bech32:         bech32,
		ShardID:        shardID,
		AddressHandler: addr,
		CryptoHolder:   holder,
	}, nil
}
