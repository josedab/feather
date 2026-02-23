# Configuration Files

Example configuration files for different Feather deployment scenarios.

| File | Purpose | When to Use |
|------|---------|-------------|
| `feather.yaml` | Default configuration with full schema | Starting point for custom deployments |
| `feather-dev.yaml` | Minimal dev config, no external deps | Local development and quick prototyping |
| `feather-local.yaml` | Local paths for warm storage | Local development without system-level paths |
| `feather-ha.yaml` | High availability production setup | Production HA with TLS, Kafka, and replication |
| `feather-tls.yaml` | TLS and mutual TLS (mTLS) | Securing traffic with certificates |
| `feather-kafka-sasl.yaml` | Kafka with SASL/SSL authentication | Managed Kafka (Confluent Cloud, AWS MSK) |
| `feather-tracing.yaml` | OpenTelemetry distributed tracing | Enabling OTLP tracing (Jaeger, Tempo, etc.) |
| `prometheus.yml` | Prometheus scrape configuration | Monitoring Feather metrics with Prometheus |

## Usage

```bash
# Run with a specific config
./feather -config configs/feather-dev.yaml

# Or with make
make run-config
```

See the [Configuration](../docs/configuration.md) guide for full details on all settings and environment variable overrides.
