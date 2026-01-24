# Getting Started with Feather in 5 Minutes

Store and retrieve your first ML features in under 5 minutes.

**Time:** ~5 minutes

## What You'll Learn

- How to install and start Feather
- How to store features for an entity
- How to retrieve features individually and in batch
- How to query point-in-time feature history

## Prerequisites

- Go 1.24+ installed
- curl available in your terminal
- A free terminal/port 8080

---

## Step 1: Install Feather

**Option A — go install (recommended):**

```bash
$ go install github.com/feather-store/feather/cmd/feather@latest
```

**Option B — build from source:**

```bash
$ git clone https://github.com/feather-store/feather.git
$ cd feather
$ make build
```

The binary is compiled to `bin/feather` (or `$GOPATH/bin/feather` with go install).

---

## Step 2: Start the Server

Start Feather with the built-in development configuration (no external dependencies required):

```bash
$ make run-dev
```

This uses `configs/feather-dev.yaml` which runs entirely in-memory with a sample `user_features` schema already defined.

You should see output like:

```output
{"time":"...","level":"INFO","msg":"starting feather feature store","version":"dev"}
{"time":"...","level":"INFO","msg":"storage initialized","hot_tier":"memory","warm_tier":"memory"}
{"time":"...","level":"INFO","msg":"registered feature group","group":"user_features","entity_type":"user"}
{"time":"...","level":"INFO","msg":"http server started","port":8080}
{"time":"...","level":"INFO","msg":"grpc server started","port":50051}
{"time":"...","level":"INFO","msg":"ingestion server started","port":8081}
```

> **Tip:** Leave this terminal running and open a new terminal for the remaining steps.

---

## Step 3: Check Health

Verify the server is running:

```bash
$ curl -s http://localhost:8080/health | jq .
```

Expected output:

```json
{
  "status": "healthy",
  "components": {
    "hot_storage": "healthy",
    "warm_storage": "healthy",
    "aggregation_engine": "healthy"
  },
  "uptime": "5s"
}
```

You can also check the simpler Kubernetes-style probes:

```bash
$ curl -s http://localhost:8080/ready
```

```output
OK
```

---

## Step 4: Store Your First Features

Store age and activity score for user `1001`:

```bash
$ curl -s -X POST http://localhost:8080/v1/features \
  -H "Content-Type: application/json" \
  -d '{
    "entity_key": "user:1001",
    "features": {
      "click_count": 42,
      "purchase_total": 189.99,
      "last_activity": "2025-01-15T10:30:00Z"
    }
  }' | jq .
```

Expected output:

```json
{
  "status": "ok",
  "entity_key": "user:1001",
  "features_stored": 3
}
```

Store features for a second user:

```bash
$ curl -s -X POST http://localhost:8080/v1/features \
  -H "Content-Type: application/json" \
  -d '{
    "entity_key": "user:1002",
    "features": {
      "click_count": 7,
      "purchase_total": 34.50,
      "last_activity": "2025-01-14T16:45:00Z"
    }
  }' | jq .
```

```json
{
  "status": "ok",
  "entity_key": "user:1002",
  "features_stored": 3
}
```

---

## Step 5: Retrieve Features

Get all features for user `1001`:

```bash
$ curl -s "http://localhost:8080/v1/features?entity=user:1001" | jq .
```

Expected output:

```json
{
  "entity_key": "user:1001",
  "features": {
    "click_count": {
      "value": 42,
      "timestamp": "2025-01-15T10:30:00Z",
      "version": 1
    },
    "purchase_total": {
      "value": 189.99,
      "timestamp": "2025-01-15T10:30:00Z",
      "version": 1
    },
    "last_activity": {
      "value": "2025-01-15T10:30:00Z",
      "timestamp": "2025-01-15T10:30:00Z",
      "version": 1
    }
  }
}
```

Retrieve a specific feature:

```bash
$ curl -s "http://localhost:8080/v1/features?entity=user:1001&feature=click_count" | jq .
```

```json
{
  "entity_key": "user:1001",
  "features": {
    "click_count": {
      "value": 42,
      "timestamp": "2025-01-15T10:30:00Z",
      "version": 1
    }
  }
}
```

---

## Step 6: Batch Retrieval

Retrieve features for multiple entities in a single request:

```bash
$ curl -s -X POST http://localhost:8080/v1/features/batch \
  -H "Content-Type: application/json" \
  -d '{
    "entities": ["user:1001", "user:1002"],
    "features": ["click_count", "purchase_total"]
  }' | jq .
```

Expected output:

```json
{
  "results": {
    "user:1001": {
      "click_count": {
        "value": 42,
        "timestamp": "2025-01-15T10:30:00Z",
        "version": 1
      },
      "purchase_total": {
        "value": 189.99,
        "timestamp": "2025-01-15T10:30:00Z",
        "version": 1
      }
    },
    "user:1002": {
      "click_count": {
        "value": 7,
        "timestamp": "2025-01-14T16:45:00Z",
        "version": 1
      },
      "purchase_total": {
        "value": 34.50,
        "timestamp": "2025-01-14T16:45:00Z",
        "version": 1
      }
    }
  }
}
```

---

## Step 7: View the Feature Schema

List the registered feature groups:

```bash
$ curl -s http://localhost:8080/v1/schema/groups | jq .
```

Expected output:

```json
{
  "groups": [
    {
      "name": "user_features",
      "entity_type": "user",
      "ttl": "24h0m0s",
      "features": [
        {"name": "click_count", "data_type": "int64"},
        {"name": "purchase_total", "data_type": "float64"},
        {"name": "last_activity", "data_type": "timestamp"}
      ]
    }
  ]
}
```

---

## What's Next?

You now have a running feature store with features stored and retrievable. Here are some next steps:

- **[Real-Time Fraud Detection](02-fraud-detection.md)** — Build a fraud detection pipeline with drift monitoring
- **[Migrating from Feast](03-migrating-from-feast.md)** — Bring your existing Feast setup to Feather
- **[Managing LLM Features](04-llm-features.md)** — Version prompts and cache LLM responses
- **[Deploying on Kubernetes](05-kubernetes-deployment.md)** — Take Feather to production

### Useful Commands

```bash
# List all available API routes and their maturity levels
$ make api-routes

# Run the full test suite
$ make test

# See all Makefile targets
$ make help
```
