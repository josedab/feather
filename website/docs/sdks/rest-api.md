---
sidebar_position: 3
title: REST API
description: HTTP REST API reference for Feather feature store.
---

# REST API Reference

Feather exposes a RESTful HTTP API for all operations. This reference covers all endpoints, request formats, and response structures.

## Base URL

```
http://localhost:8080/v1
```

All API endpoints are prefixed with `/v1` for versioning.

## Authentication

If authentication is enabled:

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" \
  http://localhost:8080/v1/features?entity=user:123
```

## Common Headers

| Header | Description |
|--------|-------------|
| `Content-Type` | `application/json` for request bodies |
| `Accept` | `application/json` (default) |
| `Accept-Encoding` | `gzip` for compressed responses |
| `X-Request-ID` | Optional request ID for tracing |

## Response Format

All responses follow this structure:

```json
{
  "success": true,
  "data": { ... },
  "error": null
}
```

Error responses:

```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "NOT_FOUND",
    "message": "Entity not found: user:999"
  }
}
```

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `NOT_FOUND` | 404 | Entity or feature not found |
| `INVALID_ARGUMENT` | 400 | Invalid request parameters |
| `INTERNAL_ERROR` | 500 | Server error |
| `UNAVAILABLE` | 503 | Service unavailable |
| `TIMEOUT` | 504 | Request timeout |

---

## Feature Endpoints

### Get Features

Retrieve feature values for an entity.

```http
GET /v1/features?entity={entity}&feature={feature1}&feature={feature2}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `entity` | string | Yes | Entity key (e.g., `user:123`) |
| `feature` | string | No | Feature names (repeat for multiple). If omitted, returns all. |

**Example:**

```bash
curl "http://localhost:8080/v1/features?entity=user:123&feature=click_count&feature=purchase_total"
```

**Response:**

```json
{
  "success": true,
  "data": {
    "entity": "user:123",
    "features": {
      "click_count": 42,
      "purchase_total": 299.99
    }
  }
}
```

---

### Store Features

Store feature values for an entity.

```http
POST /v1/features
```

**Request Body:**

```json
{
  "entity": "user:123",
  "features": {
    "click_count": 42,
    "purchase_total": 299.99,
    "is_premium": true
  }
}
```

**With Timestamp (for backfill):**

```json
{
  "entity": "user:123",
  "features": {
    "click_count": 42
  },
  "timestamp": "2024-01-15T10:00:00Z"
}
```

**Example:**

```bash
curl -X POST http://localhost:8080/v1/features \
  -H "Content-Type: application/json" \
  -d '{
    "entity": "user:123",
    "features": {
      "click_count": 42,
      "purchase_total": 299.99
    }
  }'
```

**Response:**

```json
{
  "success": true,
  "data": {
    "entity": "user:123",
    "stored": 2
  }
}
```

---

### Batch Get Features

Retrieve features for multiple entities.

```http
POST /v1/features/batch
```

**Request Body:**

```json
{
  "entities": ["user:123", "user:456", "user:789"],
  "features": ["click_count", "purchase_total"]
}
```

**Example:**

```bash
curl -X POST http://localhost:8080/v1/features/batch \
  -H "Content-Type: application/json" \
  -d '{
    "entities": ["user:123", "user:456"],
    "features": ["click_count", "purchase_total"]
  }'
```

**Response:**

```json
{
  "success": true,
  "data": {
    "entities": {
      "user:123": {
        "click_count": 42,
        "purchase_total": 299.99
      },
      "user:456": {
        "click_count": 15,
        "purchase_total": 149.99
      }
    }
  }
}
```

---

### Batch Store Features

Store features for multiple entities.

```http
POST /v1/features/batch/write
```

**Request Body:**

```json
{
  "updates": [
    {
      "entity": "user:123",
      "features": {"click_count": 42}
    },
    {
      "entity": "user:456",
      "features": {"click_count": 15}
    }
  ]
}
```

**Response:**

```json
{
  "success": true,
  "data": {
    "stored": 2
  }
}
```

---

## Point-in-Time Endpoints

### Get Features As Of

Retrieve feature values as they existed at a specific time.

```http
GET /v1/features/history?entity={entity}&feature={feature}&as_of={timestamp}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `entity` | string | Yes | Entity key |
| `feature` | string | No | Feature names |
| `as_of` | string | Yes | RFC3339 timestamp |

**Example:**

```bash
curl "http://localhost:8080/v1/features/history?entity=user:123&feature=click_count&as_of=2024-01-15T00:00:00Z"
```

**Response:**

```json
{
  "success": true,
  "data": {
    "entity": "user:123",
    "as_of": "2024-01-15T00:00:00Z",
    "features": {
      "click_count": {
        "value": 35,
        "timestamp": "2024-01-14T18:30:00Z"
      }
    }
  }
}
```

---

### Batch Point-in-Time

Query multiple entities at different timestamps.

```http
POST /v1/features/history/batch
```

**Request Body:**

```json
{
  "queries": [
    {"entity": "user:123", "as_of": "2024-01-15T00:00:00Z"},
    {"entity": "user:456", "as_of": "2024-01-16T00:00:00Z"}
  ],
  "features": ["click_count", "purchase_total"]
}
```

**Response:**

```json
{
  "success": true,
  "data": {
    "results": [
      {
        "entity": "user:123",
        "as_of": "2024-01-15T00:00:00Z",
        "features": {
          "click_count": {"value": 35, "timestamp": "2024-01-14T18:30:00Z"},
          "purchase_total": {"value": 250.00, "timestamp": "2024-01-14T12:00:00Z"}
        }
      },
      {
        "entity": "user:456",
        "as_of": "2024-01-16T00:00:00Z",
        "features": {
          "click_count": {"value": 12, "timestamp": "2024-01-15T20:00:00Z"}
        }
      }
    ]
  }
}
```

---

### Get History Range

Get the available history range for a feature.

```http
GET /v1/features/history/range?entity={entity}&feature={feature}
```

**Response:**

```json
{
  "success": true,
  "data": {
    "entity": "user:123",
    "feature": "click_count",
    "oldest_version": "2023-12-01T00:00:00Z",
    "newest_version": "2024-01-20T15:30:00Z",
    "version_count": 45
  }
}
```

---

## Vector Endpoints

### List Indexes

```http
GET /v1/vectors
```

**Response:**

```json
{
  "success": true,
  "data": {
    "indexes": [
      {
        "name": "product_embeddings",
        "dimensions": 384,
        "metric": "cosine",
        "vector_count": 100000
      }
    ]
  }
}
```

---

### Create Index

```http
POST /v1/vectors
```

**Request Body:**

```json
{
  "name": "product_embeddings",
  "dimensions": 384,
  "metric": "cosine",
  "hnsw": {
    "m": 16,
    "ef_construction": 200
  }
}
```

---

### Get Index Info

```http
GET /v1/vectors/{index}
```

**Response:**

```json
{
  "success": true,
  "data": {
    "name": "product_embeddings",
    "dimensions": 384,
    "metric": "cosine",
    "vector_count": 100000,
    "hnsw": {
      "m": 16,
      "ef_construction": 200
    }
  }
}
```

---

### Delete Index

```http
DELETE /v1/vectors/{index}
```

---

### Upsert Vectors

```http
POST /v1/vectors/{index}/upsert
```

**Request Body:**

```json
{
  "vectors": [
    {
      "id": "product:123",
      "values": [0.1, 0.2, 0.3, ...],
      "metadata": {
        "category": "electronics",
        "price": 299.99
      }
    }
  ]
}
```

---

### Search Vectors

```http
POST /v1/vectors/{index}/search
```

**Request Body:**

```json
{
  "vector": [0.1, 0.2, 0.3, ...],
  "top_k": 10,
  "filter": {
    "category": {"$eq": "electronics"},
    "price": {"$lt": 500}
  }
}
```

**Response:**

```json
{
  "success": true,
  "data": {
    "results": [
      {
        "id": "product:456",
        "score": 0.95,
        "metadata": {"category": "electronics", "price": 349.99}
      },
      {
        "id": "product:789",
        "score": 0.87,
        "metadata": {"category": "electronics", "price": 199.99}
      }
    ]
  }
}
```

---

### Get Vector

```http
GET /v1/vectors/{index}/{id}
```

---

### Delete Vector

```http
DELETE /v1/vectors/{index}/{id}
```

---

## Schema Endpoints

### List Feature Groups

```http
GET /v1/schema/groups
```

**Response:**

```json
{
  "success": true,
  "data": {
    "groups": [
      {
        "name": "user_engagement",
        "entity_type": "user",
        "ttl": "24h",
        "features": [
          {"name": "click_count", "data_type": "int64"},
          {"name": "purchase_total", "data_type": "float64"}
        ]
      }
    ]
  }
}
```

---

### Get Feature Group

```http
GET /v1/schema/groups/{name}
```

---

### Create Feature Group

```http
POST /v1/schema/groups
```

**Request Body:**

```json
{
  "name": "user_engagement",
  "entity_type": "user",
  "ttl": "24h",
  "features": [
    {"name": "click_count", "data_type": "int64"},
    {"name": "purchase_total", "data_type": "float64"}
  ]
}
```

---

## Drift Endpoints

### Get Drift Status

```http
GET /v1/drift/status
```

**Response:**

```json
{
  "success": true,
  "data": {
    "features": [
      {
        "feature": "user:click_count",
        "status": "healthy",
        "metric_value": 0.02,
        "threshold": 0.05,
        "last_check": "2024-01-15T10:30:00Z"
      }
    ]
  }
}
```

---

### Get Drift Alerts

```http
GET /v1/drift/alerts?since={timestamp}
```

---

### Register Feature for Drift

```http
POST /v1/drift/register
```

**Request Body:**

```json
{
  "feature": "user:purchase_total",
  "window_size": 1000,
  "detection_method": "ks",
  "threshold": 0.05
}
```

---

### Reset Reference Distribution

```http
POST /v1/drift/reset/{feature}
```

---

## Freshness Endpoints

### Get Freshness Status

```http
GET /v1/freshness/status
```

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `feature` | string | No | Filter to specific feature |

**Response:**

```json
{
  "success": true,
  "data": {
    "features": [
      {
        "feature": "user:click_count",
        "freshness": {
          "min_age_ms": 100,
          "max_age_ms": 890000,
          "avg_age_ms": 45000,
          "p50_age_ms": 30000,
          "p99_age_ms": 600000
        },
        "sla": {
          "max_age_ms": 900000,
          "status": "healthy",
          "entities_stale": 1234,
          "entities_stale_pct": 0.12
        }
      }
    ]
  }
}
```

---

### Get Stale Entities

```http
GET /v1/freshness/stale?feature={feature}&limit={limit}
```

---

## Export Endpoints

### Export Features

```http
POST /v1/export
```

**Request Body:**

```json
{
  "format": "parquet",
  "entities": ["user:*"],
  "features": ["click_count", "purchase_total"],
  "output_path": "/exports/features.parquet"
}
```

**Response:**

```json
{
  "success": true,
  "data": {
    "export_id": "exp-a1b2c3d4",
    "status": "completed",
    "output_path": "/exports/features.parquet",
    "stats": {
      "rows_exported": 1000000,
      "file_size_bytes": 52428800,
      "duration_ms": 12500
    }
  }
}
```

---

### Point-in-Time Export

```http
POST /v1/export/pit
```

**Request Body:**

```json
{
  "format": "parquet",
  "queries": [
    {"entity": "user:123", "as_of": "2024-01-15T00:00:00Z"},
    {"entity": "user:456", "as_of": "2024-01-16T00:00:00Z"}
  ],
  "features": ["click_count", "purchase_total"],
  "output_path": "/exports/training.parquet"
}
```

---

## Health Endpoints

### Liveness Probe

```http
GET /live
```

Returns `200 OK` if the process is running.

---

### Readiness Probe

```http
GET /ready
```

Returns `200 OK` if ready to serve traffic.

---

### Health Check

```http
GET /health
```

**Response:**

```json
{
  "status": "healthy",
  "components": {
    "hot_tier": {"status": "healthy", "memory_used": "2.1GB"},
    "warm_tier": {"status": "healthy", "disk_used": "15GB"},
    "http_server": {"status": "healthy"},
    "grpc_server": {"status": "healthy"}
  }
}
```

---

## Ingestion Endpoint

### HTTP Push

```http
POST /ingest
```

**Request Body:**

```json
{
  "entity": "user:123",
  "features": {
    "click_count": 43
  }
}
```

### Bulk Ingestion

```http
POST /ingest/bulk
```

**Request Body:**

```json
{
  "updates": [
    {"entity": "user:123", "features": {"click_count": 43}},
    {"entity": "user:456", "features": {"click_count": 16}}
  ]
}
```

---

## Rate Limiting

When rate limited, you'll receive:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 1

{
  "success": false,
  "error": {
    "code": "RATE_LIMITED",
    "message": "Rate limit exceeded"
  }
}
```

---

## Pagination

For endpoints that return lists, use pagination:

```http
GET /v1/schema/groups?limit=10&offset=0
```

**Response includes pagination info:**

```json
{
  "success": true,
  "data": {
    "groups": [...],
    "pagination": {
      "total": 25,
      "limit": 10,
      "offset": 0,
      "has_more": true
    }
  }
}
```

---

## Related Documentation

- [Go SDK](./go) - Go client library
- [Python SDK](./python) - Python client library
- [Architecture](/docs/concepts/architecture) - System design
