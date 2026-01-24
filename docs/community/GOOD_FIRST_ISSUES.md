# Good First Issues

Welcome! 👋 These issues are designed for new contributors who want to get started with Feather. Each one is self-contained, well-scoped, and has clear acceptance criteria. Pick one that matches your interest, comment on the issue to claim it, and open a PR!

---

## 📝 Documentation

### 1. Add GoDoc comments to the `internal/core/domain` package
**Labels:** `good first issue`, `documentation`
The domain types (`FeatureValue`, `FeatureGroup`, `DataType`, etc.) are missing exported doc comments. Add idiomatic GoDoc strings to every exported symbol in `internal/core/domain/types.go`.

### 2. Update API reference for vector similarity search endpoints
**Labels:** `good first issue`, `documentation`
The vector search API (`/v1/vectors/*`) has grown since the docs were last updated. Audit `internal/core/server/http.go` for vector routes and update `docs/` to reflect current request/response schemas, status codes, and examples.

### 3. Add architecture diagram for tiered storage
**Labels:** `good first issue`, `documentation`
Create a Mermaid or SVG diagram showing the data flow between the hot tier (LRU cache), warm tier (BadgerDB), and the registry. Add it to `docs/` with a short explanation of how TTL, eviction, and promotion work.

---

## 🧪 Testing

### 4. Add edge-case tests for drift detector
**Labels:** `good first issue`, `testing`
The drift detection module (`internal/core/server/`) lacks tests for edge cases: empty reference distributions, single-value distributions, NaN/Inf values, and features with zero variance. Add table-driven tests covering these cases.

### 5. Add benchmark for batch feature retrieval
**Labels:** `good first issue`, `testing`
Write a Go benchmark (`Benchmark_BatchGet`) in `test/` that measures throughput for batch gets of 10, 100, and 1 000 features. Use `testing.B` with `b.ResetTimer()` after setup.

### 6. Test concurrent schema evolution
**Labels:** `good first issue`, `testing`
Add a test that registers and updates feature group schemas from multiple goroutines simultaneously. Verify that the registry remains consistent and no data races occur (run with `-race`).

### 7. Add integration test for Feast compatibility layer
**Labels:** `good first issue`, `testing`
Write an integration test in `test/` that starts Feather, loads features via the Feast-compatible endpoint, and retrieves them via the native API. Confirm field mapping is correct.

---

## ✨ Features

### 8. Add CSV export format to offline store
**Labels:** `good first issue`, `feature`
The export package (`internal/core/export/`) supports JSON, JSONL, and Parquet. Add a CSV exporter implementing the same `Exporter` interface. Include header row and proper escaping.

### 9. Add Slack webhook formatter for alerts
**Labels:** `good first issue`, `feature`
Create a Slack webhook formatter in `internal/platform/` that converts drift alerts and health-check failures into Slack Block Kit messages. Accept a webhook URL via config.

### 10. Add metric for cache hit rate by feature group
**Labels:** `good first issue`, `feature`
Add a Prometheus counter vector `feather_cache_hits_total{group="..."}` and `feather_cache_misses_total{group="..."}` to the hot tier. Increment on every `Get()` call, partitioned by feature group name.

### 11. Add CLI shell completion for bash/zsh
**Labels:** `good first issue`, `feature`
Add `completion` subcommand to the Feather CLI that generates bash and zsh completion scripts. Use `cobra`'s built-in completion generation or write a custom one.

---

## 🐛 Bug Fixes

### 12. Fix gzip response writer flush ordering
**Labels:** `good first issue`, `bug`
The gzip middleware in `internal/core/server/` closes the gzip writer *after* the response writer, which can truncate the final bytes. Reorder the `Close()` calls so the gzip writer flushes before the underlying writer is done.

### 13. Add request timeout middleware
**Labels:** `good first issue`, `bug`
Long-running requests (e.g., large batch gets) can hold connections indefinitely. Add an HTTP middleware that wraps each request context with a configurable timeout (default 30 s) and returns `504 Gateway Timeout` on expiry.

### 14. Validate TLS certificate existence at startup
**Labels:** `good first issue`, `bug`
When TLS is enabled via config, Feather attempts to load cert/key files at listen time. If the files don't exist, the error message is opaque. Add an explicit `os.Stat` check during config validation with a clear error message.

### 15. Log warning for unused configuration keys
**Labels:** `good first issue`, `bug`
If a user passes an unrecognised key in the YAML config (e.g., `htttp_port` instead of `http_port`), Feather silently ignores it. Add a post-parse step that detects unknown keys and logs a warning.

---

## How to Contribute

1. **Comment on the issue** to let others know you're working on it.
2. **Fork the repo** and create a branch: `git checkout -b fix/issue-name`.
3. **Make your changes** — follow the conventions in `CONTRIBUTING.md`.
4. **Run checks**: `make check` (fmt, vet, lint, test).
5. **Open a PR** referencing the issue number.

Thank you for contributing to Feather! 🪶
