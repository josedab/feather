# HTTP Ingestion API

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

