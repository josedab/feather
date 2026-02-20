# Configuration Reference

> Complete reference for all Feather environment variables and configuration options.

Feather can be configured via **YAML file** or **environment variables**. Environment variables take the form `FEATHER_*` and override YAML values when both are set.

```bash
# Load from YAML
./feather -config configs/feather.yaml

# Or use environment variables
export FEATHER_HTTP_PORT=8080
./feather
```

---

## Table of Contents

- [Server — HTTP](#server--http)
- [Server — gRPC](#server--grpc)
- [Storage — Hot Tier](#storage--hot-tier)
- [Storage — Warm Tier](#storage--warm-tier)
- [Storage — Historical](#storage--historical)
- [Ingestion — Kafka](#ingestion--kafka)
- [Ingestion — Kafka Security](#ingestion--kafka-security)
- [Ingestion — HTTP](#ingestion--http)
- [Metrics](#metrics)
- [Logging](#logging)
- [Tracing (OpenTelemetry)](#tracing-opentelemetry)
- [TLS](#tls)
- [Sync](#sync)
- [UI](#ui)
- [dbt Integration](#dbt-integration)
- [Extensions](#extensions)

---

## Server — HTTP

| Variable | Type | Default | YAML Path | Description |
|----------|------|---------|-----------|-------------|
| `FEATHER_HTTP_PORT` | int | `8080` | `serving.http.port` | HTTP REST API listen port |
| `FEATHER_HTTP_READ_TIMEOUT` | duration | `10s` | `serving.http.read_timeout` | Maximum duration for reading request body |
| `FEATHER_HTTP_WRITE_TIMEOUT` | duration | `10s` | `serving.http.write_timeout` | Maximum duration for writing response |

## Server — gRPC

| Variable | Type | Default | YAML Path | Description |
|----------|------|---------|-----------|-------------|
| `FEATHER_GRPC_PORT` | int | `50051` | `serving.grpc.port` | gRPC API listen port |
| `FEATHER_GRPC_MAX_CONCURRENT` | int | `1000` | `serving.grpc.max_concurrent` | Maximum concurrent gRPC streams |

## Storage — Hot Tier

| Variable | Type | Default | YAML Path | Description |
|----------|------|---------|-----------|-------------|
| `FEATHER_HOT_MAX_MEMORY` | string | `4GB` | `storage.hot.max_memory` | Maximum memory for the in-memory LRU cache (e.g., `512MB`, `4GB`, `16GB`) |
| `FEATHER_HOT_EVICTION` | string | `lru` | `storage.hot.eviction_policy` | Cache eviction policy (`lru`) |

## Storage — Warm Tier

| Variable | Type | Default | YAML Path | Description |
|----------|------|---------|-----------|-------------|
| `FEATHER_WARM_PATH` | string | _(empty)_ | `storage.warm.path` | File path for BadgerDB warm tier storage (e.g., `/var/lib/feather/data`) |
| `FEATHER_WARM_SYNC_INTERVAL` | duration | `1s` | `storage.warm.sync_interval` | Interval for syncing data from hot to warm tier |

## Storage — Historical

| Variable | Type | Default | YAML Path | Description |
|----------|------|---------|-----------|-------------|
| `FEATHER_HISTORICAL_ENABLED` | bool | `false` | `storage.historical.enabled` | Enable historical version storage for point-in-time queries |
| `FEATHER_HISTORICAL_RETENTION` | duration | `720h` | `storage.historical.retention` | How long to retain historical versions (default: 30 days) |

## Ingestion — Kafka

| Variable | Type | Default | YAML Path | Description |
|----------|------|---------|-----------|-------------|
| `FEATHER_KAFKA_ENABLED` | bool | `false` | `ingestion.kafka.enabled` | Enable Kafka consumer for streaming ingestion |
| `FEATHER_KAFKA_BROKERS` | string | `localhost:9092` | `ingestion.kafka.brokers` | Comma-separated list of Kafka broker addresses |
| `FEATHER_KAFKA_TOPIC` | string | `feature-updates` | `ingestion.kafka.topic` | Kafka topic to consume feature updates from |
| `FEATHER_KAFKA_CONSUMER_GROUP` | string | `feather` | `ingestion.kafka.consumer_group` | Kafka consumer group ID |

## Ingestion — Kafka Security

| Variable | Type | Default | YAML Path | Description |
|----------|------|---------|-----------|-------------|
| `FEATHER_KAFKA_SECURITY_PROTOCOL` | string | _(empty)_ | `ingestion.kafka.security.protocol` | Security protocol: `PLAINTEXT`, `SSL`, `SASL_PLAINTEXT`, `SASL_SSL` |
| `FEATHER_KAFKA_SASL_MECHANISM` | string | _(empty)_ | `ingestion.kafka.security.sasl_mechanism` | SASL mechanism: `PLAIN`, `SCRAM-SHA-256`, `SCRAM-SHA-512` |
| `FEATHER_KAFKA_SASL_USERNAME` | string | _(empty)_ | `ingestion.kafka.security.sasl_username` | SASL username for Kafka authentication |
| `FEATHER_KAFKA_SASL_PASSWORD` | string | _(empty)_ | `ingestion.kafka.security.sasl_password` | SASL password for Kafka authentication |
| `FEATHER_KAFKA_SSL_CA_FILE` | string | _(empty)_ | `ingestion.kafka.security.ssl_ca_file` | Path to CA certificate for Kafka SSL |
| `FEATHER_KAFKA_SSL_CERT_FILE` | string | _(empty)_ | `ingestion.kafka.security.ssl_cert_file` | Path to client certificate for Kafka SSL |
| `FEATHER_KAFKA_SSL_KEY_FILE` | string | _(empty)_ | `ingestion.kafka.security.ssl_key_file` | Path to client private key for Kafka SSL |

## Ingestion — HTTP

| Variable | Type | Default | YAML Path | Description |
|----------|------|---------|-----------|-------------|
| `FEATHER_HTTP_INGESTION_ENABLED` | bool | `true` | `ingestion.http.enabled` | Enable HTTP push ingestion endpoint |
| `FEATHER_HTTP_INGESTION_PORT` | int | `8081` | `ingestion.http.port` | HTTP ingestion API listen port |

## Metrics

| Variable | Type | Default | YAML Path | Description |
|----------|------|---------|-----------|-------------|
| `FEATHER_PROMETHEUS_ENABLED` | bool | `true` | `metrics.prometheus.enabled` | Enable Prometheus metrics endpoint |
| `FEATHER_PROMETHEUS_PORT` | int | `9090` | `metrics.prometheus.port` | Prometheus metrics listen port |

## Logging

| Variable | Type | Default | YAML Path | Description |
|----------|------|---------|-----------|-------------|
| `FEATHER_LOG_LEVEL` | string | `info` | `logging.level` | Log level: `debug`, `info`, `warn`, `error` |
| `FEATHER_LOG_FORMAT` | string | `json` | `logging.format` | Log format: `json`, `text` |

## Tracing (OpenTelemetry)

| Variable | Type | Default | YAML Path | Description |
|----------|------|---------|-----------|-------------|
| `FEATHER_TRACING_ENABLED` | bool | `false` | `tracing.enabled` | Enable OpenTelemetry distributed tracing |
| `FEATHER_TRACING_ENDPOINT` | string | `localhost:4317` | `tracing.endpoint` | OTLP collector endpoint (gRPC) |
| `FEATHER_TRACING_SERVICE_NAME` | string | `feather` | `tracing.service_name` | Service name reported to the tracing backend |
| `FEATHER_TRACING_SAMPLE_RATE` | float | `0.1` | `tracing.sample_rate` | Sampling rate between `0.0` (none) and `1.0` (all) |
| `FEATHER_TRACING_INSECURE` | bool | `false` | `tracing.insecure` | Use insecure (non-TLS) connection to the OTLP collector |

## TLS

| Variable | Type | Default | YAML Path | Description |
|----------|------|---------|-----------|-------------|
| `FEATHER_TLS_ENABLED` | bool | `false` | `tls.enabled` | Enable TLS for all servers (HTTP, gRPC, ingestion) |
| `FEATHER_TLS_CERT_FILE` | string | _(empty)_ | `tls.cert_file` | Path to TLS server certificate (PEM) |
| `FEATHER_TLS_KEY_FILE` | string | _(empty)_ | `tls.key_file` | Path to TLS server private key (PEM) |
| `FEATHER_TLS_CA_FILE` | string | _(empty)_ | `tls.ca_file` | Path to CA certificate for client verification (PEM) |
| `FEATHER_TLS_MIN_VERSION` | string | `1.2` | `tls.min_version` | Minimum TLS version: `1.2` or `1.3` |
| `FEATHER_TLS_CLIENT_AUTH` | string | `none` | `tls.client_auth` | Client certificate policy: `none`, `request`, `require`, `verify` |

## Sync

| Variable | Type | Default | YAML Path | Description |
|----------|------|---------|-----------|-------------|
| `FEATHER_SYNC_ENABLED` | bool | `false` | `sync.enabled` | Enable central/edge sync |
| `FEATHER_SYNC_MODE` | string | `edge` | `sync.mode` | Sync mode: `central` or `edge` |
| `FEATHER_SYNC_CENTRAL_ADDRESS` | string | _(empty)_ | `sync.central_address` | Address of the central Feather instance (edge mode) |
| `FEATHER_SYNC_INTERVAL` | duration | `5s` | `sync.sync_interval` | Interval between sync cycles |
| `FEATHER_SYNC_BATCH_SIZE` | int | `1000` | `sync.batch_size` | Number of features to sync per batch |

## UI

| Variable | Type | Default | YAML Path | Description |
|----------|------|---------|-----------|-------------|
| `FEATHER_UI_ENABLED` | bool | `true` | `ui.enabled` | Enable the feature catalog web UI |

## dbt Integration

| Variable | Type | Default | YAML Path | Description |
|----------|------|---------|-----------|-------------|
| `FEATHER_DBT_ENABLED` | bool | `true` | `dbt.enabled` | Enable dbt integration for exposures/metrics |
| `FEATHER_DBT_DEFAULT_ENTITY_TYPE` | string | `unknown` | `dbt.default_entity_type` | Default entity type for imported dbt sources |
| `FEATHER_DBT_OWNER` | string | _(empty)_ | `dbt.owner` | Owner name for dbt-managed features |
| `FEATHER_DBT_TEAM` | string | _(empty)_ | `dbt.team` | Team name for dbt-managed features |
| `FEATHER_DBT_INCLUDE_SOURCES` | bool | `false` | `dbt.include_sources` | Include dbt sources in feature catalog |
| `FEATHER_DBT_INCLUDE_METRICS` | bool | `false` | `dbt.include_metrics` | Include dbt metrics in feature catalog |

## Extensions

These environment variables are used by optional extension modules. They are read directly via `os.Getenv` rather than through the central config loader.

| Variable | Type | Default | Used By | Description |
|----------|------|---------|---------|-------------|
| `FEATHER_MESH_ADVERTISE_ADDR` | string | _(empty)_ | `extensions/mesh` | Advertise address for mesh/cluster membership |
| `FEATHER_STARLARK_SIDECAR_ADDR` | string | _(empty)_ | `extensions/starlarkudf` | Address of the Starlark UDF sidecar process |
| `FEATHER_PYTHON_WORKER_ENDPOINT` | string | _(empty)_ | `extensions/pythonsdk` | Endpoint for the Python worker process |
| `FEATHER_FLINK_JOBMANAGER_ADDR` | string | _(empty)_ | `integrations/flink` | Address of the Apache Flink JobManager |
| `FEATHER_E2E_URL` | string | _(empty)_ | `test/e2e` | Base URL for end-to-end tests |

---

## Type Reference

| Type | Format | Examples |
|------|--------|---------|
| **int** | Integer | `8080`, `1000` |
| **bool** | Boolean | `true`, `false`, `1`, `0` |
| **string** | Free-form text | `info`, `/var/lib/feather/data` |
| **float** | Decimal | `0.1`, `1.0` |
| **duration** | Go duration string | `1s`, `500ms`, `10m`, `24h` |
| **memory** | Size with unit suffix | `512MB`, `4GB`, `16GB` |

## Config File Examples

| Config File | Use Case |
|-------------|----------|
| [`configs/feather-dev.yaml`](../configs/feather-dev.yaml) | Local development, zero external dependencies |
| [`configs/feather-local.yaml`](../configs/feather-local.yaml) | Local development with disk persistence |
| [`configs/feather.yaml`](../configs/feather.yaml) | Production reference with all features |
| [`configs/feather-ha.yaml`](../configs/feather-ha.yaml) | High-availability production deployment |
| [`configs/feather-tls.yaml`](../configs/feather-tls.yaml) | TLS/mTLS configuration |
| [`configs/feather-kafka-sasl.yaml`](../configs/feather-kafka-sasl.yaml) | Kafka with SASL/SSL authentication |
| [`configs/feather-tracing.yaml`](../configs/feather-tracing.yaml) | OpenTelemetry tracing setup |
