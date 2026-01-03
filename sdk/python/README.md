# Feather Python SDK

High-performance Python client for the Feather Feature Store with support for sync/async operations, Pandas/Polars DataFrames, and vector similarity search.

## Installation

```bash
# Basic installation
pip install feather-client

# With Pandas support
pip install feather-client[pandas]

# With Polars support
pip install feather-client[polars]

# With all optional dependencies
pip install feather-client[all]
```

## Quick Start

### Basic Feature Operations

```python
from feather_client import FeatherClient

# Create client
client = FeatherClient("http://localhost:8080")

# Get features
features = client.get_features(
    entity="user:123",
    features=["purchase_count", "avg_order_value"]
)
print(f"Purchase count: {features['purchase_count'].value}")

# Store features
client.put_features(
    entity="user:123",
    features={
        "purchase_count": 42,
        "avg_order_value": 89.99
    }
)

# Batch get
batch_features = client.get_features_batch(
    entities=["user:1", "user:2", "user:3"],
    features=["purchase_count"]
)

# Point-in-time retrieval
historical = client.get_features_as_of(
    entity="user:123",
    features=["purchase_count"],
    as_of="2024-01-15T10:30:00Z"
)

client.close()
```

### Async Operations

```python
from feather_client import AsyncFeatherClient
import asyncio

async def main():
    async with AsyncFeatherClient("http://localhost:8080") as client:
        # Parallel feature retrieval
        tasks = [
            client.get_features(f"user:{i}", ["score"])
            for i in range(100)
        ]
        results = await asyncio.gather(*tasks)

asyncio.run(main())
```

### DataFrame Integration

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

# Store features from DataFrame
training_data = pd.DataFrame({
    "entity": ["user:1", "user:2"],
    "purchase_count": [15, 30],
    "avg_order_value": [60.0, 80.0]
})
df_client.put_features_df(training_data, entity_column="entity")

# Enrich existing DataFrame with features
users_df = pd.DataFrame({"user_id": ["user:1", "user:2"]})
enriched = df_client.enrich_df(
    users_df,
    entity_column="user_id",
    features=["purchase_count", "avg_order_value"]
)
```

### Polars Support

```python
from feather_client.dataframe import DataFrameClient
import polars as pl

df_client = DataFrameClient("http://localhost:8080")

# Get features as Polars DataFrame
df = df_client.get_features_polars(
    entities=["user:1", "user:2"],
    features=["purchase_count"]
)

# Store from Polars
data = pl.DataFrame({
    "entity": ["user:1", "user:2"],
    "score": [0.95, 0.87]
})
df_client.put_features_polars(data, entity_column="entity")
```

### Vector Similarity Search

```python
from feather_client import FeatherClient

client = FeatherClient("http://localhost:8080")

# Create a vector index
client.vectors.create_index(
    name="product_embeddings",
    dimension=384,
    distance_type="cosine"
)

# Upsert vectors
client.vectors.upsert(
    index="product_embeddings",
    vectors=[
        {"id": "prod:1", "vector": [0.1, 0.2, ...], "metadata": {"name": "Widget"}},
        {"id": "prod:2", "vector": [0.3, 0.1, ...], "metadata": {"name": "Gadget"}},
    ]
)

# Search for similar vectors
results = client.vectors.search(
    index="product_embeddings",
    vector=[0.15, 0.18, ...],
    top_k=10,
    include_metadata=True
)

for result in results:
    print(f"{result.id}: {result.score:.4f} - {result.metadata}")
```

## Feature Groups

```python
from feather_client import FeatherClient, FeatureGroup, FeatureSpec

client = FeatherClient("http://localhost:8080")

# List groups
groups = client.list_groups()

# Create a group
group = FeatureGroup(
    name="user_features",
    entity_type="user",
    ttl=86400,  # 24 hours
    features=[
        FeatureSpec(name="purchase_count", data_type="int"),
        FeatureSpec(name="avg_order_value", data_type="float"),
    ]
)
client.create_group(group)
```

## Health Checks

```python
client = FeatherClient("http://localhost:8080")

# Full health check
health = client.health()
print(f"Status: {health.status}")

# Quick checks
if client.ready():
    print("Server is ready")

if client.live():
    print("Server is alive")
```

## Error Handling

```python
from feather_client import FeatherClient, NotFoundError, ValidationError, FeatherError

client = FeatherClient("http://localhost:8080")

try:
    features = client.get_features("user:unknown", ["score"])
except NotFoundError:
    print("Entity not found")
except ValidationError as e:
    print(f"Invalid request: {e.message}")
except FeatherError as e:
    print(f"Server error ({e.status_code}): {e.message}")
```

## Configuration

```python
client = FeatherClient(
    base_url="http://localhost:8080",
    timeout=60.0,  # Request timeout in seconds
    headers={
        "Authorization": "Bearer <token>",
        "X-Custom-Header": "value"
    }
)
```

## Type Hints

The SDK is fully typed. Use with mypy or your IDE for autocomplete:

```python
from feather_client import FeatherClient, Feature

client = FeatherClient()
features: dict[str, Feature] = client.get_features("user:1", ["score"])
```

## Development

```bash
# Install dev dependencies
pip install -e ".[dev]"

# Run tests
pytest

# Type check
mypy feather_client

# Lint
ruff check feather_client
```
