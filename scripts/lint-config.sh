#!/usr/bin/env bash
set -euo pipefail

# Validates all YAML config files in configs/ against the Config struct.
# Requires a built feather binary with -validate support.

BINARY="${1:-./bin/feather}"
CONFIG_DIR="${2:-configs}"
PASS=0
FAIL=0

if [ ! -x "$BINARY" ]; then
  echo "❌ Binary not found at $BINARY — run 'make build' first." >&2
  exit 1
fi

for f in "$CONFIG_DIR"/*.yaml; do
  [ -f "$f" ] || continue
  base=$(basename "$f")

  # Skip non-feather config files (e.g. prometheus.yml)
  case "$base" in
    prometheus*) continue ;;
  esac

  if "$BINARY" -config "$f" -validate >/dev/null 2>&1; then
    PASS=$((PASS + 1))
    echo "  ✅ $f"
  else
    FAIL=$((FAIL + 1))
    echo "  ❌ $f"
    "$BINARY" -config "$f" -validate 2>&1 | sed 's/^/     /' || true
  fi
done

echo ""
echo "Results: ${PASS}/$((PASS + FAIL)) configs valid"
if [ "${FAIL}" -ne 0 ]; then
  exit 1
fi
