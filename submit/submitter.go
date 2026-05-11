package submit

import (
	"context"
	"fmt"

	coreData "github.com/multiversx/mx-chain-core-go/data/transaction"
	sdkBlockchain "github.com/multiversx/mx-sdk-go/blockchain"
	sdkCryptoProvider "github.com/multiversx/mx-sdk-go/blockchain/cryptoProvider"
	sdkBuilders "github.com/multiversx/mx-sdk-go/builders"
	sdkInteractors "github.com/multiversx/mx-sdk-go/interactors"

	"github.com/mangonui/mx-chain-txgen-go/accounts"
	"github.com/mangonui/mx-chain-txgen-go/proxy"
)

// BunchSize is the default number of signed transactions submitted in one
// proxy round-trip. Matches mx-chain-go scripts/testnet/variables.sh's
// implicit batching expectations.
const BunchSize = 100

// Submitter is the thin layer that signs and submits transactions through
// mx-sdk-go's TransactionInteractor. One Submitter is shared across all
// scenarios; scenarios call SignAndSubmit with their already-populated tx
// list and Submitter handles per-account signing + batched dispatch.
type Submitter struct {
	prx     *proxy.Client
	builder sdkInteractors.TxBuilder
}

// New wires the Submitter against a proxy client and the SDK's default tx
// builder. The crypto provider's signer matches the chain's ed25519 scheme.
func New(prx *proxy.Client) (*Submitter, error) {
	builder, err := sdkBuilders.NewTxBuilder(sdkCryptoProvider.NewSigner())
	if err != nil {
		return nil, fmt.Errorf("new tx builder: %w", err)
	}
	return &Submitter{prx: prx, builder: builder}, nil
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
	hashes, err := ti.SendTransactionsAsBunch(ctx, BunchSize)
	if err != nil {
		return nil, fmt.Errorf("send transactions: %w", err)
	}
	return hashes, nil
}

// unwrapProxyForSDK returns the underlying SDK proxy implementation that
// NewTransactionInteractor expects. Our proxy.Client wraps this so the rest
// of the codebase deals with a slim local interface, but the interactor
// constructor needs the concrete *blockchain.proxy through its full
// interface contract.
func (s *Submitter) unwrapProxyForSDK() sdkBlockchain.Proxy {
	// The proxy.Client.SDK field is the SDK's *proxy (returned by
	// blockchain.NewProxy). The SDK's Proxy interface is a superset of our
	// local SDKProxy interface, so a direct type assertion is safe.
	if sdk, ok := s.prx.SDK.(sdkBlockchain.Proxy); ok {
		return sdk
	}
	// Defensive: in tests SDK may be a fake. The interactor needs the
	// full surface; tests that exercise SignAndSubmit must supply a real
	// proxy.
	panic("submitter: proxy.Client.SDK does not implement sdkBlockchain.Proxy")
}
