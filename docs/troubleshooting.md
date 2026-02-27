# Troubleshooting Guide

This guide consolidates common issues you may encounter when developing, building, or running Feather.

> **Quick diagnostic:** Run `make doctor` to validate your local environment (Go version, ports, memory, disk, tools).

## Table of Contents

- [Build Issues](#build-issues)
- [Port Conflicts](#port-conflicts)
- [BadgerDB Issues](#badgerdb-issues)
- [Kafka / librdkafka Issues](#kafka--librdkafka-issues)
- [Configuration Errors](#configuration-errors)
- [Test Failures](#test-failures)
- [Runtime Errors](#runtime-errors)

---

## Build Issues

### `golangci-lint not found`

Dev tools not installed. Run:

```bash
make install-tools
```

### C compiler / `librdkafka` errors during build

The default build uses `CGO_ENABLED=0` and does not require a C compiler. Only use `make build-cgo` if you need Kafka support:

```bash
# Default build (no C compiler needed)
make build

# Only if you need Kafka consumer support
make build-cgo
```

If you do need CGO, install a C toolchain:

- **macOS:** `xcode-select --install`
- **Ubuntu/Debian:** `sudo apt-get install build-essential`
- **Alpine:** `apk add gcc musl-dev`

### Build is slow or hangs

Ensure you have sufficient disk space (>1 GB) and memory (>2 GB). Run `make doctor` to check.

---

## Port Conflicts

### `bind: address already in use`

Feather uses the following default ports:

| Port  | Service          |
|-------|------------------|
| 8080  | HTTP API         |
| 8081  | Ingestion API    |
| 9090  | Prometheus metrics |
| 50051 | gRPC API         |

To identify what's using a port:

```bash
lsof -i :8080
# or
make doctor   # checks all four ports
```

To use different ports, set environment variables or update your config file:

```bash
FEATHER_HTTP_PORT=8090 FEATHER_GRPC_PORT=50052 make run-dev
```

---

## BadgerDB Issues

### `Cannot acquire directory lock` or `LOCK` file errors

BadgerDB locks its data directory. This typically means another Feather instance is running, or a previous instance didn't shut down cleanly.

```bash
# Check for running instances
ps aux | grep feather

# Remove stale lock (only if no instance is running)
rm -f data/badger/LOCK

# Or reset all data
make clean-data
```

### BadgerDB data corruption

If you encounter data corruption after a crash:

```bash
# Reset data directories
make clean-data

# Re-seed demo data if needed
make run-dev
# (in another terminal)
make demo
```

See also: [ADR-0003: BadgerDB for Persistence](adr/0003-badgerdb-for-persistence.md)

---

## Kafka / librdkafka Issues

### Kafka connection failures

Ensure Kafka is running and accessible. With Docker Compose:

```bash
docker compose up -d kafka
```

Verify the broker address in your config matches the actual Kafka endpoint:

```yaml
kafka:
  enabled: true
  brokers:
    - "localhost:9092"
```

### `librdkafka` not found

The Kafka consumer requires CGO and librdkafka. Install it:

- **macOS:** `brew install librdkafka`
- **Ubuntu/Debian:** `sudo apt-get install librdkafka-dev`

Then build with CGO:

```bash
make build-cgo
```

### Kafka consumer not starting

Check that Kafka is enabled in your configuration:

```yaml
kafka:
  enabled: true
```

Or via environment variable:

```bash
FEATHER_KAFKA_ENABLED=true
```

---

## Configuration Errors

### Config validation failures

Validate your configuration without starting the server:

```bash
# Validate a specific config file
make validate-config CONFIG=configs/feather.yaml

# Validate all YAML configs
make lint-config
```

### `feature group not found` on API calls

The server was started without a schema config. Use the dev config which includes demo schemas:

```bash
make run-dev
```

### Environment variable not taking effect

Feather environment variables use the `FEATHER_` prefix. Ensure correct naming:

```bash
# Correct
export FEATHER_HTTP_PORT=8090

# Wrong — no prefix
export HTTP_PORT=8090
```

See [Configuration Reference](configuration.md) for all 51 environment variables.

---

## Test Failures

### `make test` takes too long (5+ minutes)

The full test suite includes the race detector. Use faster alternatives during development:

```bash
make test-core     # Core packages only (~10s)
make test-quick    # All packages, short mode (~60s)
make test-changed  # Only changed packages
```

### Tests fail with race conditions

Run with the race detector to diagnose:

```bash
make test          # includes -race flag
make test-one RUN=TestFoo   # single test with verbose output
```

### Integration tests fail

Integration tests may require Docker. Ensure Docker is running:

```bash
docker info
make test-integration
```

---

## Runtime Errors

### Server starts but APIs return errors

1. Check health: `curl http://localhost:8080/health`
2. Check readiness: `curl http://localhost:8080/ready`
3. Review logs for startup errors

### High memory usage

Check the hot tier memory limit:

```bash
# Default is 4GB
FEATHER_HOT_MAX_MEMORY=2GB make run-dev
```

Monitor via Prometheus metrics at `http://localhost:9090/metrics`.

### `make quickstart` hangs

Docker is installed but the daemon isn't running. Either:

- Start Docker Desktop
- Or skip Docker: `make quickstart-local`

### Connection refused during smoke tests

The server must be running before smoke tests:

```bash
make run-dev          # Terminal 1
make smoke-test       # Terminal 2
```

Or use the background start:

```bash
make dev-start
make smoke-test
make dev-stop
```

---

## Getting Help

- Run `make doctor` for environment diagnostics
- Check [Deployment Guide](deployment.md) for production setup
- Check [Observability Guide](observability.md) for monitoring
- Check [Performance Guide](performance.md) for tuning
- Open an issue: [GitHub Issues](https://github.com/feather-store/feather/issues)
