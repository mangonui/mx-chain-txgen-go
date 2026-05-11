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
}

// ServerConfig governs the txgen's own HTTP listener.
type ServerConfig struct {
	Port int
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
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid Server.Port: %d", c.Server.Port)
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
	return nil
}
