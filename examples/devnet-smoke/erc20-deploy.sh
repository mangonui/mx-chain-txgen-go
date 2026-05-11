#!/usr/bin/env bash
#
# Tier-1+ live verifier for the ERC20 deploy → scAddress derivation
# path.
#
# Submits ONE `erc20 deploy` request to the local txgen and verifies
# that the locally-computed scAddress (from
# shards.ComputeContractAddress, which uses mx-sdk-go's
# blockchain.NewAddressGenerator) matches the address the chain
# assigns the deployed contract.
#
# Requires: ./contracts/erc20.wasm. See ../../contracts/README.md.
#
# Cost on devnet: ~0.0015 EGLD (deploy gas).

set -euo pipefail

TXGEN_URL="${TXGEN_URL:-http://localhost:7951}"
DEVNET_GATEWAY="${DEVNET_GATEWAY:-https://devnet-gateway.multiversx.com}"
POLL_INTERVAL_SECONDS="${POLL_INTERVAL_SECONDS:-3}"
POLL_TIMEOUT_SECONDS="${POLL_TIMEOUT_SECONDS:-180}"
WASM_PATH="${WASM_PATH:-../../contracts/erc20.wasm}"

need() { command -v "$1" >/dev/null 2>&1 || { echo "ERROR: $1 not found" >&2; exit 1; }; }
need curl
need jq

log() { echo "[erc20-smoke] $*"; }
fail() { echo "[erc20-smoke] FAIL: $*" >&2; exit 1; }

[[ -f "$WASM_PATH" ]] || fail "wasm not found at $WASM_PATH — see contracts/README.md to compile from mx-sdk-rs"

log "1/4 verifying txgen liveness"
curl -sf "$TXGEN_URL/healthz" >/dev/null || fail "txgen not reachable on $TXGEN_URL"

log "2/4 submitting erc20 deploy"
payload=$(jq -n \
    '{value: "0", numOfTxs: 1, gasPrice: 1000000000,
      gasLimit: 60000000, destination: "mixed", recallNonce: false,
      scenario: "erc20", data: "deploy"}')

response=$(curl -sf -X POST "$TXGEN_URL/transaction/send-multiple" \
    -H "Content-Type: application/json" -d "$payload") \
    || fail "/transaction/send-multiple returned error"

code=$(echo "$response" | jq -r '.code')
[[ "$code" == "successful" ]] || fail "code=$code body=$response"

txgen_sc=$(echo "$response" | jq -r '.data.extra.scAddress // empty')
[[ -n "$txgen_sc" ]] || fail "txgen did not return Extra.scAddress: $response"
hash=$(echo "$response" | jq -r '.data.txsHashes."0"')
log "    -> txgen computed scAddress=$txgen_sc from tx $hash"

log "3/4 validating bech32 shape"
# Contract addresses always start with "erd1qqqqqq..." (long run of leading q's
# because the public-key bytes start with many zeros after the VM-type pad).
if ! [[ "$txgen_sc" =~ ^erd1qqqqqq[a-z0-9]+$ ]]; then
    fail "scAddress $txgen_sc does not look like a contract address"
fi
log "    -> shape OK"

log "4/4 independent verification against devnet gateway"
# Walk the deploy tx's smart-contract-results for the spawned contract.
# The deployed contract receives 0 EGLD from the deployer at the system
# SC -> contract miniblock, and the SCR's receiver is the spawned
# contract address.
deadline=$(( $(date +%s) + POLL_TIMEOUT_SECONDS ))
chain_sc=""
while (( $(date +%s) < deadline )); do
    info=$(curl -sf "$DEVNET_GATEWAY/transaction/$hash?withResults=true" 2>/dev/null || echo "")
    if [[ -n "$info" ]]; then
        # The "SCDeploy" log event's first topic is the new contract address bytes.
        chain_sc=$(echo "$info" | jq -r '
            .data.transaction.logs.events[]
            | select(.identifier == "SCDeploy")
            | .address // empty
        ' | head -1)
        # Some chain versions emit address in a different topic position;
        # fall back to scanning SCRs for a receiver that is not the
        # deployer and not the system SC.
        if [[ -z "$chain_sc" ]]; then
            chain_sc=$(echo "$info" | jq -r '
                .data.transaction.smartContractResults // []
                | map(.receiver)
                | map(select(startswith("erd1qqqqqq")))
                | .[0] // empty
            ')
        fi
        [[ -n "$chain_sc" ]] && break
    fi
    sleep "$POLL_INTERVAL_SECONDS"
done

[[ -n "$chain_sc" ]] || fail "could not extract scAddress from chain within ${POLL_TIMEOUT_SECONDS}s"
log "    -> chain reports scAddress=$chain_sc"

if [[ "$txgen_sc" != "$chain_sc" ]]; then
    fail "MISMATCH: txgen='$txgen_sc' vs chain='$chain_sc' — address-derivation drift"
fi

log "==== summary ===="
log "  scAddress (local):  $txgen_sc"
log "  scAddress (chain):  $chain_sc"
log "  match:              YES"
log "  tx explorer:        https://devnet-explorer.multiversx.com/transactions/$hash"
log "  contract explorer:  https://devnet-explorer.multiversx.com/accounts/$txgen_sc"
log "Tier 1+ ERC20 deploy smoke PASSED."
