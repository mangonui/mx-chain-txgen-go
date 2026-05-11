package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"time"

	"github.com/mangonui/mx-chain-txgen-go/accounts"
	"github.com/mangonui/mx-chain-txgen-go/api"
	"github.com/mangonui/mx-chain-txgen-go/config"
	"github.com/mangonui/mx-chain-txgen-go/faucet"
	"github.com/mangonui/mx-chain-txgen-go/nonces"
	"github.com/mangonui/mx-chain-txgen-go/proxy"
	"github.com/mangonui/mx-chain-txgen-go/scenarios"
	"github.com/mangonui/mx-chain-txgen-go/shards"
	"github.com/mangonui/mx-chain-txgen-go/stats"
	"github.com/mangonui/mx-chain-txgen-go/submit"
	"github.com/mangonui/mx-chain-txgen-go/version"
)

func main() {
	cfgPath := flag.String("config", "config/config.toml", "path to config.toml")
	emitGenesis := flag.Bool("emit-genesis-balances", false,
		"do not start the server; emit a mx-chain-deploy-go initialBalances.json for the configured pool and exit")
	regenerate := flag.Bool("regenerate-accounts", false,
		"regenerate the account pool on startup (overrides config)")
	numAccountsFlag := flag.Int("num-accounts", 0,
		"override Accounts.PoolSize (used by upstream scripts/testnet wrapper)")
	newAccountsFlag := flag.Bool("new-accounts", false,
		"alias of --regenerate-accounts; matches the upstream txgen CLI flag")
	versionFlag := flag.Bool("version", false, "print version info and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("txgen %s (commit %s, built %s)\n",
			version.Version, version.Commit, version.BuildDate)
		return
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if *regenerate || *newAccountsFlag {
		cfg.Accounts.RegenerateOnStart = true
	}
	if *numAccountsFlag > 0 {
		cfg.Accounts.PoolSize = *numAccountsFlag
	}

	if *emitGenesis {
		if err := runEmitGenesis(cfg); err != nil {
			log.Fatalf("emit-genesis-balances: %v", err)
		}
		return
	}

	if err := runServer(cfg); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// runEmitGenesis materialises the account pool without touching the proxy,
// writes the genesis balances JSON to stdout, and exits. Used as part of
// the local testnet bootstrap before mx-chain-deploy-go runs.
func runEmitGenesis(cfg *config.Config) error {
	sc, err := shards.NewCoordinator(cfg.Sharding.NumShards)
	if err != nil {
		return fmt.Errorf("shard coordinator: %w", err)
	}
	pool, err := accounts.NewPool(cfg.Accounts, sc)
	if err != nil {
		return fmt.Errorf("accounts pool: %w", err)
	}
	return accounts.EmitInitialBalances(os.Stdout, pool, cfg.Accounts.InitialBalance)
}

// runServer wires every component and runs the HTTP listener until SIGINT
// or SIGTERM. Initial nonce sync is serialised against the proxy to keep
// load gentle at boot; for very large pools this is the dominant startup
// cost and could be parallelised in a later revision.
func runServer(cfg *config.Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-sigs
		log.Printf("shutdown signal: %s", s)
		cancel()
	}()

	prx, err := proxy.New(cfg.Proxy)
	if err != nil {
		return fmt.Errorf("proxy client: %w", err)
	}
	log.Printf("proxy: %s", cfg.Proxy.URL)

	netCfg, err := prx.SDK.GetNetworkConfig(ctx)
	if err != nil {
		return fmt.Errorf("get network config: %w", err)
	}
	log.Printf("network: chainID=%s minGasPrice=%d", netCfg.ChainID, netCfg.MinGasPrice)

	sc, err := shards.NewCoordinator(cfg.Sharding.NumShards)
	if err != nil {
		return fmt.Errorf("shard coordinator: %w", err)
	}

	pool, err := accounts.NewPool(cfg.Accounts, sc)
	if err != nil {
		return fmt.Errorf("accounts pool: %w", err)
	}
	log.Printf("pool: %d accounts (regenerate=%t)", pool.Len(), cfg.Accounts.RegenerateOnStart)

	tracker := nonces.New(prx)
	log.Printf("syncing nonces for %d accounts...", pool.Len())
	if err := pool.SyncNonces(ctx, tracker.Sync, cfg.Accounts.SyncConcurrency); err != nil {
		return fmt.Errorf("sync nonces: %w", err)
	}
	log.Printf("nonces synced")

	submitter, err := submit.New(prx, cfg.Submit.BunchSize)
	if err != nil {
		return fmt.Errorf("submitter: %w", err)
	}
	poller := submit.NewPoller(prx, cfg.Polling)

	if cfg.Faucet.Enabled {
		drainer, err := faucet.New(cfg.Faucet.PemPath, sc, pool, tracker, submitter, poller, prx, netCfg)
		if err != nil {
			return fmt.Errorf("faucet: %w", err)
		}
		log.Printf("faucet: draining from %s (%s atomic units per account, %d accounts)",
			drainer.FaucetBech32(), cfg.Faucet.AmountPerAccount, pool.Len())
		if err := drainer.Drain(ctx, cfg.Faucet.AmountPerAccount, cfg.Faucet.GasPrice, cfg.Faucet.GasLimit); err != nil {
			return fmt.Errorf("faucet drain: %w", err)
		}
		log.Printf("faucet: drain complete; re-syncing pool nonces from chain")
		if err := pool.SyncNonces(ctx, tracker.Sync, cfg.Accounts.SyncConcurrency); err != nil {
			return fmt.Errorf("post-drain nonce resync: %w", err)
		}
	}

	comp := &scenarios.Components{
		Pool:      pool,
		Nonces:    tracker,
		Shards:    sc,
		Submitter: submitter,
		Poller:    poller,
		Proxy:     prx,
		NetConfig: netCfg,
		Cfg:       cfg,
	}

	registry, err := scenarios.NewRegistry(
		[]scenarios.Scenario{
			scenarios.NewBasic(),
			scenarios.NewERC20(),
			scenarios.NewESDT(),
		},
		cfg.Scenarios.Enabled,
	)
	if err != nil {
		return fmt.Errorf("scenario registry: %w", err)
	}
	log.Printf("scenarios enabled: %v", registry.Names())

	// The sampler retains entries for the longest reporting window the
	// /stats endpoint exposes (1h). Disabled via config returns a nil
	// sampler which the handler interprets as "skip recording".
	var sampler *stats.Sampler
	if cfg.Stats.EnableTPSSampler {
		sampler = stats.New(time.Hour)
		log.Printf("stats: sampler enabled, retaining 1h window")
	}

	srv := api.New(cfg.Server, registry, comp, sampler)
	log.Printf("listening on :%d", cfg.Server.Port)
	return srv.Start(ctx)
}
