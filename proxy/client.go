package proxy

import (
	"context"
	"fmt"
	"time"

	coreData "github.com/multiversx/mx-chain-core-go/data/transaction"
	sdkBlockchain "github.com/multiversx/mx-sdk-go/blockchain"
	sdkCore "github.com/multiversx/mx-sdk-go/core"
	sdkData "github.com/multiversx/mx-sdk-go/data"

	"github.com/mangonui/mx-chain-txgen-go/config"
)

// SDKProxy is the subset of mx-sdk-go's Proxy interface that the txgen needs.
// Defining it locally keeps test substitution simple and avoids importing the
// SDK's internal interactors interface in callers.
type SDKProxy interface {
	GetNetworkConfig(ctx context.Context) (*sdkData.NetworkConfig, error)
	GetNetworkStatus(ctx context.Context, shardID uint32) (*sdkData.NetworkStatus, error)
	GetAccount(ctx context.Context, address sdkCore.AddressHandler) (*sdkData.Account, error)
	SendTransaction(ctx context.Context, tx *coreData.FrontendTransaction) (string, error)
	SendTransactions(ctx context.Context, txs []*coreData.FrontendTransaction) ([]string, error)
	ProcessTransactionStatus(ctx context.Context, hash string) (coreData.TxStatus, error)
	IsInterfaceNil() bool
}

// Client is the txgen's proxy facade. Wraps mx-sdk-go's *proxy.
type Client struct {
	SDK SDKProxy
}

// New creates a configured proxy client.
func New(cfg config.ProxyConfig) (*Client, error) {
	args := sdkBlockchain.ArgsProxy{
		ProxyURL:            cfg.URL,
		Client:              nil,
		SameScState:         false,
		ShouldBeSynced:      false,
		FinalityCheck:       cfg.FinalityCheck,
		AllowedDeltaToFinal: 10,
		CacheExpirationTime: time.Duration(cfg.CacheExpirationSeconds) * time.Second,
		EntityType:          sdkCore.Proxy,
	}
	p, err := sdkBlockchain.NewProxy(args)
	if err != nil {
		return nil, fmt.Errorf("new proxy: %w", err)
	}
	return &Client{SDK: p}, nil
}
