package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validConfig = `
[Server]
Port = 7951
[Proxy]
URL = "http://localhost:7950"
FinalityCheck = false
CacheExpirationSeconds = 60
[Accounts]
PoolSize = 10
StateDir = "./state"
RegenerateOnStart = true
InitialBalance = "1000"
[Sharding]
NumShards = 2
[Scenarios]
Enabled = ["basic"]
[ERC20]
WasmPath = "./contracts/erc20.wasm"
[Polling]
IntervalMilliseconds = 500
TimeoutSeconds = 60
[Stats]
EnableTPSSampler = false
SamplerIntervalSeconds = 5
[Faucet]
Enabled = false
PemPath = ""
AmountPerAccount = "0"
GasPrice = 0
GasLimit = 0
`

// writeConfig drops the body into a temp file and returns the path.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

func TestLoad_ValidConfig(t *testing.T) {
	cfg, err := Load(writeConfig(t, validConfig))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Server.Port != 7951 {
		t.Fatalf("Server.Port: got %d", cfg.Server.Port)
	}
	if cfg.Sharding.NumShards != 2 {
		t.Fatalf("Sharding.NumShards: got %d", cfg.Sharding.NumShards)
	}
}

func TestLoad_MissingFileIsError(t *testing.T) {
	if _, err := Load("/nonexistent/path/config.toml"); err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func TestLoad_ZeroPortRejected(t *testing.T) {
	body := strings.Replace(validConfig, "Port = 7951", "Port = 0", 1)
	_, err := Load(writeConfig(t, body))
	if err == nil || !strings.Contains(err.Error(), "Server.Port") {
		t.Fatalf("expected Server.Port error, got %v", err)
	}
}

func TestLoad_OutOfRangePortRejected(t *testing.T) {
	body := strings.Replace(validConfig, "Port = 7951", "Port = 70000", 1)
	_, err := Load(writeConfig(t, body))
	if err == nil || !strings.Contains(err.Error(), "Server.Port") {
		t.Fatalf("expected Server.Port error, got %v", err)
	}
}

func TestLoad_EmptyProxyURLRejected(t *testing.T) {
	body := strings.Replace(validConfig, `URL = "http://localhost:7950"`, `URL = ""`, 1)
	_, err := Load(writeConfig(t, body))
	if err == nil || !strings.Contains(err.Error(), "Proxy.URL") {
		t.Fatalf("expected Proxy.URL error, got %v", err)
	}
}

func TestLoad_ZeroPoolSizeRejected(t *testing.T) {
	body := strings.Replace(validConfig, "PoolSize = 10", "PoolSize = 0", 1)
	_, err := Load(writeConfig(t, body))
	if err == nil || !strings.Contains(err.Error(), "Accounts.PoolSize") {
		t.Fatalf("expected Accounts.PoolSize error, got %v", err)
	}
}

func TestLoad_EmptyStateDirRejected(t *testing.T) {
	body := strings.Replace(validConfig, `StateDir = "./state"`, `StateDir = ""`, 1)
	_, err := Load(writeConfig(t, body))
	if err == nil || !strings.Contains(err.Error(), "Accounts.StateDir") {
		t.Fatalf("expected Accounts.StateDir error, got %v", err)
	}
}

func TestLoad_ZeroNumShardsRejected(t *testing.T) {
	body := strings.Replace(validConfig, "NumShards = 2", "NumShards = 0", 1)
	_, err := Load(writeConfig(t, body))
	if err == nil || !strings.Contains(err.Error(), "Sharding.NumShards") {
		t.Fatalf("expected Sharding.NumShards error, got %v", err)
	}
}

func TestLoad_ZeroPollingIntervalRejected(t *testing.T) {
	body := strings.Replace(validConfig, "IntervalMilliseconds = 500", "IntervalMilliseconds = 0", 1)
	_, err := Load(writeConfig(t, body))
	if err == nil || !strings.Contains(err.Error(), "Polling.IntervalMilliseconds") {
		t.Fatalf("expected Polling.IntervalMilliseconds error, got %v", err)
	}
}

func TestLoad_FaucetEnabledWithEmptyPemRejected(t *testing.T) {
	body := strings.Replace(validConfig, "Enabled = false\nPemPath = \"\"", "Enabled = true\nPemPath = \"\"", 1)
	_, err := Load(writeConfig(t, body))
	if err == nil || !strings.Contains(err.Error(), "Faucet.PemPath") {
		t.Fatalf("expected Faucet.PemPath error, got %v", err)
	}
}

func TestLoad_FaucetEnabledWithZeroAmountRejected(t *testing.T) {
	body := strings.Replace(validConfig,
		"Enabled = false\nPemPath = \"\"\nAmountPerAccount = \"0\"",
		"Enabled = true\nPemPath = \"./walletKey.pem\"\nAmountPerAccount = \"0\"",
		1)
	_, err := Load(writeConfig(t, body))
	if err == nil || !strings.Contains(err.Error(), "Faucet.AmountPerAccount") {
		t.Fatalf("expected Faucet.AmountPerAccount error, got %v", err)
	}
}

func TestLoad_FaucetDisabledIgnoresEmptyPem(t *testing.T) {
	// Default valid config has Enabled=false and PemPath="" — this must
	// not be a validation failure. Validate by reading back the file.
	cfg, err := Load(writeConfig(t, validConfig))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Faucet.Enabled {
		t.Fatalf("Faucet.Enabled: got true, want false")
	}
	if cfg.Faucet.PemPath != "" {
		t.Fatalf("Faucet.PemPath: got %q, want empty", cfg.Faucet.PemPath)
	}
}

func TestLoad_MalformedTOMLIsError(t *testing.T) {
	if _, err := Load(writeConfig(t, "this is not toml")); err == nil {
		t.Fatalf("expected error on malformed input")
	}
}
