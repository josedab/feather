"""DataFrame integration for Feather Feature Store.

Provides utilities to work with Pandas and Polars DataFrames for batch
feature retrieval and storage.
"""

from typing import TYPE_CHECKING, Any, Optional, Union

from feather_client.client import FeatherClient
from feather_client.models import Feature

if TYPE_CHECKING:
    import pandas as pd
    import polars as pl


def _has_pandas() -> bool:
    try:
        import pandas  # noqa: F401

        return True
    except ImportError:
        return False


def _has_polars() -> bool:
    try:
        import polars  # noqa: F401

        return True
    except ImportError:
        return False


class DataFrameClient:
    """DataFrame-aware wrapper for FeatherClient.

    Provides convenient methods for working with Pandas and Polars DataFrames.

    Example with Pandas:
        >>> from feather_client.dataframe import DataFrameClient
        >>> import pandas as pd
        >>>
        >>> df_client = DataFrameClient("http://localhost:8080")
        >>>
        >>> # Get features as DataFrame
        >>> df = df_client.get_features_df(
        ...     entities=["user:1", "user:2", "user:3"],
        ...     features=["purchase_count", "avg_order_value"]
        ... )
        >>>
        >>> # Store features from DataFrame
        >>> df = pd.DataFrame({
        ...     "entity": ["user:1", "user:2"],
        ...     "purchase_count": [10, 20],
        ...     "avg_order_value": [50.0, 75.0]
        ... })
        >>> df_client.put_features_df(df, entity_column="entity")

    Example with Polars:
        >>> import polars as pl
        >>>
        >>> # Get features as Polars DataFrame
        >>> df = df_client.get_features_polars(
        ...     entities=["user:1", "user:2"],
        ...     features=["purchase_count"]
        ... )
    """

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        *,
        timeout: float = 30.0,
        client: Optional[FeatherClient] = None,
        ingestion_url: Optional[str] = None,
    ):
        """Initialize DataFrameClient.

        Args:
            base_url: Base URL of the Feather server
            timeout: Request timeout in seconds
            client: Optional existing FeatherClient to use
            ingestion_url: Optional URL for ingestion server (defaults to port 8081)
        """
        self._client = client or FeatherClient(base_url, timeout=timeout)
        self._owns_client = client is None
        self._ingestion_url = ingestion_url

    def close(self) -> None:
        """Close the client if we own it."""
        if self._owns_client:
            self._client.close()

    def __enter__(self) -> "DataFrameClient":
        return self

    def __exit__(self, *args: Any) -> None:
        self.close()

    def get_features_df(
        self,
        entities: list[str],
        features: list[str],
        *,
        include_timestamps: bool = False,
    ) -> "pd.DataFrame":
        """Get features for multiple entities as a Pandas DataFrame.

        Args:
            entities: List of entity keys
            features: List of feature names
            include_timestamps: Include timestamp columns

        Returns:
            Pandas DataFrame with entity column and feature columns
        """
        if not _has_pandas():
            raise ImportError(
                "pandas is required for this method. "
                "Install with: pip install feather-client[pandas]"
            )

        import pandas as pd

        # Fetch features
        result = self._client.get_features_batch(entities, features)

        # Build DataFrame
        rows = []
        for entity in entities:
            row: dict[str, Any] = {"entity": entity}
            entity_features = result.get(entity, {})

            for feature_name in features:
                feature = entity_features.get(feature_name)
                if feature:
                    row[feature_name] = feature.value
                    if include_timestamps:
                        row[f"{feature_name}_timestamp"] = feature.timestamp
                else:
                    row[feature_name] = None
                    if include_timestamps:
                        row[f"{feature_name}_timestamp"] = None

            rows.append(row)

        return pd.DataFrame(rows)

    def get_features_polars(
        self,
        entities: list[str],
        features: list[str],
        *,
        include_timestamps: bool = False,
    ) -> "pl.DataFrame":
        """Get features for multiple entities as a Polars DataFrame.

        Args:
            entities: List of entity keys
            features: List of feature names
            include_timestamps: Include timestamp columns

        Returns:
            Polars DataFrame with entity column and feature columns
        """
        if not _has_polars():
            raise ImportError(
                "polars is required for this method. "
                "Install with: pip install feather-client[polars]"
            )

        import polars as pl

        # Fetch features
        result = self._client.get_features_batch(entities, features)

        # Build column data
        data: dict[str, list[Any]] = {"entity": entities}
        for feature_name in features:
            data[feature_name] = []
            if include_timestamps:
                data[f"{feature_name}_timestamp"] = []

        for entity in entities:
            entity_features = result.get(entity, {})
            for feature_name in features:
                feature = entity_features.get(feature_name)
                if feature:
                    data[feature_name].append(feature.value)
                    if include_timestamps:
                        data[f"{feature_name}_timestamp"].append(feature.timestamp)
                else:
                    data[feature_name].append(None)
                    if include_timestamps:
                        data[f"{feature_name}_timestamp"].append(None)

        return pl.DataFrame(data)

    def put_features_df(
        self,
        df: "pd.DataFrame",
        entity_column: str = "entity",
        *,
        feature_columns: Optional[list[str]] = None,
        timestamp_column: Optional[str] = None,
        batch_size: int = 1000,
    ) -> int:
        """Store features from a Pandas DataFrame.

        Uses batch API for efficient bulk inserts, avoiding N+1 individual calls.

        Args:
            df: DataFrame with entity column and feature columns
            entity_column: Name of the column containing entity keys
            feature_columns: Columns to use as features (default: all except entity)
            timestamp_column: Optional column containing timestamps
            batch_size: Number of rows to send per batch request (default: 1000)

        Returns:
            Number of entities successfully updated
        """
        if not _has_pandas():
            raise ImportError(
                "pandas is required for this method. "
                "Install with: pip install feather-client[pandas]"
            )

        if entity_column not in df.columns:
            raise ValueError(f"Entity column '{entity_column}' not found in DataFrame")

        # Determine feature columns
        if feature_columns is None:
            exclude = {entity_column}
            if timestamp_column:
                exclude.add(timestamp_column)
            feature_columns = [c for c in df.columns if c not in exclude]

        # Build batch updates
        updates: list[dict[str, Any]] = []
        for _, row in df.iterrows():
            entity = str(row[entity_column])
            features = {col: row[col] for col in feature_columns if row[col] is not None}

            if not features:
                continue

            update: dict[str, Any] = {
                "entity_key": entity,
                "features": features,
            }
            if timestamp_column and timestamp_column in row:
                update["timestamp"] = int(row[timestamp_column])

            updates.append(update)

        if not updates:
            return 0

        # Send in batches
        total_success = 0
        for i in range(0, len(updates), batch_size):
            batch = updates[i : i + batch_size]
            result = self._client.put_features_batch(
                batch, ingestion_url=self._ingestion_url
            )
            total_success += result.get("success", 0)

        return total_success

    def put_features_polars(
        self,
        df: "pl.DataFrame",
        entity_column: str = "entity",
        *,
        feature_columns: Optional[list[str]] = None,
        timestamp_column: Optional[str] = None,
        batch_size: int = 1000,
    ) -> int:
        """Store features from a Polars DataFrame.

        Uses batch API for efficient bulk inserts, avoiding N+1 individual calls.

        Args:
            df: DataFrame with entity column and feature columns
            entity_column: Name of the column containing entity keys
            feature_columns: Columns to use as features (default: all except entity)
            timestamp_column: Optional column containing timestamps
            batch_size: Number of rows to send per batch request (default: 1000)

        Returns:
            Number of entities successfully updated
        """
        if not _has_polars():
            raise ImportError(
                "polars is required for this method. "
                "Install with: pip install feather-client[polars]"
            )

        if entity_column not in df.columns:
            raise ValueError(f"Entity column '{entity_column}' not found in DataFrame")

        # Determine feature columns
        if feature_columns is None:
            exclude = {entity_column}
            if timestamp_column:
                exclude.add(timestamp_column)
            feature_columns = [c for c in df.columns if c not in exclude]

        # Build batch updates
        updates: list[dict[str, Any]] = []
        for row in df.iter_rows(named=True):
            entity = str(row[entity_column])
            features = {
                col: row[col] for col in feature_columns if row[col] is not None
            }

            if not features:
                continue

            update: dict[str, Any] = {
                "entity_key": entity,
                "features": features,
            }
            if timestamp_column and timestamp_column in row:
                update["timestamp"] = int(row[timestamp_column])

            updates.append(update)

        if not updates:
            return 0

        # Send in batches
        total_success = 0
        for i in range(0, len(updates), batch_size):
            batch = updates[i : i + batch_size]
            result = self._client.put_features_batch(
                batch, ingestion_url=self._ingestion_url
            )
            total_success += result.get("success", 0)

        return total_success

    def enrich_df(
        self,
        df: "pd.DataFrame",
        entity_column: str,
        features: list[str],
        *,
        prefix: str = "",
    ) -> "pd.DataFrame":
        """Enrich a DataFrame with features from Feather.

        Args:
            df: Input DataFrame
            entity_column: Column containing entity keys
            features: Feature names to add
            prefix: Prefix for feature column names

        Returns:
            DataFrame with added feature columns
        """
        if not _has_pandas():
            raise ImportError("pandas is required")

        import pandas as pd

        entities = df[entity_column].unique().tolist()
        feature_data = self._client.get_features_batch(entities, features)

        # Create feature columns
        for feature_name in features:
            col_name = f"{prefix}{feature_name}" if prefix else feature_name
            df[col_name] = df[entity_column].map(
                lambda e: (
                    feature_data.get(e, {}).get(feature_name, Feature(value=None)).value
                )
            )

        return df

    def enrich_polars(
        self,
        df: "pl.DataFrame",
        entity_column: str,
        features: list[str],
        *,
        prefix: str = "",
    ) -> "pl.DataFrame":
        """Enrich a Polars DataFrame with features from Feather.

        Args:
            df: Input DataFrame
            entity_column: Column containing entity keys
            features: Feature names to add
            prefix: Prefix for feature column names

        Returns:
            DataFrame with added feature columns
        """
        if not _has_polars():
            raise ImportError("polars is required")

        import polars as pl

        entities = df[entity_column].unique().to_list()
        feature_data = self._client.get_features_batch(entities, features)

        # Build lookup dictionaries
        lookups: dict[str, dict[str, Any]] = {}
        for feature_name in features:
            lookups[feature_name] = {}
            for entity in entities:
                feat = feature_data.get(entity, {}).get(feature_name)
                lookups[feature_name][entity] = feat.value if feat else None

        # Add columns
        for feature_name in features:
            col_name = f"{prefix}{feature_name}" if prefix else feature_name
            lookup = lookups[feature_name]
            df = df.with_columns(
                pl.col(entity_column).map_elements(
                    lambda e: lookup.get(e),
                    return_dtype=pl.Object,
                ).alias(col_name)
            )

        return df


# Convenience functions for quick access


def get_features_df(
    entities: list[str],
    features: list[str],
    *,
    base_url: str = "http://localhost:8080",
    include_timestamps: bool = False,
) -> "pd.DataFrame":
    """Get features as a Pandas DataFrame.

    Convenience function that creates a temporary client.
    """
    with DataFrameClient(base_url) as client:
        return client.get_features_df(
            entities, features, include_timestamps=include_timestamps
        )


def get_features_polars(
    entities: list[str],
    features: list[str],
    *,
    base_url: str = "http://localhost:8080",
    include_timestamps: bool = False,
) -> "pl.DataFrame":
    """Get features as a Polars DataFrame.

    Convenience function that creates a temporary client.
    """
    with DataFrameClient(base_url) as client:
        return client.get_features_polars(
            entities, features, include_timestamps=include_timestamps
        )
