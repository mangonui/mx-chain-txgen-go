package submit

import (
	"context"
	"fmt"

	coreData "github.com/multiversx/mx-chain-core-go/data/transaction"
	sdkCryptoProvider "github.com/multiversx/mx-sdk-go/blockchain/cryptoProvider"
	sdkBuilders "github.com/multiversx/mx-sdk-go/builders"
	sdkInteractors "github.com/multiversx/mx-sdk-go/interactors"

	"github.com/mangonui/mx-chain-txgen-go/accounts"
	"github.com/mangonui/mx-chain-txgen-go/proxy"
)

// DefaultBunchSize is the fallback batch size when a caller constructs
// a Submitter with bunchSize <= 0. Matches the upstream txgen's
// implicit batching expectation.
const DefaultBunchSize = 100

// Submitter is the thin layer that signs and submits transactions through
// mx-sdk-go's TransactionInteractor. One Submitter is shared across all
// scenarios; scenarios call SignAndSubmit with their already-populated tx
// list and Submitter handles per-account signing + batched dispatch.
type Submitter struct {
	prx       *proxy.Client
	builder   sdkInteractors.TxBuilder
	bunchSize int
}

// New wires the Submitter against a proxy client and the SDK's default
// tx builder. bunchSize <= 0 selects DefaultBunchSize.
func New(prx *proxy.Client, bunchSize int) (*Submitter, error) {
	builder, err := sdkBuilders.NewTxBuilder(sdkCryptoProvider.NewSigner())
	if err != nil {
		return nil, fmt.Errorf("new tx builder: %w", err)
	}
	if bunchSize <= 0 {
		bunchSize = DefaultBunchSize
	}
	return &Submitter{prx: prx, builder: builder, bunchSize: bunchSize}, nil
}

// Job binds a transaction to the account that should sign it. Scenarios
// emit a list of Jobs; the Submitter assigns nonces, signs, batches.
type Job struct {
	Sender *accounts.Account
	Tx     *coreData.FrontendTransaction
}

// SignAndSubmit signs every job's transaction with the bound sender and
// flushes batches of BunchSize through the proxy. Returns the slice of
// resulting transaction hashes (in input order, modulo proxy ordering).
//
// The interactor is recreated per call rather than long-lived because
// AddTransaction in mx-sdk-go accumulates state internally; isolating that
// state to one invocation avoids cross-batch contamination if a scenario
// emits multiple batches.
func (s *Submitter) SignAndSubmit(ctx context.Context, jobs []Job) ([]string, error) {
	if len(jobs) == 0 {
		return nil, nil
	}
	ti, err := sdkInteractors.NewTransactionInteractor(s.unwrapProxyForSDK(), s.builder)
	if err != nil {
		return nil, fmt.Errorf("new tx interactor: %w", err)
	}
	for i, job := range jobs {
		if err := ti.ApplyUserSignature(job.Sender.CryptoHolder, job.Tx); err != nil {
			return nil, fmt.Errorf("sign job %d: %w", i, err)
		}
		ti.AddTransaction(job.Tx)
	}
	hashes, err := ti.SendTransactionsAsBunch(ctx, s.bunchSize)
	if err != nil {
		return nil, fmt.Errorf("send transactions: %w", err)
	}
	return hashes, nil
}

// unwrapProxyForSDK returns the underlying SDK proxy implementation
// that NewTransactionInteractor expects.
//
// We deliberately assert against sdkInteractors.Proxy (5 methods —
// exactly what the interactor needs) rather than sdkBlockchain.Proxy
// (8 methods). The latter is broken in mx-sdk-go v1.4.8: its
// FilterLogs signature returns []string but the concrete *proxy.
// FilterLogs returns []*transaction.Events, so the dynamic-type check
// `s.prx.SDK.(sdkBlockchain.Proxy)` fails at runtime. Asserting
// against the smaller (correct) interactors.Proxy avoids this entirely.
//
// In tests SDK may be a fake; that fake must satisfy interactors.Proxy
// (i.e. implement the five methods listed in the interactor interface).
func (s *Submitter) unwrapProxyForSDK() sdkInteractors.Proxy {
	if sdk, ok := s.prx.SDK.(sdkInteractors.Proxy); ok {
		return sdk
	}
	panic("submitter: proxy.Client.SDK does not implement sdkInteractors.Proxy")
}
