# Tier 1 — wiring smoke against the public MultiversX devnet

The goal of this example is to **validate that the txgen's HTTP wiring,
signing, faucet drain, and tx submission paths all function against a
live Supernova-class chain**, without bringing up a 16-process local
testnet.

It does **not** test the txgen at load. The public devnet is shared,
rate-limited infrastructure; one polite 10-tx batch is the right scope
here. For real load testing, see Tier 2 (local testnet) — separate
exercise.

## What this validates

| Layer | Verified by this exercise |
|---|---|
| SDK proxy client against a real gateway | ✓ |
| `/network/config` parse (chainID, MinGasPrice) | ✓ |
| Faucet PEM load + ed25519 signing | ✓ |
| Per-account nonce sync via parallel `GetAccount` | ✓ |
| Faucet drain → 10 funded pool accounts | ✓ |
| `basic` scenario submission via `/transaction/send-multiple` | ✓ |
| Cross-shard receiver picker against 3-shard devnet | ✓ |
| Tx hash → terminal status round-trip | ✓ |
| Build info / logging middleware / `/healthz` | ✓ |

## What this deliberately skips

- ERC20 + ESDT scenarios. They work the same way; adding them is a
  matter of supplying a wasm and re-running. The point of Tier 1 is to
  prove the wire, not exhaustively re-test every scenario shape.
- Sustained TPS. Public devnet rate-limits aggressively; sustained load
  here would be both throttled and abuse-of-shared-resource.

## Pre-flight (one-time)

### 1. Install prerequisites

```bash
sudo apt install jq curl                       # smoke.sh needs jq
pipx install multiversx-sdk-cli                # mxpy: wallet generation
```

(or `pip install multiversx-sdk-cli` if you don't have pipx)

### 2. Generate a wallet PEM

```bash
cd examples/devnet-smoke
mxpy wallet new --format pem --outfile wallet.pem
mxpy wallet convert --infile wallet.pem --in-format pem --out-format address-bech32
# -> prints your erd1... address; copy it for the next step
```

### 3. Fund it from the devnet faucet

Visit <https://devnet-wallet.multiversx.com>, sign in (or create a Keystore
session), and use the in-UI faucet to drip ~5 EGLD onto the `erd1...`
address you generated. One drip per 24 h is enough for many smoke
runs — the test costs ~0.001 EGLD total.

Verify funding:

```bash
curl -s https://devnet-gateway.multiversx.com/address/<erd1...> | jq '.data.account.balance'
# should print a big string like "5000000000000000000" (5 EGLD in atomic units)
```

### 4. Build the txgen binary

From the repo root:

```bash
make build
```

That produces `cmd/txgen/txgen` with version/commit/build-date injected
via ldflags.

## Run the smoke test

```bash
cd examples/devnet-smoke
../../cmd/txgen/txgen --config ./config.toml &
TXGEN_PID=$!

# Wait a few seconds for the faucet drain to finish (you'll see it in the txgen log)
sleep 30

./smoke.sh

# When done:
kill $TXGEN_PID
```

You can also run the txgen in the foreground in one shell and
`./smoke.sh` in another.

## Expected output

```
[smoke] 1/4 verifying txgen liveness at http://localhost:7951
[smoke] 2/4 fetching txgen /status
{
  "scenarios": ["basic", "erc20", "esdt"],
  "poolSize": 10,
  "build": { "version": "...", "commit": "...", "buildDate": "..." }
}
[smoke] 3/4 submitting 10 basic txs (mixed shards)
[smoke]     -> txgen accepted 10 / 10 transactions
[smoke]     -> sample hashes: 1a2b... .. 9f8e...
[smoke] 4/4 polling devnet-gateway for terminal status of each tx (timeout 180s)
[smoke] ==== per-tx results ====
  OK   1a2b... -> success
  OK   9f8e... -> success
  ...
[smoke] ==== summary ====
[smoke]   total submitted: 10
[smoke]   executed OK:     10
[smoke]   failed/invalid:  0
[smoke]   explorer:        https://devnet-explorer.multiversx.com/transactions/1a2b...
[smoke] Tier 1 wiring smoke PASSED.
```

End-to-end runtime: ~30–60 s (10 txs × ~6 s per round × 1–2 rounds to
reach `success`, plus 30 s faucet-drain at txgen startup).

## Failure modes you might hit (and what they mean)

| Symptom | Diagnosis |
|---|---|
| `faucet: read faucet pem ./wallet.pem: no such file` | Step 2 not done — generate the wallet |
| `faucet drain: ... insufficientBalance` | Step 3 not done — faucet the wallet |
| `faucet: last drain tx ... terminal status "fail"` | Devnet gateway is degraded; retry in ~5 min |
| smoke.sh: `429 Too Many Requests` | You're running too fast / from a shared NAT — wait, then retry |
| Some txs return `failed` instead of `success` | Likely "insufficientBalance" on a pool account; the faucet drain didn't land for that account. Check the txgen log for drain hashes. |
| Polling timeout | Devnet has cross-shard miniblock delay — raise `POLL_TIMEOUT_SECONDS=300 ./smoke.sh` |

## What "PASSED" actually proves

When this exits 0, you have **independently verified** every
high-uncertainty assumption in the gap analysis except sustained-load
behaviour:

- Address derivation matches what the chain computes ✓
- Tx signing produces signatures the chain accepts ✓
- ChainID, MinGasPrice, MinTransactionVersion read dynamically from
  the chain are correct for the txgen's tx construction ✓
- Cross-shard receiver picking lands receivers in the expected shards ✓
- Faucet drain pre-funds the pool ✓
- `/transaction/send-multiple` JSON contract is what the proxy accepts ✓
- The chain returns hashes that resolve to terminal status via
  `/transaction/{hash}/process-status` ✓
- Build-info ldflags injection survives `go build` and is visible at
  runtime ✓

After Tier 1 passes, the only honest remaining unknown is:
**does this hold up at sustained TPS?** That requires Tier 2 (local
testnet), where you can hammer it without abusing shared infrastructure.

## Cleanup

```bash
rm -rf state-devnet                # pool persistence — safe to delete
# wallet.pem stays — you'll reuse it; just re-faucet for the next smoke run
```
