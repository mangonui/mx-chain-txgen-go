package accounts

import (
	"encoding/json"
	"fmt"
	"io"
)

// GenesisBalance is one row in the mx-chain-deploy-go initialBalances.json
// input. Fields mirror the schema mx-chain-deploy-go/cmd/filegen consumes
// when seeding a local testnet's genesis state.
//
// If filegen changes the schema in a future release, update this struct.
type GenesisBalance struct {
	PubKey       string            `json:"pubkey"`
	Supply       string            `json:"supply"`
	Balance      string            `json:"balance"`
	StakingValue string            `json:"stakingvalue"`
	Delegation   GenesisDelegation `json:"delegation"`
}

// GenesisDelegation is the optional delegation pointer in a genesis row.
// Set Address="" and Value="0" for non-delegated EOAs.
type GenesisDelegation struct {
	Address string `json:"address"`
	Value   string `json:"value"`
}

// EmitInitialBalances writes one GenesisBalance row per account in JSON
// array form. The balance/supply is the same for every account. Operators
// pipe this file into mx-chain-deploy-go/cmd/filegen as the initial
// balances input before running config.sh.
func EmitInitialBalances(w io.Writer, pool *Pool, perAccountBalance string) error {
	rows := make([]GenesisBalance, 0, pool.Len())
	for _, acc := range pool.All() {
		rows = append(rows, GenesisBalance{
			PubKey:       acc.Bech32,
			Supply:       perAccountBalance,
			Balance:      perAccountBalance,
			StakingValue: "0",
			Delegation: GenesisDelegation{
				Address: "",
				Value:   "0",
			},
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rows); err != nil {
		return fmt.Errorf("emit genesis balances: %w", err)
	}
	return nil
}
