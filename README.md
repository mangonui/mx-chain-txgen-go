# mx-chain-txgen-go

Synthetic transaction-generation service for MultiversX local testnets.

A re-implementation of the historically private `multiversx/mx-chain-txgen-go`
tool, exposing the same HTTP contract on port `7951` so the existing
`scripts/testnet/txgen-*.sh` drivers in `mx-chain-go` work unchanged. Built
against MultiversX Supernova (`mx-chain-go v2.0.0`) using the public
`mx-sdk-go`.

## What it does

- Boots a pool of N pre-funded EOAs (genesis injection or runtime faucet).
- Maintains in-memory `(address → nonce)` for high-throughput signing without
  per-tx nonce roundtrips.
- Maps addresses to shards using the same algorithm as `mx-chain-go`.
- Exposes one HTTP endpoint that drives load by scenario:
  - `basic` — native EGLD transfers between accounts.
  - `erc20` — deploys an ERC20-style wasm contract, mints, then transfer-floods.
  - `esdt` — issues a native ESDT token, mints, then transfer-floods.
- Submits batched signed transactions through `mx-chain-proxy-go`.
- Polls `network/status` for honest **included** TPS (distinct from
  *submitted* TPS).

## Build

```bash
make tidy
make build
# binary at cmd/txgen/txgen
```

Requires Go 1.23+.

## Run standalone

```bash
./cmd/txgen/txgen --config config/config.toml
```

## Run inside the mx-chain-go testnet scripts

In `mx-chain-go/scripts/testnet/variables.sh`, point `TXGENDIR` at this repo
and enable txgen:

```bash
export USE_TXGEN=1
export TXGENDIR="<path-to-this-repo>/cmd/txgen"
```

The original scripts clone the upstream private repo via SSH; for this
implementation, clone manually as a sibling of `mx-chain-go`:

```
mx-chain-go/
mx-chain-deploy-go/
mx-chain-proxy-go/
mx-chain-txgen-go/      ← this repo
```

Then run `./prerequisites.sh && ./config.sh && ./start.sh` as usual. The
`txgen-basic.sh`, `txgen-erc20.sh`, `txgen-esdt.sh` shell drivers work
unchanged.

## HTTP contract

`POST http://localhost:7951/transaction/send-multiple`

Body:

```json
{
  "value": 1,
  "numOfTxs": 250,
  "gasPrice": 1000000000,
  "gasLimit": 50000,
  "destination": "mixed",
  "recallNonce": false,
  "scenario": "basic",
  "data": "",
  "scAddress": ""
}
```

| Field | Type | Meaning |
|---|---|---|
| `value` | int / string | EGLD value (in atomic units) per generated tx |
| `numOfTxs` | int | Number of transactions to emit in this batch |
| `gasPrice` | uint64 | Gas price in atomic units |
| `gasLimit` | uint64 | Gas limit per tx |
| `destination` | string | `same_shard` / `cross_shard` / `mixed` |
| `recallNonce` | bool | If true, GET nonce from proxy before each batch; if false, use the in-memory tracker |
| `scenario` | string | `basic` / `erc20` / `esdt` |
| `data` | string | Scenario sub-command. ERC20: `deploy`/`mint`/`transfer`. ESDT: `issue`/`mint`/`transfer`. Empty for `basic`. |
| `scAddress` | string | Contract address for ERC20 transfer mode (returned by the prior `deploy` response) |

Response:

```json
{
  "data": { "numOfSentTxs": 250, "txsHashes": { "0": "abc...", ... } },
  "error": "",
  "code": "successful"
}
```

## Scenario sub-commands

### `basic`

Single mode. Generates `numOfTxs` native EGLD transfers between random
accounts in the pool, respecting `destination` for cross-shard ratio.

### `erc20`

Three sub-commands, called in sequence by `txgen-erc20.sh`:

1. `deploy` — deploys the bundled ERC20 wasm to a designated deployer
   account, polls until the tx is `executed`, returns the contract
   address in `scAddress`.
2. `mint` — calls the contract's `mint` endpoint to credit each pool
   account with an initial balance.
3. `transfer` — floods `transfer` calls between pool accounts.

### `esdt`

Three sub-commands:

1. `issue` — calls the ESDT system smart-contract at
   `00000000000000000500...02ffff` to issue a token; polls until the
   issuance log surfaces the token identifier.
2. `mint` — built-in `ESDTLocalMint` calls to credit pool accounts.
3. `transfer` — built-in `ESDTTransfer` floods between pool accounts.

## Account pool

On first boot the service generates N keypairs (configurable, default 1000),
writes them to `state/accounts.pem` and `state/pool.json`. Subsequent runs
load from disk unless `--regenerate-accounts` is passed.

For local testnets, **genesis injection** is the recommended funding mode:

```bash
./cmd/txgen/txgen --emit-genesis-balances > /tmp/initialBalances.json
# Then feed that file into mx-chain-deploy-go/cmd/filegen before config.sh
```

For long-running testnets, **runtime faucet** mode pulls EGLD from a
configured faucet account and broadcasts top-up txs at startup.

## Build / runtime topology assumed

```
seednode  →  validators × N  →  observers × N
                                       ↓
                                  mx-chain-proxy-go :7950
                                       ↓
                                       │ /transaction/send-multiple
                                       │ /address/{addr}/nonce
                                       │ /network/status/{shard}
                                       │ /transaction/{hash}
                                       ↓
                                  this txgen :7951
                                       ↑
                          txgen-basic.sh / -erc20.sh / -esdt.sh
                          (curl loops driving the scenarios)
```

## Migration notes for DRWA

This implementation is deliberately upstream-MultiversX-shaped. To migrate
to DRWA:

1. Rename the module path from `github.com/mangonui/mx-chain-txgen-go` to
   the DRWA path; update imports.
2. Add DRWA-specific scenarios in `scenarios/` (e.g. `rwa.go` for parcel
   tokenisation, `mrv.go` for synthetic oracle ingest).
3. Repoint the proxy client at the DRWA proxy if it diverges from
   upstream `mx-chain-proxy-go` shape.
4. Update the genesis-injection helper if DRWA changes the
   `initialBalances.json` schema.

The three baseline scenarios (`basic`, `erc20`, `esdt`) remain useful as
parity benchmarks against the upstream chain even after the DRWA-specific
scenarios are added.

## License

Apache 2.0 — see [LICENSE](LICENSE).
