"""Tests for schema inference and feature decorator."""

import datetime
import sys

import pytest

try:
    import pandas as pd

    HAS_PANDAS = True
except ImportError:
    HAS_PANDAS = False

try:
    import polars as pl

    HAS_POLARS = True
except ImportError:
    HAS_POLARS = False

from feather_client.feature_decorator import (
    FeatureGroupDefinition,
    clear_registry,
    feature_group,
    get_registry,
)
from feather_client.schema_inference import (
    InferredSchema,
    infer_from_pandas,
    infer_from_polars,
)


@pytest.mark.skipif(not HAS_PANDAS, reason="pandas not installed")
class TestInferFromPandas:
    """Tests for Pandas schema inference."""

    def test_basic_inference(self):
        df = pd.DataFrame(
            {
                "user_id": ["u1", "u2", "u3"],
                "age": [25, 30, 35],
                "score": [0.9, 0.8, 0.7],
                "name": ["Alice", "Bob", "Charlie"],
            }
        )
        schema = infer_from_pandas(df, name="user_features", entity_column="user_id")

        assert schema.name == "user_features"
        assert schema.entity_column == "user_id"
        assert schema.row_count == 3
        assert len(schema.features) == 3

        feature_names = [f.name for f in schema.features]
        assert "age" in feature_names
        assert "score" in feature_names
        assert "name" in feature_names
        assert "user_id" not in feature_names

    def test_nullable_columns(self):
        df = pd.DataFrame(
            {
                "entity": ["a", "b", "c"],
                "val": [1.0, None, 3.0],
            }
        )
        schema = infer_from_pandas(df, name="test", entity_column="entity")

        val_feat = [f for f in schema.features if f.name == "val"][0]
        assert val_feat.nullable is True

    def test_exclude_columns(self):
        df = pd.DataFrame(
            {
                "entity": ["a", "b"],
                "keep": [1, 2],
                "drop": [3, 4],
            }
        )
        schema = infer_from_pandas(
            df, name="test", entity_column="entity", exclude_columns=["drop"]
        )
        feature_names = [f.name for f in schema.features]
        assert "keep" in feature_names
        assert "drop" not in feature_names

    def test_timestamp_column(self):
        df = pd.DataFrame(
            {
                "entity": ["a"],
                "ts": pd.to_datetime(["2024-01-01"]),
                "val": [1],
            }
        )
        schema = infer_from_pandas(
            df, name="test", entity_column="entity", timestamp_column="ts"
        )
        assert schema.timestamp_column == "ts"
        feature_names = [f.name for f in schema.features]
        assert "ts" not in feature_names
        assert "val" in feature_names

    def test_to_dict(self):
        df = pd.DataFrame(
            {
                "entity": ["a", "b"],
                "age": [25, 30],
            }
        )
        schema = infer_from_pandas(df, name="users", entity_column="entity")
        d = schema.to_dict()

        assert d["name"] == "users"
        assert d["entity_type"] == "entity"
        assert len(d["features"]) == 1
        assert d["features"][0]["name"] == "age"
        assert d["features"][0]["data_type"] == "int64"

    def test_to_yaml(self):
        df = pd.DataFrame(
            {
                "entity": ["a"],
                "age": [25],
            }
        )
        schema = infer_from_pandas(df, name="users", entity_column="entity")
        yaml_str = schema.to_yaml()

        assert "name: users" in yaml_str
        assert "entity_type: entity" in yaml_str
        assert "- name: age" in yaml_str
        assert "data_type: int64" in yaml_str

    def test_stats_computation(self):
        df = pd.DataFrame(
            {
                "entity": ["a", "b", "c"],
                "val": [10.0, 20.0, 30.0],
            }
        )
        schema = infer_from_pandas(df, name="test", entity_column="entity")
        val_feat = [f for f in schema.features if f.name == "val"][0]

        assert val_feat.stats["min"] == 10.0
        assert val_feat.stats["max"] == 30.0
        assert val_feat.stats["mean"] == 20.0
        assert val_feat.stats["null_rate"] == 0.0
        assert val_feat.stats["unique_count"] == 3

    def test_dtype_mapping(self):
        df = pd.DataFrame(
            {
                "entity": ["a"],
                "int_col": pd.array([1], dtype="int64"),
                "float_col": pd.array([1.0], dtype="float64"),
                "bool_col": pd.array([True], dtype="bool"),
                "str_col": pd.array(["x"], dtype="object"),
                "dt_col": pd.to_datetime(["2024-01-01"]),
            }
        )
        schema = infer_from_pandas(df, name="types", entity_column="entity")
        type_map = {f.name: f.data_type for f in schema.features}

        assert type_map["int_col"] == "int64"
        assert type_map["float_col"] == "float64"
        assert type_map["bool_col"] == "bool"
        assert type_map["str_col"] == "string"
        assert type_map["dt_col"] == "timestamp"


@pytest.mark.skipif(not HAS_POLARS, reason="polars not installed")
class TestInferFromPolars:
    """Tests for Polars schema inference."""

    def test_basic_inference(self):
        df = pl.DataFrame(
            {
                "user_id": ["u1", "u2", "u3"],
                "age": [25, 30, 35],
                "score": [0.9, 0.8, 0.7],
            }
        )
        schema = infer_from_polars(df, name="user_features", entity_column="user_id")

        assert schema.name == "user_features"
        assert schema.row_count == 3
        assert len(schema.features) == 2

        feature_names = [f.name for f in schema.features]
        assert "age" in feature_names
        assert "score" in feature_names
        assert "user_id" not in feature_names

    def test_nullable_columns(self):
        df = pl.DataFrame(
            {
                "entity": ["a", "b", "c"],
                "val": [1.0, None, 3.0],
            }
        )
        schema = infer_from_polars(df, name="test", entity_column="entity")
        val_feat = [f for f in schema.features if f.name == "val"][0]
        assert val_feat.nullable is True

    def test_exclude_columns(self):
        df = pl.DataFrame(
            {
                "entity": ["a", "b"],
                "keep": [1, 2],
                "drop": [3, 4],
            }
        )
        schema = infer_from_polars(
            df, name="test", entity_column="entity", exclude_columns=["drop"]
        )
        feature_names = [f.name for f in schema.features]
        assert "keep" in feature_names
        assert "drop" not in feature_names

    def test_stats_computation(self):
        df = pl.DataFrame(
            {
                "entity": ["a", "b", "c"],
                "val": [10.0, 20.0, 30.0],
            }
        )
        schema = infer_from_polars(df, name="test", entity_column="entity")
        val_feat = [f for f in schema.features if f.name == "val"][0]

        assert val_feat.stats["min"] == 10.0
        assert val_feat.stats["max"] == 30.0
        assert val_feat.stats["mean"] == 20.0
        assert val_feat.stats["null_rate"] == 0.0

    def test_to_dict(self):
        df = pl.DataFrame(
            {
                "entity": ["a"],
                "age": [25],
            }
        )
        schema = infer_from_polars(df, name="users", entity_column="entity")
        d = schema.to_dict()
        assert d["name"] == "users"
        assert len(d["features"]) == 1

    def test_dtype_mapping(self):
        df = pl.DataFrame(
            {
                "entity": ["a"],
                "int_col": pl.Series([1], dtype=pl.Int64),
                "float_col": pl.Series([1.0], dtype=pl.Float64),
                "bool_col": pl.Series([True], dtype=pl.Boolean),
                "str_col": pl.Series(["x"], dtype=pl.Utf8),
            }
        )
        schema = infer_from_polars(df, name="types", entity_column="entity")
        type_map = {f.name: f.data_type for f in schema.features}

        assert type_map["int_col"] == "int64"
        assert type_map["float_col"] == "float64"
        assert type_map["bool_col"] == "bool"
        assert type_map["str_col"] == "string"


class TestFeatureDecorator:
    """Tests for decorator-based feature definitions."""

    def setup_method(self):
        clear_registry()

    def test_basic_decorator(self):
        @feature_group("test_features", entity="user_id")
        class TestFeatures:
            age: int
            name: str

        assert hasattr(TestFeatures, "__feather_group__")
        group = TestFeatures.__feather_group__
        assert group.name == "test_features"
        assert group.entity == "user_id"
        assert len(group.features) == 2

    def test_type_mapping(self):
        @feature_group("typed", entity="eid")
        class TypedFeatures:
            int_feat: int
            float_feat: float
            str_feat: str
            bool_feat: bool
            ts_feat: datetime.datetime
            vec_feat: list[float]

        group = TypedFeatures.__feather_group__
        type_map = {f.name: f.data_type for f in group.features}

        assert type_map["int_feat"] == "int64"
        assert type_map["float_feat"] == "float64"
        assert type_map["str_feat"] == "string"
        assert type_map["bool_feat"] == "bool"
        assert type_map["ts_feat"] == "timestamp"
        assert type_map["vec_feat"] == "vector"

    def test_registry(self):
        @feature_group("group_a", entity="id")
        class GroupA:
            x: int

        @feature_group("group_b", entity="id")
        class GroupB:
            y: float

        registry = get_registry()
        assert "group_a" in registry
        assert "group_b" in registry
        assert registry["group_a"].features[0].name == "x"

    def test_to_dict(self):
        @feature_group("my_group", entity="user_id", description="A test group", ttl_seconds=300)
        class MyGroup:
            score: float

        d = MyGroup.__feather_group__.to_dict()
        assert d["name"] == "my_group"
        assert d["entity_type"] == "user_id"
        assert d["description"] == "A test group"
        assert d["ttl"] == 300
        assert d["features"][0]["name"] == "score"
        assert d["features"][0]["data_type"] == "float64"

    def test_feature_names(self):
        @feature_group("names_test", entity="id")
        class NamesTest:
            a: int
            b: str
            c: float

        group = NamesTest.__feather_group__
        assert group.feature_names() == ["a", "b", "c"]

    def test_clear_registry(self):
        @feature_group("temp", entity="id")
        class Temp:
            x: int

        assert "temp" in get_registry()
        clear_registry()
        assert len(get_registry()) == 0

    def test_tags(self):
        @feature_group("tagged", entity="id", tags=["ml", "production"])
        class Tagged:
            x: int

        group = Tagged.__feather_group__
        assert group.tags == ["ml", "production"]

    def test_default_values(self):
        @feature_group("defaults", entity="id")
        class WithDefaults:
            score: float = 0.0

        group = WithDefaults.__feather_group__
        score_feat = [f for f in group.features if f.name == "score"][0]
        assert score_feat.default == 0.0
