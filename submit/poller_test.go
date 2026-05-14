package submit

import (
	"context"
	"errors"
	"testing"
	"time"

	coreData "github.com/multiversx/mx-chain-core-go/data/transaction"
	sdkCore "github.com/multiversx/mx-sdk-go/core"
	sdkData "github.com/multiversx/mx-sdk-go/data"

	"github.com/mangonui/mx-chain-txgen-go/config"
	"github.com/mangonui/mx-chain-txgen-go/proxy"
)

type sdkProxyStub struct {
	statuses []coreData.TxStatus
	err      error
	calls    int
}

func (stub *sdkProxyStub) GetNetworkConfig(context.Context) (*sdkData.NetworkConfig, error) {
	return nil, nil
}

func (stub *sdkProxyStub) GetNetworkStatus(context.Context, uint32) (*sdkData.NetworkStatus, error) {
	return nil, nil
}

func (stub *sdkProxyStub) GetAccount(context.Context, sdkCore.AddressHandler) (*sdkData.Account, error) {
	return nil, nil
}

func (stub *sdkProxyStub) SendTransaction(context.Context, *coreData.FrontendTransaction) (string, error) {
	return "", nil
}

func (stub *sdkProxyStub) SendTransactions(context.Context, []*coreData.FrontendTransaction) ([]string, error) {
	return nil, nil
}

func (stub *sdkProxyStub) ProcessTransactionStatus(context.Context, string) (coreData.TxStatus, error) {
	stub.calls++
	if stub.err != nil {
		return "", stub.err
	}
	if len(stub.statuses) == 0 {
		return coreData.TxStatus("pending"), nil
	}
	status := stub.statuses[0]
	stub.statuses = stub.statuses[1:]
	return status, nil
}

func (stub *sdkProxyStub) GetTransactionInfoWithResults(context.Context, string) (*sdkData.TransactionInfo, error) {
	return nil, nil
}

func (stub *sdkProxyStub) IsInterfaceNil() bool {
	return stub == nil
}

func TestPollerWaitForExecutionShouldReturnTerminalStatus(t *testing.T) {
	sdkStub := &sdkProxyStub{statuses: []coreData.TxStatus{"pending", "success"}}
	poller := NewPoller(&proxy.Client{SDK: sdkStub}, config.PollingConfig{
		IntervalMilliseconds: 1,
		TimeoutSeconds:       1,
	})

	status, err := poller.WaitForExecution(context.Background(), "tx")

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if status != "success" {
		t.Fatalf("expected success status, got %q", status)
	}
	if sdkStub.calls != 2 {
		t.Fatalf("expected 2 status calls, got %d", sdkStub.calls)
	}
}

func TestPollerWaitForExecutionShouldHonorContextCancellation(t *testing.T) {
	sdkStub := &sdkProxyStub{err: errors.New("temporary")}
	poller := NewPoller(&proxy.Client{SDK: sdkStub}, config.PollingConfig{
		IntervalMilliseconds: 1,
		TimeoutSeconds:       1,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	status, err := poller.WaitForExecution(ctx, "tx")

	if status != "" {
		t.Fatalf("expected empty status, got %q", status)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestTerminalStatusShouldClassifyKnownStatuses(t *testing.T) {
	if !TerminalStatus["success"] {
		t.Fatal("success should be terminal")
	}
	if !TerminalStatus["executed"] {
		t.Fatal("executed should be terminal")
	}
	if !TerminalStatus["failed"] {
		t.Fatal("failed should be terminal")
	}
	if TerminalStatus["pending"] {
		t.Fatal("pending should not be terminal")
	}
	if TerminalStatus["received"] {
		t.Fatal("received should not be terminal")
	}

	_, known := TerminalStatus["unknown"]
	if known {
		t.Fatal("unknown status should not be present")
	}
}

func TestNewPollerShouldApplyDurations(t *testing.T) {
	poller := NewPoller(&proxy.Client{}, config.PollingConfig{
		IntervalMilliseconds: 25,
		TimeoutSeconds:       3,
	})

	if poller.interval != 25*time.Millisecond {
		t.Fatalf("expected 25ms interval, got %v", poller.interval)
	}
	if poller.timeout != 3*time.Second {
		t.Fatalf("expected 3s timeout, got %v", poller.timeout)
	}
}
