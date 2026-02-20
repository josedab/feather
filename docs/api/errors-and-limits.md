# Error Handling, Rate Limiting & Pagination

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
