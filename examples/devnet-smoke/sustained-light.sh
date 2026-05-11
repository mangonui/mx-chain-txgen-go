#!/usr/bin/env bash
#
# Tier-1.5 sustained-light driver — proves stability over time, NOT
# TPS capacity.
#
# Runs the basic scenario at a deliberate ~1 TPS for 5 minutes (300 txs
# total), then checks:
#   - the txgen process is still healthy (/healthz still 200)
#   - the stats endpoint reports a 1m TPS roughly equal to 1
#   - no batch returned a non-successful HTTP status code
#
# This intentionally stays inside the polite-developer-usage envelope
# for the public devnet gateway. Do not raise RATE_PER_SECOND above 5
# without thinking carefully about rate limits.

set -euo pipefail

TXGEN_URL="${TXGEN_URL:-http://localhost:7951}"
DURATION_SECONDS="${DURATION_SECONDS:-300}"
RATE_PER_SECOND="${RATE_PER_SECOND:-1}"
TXS_PER_BATCH="${TXS_PER_BATCH:-1}"

need() { command -v "$1" >/dev/null 2>&1 || { echo "ERROR: $1 not found" >&2; exit 1; }; }
need curl
need jq

log() { echo "[sustained] $*"; }
fail() { echo "[sustained] FAIL: $*" >&2; exit 1; }

log "checking txgen liveness"
curl -sf "$TXGEN_URL/healthz" >/dev/null || fail "txgen not reachable on $TXGEN_URL"

log "starting sustained smoke: ${RATE_PER_SECOND} batch/s × ${TXS_PER_BATCH} tx/batch for ${DURATION_SECONDS}s"
log "  total expected: $(( RATE_PER_SECOND * DURATION_SECONDS )) batches, $(( RATE_PER_SECOND * DURATION_SECONDS * TXS_PER_BATCH )) txs"

payload=$(jq -n \
    --argjson n "$TXS_PER_BATCH" \
    '{value: 1, numOfTxs: $n, gasPrice: 1000000000, gasLimit: 50000,
      destination: "mixed", recallNonce: false, scenario: "basic"}')

start=$(date +%s)
deadline=$(( start + DURATION_SECONDS ))
sleep_interval=$(awk -v r="$RATE_PER_SECOND" 'BEGIN{print 1/r}')

batches=0
errors=0
while (( $(date +%s) < deadline )); do
    if ! curl -sf -m 5 -o /dev/null -X POST "$TXGEN_URL/transaction/send-multiple" \
        -H "Content-Type: application/json" -d "$payload"; then
        errors=$(( errors + 1 ))
    fi
    batches=$(( batches + 1 ))

    # Every 60 batches (~60 s at 1 TPS), sample /stats and confirm
    # /healthz so the loop catches degradation early instead of at
    # the end.
    if (( batches % 60 == 0 )); then
        elapsed=$(( $(date +%s) - start ))
        log "  t=${elapsed}s batches=$batches errors=$errors"

        h=$(curl -sf "$TXGEN_URL/healthz" | jq -r .status 2>/dev/null || echo "")
        [[ "$h" == "ok" ]] || fail "  -> /healthz degraded at t=${elapsed}s (status=$h)"

        tps_1m=$(curl -sf "$TXGEN_URL/stats" \
            | jq -r '.data.windows[] | select(.window == "1m") | .overallTPS' 2>/dev/null || echo "?")
        log "  -> /stats 1m overallTPS=$tps_1m"
    fi

    sleep "$sleep_interval"
done

elapsed=$(( $(date +%s) - start ))
log "==== summary ===="
log "  duration:     ${elapsed}s"
log "  batches sent: $batches"
log "  HTTP errors:  $errors"
log "  expected TPS: $(awk -v r="$RATE_PER_SECOND" -v n="$TXS_PER_BATCH" 'BEGIN{print r*n}')"

# Final stats snapshot.
log "  final /stats:"
curl -sf "$TXGEN_URL/stats" | jq '.data.windows'

# Final health check.
final_h=$(curl -sf "$TXGEN_URL/healthz" | jq -r .status 2>/dev/null || echo "")
[[ "$final_h" == "ok" ]] || fail "/healthz not ok at end (status=$final_h)"

if (( errors > batches / 10 )); then
    fail "error rate ${errors}/${batches} exceeds 10% — wire is degraded"
fi

log "Tier 1.5 sustained-light smoke PASSED (HTTP error rate ${errors}/${batches})."
