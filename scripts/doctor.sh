#!/usr/bin/env bash
set -u

fail=0

check_cmd() {
  local name="$1"
  local required="$2"
  if command -v "${name}" >/dev/null 2>&1; then
    echo "✅ ${name} found"
    return 0
  fi

  if [ "${required}" = "required" ]; then
    echo "❌ ${name} not found (required)" >&2
    fail=1
    return 1
  fi

  echo "⚠️  ${name} not found (optional)" >&2
  return 0
}

check_go_version() {
  local required="1.24"
  if ! command -v go >/dev/null 2>&1; then
    echo "❌ Go not found (required ${required}+)" >&2
    fail=1
    return
  fi

  local version
  version="$(go env GOVERSION 2>/dev/null | sed 's/^go//')"
  if [ -z "${version}" ]; then
    echo "❌ Unable to determine Go version" >&2
    fail=1
    return
  fi

  if [ "$(printf '%s\n' "${required}" "${version}" | sort -V | head -n1)" != "${required}" ]; then
    echo "❌ Go ${required}+ required (found ${version})" >&2
    fail=1
  else
    echo "✅ Go ${version}"
  fi
}

check_port() {
  local port="$1"
  local label="$2"
  if lsof -i ":${port}" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "⚠️  Port ${port} (${label}) is in use" >&2
    local proc
    proc=$(lsof -i ":${port}" -sTCP:LISTEN -t 2>/dev/null | head -1)
    if [ -n "${proc}" ]; then
      local pname
      pname=$(ps -p "${proc}" -o comm= 2>/dev/null || echo "unknown")
      echo "   └─ PID ${proc} (${pname})" >&2
    fi
  else
    echo "✅ Port ${port} (${label}) available"
  fi
}

check_dev_tool() {
  local name="$1"
  if command -v "${name}" >/dev/null 2>&1; then
    echo "✅ ${name} found"
  else
    echo "⚠️  ${name} not found — run 'make install-tools'" >&2
  fi
}

echo "Feather environment check"
echo ""

echo "Required tools"
check_go_version
check_cmd make required
check_cmd curl required

echo ""
echo "Optional tools"
check_cmd docker optional
if command -v docker >/dev/null 2>&1; then
  if ! docker info >/dev/null 2>&1; then
    echo "⚠️  Docker is installed but not running" >&2
  fi
fi

echo ""
echo "CGO / Kafka support"
if command -v gcc >/dev/null 2>&1 || command -v cc >/dev/null 2>&1; then
  echo "✅ C compiler found (Kafka/CGO builds available via 'make build-cgo')"
else
  echo "ℹ️  No C compiler found — Kafka/CGO builds unavailable (core builds work fine)"
fi

echo ""
echo "Development tools"
check_dev_tool golangci-lint
check_dev_tool goimports

echo ""
echo "Port availability"
check_port 8080  "HTTP API"
check_port 50051 "gRPC API"
check_port 8081  "HTTP ingestion"
check_port 9090  "Prometheus metrics"

if [ "${fail}" -ne 0 ]; then
  echo ""
  echo "Some required checks failed." >&2
  exit 1
fi

echo ""
echo "All checks passed. Ready to run: make run-dev"
