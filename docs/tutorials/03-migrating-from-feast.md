# Migrating from Feast to Feather

Move your existing Feast feature store to Feather using the built-in compatibility API, then gradually adopt native Feather features.

**Time:** ~15 minutes

## What You'll Learn

- How Feast concepts map to Feather
- How to use the Feast-compatible API for zero-change migration
- How to register feature view mappings
- How to validate feature parity between Feast and Feather
- How to gradually migrate to native Feather APIs

## Prerequisites

- Feather running locally (`make run-dev`) — see [Getting Started](01-getting-started.md)
- Familiarity with Feast concepts (feature views, entities, feature services)
- curl and jq installed

---

## Concept Mapping

Before migrating, understand how Feast concepts translate to Feather:

| Feast Concept | Feather Equivalent | Notes |
|---|---|---|
| Entity | Entity Type | Same idea — the primary key for feature lookup (e.g., `user`, `driver`) |
| Feature View | Feature Group | Contains feature definitions with types and metadata |
| Feature Service | Batch Request | Group multiple feature groups in a single query |
| Online Store | Hot Tier | In-memory, sub-millisecond retrieval |
| Offline Store | Warm Tier | BadgerDB-backed persistent storage with history |
| `feast materialize` | Automatic | Features flow through hot → warm tiers automatically |
| `feast apply` | `POST /v1/schema/groups` | Register schemas via API instead of Python decorators |
| Feature Reference (`view:feature`) | `group.feature` | Dot notation instead of colon |
| Registry | Schema Registry | Built-in, no external database needed |

---

## Step 1: Register Your Feast Feature Views as Feather Groups

Suppose your Feast project has these feature views:

```python
# Feast Python definition (for reference)
driver_stats = FeatureView(
    name="driver_stats",
    entities=[driver],
    schema=[
        Field(name="conv_rate", dtype=Float64),
        Field(name="acc_rate", dtype=Float64),
        Field(name="avg_daily_trips", dtype=Int64),
    ],
    ttl=timedelta(days=1),
)
```

Register the equivalent in Feather:

```bash
$ curl -s -X POST http://localhost:8080/v1/schema/groups \
  -H "Content-Type: application/json" \
  -d '{
    "name": "driver_stats",
    "entity_type": "driver",
    "ttl": "24h",
    "features": [
      {"name": "conv_rate", "data_type": "float64"},
      {"name": "acc_rate", "data_type": "float64"},
      {"name": "avg_daily_trips", "data_type": "int64"}
    ]
  }' | jq .
```

Expected output:

```json
{
  "status": "ok",
  "group": {
    "name": "driver_stats",
    "entity_type": "driver",
    "ttl": "24h0m0s",
    "features": [
      {"name": "conv_rate", "data_type": "float64"},
      {"name": "acc_rate", "data_type": "float64"},
      {"name": "avg_daily_trips", "data_type": "int64"}
    ]
  }
}
```

---

## Step 2: Register the Feast Mapping

Create a mapping that tells Feather how to translate Feast-style requests to native lookups:

```bash
$ curl -s -X POST http://localhost:8080/v1/feast/mappings \
  -H "Content-Type: application/json" \
  -d '{
    "feature_view": "driver_stats",
    "feather_group": "driver_stats",
    "entity_mapping": {
      "driver_id": "driver"
    },
    "feature_mapping": {
      "conv_rate": "conv_rate",
      "acc_rate": "acc_rate",
      "avg_daily_trips": "avg_daily_trips"
    }
  }' | jq .
```

Expected output:

```json
{
  "status": "ok",
  "mapping": {
    "feature_view": "driver_stats",
    "feather_group": "driver_stats",
    "entity_mapping": {"driver_id": "driver"},
    "feature_mapping": {
      "conv_rate": "conv_rate",
      "acc_rate": "acc_rate",
      "avg_daily_trips": "avg_daily_trips"
    }
  }
}
```

Verify the mapping was registered:

```bash
$ curl -s http://localhost:8080/v1/feast/mappings | jq .
```

```json
{
  "mappings": [
    {
      "feature_view": "driver_stats",
      "feather_group": "driver_stats",
      "entity_mapping": {"driver_id": "driver"},
      "feature_mapping": {
        "conv_rate": "conv_rate",
        "acc_rate": "acc_rate",
        "avg_daily_trips": "avg_daily_trips"
      }
    }
  ]
}
```

---

## Step 3: Ingest Sample Data

Load some driver features so we can test retrieval:

```bash
$ curl -s -X POST http://localhost:8080/v1/features \
  -H "Content-Type: application/json" \
  -d '{
    "entity_key": "driver:1001",
    "features": {
      "conv_rate": 0.85,
      "acc_rate": 0.92,
      "avg_daily_trips": 15
    }
  }' | jq .
```

```bash
$ curl -s -X POST http://localhost:8080/v1/features \
  -H "Content-Type: application/json" \
  -d '{
    "entity_key": "driver:1002",
    "features": {
      "conv_rate": 0.72,
      "acc_rate": 0.88,
      "avg_daily_trips": 22
    }
  }' | jq .
```

---

## Step 4: Use the Feast-Compatible API

The Feast compatibility endpoint accepts requests in the same format as Feast's online serving API. Your existing Feast SDK clients can point to Feather with no code changes:

```bash
$ curl -s -X POST http://localhost:8080/v1/feast/get-online-features \
  -H "Content-Type: application/json" \
  -d '{
    "features": [
      "driver_stats:conv_rate",
      "driver_stats:acc_rate",
      "driver_stats:avg_daily_trips"
    ],
    "entities": {
      "driver_id": ["1001", "1002"]
    }
  }' | jq .
```

Expected output (Feast-compatible response format):

```json
{
  "metadata": {
    "feature_names": ["driver_id", "conv_rate", "acc_rate", "avg_daily_trips"]
  },
  "results": [
    {
      "values": ["1001", 0.85, 0.92, 15],
      "statuses": ["PRESENT", "PRESENT", "PRESENT", "PRESENT"],
      "event_timestamps": [
        "2025-01-15T10:00:00Z",
        "2025-01-15T10:00:00Z",
        "2025-01-15T10:00:00Z",
        "2025-01-15T10:00:00Z"
      ]
    },
    {
      "values": ["1002", 0.72, 0.88, 22],
      "statuses": ["PRESENT", "PRESENT", "PRESENT", "PRESENT"],
      "event_timestamps": [
        "2025-01-15T10:00:00Z",
        "2025-01-15T10:00:00Z",
        "2025-01-15T10:00:00Z",
        "2025-01-15T10:00:00Z"
      ]
    }
  ]
}
```

> **Note:** Feather also supports the `feature_view__feature` format (double underscore) used by some Feast SDK versions.

---

## Step 5: Update Your Feast SDK Client

If you use the Feast Python SDK, point it at Feather's compatibility endpoint:

```python
# Before (Feast)
from feast import FeatureStore
store = FeatureStore(repo_path="./feature_repo")

# After (Feather compatibility mode)
# Option A: Use Feather's Feast-compat endpoint directly
import requests

response = requests.post(
    "http://localhost:8080/v1/feast/get-online-features",
    json={
        "features": [
            "driver_stats:conv_rate",
            "driver_stats:acc_rate",
        ],
        "entities": {
            "driver_id": ["1001"]
        }
    }
)
features = response.json()

# Option B: Configure Feast SDK to use Feather as the online store
# In feature_store.yaml:
# online_store:
#   type: http
#   endpoint: http://feather-host:8080/v1/feast
```

---

## Step 6: Validate Feature Parity

Check the Feast compatibility statistics to verify all requests are being served correctly:

```bash
$ curl -s http://localhost:8080/v1/feast/stats | jq .
```

Expected output:

```json
{
  "total_requests": 1,
  "successful_requests": 1,
  "failed_requests": 0,
  "feature_views_served": ["driver_stats"],
  "unmapped_features": [],
  "avg_latency_ms": 0.45
}
```

Key things to verify:
- `failed_requests` should be 0
- `unmapped_features` should be empty — any entries here indicate Feast features that don't have a Feather mapping

---

## Step 7: Gradual Migration to Native API

Once the compatibility layer is working, you can gradually migrate to Feather's native API for better performance and access to advanced features:

```bash
# Feast-compat API (works but adds translation overhead):
$ curl -s -X POST http://localhost:8080/v1/feast/get-online-features \
  -d '{"features": ["driver_stats:conv_rate"], "entities": {"driver_id": ["1001"]}}'

# Native Feather API (direct, faster):
$ curl -s "http://localhost:8080/v1/features?entity=driver:1001&feature=conv_rate"
```

### Migration Checklist

- [ ] All Feast feature views registered as Feather groups
- [ ] All entity mappings configured via `/v1/feast/mappings`
- [ ] Feast SDK clients pointed at Feather compatibility endpoint
- [ ] Feature parity validated (check `/v1/feast/stats`)
- [ ] Data pipeline updated to ingest into Feather
- [ ] Monitoring set up for drift detection
- [ ] Gradual migration of SDK clients to native Feather API

---

## What's Next?

- **[Managing LLM Features](04-llm-features.md)** — Use features not available in Feast
- **[Deploying on Kubernetes](05-kubernetes-deployment.md)** — Run Feather in production
