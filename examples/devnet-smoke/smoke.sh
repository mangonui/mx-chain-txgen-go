#!/usr/bin/env bash
#
# Tier-1 wiring-smoke driver for the public MultiversX devnet.
#
# Assumes:
#   - txgen binary running locally on :7951, configured per ./config.toml
#   - the configured Faucet.PemPath wallet has at least 2 EGLD on devnet
#
# What it does:
#   1. Confirms the txgen is alive (GET /healthz)
#   2. Posts ONE small basic-scenario batch (10 native-EGLD transfers)
#   3. Polls devnet-gateway directly for each tx hash until terminal
#   4. Prints a per-tx success/failure summary
#
# Deliberately scoped: 10 txs, ~one round of submission. No sustained
# load against the public gateway.

set -euo pipefail

TXGEN_URL="${TXGEN_URL:-http://localhost:7951}"
DEVNET_GATEWAY="${DEVNET_GATEWAY:-https://devnet-gateway.multiversx.com}"
NUM_TXS="${NUM_TXS:-10}"
POLL_INTERVAL_SECONDS="${POLL_INTERVAL_SECONDS:-3}"
POLL_TIMEOUT_SECONDS="${POLL_TIMEOUT_SECONDS:-180}"

need() {
    command -v "$1" >/dev/null 2>&1 || { echo "ERROR: $1 not found in PATH" >&2; exit 1; }
}
need curl
need jq

log() { echo "[smoke] $*"; }
fail() { echo "[smoke] FAIL: $*" >&2; exit 1; }

log "1/4 verifying txgen liveness at $TXGEN_URL"
health=$(curl -sf "$TXGEN_URL/healthz") || fail "txgen not reachable on $TXGEN_URL"
status=$(echo "$health" | jq -r .status)
[[ "$status" == "ok" ]] || fail "/healthz returned status=$status (want ok)"

log "2/4 fetching txgen /status"
curl -sf "$TXGEN_URL/status" | jq '.data | {scenarios, poolSize, build}'

log "3/4 submitting $NUM_TXS basic txs (mixed shards)"
payload=$(jq -n \
    --argjson n "$NUM_TXS" \
    '{value: 1, numOfTxs: $n, gasPrice: 1000000000, gasLimit: 50000,
      destination: "mixed", recallNonce: false, scenario: "basic"}')

response=$(curl -sf -X POST "$TXGEN_URL/transaction/send-multiple" \
    -H "Content-Type: application/json" -d "$payload") \
    || fail "/transaction/send-multiple failed"

sent=$(echo "$response" | jq -r '.data.numOfSentTxs')
code=$(echo "$response" | jq -r '.code')
[[ "$code" == "successful" ]] || fail "code=$code body=$response"
[[ "$sent" == "$NUM_TXS" ]] || fail "sent=$sent want=$NUM_TXS"
log "    -> txgen accepted $sent / $NUM_TXS transactions"

# Extract the tx hashes the proxy returned. Map shape: {"0": "hash0", "1": "hash1", ...}
mapfile -t hashes < <(echo "$response" | jq -r '.data.txsHashes | to_entries | .[].value')
[[ ${#hashes[@]} -eq "$NUM_TXS" ]] || fail "expected $NUM_TXS hashes, got ${#hashes[@]}"
log "    -> sample hashes: ${hashes[0]} .. ${hashes[-1]}"

log "4/4 polling devnet-gateway for terminal status of each tx (timeout ${POLL_TIMEOUT_SECONDS}s)"
deadline=$(( $(date +%s) + POLL_TIMEOUT_SECONDS ))

declare -A final_status
remaining=${#hashes[@]}

while (( remaining > 0 )); do
    now=$(date +%s)
    if (( now > deadline )); then
        fail "timeout reached with $remaining txs still pending"
    fi

    for h in "${hashes[@]}"; do
        [[ -n "${final_status[$h]+set}" ]] && continue
        s=$(curl -sf "$DEVNET_GATEWAY/transaction/$h/process-status" \
            | jq -r '.data.status // empty' 2>/dev/null || true)
        case "$s" in
            success|executed|fail|failed|invalid)
                final_status[$h]=$s
                remaining=$(( remaining - 1 ))
                ;;
        esac
    done
    (( remaining > 0 )) && sleep "$POLL_INTERVAL_SECONDS"
done

log "==== per-tx results ===="
ok=0; bad=0
for h in "${hashes[@]}"; do
    s=${final_status[$h]}
    case "$s" in
        success|executed) printf "  OK   %s -> %s\n" "$h" "$s"; ok=$(( ok+1 ));;
        *)                printf "  FAIL %s -> %s\n" "$h" "$s"; bad=$(( bad+1 ));;
    esac
done

log "==== summary ===="
log "  total submitted: $NUM_TXS"
log "  executed OK:     $ok"
log "  failed/invalid:  $bad"
log "  explorer:        https://devnet-explorer.multiversx.com/transactions/${hashes[0]}"

(( bad == 0 )) || exit 1
log "Tier 1 wiring smoke PASSED."
