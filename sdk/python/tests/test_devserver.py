"""Tests for the Feather dev server."""

import threading
import time

import httpx
import pytest

from feather_client.declarative import feature, _GLOBAL_FEATURES
from feather_client.devserver import DevServer, InMemoryStore


# ---------------------------------------------------------------------------
# InMemoryStore tests
# ---------------------------------------------------------------------------


class TestInMemoryStore:
    def test_put_and_get(self):
        store = InMemoryStore()
        store.put("user:1", {"clicks": 10, "purchases": 5.5})
        result = store.get("user:1")
        assert result["clicks"]["value"] == 10
        assert result["purchases"]["value"] == 5.5

    def test_get_specific_features(self):
        store = InMemoryStore()
        store.put("user:1", {"clicks": 10, "purchases": 5.5, "name": "Alice"})
        result = store.get("user:1", ["clicks", "purchases"])
        assert len(result) == 2
        assert "name" not in result

    def test_get_nonexistent_entity(self):
        store = InMemoryStore()
        result = store.get("user:999")
        assert result == {}

    def test_register_group(self):
        store = InMemoryStore()
        store.register_group({"name": "user_features", "entity_type": "user"})
        groups = store.list_groups()
        assert len(groups) == 1
        assert groups[0]["name"] == "user_features"

    def test_stats(self):
        store = InMemoryStore()
        store.put("user:1", {"clicks": 10})
        store.put("user:2", {"clicks": 20, "purchases": 5})
        stats = store.stats()
        assert stats["entities"] == 2
        assert stats["features"] == 3

    def test_overwrite_feature(self):
        store = InMemoryStore()
        store.put("user:1", {"clicks": 10})
        store.put("user:1", {"clicks": 20})
        result = store.get("user:1")
        assert result["clicks"]["value"] == 20


# ---------------------------------------------------------------------------
# DevServer tests
# ---------------------------------------------------------------------------


class TestDevServer:
    def setup_method(self):
        # Clear global feature registry between tests
        _GLOBAL_FEATURES.clear()

    def test_server_lifecycle(self):
        server = DevServer(port=19876)
        server.start()
        assert server.running
        assert server.base_url == "http://127.0.0.1:19876"
        server.stop()
        assert not server.running

    def test_health_endpoint(self):
        server = DevServer(port=19877)
        server.start()
        try:
            resp = httpx.get(f"{server.base_url}/health")
            assert resp.status_code == 200
            data = resp.json()
            assert data["status"] == "healthy"
            assert data["mode"] == "dev"
        finally:
            server.stop()

    def test_put_and_get_features(self):
        server = DevServer(port=19878)
        server.start()
        try:
            # Put features
            resp = httpx.post(
                f"{server.base_url}/v1/features",
                json={"entity_key": "user:1", "features": {"clicks": 42}},
            )
            assert resp.status_code == 201

            # Get features
            resp = httpx.get(
                f"{server.base_url}/v1/features",
                params={"entity": "user:1", "feature": "clicks"},
            )
            assert resp.status_code == 200
            data = resp.json()
            entities = data["data"]["entities"]
            assert entities["user:1"]["features"]["clicks"]["value"] == 42
        finally:
            server.stop()

    def test_schema_groups(self):
        server = DevServer(port=19879)
        server.start()
        try:
            # Register group
            resp = httpx.post(
                f"{server.base_url}/v1/schema/groups",
                json={"name": "test_group", "entity_type": "user"},
            )
            assert resp.status_code == 201

            # List groups
            resp = httpx.get(f"{server.base_url}/v1/schema/groups")
            assert resp.status_code == 200
            data = resp.json()
            assert len(data["groups"]) == 1
        finally:
            server.stop()

    def test_discover_features(self):
        @feature(entity_type="user", description="Click count")
        def test_clicks(raw_count: int) -> int:
            return raw_count

        server = DevServer(port=19880)
        discovered = server.discover()
        assert "test_clicks" in discovered
        server.start()
        try:
            resp = httpx.get(f"{server.base_url}/v1/dev/features")
            assert resp.status_code == 200
            data = resp.json()
            assert any(f["name"] == "test_clicks" for f in data["features"])
        finally:
            server.stop()

    def test_dev_stats(self):
        server = DevServer(port=19881)
        server.start()
        try:
            httpx.post(
                f"{server.base_url}/v1/features",
                json={"entity_key": "user:1", "features": {"clicks": 10}},
            )
            resp = httpx.get(f"{server.base_url}/v1/dev/stats")
            assert resp.status_code == 200
            data = resp.json()
            assert data["stats"]["entities"] == 1
        finally:
            server.stop()

    def test_batch_features(self):
        server = DevServer(port=19882)
        server.start()
        try:
            for i in range(3):
                httpx.post(
                    f"{server.base_url}/v1/features",
                    json={"entity_key": f"user:{i}", "features": {"clicks": i * 10}},
                )
            resp = httpx.post(
                f"{server.base_url}/v1/features/batch",
                json={
                    "entities": ["user:0", "user:1", "user:2"],
                    "features": ["clicks"],
                },
            )
            assert resp.status_code == 200
            data = resp.json()
            assert len(data["data"]["entities"]) == 3
        finally:
            server.stop()

    def test_not_found(self):
        server = DevServer(port=19883)
        server.start()
        try:
            resp = httpx.get(f"{server.base_url}/v1/nonexistent")
            assert resp.status_code == 404
        finally:
            server.stop()
