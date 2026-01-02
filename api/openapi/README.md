# Feather OpenAPI Specification

This directory contains the OpenAPI 3.0 specification for Feather's HTTP REST API.

## Files

- `feather.yaml` - Complete OpenAPI 3.0 specification

## Usage

### View Documentation

**Swagger UI:**
```bash
# Using Docker
docker run -p 8080:8080 -e SWAGGER_JSON=/spec/feather.yaml -v $(pwd):/spec swaggerapi/swagger-ui

# Then open http://localhost:8080
```

**Redoc:**
```bash
# Using npx
npx @redocly/cli preview-docs feather.yaml

# Or with Docker
docker run -p 8080:80 -v $(pwd)/feather.yaml:/usr/share/nginx/html/spec.yaml -e SPEC_URL=spec.yaml redocly/redoc
```

### Validate Specification

```bash
# Using Redocly CLI
npx @redocly/cli lint feather.yaml

# Using Spectral
npx @stoplight/spectral-cli lint feather.yaml
```

### Generate Client SDKs

**Using OpenAPI Generator:**

```bash
# Install OpenAPI Generator
brew install openapi-generator

# Generate Python client
openapi-generator generate -i feather.yaml -g python -o ./sdk/python-generated

# Generate TypeScript client
openapi-generator generate -i feather.yaml -g typescript-fetch -o ./sdk/typescript-generated

# Generate Go client
openapi-generator generate -i feather.yaml -g go -o ./sdk/go-generated

# Generate Java client
openapi-generator generate -i feather.yaml -g java -o ./sdk/java-generated
```

**Using oapi-codegen (Go):**
```bash
go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest
oapi-codegen -generate types,client -package feather feather.yaml > client.go
```

### Generate Server Stubs

```bash
# Generate Go server (Chi router)
openapi-generator generate -i feather.yaml -g go-server -o ./server-generated

# Generate Go server (Echo)
openapi-generator generate -i feather.yaml -g go-echo-server -o ./server-generated
```

## API Overview

### Core Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/features` | Get features for an entity |
| POST | `/v1/features` | Store features for an entity |
| POST | `/v1/features/batch` | Batch get features |
| GET | `/v1/features/history` | Point-in-time query |

### Schema Management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/schema/groups` | List feature groups |
| POST | `/v1/schema/groups` | Create feature group |
| GET | `/v1/schema/groups/{name}` | Get feature group |
| DELETE | `/v1/schema/groups/{name}` | Delete feature group |

### Vector Similarity

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/vectors` | List vector indexes |
| POST | `/v1/vectors` | Create vector index |
| POST | `/v1/vectors/{index}/upsert` | Upsert vectors |
| POST | `/v1/vectors/{index}/search` | Search similar vectors |

### Monitoring

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/drift/status` | Drift detection status |
| GET | `/v1/drift/alerts` | Drift alerts |
| GET | `/v1/freshness/status` | Feature freshness |
| GET | `/v1/freshness/sla` | SLA compliance |

### Health

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Deep health check |
| GET | `/ready` | Readiness probe |
| GET | `/live` | Liveness probe |

## Authentication

The API supports API key authentication via the `X-API-Key` header:

```bash
curl -H "X-API-Key: your-api-key" https://feather.example.com/v1/features?entity=user:123
```

## Rate Limiting

Requests are rate-limited per client IP. Rate limit headers are included in responses:

- `X-RateLimit-Limit`: Maximum requests per window
- `X-RateLimit-Remaining`: Remaining requests
- `X-RateLimit-Reset`: Unix timestamp when window resets

## Error Handling

All errors return a consistent JSON structure:

```json
{
  "success": false,
  "error": {
    "code": "INVALID_REQUEST",
    "message": "Entity ID is required"
  },
  "request_id": "req_abc123"
}
```

## Examples

### Get Features

```bash
curl "http://localhost:8080/v1/features?entity=user:12345&features=click_count,last_active"
```

Response:
```json
{
  "success": true,
  "data": {
    "entity_id": "user:12345",
    "features": {
      "click_count": {
        "value": 42,
        "timestamp": "2024-01-15T10:30:00Z",
        "version": 1
      },
      "last_active": {
        "value": "2024-01-15T09:00:00Z",
        "timestamp": "2024-01-15T10:30:00Z",
        "version": 1
      }
    }
  },
  "request_id": "req_abc123"
}
```

### Store Features

```bash
curl -X POST "http://localhost:8080/v1/features" \
  -H "Content-Type: application/json" \
  -d '{
    "entity_id": "user:12345",
    "features": {
      "click_count": 42,
      "last_active": "2024-01-15T10:30:00Z"
    }
  }'
```

### Vector Search

```bash
curl -X POST "http://localhost:8080/v1/vectors/product_embeddings/search" \
  -H "Content-Type: application/json" \
  -d '{
    "vector": [0.1, 0.2, 0.3, ...],
    "top_k": 10,
    "include_metadata": true
  }'
```

## Versioning

The API is versioned via URL path (`/v1/`). Breaking changes will be introduced in new versions (`/v2/`).

## Contributing

When adding new endpoints:

1. Update `feather.yaml` with the new endpoint
2. Run validation: `npx @redocly/cli lint feather.yaml`
3. Update this README with endpoint documentation
4. Regenerate any auto-generated clients
