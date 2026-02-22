# Real-Time Fraud Detection Features

Build a real-time fraud detection feature pipeline, ingest transaction data, monitor for distribution drift, and serve features to a scoring model.

**Time:** ~15 minutes

## What You'll Learn

- How to define a feature group for transaction data
- How to ingest features via the HTTP API
- How to set up drift monitoring to detect distribution shifts
- How to query features at serving time for a fraud model
- How to use point-in-time retrieval for training data

## Prerequisites

- Feather running locally (`make run-dev`) — see [Getting Started](01-getting-started.md)
- curl and jq installed
- A second terminal for API calls

---

## Scenario

You are building a real-time fraud scoring model. For each credit card transaction, you need to look up the cardholder's recent behavior to compute a fraud score. You will:

1. Define a feature group for transaction-level features
2. Ingest sample transaction data
3. Monitor the `amount` feature for distribution drift
4. Query features for real-time inference

---

## Step 1: Define a Transaction Feature Group

Register a feature group that describes the features your fraud model needs:

```bash
$ curl -s -X POST http://localhost:8080/v1/schema/groups \
  -H "Content-Type: application/json" \
  -d '{
    "name": "transaction_features",
    "entity_type": "card",
    "ttl": "72h",
    "features": [
      {"name": "amount", "data_type": "float64"},
      {"name": "merchant_category", "data_type": "string"},
      {"name": "hour_of_day", "data_type": "int64"},
      {"name": "is_international", "data_type": "bool"},
      {"name": "tx_count_24h", "data_type": "int64"},
      {"name": "avg_amount_7d", "data_type": "float64"}
    ]
  }' | jq .
```

Expected output:

```json
{
  "status": "ok",
  "group": {
    "name": "transaction_features",
    "entity_type": "card",
    "ttl": "72h0m0s",
    "features": [
      {"name": "amount", "data_type": "float64"},
      {"name": "merchant_category", "data_type": "string"},
      {"name": "hour_of_day", "data_type": "int64"},
      {"name": "is_international", "data_type": "bool"},
      {"name": "tx_count_24h", "data_type": "int64"},
      {"name": "avg_amount_7d", "data_type": "float64"}
    ]
  }
}
```

---

## Step 2: Ingest Transaction Features

Simulate ingesting features from a payment processing pipeline. Each entity key represents a credit card:

**Normal transaction — domestic purchase:**

```bash
$ curl -s -X POST http://localhost:8080/v1/features \
  -H "Content-Type: application/json" \
  -d '{
    "entity_key": "card:4532-xxxx-1234",
    "features": {
      "amount": 45.99,
      "merchant_category": "grocery",
      "hour_of_day": 14,
      "is_international": false,
      "tx_count_24h": 3,
      "avg_amount_7d": 52.30
    }
  }' | jq .
```

```json
{
  "status": "ok",
  "entity_key": "card:4532-xxxx-1234",
  "features_stored": 6
}
```

**Suspicious transaction — large international purchase at 3 AM:**

```bash
$ curl -s -X POST http://localhost:8080/v1/features \
  -H "Content-Type: application/json" \
  -d '{
    "entity_key": "card:4532-xxxx-5678",
    "features": {
      "amount": 2499.99,
      "merchant_category": "electronics",
      "hour_of_day": 3,
      "is_international": true,
      "tx_count_24h": 12,
      "avg_amount_7d": 89.50
    }
  }' | jq .
```

**Bulk ingest via the ingestion endpoint (port 8081):**

```bash
$ curl -s -X POST http://localhost:8081/ingest/batch \
  -H "Content-Type: application/json" \
  -d '{
    "events": [
      {
        "entity_key": "card:4532-xxxx-9012",
        "features": {
          "amount": 12.50,
          "merchant_category": "coffee_shop",
          "hour_of_day": 8,
          "is_international": false,
          "tx_count_24h": 1,
          "avg_amount_7d": 15.20
        }
      },
      {
        "entity_key": "card:4532-xxxx-3456",
        "features": {
          "amount": 199.00,
          "merchant_category": "clothing",
          "hour_of_day": 19,
          "is_international": false,
          "tx_count_24h": 5,
          "avg_amount_7d": 120.75
        }
      }
    ]
  }' | jq .
```

Expected output:

```json
{
  "status": "ok",
  "processed": 2,
  "errors": []
}
```

---

## Step 3: Set Up Drift Monitoring

Register the `amount` feature for distribution drift detection. Feather uses the Kolmogorov-Smirnov test for numeric features to detect when live data drifts from the reference distribution:

```bash
$ curl -s -X POST http://localhost:8080/v1/drift/register \
  -H "Content-Type: application/json" \
  -d '{
    "feature_name": "amount",
    "feature_type": "numeric",
    "window_size": 1000,
    "threshold": 0.05
  }' | jq .
```

Expected output:

```json
{
  "status": "ok",
  "feature": "amount",
  "detector": "ks_test",
  "window_size": 1000,
  "threshold": 0.05
}
```

Also register the `merchant_category` for categorical drift detection (uses Population Stability Index):

```bash
$ curl -s -X POST http://localhost:8080/v1/drift/register \
  -H "Content-Type: application/json" \
  -d '{
    "feature_name": "merchant_category",
    "feature_type": "categorical",
    "window_size": 500,
    "threshold": 0.1
  }' | jq .
```

---

## Step 4: Query Features for Real-Time Scoring

When a new transaction arrives, your fraud model needs the cardholder's features in real time. Query them:

```bash
$ curl -s "http://localhost:8080/v1/features?entity=card:4532-xxxx-5678" | jq .
```

Expected output:

```json
{
  "entity_key": "card:4532-xxxx-5678",
  "features": {
    "amount": {
      "value": 2499.99,
      "timestamp": "2025-01-15T03:12:00Z",
      "version": 1
    },
    "merchant_category": {
      "value": "electronics",
      "timestamp": "2025-01-15T03:12:00Z",
      "version": 1
    },
    "hour_of_day": {
      "value": 3,
      "timestamp": "2025-01-15T03:12:00Z",
      "version": 1
    },
    "is_international": {
      "value": true,
      "timestamp": "2025-01-15T03:12:00Z",
      "version": 1
    },
    "tx_count_24h": {
      "value": 12,
      "timestamp": "2025-01-15T03:12:00Z",
      "version": 1
    },
    "avg_amount_7d": {
      "value": 89.50,
      "timestamp": "2025-01-15T03:12:00Z",
      "version": 1
    }
  }
}
```

For batch scoring, fetch multiple cards at once:

```bash
$ curl -s -X POST http://localhost:8080/v1/features/batch \
  -H "Content-Type: application/json" \
  -d '{
    "entities": [
      "card:4532-xxxx-1234",
      "card:4532-xxxx-5678",
      "card:4532-xxxx-9012"
    ],
    "features": ["amount", "tx_count_24h", "is_international"]
  }' | jq .
```

---

## Step 5: Check Drift Alerts

After ingesting enough data, check whether any feature distributions have drifted:

```bash
$ curl -s http://localhost:8080/v1/drift/status | jq .
```

Expected output (no drift yet):

```json
{
  "monitors": [
    {
      "feature": "amount",
      "detector": "ks_test",
      "status": "healthy",
      "drift_score": 0.02,
      "threshold": 0.05,
      "samples_collected": 4,
      "window_size": 1000
    },
    {
      "feature": "merchant_category",
      "detector": "psi",
      "status": "healthy",
      "drift_score": 0.03,
      "threshold": 0.1,
      "samples_collected": 4,
      "window_size": 500
    }
  ]
}
```

Query for recent drift alerts:

```bash
$ curl -s "http://localhost:8080/v1/drift/alerts?since=2025-01-01T00:00:00Z" | jq .
```

```json
{
  "alerts": []
}
```

> **In production**, when `drift_score` exceeds the `threshold`, Feather generates an alert. This often indicates a data pipeline issue, a change in user behavior, or a new fraud pattern — all of which warrant retraining your model.

---

## Step 6: Reset Reference Distribution

If drift is expected (e.g., after a model retrain), reset the reference distribution so future comparisons use the new baseline:

```bash
$ curl -s -X POST http://localhost:8080/v1/drift/reset/amount | jq .
```

Expected output:

```json
{
  "status": "ok",
  "feature": "amount",
  "message": "reference distribution reset"
}
```

---

## Putting It All Together

Here's how the pieces fit in a production fraud detection system:

```
Payment Gateway
       │
       ▼
┌──────────────┐     ┌───────────────────┐
│ Feature       │────▶│   Feather          │
│ Pipeline      │     │   Feature Store    │
│ (Flink/Spark) │     │                   │
└──────────────┘     │  ┌──────────────┐  │
                      │  │ Hot Tier     │  │◀──── Real-time queries
                      │  │ (in-memory)  │  │      (sub-millisecond)
                      │  └──────────────┘  │
                      │  ┌──────────────┐  │
                      │  │ Drift Monitor│  │──── Alerts on distribution shift
                      │  └──────────────┘  │
                      └───────────────────┘
                              │
                              ▼
                      ┌──────────────┐
                      │  Fraud Model  │
                      │  (real-time)  │
                      └──────────────┘
```

---

## What's Next?

- **[Migrating from Feast](03-migrating-from-feast.md)** — If you're coming from Feast
- **[Managing LLM Features](04-llm-features.md)** — Cache and version LLM prompts
- **[Deploying on Kubernetes](05-kubernetes-deployment.md)** — Take this pipeline to production
