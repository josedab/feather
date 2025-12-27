# Feather Python SDK

High-performance Python client for the Feather Feature Store with support for sync/async operations, DataFrame integration (Pandas/Polars), vector similarity search, ML connectors, and feature transformations.

[![PyPI](https://img.shields.io/pypi/v/feather-client.svg)](https://pypi.org/project/feather-client/)
[![Python](https://img.shields.io/pypi/pyversions/feather-client.svg)](https://pypi.org/project/feather-client/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Client Configuration](#client-configuration)
- [Feature Operations](#feature-operations)
  - [Get Features](#get-features)
  - [Store Features](#store-features)
  - [Batch Operations](#batch-operations)
  - [Point-in-Time Queries](#point-in-time-queries)
- [Async Client](#async-client)
  - [Connection Pooling](#connection-pooling)
  - [Automatic Retry](#automatic-retry)
  - [Parallel Operations](#parallel-operations)
- [DataFrame Integration](#dataframe-integration)
  - [Pandas Support](#pandas-support)
  - [Polars Support](#polars-support)
  - [DataFrame Enrichment](#dataframe-enrichment)
  - [Bulk Operations](#bulk-operations)
- [Vector Similarity Search](#vector-similarity-search)
- [Feature Transformations](#feature-transformations)
- [ML Connectors](#ml-connectors)
- [Schema Management](#schema-management)
- [Health Checks](#health-checks)
- [Error Handling](#error-handling)
- [Performance Optimization](#performance-optimization)
- [Testing](#testing)
- [Troubleshooting](#troubleshooting)
- [API Reference](#api-reference)

## Installation

```bash
# Basic installation
pip install feather-client

# With Pandas support
pip install feather-client[pandas]

# With Polars support
pip install feather-client[polars]

# With async support (includes httpx with HTTP/2)
pip install feather-client[async]

# With all optional dependencies
pip install feather-client[all]
```

### Requirements

- Python 3.9+
- httpx >= 0.24.0
- pydantic >= 2.0.0

## Quick Start

```python
from feather_client import FeatherClient

# Create a client
client = FeatherClient("http://localhost:8080")

# Store features
client.put_features(
    entity="user:123",
    features={
        "purchase_count": 42,
        "avg_order_value": 89.99,
        "last_active_days": 3
    }
)

# Get features
features = client.get_features(
    entity="user:123",
    features=["purchase_count", "avg_order_value"]
)

print(f"Purchase count: {features['purchase_count'].value}")
print(f"Avg order value: ${features['avg_order_value'].value:.2f}")

# Always close the client when done
client.close()
```

### Using Context Manager

```python
from feather_client import FeatherClient

with FeatherClient("http://localhost:8080") as client:
    features = client.get_features("user:123", ["purchase_count"])
    print(features["purchase_count"].value)
# Client automatically closed
```

## Client Configuration

### Basic Configuration

```python
from feather_client import FeatherClient

client = FeatherClient(
    base_url="http://localhost:8080",
    timeout=60.0,  # Request timeout in seconds
    headers={
        "Authorization": "Bearer <token>",
        "X-Request-ID": "request-123",
    }
)
```

### Environment-Based Configuration

```python
import os
from feather_client import FeatherClient

client = FeatherClient(
    base_url=os.getenv("FEATHER_URL", "http://localhost:8080"),
    timeout=float(os.getenv("FEATHER_TIMEOUT", "30")),
    headers={
        "Authorization": f"Bearer {os.getenv('FEATHER_TOKEN', '')}",
    }
)
```

## Feature Operations

### Get Features

```python
from feather_client import FeatherClient

client = FeatherClient("http://localhost:8080")

# Get specific features
features = client.get_features(
    entity="user:123",
    features=["purchase_count", "avg_order_value", "loyalty_tier"]
)

# Access feature values
for name, feature in features.items():
    print(f"{name}: {feature.value}")

    # Access timestamp if available
    if feature.timestamp:
        print(f"  Updated: {feature.timestamp_datetime}")
```

### Store Features

```python
from feather_client import FeatherClient
import time

client = FeatherClient("http://localhost:8080")

# Store features with automatic timestamp
client.put_features(
    entity="user:123",
    features={
        "purchase_count": 42,
        "avg_order_value": 89.99,
        "is_premium": True,
        "tags": ["vip", "early_adopter"]
    }
)

# Store with explicit timestamp (nanoseconds since epoch)
client.put_features(
    entity="user:123",
    features={"last_login": time.time()},
    timestamp=time.time_ns()
)

# Store with version for optimistic concurrency
client.put_features(
    entity="user:123",
    features={"balance": 100.50},
    version=5  # Only updates if current version < 5
)
```

### Batch Operations

```python
from feather_client import FeatherClient

client = FeatherClient("http://localhost:8080")

# Get features for multiple entities
batch_result = client.get_features_batch(
    entities=["user:1", "user:2", "user:3", "user:4", "user:5"],
    features=["purchase_count", "avg_order_value"]
)

# Process results
for entity_key, entity_features in batch_result.items():
    print(f"{entity_key}:")
    for name, feature in entity_features.items():
        print(f"  {name}: {feature.value}")
```

### Bulk Feature Storage

```python
from feather_client import FeatherClient

client = FeatherClient("http://localhost:8080")

# Store features for multiple entities in a single batch
updates = [
    {"entity_key": "user:1", "features": {"score": 0.95, "tier": "gold"}},
    {"entity_key": "user:2", "features": {"score": 0.87, "tier": "silver"}},
    {"entity_key": "user:3", "features": {"score": 0.72, "tier": "bronze"}},
]

result = client.put_features_batch(updates)
print(f"Updated {result['success']} entities")
```

### Point-in-Time Queries

Retrieve features as they existed at a specific point in time:

```python
from feather_client import FeatherClient
from datetime import datetime, timedelta

client = FeatherClient("http://localhost:8080")

# Get features as of 24 hours ago
yesterday = datetime.utcnow() - timedelta(days=1)
historical = client.get_features_as_of(
    entity="user:123",
    features=["purchase_count", "avg_order_value"],
    as_of=yesterday.isoformat() + "Z"
)

print(f"Purchase count yesterday: {historical['purchase_count'].value}")

# Get features at a specific timestamp
training_time = "2024-01-15T10:30:00Z"
training_features = client.get_features_as_of(
    entity="user:123",
    features=["purchase_count", "avg_order_value"],
    as_of=training_time
)
```

## Async Client

The async client provides high-performance concurrent operations with connection pooling, automatic retry, and full async/await support.

### Basic Async Usage

```python
from feather_client import AsyncFeatherClient
import asyncio

async def main():
    async with AsyncFeatherClient("http://localhost:8080") as client:
        features = await client.get_features("user:123", ["purchase_count"])
        print(features["purchase_count"].value)

asyncio.run(main())
```

### Connection Pooling

Configure connection pool settings for optimal throughput:

```python
from feather_client import AsyncFeatherClient

client = AsyncFeatherClient(
    base_url="http://localhost:8080",
    timeout=30.0,

    # Connection pool settings
    max_connections=100,           # Max total connections
    max_keepalive_connections=20,  # Max idle connections
    keepalive_expiry=5.0,          # Idle connection timeout (seconds)
)
```

### Automatic Retry

Enable exponential backoff retry for transient failures:

```python
from feather_client import AsyncFeatherClient

client = AsyncFeatherClient(
    base_url="http://localhost:8080",

    # Retry configuration
    max_retries=3,      # Maximum retry attempts
    retry_delay=0.1,    # Initial delay (exponential backoff)
)

# Retries automatically on:
# - Connection errors
# - Read/Write timeouts
# - 5xx server errors
```

### Parallel Operations

Execute multiple operations concurrently:

```python
from feather_client import AsyncFeatherClient
import asyncio

async def fetch_all_users(user_ids: list[str]) -> dict:
    async with AsyncFeatherClient("http://localhost:8080") as client:
        # Create tasks for parallel execution
        tasks = [
            client.get_features(f"user:{uid}", ["score", "tier"])
            for uid in user_ids
        ]

        # Execute all concurrently
        results = await asyncio.gather(*tasks, return_exceptions=True)

        # Process results
        return {
            uid: result for uid, result in zip(user_ids, results)
            if not isinstance(result, Exception)
        }

# Fetch features for 1000 users concurrently
user_ids = [str(i) for i in range(1000)]
all_features = asyncio.run(fetch_all_users(user_ids))
```

### Async Sub-Clients

The async client provides specialized sub-clients for different operations:

```python
from feather_client import AsyncFeatherClient
import asyncio

async def main():
    async with AsyncFeatherClient("http://localhost:8080") as client:
        # Vector operations
        await client.vectors.create_index("embeddings", dimension=384)

        # Transformation operations
        transforms = await client.transforms.list()

        # ML connector operations
        connectors = await client.ml.list_connectors()

        # Benchmark operations
        result = await client.benchmarks.run("hot_get", iterations=1000)

asyncio.run(main())
```

## DataFrame Integration

### Pandas Support

```python
from feather_client.dataframe import DataFrameClient
import pandas as pd

df_client = DataFrameClient("http://localhost:8080")

# Get features as DataFrame
df = df_client.get_features_df(
    entities=["user:1", "user:2", "user:3"],
    features=["purchase_count", "avg_order_value"]
)

print(df)
#     entity  purchase_count  avg_order_value
# 0   user:1              10            50.00
# 1   user:2              25            75.50
# 2   user:3               5            30.00

# Include timestamps
df_with_ts = df_client.get_features_df(
    entities=["user:1", "user:2"],
    features=["purchase_count"],
    include_timestamps=True
)
```

### Storing from DataFrame

```python
from feather_client.dataframe import DataFrameClient
import pandas as pd

df_client = DataFrameClient("http://localhost:8080")

# Create training data
training_data = pd.DataFrame({
    "entity": ["user:1", "user:2", "user:3"],
    "purchase_count": [15, 30, 8],
    "avg_order_value": [60.0, 80.0, 45.0],
    "churn_probability": [0.1, 0.05, 0.3]
})

# Store all features (uses batch API for efficiency)
updated = df_client.put_features_df(
    training_data,
    entity_column="entity",
    batch_size=1000  # Rows per batch request
)

print(f"Updated {updated} entities")
```

### Polars Support

```python
from feather_client.dataframe import DataFrameClient
import polars as pl

df_client = DataFrameClient("http://localhost:8080")

# Get features as Polars DataFrame
df = df_client.get_features_polars(
    entities=["user:1", "user:2"],
    features=["purchase_count", "score"]
)

print(df)

# Store from Polars DataFrame
data = pl.DataFrame({
    "entity": ["user:1", "user:2"],
    "score": [0.95, 0.87],
    "tier": ["gold", "silver"]
})

df_client.put_features_polars(data, entity_column="entity")
```

### DataFrame Enrichment

Enrich existing DataFrames with features from Feather:

```python
from feather_client.dataframe import DataFrameClient
import pandas as pd

df_client = DataFrameClient("http://localhost:8080")

# Your existing data
users_df = pd.DataFrame({
    "user_id": ["user:1", "user:2", "user:3"],
    "name": ["Alice", "Bob", "Charlie"],
    "signup_date": ["2023-01-15", "2023-02-20", "2023-03-10"]
})

# Enrich with features from Feather
enriched = df_client.enrich_df(
    users_df,
    entity_column="user_id",
    features=["purchase_count", "avg_order_value", "loyalty_tier"],
    prefix="feat_"  # Optional prefix for feature columns
)

print(enriched.columns.tolist())
# ['user_id', 'name', 'signup_date', 'feat_purchase_count', 'feat_avg_order_value', 'feat_loyalty_tier']
```

### Bulk Operations

Efficiently store large datasets:

```python
from feather_client.dataframe import DataFrameClient
import pandas as pd

df_client = DataFrameClient("http://localhost:8080")

# Load large dataset
large_df = pd.read_parquet("user_features.parquet")

# Store with custom batch size
updated = df_client.put_features_df(
    large_df,
    entity_column="user_id",
    feature_columns=["score", "tier", "last_active"],  # Specific columns
    batch_size=5000  # Larger batch for better throughput
)

print(f"Stored features for {updated} entities")
```

### Convenience Functions

Quick one-off operations without managing client lifecycle:

```python
from feather_client.dataframe import get_features_df, get_features_polars

# Get features directly as DataFrame
df = get_features_df(
    entities=["user:1", "user:2"],
    features=["score", "tier"],
    base_url="http://localhost:8080"
)
```

## Vector Similarity Search

### Creating and Managing Indexes

```python
from feather_client import FeatherClient

client = FeatherClient("http://localhost:8080")

# Create a vector index
client.vectors.create_index(
    name="product_embeddings",
    dimension=384,
    distance_type="cosine"  # cosine, euclidean, or dot_product
)

# List all indexes
indexes = client.vectors.list_indexes()
print(f"Indexes: {indexes}")

# Get index info
info = client.vectors.get_index("product_embeddings")
print(f"Index size: {info.size} vectors")

# Delete an index
client.vectors.delete_index("product_embeddings")
```

### Upserting Vectors

```python
from feather_client import FeatherClient

client = FeatherClient("http://localhost:8080")

# Upsert vectors with metadata
count = client.vectors.upsert(
    index="product_embeddings",
    vectors=[
        {
            "id": "prod:1",
            "vector": [0.1, 0.2, 0.3, ...],  # 384 dimensions
            "metadata": {"name": "Widget", "category": "electronics", "price": 29.99}
        },
        {
            "id": "prod:2",
            "vector": [0.3, 0.1, 0.4, ...],
            "metadata": {"name": "Gadget", "category": "electronics", "price": 49.99}
        },
    ]
)

print(f"Upserted {count} vectors")
```

### Searching for Similar Vectors

```python
from feather_client import FeatherClient

client = FeatherClient("http://localhost:8080")

# Search for similar products
results = client.vectors.search(
    index="product_embeddings",
    vector=[0.15, 0.18, 0.25, ...],  # Query vector
    top_k=10,
    include_metadata=True,
    include_vectors=False,  # Don't return the actual vectors
    ef=100  # Search expansion factor (higher = more accurate)
)

for result in results:
    print(f"ID: {result.id}")
    print(f"  Score: {result.score:.4f}")
    print(f"  Distance: {result.distance:.4f}")
    print(f"  Metadata: {result.metadata}")
```

### Vector CRUD Operations

```python
from feather_client import FeatherClient

client = FeatherClient("http://localhost:8080")

# Get a specific vector
vector = client.vectors.get(
    index="product_embeddings",
    vector_id="prod:1",
    include_vector=True  # Include the actual vector values
)
print(f"Vector: {vector.vector[:5]}...")  # First 5 dimensions
print(f"Metadata: {vector.metadata}")

# Delete a vector
client.vectors.delete(
    index="product_embeddings",
    vector_id="prod:1"
)
```

### Async Vector Operations

```python
from feather_client import AsyncFeatherClient
import asyncio

async def batch_upsert_embeddings():
    async with AsyncFeatherClient("http://localhost:8080") as client:
        # Create index
        await client.vectors.create_index("embeddings", dimension=768)

        # Upsert in parallel batches
        batch_size = 100
        all_vectors = [...]  # Your vectors

        tasks = []
        for i in range(0, len(all_vectors), batch_size):
            batch = all_vectors[i:i+batch_size]
            tasks.append(client.vectors.upsert("embeddings", batch))

        results = await asyncio.gather(*tasks)
        total = sum(results)
        print(f"Upserted {total} vectors")

asyncio.run(batch_upsert_embeddings())
```

## Feature Transformations

Define and execute feature transformations:

```python
from feather_client import AsyncFeatherClient
from feather_client.models import Transform
import asyncio

async def main():
    async with AsyncFeatherClient("http://localhost:8080") as client:
        # List existing transforms
        transforms = await client.transforms.list()

        # Create a new transform
        transform = Transform(
            name="user_score_normalized",
            type="expression",
            expression="(raw_score - min_score) / (max_score - min_score)",
            inputs=["raw_score", "min_score", "max_score"],
            output="normalized_score",
            output_type="float"
        )
        await client.transforms.create(transform)

        # Execute a transform
        result = await client.transforms.execute(
            name="user_score_normalized",
            entity_id="user:123"
        )
        print(f"Normalized score: {result}")

        # Execute and store the result
        output_feature = await client.transforms.execute_and_store(
            name="user_score_normalized",
            entity_id="user:123"
        )
        print(f"Stored as: {output_feature}")

        # Define using DSL syntax
        await client.transforms.define_dsl(
            name="total_value",
            expression="purchase_count * avg_order_value"
        )

        # Execute a chain of dependent transforms
        final_result = await client.transforms.execute_chain(
            output_feature="final_score",
            entity_id="user:123"
        )

asyncio.run(main())
```

## ML Connectors

Connect to ML serving platforms for real-time inference:

```python
from feather_client import AsyncFeatherClient
import asyncio

async def main():
    async with AsyncFeatherClient("http://localhost:8080") as client:
        # Register an ML connector
        connector = await client.ml.register_connector(
            name="sklearn_models",
            connector_type="mlflow",
            endpoint="http://mlflow:5000"
        )

        # Connect
        await client.ml.connect("sklearn_models")

        # Make a prediction using features from Feather
        prediction = await client.ml.predict(
            connector="sklearn_models",
            model_name="churn_model",
            model_version="1",
            entity_id="user:123",
            feature_names=["purchase_count", "avg_order_value", "days_since_active"]
        )
        print(f"Prediction: {prediction.predictions}")
        print(f"Latency: {prediction.latency_ms}ms")

        # Batch predictions
        batch_result = await client.ml.batch_predict(
            connector="sklearn_models",
            model_name="churn_model",
            entity_ids=["user:1", "user:2", "user:3"],
            feature_names=["purchase_count", "avg_order_value"]
        )
        print(f"Batch predictions: {batch_result.predictions}")

        # Disconnect and unregister
        await client.ml.disconnect("sklearn_models")
        await client.ml.unregister_connector("sklearn_models")

asyncio.run(main())
```

## Schema Management

### Feature Groups

```python
from feather_client import FeatherClient, FeatureGroup, FeatureSpec

client = FeatherClient("http://localhost:8080")

# List existing groups
groups = client.list_groups()
for group in groups:
    print(f"{group.name}: {len(group.features)} features")

# Get a specific group
group = client.get_group("user_features")
print(f"Entity type: {group.entity_type}")
print(f"TTL: {group.ttl} seconds")

# Create a new feature group
new_group = FeatureGroup(
    name="user_engagement",
    entity_type="user",
    description="User engagement metrics",
    ttl=86400 * 7,  # 7 days
    features=[
        FeatureSpec(
            name="page_views_7d",
            data_type="int",
            default=0
        ),
        FeatureSpec(
            name="session_duration_avg",
            data_type="float",
            default=0.0
        ),
        FeatureSpec(
            name="embedding",
            data_type="float",
            dimensions=[384]  # Vector feature
        )
    ]
)

created = client.create_group(new_group)
print(f"Created group: {created.name}")
```

## Health Checks

```python
from feather_client import FeatherClient

client = FeatherClient("http://localhost:8080")

# Full health check with component status
health = client.health()
print(f"Overall status: {health.status}")

if health.components:
    for component, status in health.components.items():
        print(f"  {component}: {status}")

# Quick readiness check (for load balancers)
if client.ready():
    print("Server is ready to accept traffic")
else:
    print("Server is not ready")

# Liveness check (for container orchestration)
if client.live():
    print("Server is alive")
else:
    print("Server is down")
```

### Async Health Checks

```python
from feather_client import AsyncFeatherClient
import asyncio

async def health_check_loop():
    async with AsyncFeatherClient("http://localhost:8080") as client:
        while True:
            if await client.ready():
                print("✓ Healthy")
            else:
                print("✗ Unhealthy")
            await asyncio.sleep(10)

asyncio.run(health_check_loop())
```

## Error Handling

### Exception Types

```python
from feather_client import (
    FeatherClient,
    FeatherError,      # Base exception
    NotFoundError,     # Entity or resource not found
    ValidationError,   # Invalid request
)

client = FeatherClient("http://localhost:8080")

try:
    features = client.get_features("user:unknown", ["score"])
except NotFoundError as e:
    print(f"Entity not found: {e.message}")
except ValidationError as e:
    print(f"Invalid request: {e.message}")
    print(f"Status code: {e.status_code}")
except FeatherError as e:
    print(f"Server error ({e.status_code}): {e.message}")
except Exception as e:
    print(f"Unexpected error: {e}")
```

### Graceful Degradation

```python
from feather_client import FeatherClient, NotFoundError, FeatherError

def get_features_with_defaults(
    client: FeatherClient,
    entity: str,
    features: list[str],
    defaults: dict
) -> dict:
    """Get features with fallback to defaults on error."""
    try:
        result = client.get_features(entity, features)
        return {name: feat.value for name, feat in result.items()}
    except NotFoundError:
        return defaults
    except FeatherError:
        # Log error, return defaults
        return defaults

# Usage
features = get_features_with_defaults(
    client,
    entity="user:123",
    features=["score", "tier"],
    defaults={"score": 0.5, "tier": "standard"}
)
```

### Retry with Custom Logic

```python
from feather_client import FeatherClient, FeatherError
import time

def get_with_retry(
    client: FeatherClient,
    entity: str,
    features: list[str],
    max_retries: int = 3,
    backoff_factor: float = 1.5
) -> dict:
    """Get features with custom retry logic."""
    last_error = None
    delay = 0.1

    for attempt in range(max_retries + 1):
        try:
            return client.get_features(entity, features)
        except FeatherError as e:
            last_error = e
            if e.status_code and e.status_code < 500:
                raise  # Don't retry client errors
            if attempt < max_retries:
                time.sleep(delay)
                delay *= backoff_factor

    raise last_error
```

## Performance Optimization

### Connection Reuse

Always reuse client instances instead of creating new ones:

```python
# Good: Reuse a single client
client = FeatherClient("http://localhost:8080")
for user_id in user_ids:
    features = client.get_features(f"user:{user_id}", ["score"])
client.close()

# Bad: Creating a new client per request
for user_id in user_ids:
    client = FeatherClient("http://localhost:8080")
    features = client.get_features(f"user:{user_id}", ["score"])
    client.close()  # Wasteful!
```

### Batch Operations

Use batch APIs to reduce network round trips:

```python
from feather_client import FeatherClient

client = FeatherClient("http://localhost:8080")

# Good: Single batch request
features = client.get_features_batch(
    entities=["user:1", "user:2", "user:3"],
    features=["score", "tier"]
)

# Bad: Multiple individual requests
for uid in ["user:1", "user:2", "user:3"]:
    features = client.get_features(uid, ["score", "tier"])
```

### Async for High Throughput

Use the async client for concurrent operations:

```python
from feather_client import AsyncFeatherClient
import asyncio

async def high_throughput_fetch():
    async with AsyncFeatherClient(
        "http://localhost:8080",
        max_connections=100,
        max_keepalive_connections=50,
    ) as client:
        # Fetch 10,000 entities with 100 concurrent connections
        entities = [f"user:{i}" for i in range(10000)]

        # Use semaphore to limit concurrency
        semaphore = asyncio.Semaphore(100)

        async def fetch_one(entity: str):
            async with semaphore:
                return await client.get_features(entity, ["score"])

        results = await asyncio.gather(*[
            fetch_one(e) for e in entities
        ])

        return results

results = asyncio.run(high_throughput_fetch())
```

### DataFrame Batch Size Tuning

Optimize batch sizes for DataFrame operations:

```python
from feather_client.dataframe import DataFrameClient

df_client = DataFrameClient("http://localhost:8080")

# Small entities, many features: smaller batches
df_client.put_features_df(df, entity_column="id", batch_size=500)

# Large entities, few features: larger batches
df_client.put_features_df(df, entity_column="id", batch_size=5000)
```

### Memory Efficiency with Polars

For large datasets, prefer Polars over Pandas:

```python
from feather_client.dataframe import DataFrameClient

df_client = DataFrameClient("http://localhost:8080")

# More memory efficient for large datasets
df = df_client.get_features_polars(
    entities=large_entity_list,  # Millions of entities
    features=["score", "tier"]
)
```

## Testing

### Unit Testing with Mocks

```python
from unittest.mock import Mock, patch
from feather_client import FeatherClient
from feather_client.models import Feature

def test_get_features():
    with patch.object(FeatherClient, 'get_features') as mock_get:
        mock_get.return_value = {
            "score": Feature(value=0.95, timestamp=1234567890),
            "tier": Feature(value="gold", timestamp=1234567890)
        }

        client = FeatherClient("http://localhost:8080")
        features = client.get_features("user:123", ["score", "tier"])

        assert features["score"].value == 0.95
        assert features["tier"].value == "gold"
```

### Integration Testing

```python
import pytest
from feather_client import FeatherClient

@pytest.fixture
def client():
    """Create a test client."""
    client = FeatherClient("http://localhost:8080")
    yield client
    client.close()

def test_roundtrip(client):
    """Test storing and retrieving features."""
    # Store features
    client.put_features(
        entity="test:1",
        features={"value": 42, "name": "test"}
    )

    # Retrieve and verify
    features = client.get_features("test:1", ["value", "name"])

    assert features["value"].value == 42
    assert features["name"].value == "test"
```

### Async Testing

```python
import pytest
import asyncio
from feather_client import AsyncFeatherClient

@pytest.fixture
async def async_client():
    """Create an async test client."""
    client = AsyncFeatherClient("http://localhost:8080")
    yield client
    await client.close()

@pytest.mark.asyncio
async def test_async_operations(async_client):
    """Test async feature operations."""
    await async_client.put_features(
        entity="test:async",
        features={"score": 0.99}
    )

    features = await async_client.get_features("test:async", ["score"])
    assert features["score"].value == 0.99
```

## Troubleshooting

### Connection Issues

```python
from feather_client import FeatherClient, FeatherError
import httpx

try:
    client = FeatherClient("http://localhost:8080", timeout=5.0)
    client.health()
except httpx.ConnectError:
    print("Cannot connect to Feather server")
    print("Check that the server is running and accessible")
except httpx.TimeoutException:
    print("Connection timed out")
    print("Server might be overloaded or network issues")
```

### Debug Logging

Enable httpx debug logging:

```python
import logging
import httpx

# Enable debug logging
logging.basicConfig(level=logging.DEBUG)
logging.getLogger("httpx").setLevel(logging.DEBUG)
logging.getLogger("httpcore").setLevel(logging.DEBUG)

from feather_client import FeatherClient

client = FeatherClient("http://localhost:8080")
features = client.get_features("user:123", ["score"])
# Will show detailed HTTP request/response logs
```

### Timeout Issues

```python
from feather_client import FeatherClient

# Increase timeout for slow networks
client = FeatherClient(
    "http://localhost:8080",
    timeout=120.0  # 2 minutes
)

# Or set different timeouts for connect vs read
import httpx

# Using httpx directly for fine-grained control
timeout = httpx.Timeout(
    connect=5.0,     # Connection timeout
    read=60.0,       # Read timeout
    write=30.0,      # Write timeout
    pool=10.0        # Pool timeout
)
```

### Empty Results

```python
from feather_client import FeatherClient

client = FeatherClient("http://localhost:8080")

# Check if features exist before using
features = client.get_features("user:123", ["score", "tier"])

if not features:
    print("No features found for entity")
elif "score" not in features:
    print("Score feature not found")
else:
    print(f"Score: {features['score'].value}")
```

## API Reference

### FeatherClient

| Method | Description |
|--------|-------------|
| `get_features(entity, features)` | Get features for an entity |
| `get_features_batch(entities, features)` | Get features for multiple entities |
| `put_features(entity, features, timestamp?, version?)` | Store features |
| `put_features_batch(updates, ingestion_url?)` | Bulk store features |
| `get_features_as_of(entity, features, as_of)` | Point-in-time query |
| `list_groups()` | List feature groups |
| `get_group(name)` | Get a feature group |
| `create_group(group)` | Create a feature group |
| `health()` | Full health check |
| `ready()` | Readiness probe |
| `live()` | Liveness probe |
| `close()` | Close the client |

### AsyncFeatherClient

All methods from `FeatherClient` as async, plus:

| Property | Description |
|----------|-------------|
| `vectors` | Vector similarity search operations |
| `transforms` | Feature transformation operations |
| `ml` | ML connector operations |
| `benchmarks` | Benchmark operations |

### VectorClient / AsyncVectorClient

| Method | Description |
|--------|-------------|
| `list_indexes()` | List all vector indexes |
| `create_index(name, dimension, distance_type)` | Create an index |
| `get_index(name)` | Get index info |
| `delete_index(name)` | Delete an index |
| `upsert(index, vectors)` | Upsert vectors |
| `search(index, vector, top_k, ef?, include_metadata?, include_vectors?)` | Search |
| `get(index, vector_id, include_vector?)` | Get a vector |
| `delete(index, vector_id)` | Delete a vector |

### DataFrameClient

| Method | Description |
|--------|-------------|
| `get_features_df(entities, features, include_timestamps?)` | Get as Pandas DataFrame |
| `get_features_polars(entities, features, include_timestamps?)` | Get as Polars DataFrame |
| `put_features_df(df, entity_column, feature_columns?, batch_size?)` | Store from Pandas |
| `put_features_polars(df, entity_column, feature_columns?, batch_size?)` | Store from Polars |
| `enrich_df(df, entity_column, features, prefix?)` | Enrich Pandas DataFrame |
| `enrich_polars(df, entity_column, features, prefix?)` | Enrich Polars DataFrame |

### Models

| Model | Description |
|-------|-------------|
| `Feature` | Feature value with timestamp |
| `FeatureGroup` | Feature group definition |
| `FeatureSpec` | Feature specification |
| `VectorIndex` | Vector index information |
| `VectorRecord` | Vector with metadata |
| `VectorSearchResult` | Search result |
| `HealthStatus` | Health check response |
| `Transform` | Transformation definition |
| `MLConnector` | ML connector definition |
| `PredictResponse` | Prediction result |
| `BenchmarkResult` | Benchmark result |

### Exceptions

| Exception | Description |
|-----------|-------------|
| `FeatherError` | Base exception |
| `NotFoundError` | Resource not found (404) |
| `ValidationError` | Invalid request (400) |

## License

MIT License - see [LICENSE](LICENSE) for details.
