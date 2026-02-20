# API Overview

> Overview, authentication, and common patterns for Feather APIs.

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

