"""Jupyter notebook integration for Feather Feature Store.

Provides cell magics, display helpers, and interactive widgets
for exploring features directly in Jupyter notebooks.

Usage:
    In a Jupyter notebook cell:

        %load_ext feather_client.notebook

    Then use the magic:

        %%feather
        SELECT * FROM user_features WHERE entity = 'user:123'

    Or explore features:

        from feather_client.notebook import FeatureExplorer
        explorer = FeatureExplorer("http://localhost:8080")
        explorer.show_groups()
        explorer.show_features("user:123", ["click_count", "purchase_total"])
"""

from __future__ import annotations

import json
import time
from typing import Any, Optional

import httpx


class FeatureExplorer:
    """Interactive feature explorer for Jupyter notebooks.

    Provides methods for exploring feature groups, inspecting entity
    features, and comparing feature values across time windows.

    Args:
        base_url: Feather server URL.
        timeout: Request timeout in seconds.

    Example:
        >>> explorer = FeatureExplorer("http://localhost:8080")
        >>> explorer.show_groups()
        >>> explorer.show_features("user:123", ["clicks", "purchases"])
        >>> explorer.compare("user:123", "user:456", ["clicks"])
    """

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        *,
        timeout: float = 30.0,
    ) -> None:
        self._base_url = base_url.rstrip("/")
        self._http = httpx.Client(timeout=timeout)
        self._history: list[dict[str, Any]] = []

    def show_groups(self) -> list[dict[str, Any]]:
        """List all feature groups.

        Returns:
            List of feature group definitions.
        """
        resp = self._http.get(f"{self._base_url}/v1/schema/groups")
        if resp.status_code == 200:
            data = resp.json()
            groups = data.get("groups", data.get("data", []))
            if isinstance(groups, list):
                return groups
            return [groups] if groups else []
        return []

    def show_features(
        self,
        entity_key: str,
        features: list[str] | None = None,
    ) -> dict[str, Any]:
        """Retrieve and display features for an entity.

        Args:
            entity_key: Entity key (e.g. ``"user:123"``).
            features: Optional list of feature names to filter.

        Returns:
            Dict of feature name to value.
        """
        params: dict[str, Any] = {"entity": entity_key}
        if features:
            params["feature"] = features
        resp = self._http.get(f"{self._base_url}/v1/features", params=params)
        result: dict[str, Any] = {}
        if resp.status_code == 200:
            data = resp.json()
            entities = data.get("data", {}).get("entities", data.get("entities", {}))
            entity_data = entities.get(entity_key, {})
            feat_map = entity_data.get("features", {})
            for name, feat in feat_map.items():
                if isinstance(feat, dict):
                    result[name] = feat.get("value")
                else:
                    result[name] = feat
        self._history.append({
            "action": "show_features",
            "entity": entity_key,
            "result": result,
            "timestamp": time.time(),
        })
        return result

    def compare(
        self,
        entity_a: str,
        entity_b: str,
        features: list[str],
    ) -> dict[str, dict[str, Any]]:
        """Compare features between two entities.

        Args:
            entity_a: First entity key.
            entity_b: Second entity key.
            features: Feature names to compare.

        Returns:
            Dict with comparison data per feature.
        """
        vals_a = self.show_features(entity_a, features)
        vals_b = self.show_features(entity_b, features)
        comparison: dict[str, dict[str, Any]] = {}
        for name in features:
            va = vals_a.get(name)
            vb = vals_b.get(name)
            diff: dict[str, Any] = {
                entity_a: va,
                entity_b: vb,
                "match": va == vb,
            }
            if isinstance(va, (int, float)) and isinstance(vb, (int, float)):
                diff["difference"] = vb - va
                if va != 0:
                    diff["change_pct"] = ((vb - va) / abs(va)) * 100
            comparison[name] = diff
        return comparison

    def history_as_table(
        self,
        entity_key: str,
        features: list[str],
        as_of: str | None = None,
    ) -> dict[str, Any]:
        """Retrieve historical feature values.

        Args:
            entity_key: Entity key.
            features: Feature names.
            as_of: Optional ISO-8601 timestamp for point-in-time query.

        Returns:
            Historical feature data.
        """
        params: dict[str, Any] = {
            "entity": entity_key,
            "feature": features,
        }
        if as_of:
            params["as_of"] = as_of
        resp = self._http.get(
            f"{self._base_url}/v1/features/history", params=params
        )
        if resp.status_code == 200:
            return resp.json()
        return {}

    def health(self) -> dict[str, Any]:
        """Check server health.

        Returns:
            Health check response.
        """
        resp = self._http.get(f"{self._base_url}/health")
        if resp.status_code == 200:
            return resp.json()
        return {"status": "unreachable", "code": resp.status_code}

    def query_history(self) -> list[dict[str, Any]]:
        """Return the exploration history for this session.

        Returns:
            List of previous actions with timestamps.
        """
        return list(self._history)


# ---------------------------------------------------------------------------
# Cell magic for Jupyter / IPython
# ---------------------------------------------------------------------------


class FeatherMagic:
    """IPython magic for querying Feather from notebook cells.

    Registers ``%feather`` line magic and ``%%feather`` cell magic.

    Usage:
        %load_ext feather_client.notebook

        %feather health
        %feather groups

        %%feather
        entity=user:123 features=click_count,purchase_total
    """

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
    ) -> None:
        self._explorer = FeatureExplorer(base_url)

    def line_magic(self, line: str) -> Any:
        """Handle ``%feather <command>``."""
        parts = line.strip().split()
        if not parts:
            return {"help": "Commands: health, groups, stats"}

        cmd = parts[0].lower()

        if cmd == "health":
            return self._explorer.health()
        elif cmd == "groups":
            return self._explorer.show_groups()
        elif cmd == "features" and len(parts) >= 2:
            entity = parts[1]
            features = parts[2:] if len(parts) > 2 else None
            return self._explorer.show_features(entity, features)
        elif cmd == "compare" and len(parts) >= 4:
            entity_a = parts[1]
            entity_b = parts[2]
            features = parts[3:]
            return self._explorer.compare(entity_a, entity_b, features)
        elif cmd == "history":
            return self._explorer.query_history()
        else:
            return {"error": f"Unknown command: {cmd}"}

    def cell_magic(self, line: str, cell: str) -> Any:
        """Handle ``%%feather`` cell magic.

        Parses cell content as key=value pairs:
            entity=user:123
            features=click_count,purchase_total
            as_of=2024-01-15T00:00:00Z
        """
        params: dict[str, str] = {}

        # Parse the header line
        if line.strip():
            for part in line.strip().split():
                if "=" in part:
                    k, v = part.split("=", 1)
                    params[k.strip()] = v.strip()

        # Parse the cell body
        for raw_line in cell.strip().splitlines():
            raw_line = raw_line.strip()
            if not raw_line or raw_line.startswith("#"):
                continue
            if "=" in raw_line:
                k, v = raw_line.split("=", 1)
                params[k.strip()] = v.strip()

        entity = params.get("entity", "")
        features_str = params.get("features", "")
        features = [f.strip() for f in features_str.split(",") if f.strip()] if features_str else None
        as_of = params.get("as_of")

        if not entity:
            return {"error": "Missing 'entity' parameter"}

        if as_of:
            return self._explorer.history_as_table(entity, features or [], as_of)
        return self._explorer.show_features(entity, features)


# Module-level magic instance (created on %load_ext)
_magic_instance: Optional[FeatherMagic] = None


def load_ipython_extension(ipython: Any) -> None:
    """Register Feather magics when ``%load_ext feather_client.notebook`` is run.

    Args:
        ipython: The IPython shell instance.
    """
    import os

    global _magic_instance

    base_url = os.environ.get("FEATHER_URL", "http://localhost:8080")
    _magic_instance = FeatherMagic(base_url)

    ipython.register_magic_function(
        _magic_instance.line_magic,
        magic_kind="line",
        magic_name="feather",
    )
    ipython.register_magic_function(
        _magic_instance.cell_magic,
        magic_kind="cell",
        magic_name="feather",
    )


def unload_ipython_extension(ipython: Any) -> None:
    """Clean up when the extension is unloaded."""
    global _magic_instance
    _magic_instance = None
