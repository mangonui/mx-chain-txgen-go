package scenarios

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"

	coreData "github.com/multiversx/mx-chain-core-go/data/transaction"

	"github.com/mangonui/mx-chain-txgen-go/shards"
	"github.com/mangonui/mx-chain-txgen-go/submit"
)

// ERC20Scenario implements the upstream txgen's three ERC20 sub-commands:
// deploy / mint / transfer. Sub-commands are dispatched on the Request.Data
// field; the bundled txgen-erc20.sh shell driver issues them in that order.
//
// Deploy requires a compiled wasm at Cfg.ERC20.WasmPath. The wasm must be
// a contract with a `transfer(receiver, amount)` view and an
// owner-callable `mint(receiver, amount)` operation. Any ERC20-style wasm
// produced by the mx-sdk-rs `erc20` example contracts satisfies this.
type ERC20Scenario struct{}

// NewERC20 constructs the scenario. Stateless across requests.
func NewERC20() *ERC20Scenario { return &ERC20Scenario{} }

// Name implements Scenario.
func (e *ERC20Scenario) Name() string { return "erc20" }

// deployerAddress is the conventional deployer for the ERC20 wasm.
// We pin it to pool index 0 so subsequent mint calls have a known signer.
const erc20DeployerIndex = 0

// zeroAddressBech32 is the destination for contract deploys — mx-chain-go
// computes the resulting contract address from sender + nonce when receiver
// is the all-zero address.
const zeroAddressBech32 = "erd1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq6gq4hu"

// defaultERC20VMType is the WASM VM identifier (0500). Mandatory in the
// deploy data payload immediately after the wasm hex.
const defaultERC20VMType = "0500"

// defaultCodeMetadata gives the contract upgradeable + payable defaults.
// 0x0500 = upgradeable + payable on smart contract calls.
const defaultCodeMetadata = "0500"

// Run implements Scenario.
func (e *ERC20Scenario) Run(ctx context.Context, req Request, comp *Components) (*Result, error) {
	switch req.Data {
	case "deploy":
		return e.deploy(ctx, req, comp)
	case "mint":
		return e.mint(ctx, req, comp)
	case "transfer", "":
		return e.transfer(ctx, req, comp)
	default:
		return nil, fmt.Errorf("erc20: unknown sub-command %q (expected deploy|mint|transfer)", req.Data)
	}
}

// deploy issues one deploy transaction, polls it to terminal status, and
// returns the resulting contract address in Result.Extra["scAddress"]. The
// driver script captures that address and threads it into subsequent
// mint/transfer calls.
func (e *ERC20Scenario) deploy(ctx context.Context, req Request, comp *Components) (*Result, error) {
	deployer := comp.Pool.Index(erc20DeployerIndex)
	if deployer == nil {
		return nil, fmt.Errorf("erc20: pool index %d not present", erc20DeployerIndex)
	}
	wasmBytes, err := os.ReadFile(comp.Cfg.ERC20.WasmPath)
	if err != nil {
		return nil, fmt.Errorf("erc20: read wasm %s: %w", comp.Cfg.ERC20.WasmPath, err)
	}
	initialSupply := big.NewInt(0).Mul(big.NewInt(int64(comp.Pool.Len())), big.NewInt(1_000_000_000))
	data := EncodeData(
		hex.EncodeToString(wasmBytes),
		defaultERC20VMType,
		defaultCodeMetadata,
		HexBigInt(initialSupply),
	)
	if req.RecallNonce {
		if err := comp.Nonces.Refresh(ctx, deployer.AddressHandler, deployer.Bech32); err != nil {
			return nil, err
		}
	}
	tx := &coreData.FrontendTransaction{
		Nonce:    comp.Nonces.Next(deployer.Bech32),
		Value:    "0",
		Sender:   deployer.Bech32,
		Receiver: zeroAddressBech32,
		GasPrice: req.GasPrice,
		GasLimit: req.GasLimit,
		Data:     []byte(data),
		ChainID:  comp.NetConfig.ChainID,
		Version:  1,
	}
	hashes, err := comp.Submitter.SignAndSubmit(ctx, []submit.Job{{Sender: deployer, Tx: tx}})
	if err != nil {
		return nil, err
	}
	if len(hashes) == 0 {
		return nil, fmt.Errorf("erc20 deploy: submitter returned no hashes")
	}
	status, err := comp.Poller.WaitForExecution(ctx, hashes[0])
	if err != nil {
		return nil, fmt.Errorf("erc20 deploy: %w", err)
	}
	if status != "success" && status != "executed" {
		return nil, fmt.Errorf("erc20 deploy: terminal status %q", status)
	}
	scAddress, err := shards.ComputeContractAddress(deployer.PublicKey, tx.Nonce, comp.Shards.NumShards())
	if err != nil {
		return nil, fmt.Errorf("erc20 deploy: derive contract address: %w", err)
	}
	return &Result{
		NumSent: 1,
		Hashes:  hashes,
		Extra:   map[string]any{"scAddress": scAddress},
	}, nil
}

// mint generates one mint call per account in the pool. The deployer is
// the implicit signer for all of them (mint authority).
func (e *ERC20Scenario) mint(ctx context.Context, req Request, comp *Components) (*Result, error) {
	if req.SCAddress == "" {
		return nil, fmt.Errorf("erc20 mint: scAddress is required")
	}
	deployer := comp.Pool.Index(erc20DeployerIndex)
	if deployer == nil {
		return nil, fmt.Errorf("erc20 mint: pool index %d not present", erc20DeployerIndex)
	}
	mintAmount := big.NewInt(1_000_000_000)
	jobs := make([]submit.Job, 0, comp.Pool.Len())
	for _, acc := range comp.Pool.All() {
		data := EncodeData(
			"mint",
			HexBytes(acc.PublicKey),
			HexBigInt(mintAmount),
		)
		tx := &coreData.FrontendTransaction{
			Nonce:    comp.Nonces.Next(deployer.Bech32),
			Value:    "0",
			Sender:   deployer.Bech32,
			Receiver: req.SCAddress,
			GasPrice: req.GasPrice,
			GasLimit: req.GasLimit,
			Data:     []byte(data),
			ChainID:  comp.NetConfig.ChainID,
			Version:  1,
		}
		jobs = append(jobs, submit.Job{Sender: deployer, Tx: tx})
	}
	hashes, err := comp.Submitter.SignAndSubmit(ctx, jobs)
	if err != nil {
		return nil, err
	}
	return &Result{NumSent: len(hashes), Hashes: hashes}, nil
}

// transfer floods transfer calls between random pool members.
func (e *ERC20Scenario) transfer(ctx context.Context, req Request, comp *Components) (*Result, error) {
	if req.SCAddress == "" {
		return nil, fmt.Errorf("erc20 transfer: scAddress is required")
	}
	if req.NumOfTxs <= 0 {
		return nil, fmt.Errorf("numOfTxs must be > 0")
	}
	transferAmount := big.NewInt(1)
	if req.Value != "" && req.Value != "0" {
		if v, ok := new(big.Int).SetString(req.Value, 10); ok {
			transferAmount = v
		}
	}
	jobs := make([]submit.Job, 0, req.NumOfTxs)
	for i := 0; i < req.NumOfTxs; i++ {
		sender := comp.Pool.Random()
		if sender == nil {
			return nil, fmt.Errorf("erc20 transfer: pool is empty")
		}
		receiver := comp.Pool.PickReceiver(sender.ShardID, req.Destination, comp.Shards.NumShards())
		if receiver == nil {
			return nil, fmt.Errorf("erc20 transfer: could not pick receiver")
		}
		if req.RecallNonce {
			if err := comp.Nonces.Refresh(ctx, sender.AddressHandler, sender.Bech32); err != nil {
				return nil, err
			}
		}
		data := EncodeData(
			"transfer",
			HexBytes(receiver.PublicKey),
			HexBigInt(transferAmount),
		)
		tx := &coreData.FrontendTransaction{
			Nonce:    comp.Nonces.Next(sender.Bech32),
			Value:    "0",
			Sender:   sender.Bech32,
			Receiver: req.SCAddress,
			GasPrice: req.GasPrice,
			GasLimit: req.GasLimit,
			Data:     []byte(data),
			ChainID:  comp.NetConfig.ChainID,
			Version:  1,
		}
		jobs = append(jobs, submit.Job{Sender: sender, Tx: tx})
	}
	hashes, err := comp.Submitter.SignAndSubmit(ctx, jobs)
	if err != nil {
		return nil, err
	}
	return &Result{NumSent: len(hashes), Hashes: hashes}, nil
}

