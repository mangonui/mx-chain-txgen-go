#!/usr/bin/env bash
#
# Tier-1+ live verifier for the ESDT issuance → token-identifier
# extraction path.
#
# Submits ONE `esdt issue` request to the local txgen, lets it execute
# on devnet, then verifies that:
#   1. The txgen returned a tokenIdentifier in Result.Extra
#   2. That identifier matches "<TICKER>-<6hex>" format
#   3. Querying the chain directly for the same tx returns the same id
#
# Cost on devnet: 0.05 EGLD (the chain's fixed issuance fee).

set -euo pipefail

TXGEN_URL="${TXGEN_URL:-http://localhost:7951}"
DEVNET_GATEWAY="${DEVNET_GATEWAY:-https://devnet-gateway.multiversx.com}"
POLL_INTERVAL_SECONDS="${POLL_INTERVAL_SECONDS:-3}"
POLL_TIMEOUT_SECONDS="${POLL_TIMEOUT_SECONDS:-180}"

need() { command -v "$1" >/dev/null 2>&1 || { echo "ERROR: $1 not found" >&2; exit 1; }; }
need curl
need jq

log() { echo "[esdt-smoke] $*"; }
fail() { echo "[esdt-smoke] FAIL: $*" >&2; exit 1; }

log "1/4 verifying txgen liveness"
curl -sf "$TXGEN_URL/healthz" >/dev/null || fail "txgen not reachable on $TXGEN_URL"

log "2/4 submitting esdt issue"
# Issuance: value=0.05 EGLD (chain fee), gasLimit=60M (issuance is expensive),
# scAddress unused on issue, data="issue" routes the sub-command.
payload=$(jq -n \
    '{value: "50000000000000000", numOfTxs: 1, gasPrice: 1000000000,
      gasLimit: 60000000, destination: "mixed", recallNonce: false,
      scenario: "esdt", data: "issue"}')

response=$(curl -sf -X POST "$TXGEN_URL/transaction/send-multiple" \
    -H "Content-Type: application/json" -d "$payload") \
    || fail "/transaction/send-multiple returned error"

code=$(echo "$response" | jq -r '.code')
[[ "$code" == "successful" ]] || fail "code=$code body=$response"

txgen_token_id=$(echo "$response" | jq -r '.data.extra.tokenIdentifier // empty')
[[ -n "$txgen_token_id" ]] || fail "txgen did not return Extra.tokenIdentifier: $response"
hash=$(echo "$response" | jq -r '.data.txsHashes."0"')
log "    -> txgen extracted tokenIdentifier=$txgen_token_id from tx $hash"

log "3/4 validating tokenIdentifier format"
# Expected shape: <TICKER>-<6 hex chars>. TICKER is 3-10 alnum chars.
if ! [[ "$txgen_token_id" =~ ^[A-Z0-9]{3,10}-[a-f0-9]{6}$ ]]; then
    fail "tokenIdentifier $txgen_token_id does not match <TICKER>-<6hex> format"
fi
log "    -> format OK"

log "4/4 independent verification against devnet gateway"
# Fetch the same tx directly and walk its logs to confirm the chain
# reports the same identifier. This proves our extractor logic matches
# the chain's emission, not just the SDK's parser.
deadline=$(( $(date +%s) + POLL_TIMEOUT_SECONDS ))
chain_token_id=""
while (( $(date +%s) < deadline )); do
    info=$(curl -sf "$DEVNET_GATEWAY/transaction/$hash?withResults=true" 2>/dev/null || echo "")
    if [[ -n "$info" ]]; then
        # First topic of the issuance log event, base64-decoded.
        chain_token_id=$(echo "$info" | jq -r '
            .data.transaction.logs.events[]
            | select(.identifier == "issue" or
                     .identifier == "issueSemiFungible" or
                     .identifier == "issueNonFungible")
            | .topics[0] // empty
        ' | head -1 | base64 -d 2>/dev/null || echo "")
        [[ -n "$chain_token_id" ]] && break
    fi
    sleep "$POLL_INTERVAL_SECONDS"
done

[[ -n "$chain_token_id" ]] || fail "could not extract tokenIdentifier from chain within ${POLL_TIMEOUT_SECONDS}s"
log "    -> chain reports tokenIdentifier=$chain_token_id"

if [[ "$txgen_token_id" != "$chain_token_id" ]]; then
    fail "MISMATCH: txgen='$txgen_token_id' vs chain='$chain_token_id'"
fi

log "==== summary ===="
log "  tokenIdentifier:    $txgen_token_id"
log "  txgen and chain:    MATCH"
log "  tx explorer:        https://devnet-explorer.multiversx.com/transactions/$hash"
log "  token explorer:     https://devnet-explorer.multiversx.com/tokens/$txgen_token_id"
log "Tier 1+ ESDT issuance smoke PASSED."
