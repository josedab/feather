# Feather API Reference

> Complete reference for Feather's HTTP REST and gRPC APIs.

## Table of Contents

- [Overview](#overview)
- [Authentication](#authentication)
- [HTTP REST API](#http-rest-api)
  - [Feature Operations](#feature-operations)
  - [Schema Management](#schema-management)
  - [Vector Search](#vector-search)
  - [Drift Detection](#drift-detection)
  - [Health & Monitoring](#health--monitoring)
- [HTTP Ingestion API](#http-ingestion-api)
- [gRPC API](#grpc-api)
- [Error Handling](#error-handling)
- [Rate Limiting](#rate-limiting)
- [Pagination](#pagination)

---

## Overview

Feather exposes three API interfaces:

| API | Port | Protocol | Use Case |
|-----|------|----------|----------|
| **HTTP REST** | 8080 | HTTP/1.1, HTTP/2 | Feature serving, schema management |
| **HTTP Ingestion** | 8081 | HTTP/1.1 | Real-time feature ingestion |
| **gRPC** | 50051 | HTTP/2 | High-performance serving, streaming |

### Base URLs

```
HTTP REST API:     http://localhost:8080
HTTP Ingestion:    http://localhost:8081
gRPC:              localhost:50051
```

### Common Headers

| Header | Description |
|--------|-------------|
| `Content-Type` | `application/json` for all requests |
| `X-Request-ID` | Client-provided request ID (optional, auto-generated if absent) |
| `Accept-Encoding` | `gzip` supported for response compression |

### Response Envelope

All HTTP responses follow a standard envelope format:

```json
{
  "success": true,
  "data": { ... },
  "request_id": "req-a1b2c3d4-e5f6-7890",
  "meta": {
    "total_count": 100,
    "page_size": 50,
    "next_cursor": "eyJvZmZzZXQiOjUwfQ=="
  }
}
```

**Error Response:**

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "entity query parameter is required",
    "details": {
      "field": "entity",
      "constraint": "required"
    }
  },
  "request_id": "req-a1b2c3d4-e5f6-7890"
}
```

---

## Authentication

Authentication is configurable and supports multiple methods:

### API Key Authentication

```bash
curl -H "Authorization: Bearer <api-key>" \
  http://localhost:8080/v1/features?entity=user:123
```

### mTLS (gRPC)

Configure client certificates for mutual TLS authentication:

```yaml
tls:
  enabled: true
  cert_file: /etc/feather/client.crt
  key_file: /etc/feather/client.key
  ca_file: /etc/feather/ca.crt
```

---

## HTTP REST API

### Feature Operations

#### Get Features

Retrieve features for a single entity.

```
GET /v1/features
```

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `entity` | string | Yes | Entity key (e.g., `user:123`) |
| `feature` | string[] | No | Feature names to retrieve. If omitted, returns all features. |

**Example Request:**

```bash
curl "http://localhost:8080/v1/features?entity=user:123&feature=click_count&feature=purchase_total"
```

**Example Response:**

```json
{
  "success": true,
  "data": {
    "entities": {
      "user:123": {
        "features": {
          "click_count": {
            "value": 42,
            "timestamp": 1705315800000000000,
            "version": 5
          },
          "purchase_total": {
            "value": 1250.75,
            "timestamp": 1705315800000000000,
            "version": 3
          }
        }
      }
    }
  },
  "request_id": "req-abc123"
}
```

**Status Codes:**

| Code | Description |
|------|-------------|
| 200 | Success |
| 400 | Invalid request (missing entity parameter) |
| 404 | Entity not found |
| 500 | Internal server error |

---

#### Store Features

Store or update features for an entity.

```
POST /v1/features
```

**Request Body:**

```json
{
  "entity_key": "user:123",
  "features": {
    "click_count": 42,
    "purchase_total": 1250.75,
    "last_login": "2024-01-15T10:30:00Z"
  },
  "timestamp": 1705315800000000000,
  "version": 1
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `entity_key` | string | Yes | Entity identifier |
| `features` | object | Yes | Map of feature names to values |
| `timestamp` | int64 | No | Unix nanoseconds. Defaults to current time. |
| `version` | int64 | No | Version number for optimistic locking |

**Example Request:**

```bash
curl -X POST http://localhost:8080/v1/features \
  -H "Content-Type: application/json" \
  -d '{
    "entity_key": "user:123",
    "features": {
      "click_count": 42,
      "purchase_total": 1250.75
    }
  }'
```

**Example Response:**

```json
{
  "success": true,
  "data": {
    "entity_key": "user:123",
    "features_stored": 2,
    "timestamp": 1705315800000000000
  },
  "request_id": "req-def456"
}
```

**Status Codes:**

| Code | Description |
|------|-------------|
| 200 | Success |
| 400 | Invalid request body |
| 422 | Schema validation failed |
| 500 | Internal server error |

---

#### Batch Get Features

Retrieve features for multiple entities in a single request.

```
POST /v1/features/batch
```

**Request Body:**

```json
{
  "entities": ["user:123", "user:456", "user:789"],
  "features": ["click_count", "purchase_total"]
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `entities` | string[] | Yes | List of entity keys (max 1000) |
| `features` | string[] | No | Feature names. If omitted, returns all features. |

**Example Request:**

```bash
curl -X POST http://localhost:8080/v1/features/batch \
  -H "Content-Type: application/json" \
  -d '{
    "entities": ["user:123", "user:456"],
    "features": ["click_count", "purchase_total"]
  }'
```

**Example Response:**

```json
{
  "success": true,
  "data": {
    "entities": {
      "user:123": {
        "features": {
          "click_count": {"value": 42, "timestamp": 1705315800000000000},
          "purchase_total": {"value": 1250.75, "timestamp": 1705315800000000000}
        }
      },
      "user:456": {
        "features": {
          "click_count": {"value": 15, "timestamp": 1705315700000000000},
          "purchase_total": {"value": 89.99, "timestamp": 1705315700000000000}
        }
      }
    }
  },
  "meta": {
    "total_count": 2,
    "found_count": 2,
    "missing_count": 0
  },
  "request_id": "req-ghi789"
}
```

---

#### Point-in-Time Query

Retrieve feature values as they existed at a specific timestamp. Essential for generating training data without data leakage.

```
GET /v1/features/history
```

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `entity` | string | Yes | Entity key |
| `feature` | string[] | No | Feature names to retrieve |
| `as_of` | string | Yes | RFC3339 timestamp (e.g., `2024-01-15T00:00:00Z`) |

**Example Request:**

```bash
curl "http://localhost:8080/v1/features/history?entity=user:123&feature=click_count&as_of=2024-01-15T00:00:00Z"
```

**Example Response:**

```json
{
  "success": true,
  "data": {
    "entity": "user:123",
    "as_of": "2024-01-15T00:00:00Z",
    "features": {
      "click_count": {
        "value": 38,
        "timestamp": 1705276800000000000,
        "version": 3
      }
    }
  },
  "request_id": "req-jkl012"
}
```

---

### Schema Management

#### List Feature Groups

```
GET /v1/schema/groups
```

**Example Response:**

```json
{
  "success": true,
  "data": {
    "groups": [
      {
        "name": "user_features",
        "entity_type": "user",
        "ttl": "720h",
        "features": [
          {
            "name": "click_count",
            "data_type": "int64",
            "description": "Total click count"
          },
          {
            "name": "purchase_total",
            "data_type": "float64",
            "description": "Lifetime purchase value"
          }
        ],
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-10T12:00:00Z"
      }
    ]
  },
  "meta": {
    "total_count": 1
  },
  "request_id": "req-mno345"
}
```

---

#### Get Feature Group

```
GET /v1/schema/groups/{name}
```

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Feature group name |

**Example Request:**

```bash
curl http://localhost:8080/v1/schema/groups/user_features
```

---

#### Create Feature Group

```
POST /v1/schema/groups
```

**Request Body:**

```json
{
  "name": "user_engagement",
  "entity_type": "user",
  "ttl": "720h",
  "features": [
    {
      "name": "clicks_last_hour",
      "data_type": "int64",
      "description": "Click count in the last hour",
      "aggregation": {
        "function": "count",
        "window": "1h",
        "slide_by": "1m"
      }
    },
    {
      "name": "avg_session_duration",
      "data_type": "float64",
      "description": "Average session duration in seconds",
      "validation": {
        "min": 0,
        "max": 86400
      }
    }
  ]
}
```

**Supported Data Types:**

| Type | Description | Example |
|------|-------------|---------|
| `int64` | 64-bit integer | `42` |
| `float64` | 64-bit float | `3.14159` |
| `string` | UTF-8 string | `"hello"` |
| `bool` | Boolean | `true` |
| `bytes` | Binary data | Base64 encoded |
| `vector` | Float array | `[0.1, 0.2, 0.3]` |
| `timestamp` | RFC3339 string | `"2024-01-15T10:30:00Z"` |

**Aggregation Functions:**

| Function | Description |
|----------|-------------|
| `count` | Number of values in window |
| `sum` | Sum of values |
| `avg` | Average value |
| `min` | Minimum value |
| `max` | Maximum value |
| `last` | Most recent value |

---

#### Update Feature Group

```
PUT /v1/schema/groups/{name}
```

Updates an existing feature group. Only additive changes are allowed (adding new features). Removing features or changing data types is not permitted.

---

#### Delete Feature Group

```
DELETE /v1/schema/groups/{name}
```

Deletes a feature group schema. Does not delete stored feature data.

---

### Vector Search

#### List Vector Indexes

```
GET /v1/vectors
```

**Example Response:**

```json
{
  "success": true,
  "data": {
    "indexes": [
      {
        "name": "product_embeddings",
        "dimension": 384,
        "distance_type": "cosine",
        "vector_count": 1500000,
        "created_at": "2024-01-01T00:00:00Z"
      }
    ]
  },
  "request_id": "req-pqr678"
}
```

---

#### Create Vector Index

```
POST /v1/vectors
```

**Request Body:**

```json
{
  "name": "product_embeddings",
  "dimension": 384,
  "distance_type": "cosine",
  "config": {
    "m": 16,
    "ef_construction": 200
  }
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Index name |
| `dimension` | int | Yes | Vector dimension (1-2048) |
| `distance_type` | string | Yes | `cosine`, `euclidean`, or `manhattan` |
| `config.m` | int | No | HNSW M parameter (default: 16) |
| `config.ef_construction` | int | No | HNSW efConstruction (default: 200) |

---

#### Get Vector Index

```
GET /v1/vectors/{index}
```

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `index` | string | Index name |

---

#### Delete Vector Index

```
DELETE /v1/vectors/{index}
```

Deletes the index and all vectors stored in it.

---

#### Upsert Vectors

```
POST /v1/vectors/{index}/upsert
```

**Request Body:**

```json
{
  "vectors": [
    {
      "id": "prod_001",
      "values": [0.1, 0.2, 0.3, ...],
      "metadata": {
        "category": "electronics",
        "price": 299.99
      }
    },
    {
      "id": "prod_002",
      "values": [0.4, 0.5, 0.6, ...],
      "metadata": {
        "category": "books",
        "price": 19.99
      }
    }
  ]
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `vectors[].id` | string | Yes | Unique vector ID |
| `vectors[].values` | float[] | Yes | Vector values (must match index dimension) |
| `vectors[].metadata` | object | No | Arbitrary metadata for filtering |

**Example Response:**

```json
{
  "success": true,
  "data": {
    "upserted_count": 2
  },
  "request_id": "req-stu901"
}
```

---

#### Search Vectors

```
POST /v1/vectors/{index}/search
```

**Request Body:**

```json
{
  "vector": [0.15, 0.25, 0.35, ...],
  "top_k": 10,
  "filter": {
    "category": "electronics"
  },
  "include_metadata": true,
  "include_values": false
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `vector` | float[] | Yes | Query vector |
| `top_k` | int | No | Number of results (default: 10, max: 1000) |
| `filter` | object | No | Metadata filter (exact match) |
| `include_metadata` | bool | No | Include metadata in results (default: true) |
| `include_values` | bool | No | Include vector values in results (default: false) |

**Example Response:**

```json
{
  "success": true,
  "data": {
    "results": [
      {
        "id": "prod_001",
        "score": 0.95,
        "metadata": {
          "category": "electronics",
          "price": 299.99
        }
      },
      {
        "id": "prod_042",
        "score": 0.89,
        "metadata": {
          "category": "electronics",
          "price": 149.99
        }
      }
    ]
  },
  "meta": {
    "query_time_ms": 5
  },
  "request_id": "req-vwx234"
}
```

---

#### Get Vector by ID

```
GET /v1/vectors/{index}/{id}
```

---

#### Delete Vector

```
DELETE /v1/vectors/{index}/{id}
```

---

### Drift Detection

#### Get Drift Status

```
GET /v1/drift/status
```

Returns drift monitoring status for all registered features.

**Example Response:**

```json
{
  "success": true,
  "data": {
    "features": {
      "click_count": {
        "status": "stable",
        "drift_score": 0.02,
        "reference_mean": 45.2,
        "current_mean": 44.8,
        "last_check": "2024-01-15T12:00:00Z"
      },
      "purchase_total": {
        "status": "drifting",
        "drift_score": 0.35,
        "reference_mean": 125.50,
        "current_mean": 168.75,
        "last_check": "2024-01-15T12:00:00Z"
      }
    }
  },
  "request_id": "req-yza567"
}
```

---

#### Get Drift Alerts

```
GET /v1/drift/alerts
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `since` | string | RFC3339 timestamp to filter alerts |
| `severity` | string | Filter by severity: `low`, `medium`, `high` |

**Example Response:**

```json
{
  "success": true,
  "data": {
    "alerts": [
      {
        "id": "alert_001",
        "feature": "purchase_total",
        "severity": "high",
        "drift_score": 0.35,
        "message": "Significant distribution shift detected",
        "detected_at": "2024-01-15T11:45:00Z"
      }
    ]
  },
  "request_id": "req-bcd890"
}
```

---

#### Register Feature for Drift Monitoring

```
POST /v1/drift/register
```

**Request Body:**

```json
{
  "feature": "click_count",
  "config": {
    "threshold": 0.1,
    "window_size": "24h",
    "check_interval": "1h"
  }
}
```

---

#### Reset Reference Distribution

```
POST /v1/drift/reset/{feature}
```

Resets the reference distribution for a feature to the current distribution.

---

### Health & Monitoring

#### Deep Health Check

```
GET /health
```

Returns detailed health status of all system components.

**Example Response:**

```json
{
  "status": "healthy",
  "components": {
    "hot_tier": {
      "status": "healthy",
      "latency_ms": 0.1,
      "metrics": {
        "hits": 1500000,
        "misses": 230000,
        "hit_rate": 0.867,
        "size_bytes": 2147483648
      }
    },
    "warm_tier": {
      "status": "healthy",
      "latency_ms": 2.5,
      "metrics": {
        "size_bytes": 10737418240
      }
    },
    "schema_registry": {
      "status": "healthy",
      "latency_ms": 0.01,
      "metrics": {
        "registered_groups": 5
      }
    },
    "aggregation_engine": {
      "status": "healthy",
      "latency_ms": 0.05,
      "metrics": {
        "active_windows": 342
      }
    }
  },
  "version": "1.0.0",
  "uptime_seconds": 86400
}
```

**Status Values:**

| Status | Description |
|--------|-------------|
| `healthy` | Component operating normally |
| `degraded` | Component experiencing issues but functional |
| `unhealthy` | Component failed or not initialized |

---

#### Readiness Probe

```
GET /ready
```

Returns 200 if the service is ready to accept traffic.

**Example Response:**

```json
{
  "ready": true
}
```

---

#### Liveness Probe

```
GET /live
```

Returns 200 if the service is alive (not deadlocked).

**Example Response:**

```json
{
  "live": true
}
```

---

## HTTP Ingestion API

The ingestion API runs on a separate port (default: 8081) and is optimized for high-throughput data ingestion.

### Single Feature Update

```
POST /ingest
```

**Request Body:**

```json
{
  "entity_key": "user:123",
  "features": {
    "click_count": 43,
    "last_activity": "2024-01-15T12:30:00Z"
  },
  "timestamp": 1705319400000000000
}
```

**Example Request:**

```bash
curl -X POST http://localhost:8081/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "entity_key": "user:123",
    "features": {"click_count": 43}
  }'
```

---

### Bulk Ingestion

```
POST /ingest/bulk
```

**Request Body:**

```json
{
  "updates": [
    {
      "entity_key": "user:123",
      "features": {"click_count": 43}
    },
    {
      "entity_key": "user:456",
      "features": {"click_count": 12}
    }
  ]
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `updates` | array | Yes | List of feature updates (max 10000) |

**Example Response:**

```json
{
  "success": true,
  "data": {
    "processed": 2,
    "failed": 0
  },
  "request_id": "req-efg123"
}
```

---

## gRPC API

### Service Definition

```protobuf
syntax = "proto3";

package feather.v1;

service FeatureService {
  // Get features for a single entity
  rpc GetFeatures(GetFeaturesRequest) returns (GetFeaturesResponse);

  // Get features at a specific point in time
  rpc GetFeaturesAsOf(GetFeaturesAsOfRequest) returns (GetFeaturesResponse);

  // Store features for an entity
  rpc PutFeatures(PutFeaturesRequest) returns (PutFeaturesResponse);

  // Batch get features for multiple entities
  rpc BatchGetFeatures(BatchGetFeaturesRequest) returns (BatchGetFeaturesResponse);

  // Stream feature updates (bidirectional)
  rpc StreamFeatures(stream FeatureUpdate) returns (stream FeatureResponse);
}

message GetFeaturesRequest {
  string entity_key = 1;
  repeated string features = 2;
}

message GetFeaturesResponse {
  string entity_key = 1;
  map<string, FeatureValue> features = 2;
}

message FeatureValue {
  oneof value {
    int64 int_value = 1;
    double float_value = 2;
    string string_value = 3;
    bool bool_value = 4;
    bytes bytes_value = 5;
  }
  int64 timestamp = 10;
  int64 version = 11;
}

message PutFeaturesRequest {
  string entity_key = 1;
  map<string, FeatureValue> features = 2;
  int64 timestamp = 3;
}

message PutFeaturesResponse {
  int32 features_stored = 1;
  int64 timestamp = 2;
}

message GetFeaturesAsOfRequest {
  string entity_key = 1;
  repeated string features = 2;
  int64 as_of_timestamp = 3;
}

message BatchGetFeaturesRequest {
  repeated string entity_keys = 1;
  repeated string features = 2;
}

message BatchGetFeaturesResponse {
  map<string, EntityFeatures> entities = 1;
}

message EntityFeatures {
  map<string, FeatureValue> features = 1;
}

message FeatureUpdate {
  string entity_key = 1;
  map<string, FeatureValue> features = 2;
  int64 timestamp = 3;
}

message FeatureResponse {
  bool success = 1;
  string error_message = 2;
}
```

### Go Client Example

```go
package main

import (
    "context"
    "log"

    pb "github.com/your-org/feather/api/proto/v1"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    client := pb.NewFeatureServiceClient(conn)

    // Get features
    resp, err := client.GetFeatures(context.Background(), &pb.GetFeaturesRequest{
        EntityKey: "user:123",
        Features:  []string{"click_count", "purchase_total"},
    })
    if err != nil {
        log.Fatal(err)
    }

    for name, value := range resp.Features {
        log.Printf("%s: %v\n", name, value)
    }
}
```

### Python Client Example

```python
import grpc
from feather.proto import feature_service_pb2 as pb
from feather.proto import feature_service_pb2_grpc as pb_grpc

channel = grpc.insecure_channel('localhost:50051')
stub = pb_grpc.FeatureServiceStub(channel)

# Get features
response = stub.GetFeatures(pb.GetFeaturesRequest(
    entity_key="user:123",
    features=["click_count", "purchase_total"]
))

for name, value in response.features.items():
    print(f"{name}: {value}")
```

---

## Error Handling

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `BAD_REQUEST` | 400 | Invalid request format or parameters |
| `UNAUTHORIZED` | 401 | Missing or invalid authentication |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Entity or resource not found |
| `CONFLICT` | 409 | Version conflict (optimistic locking) |
| `VALIDATION_FAILED` | 422 | Schema validation failed |
| `RATE_LIMITED` | 429 | Too many requests |
| `INTERNAL_ERROR` | 500 | Internal server error |
| `SERVICE_UNAVAILABLE` | 503 | Service temporarily unavailable |

### Error Response Format

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "Feature 'age' must be between 0 and 150",
    "details": {
      "feature": "age",
      "value": -5,
      "constraint": "min",
      "expected": 0
    }
  },
  "request_id": "req-abc123"
}
```

---

## Rate Limiting

Rate limiting is applied per client IP address.

### Default Limits

| Endpoint | Limit |
|----------|-------|
| Feature reads | 10,000 req/sec |
| Feature writes | 5,000 req/sec |
| Batch operations | 1,000 req/sec |
| Ingestion API | 50,000 req/sec |

### Rate Limit Headers

```http
X-RateLimit-Limit: 10000
X-RateLimit-Remaining: 9850
X-RateLimit-Reset: 1705315860
```

### Rate Limit Exceeded Response

```json
{
  "success": false,
  "error": {
    "code": "RATE_LIMITED",
    "message": "Rate limit exceeded. Retry after 1 second.",
    "details": {
      "limit": 10000,
      "window_seconds": 1,
      "retry_after": 1
    }
  }
}
```

---

## Pagination

List endpoints support cursor-based pagination.

### Request Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | int | Maximum items per page (default: 100, max: 1000) |
| `cursor` | string | Cursor from previous response |

### Response Metadata

```json
{
  "meta": {
    "total_count": 5000,
    "page_size": 100,
    "has_more": true,
    "next_cursor": "eyJvZmZzZXQiOjEwMH0="
  }
}
```

### Example

```bash
# First page
curl "http://localhost:8080/v1/schema/groups?limit=100"

# Next page
curl "http://localhost:8080/v1/schema/groups?limit=100&cursor=eyJvZmZzZXQiOjEwMH0="
```

---

## Further Reading

- [Architecture Overview](./architecture.md) - System design and data flow
- [Deployment Guide](./deployment.md) - Production deployment instructions
- [Contributing Guide](./contributing.md) - Development guidelines
