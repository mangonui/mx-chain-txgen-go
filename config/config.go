package config

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

// Config is the top-level configuration struct loaded from config.toml.
type Config struct {
	Server    ServerConfig
	Proxy     ProxyConfig
	Accounts  AccountsConfig
	Sharding  ShardingConfig
	Scenarios ScenariosConfig
	ERC20     ERC20Config
	Polling   PollingConfig
	Stats     StatsConfig
	Faucet    FaucetConfig
	Submit    SubmitConfig
}

// ServerConfig governs the txgen's own HTTP listener.
type ServerConfig struct {
	Port                   int
	ShutdownTimeoutSeconds int
}

// ProxyConfig governs the upstream proxy client.
type ProxyConfig struct {
	URL                    string
	FinalityCheck          bool
	CacheExpirationSeconds int
}

// AccountsConfig governs the synthetic account pool.
type AccountsConfig struct {
	PoolSize          int
	StateDir          string
	RegenerateOnStart bool
	InitialBalance    string
	// SyncConcurrency caps the number of in-flight GetAccount requests
	// the boot-time nonce sync issues against the proxy. Default 16 is
	// gentle on local proxies; tune up for large pools when the proxy
	// is on different hardware.
	SyncConcurrency int
}

// ShardingConfig must match the testnet's shard count so the
// shardCoordinator computes the same shard IDs the chain uses.
type ShardingConfig struct {
	NumShards uint32
}

// ScenariosConfig gates which scenarios the HTTP handler will dispatch.
type ScenariosConfig struct {
	Enabled []string
}

// ERC20Config supplies the wasm path for the deploy sub-command.
type ERC20Config struct {
	WasmPath string
}

// PollingConfig governs tx-status polling for scenario sub-commands that
// must wait for chain inclusion (deploy, issue).
type PollingConfig struct {
	IntervalMilliseconds int
	TimeoutSeconds       int
}

// StatsConfig governs the included-TPS sampler.
type StatsConfig struct {
	EnableTPSSampler       bool
	SamplerIntervalSeconds int
}

// SubmitConfig governs the transaction submitter.
type SubmitConfig struct {
	// BunchSize is the maximum number of signed transactions packed
	// into a single POST /transaction/send-multiple call. 100 mirrors
	// the upstream txgen's implicit default and is comfortable for
	// stock mx-chain-proxy-go. Increase only if the proxy is known to
	// accept larger batches.
	BunchSize int
}

// FaucetConfig governs the optional boot-time funding step.
//
// On a stock mx-chain-go local testnet, mx-chain-deploy-go/filegen
// generates a pre-funded "mint" wallet (canonically walletKey.pem) and
// the scripts/testnet bootstrap copies it into the txgen's config
// directory. When Enabled is true, the txgen reads that PEM at startup
// and pre-funds every pool account with AmountPerAccount atomic units
// before serving any scenario requests.
type FaucetConfig struct {
	Enabled          bool
	PemPath          string
	AmountPerAccount string
	GasPrice         uint64
	GasLimit         uint64
}

// Load reads and validates a config file from disk.
func Load(path string) (*Config, error) {
	cfg := &Config{}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	c.applyDefaults()
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid Server.Port: %d", c.Server.Port)
	}
	if c.Server.ShutdownTimeoutSeconds < 0 {
		return fmt.Errorf("Server.ShutdownTimeoutSeconds must be >= 0")
	}
	if c.Accounts.SyncConcurrency <= 0 {
		return fmt.Errorf("Accounts.SyncConcurrency must be > 0")
	}
	if c.Submit.BunchSize <= 0 {
		return fmt.Errorf("Submit.BunchSize must be > 0")
	}
	if c.Proxy.URL == "" {
		return fmt.Errorf("Proxy.URL is required")
	}
	if c.Accounts.PoolSize <= 0 {
		return fmt.Errorf("Accounts.PoolSize must be > 0")
	}
	if c.Accounts.StateDir == "" {
		return fmt.Errorf("Accounts.StateDir is required")
	}
	if c.Sharding.NumShards == 0 {
		return fmt.Errorf("Sharding.NumShards must be > 0")
	}
	if c.Polling.IntervalMilliseconds <= 0 {
		return fmt.Errorf("Polling.IntervalMilliseconds must be > 0")
	}
	if c.Polling.TimeoutSeconds <= 0 {
		return fmt.Errorf("Polling.TimeoutSeconds must be > 0")
	}
	if c.Faucet.Enabled {
		if c.Faucet.PemPath == "" {
			return fmt.Errorf("Faucet.Enabled=true but Faucet.PemPath is empty")
		}
		if c.Faucet.AmountPerAccount == "" || c.Faucet.AmountPerAccount == "0" {
			return fmt.Errorf("Faucet.AmountPerAccount must be a positive integer string")
		}
	}
	return nil
}

// applyDefaults backfills zero-valued knobs with sane defaults so older
// config files don't break after a new tunable is added.
func (c *Config) applyDefaults() {
	if c.Server.ShutdownTimeoutSeconds == 0 {
		c.Server.ShutdownTimeoutSeconds = 15
	}
	if c.Accounts.SyncConcurrency == 0 {
		c.Accounts.SyncConcurrency = 16
	}
	if c.Submit.BunchSize == 0 {
		c.Submit.BunchSize = 100
	}
}
