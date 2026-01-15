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
echo "Health check: curl http://localhost:8080/health"
