---
sidebar_position: 2
title: Python SDK
description: Official Python client for Feather feature store.
---

# Python SDK

The official Python client provides a Pythonic interface to Feather, with pandas integration for data science workflows.

## Installation

```bash
pip install feather-client
```

**Requirements:** Python 3.8+

### Optional Dependencies

```bash
# For pandas integration
pip install feather-client[pandas]

# For async support
pip install feather-client[async]

# All extras
pip install feather-client[all]
```

## Quick Start

```python
from feather import FeatherClient

# Create client
client = FeatherClient("localhost:8080")

# Store features
client.put_features("user:123", {
    "click_count": 42,
    "purchase_total": 299.99,
    "is_premium": True
})

# Retrieve features
features = client.get_features("user:123", ["click_count", "purchase_total"])
print(features)
# {'click_count': 42, 'purchase_total': 299.99}
```

## Client Configuration

### Basic Configuration

```python
client = FeatherClient("localhost:8080")
```

### Advanced Configuration

```python
client = FeatherClient(
    host="localhost:8080",
    timeout=5.0,           # Request timeout in seconds
    retries=3,             # Max retry attempts
    pool_size=10,          # Connection pool size
    compression=True,      # Enable gzip compression
)
```

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `timeout` | Request timeout (seconds) | 30.0 |
| `retries` | Max retry attempts | 3 |
| `pool_size` | Connection pool size | 10 |
| `compression` | Enable gzip | False |
| `ssl_verify` | Verify SSL certificates | True |
| `api_key` | API key for authentication | None |

### TLS Configuration

```python
client = FeatherClient(
    host="localhost:8080",
    ssl_verify=True,
    ssl_ca_cert="/path/to/ca.crt",
    ssl_client_cert="/path/to/client.crt",
    ssl_client_key="/path/to/client.key",
)
```

### Using Environment Variables

```python
import os

# Configure via environment
os.environ["FEATHER_HOST"] = "localhost:8080"
os.environ["FEATHER_API_KEY"] = "your-api-key"

client = FeatherClient.from_env()
```

## Storing Features

### Single Entity

```python
client.put_features("user:123", {
    "click_count": 42,
    "purchase_total": 299.99,
    "is_premium": True,
    "last_activity": datetime.now()
})
```

### Batch Write

```python
updates = [
    {"entity": "user:123", "features": {"click_count": 42}},
    {"entity": "user:456", "features": {"click_count": 15}},
    {"entity": "user:789", "features": {"click_count": 27}},
]

client.put_features_batch(updates)
```

### With Timestamp (Backfill)

```python
from datetime import datetime

# Store with explicit timestamp
client.put_features(
    "user:123",
    {"click_count": 42},
    timestamp=datetime(2024, 1, 15, 10, 0, 0)
)
```

### From Pandas DataFrame

```python
import pandas as pd

df = pd.DataFrame({
    "entity": ["user:123", "user:456", "user:789"],
    "click_count": [42, 15, 27],
    "purchase_total": [299.99, 149.99, 99.99]
})

client.put_features_from_dataframe(df, entity_column="entity")
```

## Retrieving Features

### Single Entity

```python
features = client.get_features("user:123", ["click_count", "purchase_total"])
# {'click_count': 42, 'purchase_total': 299.99}
```

### All Features for Entity

```python
all_features = client.get_features("user:123")
# Returns all features for the entity
```

### Batch Read

```python
entities = ["user:123", "user:456", "user:789"]
feature_names = ["click_count", "purchase_total"]

results = client.get_features_batch(entities, feature_names)

for entity, features in results.items():
    print(f"{entity}: {features}")
```

### With Metadata

```python
features, metadata = client.get_features(
    "user:123",
    ["click_count"],
    include_metadata=True
)

for name, meta in metadata.items():
    print(f"{name}: updated {meta.timestamp} ({meta.age_ms}ms ago)")
```

### As Pandas DataFrame

```python
df = client.get_features_as_dataframe(
    entities=["user:123", "user:456", "user:789"],
    features=["click_count", "purchase_total"]
)

print(df)
#          entity  click_count  purchase_total
# 0     user:123           42          299.99
# 1     user:456           15          149.99
# 2     user:789           27           99.99
```

## Point-in-Time Queries

### Single Query

```python
from datetime import datetime

features = client.get_features_as_of(
    "user:123",
    ["click_count", "purchase_total"],
    as_of=datetime(2024, 1, 15)
)
```

### Batch Point-in-Time

```python
queries = [
    {"entity": "user:123", "as_of": "2024-01-15T00:00:00Z"},
    {"entity": "user:456", "as_of": "2024-01-16T00:00:00Z"},
    {"entity": "user:789", "as_of": "2024-01-17T00:00:00Z"},
]

results = client.get_features_as_of_batch(
    queries,
    features=["click_count", "purchase_total"]
)

for result in results:
    print(f"{result.entity} @ {result.as_of}: {result.features}")
```

### Training Data Generation

```python
import pandas as pd

# Your labels with timestamps
labels_df = pd.DataFrame({
    "entity": ["user:123", "user:456", "user:789"],
    "label_time": ["2024-01-15", "2024-01-16", "2024-01-17"],
    "churned": [1, 0, 1]
})

# Generate training data with point-in-time features
training_df = client.get_training_data(
    labels_df,
    entity_column="entity",
    timestamp_column="label_time",
    features=["click_count", "purchase_total", "days_since_signup"]
)

print(training_df)
#        entity  label_time  churned  click_count  purchase_total  days_since_signup
# 0   user:123  2024-01-15        1           42          299.99                 30
# 1   user:456  2024-01-16        0           15          149.99                 45
# 2   user:789  2024-01-17        1           27           99.99                 60
```

## Vector Operations

### Create Index

```python
client.vectors.create_index(
    name="product_embeddings",
    dimensions=384,
    metric="cosine",
    hnsw={
        "m": 16,
        "ef_construction": 200
    }
)
```

### Upsert Vectors

```python
from sentence_transformers import SentenceTransformer

model = SentenceTransformer('all-MiniLM-L6-v2')

products = [
    {"id": "product:123", "text": "Wireless headphones", "category": "electronics"},
    {"id": "product:456", "text": "Running shoes", "category": "sports"},
]

vectors = [
    {
        "id": p["id"],
        "values": model.encode(p["text"]).tolist(),
        "metadata": {"category": p["category"]}
    }
    for p in products
]

client.vectors.upsert("product_embeddings", vectors)
```

### Search Vectors

```python
# Encode query
query_embedding = model.encode("best headphones for music").tolist()

# Search
results = client.vectors.search(
    index="product_embeddings",
    vector=query_embedding,
    top_k=10,
    filter={
        "category": {"$eq": "electronics"}
    }
)

for result in results:
    print(f"{result.id}: {result.score:.3f}")
```

### List and Manage Indexes

```python
# List all indexes
indexes = client.vectors.list_indexes()

# Get index info
info = client.vectors.get_index("product_embeddings")
print(f"Vector count: {info.vector_count}")

# Delete a vector
client.vectors.delete("product_embeddings", "product:123")

# Delete index
client.vectors.delete_index("product_embeddings")
```

## Async Client

For async applications:

```python
import asyncio
from feather import AsyncFeatherClient

async def main():
    client = AsyncFeatherClient("localhost:8080")

    # Store features
    await client.put_features("user:123", {
        "click_count": 42,
        "purchase_total": 299.99
    })

    # Retrieve features
    features = await client.get_features("user:123", ["click_count"])
    print(features)

    # Batch operations
    entities = ["user:123", "user:456", "user:789"]
    results = await client.get_features_batch(entities, ["click_count"])

    await client.close()

asyncio.run(main())
```

### Async Context Manager

```python
async with AsyncFeatherClient("localhost:8080") as client:
    features = await client.get_features("user:123", ["click_count"])
```

## Schema Management

### List Feature Groups

```python
groups = client.schema.list_groups()

for group in groups:
    print(f"Group: {group.name} (entity: {group.entity_type})")
    for feature in group.features:
        print(f"  - {feature.name}: {feature.data_type}")
```

### Create Feature Group

```python
client.schema.create_group(
    name="user_engagement",
    entity_type="user",
    ttl="24h",
    features=[
        {"name": "click_count", "data_type": "int64"},
        {"name": "purchase_total", "data_type": "float64"},
        {"name": "is_premium", "data_type": "bool"},
    ]
)
```

## Drift Detection

```python
# Register feature for monitoring
client.drift.register(
    feature="user:purchase_total",
    window_size=1000,
    detection_method="ks",
    threshold=0.05
)

# Check status
status = client.drift.status()
for feature in status.features:
    if feature.status == "drifted":
        print(f"DRIFT: {feature.name} ({feature.metric_value:.3f})")

# Get alerts
alerts = client.drift.alerts(since="2024-01-01T00:00:00Z")
for alert in alerts:
    print(f"Alert: {alert.feature} at {alert.detected_at}")

# Reset after expected change
client.drift.reset("user:purchase_total")
```

## Freshness Monitoring

```python
# Get freshness status
status = client.freshness.status()

for feature in status.features:
    print(f"{feature.name}: avg age {feature.freshness.avg_age_ms / 1000:.1f}s")
    if feature.sla.status == "breached":
        print(f"  SLA BREACHED: {feature.sla.entities_stale} entities stale")

# Get stale entities
stale = client.freshness.get_stale_entities("user:click_count", limit=100)
for entity in stale:
    print(f"  {entity.entity_id}: {entity.age_ms / 1000:.0f}s old")
```

## Export

### Export to File

```python
result = client.export(
    format="parquet",
    entities=["user:*"],
    features=["click_count", "purchase_total"],
    output_path="/data/export.parquet"
)

print(f"Exported {result.rows_exported} rows")

# Read with pandas
import pandas as pd
df = pd.read_parquet("/data/export.parquet")
```

### Export to Cloud Storage

```python
# Export to S3
result = client.export(
    format="parquet",
    entities=["user:*"],
    features=["click_count", "purchase_total"],
    output_path="s3://bucket/exports/features.parquet"
)
```

### Point-in-Time Export

```python
queries = [
    {"entity": "user:123", "as_of": "2024-01-15T00:00:00Z"},
    {"entity": "user:456", "as_of": "2024-01-16T00:00:00Z"},
]

result = client.export_pit(
    format="parquet",
    queries=queries,
    features=["click_count", "purchase_total"],
    output_path="/data/training.parquet"
)
```

## Error Handling

### Exception Types

```python
from feather.exceptions import (
    FeatherError,
    NotFoundError,
    TimeoutError,
    UnavailableError,
    InvalidArgumentError,
)

try:
    features = client.get_features("user:123", ["click_count"])
except NotFoundError:
    print("Entity not found")
except TimeoutError:
    print("Request timed out")
except UnavailableError:
    print("Server unavailable")
except InvalidArgumentError as e:
    print(f"Invalid request: {e}")
except FeatherError as e:
    print(f"Feather error: {e}")
```

### Retries

The client automatically retries transient errors:

```python
client = FeatherClient(
    host="localhost:8080",
    retries=3,
    retry_backoff=0.1,  # Initial backoff in seconds
)
```

## Health Checks

```python
# Simple health check
is_healthy = client.health_check()

# Detailed health status
health = client.health()
print(f"Status: {health.status}")
for component, status in health.components.items():
    print(f"  {component}: {status.status}")
```

## Logging

```python
import logging

# Enable debug logging
logging.basicConfig(level=logging.DEBUG)
logger = logging.getLogger("feather")
logger.setLevel(logging.DEBUG)

client = FeatherClient("localhost:8080")
```

## Complete Example

```python
from datetime import datetime
import pandas as pd
from feather import FeatherClient

# Create client
client = FeatherClient("localhost:8080", timeout=5.0)

# Store features
client.put_features("user:123", {
    "click_count": 42,
    "purchase_total": 299.99,
    "is_premium": True
})

# Batch store
updates = [
    {"entity": "user:456", "features": {"click_count": 15, "purchase_total": 149.99}},
    {"entity": "user:789", "features": {"click_count": 27, "purchase_total": 99.99}},
]
client.put_features_batch(updates)

# Retrieve features
features = client.get_features("user:123", ["click_count", "purchase_total"])
print(f"Features: {features}")

# Get as DataFrame
df = client.get_features_as_dataframe(
    entities=["user:123", "user:456", "user:789"],
    features=["click_count", "purchase_total"]
)
print(df)

# Point-in-time query for training data
labels = pd.DataFrame({
    "entity": ["user:123", "user:456"],
    "label_time": ["2024-01-15", "2024-01-16"],
    "label": [1, 0]
})

training_data = client.get_training_data(
    labels,
    entity_column="entity",
    timestamp_column="label_time",
    features=["click_count", "purchase_total"]
)
print(training_data)

# Vector search
client.vectors.create_index("embeddings", dimensions=384, metric="cosine")
client.vectors.upsert("embeddings", [
    {"id": "item:1", "values": [0.1] * 384, "metadata": {"category": "A"}}
])
results = client.vectors.search("embeddings", [0.1] * 384, top_k=5)
print(f"Similar items: {[r.id for r in results]}")

# Check health
health = client.health()
print(f"Server status: {health.status}")
```

## Related Documentation

- [REST API](./rest-api) - HTTP API reference
- [Go SDK](./go) - Go client
- [Point-in-Time Queries](/docs/concepts/point-in-time) - Historical features
