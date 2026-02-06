#!/usr/bin/env bash
set -euo pipefail

# End-to-end smoke test that validates the README API examples work.
# Requires a running Feather server with the dev schema (make run-dev).

BASE_URL="${FEATHER_URL:-http://localhost:8080}"
PASS=0
FAIL=0

# Pre-flight: verify server is reachable before running tests
if ! curl -sf --connect-timeout 3 "${BASE_URL}/health" >/dev/null 2>&1; then
  echo "❌ Server not reachable at ${BASE_URL}" >&2
  echo "" >&2
  echo "Start the server first:" >&2
  echo "  make run-dev       # foreground" >&2
  echo "  make dev-start     # background" >&2
  exit 1
fi

pass() { PASS=$((PASS + 1)); echo "  ✅ $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  ❌ $1" >&2; }

check_json() {
  local desc="$1" url="$2" expected="$3"
  local body
  body=$(curl -sS "$url") || { fail "${desc}: request failed"; return; }
  if echo "$body" | grep -q "$expected"; then
    pass "$desc"
  else
    fail "${desc}: expected '${expected}' in response"
    echo "    got: ${body}" >&2
  fi
}

echo "Feather smoke tests (${BASE_URL})"
echo ""

# --- Health endpoints ---
echo "Health endpoints"
check_json "GET /health" "${BASE_URL}/health" '"status":"healthy"'
check_json "GET /live"   "${BASE_URL}/live"   '"status"'
check_json "GET /ready"  "${BASE_URL}/ready"  '"status"'

# --- Store features ---
echo ""
echo "Feature CRUD"
STORE_RESP=$(curl -sS -X POST "${BASE_URL}/v1/features" \
  -H "Content-Type: application/json" \
  -d '{
    "entity_key": "user:smoke-test",
    "features": {
      "click_count": 42,
      "purchase_total": 99.99,
      "last_activity": "2024-06-15T12:00:00Z"
    }
  }') || { fail "POST /v1/features: request failed"; }
if echo "$STORE_RESP" | grep -q '"success":true'; then
  pass "POST /v1/features (store)"
else
  fail "POST /v1/features (store): ${STORE_RESP}"
fi

# --- Retrieve features ---
check_json "GET /v1/features (single)" \
  "${BASE_URL}/v1/features?entity=user:smoke-test&feature=click_count" \
  '"click_count"'

# --- Batch retrieval ---
BATCH_RESP=$(curl -sS -X POST "${BASE_URL}/v1/features/batch" \
  -H "Content-Type: application/json" \
  -d '{
    "entities": ["user:smoke-test"],
    "features": ["click_count", "purchase_total"]
  }') || { fail "POST /v1/features/batch: request failed"; }
if echo "$BATCH_RESP" | grep -q '"click_count"' && echo "$BATCH_RESP" | grep -q '"purchase_total"'; then
  pass "POST /v1/features/batch"
else
  fail "POST /v1/features/batch: ${BATCH_RESP}"
fi

# --- Point-in-time query ---
PIT_RESP=$(curl -sS "${BASE_URL}/v1/features/history?entity=user:smoke-test&feature=click_count&as_of=2099-01-01T00:00:00Z") || { fail "GET /v1/features/history: request failed"; }
if echo "$PIT_RESP" | grep -q '"click_count"' || echo "$PIT_RESP" | grep -q '"success":true'; then
  pass "GET /v1/features/history (point-in-time)"
else
  fail "GET /v1/features/history: ${PIT_RESP}"
fi

# --- Vector search ---
echo ""
echo "Vector search"
# Create index
CREATE_RESP=$(curl -sS -X POST "${BASE_URL}/v1/vectors" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "smoke_test_idx",
    "dimension": 4,
    "distance_type": "cosine"
  }') || { fail "POST /v1/vectors (create): request failed"; }
if echo "$CREATE_RESP" | grep -q 'smoke_test_idx'; then
  pass "POST /v1/vectors (create index)"
else
  fail "POST /v1/vectors (create index): ${CREATE_RESP}"
fi

# Upsert vectors
UPSERT_RESP=$(curl -sS -X POST "${BASE_URL}/v1/vectors/smoke_test_idx/upsert" \
  -H "Content-Type: application/json" \
  -d '{
    "vectors": [
      {"id": "v1", "vector": [0.1, 0.2, 0.3, 0.4]},
      {"id": "v2", "vector": [0.9, 0.8, 0.7, 0.6]}
    ]
  }') || { fail "POST /v1/vectors/.../upsert: request failed"; }
if echo "$UPSERT_RESP" | grep -qiE 'success|upserted|ok|"count"'; then
  pass "POST /v1/vectors/.../upsert"
else
  fail "POST /v1/vectors/.../upsert: ${UPSERT_RESP}"
fi

# Search
SEARCH_RESP=$(curl -sS -X POST "${BASE_URL}/v1/vectors/smoke_test_idx/search" \
  -H "Content-Type: application/json" \
  -d '{
    "vector": [0.1, 0.2, 0.3, 0.4],
    "top_k": 2
  }') || { fail "POST /v1/vectors/.../search: request failed"; }
if echo "$SEARCH_RESP" | grep -q '"v1"'; then
  pass "POST /v1/vectors/.../search"
else
  fail "POST /v1/vectors/.../search: ${SEARCH_RESP}"
fi

# Clean up vector index
curl -sS -X DELETE "${BASE_URL}/v1/vectors/smoke_test_idx" >/dev/null 2>&1 || true

# --- Schema ---
echo ""
echo "Schema"
check_json "GET /v1/schema/groups" "${BASE_URL}/v1/schema/groups" 'user_features'

# --- Summary ---
echo ""
TOTAL=$((PASS + FAIL))
echo "Results: ${PASS}/${TOTAL} passed"
if [ "${FAIL}" -ne 0 ]; then
  echo "${FAIL} test(s) failed." >&2
  exit 1
fi
echo "All smoke tests passed."
