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

### `POST /transaction/send-multiple` — drive load

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

### `GET /status` — diagnostic info

Returns the enabled scenarios and the current account-pool size.

### `GET /stats` — rolling submitted-TPS report

When `Stats.EnableTPSSampler = true`, every successful
`/transaction/send-multiple` response increments a per-scenario counter
in an in-memory ring. `/stats` returns the rolled-up TPS over `1m`,
`5m`, and `1h` rolling windows:

```json
{
  "data": {
    "enabled": true,
    "windows": [
      {
        "window": "1m",
        "windowSeconds": 60,
        "totalTxs": 1500,
        "overallTPS": 25.0,
        "perScenarioTxs": { "basic": 1200, "esdt": 300 },
        "perScenarioTPS": { "basic": 20.0, "esdt": 5.0 }
      },
      { "window": "5m", "...": "..." },
      { "window": "1h", "...": "..." }
    ]
  },
  "code": "successful"
}
```

When the sampler is disabled, `data.enabled` is `false` and `data.windows`
is empty.

**This is *submitted* TPS — what the txgen pushed at the proxy, not what
the chain included in a block.** Honest *included* TPS requires polling
`/network/status/{shard}` and tracking per-shard tx counts on the chain
side; that path is a planned follow-up. Until then, included-TPS must be
read directly off the proxy/explorer.

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
writes them to `state/accounts.json`. Subsequent runs load from disk unless
`--regenerate-accounts` (or the upstream-compatible alias `--new-accounts`)
is passed.

## Funding the pool

The accounts the txgen generates start with **zero balance** on the chain
and cannot send any transaction without being funded first. There is one
production path:

### Runtime faucet drain (recommended, matches upstream wiring)

`mx-chain-go/scripts/testnet/include/config.sh` already copies
`walletKey.pem` (the pre-funded "mint wallet" emitted by
`mx-chain-deploy-go/filegen`) into the txgen's config directory. Enable
the faucet in `config.toml` to drain from it at boot:

```toml
[Faucet]
    Enabled = true
    PemPath = "./walletKey.pem"
    AmountPerAccount = "1000000000000000000000"  # 1000 EGLD
    GasPrice = 0                                  # 0 = use MinGasPrice
    GasLimit = 50000
```

On startup the txgen will:

1. Load the PEM, derive the faucet's bech32 + shard ID.
2. Sync the faucet's on-chain nonce.
3. Emit one move-balance tx per pool account, with the faucet as sender.
4. Wait for the *last* tx in the bunch to reach terminal status — the
   chain processes faucet txs in strict nonce order so the last
   determines the inclusion frontier.
5. Re-sync every pool member's nonce from the chain (now non-zero
   because the funding txs landed).

Only after all that does the HTTP server start accepting scenario
requests.

### About `--emit-genesis-balances`

The `--emit-genesis-balances` flag remains for non-MultiversX downstream
bootstraps that *can* take an external initial-balance JSON. **It does
not integrate with stock `mx-chain-deploy-go/cmd/filegen`** — filegen
generates the genesis state algorithmically from `total-supply` and
`node-price` parameters and does not accept an external initial-balance
file. Use this flag only if you are wiring into a custom genesis
generator (e.g. a DRWA-specific tool).

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

## Quick start: validate against public devnet

The fastest way to convince yourself the wiring works end-to-end is the
Tier 1 smoke under [`examples/devnet-smoke/`](examples/devnet-smoke/).
That sends 10 native-EGLD transfers via the public devnet gateway from
a faucet-funded wallet and polls each tx hash to terminal status.

```bash
make build
cd examples/devnet-smoke
# follow the README to create wallet.pem and faucet-fund it
../../cmd/txgen/txgen --config ./config.toml &
./smoke.sh
```

The example is scoped to a single 10-tx batch so it does not abuse the
shared public devnet infrastructure. For real load testing, run a local
testnet (see below).

## License

Apache 2.0 — see [LICENSE](LICENSE).
