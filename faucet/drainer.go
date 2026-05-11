// Package faucet provides the boot-time funding mechanism used by the
// txgen to pre-fund its account pool from a single PEM-loaded wallet.
//
// This mirrors how mx-chain-go's scripts/testnet/include/config.sh wires
// the upstream txgen: filegen generates a chain genesis that includes a
// fully-funded "mint" wallet (`walletKey.pem`); config.sh copies that PEM
// into the txgen's config directory; the txgen reads it at startup and
// emits N transfer transactions to its pool, one per pool member.
//
// Genesis-time injection of pool accounts directly into the initial state
// is not supported by mx-chain-deploy-go/cmd/filegen (which generates
// initial accounts algorithmically from total-supply / node-price, with
// no external JSON input). The faucet-drain pattern is therefore the
// only realistic funding path on a stock local testnet.
package faucet

import (
	"context"
	"fmt"
	"os"

	"github.com/multiversx/mx-chain-crypto-go/signing"
	"github.com/multiversx/mx-chain-crypto-go/signing/ed25519"
	coreData "github.com/multiversx/mx-chain-core-go/data/transaction"
	sdkBlockchainCrypto "github.com/multiversx/mx-sdk-go/blockchain/cryptoProvider"
	sdkCore "github.com/multiversx/mx-sdk-go/core"
	sdkData "github.com/multiversx/mx-sdk-go/data"
	sdkInteractors "github.com/multiversx/mx-sdk-go/interactors"

	"github.com/mangonui/mx-chain-txgen-go/accounts"
	"github.com/mangonui/mx-chain-txgen-go/nonces"
	"github.com/mangonui/mx-chain-txgen-go/proxy"
	"github.com/mangonui/mx-chain-txgen-go/shards"
	"github.com/mangonui/mx-chain-txgen-go/submit"
)

// Drainer broadcasts pre-funding transfers from a single PEM-loaded
// faucet account to every member of the pool.
type Drainer struct {
	faucet    *accounts.Account
	pool      *accounts.Pool
	tracker   *nonces.Tracker
	submitter *submit.Submitter
	poller    *submit.Poller
	prx       *proxy.Client
	netCfg    *sdkData.NetworkConfig
}

// New constructs a Drainer. faucetPemPath is the absolute or working-dir-
// relative path of the wallet PEM emitted by mx-chain-deploy-go/filegen
// (canonically `walletKey.pem`). sc is required to compute the faucet's
// shard ID so cross-shard funding works the same way scenarios route
// receivers.
func New(
	faucetPemPath string,
	sc *shards.Coordinator,
	pool *accounts.Pool,
	tracker *nonces.Tracker,
	submitter *submit.Submitter,
	poller *submit.Poller,
	prx *proxy.Client,
	netCfg *sdkData.NetworkConfig,
) (*Drainer, error) {
	faucet, err := loadFaucet(faucetPemPath, sc)
	if err != nil {
		return nil, err
	}
	return &Drainer{
		faucet:    faucet,
		pool:      pool,
		tracker:   tracker,
		submitter: submitter,
		poller:    poller,
		prx:       prx,
		netCfg:    netCfg,
	}, nil
}

// FaucetBech32 returns the bech32 address of the loaded faucet. Logged at
// startup so operators can verify the PEM that was loaded is the one
// they expect.
func (d *Drainer) FaucetBech32() string { return d.faucet.Bech32 }

// Drain emits one transfer transaction per pool member, from the faucet
// to that pool member, for amountPerAccount atomic units. It then waits
// for the last submitted transaction to reach a terminal status — since
// the chain processes faucet transactions in strict nonce order, the
// last one's terminal state implies all previous ones executed too.
//
// Gas defaults to 50_000 (a standard move-balance) per tx; callers
// override via gasLimit. gasPrice is the chain's MinGasPrice unless
// callers explicitly provide a higher one.
func (d *Drainer) Drain(ctx context.Context, amountPerAccount string, gasPrice, gasLimit uint64) error {
	if d.pool.Len() == 0 {
		return nil
	}
	if gasPrice == 0 {
		gasPrice = d.netCfg.MinGasPrice
	}
	if gasLimit == 0 {
		gasLimit = 50_000
	}

	// Faucet's on-chain nonce must be synced once before we start
	// because the tracker.Sync runs on pool accounts at startup, not on
	// the faucet.
	if err := d.tracker.Sync(ctx, d.faucet.AddressHandler, d.faucet.Bech32); err != nil {
		return fmt.Errorf("faucet: initial nonce sync: %w", err)
	}

	jobs := make([]submit.Job, 0, d.pool.Len())
	for _, target := range d.pool.All() {
		tx := &coreData.FrontendTransaction{
			Nonce:    d.tracker.Next(d.faucet.Bech32),
			Value:    amountPerAccount,
			Sender:   d.faucet.Bech32,
			Receiver: target.Bech32,
			GasPrice: gasPrice,
			GasLimit: gasLimit,
			ChainID:  d.netCfg.ChainID,
			Version:  1,
		}
		jobs = append(jobs, submit.Job{Sender: d.faucet, Tx: tx})
	}
	hashes, err := d.submitter.SignAndSubmit(ctx, jobs)
	if err != nil {
		return fmt.Errorf("faucet: sign and submit %d txs: %w", len(jobs), err)
	}
	if len(hashes) == 0 {
		return fmt.Errorf("faucet: submitter returned zero hashes for %d jobs", len(jobs))
	}
	last := hashes[len(hashes)-1]
	status, err := d.poller.WaitForExecution(ctx, last)
	if err != nil {
		return fmt.Errorf("faucet: wait for last tx %s: %w", last, err)
	}
	if status != "success" && status != "executed" {
		return fmt.Errorf("faucet: last drain tx %s terminal status %q", last, status)
	}
	return nil
}

// loadFaucet reads a single-key PEM (MultiversX wallet format) and
// builds an accounts.Account with the same crypto holder pattern the
// pool uses, so the submitter can sign faucet txs without any special-
// casing.
func loadFaucet(pemPath string, sc *shards.Coordinator) (*accounts.Account, error) {
	if pemPath == "" {
		return nil, fmt.Errorf("faucet PEM path is empty")
	}
	pemBytes, err := os.ReadFile(pemPath)
	if err != nil {
		return nil, fmt.Errorf("read faucet pem %s: %w", pemPath, err)
	}
	w := sdkInteractors.NewWallet()
	skBytes, err := w.LoadPrivateKeyFromPemData(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("decode pem: %w", err)
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
		return nil, fmt.Errorf("public key to bytes: %w", err)
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
	return &accounts.Account{
		Index:          -1, // signals "not in the pool"
		PrivateKey:     skBytes,
		PublicKey:      pkBytes,
		Bech32:         bech32,
		ShardID:        shardID,
		AddressHandler: addr,
		CryptoHolder:   holder,
	}, nil
}

// Ensure unused import discipline — the SDK Core type is referenced
// through the holder factory above but the explicit import keeps godoc
// linkage clear for readers.
var _ = sdkCore.AddressHandler(nil)
