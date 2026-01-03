# Feather: Real-time Feature Store

Feather is a high-performance, real-time feature store designed for ultra-low-latency serving at the edge. It enables sub-millisecond feature retrieval through embeddable deployment, background synchronization, and memory-first architecture.

## Features

- **Sub-millisecond P99 latency** for point lookups
- **Tiered storage**: Hot tier (memory) + Warm tier (disk with BadgerDB)
- **Real-time aggregations**: Count, Sum, Avg, Min, Max with sliding windows
- **Point-in-time retrieval**: Historical feature access for training
- **Multiple ingestion sources**: Kafka streaming and HTTP push
- **Multiple serving APIs**: gRPC and HTTP REST
- **Prometheus metrics**: Full observability

## Quick Start

### Build

```bash
make build
```

### Run

```bash
# With default configuration (environment variables)
./bin/feather

# With configuration file
./bin/feather -config configs/feather.yaml
```

### Docker

```bash
make docker-build
make docker-run
```

## API Examples

### Store Features (HTTP)

```bash
curl -X POST http://localhost:8080/v1/features \
  -H "Content-Type: application/json" \
  -d '{
    "entity_key": "user:123",
    "features": {
      "click_count": 15,
      "purchase_total": 245.50,
      "last_activity": "2024-01-15T10:30:00Z"
    }
  }'
```

### Get Features (HTTP)

```bash
curl "http://localhost:8080/v1/features?entity=user:123&feature=click_count&feature=purchase_total"
```

### Batch Get (HTTP)

```bash
curl -X POST http://localhost:8080/v1/features/batch \
  -H "Content-Type: application/json" \
  -d '{
    "entities": ["user:123", "user:456"],
    "features": ["click_count", "purchase_total"]
  }'
```

### Point-in-Time Retrieval

```bash
curl "http://localhost:8080/v1/features/history?entity=user:123&feature=click_count&as_of=2024-01-15T00:00:00Z"
```

## Configuration

Feather can be configured via YAML file or environment variables.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `FEATHER_HTTP_PORT` | 8080 | HTTP server port |
| `FEATHER_GRPC_PORT` | 50051 | gRPC server port |
| `FEATHER_PROMETHEUS_PORT` | 9090 | Prometheus metrics port |
| `FEATHER_HOT_MAX_MEMORY` | 4GB | Maximum memory for hot tier |
| `FEATHER_WARM_PATH` | /var/lib/feather/data | Path for warm tier storage |
| `FEATHER_KAFKA_ENABLED` | false | Enable Kafka ingestion |
| `FEATHER_KAFKA_BROKERS` | localhost:9092 | Kafka broker addresses |

### YAML Configuration

See `configs/feather.yaml` for a complete example.

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        FEATHER                                │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│   ┌─────────────────────────────────────────────────────┐    │
│   │                 INGESTION LAYER                      │    │
│   │    Kafka Consumer  │  HTTP Push  │  Batch Import    │    │
│   └─────────────────────────────────────────────────────┘    │
│                            │                                  │
│   ┌─────────────────────────────────────────────────────┐    │
│   │                 PROCESSING LAYER                     │    │
│   │    Schema Registry  │  Aggregation Engine           │    │
│   └─────────────────────────────────────────────────────┘    │
│                            │                                  │
│   ┌─────────────────────────────────────────────────────┐    │
│   │                 STORAGE ENGINE                       │    │
│   │    Hot Tier (Memory)  │  Warm Tier (BadgerDB)       │    │
│   └─────────────────────────────────────────────────────┘    │
│                            │                                  │
│   ┌─────────────────────────────────────────────────────┐    │
│   │                 SERVING LAYER                        │    │
│   │    gRPC Server  │  HTTP Server  │  Embedded Client  │    │
│   └─────────────────────────────────────────────────────┘    │
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

## Performance

| Metric | Target |
|--------|--------|
| P99 read latency (hot) | <1ms |
| P99 batch latency (100 entities) | <5ms |
| Feature freshness | <1 second |
| Throughput | 1M reads/sec/node |
| Memory efficiency | 1B features in 64GB |

## Development

```bash
# Run tests
make test

# Run with race detector
make test -race

# Lint
make lint

# Format code
make fmt

# Build and test
make dev
```

## License

MIT License
