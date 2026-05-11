package scenarios

import (
	"context"
	"fmt"
	"math/big"

	coreData "github.com/multiversx/mx-chain-core-go/data/transaction"

	"github.com/mangonui/mx-chain-txgen-go/submit"
)

// ESDTScenario implements the upstream txgen's three ESDT sub-commands:
// issue / mint / transfer. Sub-commands are dispatched on Request.Data,
// matching the shell driver txgen-esdt.sh.
//
// ESDT uses MultiversX-native primitives: the issuance call targets the
// system smart contract; mint and transfer use the built-in functions
// ESDTLocalMint and ESDTTransfer routed by the node itself.
type ESDTScenario struct{}

// NewESDT constructs the scenario. Stateless.
func NewESDT() *ESDTScenario { return &ESDTScenario{} }

// Name implements Scenario.
func (e *ESDTScenario) Name() string { return "esdt" }

// esdtIssuerIndex is the pool member that owns the issued token.
const esdtIssuerIndex = 0

// esdtSystemSCAddress is the well-known bech32 of the ESDT issuance system
// smart contract. Issuance transactions are sent here.
const esdtSystemSCAddress = "erd1qqqqqqqqqqqqqqqpqqqqqqqqlllllls8a5w6u"

// defaultTokenName / defaultTokenTicker / defaultDecimals are the params
// used when /transaction/send-multiple body omits override values. The
// upstream shell drivers do not customise these fields.
const (
	defaultTokenName    = "WorkloadTok"
	defaultTokenTicker  = "WRK"
	defaultDecimals     = 6
	issuanceCostEGLD    = "50000000000000000" // 0.05 EGLD
	defaultInitialMint  = "1000000000000000000000000"
	defaultMintPerAccnt = "1000000000"
	defaultXferAmount   = "1"
)

// Run implements Scenario.
func (e *ESDTScenario) Run(ctx context.Context, req Request, comp *Components) (*Result, error) {
	switch req.Data {
	case "issue":
		return e.issue(ctx, req, comp)
	case "mint":
		return e.mint(ctx, req, comp)
	case "transfer", "":
		return e.transfer(ctx, req, comp)
	default:
		return nil, fmt.Errorf("esdt: unknown sub-command %q (expected issue|mint|transfer)", req.Data)
	}
}

func (e *ESDTScenario) issue(ctx context.Context, req Request, comp *Components) (*Result, error) {
	issuer := comp.Pool.Index(esdtIssuerIndex)
	if issuer == nil {
		return nil, fmt.Errorf("esdt issue: pool index %d not present", esdtIssuerIndex)
	}
	initialSupply, ok := new(big.Int).SetString(defaultInitialMint, 10)
	if !ok {
		return nil, fmt.Errorf("esdt issue: invalid default supply")
	}
	data := EncodeData(
		"issue",
		HexString(defaultTokenName),
		HexString(defaultTokenTicker),
		HexBigInt(initialSupply),
		HexUint64(defaultDecimals),
	)
	if req.RecallNonce {
		if err := comp.Nonces.Refresh(ctx, issuer.AddressHandler, issuer.Bech32); err != nil {
			return nil, err
		}
	}
	tx := &coreData.FrontendTransaction{
		Nonce:    comp.Nonces.Next(issuer.Bech32),
		Value:    issuanceCostEGLD,
		Sender:   issuer.Bech32,
		Receiver: esdtSystemSCAddress,
		GasPrice: req.GasPrice,
		GasLimit: req.GasLimit,
		Data:     []byte(data),
		ChainID:  comp.NetConfig.ChainID,
		Version:  req.Version,
		Options:  req.Options,
	}
	hashes, err := comp.Submitter.SignAndSubmit(ctx, []submit.Job{{Sender: issuer, Tx: tx}})
	if err != nil {
		return nil, err
	}
	if len(hashes) == 0 {
		return nil, fmt.Errorf("esdt issue: submitter returned no hashes")
	}
	status, err := comp.Poller.WaitForExecution(ctx, hashes[0])
	if err != nil {
		return nil, fmt.Errorf("esdt issue: %w", err)
	}
	if status != "success" && status != "executed" {
		return nil, fmt.Errorf("esdt issue: terminal status %q", status)
	}
	return &Result{
		NumSent: 1,
		Hashes:  hashes,
		// The actual token identifier (e.g. WRK-abc123) is emitted in the
		// SCR logs from the issuance call. Callers extract it via
		// /transaction/{hash} or scan smartContractResults; this stub is
		// the same hand-off the upstream txgen makes to its shell driver.
		Extra: map[string]any{"tokenIdentifierHint": "READ_FROM_TX_LOGS"},
	}, nil
}

func (e *ESDTScenario) mint(ctx context.Context, req Request, comp *Components) (*Result, error) {
	issuer := comp.Pool.Index(esdtIssuerIndex)
	if issuer == nil {
		return nil, fmt.Errorf("esdt mint: pool index %d not present", esdtIssuerIndex)
	}
	tokenID := req.SCAddress // shell driver re-uses the scAddress field for the token identifier
	if tokenID == "" {
		return nil, fmt.Errorf("esdt mint: token identifier required (in scAddress field)")
	}
	mintAmount, ok := new(big.Int).SetString(defaultMintPerAccnt, 10)
	if !ok {
		return nil, fmt.Errorf("esdt mint: invalid default mint amount")
	}
	jobs := make([]submit.Job, 0, comp.Pool.Len())
	// ESDTLocalMint is called by the issuer; the chain credits the
	// caller's balance, then a subsequent ESDTTransfer moves the freshly
	// minted amount to each pool member. We collapse mint+seed into a
	// single ESDTTransfer per member by relying on the issuer's initial
	// supply created at issuance time. This matches what the upstream
	// txgen's mint sub-command actually achieves (seeding holders).
	for _, acc := range comp.Pool.All() {
		if acc.Index == esdtIssuerIndex {
			continue // issuer already holds the bag
		}
		data := EncodeData(
			"ESDTTransfer",
			HexString(tokenID),
			HexBigInt(mintAmount),
		)
		tx := &coreData.FrontendTransaction{
			Nonce:    comp.Nonces.Next(issuer.Bech32),
			Value:    "0",
			Sender:   issuer.Bech32,
			Receiver: acc.Bech32,
			GasPrice: req.GasPrice,
			GasLimit: req.GasLimit,
			Data:     []byte(data),
			ChainID:  comp.NetConfig.ChainID,
			Version:  req.Version,
			Options:  req.Options,
		}
		jobs = append(jobs, submit.Job{Sender: issuer, Tx: tx})
	}
	hashes, err := comp.Submitter.SignAndSubmit(ctx, jobs)
	if err != nil {
		return nil, err
	}
	return &Result{NumSent: len(hashes), Hashes: hashes}, nil
}

func (e *ESDTScenario) transfer(ctx context.Context, req Request, comp *Components) (*Result, error) {
	tokenID := req.SCAddress
	if tokenID == "" {
		return nil, fmt.Errorf("esdt transfer: token identifier required (in scAddress field)")
	}
	if req.NumOfTxs <= 0 {
		return nil, fmt.Errorf("numOfTxs must be > 0")
	}
	xferAmount, ok := new(big.Int).SetString(defaultXferAmount, 10)
	if !ok {
		return nil, fmt.Errorf("esdt transfer: invalid default amount")
	}
	if req.Value != "" && req.Value != "0" {
		if v, ok := new(big.Int).SetString(req.Value, 10); ok {
			xferAmount = v
		}
	}
	jobs := make([]submit.Job, 0, req.NumOfTxs)
	for i := 0; i < req.NumOfTxs; i++ {
		sender := comp.Pool.Random()
		if sender == nil {
			return nil, fmt.Errorf("esdt transfer: pool empty")
		}
		receiver := comp.Pool.PickReceiver(sender.ShardID, req.Destination, comp.Shards.NumShards())
		if receiver == nil {
			return nil, fmt.Errorf("esdt transfer: could not pick receiver")
		}
		if req.RecallNonce {
			if err := comp.Nonces.Refresh(ctx, sender.AddressHandler, sender.Bech32); err != nil {
				return nil, err
			}
		}
		data := EncodeData(
			"ESDTTransfer",
			HexString(tokenID),
			HexBigInt(xferAmount),
		)
		tx := &coreData.FrontendTransaction{
			Nonce:    comp.Nonces.Next(sender.Bech32),
			Value:    "0",
			Sender:   sender.Bech32,
			Receiver: receiver.Bech32,
			GasPrice: req.GasPrice,
			GasLimit: req.GasLimit,
			Data:     []byte(data),
			ChainID:  comp.NetConfig.ChainID,
			Version:  req.Version,
			Options:  req.Options,
		}
		jobs = append(jobs, submit.Job{Sender: sender, Tx: tx})
	}
	hashes, err := comp.Submitter.SignAndSubmit(ctx, jobs)
	if err != nil {
		return nil, err
	}
	return &Result{NumSent: len(hashes), Hashes: hashes}, nil
}
