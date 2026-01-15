#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${FEATHER_URL:-http://localhost:8080}"

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required but was not found." >&2
  exit 1
fi

if ! curl -sf "${BASE_URL}/health" >/dev/null; then
  echo "Feather is not reachable at ${BASE_URL}. Start the server and retry." >&2
  exit 1
fi

echo "Seeding features..."
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

echo "Fetching features..."
curl -sS "${BASE_URL}/v1/features?entity=user:123&feature=click_count&feature=purchase_total"
echo ""
