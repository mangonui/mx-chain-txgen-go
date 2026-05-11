package scenarios

import (
	coreData "github.com/multiversx/mx-chain-core-go/data/transaction"
)

// buildTx assembles a FrontendTransaction with the fields that every
// scenario shares (gas, chain ID, version, options) pulled from req and
// comp, plus the scenario-specific (sender, receiver, nonce, value,
// data) supplied per call.
//
// Pass nil for data on pure move-balance txs (basic scenario); empty
// byte slices would serialise as `"data": ""` in the JSON body and the
// chain accepts both, but nil keeps the request body smaller.
//
// This is a thin DRY layer over the struct literal — keeping it here
// (rather than in submit/) avoids importing submit from scenarios where
// the only thing scenarios know about submit is the Job binding.
func buildTx(
	req Request,
	comp *Components,
	sender string,
	receiver string,
	nonce uint64,
	value string,
	data []byte,
) *coreData.FrontendTransaction {
	return &coreData.FrontendTransaction{
		Nonce:    nonce,
		Value:    value,
		Sender:   sender,
		Receiver: receiver,
		GasPrice: req.GasPrice,
		GasLimit: req.GasLimit,
		Data:     data,
		ChainID:  comp.NetConfig.ChainID,
		Version:  req.Version,
		Options:  req.Options,
	}
}
