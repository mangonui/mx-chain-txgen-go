package submit

import (
	"context"
	"fmt"
	"time"

	"github.com/mangonui/mx-chain-txgen-go/config"
	"github.com/mangonui/mx-chain-txgen-go/proxy"
)

// Poller waits for a transaction hash to reach a terminal state. Used by
// scenarios that must gate on tx execution before proceeding (erc20 deploy
// must return a contract address; esdt issue must return a token
// identifier).
type Poller struct {
	prx      *proxy.Client
	interval time.Duration
	timeout  time.Duration
}

// NewPoller configures a poller from the global Polling section.
func NewPoller(prx *proxy.Client, cfg config.PollingConfig) *Poller {
	return &Poller{
		prx:      prx,
		interval: time.Duration(cfg.IntervalMilliseconds) * time.Millisecond,
		timeout:  time.Duration(cfg.TimeoutSeconds) * time.Second,
	}
}

// TerminalStatus enumerates the strings the proxy/observers return in the
// /transaction/{hash}/process-status endpoint that mean "stop waiting".
var TerminalStatus = map[string]bool{
	"success":   true,
	"executed":  true,
	"fail":      true,
	"failed":    true,
	"invalid":   true,
	"pending":   false, // explicit non-terminal
	"received":  false,
}

// WaitForExecution polls the proxy until the transaction reaches a
// terminal status or the configured timeout elapses. The status string is
// returned so callers can distinguish success from failure. The context
// can be cancelled externally to abort the poll.
func (p *Poller) WaitForExecution(ctx context.Context, hash string) (string, error) {
	deadline := time.Now().Add(p.timeout)
	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timeout waiting for tx %s after %s", hash, p.timeout)
		}
		status, err := p.prx.SDK.ProcessTransactionStatus(ctx, hash)
		if err == nil {
			s := string(status)
			if terminal, known := TerminalStatus[s]; known && terminal {
				return s, nil
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(p.interval):
		}
	}
}
