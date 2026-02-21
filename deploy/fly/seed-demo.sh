#!/usr/bin/env bash
# Seed the demo instance with sample fraud detection features
set -euo pipefail

FEATHER_URL="${FEATHER_URL:-http://localhost:8080}"

echo "🪶 Seeding demo data..."

# Store sample user features
for i in $(seq 1 10); do
  curl -s -X POST "${FEATHER_URL}/v1/features" \
    -H "Content-Type: application/json" \
    -d "{
      \"entity\": \"user:${i}\",
      \"features\": {
        \"age\": {\"value\": $((20 + RANDOM % 50)), \"timestamp\": 0},
        \"account_days\": {\"value\": $((30 + RANDOM % 1000)), \"timestamp\": 0},
        \"avg_transaction\": {\"value\": $((10 + RANDOM % 500)).$(($RANDOM % 100)), \"timestamp\": 0},
        \"risk_score\": {\"value\": 0.$((RANDOM % 100)), \"timestamp\": 0}
      }
    }" > /dev/null
done

echo "✅ Seeded 10 user entities with 4 features each"

# Register drift monitoring
curl -s -X POST "${FEATHER_URL}/v1/drift/register" \
  -H "Content-Type: application/json" \
  -d '{"name": "avg_transaction", "type": "numeric"}' > /dev/null

curl -s -X POST "${FEATHER_URL}/v1/drift/register" \
  -H "Content-Type: application/json" \
  -d '{"name": "risk_score", "type": "numeric"}' > /dev/null

echo "✅ Registered drift monitors for avg_transaction, risk_score"
echo "🎉 Demo ready at ${FEATHER_URL}"
