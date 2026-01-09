---
sidebar_position: 6
title: API Reference
description: Complete API reference for Feather feature store.
---

# API Reference

Feather provides multiple API interfaces for different use cases. This page serves as a quick reference and navigation guide.

## API Interfaces

| Interface | Port | Use Case | Protocol |
|-----------|------|----------|----------|
| **HTTP REST** | 8080 | General purpose, SDKs | HTTP/1.1, HTTP/2 |
| **gRPC** | 50051 | High-throughput serving | HTTP/2 |
| **Ingestion** | 8081 | Data ingestion | HTTP/1.1 |
| **Metrics** | 9090 | Prometheus scraping | HTTP/1.1 |

## Quick Reference

### Feature Operations

| Operation | HTTP | gRPC |
|-----------|------|------|
| Get features | `GET /v1/features` | `GetFeatures` |
| Store features | `POST /v1/features` | `PutFeatures` |
| Batch get | `POST /v1/features/batch` | `GetFeaturesBatch` |
| Point-in-time | `GET /v1/features/history` | `GetFeaturesAsOf` |

### Vector Operations

| Operation | HTTP |
|-----------|------|
| Create index | `POST /v1/vectors` |
| Upsert vectors | `POST /v1/vectors/{index}/upsert` |
| Search | `POST /v1/vectors/{index}/search` |
| Delete | `DELETE /v1/vectors/{index}/{id}` |

### Schema Operations

| Operation | HTTP |
|-----------|------|
| List groups | `GET /v1/schema/groups` |
| Create group | `POST /v1/schema/groups` |
| Get group | `GET /v1/schema/groups/{name}` |

### dbt Integration

| Operation | HTTP |
|-----------|------|
| Sync manifest | `POST /v1/dbt/sync` |
| Validate manifest | `POST /v1/dbt/validate` |
| Get sync status | `GET /v1/dbt/status` |

### Feature Catalog

| Operation | HTTP |
|-----------|------|
| List features | `GET /v1/catalog/features` |
| Get feature | `GET /v1/catalog/features/{name}` |
| Get lineage | `GET /v1/catalog/features/{name}/lineage` |

### Monitoring

| Operation | HTTP |
|-----------|------|
| Health check | `GET /health` |
| Readiness | `GET /ready` |
| Liveness | `GET /live` |
| Drift status | `GET /v1/drift/status` |
| Freshness | `GET /v1/freshness/status` |

## Detailed Documentation

- **[REST API Reference](./sdks/rest-api)** - Complete HTTP endpoint documentation
- **[Go SDK](./sdks/go)** - Go client library
- **[Python SDK](./sdks/python)** - Python client library

## gRPC API

### Service Definition

```protobuf
service FeatherService {
  // Feature operations
  rpc GetFeatures(GetFeaturesRequest) returns (GetFeaturesResponse);
  rpc PutFeatures(PutFeaturesRequest) returns (PutFeaturesResponse);
  rpc GetFeaturesBatch(GetFeaturesBatchRequest) returns (GetFeaturesBatchResponse);
  rpc GetFeaturesAsOf(GetFeaturesAsOfRequest) returns (GetFeaturesAsOfResponse);

  // Streaming
  rpc StreamFeatures(stream FeatureUpdate) returns (StreamResponse);
  rpc WatchFeatures(WatchRequest) returns (stream FeatureUpdate);
}
```

### Proto Files

Proto files are available at:
```
github.com/feather-store/feather/api/proto/feather.proto
```

### gRPC Client Example (Go)

```go
import (
    "google.golang.org/grpc"
    pb "github.com/feather-store/feather/api/proto"
)

conn, _ := grpc.Dial("localhost:50051", grpc.WithInsecure())
client := pb.NewFeatherServiceClient(conn)

resp, _ := client.GetFeatures(ctx, &pb.GetFeaturesRequest{
    Entity:   "user:123",
    Features: []string{"click_count", "purchase_total"},
})
```

### gRPC Client Example (Python)

```python
import grpc
from feather.proto import feather_pb2, feather_pb2_grpc

channel = grpc.insecure_channel('localhost:50051')
stub = feather_pb2_grpc.FeatherServiceStub(channel)

response = stub.GetFeatures(feather_pb2.GetFeaturesRequest(
    entity="user:123",
    features=["click_count", "purchase_total"]
))
```

## Data Types

### Supported Types

| Type | Go | Python | JSON |
|------|-----|--------|------|
| `int64` | `int64` | `int` | `42` |
| `float64` | `float64` | `float` | `3.14` |
| `string` | `string` | `str` | `"hello"` |
| `bool` | `bool` | `bool` | `true` |
| `bytes` | `[]byte` | `bytes` | `"base64..."` |
| `timestamp` | `time.Time` | `datetime` | `"2024-01-15T..."` |
| `vector` | `[]float32` | `list[float]` | `[0.1, 0.2, ...]` |

### Type Coercion

The API performs automatic type coercion where safe:

| Input | Target Type | Result |
|-------|-------------|--------|
| `42` | float64 | `42.0` |
| `"42"` | int64 | `42` |
| `1705334400` | timestamp | `2024-01-15T16:00:00Z` |

## Error Handling

### Error Response Format

```json
{
  "success": false,
  "error": {
    "code": "NOT_FOUND",
    "message": "Entity not found: user:999",
    "details": {
      "entity": "user:999"
    }
  }
}
```

### Error Codes

| Code | HTTP | gRPC | Description |
|------|------|------|-------------|
| `NOT_FOUND` | 404 | 5 | Resource not found |
| `INVALID_ARGUMENT` | 400 | 3 | Invalid request |
| `ALREADY_EXISTS` | 409 | 6 | Resource exists |
| `PERMISSION_DENIED` | 403 | 7 | Not authorized |
| `INTERNAL_ERROR` | 500 | 13 | Server error |
| `UNAVAILABLE` | 503 | 14 | Service unavailable |
| `DEADLINE_EXCEEDED` | 504 | 4 | Request timeout |

## Rate Limiting

Default rate limits:

| Endpoint | Limit |
|----------|-------|
| Read operations | 10,000 req/min |
| Write operations | 1,000 req/min |
| Batch operations | 100 req/min |
| Export operations | 10 req/min |

Rate limit headers:

```http
X-RateLimit-Limit: 10000
X-RateLimit-Remaining: 9500
X-RateLimit-Reset: 1705334460
```

## Versioning

The API uses URL path versioning:

```
/v1/features  (current)
/v2/features  (future)
```

Backward compatibility is maintained within major versions.

## OpenAPI Specification

The OpenAPI (Swagger) specification is available at:

```
http://localhost:8080/openapi.json
http://localhost:8080/swagger-ui
```

## Related Documentation

- [REST API Details](./sdks/rest-api)
- [Go SDK](./sdks/go)
- [Python SDK](./sdks/python)
- [Configuration](./configuration)
