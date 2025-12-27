"""Feather Feature Store Python SDK.

A high-performance Python client for the Feather Feature Store with support for:
- Sync and async operations
- Pandas and Polars DataFrame integration
- Vector similarity search
- Batch operations

Example:
    >>> from feather_client import FeatherClient
    >>>
    >>> client = FeatherClient("http://localhost:8080")
    >>>
    >>> # Get features
    >>> features = client.get_features(
    ...     entity="user:123",
    ...     features=["purchase_count", "avg_order_value"]
    ... )
    >>>
    >>> # Store features
    >>> client.put_features(
    ...     entity="user:123",
    ...     features={"purchase_count": 42, "avg_order_value": 89.99}
    ... )
    >>>
    >>> # Vector search
    >>> results = client.vectors.search(
    ...     index="embeddings",
    ...     vector=[0.1, 0.2, 0.3, ...],
    ...     top_k=10
    ... )
"""

from feather_client.client import FeatherClient
from feather_client.async_client import AsyncFeatherClient
from feather_client.models import (
    Feature,
    FeatureValue,
    FeatureGroup,
    VectorIndex,
    VectorSearchResult,
    FeatherError,
    NotFoundError,
    ValidationError,
)

__version__ = "1.0.0"
__all__ = [
    "FeatherClient",
    "AsyncFeatherClient",
    "Feature",
    "FeatureValue",
    "FeatureGroup",
    "VectorIndex",
    "VectorSearchResult",
    "FeatherError",
    "NotFoundError",
    "ValidationError",
]
