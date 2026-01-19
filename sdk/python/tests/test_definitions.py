"""Tests for the feature definition SDK."""

from __future__ import annotations

import json
import os
import tempfile

import pytest

from feather.definitions import (
    DataType,
    FeatureDefinition,
    FeatureStore,
    SchemaExport,
    ValidationResult,
    _clear_registry,
    _registry,
    depends_on,
    feature,
    window,
)


# ---------------------------------------------------------------------------
# Helpers – clear global state between tests
# ---------------------------------------------------------------------------


@pytest.fixture(autouse=True)
def _clear_global_registry():
    """Clear the global feature registry before and after each test."""
    _clear_registry()
    yield
    _clear_registry()


# ---------------------------------------------------------------------------
# test_basic_feature_definition
# ---------------------------------------------------------------------------


class TestBasicFeatureDefinition:
    def test_decorator_attaches_metadata(self):
        @feature(entity="user", name="age_bucket")
        def user_age_bucket(age: float) -> str:
            if age < 18:
                return "minor"
            return "adult"

        assert hasattr(user_age_bucket, "_feather_definition")
        defn: FeatureDefinition = user_age_bucket._feather_definition
        assert defn.name == "age_bucket"
        assert defn.entity == "user"
        assert defn.data_type == DataType.STRING

    def test_default_name_from_function(self):
        @feature(entity="user")
        def login_count(raw: int) -> int:
            return raw

        defn: FeatureDefinition = login_count._feather_definition
        assert defn.name == "login_count"

    def test_function_remains_callable(self):
        @feature(entity="user")
        def double(x: float) -> float:
            return x * 2

        assert double(5.0) == 10.0

    def test_global_registration(self):
        @feature(entity="user", name="feat_a")
        def feat_a(x: int) -> int:
            return x

        assert "feat_a" in _registry

    def test_description_and_owner(self):
        @feature(entity="item", description="Item price", owner="ml-team", tags=["core"])
        def price(raw: float) -> float:
            return raw

        defn: FeatureDefinition = price._feather_definition
        assert defn.description == "Item price"
        assert defn.owner == "ml-team"
        assert defn.tags == ["core"]

    def test_freshness(self):
        @feature(entity="user", freshness="1h")
        def recent(x: float) -> float:
            return x

        defn: FeatureDefinition = recent._feather_definition
        assert defn.freshness == "1h"


# ---------------------------------------------------------------------------
# test_type_inference
# ---------------------------------------------------------------------------


class TestTypeInference:
    def test_str_return(self):
        @feature(entity="user")
        def name(raw: str) -> str:
            return raw

        assert name._feather_definition.data_type == DataType.STRING

    def test_int_return(self):
        @feature(entity="user")
        def count(raw: int) -> int:
            return raw

        assert count._feather_definition.data_type == DataType.INT

    def test_float_return(self):
        @feature(entity="user")
        def score(raw: float) -> float:
            return raw

        assert score._feather_definition.data_type == DataType.FLOAT

    def test_bool_return(self):
        @feature(entity="user")
        def is_active(raw: bool) -> bool:
            return raw

        assert is_active._feather_definition.data_type == DataType.BOOL

    def test_no_return_annotation_defaults_string(self):
        @feature(entity="user")
        def unknown(raw):
            return raw

        assert unknown._feather_definition.data_type == DataType.STRING


# ---------------------------------------------------------------------------
# test_window_decorator
# ---------------------------------------------------------------------------


class TestWindowDecorator:
    def test_window_applied(self):
        @feature(entity="user", name="daily_total", freshness="1h")
        @window(duration="24h", aggregation="sum")
        def daily_total(amount: float) -> float:
            return amount

        defn: FeatureDefinition = daily_total._feather_definition
        assert defn.window_duration == "24h"
        assert defn.window_aggregation == "sum"

    def test_window_default_aggregation(self):
        @feature(entity="user", name="hourly_count")
        @window(duration="1h")
        def hourly_count(x: int) -> int:
            return x

        defn: FeatureDefinition = hourly_count._feather_definition
        assert defn.window_aggregation == "sum"


# ---------------------------------------------------------------------------
# test_depends_on_decorator
# ---------------------------------------------------------------------------


class TestDependsOn:
    def test_explicit_dependencies(self):
        @feature(entity="user", name="ratio")
        @depends_on("purchase_count", "return_count")
        def ratio(purchase_count: int, return_count: int) -> float:
            if purchase_count == 0:
                return 0.0
            return return_count / purchase_count

        defn: FeatureDefinition = ratio._feather_definition
        assert defn.dependencies == ["purchase_count", "return_count"]


# ---------------------------------------------------------------------------
# test_feature_store
# ---------------------------------------------------------------------------


class TestFeatureStore:
    def test_register_and_list(self):
        @feature(entity="user", name="f1")
        def f1(x: int) -> int:
            return x

        @feature(entity="user", name="f2")
        def f2(x: float) -> float:
            return x

        store = FeatureStore()
        store.register(f1)
        store.register(f2)

        names = [d.name for d in store.list_features()]
        assert "f1" in names
        assert "f2" in names

    def test_register_definition_directly(self):
        defn = FeatureDefinition(name="manual", entity="item")
        store = FeatureStore()
        store.register(defn)
        assert store.get_feature("manual").entity == "item"

    def test_get_feature_missing_raises(self):
        store = FeatureStore()
        with pytest.raises(KeyError, match="not registered"):
            store.get_feature("nonexistent")

    def test_register_invalid_raises(self):
        store = FeatureStore()
        with pytest.raises(ValueError, match="must be"):
            store.register("not_a_feature")

    def test_auto_discover(self):
        @feature(entity="user", name="discovered")
        def discovered(x: int) -> int:
            return x

        store = FeatureStore()
        found = store.auto_discover()
        assert "discovered" in found
        assert store.get_feature("discovered").entity == "user"


# ---------------------------------------------------------------------------
# test_validation
# ---------------------------------------------------------------------------


class TestValidation:
    def test_valid_feature(self):
        @feature(entity="user", name="ok_feat", freshness="30m")
        def ok_feat(x: int) -> int:
            return x

        store = FeatureStore()
        store.register(ok_feat)
        results = store.validate()
        assert len(results) == 1
        assert results[0].valid is True
        assert results[0].errors == []

    def test_invalid_freshness(self):
        @feature(entity="user", name="bad_fresh", freshness="not_valid")
        def bad_fresh(x: int) -> int:
            return x

        store = FeatureStore()
        store.register(bad_fresh)
        results = store.validate()
        assert len(results) == 1
        assert results[0].valid is False
        assert any("freshness" in e for e in results[0].errors)

    def test_missing_return_annotation_warning(self):
        @feature(entity="user", name="no_ann")
        def no_ann(x):
            return x

        store = FeatureStore()
        store.register(no_ann)
        results = store.validate()
        assert any("return type annotation" in w for w in results[0].warnings)

    def test_circular_dependency_detected(self):
        defn_a = FeatureDefinition(name="a", entity="user", dependencies=["b"])
        defn_b = FeatureDefinition(name="b", entity="user", dependencies=["a"])
        store = FeatureStore()
        store.register(defn_a)
        store.register(defn_b)
        results = store.validate()
        cycle_errors = [r for r in results if any("Circular" in e for e in r.errors)]
        assert len(cycle_errors) > 0


# ---------------------------------------------------------------------------
# test_schema_generation
# ---------------------------------------------------------------------------


class TestSchemaGeneration:
    def test_schema_groups_by_entity(self):
        @feature(entity="user", name="user_f")
        def user_f(x: int) -> int:
            return x

        @feature(entity="item", name="item_f")
        def item_f(x: float) -> float:
            return x

        store = FeatureStore()
        store.register(user_f)
        store.register(item_f)
        schema = store.generate_schema()

        assert schema.version == "1.0"
        assert len(schema.groups) == 2
        group_names = {g["name"] for g in schema.groups}
        assert "user_features" in group_names
        assert "item_features" in group_names

    def test_schema_feature_details(self):
        @feature(entity="user", name="score", freshness="5m", owner="ml")
        def score(x: float) -> float:
            return x

        store = FeatureStore()
        store.register(score)
        schema = store.generate_schema()

        feat = schema.groups[0]["features"][0]
        assert feat["name"] == "score"
        assert feat["data_type"] == "float64"
        assert feat["freshness"] == "5m"
        assert feat["owner"] == "ml"


# ---------------------------------------------------------------------------
# test_json_export
# ---------------------------------------------------------------------------


class TestJsonExport:
    def test_export_json(self):
        @feature(entity="user", name="total")
        def total(x: float) -> float:
            return x

        store = FeatureStore()
        store.register(total)

        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
            path = f.name
        try:
            store.export_json(path)
            with open(path) as f:
                data = json.load(f)
            assert data["version"] == "1.0"
            assert len(data["groups"]) == 1
            assert data["groups"][0]["features"][0]["name"] == "total"
        finally:
            os.unlink(path)

    def test_export_yaml(self):
        @feature(entity="user", name="clicks", freshness="1h")
        def clicks(x: int) -> int:
            return x

        store = FeatureStore()
        store.register(clicks)

        with tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False) as f:
            path = f.name
        try:
            store.export_yaml(path)
            with open(path) as f:
                content = f.read()
            assert "version:" in content
            assert "clicks" in content
            assert "1h" in content
        finally:
            os.unlink(path)


# ---------------------------------------------------------------------------
# test_dependency_graph
# ---------------------------------------------------------------------------


class TestDependencyGraph:
    def test_graph_structure(self):
        @feature(entity="user", name="base")
        def base(raw: int) -> int:
            return raw

        @feature(entity="user", name="derived")
        @depends_on("base")
        def derived(base: int) -> int:
            return base * 2

        store = FeatureStore()
        store.register(base)
        store.register(derived)
        graph = store.dependency_graph()

        assert "base" in graph
        assert "derived" in graph
        assert graph["derived"] == ["base"]


# ---------------------------------------------------------------------------
# test_local_compute
# ---------------------------------------------------------------------------


class TestLocalCompute:
    def test_compute(self):
        @feature(entity="user", name="squared")
        def squared(x: float) -> float:
            return x ** 2

        store = FeatureStore()
        store.register(squared)
        assert store.compute("squared", x=3.0) == 9.0

    def test_compute_missing_raises(self):
        store = FeatureStore()
        with pytest.raises(KeyError):
            store.compute("missing", x=1)

    def test_compute_no_func_raises(self):
        defn = FeatureDefinition(name="no_func", entity="user")
        store = FeatureStore()
        store.register(defn)
        with pytest.raises(TypeError, match="no callable"):
            store.compute("no_func")
