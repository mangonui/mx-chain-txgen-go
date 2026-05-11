package scenarios

import (
	"context"

	sdkData "github.com/multiversx/mx-sdk-go/data"

	"github.com/mangonui/mx-chain-txgen-go/accounts"
	"github.com/mangonui/mx-chain-txgen-go/config"
	"github.com/mangonui/mx-chain-txgen-go/nonces"
	"github.com/mangonui/mx-chain-txgen-go/proxy"
	"github.com/mangonui/mx-chain-txgen-go/shards"
	"github.com/mangonui/mx-chain-txgen-go/submit"
)

// Request is the normalized form of an HTTP load-test request. The API
// layer parses the raw JSON into this struct before dispatching to a
// scenario; this keeps scenarios free of HTTP plumbing.
//
// Version / Options are applied to every transaction the scenario
// builds. They are normalised by the API layer so scenarios can assume
// Version is non-zero (defaults to 1) and Options is the raw bitmask.
type Request struct {
	Value       string
	NumOfTxs    int
	GasPrice    uint64
	GasLimit    uint64
	Destination shards.Destination
	RecallNonce bool
	Data        string
	SCAddress   string
	Version     uint32
	Options     uint32
}

// Result is what a scenario returns to the API layer.
type Result struct {
	NumSent int
	Hashes  []string
	// Extra carries scenario-specific outputs back to the caller.
	// erc20 deploy returns {"scAddress": "erd1..."}.
	// esdt issue returns {"tokenIdentifier": "MYTOKEN-abc123"}.
	Extra map[string]any
}

// Components is the dependency bundle passed to every scenario. Sharing
// one bundle keeps scenario signatures stable as new components are
// introduced (e.g. a stats sampler in the future).
type Components struct {
	Pool      *accounts.Pool
	Nonces    *nonces.Tracker
	Shards    *shards.Coordinator
	Submitter *submit.Submitter
	Poller    *submit.Poller
	Proxy     *proxy.Client
	NetConfig *sdkData.NetworkConfig
	Cfg       *config.Config
}

// Scenario is the contract every load-test scenario implements.
type Scenario interface {
	Name() string
	Run(ctx context.Context, req Request, comp *Components) (*Result, error)
}
