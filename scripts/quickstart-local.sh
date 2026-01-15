#!/usr/bin/env bash
set -euo pipefail

# Feather local quickstart: build, start, seed, and verify — all in one command.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN="${ROOT_DIR}/bin/feather"
CONFIG="${ROOT_DIR}/configs/feather-dev.yaml"
PID_FILE="${ROOT_DIR}/.feather.pid"
LOG_FILE="${ROOT_DIR}/.feather-quickstart.log"
BASE_URL="http://localhost:8080"
START_TIME=$(date +%s)

cleanup() {
  if [ -f "${PID_FILE}" ]; then
    local pid
    pid=$(<"${PID_FILE}")
    kill "${pid}" 2>/dev/null || true
    rm -f "${PID_FILE}"
  fi
  rm -f "${LOG_FILE}"
}
trap cleanup EXIT

# Build
echo "▸ Building Feather..."
(cd "${ROOT_DIR}" && make build --quiet 2>&1) || {
  echo "Build failed. Run 'make doctor' to check prerequisites." >&2
  exit 1
}

# Check if ports are free
for port in 8080 50051 9090 8081; do
  if lsof -i ":${port}" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "Port ${port} is already in use. Free it and retry." >&2
    echo "  Hint: lsof -i :${port} to see what's using it" >&2
    exit 1
  fi
done

# Start server in background, capture logs for diagnostics
echo "▸ Starting Feather with dev config..."
"${BIN}" -config "${CONFIG}" > "${LOG_FILE}" 2>&1 &
SERVER_PID=$!
echo "${SERVER_PID}" > "${PID_FILE}"

# Wait for health with progress indicator
printf "▸ Waiting for server to be ready"
HEALTHY=false
for i in $(seq 1 60); do
  if curl -sf "${BASE_URL}/health" >/dev/null 2>&1; then
    HEALTHY=true
    break
  fi
  if ! kill -0 "${SERVER_PID}" 2>/dev/null; then
    echo ""
    echo "Server exited unexpectedly. Last log output:" >&2
    tail -20 "${LOG_FILE}" 2>/dev/null >&2 || true
    rm -f "${PID_FILE}"
    exit 1
  fi
  printf "."
  sleep 0.5
done
echo ""

if [ "${HEALTHY}" != "true" ]; then
  echo "Server did not become healthy in 30s. Last log output:" >&2
  tail -20 "${LOG_FILE}" 2>/dev/null >&2 || true
  cleanup
  exit 1
fi

# Seed demo data
echo "▸ Seeding demo data..."
curl -sS -X POST "${BASE_URL}/v1/features" \
  -H "Content-Type: application/json" \
  -d '{
    "entity_key": "user:123",
    "features": {
      "click_count": 15,
      "purchase_total": 245.50,
      "last_activity": "2024-01-15T10:30:00Z"
    }
  }' >/dev/null

# Verify
echo "▸ Verifying..."
RESPONSE=$(curl -sS "${BASE_URL}/v1/features?entity=user:123&feature=click_count&feature=purchase_total")
if command -v jq >/dev/null 2>&1; then
  echo "${RESPONSE}" | jq .
else
  echo "${RESPONSE}"
fi

# Remove EXIT trap so server keeps running after script finishes
trap - EXIT
rm -f "${LOG_FILE}"

ELAPSED=$(( $(date +%s) - START_TIME ))
echo ""
echo "✅ Feather is running! (ready in ${ELAPSED}s)"
echo ""
echo "  # Health check"
echo "  curl ${BASE_URL}/health"
echo ""
echo "  # Get features"
echo "  curl '${BASE_URL}/v1/features?entity=user:123&feature=click_count'"
echo ""
echo "  # Store features"
echo "  curl -X POST ${BASE_URL}/v1/features -H 'Content-Type: application/json' \\"
echo "    -d '{\"entity_key\":\"user:456\",\"features\":{\"click_count\":7}}'"
echo ""
echo "  # Walk through all API features interactively"
echo "  make explore"
echo ""
echo "  # Stop the server"
echo "  make stop-dev   # or: kill ${SERVER_PID}"
