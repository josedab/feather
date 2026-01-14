"""Tests for the declarative feature definition SDK.

All tests run without a live Feather server - HTTP calls are mocked.
"""

from __future__ import annotations

import json
from typing import Any
from unittest.mock import MagicMock, patch

import pytest

from feather_client.compute import ComputeEngine, ComputePlan
from feather_client.declarative import (
    FeatureDefinition,
    FeatureRegistry,
    FeatureSet,
    FeatureView,
    _GLOBAL_FEATURES,
    feature,
    infer_feather_type,
)


# ---------------------------------------------------------------------------
# Helpers – clear global state between tests
# ---------------------------------------------------------------------------

@pytest.fixture(autouse=True)
def _clear_global_features():
    """Clear the global feature registry before each test."""
    _GLOBAL_FEATURES.clear()
    yield
    _GLOBAL_FEATURES.clear()


# ---------------------------------------------------------------------------
# test_feature_decorator
# ---------------------------------------------------------------------------


class TestFeatureDecorator:
    def test_basic_decoration(self):
        @feature(entity_type="user", description="Sum of purchases")
        def total_purchases(purchase_total: float) -> float:
            return purchase_total

        assert hasattr(total_purchases, "_feather_feature")
        defn: FeatureDefinition = total_purchases._feather_feature
        assert defn.name == "total_purchases"
        assert defn.entity_type == "user"
        assert defn.description == "Sum of purchases"
        assert defn.output_type == "float64"
        assert defn.inputs == {"purchase_total": "float64"}

    def test_tags_ttl_version(self):
        @feature(entity_type="item", description="d", tags=["ml"], ttl=3600, version=2)
        def price(raw: float) -> float:
            return raw

        defn: FeatureDefinition = price._feather_feature
        assert defn.tags == ["ml"]
        assert defn.ttl == 3600
        assert defn.version == 2

    def test_multiple_params(self):
        @feature(entity_type="user", description="multi")
        def combined(a: int, b: float, c: str) -> str:
            return f"{a}-{b}-{c}"

        defn: FeatureDefinition = combined._feather_feature
        assert defn.inputs == {"a": "int64", "b": "float64", "c": "string"}
        assert defn.output_type == "string"

    def test_bool_return(self):
        @feature(entity_type="user", description="flag")
        def is_active(active: bool) -> bool:
            return active

        defn: FeatureDefinition = is_active._feather_feature
        assert defn.output_type == "bool"
        assert defn.inputs == {"active": "bool"}

    def test_global_registration(self):
        @feature(entity_type="x", description="d")
        def my_feat(v: int) -> int:
            return v

        assert "my_feat" in _GLOBAL_FEATURES

    def test_function_still_callable(self):
        @feature(entity_type="user", description="identity")
        def identity(x: float) -> float:
            return x

        assert identity(42.0) == 42.0


# ---------------------------------------------------------------------------
# test_type_inference
# ---------------------------------------------------------------------------


class TestTypeInference:
    def test_int(self):
        assert infer_feather_type(int) == "int64"

    def test_float(self):
        assert infer_feather_type(float) == "float64"

    def test_str(self):
        assert infer_feather_type(str) == "string"

    def test_bool(self):
        assert infer_feather_type(bool) == "bool"

    def test_bytes(self):
        assert infer_feather_type(bytes) == "bytes"

    def test_list_float_is_vector(self):
        assert infer_feather_type(list[float]) == "vector"

    def test_unknown_type_raises(self):
        with pytest.raises(TypeError):
            infer_feather_type(dict)

    def test_vector_in_decorator(self):
        @feature(entity_type="item", description="embedding")
        def embedding(raw: str) -> list[float]:
            return [0.0]

        defn: FeatureDefinition = embedding._feather_feature
        assert defn.output_type == "vector"


# ---------------------------------------------------------------------------
# test_feature_set
# ---------------------------------------------------------------------------


class TestFeatureSet:
    def test_basic_set(self):
        fs = FeatureSet(name="user_features", entity_type="user", tags=["core"])

        @fs.add(description="Login count")
        def login_count(raw_logins: int) -> int:
            return raw_logins

        assert "login_count" in fs.features
        assert fs.features["login_count"].entity_type == "user"
        assert "core" in fs.features["login_count"].tags

    def test_inherits_entity_type(self):
        fs = FeatureSet(name="s", entity_type="session")

        @fs.add(description="dur")
        def duration(seconds: float) -> float:
            return seconds

        assert fs.features["duration"].entity_type == "session"

    def test_tag_merging(self):
        fs = FeatureSet(name="s", entity_type="user", tags=["base"])

        @fs.add(description="d", tags=["extra"])
        def feat(x: int) -> int:
            return x

        assert set(fs.features["feat"].tags) == {"base", "extra"}

    def test_list_features(self):
        fs = FeatureSet(name="s", entity_type="e")

        @fs.add(description="a")
        def alpha(x: int) -> int:
            return x

        @fs.add(description="b")
        def beta(x: int) -> int:
            return x

        assert sorted(fs.list_features()) == ["alpha", "beta"]

    def test_ttl_version_override(self):
        fs = FeatureSet(name="s", entity_type="e", ttl=100, version=3)

        @fs.add(description="d", ttl=999, version=7)
        def f(x: int) -> int:
            return x

        assert fs.features["f"].ttl == 999
        assert fs.features["f"].version == 7

    def test_ttl_version_inherit(self):
        fs = FeatureSet(name="s", entity_type="e", ttl=100, version=3)

        @fs.add(description="d")
        def g(x: int) -> int:
            return x

        assert fs.features["g"].ttl == 100
        assert fs.features["g"].version == 3


# ---------------------------------------------------------------------------
# test_registry_discover
# ---------------------------------------------------------------------------


class TestRegistryDiscover:
    def test_discover_from_global(self):
        @feature(entity_type="user", description="d")
        def feat_a(x: int) -> int:
            return x

        @feature(entity_type="user", description="d")
        def feat_b(x: float) -> float:
            return x

        registry = FeatureRegistry()
        discovered = registry.discover()
        assert "feat_a" in discovered
        assert "feat_b" in discovered

    def test_list_after_discover(self):
        @feature(entity_type="user", description="sum")
        def total(x: int) -> int:
            return x

        registry = FeatureRegistry()
        registry.discover()
        items = registry.list()
        names = [i["name"] for i in items]
        assert "total" in names

    def test_manual_add(self):
        def my_func(x: int) -> int:
            return x

        defn = FeatureDefinition(
            name="manual_feat",
            func=my_func,
            entity_type="item",
            description="manual",
            inputs={"x": "int64"},
            output_type="int64",
        )
        registry = FeatureRegistry()
        registry.add(defn)
        items = registry.list()
        assert any(i["name"] == "manual_feat" for i in items)


# ---------------------------------------------------------------------------
# test_compute
# ---------------------------------------------------------------------------


class TestCompute:
    def test_simple_compute(self):
        @feature(entity_type="user", description="double")
        def doubled(x: float) -> float:
            return x * 2

        registry = FeatureRegistry()
        registry.discover()
        assert registry.compute("doubled", {"x": 5.0}) == 10.0

    def test_engine_dependency_chain(self):
        engine = ComputeEngine()
        engine.register("base", lambda x: x * 2)
        engine.register("derived", lambda base: base + 1, deps=["base"])

        result = engine.compute("derived", {"x": 3})
        assert result == 7  # base=6, derived=7

    def test_incremental_caching(self):
        call_count = 0

        def expensive(x: float) -> float:
            nonlocal call_count
            call_count += 1
            return x ** 2

        engine = ComputeEngine()
        engine.register("sq", expensive)

        assert engine.compute_incremental("sq", {"x": 3.0}) == 9.0
        assert call_count == 1
        # Same inputs → cached.
        assert engine.compute_incremental("sq", {"x": 3.0}) == 9.0
        assert call_count == 1
        # Changed inputs → recomputed.
        assert engine.compute_incremental("sq", {"x": 4.0}) == 16.0
        assert call_count == 2

    def test_invalidate_cache(self):
        call_count = 0

        def f(x: float) -> float:
            nonlocal call_count
            call_count += 1
            return x

        engine = ComputeEngine()
        engine.register("f", f)
        engine.compute_incremental("f", {"x": 1.0})
        assert call_count == 1
        engine.invalidate("f")
        engine.compute_incremental("f", {"x": 1.0})
        assert call_count == 2

    def test_unknown_feature_raises(self):
        engine = ComputeEngine()
        with pytest.raises(ValueError, match="Unknown feature"):
            engine.compute("nope", {})


# ---------------------------------------------------------------------------
# test_compute_plan
# ---------------------------------------------------------------------------


class TestComputePlan:
    def test_single_feature(self):
        engine = ComputeEngine()
        engine.register("a", lambda x: x)
        plan = engine.plan("a")
        assert plan.execution_order == ["a"]

    def test_dependency_order(self):
        engine = ComputeEngine()
        engine.register("base", lambda x: x)
        engine.register("mid", lambda base: base, deps=["base"])
        engine.register("top", lambda mid: mid, deps=["mid"])

        plan = engine.plan("top")
        assert plan.execution_order.index("base") < plan.execution_order.index("mid")
        assert plan.execution_order.index("mid") < plan.execution_order.index("top")

    def test_cycle_detection(self):
        engine = ComputeEngine()
        engine.register("a", lambda b: b, deps=["b"])
        engine.register("b", lambda a: a, deps=["a"])
        with pytest.raises(ValueError, match="cycle"):
            engine.plan("a")

    def test_plan_str(self):
        engine = ComputeEngine()
        engine.register("a", lambda x: x)
        plan = engine.plan("a")
        text = str(plan)
        assert "ComputePlan" in text
        assert "a" in text


# ---------------------------------------------------------------------------
# test_export_schema
# ---------------------------------------------------------------------------


class TestExportSchema:
    def test_json_export(self):
        @feature(entity_type="user", description="count")
        def login_count(raw: int) -> int:
            return raw

        registry = FeatureRegistry()
        registry.discover()
        out = registry.export_schema("json")
        data = json.loads(out)
        assert "features" in data
        names = [f["name"] for f in data["features"]]
        assert "login_count" in names

    def test_yaml_export(self):
        @feature(entity_type="item", description="price feature", tags=["ml"])
        def price(raw: float) -> float:
            return raw

        registry = FeatureRegistry()
        registry.discover()
        out = registry.export_schema("yaml")
        assert "features:" in out
        assert "price" in out
        assert "ml" in out

    def test_invalid_format_raises(self):
        registry = FeatureRegistry()
        with pytest.raises(ValueError, match="Unsupported format"):
            registry.export_schema("xml")


# ---------------------------------------------------------------------------
# test_feature_view
# ---------------------------------------------------------------------------


class TestFeatureView:
    def _mock_client(self, response_json: dict[str, Any], status_code: int = 200):
        """Create a FeatureView with a mocked httpx.Client."""
        mock_http = MagicMock(spec=["get"])
        mock_resp = MagicMock()
        mock_resp.status_code = status_code
        mock_resp.json.return_value = response_json
        mock_http.get.return_value = mock_resp
        return FeatureView(base_url="http://test:8080", http_client=mock_http), mock_http

    def test_query_single_entity(self):
        resp = {
            "entities": {
                "user:1": {
                    "features": {
                        "score": {"value": 0.95, "timestamp": 0},
                    }
                }
            }
        }
        view, mock_http = self._mock_client(resp)
        result = view.query(entity_keys=["user:1"], features=["score"])
        assert result["user:1"]["score"] == 0.95
        mock_http.get.assert_called_once()

    def test_query_with_as_of(self):
        resp = {
            "entities": {
                "user:1": {
                    "features": {
                        "score": {"value": 0.5, "timestamp": 0},
                    }
                }
            }
        }
        view, mock_http = self._mock_client(resp)
        result = view.query(
            entity_keys=["user:1"],
            features=["score"],
            as_of="2024-01-01T00:00:00Z",
        )
        assert result["user:1"]["score"] == 0.5
        # Should use the /history endpoint.
        call_args = mock_http.get.call_args
        assert "/v1/features/history" in call_args[0][0]

    def test_to_dataframe(self):
        resp = {
            "entities": {
                "user:1": {
                    "features": {
                        "a": {"value": 1},
                        "b": {"value": 2},
                    }
                },
                "user:2": {
                    "features": {
                        "a": {"value": 3},
                        "b": {"value": 4},
                    }
                },
            }
        }
        # Need separate responses per entity_key since query loops.
        mock_http = MagicMock(spec=["get"])
        mock_resp = MagicMock()
        mock_resp.status_code = 200
        mock_resp.json.return_value = resp
        mock_http.get.return_value = mock_resp

        view = FeatureView(base_url="http://test:8080", http_client=mock_http)
        view.query(entity_keys=["user:1", "user:2"], features=["a", "b"])
        df_dict = view.to_dataframe()
        assert "a" in df_dict
        assert "b" in df_dict

    def test_query_http_error(self):
        view, _ = self._mock_client({}, status_code=500)
        result = view.query(entity_keys=["user:1"], features=["score"])
        assert result["user:1"] == {}


# ---------------------------------------------------------------------------
# test_validate
# ---------------------------------------------------------------------------


class TestValidate:
    def test_valid_features(self):
        @feature(entity_type="user", description="d")
        def ok(x: int) -> int:
            return x

        registry = FeatureRegistry()
        registry.discover()
        errors = registry.validate()
        assert errors == []

    def test_missing_entity_type(self):
        defn = FeatureDefinition(
            name="bad",
            func=lambda x: x,
            entity_type="",
            description="d",
        )
        registry = FeatureRegistry()
        registry.add(defn)
        errors = registry.validate()
        assert any("missing entity_type" in e for e in errors)


# ---------------------------------------------------------------------------
# test_register_all
# ---------------------------------------------------------------------------


class TestRegisterAll:
    def test_register_groups_created(self):
        @feature(entity_type="user", description="d")
        def feat_x(x: int) -> int:
            return x

        mock_http = MagicMock(spec=["post", "get"])
        mock_resp = MagicMock()
        mock_resp.status_code = 200
        mock_resp.json.return_value = {
            "name": "user_features",
            "entity_type": "user",
            "features": [{"name": "feat_x", "data_type": "int64"}],
        }
        mock_http.post.return_value = mock_resp

        registry = FeatureRegistry(http_client=mock_http)
        registry.discover()
        groups = registry.register_all()

        assert len(groups) == 1
        assert groups[0].name == "user_features"
        mock_http.post.assert_called_once()


# ---------------------------------------------------------------------------
# test_compute_and_store
# ---------------------------------------------------------------------------


class TestComputeAndStore:
    def test_compute_and_store(self):
        @feature(entity_type="user", description="sq")
        def squared(x: float) -> float:
            return x ** 2

        mock_http = MagicMock(spec=["post", "get"])
        mock_resp = MagicMock()
        mock_resp.status_code = 200
        mock_http.post.return_value = mock_resp

        registry = FeatureRegistry(http_client=mock_http)
        registry.discover()
        result = registry.compute_and_store("squared", "user:1", {"x": 4.0})
        assert result == 16.0
        mock_http.post.assert_called_once()
        payload = mock_http.post.call_args[1]["json"]
        assert payload["entity_key"] == "user:1"
        assert payload["features"]["squared"] == 16.0


# ---------------------------------------------------------------------------
# test_on_demand decorator
# ---------------------------------------------------------------------------


class TestOnDemand:
    def test_on_demand_decorator(self):
        from feather_client.declarative import on_demand, _GLOBAL_ON_DEMAND

        @on_demand(
            entity_type="user",
            source_features=["purchase_count", "return_count"],
            description="Return rate",
        )
        def return_rate(purchase_count: int = 0, return_count: int = 0) -> float:
            if purchase_count == 0:
                return 0.0
            return return_count / purchase_count

        assert "return_rate" in _GLOBAL_ON_DEMAND
        defn = _GLOBAL_ON_DEMAND["return_rate"]
        assert defn.entity_type == "user"
        assert defn.source_features == ["purchase_count", "return_count"]
        assert defn.output_type == "float64"

    def test_on_demand_computation(self):
        from feather_client.declarative import on_demand

        @on_demand(
            entity_type="item",
            source_features=["price", "quantity"],
            description="Total value",
        )
        def total_value(price: float = 0.0, quantity: int = 0) -> float:
            return price * quantity

        result = total_value(price=10.5, quantity=3)
        assert result == 31.5


# ---------------------------------------------------------------------------
# test_feature_service
# ---------------------------------------------------------------------------


class TestFeatureService:
    def test_discover(self):
        from feather_client.declarative import FeatureService, on_demand, _GLOBAL_ON_DEMAND

        @on_demand(
            entity_type="user",
            source_features=["clicks"],
            description="Doubled clicks",
        )
        def doubled_clicks(clicks: int = 0) -> int:
            return clicks * 2

        service = FeatureService()
        discovered = service.discover()
        assert "doubled_clicks" in discovered

    def test_list(self):
        from feather_client.declarative import FeatureService, OnDemandDefinition

        service = FeatureService()
        service.add(OnDemandDefinition(
            name="test_feat",
            func=lambda x=0: x,
            entity_type="user",
            description="test",
            source_features=["x"],
        ))
        listing = service.list()
        assert len(listing) == 1
        assert listing[0]["name"] == "test_feat"
