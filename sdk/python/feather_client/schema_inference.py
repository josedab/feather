"""
Schema inference utilities for automatically generating Feather feature
definitions from Pandas and Polars DataFrames.
"""

from __future__ import annotations

import datetime
from dataclasses import dataclass, field
from typing import Any, Optional


@dataclass
class InferredFeature:
    """A feature definition inferred from a DataFrame column."""

    name: str
    data_type: str  # int64, float64, string, bool, timestamp, vector
    nullable: bool = False
    description: str = ""
    tags: list[str] = field(default_factory=list)
    stats: dict[str, Any] = field(default_factory=dict)


@dataclass
class InferredSchema:
    """A complete schema inferred from a DataFrame."""

    name: str
    entity_column: str
    timestamp_column: Optional[str]
    features: list[InferredFeature]
    row_count: int = 0
    inferred_at: Optional[datetime.datetime] = None

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary suitable for Feather API registration."""
        return {
            "name": self.name,
            "entity_type": self.entity_column,
            "features": [
                {
                    "name": f.name,
                    "data_type": f.data_type,
                }
                for f in self.features
            ],
        }

    def to_yaml(self) -> str:
        """Render as YAML for config file inclusion."""
        lines = [
            f"name: {self.name}",
            f"entity_type: {self.entity_column}",
            "features:",
        ]
        for f in self.features:
            lines.append(f"  - name: {f.name}")
            lines.append(f"    data_type: {f.data_type}")
            if f.nullable:
                lines.append(f"    nullable: true")
            if f.description:
                lines.append(f"    description: {f.description}")
            if f.tags:
                lines.append(f"    tags: [{', '.join(f.tags)}]")
        return "\n".join(lines) + "\n"


def infer_from_pandas(
    df: Any,
    name: str,
    entity_column: str,
    timestamp_column: Optional[str] = None,
    exclude_columns: Optional[list[str]] = None,
) -> InferredSchema:
    """Infer a Feather feature schema from a Pandas DataFrame.

    Maps pandas dtypes: int64->int64, float64->float64, object/string->string,
    bool->bool, datetime64->timestamp. Computes basic stats (min, max, null_rate,
    unique_count) for each column.
    """
    import pandas as pd

    exclude = {entity_column}
    if timestamp_column:
        exclude.add(timestamp_column)
    if exclude_columns:
        exclude.update(exclude_columns)

    features: list[InferredFeature] = []
    for col in df.columns:
        if col in exclude:
            continue

        dtype = df[col].dtype
        feather_type = _map_pandas_dtype(dtype)
        null_count = int(df[col].isna().sum())
        total = len(df)
        nullable = null_count > 0

        stats: dict[str, Any] = {
            "null_rate": null_count / total if total > 0 else 0.0,
            "unique_count": int(df[col].nunique()),
        }

        if feather_type in ("int64", "float64"):
            non_null = df[col].dropna()
            if len(non_null) > 0:
                stats["min"] = float(non_null.min())
                stats["max"] = float(non_null.max())
                stats["mean"] = float(non_null.mean())

        features.append(
            InferredFeature(
                name=col,
                data_type=feather_type,
                nullable=nullable,
                stats=stats,
            )
        )

    return InferredSchema(
        name=name,
        entity_column=entity_column,
        timestamp_column=timestamp_column,
        features=features,
        row_count=len(df),
        inferred_at=datetime.datetime.now(datetime.timezone.utc),
    )


def infer_from_polars(
    df: Any,
    name: str,
    entity_column: str,
    timestamp_column: Optional[str] = None,
    exclude_columns: Optional[list[str]] = None,
) -> InferredSchema:
    """Infer a Feather feature schema from a Polars DataFrame."""
    import polars as pl

    exclude = {entity_column}
    if timestamp_column:
        exclude.add(timestamp_column)
    if exclude_columns:
        exclude.update(exclude_columns)

    features: list[InferredFeature] = []
    for col in df.columns:
        if col in exclude:
            continue

        dtype = df[col].dtype
        feather_type = _map_polars_dtype(dtype)
        null_count = int(df[col].null_count())
        total = len(df)
        nullable = null_count > 0

        stats: dict[str, Any] = {
            "null_rate": null_count / total if total > 0 else 0.0,
            "unique_count": int(df[col].n_unique()),
        }

        if feather_type in ("int64", "float64"):
            non_null = df[col].drop_nulls()
            if len(non_null) > 0:
                stats["min"] = float(non_null.min())  # type: ignore[arg-type]
                stats["max"] = float(non_null.max())  # type: ignore[arg-type]
                stats["mean"] = float(non_null.mean())  # type: ignore[arg-type]

        features.append(
            InferredFeature(
                name=col,
                data_type=feather_type,
                nullable=nullable,
                stats=stats,
            )
        )

    return InferredSchema(
        name=name,
        entity_column=entity_column,
        timestamp_column=timestamp_column,
        features=features,
        row_count=len(df),
        inferred_at=datetime.datetime.now(datetime.timezone.utc),
    )


def _map_pandas_dtype(dtype: Any) -> str:
    """Map pandas dtype to Feather data type string."""
    dtype_str = str(dtype)

    if "int" in dtype_str:
        return "int64"
    if "float" in dtype_str:
        return "float64"
    if "bool" in dtype_str:
        return "bool"
    if "datetime" in dtype_str:
        return "timestamp"
    # object and string dtypes
    return "string"


def _map_polars_dtype(dtype: Any) -> str:
    """Map Polars dtype to Feather data type string."""
    import polars as pl

    if dtype in (pl.Int8, pl.Int16, pl.Int32, pl.Int64, pl.UInt8, pl.UInt16, pl.UInt32, pl.UInt64):
        return "int64"
    if dtype in (pl.Float32, pl.Float64):
        return "float64"
    if dtype == pl.Boolean:
        return "bool"
    if dtype in (pl.Date, pl.Datetime, pl.Time):
        return "timestamp"
    if dtype == pl.List(pl.Float64) or dtype == pl.List(pl.Float32):
        return "vector"
    return "string"
