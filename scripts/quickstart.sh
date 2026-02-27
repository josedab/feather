#!/usr/bin/env bash
set -euo pipefail

IMAGE="ghcr.io/feather-store/feather:latest"
CONTAINER="feather"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required but was not found. Install Docker Desktop and retry." >&2
  exit 1
fi

if ! timeout 5 docker info >/dev/null 2>&1; then
  echo "Docker is installed but not running. Start Docker Desktop and retry." >&2
  exit 1
fi

if docker ps -a --format '{{.Names}}' | grep -x "${CONTAINER}" >/dev/null 2>&1; then
  echo "Container '${CONTAINER}' already exists." >&2
  echo "Remove it with: docker rm -f ${CONTAINER}" >&2
  exit 1
fi

# Try pulling the image; fall back to local build if unavailable
if ! docker pull "${IMAGE}" 2>/dev/null; then
  echo "Image '${IMAGE}' not available — building locally instead..."
  SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
  ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
  docker build -t feather:latest "${ROOT_DIR}" || {
    echo "Docker build failed. Try 'make quickstart-local' to build from source." >&2
    exit 1
  }
  IMAGE="feather:latest"
fi

docker run -d \
  --name "${CONTAINER}" \
  -p 8080:8080 \
  -p 50051:50051 \
  -p 9090:9090 \
  "${IMAGE}" >/dev/null

echo "Feather is starting..."

# Wait for health
printf "Waiting for server to be ready"
HEALTHY=false
for i in $(seq 1 30); do
  if curl -sf http://localhost:8080/health >/dev/null 2>&1; then
    HEALTHY=true
    break
  fi
  printf "."
  sleep 1
done
echo ""

if [ "${HEALTHY}" != "true" ]; then
  echo "Server did not become healthy in 30s." >&2
  docker logs "${CONTAINER}" 2>&1 | tail -20 >&2
  docker rm -f "${CONTAINER}" >/dev/null 2>&1
  exit 1
fi

# Seed demo data
echo "Seeding demo data..."
curl -sS -X POST http://localhost:8080/v1/features \
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
echo "Verifying..."
RESPONSE=$(curl -sS "http://localhost:8080/v1/features?entity=user:123&feature=click_count&feature=purchase_total")
if command -v jq >/dev/null 2>&1; then
  echo "${RESPONSE}" | jq .
else
  echo "${RESPONSE}"
fi

echo ""
echo "✅ Feather is running!"
echo ""
echo "  Health check: curl http://localhost:8080/health"
echo "  Get features: curl 'http://localhost:8080/v1/features?entity=user:123&feature=click_count'"
echo "  Stop:         docker rm -f ${CONTAINER}"
