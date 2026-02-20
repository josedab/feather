# Feather API Reference

> Complete reference for Feather's HTTP REST and gRPC APIs.
>
> **This document is also available as separate pages for easier navigation:**
>
> | Section | File |
> |---------|------|
> | Overview & Authentication | [api/overview.md](./api/overview.md) |
> | HTTP REST API (Features, Schema, Vectors, Drift, Health) | [api/features.md](./api/features.md) |
> | HTTP Ingestion API | [api/ingestion.md](./api/ingestion.md) |
> | gRPC API | [api/grpc.md](./api/grpc.md) |
> | Extension APIs (Sharding, Marketplace, FeatherQL, etc.) | [api/extensions.md](./api/extensions.md) |
> | Error Handling, Rate Limiting & Pagination | [api/errors-and-limits.md](./api/errors-and-limits.md) |

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
- [Extension APIs](#extension-apis)
  - [Sharding & Replication](#sharding--replication)
  - [Feature Marketplace](#feature-marketplace)
  - [Cloud Service](#cloud-service)
  - [FeatherQL (Declarative Pipelines)](#featherql-declarative-pipelines)
  - [LLM Cache](#llm-cache)
  - [AutoFE (Automated Feature Engineering)](#autofe-automated-feature-engineering)
  - [Geo-Routing](#geo-routing)
  - [A/B Rollout](#ab-rollout)
  - [Edge Runtime](#edge-runtime)
  - [Additional Extension Handlers](#additional-extension-handlers)
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
  // Retrieve features for one or more entities.
  rpc GetFeatures(GetFeaturesRequest) returns (GetFeaturesResponse);

  // Retrieve features with server-side streaming for large results.
  rpc GetFeaturesStream(GetFeaturesRequest) returns (stream EntityFeaturesResponse);

  // Retrieve features as of a specific timestamp.
  rpc GetFeaturesAsOf(GetFeaturesAsOfRequest) returns (GetFeaturesResponse);

  // Store features for an entity.
  rpc PutFeatures(PutFeaturesRequest) returns (PutFeaturesResponse);
}

message GetFeaturesRequest {
  repeated string entities = 1;
  repeated string features = 2;
}

message GetFeaturesAsOfRequest {
  string entity_key = 1;
  repeated string features = 2;
  int64 as_of_timestamp = 3;  // Unix nanoseconds
}

message GetFeaturesResponse {
  map<string, EntityFeatures> entities = 1;
}

message EntityFeaturesResponse {
  string entity_key = 1;
  EntityFeatures features = 2;
}

message EntityFeatures {
  map<string, FeatureValue> features = 1;
}

message FeatureValue {
  oneof value {
    int64 int_value = 1;
    double double_value = 2;
    string string_value = 3;
    bool bool_value = 4;
    bytes bytes_value = 5;
    VectorValue vector_value = 6;
  }
  int64 timestamp = 10;  // Unix nanoseconds
}

message VectorValue {
  repeated float values = 1;
}

message PutFeaturesRequest {
  string entity_key = 1;
  map<string, FeatureValue> features = 2;
  int64 version = 3;  // Optional, for conflict resolution
}

message PutFeaturesResponse {
  bool success = 1;
  string error = 2;
}

// Health check service (gRPC health checking protocol)
service Health {
  rpc Check(HealthCheckRequest) returns (HealthCheckResponse);
  rpc Watch(HealthCheckRequest) returns (stream HealthCheckResponse);
}

message HealthCheckRequest {
  string service = 1;
}

message HealthCheckResponse {
  enum ServingStatus {
    UNKNOWN = 0;
    SERVING = 1;
    NOT_SERVING = 2;
    SERVICE_UNKNOWN = 3;
  }
  ServingStatus status = 1;
}
```

### Go Client Example

```go
package main

import (
    "context"
    "log"

    pb "github.com/feather-store/feather/api/featherpb"
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
        Entities: []string{"user:123"},
        Features: []string{"click_count", "purchase_total"},
    })
    if err != nil {
        log.Fatal(err)
    }

    for entity, feats := range resp.Entities {
        for name, value := range feats.Features {
            log.Printf("%s/%s: %v\n", entity, name, value)
        }
    }
}
```

### Python Client Example

```python
import grpc
from feather.proto import feather_pb2 as pb
from feather.proto import feather_pb2_grpc as pb_grpc

channel = grpc.insecure_channel('localhost:50051')
stub = pb_grpc.FeatureServiceStub(channel)

# Get features
response = stub.GetFeatures(pb.GetFeaturesRequest(
    entities=["user:123"],
    features=["click_count", "purchase_total"]
))

for entity, feats in response.entities.items():
    for name, value in feats.features.items():
        print(f"{entity}/{name}: {value}")
```

---

## Extension APIs

Extension handlers are enabled via the `EnabledFeatures` map in `cmd/feather/main.go`. Run `make api-routes` to see all registered handlers with maturity levels. All extension endpoints use the standard JSON response envelope and error format described in [Error Handling](#error-handling).

> **Maturity levels**: **stable** = production-ready, **beta** = functional but API may change, **experimental** = may be incomplete.

### Sharding & Replication

**Maturity:** stable

Distributed sharding with consistent hashing for horizontal scaling.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/sharding/stats` | Get shard routing statistics |
| `GET` | `/v1/sharding/partition?key=X` | Get partition for a key |
| `GET` | `/v1/sharding/owners?key=X` | Get replica owners for a key |
| `POST` | `/v1/sharding/recompute` | Recompute partition map |

**Example — Get partition for a key:**

```bash
curl "http://localhost:8080/v1/sharding/partition?key=user:123"
```

```json
{
  "success": true,
  "data": {
    "key": "user:123",
    "partition": 42,
    "node": "node-1"
  }
}
```

---

### Feature Marketplace

**Maturity:** stable

Publish, discover, and subscribe to shared feature definitions across teams.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/marketplace/features` | List published features |
| `POST` | `/v1/marketplace/features` | Publish a feature |
| `GET` | `/v1/marketplace/features/{id}` | Get feature details |
| `GET` | `/v1/marketplace/features/{id}/subscribers` | List feature subscribers |
| `POST` | `/v1/marketplace/features/{id}/subscribe` | Subscribe to a feature |
| `DELETE` | `/v1/marketplace/features/{id}/subscribe` | Unsubscribe from a feature |
| `POST` | `/v1/marketplace/features/{id}/deprecate` | Deprecate a feature |
| `GET` | `/v1/marketplace/search?q=X` | Search marketplace |
| `GET` | `/v1/marketplace/stats` | Marketplace statistics |

**Example — Publish a feature:**

```bash
curl -X POST http://localhost:8080/v1/marketplace/features \
  -H "Content-Type: application/json" \
  -d '{
    "name": "user_click_count",
    "description": "Hourly click count per user",
    "entity_type": "user",
    "data_type": "int64",
    "owner": "ml-team",
    "tags": ["engagement", "real-time"]
  }'
```

```json
{
  "success": true,
  "data": {
    "id": "feat_abc123",
    "name": "user_click_count",
    "status": "published"
  }
}
```

---

### Cloud Service

**Maturity:** beta

Managed instance provisioning and scaling.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/cloud/instances` | List managed instances |
| `POST` | `/v1/cloud/instances` | Provision a new instance |
| `GET` | `/v1/cloud/instances/{id}` | Get instance details |
| `GET` | `/v1/cloud/instances/{id}/metrics` | Get instance metrics |
| `POST` | `/v1/cloud/instances/{id}/scale` | Scale an instance |
| `POST` | `/v1/cloud/instances/{id}/autoscale` | Configure autoscaling |
| `DELETE` | `/v1/cloud/instances/{id}` | Terminate an instance |

**Example — Provision an instance:**

```bash
curl -X POST http://localhost:8080/v1/cloud/instances \
  -H "Content-Type: application/json" \
  -d '{
    "name": "prod-us-east",
    "tier": "standard",
    "region": "us-east-1",
    "replicas": 3
  }'
```

---

### FeatherQL (Declarative Pipelines)

**Maturity:** beta

SQL-like DSL for declarative feature pipelines.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/featherql/parse` | Parse a FeatherQL query |
| `POST` | `/v1/featherql/compile` | Compile a FeatherQL pipeline |
| `POST` | `/v1/featherql/execute` | Execute a FeatherQL query |
| `POST` | `/v1/featherql/validate` | Validate a FeatherQL query |
| `GET` | `/v1/featherql/pipelines` | List compiled pipelines |

**v2 endpoints:**

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/featherql/v2/parse` | Parse (v2 engine) |
| `POST` | `/v1/featherql/v2/compile` | Compile (v2 engine) |
| `POST` | `/v1/featherql/v2/execute` | Execute (v2 engine) |
| `GET` | `/v1/featherql/v2/pipelines` | List pipelines (v2) |
| `GET` | `/v1/featherql/v2/pipelines/{id}` | Get pipeline details (v2) |
| `DELETE` | `/v1/featherql/v2/pipelines/{id}` | Delete pipeline (v2) |

**Example — Execute a query:**

```bash
curl -X POST http://localhost:8080/v1/featherql/execute \
  -H "Content-Type: application/json" \
  -d '{
    "query": "SELECT click_count, purchase_total FROM user_features WHERE entity = '\''user:123'\''"
  }'
```

```json
{
  "success": true,
  "data": {
    "columns": ["click_count", "purchase_total"],
    "rows": [
      {"click_count": 42, "purchase_total": 1250.75}
    ]
  }
}
```

---

### LLM Cache

**Maturity:** beta

Semantic caching for LLM prompt/response pairs to reduce latency and cost.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/llm/cache/lookup` | Lookup cached LLM response |
| `POST` | `/v1/llm/cache/store` | Store LLM response |
| `POST` | `/v1/llm/cache/clear` | Clear cache entries |
| `DELETE` | `/v1/llm/cache` | Delete all cache entries |
| `GET` | `/v1/llm/cache/stats` | Cache hit/miss statistics |
| `GET` | `/v1/llm/cache/costs` | Cost savings by provider |

**Example — Lookup cached response:**

```bash
curl -X POST http://localhost:8080/v1/llm/cache/lookup \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "What is the capital of France?",
    "model": "gpt-4",
    "similarity_threshold": 0.95
  }'
```

```json
{
  "success": true,
  "data": {
    "hit": true,
    "response": "The capital of France is Paris.",
    "similarity": 0.99,
    "saved_latency_ms": 850
  }
}
```

---

### AutoFE (Automated Feature Engineering)

**Maturity:** beta

Automated generation and ranking of candidate features.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/autofe/generate` | Generate candidate features |
| `GET` | `/v1/autofe/candidates` | List all candidates |
| `GET` | `/v1/autofe/candidates/top` | Get top candidates by score |
| `GET` | `/v1/autofe/stats` | Generation statistics |

**Example — Generate candidates:**

```bash
curl -X POST http://localhost:8080/v1/autofe/generate \
  -H "Content-Type: application/json" \
  -d '{
    "entity_type": "user",
    "source_features": ["click_count", "purchase_total", "session_duration"],
    "max_candidates": 20
  }'
```

```json
{
  "success": true,
  "data": {
    "candidates": [
      {
        "name": "click_count_rolling_avg_7d",
        "expression": "avg(click_count, 7d)",
        "score": 0.92
      }
    ],
    "total": 15
  }
}
```

---

### Geo-Routing

**Maturity:** beta

Multi-cloud geo-routing with data residency compliance.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/georouting/regions` | List registered regions |
| `POST` | `/v1/georouting/regions` | Add a cloud region |
| `DELETE` | `/v1/georouting/regions/{id}` | Remove a region |
| `GET` | `/v1/georouting/route?entity=X` | Route request to best region |
| `GET` | `/v1/georouting/stats` | Routing statistics |
| `GET` | `/v1/georouting/metrics` | Latency and throughput metrics |

**Example — Route a request:**

```bash
curl "http://localhost:8080/v1/georouting/route?entity=user:123"
```

```json
{
  "success": true,
  "data": {
    "entity": "user:123",
    "region": "eu-west-1",
    "latency_ms": 12,
    "reason": "data_residency"
  }
}
```

---

### A/B Rollout

**Maturity:** beta

Feature versioning with canary rollouts and quality gates.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/rollouts` | List rollouts |
| `POST` | `/v1/rollouts` | Start a canary rollout |
| `GET` | `/v1/rollouts/{id}` | Get rollout details |
| `POST` | `/v1/rollouts/{id}/advance` | Advance to next traffic step |
| `POST` | `/v1/rollouts/{id}/pause` | Pause a rollout |
| `POST` | `/v1/rollouts/{id}/rollback` | Rollback to base version |
| `GET` | `/v1/rollouts/{id}/quality` | Evaluate quality gates |
| `GET` | `/v1/rollouts/resolve?feature=X&entity=Y` | Resolve version for entity |
| `GET` | `/v1/rollouts/stats` | Rollout statistics |

**Example — Start a canary rollout:**

```bash
curl -X POST http://localhost:8080/v1/rollouts \
  -H "Content-Type: application/json" \
  -d '{
    "feature": "user_click_count",
    "base_version": "v1",
    "canary_version": "v2",
    "traffic_steps": [5, 25, 50, 100]
  }'
```

```json
{
  "success": true,
  "data": {
    "id": "rollout_xyz",
    "status": "active",
    "current_step": 0,
    "canary_traffic_pct": 5
  }
}
```

---

### Edge Runtime

**Maturity:** beta

Lightweight edge runtime with offline-first sync and WASM module deployment.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/edge/devices` | List edge devices |
| `POST` | `/v1/edge/devices` | Register a new device |
| `GET` | `/v1/edge/devices/{id}` | Get device details |
| `GET` | `/v1/edge/devices/{id}/stats` | Get device statistics |
| `GET` | `/v1/edge/devices/{id}/pending` | Get pending sync items |
| `POST` | `/v1/edge/devices/{id}/sync` | Trigger sync for device |
| `POST` | `/v1/edge/devices/{id}/heartbeat` | Device heartbeat |
| `POST` | `/v1/edge/devices/{id}/deploy/{moduleId}` | Deploy module to device |
| `GET` | `/v1/edge/modules` | List edge modules |
| `POST` | `/v1/edge/modules` | Register a module |
| `GET` | `/v1/edge/modules/{id}` | Get module details |
| `DELETE` | `/v1/edge/modules/{id}` | Remove a module |
| `GET` | `/v1/edge/stats` | Edge fleet statistics |

**Example — Trigger device sync:**

```bash
curl -X POST http://localhost:8080/v1/edge/devices/device-001/sync
```

```json
{
  "success": true,
  "data": {
    "device_id": "device-001",
    "synced_features": 142,
    "sync_duration_ms": 230
  }
}
```

---

### Additional Extension Handlers

The following handlers are registered but not documented in detail here. Enable them via `EnabledFeatures` and run `make api-routes` to see their maturity levels.

#### Stable Handlers

| Handler | Route Prefix | Description |
|---------|-------------|-------------|
| `groups` | `/v1/schema/groups` | Feature group management |
| `backfill` | `/v1/backfill/` | Historical data backfill jobs |
| `streaming` | `/v1/streaming/` | Real-time streaming pipelines |
| `catalog` | `/v1/catalog/` | Feature catalog and discovery |
| `auth` | `/v1/auth/` | API key management and RBAC |
| `ml` | `/v1/models/` | ML model serving and inference |
| `transform` | `/v1/transforms/` | Feature transformation pipelines |
| `cache` | `/v1/cache/` | Cache management and warming |
| `consistency` | `/v1/consistency/` | Data consistency checks |
| `observability` | `/v1/observability/` | Metrics and tracing configuration |
| `benchmark` | `/v1/benchmarks/` | Performance benchmarking |
| `impact` | `/v1/impact/` | Feature impact analysis |
| `model_serving` | `/v1/models/` | Model deployment and serving |
| `governance` | `/v1/governance/` | Data governance and compliance |
| `freshness` | `/v1/freshness/` | Feature freshness SLAs |
| `sla` | `/v1/sla/` | SLA management and monitoring |
| `drift` | `/v1/drift/` | Statistical drift detection |
| `semantic` | `/v1/semantic/` | Semantic search for features |
| `quality` | `/v1/quality/` | Data quality validation |

#### Beta Handlers

| Handler | Route Prefix | Description |
|---------|-------------|-------------|
| `tenant` | `/v1/tenants/` | Multi-tenant isolation |
| `warehouse` | `/v1/warehouse/` | Data warehouse sync |
| `embedding` | `/v1/embeddings/` | Embedding generation |
| `composition` | `/v1/composition/` | Feature composition DAGs |
| `migration` | `/v1/migrations/` | Schema migration management |
| `saas` | `/v1/saas/` | SaaS provisioning and billing |
| `cost` | `/v1/cost/` | Cost tracking and budgets |
| `scheduler` | `/v1/scheduler/` | Job scheduling (cron) |
| `lineage` | `/v1/lineage/` | Feature lineage tracking |
| `federation` | `/v1/federation/` | Distributed federation |
| `experiment` | `/v1/experiments/` | A/B experiment management |
| `dbt` | `/v1/dbt/` | dbt integration |
| `compute` | `/v1/compute/` | Compute engine |
| `consensus` | `/v1/consensus/` | Raft consensus |
| `stream_sql` | `/v1/stream-sql/` | Stream SQL processing |
| `control_plane` | `/v1/controlplane/` | Control plane management |
| `versioning` | `/v1/versioning/` | Feature versioning (branches, tags) |
| `validation` | `/v1/validation/` | Schema validation rules |
| `billing` | `/v1/billing/` | Usage billing |
| `contracts` | `/v1/contracts/` | Feature contracts |
| `materialization` | `/v1/materialization/` | Feature materialization |
| `replication` | `/v1/replication/` | Data replication |

#### Experimental Handlers

| Handler | Route Prefix | Description |
|---------|-------------|-------------|
| `graphql` | `/v1/graphql` | GraphQL API |
| `wasm` | `/v1/wasm/` | WASM runtime for transforms |
| `rag` | `/v1/rag/` | RAG pipeline |
| `plugin` | `/v1/plugins/` | Plugin system |
| `playground` | `/v1/playground/` | API playground |
| `gitops` | `/v1/gitops/` | GitOps schema management |
| `time_travel` | `/v1/timetravel/` | Time travel debugging |
| `llm_gateway` | `/v1/llm/gateway/` | LLM gateway routing |
| `compute_graph` | `/v1/compute-graph/` | Compute graph engine |

> **Tip:** Run `make api-routes` for a complete list of all handlers with maturity levels and enabled status.

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
