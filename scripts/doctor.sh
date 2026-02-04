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
check_cmd node optional
check_cmd npm optional

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

echo ""
echo "System resources"

# Memory check: warn if less than 2GB available
check_memory() {
  local available_mb=0
  if [ "$(uname)" = "Darwin" ]; then
    # macOS: use vm_stat to estimate free memory
    local page_size
    page_size=$(sysctl -n hw.pagesize 2>/dev/null || echo 4096)
    local free_pages
    free_pages=$(vm_stat 2>/dev/null | awk '/Pages free/ {gsub(/\./,"",$3); print $3}')
    local inactive_pages
    inactive_pages=$(vm_stat 2>/dev/null | awk '/Pages inactive/ {gsub(/\./,"",$3); print $3}')
    if [ -n "${free_pages}" ] && [ -n "${inactive_pages}" ]; then
      available_mb=$(( (free_pages + inactive_pages) * page_size / 1024 / 1024 ))
    fi
  elif [ -f /proc/meminfo ]; then
    # Linux: read MemAvailable
    available_mb=$(awk '/MemAvailable/ {printf "%d", $2/1024}' /proc/meminfo 2>/dev/null)
  fi

  if [ "${available_mb}" -gt 0 ] 2>/dev/null; then
    if [ "${available_mb}" -lt 2048 ]; then
      echo "⚠️  Available memory: ${available_mb}MB — less than 2GB (hot tier defaults to 4GB)" >&2
    else
      echo "✅ Available memory: ${available_mb}MB"
    fi
  else
    echo "ℹ️  Could not determine available memory"
  fi
}
check_memory

# Disk space check: warn if less than 1GB free in current directory's filesystem
check_disk() {
  local free_mb=0
  if command -v df >/dev/null 2>&1; then
    # Use POSIX-compatible df output (1K blocks)
    free_mb=$(df -Pk . 2>/dev/null | awk 'NR==2 {printf "%d", $4/1024}')
  fi

  if [ "${free_mb}" -gt 0 ] 2>/dev/null; then
    if [ "${free_mb}" -lt 1024 ]; then
      echo "⚠️  Free disk space: ${free_mb}MB — less than 1GB (BadgerDB warm tier needs disk)" >&2
    else
      echo "✅ Free disk space: ${free_mb}MB"
    fi
  else
    echo "ℹ️  Could not determine free disk space"
  fi
}
check_disk

if [ "${fail}" -ne 0 ]; then
  echo ""
  echo "Some required checks failed." >&2
  exit 1
fi

echo ""
echo "All checks passed. Ready to run: make run-dev"
