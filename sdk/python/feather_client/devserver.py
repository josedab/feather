"""Local development server for Feather features.

Provides an in-memory feature store that starts from Python for local
development and testing. Features defined with ``@feature`` and ``@on_demand``
decorators are automatically served.

Usage:
    from feather_client.devserver import DevServer

    server = DevServer(port=8080)
    server.discover()   # auto-discover @feature/@on_demand functions
    server.start()      # starts HTTP server in background thread

    # Now use FeatherClient pointed at localhost:8080
    from feather_client import FeatherClient
    client = FeatherClient("http://localhost:8080")
    client.put_features("user:123", {"click_count": 10})
    features = client.get_features("user:123", ["click_count"])

    server.stop()
"""

from __future__ import annotations

import json
import threading
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from typing import Any, Optional
from urllib.parse import urlparse, parse_qs

from feather_client.declarative import (
    _GLOBAL_FEATURES,
    _GLOBAL_ON_DEMAND,
    FeatureDefinition,
    OnDemandDefinition,
)


class InMemoryStore:
    """Simple in-memory feature store for development."""

    def __init__(self) -> None:
        self._data: dict[str, dict[str, Any]] = {}  # entity -> {feature: value}
        self._timestamps: dict[str, dict[str, int]] = {}  # entity -> {feature: ts}
        self._groups: dict[str, dict[str, Any]] = {}  # group_name -> schema
        self._lock = threading.Lock()

    def put(
        self,
        entity_key: str,
        features: dict[str, Any],
        timestamp: int | None = None,
    ) -> None:
        ts = timestamp or int(time.time() * 1e9)
        with self._lock:
            if entity_key not in self._data:
                self._data[entity_key] = {}
                self._timestamps[entity_key] = {}
            for name, value in features.items():
                self._data[entity_key][name] = value
                self._timestamps[entity_key][name] = ts

    def get(
        self,
        entity_key: str,
        feature_names: list[str] | None = None,
    ) -> dict[str, dict[str, Any]]:
        with self._lock:
            entity_data = self._data.get(entity_key, {})
            entity_ts = self._timestamps.get(entity_key, {})
            if feature_names:
                return {
                    name: {
                        "value": entity_data.get(name),
                        "timestamp": entity_ts.get(name, 0),
                    }
                    for name in feature_names
                    if name in entity_data
                }
            return {
                name: {"value": val, "timestamp": entity_ts.get(name, 0)}
                for name, val in entity_data.items()
            }

    def register_group(self, group: dict[str, Any]) -> None:
        with self._lock:
            self._groups[group.get("name", "")] = group

    def list_groups(self) -> list[dict[str, Any]]:
        with self._lock:
            return list(self._groups.values())

    def stats(self) -> dict[str, int]:
        with self._lock:
            return {
                "entities": len(self._data),
                "features": sum(len(v) for v in self._data.values()),
                "groups": len(self._groups),
            }


class _DevHandler(BaseHTTPRequestHandler):
    """HTTP handler for the dev server."""

    store: InMemoryStore
    features: dict[str, FeatureDefinition]
    on_demand: dict[str, OnDemandDefinition]

    def log_message(self, format: str, *args: Any) -> None:
        pass  # suppress request logging

    def _send_json(self, status: int, data: Any) -> None:
        body = json.dumps(data, default=str).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _read_body(self) -> dict[str, Any]:
        length = int(self.headers.get("Content-Length", 0))
        if length == 0:
            return {}
        raw = self.rfile.read(length)
        return json.loads(raw)

    def do_GET(self) -> None:
        parsed = urlparse(self.path)
        path = parsed.path.rstrip("/")
        params = parse_qs(parsed.query)

        if path == "/health":
            self._send_json(200, {
                "status": "healthy",
                "mode": "dev",
                "components": {"store": "healthy"},
            })
        elif path == "/v1/features":
            entity = params.get("entity", [None])[0]
            features = params.get("feature", [])
            if not entity:
                self._send_json(400, {"error": "entity parameter required"})
                return
            feat_data = self.store.get(entity, features or None)
            self._send_json(200, {
                "success": True,
                "data": {"entities": {entity: {"features": feat_data}}},
            })
        elif path == "/v1/features/history":
            entity = params.get("entity", [None])[0]
            features = params.get("feature", [])
            if not entity:
                self._send_json(400, {"error": "entity parameter required"})
                return
            feat_data = self.store.get(entity, features or None)
            self._send_json(200, {
                "success": True,
                "data": {"entities": {entity: {"features": feat_data}}},
            })
        elif path == "/v1/schema/groups":
            self._send_json(200, {
                "success": True,
                "groups": self.store.list_groups(),
            })
        elif path == "/v1/dev/stats":
            stats = self.store.stats()
            stats["registered_features"] = len(self.features)
            stats["on_demand_features"] = len(self.on_demand)
            self._send_json(200, {"success": True, "stats": stats})
        elif path == "/v1/dev/features":
            result = []
            for defn in self.features.values():
                result.append({
                    "name": defn.name,
                    "entity_type": defn.entity_type,
                    "description": defn.description,
                    "output_type": defn.output_type,
                    "inputs": defn.inputs,
                })
            self._send_json(200, {"success": True, "features": result})
        else:
            self._send_json(404, {"error": f"not found: {path}"})

    def do_POST(self) -> None:
        parsed = urlparse(self.path)
        path = parsed.path.rstrip("/")

        if path == "/v1/features":
            body = self._read_body()
            entity_key = body.get("entity_key", "")
            features = body.get("features", {})
            if not entity_key:
                self._send_json(400, {"error": "entity_key required"})
                return
            self.store.put(entity_key, features, body.get("timestamp"))
            self._send_json(201, {"success": True, "message": "features stored"})
        elif path == "/v1/features/batch":
            body = self._read_body()
            entities = body.get("entities", [])
            features = body.get("features", [])
            result: dict[str, Any] = {}
            for entity in entities:
                feat_data = self.store.get(entity, features or None)
                result[entity] = {"features": feat_data}
            self._send_json(200, {
                "success": True,
                "data": {"entities": result},
            })
        elif path == "/v1/schema/groups":
            body = self._read_body()
            self.store.register_group(body)
            self._send_json(201, body)
        elif path == "/v1/dev/compute":
            body = self._read_body()
            name = body.get("name", "")
            entity_key = body.get("entity_key", "")
            inputs = body.get("inputs", {})
            defn = self.features.get(name)
            if defn is None:
                self._send_json(404, {"error": f"feature {name!r} not found"})
                return
            try:
                value = defn.func(**inputs)
                if entity_key:
                    self.store.put(entity_key, {name: value})
                self._send_json(200, {
                    "success": True,
                    "feature": name,
                    "value": value,
                })
            except Exception as exc:
                self._send_json(500, {"error": str(exc)})
        else:
            self._send_json(404, {"error": f"not found: {path}"})


class DevServer:
    """Local development server for Feather features.

    Starts an in-memory HTTP server that mimics the Feather API,
    enabling local development without a real Feather deployment.

    Args:
        host: Bind address (default ``"127.0.0.1"``).
        port: Port number (default ``8080``).

    Example:
        >>> server = DevServer(port=9090)
        >>> server.discover()
        >>> server.start()
        >>> # ... use FeatherClient("http://localhost:9090") ...
        >>> server.stop()
    """

    def __init__(self, host: str = "127.0.0.1", port: int = 8080) -> None:
        self.host = host
        self.port = port
        self.store = InMemoryStore()
        self.features: dict[str, FeatureDefinition] = {}
        self.on_demand: dict[str, OnDemandDefinition] = {}
        self._server: Optional[HTTPServer] = None
        self._thread: Optional[threading.Thread] = None

    def discover(self) -> list[str]:
        """Discover ``@feature`` and ``@on_demand`` functions.

        Returns:
            List of discovered feature names.
        """
        discovered: list[str] = []
        for name, defn in _GLOBAL_FEATURES.items():
            self.features[name] = defn
            discovered.append(name)
        for name, defn in _GLOBAL_ON_DEMAND.items():
            self.on_demand[name] = defn
            discovered.append(f"on_demand:{name}")
        return discovered

    def start(self) -> None:
        """Start the dev server in a background thread."""
        handler_class = type(
            "_BoundDevHandler",
            (_DevHandler,),
            {
                "store": self.store,
                "features": self.features,
                "on_demand": self.on_demand,
            },
        )
        self._server = HTTPServer((self.host, self.port), handler_class)
        self._thread = threading.Thread(
            target=self._server.serve_forever,
            daemon=True,
        )
        self._thread.start()

    def stop(self) -> None:
        """Stop the dev server."""
        if self._server is not None:
            self._server.shutdown()
            self._server = None
        if self._thread is not None:
            self._thread.join(timeout=5)
            self._thread = None

    @property
    def base_url(self) -> str:
        """Return the base URL for the running server."""
        return f"http://{self.host}:{self.port}"

    @property
    def running(self) -> bool:
        """Check if the server is running."""
        return self._thread is not None and self._thread.is_alive()
