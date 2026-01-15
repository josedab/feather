#!/usr/bin/env bash
set -euo pipefail

# Interactive API exploration script for Feather.
# Walks through core API operations with labeled output.
#
# Usage:
#   make explore           (requires a running server: make run-dev)

BASE_URL="${FEATHER_URL:-http://localhost:8080}"

# Colors (disable if not a terminal)
if [ -t 1 ]; then
  BOLD="\033[1m"
  DIM="\033[2m"
  GREEN="\033[32m"
  CYAN="\033[36m"
  RESET="\033[0m"
else
  BOLD="" DIM="" GREEN="" CYAN="" RESET=""
fi

section() { printf "\n${BOLD}${CYAN}▸ %s${RESET}\n" "$1"; }
run_curl() {
  local desc="$1"; shift
  printf "${DIM}  \$ curl %s${RESET}\n" "$*"
  local resp
  resp=$(curl -sS "$@") || { echo "  ❌ Request failed"; return 1; }
  echo "$resp" | python3 -m json.tool 2>/dev/null | sed 's/^/  /' || echo "  $resp"
  echo ""
}

# ── Preflight ────────────────────────────────────────────────────────
if ! curl -sf "${BASE_URL}/health" >/dev/null 2>&1; then
  echo "Feather is not reachable at ${BASE_URL}."
  echo ""
  echo "Start the server first:"
  echo "  make run-dev"
  echo ""
  echo "Then re-run:"
  echo "  make explore"
  exit 1
fi

echo ""
printf "${BOLD}Feather API Explorer${RESET}\n"
printf "Server: ${BASE_URL}\n"

# ── Health ────────────────────────────────────────────────────────
section "Health check"
run_curl "Health" "${BASE_URL}/health"

# ── Store features for multiple entities ──────────────────────────
section "Storing features for 5 users"

for i in 1 2 3 4 5; do
  clicks=$((i * 10))
  total=$(echo "$i * 49.99" | bc)
  curl -sS -X POST "${BASE_URL}/v1/features" \
    -H "Content-Type: application/json" \
    -d "{
      \"entity_key\": \"user:${i}\",
      \"features\": {
        \"click_count\": ${clicks},
        \"purchase_total\": ${total},
        \"last_activity\": \"2024-0${i}-15T10:30:00Z\"
      }
    }" >/dev/null
  printf "  ✓ user:${i} (clicks=${clicks}, total=${total})\n"
done
echo ""

# ── Single entity retrieval ───────────────────────────────────────
section "Single entity retrieval"
run_curl "Get user:3" "${BASE_URL}/v1/features?entity=user:3&feature=click_count&feature=purchase_total"

# ── Batch retrieval ───────────────────────────────────────────────
section "Batch retrieval (3 users)"
printf "${DIM}  \$ curl -X POST ${BASE_URL}/v1/features/batch ...${RESET}\n"
BATCH_RESP=$(curl -sS -X POST "${BASE_URL}/v1/features/batch" \
  -H "Content-Type: application/json" \
  -d '{
    "entities": ["user:1", "user:3", "user:5"],
    "features": ["click_count", "purchase_total"]
  }')
echo "$BATCH_RESP" | python3 -m json.tool 2>/dev/null | sed 's/^/  /' || echo "  $BATCH_RESP"
echo ""

# ── Point-in-time query ──────────────────────────────────────────
section "Point-in-time query (as of 2024-03-01)"
run_curl "History" "${BASE_URL}/v1/features/history?entity=user:1&feature=click_count&as_of=2024-03-01T00:00:00Z"

# ── Vector search ─────────────────────────────────────────────────
section "Creating vector index"
printf "${DIM}  \$ curl -X POST ${BASE_URL}/v1/vectors ...${RESET}\n"
CREATE_RESP=$(curl -sS -X POST "${BASE_URL}/v1/vectors" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "explore_demo",
    "dimension": 4,
    "distance_type": "cosine"
  }')
echo "$CREATE_RESP" | python3 -m json.tool 2>/dev/null | sed 's/^/  /' || echo "  $CREATE_RESP"
echo ""

section "Upserting vectors"
printf "${DIM}  \$ curl -X POST ${BASE_URL}/v1/vectors/explore_demo/upsert ...${RESET}\n"
UPSERT_RESP=$(curl -sS -X POST "${BASE_URL}/v1/vectors/explore_demo/upsert" \
  -H "Content-Type: application/json" \
  -d '{
    "vectors": [
      {"id": "prod_a", "vector": [0.1, 0.8, 0.3, 0.5]},
      {"id": "prod_b", "vector": [0.9, 0.2, 0.7, 0.1]},
      {"id": "prod_c", "vector": [0.2, 0.7, 0.4, 0.6]}
    ]
  }')
echo "$UPSERT_RESP" | python3 -m json.tool 2>/dev/null | sed 's/^/  /' || echo "  $UPSERT_RESP"
echo ""

section "Similarity search (top 2)"
printf "${DIM}  \$ curl -X POST ${BASE_URL}/v1/vectors/explore_demo/search ...${RESET}\n"
SEARCH_RESP=$(curl -sS -X POST "${BASE_URL}/v1/vectors/explore_demo/search" \
  -H "Content-Type: application/json" \
  -d '{
    "vector": [0.15, 0.75, 0.35, 0.55],
    "top_k": 2
  }')
echo "$SEARCH_RESP" | python3 -m json.tool 2>/dev/null | sed 's/^/  /' || echo "  $SEARCH_RESP"
echo ""

# Cleanup vector index
curl -sS -X DELETE "${BASE_URL}/v1/vectors/explore_demo" >/dev/null 2>&1 || true

# ── Schema listing ────────────────────────────────────────────────
section "Schema listing"
run_curl "Schema" "${BASE_URL}/v1/schema/groups"

# ── Done ──────────────────────────────────────────────────────────
printf "${GREEN}${BOLD}✅ All examples complete.${RESET}\n"
echo ""
echo "Next steps:"
echo "  • Full API reference: docs/api-reference.md"
echo "  • Python SDK:         pip install -e sdk/python/ && python sdk/python/quickstart/quickstart.py"
echo "  • Go SDK:             cd sdk/go/feather/quickstart && go run main.go"
echo "  • Run smoke tests:    make smoke-test"
echo ""
