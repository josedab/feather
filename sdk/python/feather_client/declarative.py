"""Declarative feature definition SDK for Feather.

Provides Python decorators and classes for defining features as code,
with automatic schema registration and type inference.

Usage:
    from feather_client.declarative import feature, FeatureSet, FeatureRegistry

    @feature(entity_type="user", description="Total purchase amount")
    def total_purchases(click_count: int, purchase_total: float) -> float:
        return purchase_total

    @feature(entity_type="user", description="Is high value user")
    def is_high_value(purchase_total: float) -> bool:
        return purchase_total > 1000.0

    registry = FeatureRegistry(base_url="http://localhost:8080")
    registry.discover()  # Auto-discovers @feature functions
    registry.register_all()  # Registers schemas with Feather
"""

from __future__ import annotations

import inspect
import json
import sys
from dataclasses import dataclass, field
from typing import Any, Callable, Optional, get_args, get_origin, get_type_hints

import httpx
from pydantic import BaseModel

from feather_client.compute import ComputeEngine
from feather_client.models import FeatureGroup, FeatureSpec


# ---------------------------------------------------------------------------
# Type inference helpers
# ---------------------------------------------------------------------------

# Mapping from Python types to Feather data type strings.
_TYPE_MAP: dict[type, str] = {
    int: "int64",
    float: "float64",
    str: "string",
    bool: "bool",
    bytes: "bytes",
}


def infer_feather_type(py_type: Any) -> str:
    """Map a Python type annotation to a Feather data type string.

    Supports basic types and ``list[float]`` as vector.

    Args:
        py_type: A Python type or generic alias.

    Returns:
        Feather type string (e.g. ``"float64"``, ``"vector"``).

    Raises:
        TypeError: If the type cannot be mapped.
    """
    if py_type in _TYPE_MAP:
        return _TYPE_MAP[py_type]

    origin = get_origin(py_type)
    if origin is list:
        args = get_args(py_type)
        if args and args[0] is float:
            return "vector"
        return "string"  # fallback for other list types

    raise TypeError(f"Cannot map Python type {py_type!r} to a Feather type")


# ---------------------------------------------------------------------------
# @feature decorator
# ---------------------------------------------------------------------------

# Global set of all decorated feature functions.
_GLOBAL_FEATURES: dict[str, FeatureDefinition] = {}


@dataclass
class FeatureDefinition:
    """Metadata captured by the ``@feature`` decorator."""

    name: str
    func: Callable[..., Any]
    entity_type: str
    description: str
    tags: list[str] = field(default_factory=list)
    ttl: int | None = None
    version: int = 1
    inputs: dict[str, str] = field(default_factory=dict)
    output_type: str = "string"
    module: str = ""


def feature(
    entity_type: str,
    description: str = "",
    *,
    tags: list[str] | None = None,
    ttl: int | None = None,
    version: int = 1,
) -> Callable[[Callable[..., Any]], Callable[..., Any]]:
    """Decorator that marks a function as a Feather feature definition.

    The function's parameters become input features and the return type
    becomes the output feature type.

    Args:
        entity_type: Entity type this feature belongs to (e.g. ``"user"``).
        description: Human-readable description.
        tags: Optional list of tags.
        ttl: Time-to-live in seconds. ``None`` means no expiry.
        version: Schema version number.

    Returns:
        The original function, with ``_feather_feature`` metadata attached.
    """

    def decorator(func: Callable[..., Any]) -> Callable[..., Any]:
        sig = inspect.signature(func)
        try:
            hints = get_type_hints(func)
        except Exception:
            hints = {}

        # Resolve input types
        inputs: dict[str, str] = {}
        for param_name, param in sig.parameters.items():
            ann = hints.get(param_name)
            if ann is not None:
                try:
                    inputs[param_name] = infer_feather_type(ann)
                except TypeError:
                    inputs[param_name] = "string"
            else:
                inputs[param_name] = "string"

        # Resolve output type
        return_ann = hints.get("return")
        if return_ann is not None:
            try:
                output_type = infer_feather_type(return_ann)
            except TypeError:
                output_type = "string"
        else:
            output_type = "string"

        defn = FeatureDefinition(
            name=func.__name__,
            func=func,
            entity_type=entity_type,
            description=description,
            tags=list(tags) if tags else [],
            ttl=ttl,
            version=version,
            inputs=inputs,
            output_type=output_type,
            module=func.__module__,
        )
        # Attach metadata to the function itself.
        func._feather_feature = defn  # type: ignore[attr-defined]
        _GLOBAL_FEATURES[defn.name] = defn
        return func

    return decorator


# ---------------------------------------------------------------------------
# FeatureSet
# ---------------------------------------------------------------------------


class FeatureSet:
    """Groups related features with shared metadata.

    Example:
        >>> user_features = FeatureSet(
        ...     name="user_features",
        ...     entity_type="user",
        ...     tags=["core"],
        ... )
        >>>
        >>> @user_features.add(description="Login count")
        ... def login_count(raw_logins: int) -> int:
        ...     return raw_logins
    """

    def __init__(
        self,
        name: str,
        entity_type: str,
        *,
        description: str = "",
        tags: list[str] | None = None,
        ttl: int | None = None,
        version: int = 1,
    ) -> None:
        self.name = name
        self.entity_type = entity_type
        self.description = description
        self.tags = list(tags) if tags else []
        self.ttl = ttl
        self.version = version
        self.features: dict[str, FeatureDefinition] = {}

    def add(
        self,
        description: str = "",
        *,
        tags: list[str] | None = None,
        ttl: int | None = None,
        version: int | None = None,
    ) -> Callable[[Callable[..., Any]], Callable[..., Any]]:
        """Decorator to add a feature to this set.

        Inherits ``entity_type`` and default ``tags``/``ttl``/``version``
        from the set, but per-feature overrides are accepted.
        """
        merged_tags = list(self.tags) + (list(tags) if tags else [])
        effective_ttl = ttl if ttl is not None else self.ttl
        effective_version = version if version is not None else self.version

        def decorator(func: Callable[..., Any]) -> Callable[..., Any]:
            decorated = feature(
                entity_type=self.entity_type,
                description=description,
                tags=merged_tags,
                ttl=effective_ttl,
                version=effective_version,
            )(func)
            defn: FeatureDefinition = decorated._feather_feature  # type: ignore[attr-defined]
            self.features[defn.name] = defn
            return decorated

        return decorator

    def list_features(self) -> list[str]:
        """Return names of all features in this set."""
        return list(self.features.keys())


# ---------------------------------------------------------------------------
# FeatureView
# ---------------------------------------------------------------------------


class FeatureView:
    """Point-in-time feature view for querying stored features.

    Example:
        >>> view = FeatureView(base_url="http://localhost:8080")
        >>> result = view.query(
        ...     entity_keys=["user:1", "user:2"],
        ...     features=["purchase_count", "avg_order"],
        ... )
    """

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        *,
        timeout: float = 30.0,
        http_client: httpx.Client | None = None,
    ) -> None:
        self._base_url = base_url.rstrip("/")
        self._http = http_client or httpx.Client(timeout=timeout)
        self._last_result: dict[str, dict[str, Any]] = {}

    def query(
        self,
        entity_keys: list[str],
        features: list[str],
        as_of: str | None = None,
    ) -> dict[str, dict[str, Any]]:
        """Retrieve features for one or more entities.

        Args:
            entity_keys: List of entity keys (e.g. ``["user:1"]``).
            features: Feature names to retrieve.
            as_of: Optional ISO-8601 timestamp for point-in-time retrieval.

        Returns:
            Nested dict: ``{entity_key: {feature_name: value}}``.
        """
        result: dict[str, dict[str, Any]] = {}

        for entity_key in entity_keys:
            if as_of:
                params: dict[str, Any] = {
                    "entity": entity_key,
                    "feature": features,
                    "as_of": as_of,
                }
                resp = self._http.get(
                    f"{self._base_url}/v1/features/history", params=params
                )
            else:
                params = {"entity": entity_key, "feature": features}
                resp = self._http.get(
                    f"{self._base_url}/v1/features", params=params
                )

            if resp.status_code == 200:
                data = resp.json()
                entities = data.get("entities", {})
                entity_data = entities.get(entity_key, {})
                feat_map = entity_data.get("features", {})
                result[entity_key] = {
                    name: feat.get("value") for name, feat in feat_map.items()
                }
            else:
                result[entity_key] = {}

        self._last_result = result
        return result

    def to_dataframe(self) -> dict[str, dict[str, Any]]:
        """Convert the last query result to a dict suitable for ``pandas.DataFrame``.

        Returns:
            Column-oriented dict: ``{column: {entity_key: value}}``.
        """
        columns: dict[str, dict[str, Any]] = {}
        for entity_key, feats in self._last_result.items():
            for feat_name, val in feats.items():
                columns.setdefault(feat_name, {})[entity_key] = val
        return columns


# ---------------------------------------------------------------------------
# FeatureRegistry
# ---------------------------------------------------------------------------


class FeatureRegistry:
    """Central registry that discovers, validates, and registers features.

    Example:
        >>> registry = FeatureRegistry(base_url="http://localhost:8080")
        >>> registry.discover()
        >>> registry.validate()
        >>> registry.register_all()
    """

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        *,
        timeout: float = 30.0,
        http_client: httpx.Client | None = None,
    ) -> None:
        self._base_url = base_url.rstrip("/")
        self._http = http_client or httpx.Client(timeout=timeout)
        self._features: dict[str, FeatureDefinition] = {}
        self._engine = ComputeEngine()

    # -- Discovery ----------------------------------------------------------

    def discover(self, module: Any | None = None) -> list[str]:
        """Auto-discover ``@feature`` decorated functions.

        If *module* is ``None``, discovers from the caller's module and the
        global registry.

        Args:
            module: Python module object to scan. Defaults to caller's module.

        Returns:
            List of discovered feature names.
        """
        discovered: list[str] = []

        # Scan the explicit module (or caller's module).
        target_module = module
        if target_module is None:
            frame = inspect.stack()[1]
            caller_module_name = frame[0].f_globals.get("__name__", "")
            target_module = sys.modules.get(caller_module_name)

        if target_module is not None:
            for _attr_name in dir(target_module):
                obj = getattr(target_module, _attr_name, None)
                if callable(obj) and hasattr(obj, "_feather_feature"):
                    defn: FeatureDefinition = obj._feather_feature
                    self._features[defn.name] = defn
                    discovered.append(defn.name)

        # Also pull in anything from the global registry.
        for name, defn in _GLOBAL_FEATURES.items():
            if name not in self._features:
                self._features[name] = defn
                discovered.append(name)

        # Register functions in the compute engine.
        for name, defn in self._features.items():
            dep_names = [
                p for p in defn.inputs if p in self._features
            ]
            self._engine.register(name, defn.func, deps=dep_names or None)

        return discovered

    def add(self, defn: FeatureDefinition) -> None:
        """Manually add a feature definition."""
        self._features[defn.name] = defn
        dep_names = [p for p in defn.inputs if p in self._features]
        self._engine.register(defn.name, defn.func, deps=dep_names or None)

    # -- Validation ---------------------------------------------------------

    def validate(self) -> list[str]:
        """Validate all registered feature definitions.

        Returns:
            List of validation error messages (empty means valid).
        """
        errors: list[str] = []
        for name, defn in self._features.items():
            if not defn.entity_type:
                errors.append(f"{name}: missing entity_type")
            if not callable(defn.func):
                errors.append(f"{name}: func is not callable")
            if defn.version < 1:
                errors.append(f"{name}: version must be >= 1")
            if defn.ttl is not None and defn.ttl < 0:
                errors.append(f"{name}: ttl must be non-negative")
        return errors

    # -- Registration -------------------------------------------------------

    def register_all(self) -> list[FeatureGroup]:
        """Register all discovered features with the Feather server.

        Groups features by ``entity_type`` and creates one ``FeatureGroup``
        per entity type.

        Returns:
            List of created ``FeatureGroup`` objects.
        """
        groups_map: dict[str, list[FeatureDefinition]] = {}
        for defn in self._features.values():
            groups_map.setdefault(defn.entity_type, []).append(defn)

        created: list[FeatureGroup] = []
        for entity_type, defns in groups_map.items():
            specs = [
                FeatureSpec(
                    name=d.name,
                    data_type=d.output_type,
                )
                for d in defns
            ]
            # Determine group-level TTL (use minimum non-None TTL).
            ttls = [d.ttl for d in defns if d.ttl is not None]
            group_ttl = min(ttls) if ttls else None

            group = FeatureGroup(
                name=f"{entity_type}_features",
                entity_type=entity_type,
                description=f"Auto-registered features for {entity_type}",
                ttl=group_ttl,
                features=specs,
            )
            resp = self._http.post(
                f"{self._base_url}/v1/schema/groups",
                json=group.model_dump(exclude_none=True),
            )
            if resp.status_code < 400:
                created.append(FeatureGroup(**resp.json()))
            else:
                created.append(group)

        return created

    # -- Local compute ------------------------------------------------------

    def compute(self, name: str, inputs: dict[str, Any]) -> Any:
        """Compute a feature value locally.

        Args:
            name: Feature name.
            inputs: Input values.

        Returns:
            The computed feature value.
        """
        return self._engine.compute(name, inputs)

    def compute_and_store(
        self,
        name: str,
        entity_key: str,
        inputs: dict[str, Any],
        *,
        timestamp: int | None = None,
    ) -> Any:
        """Compute a feature locally and store it on the Feather server.

        Args:
            name: Feature name.
            entity_key: Entity key (e.g. ``"user:123"``).
            inputs: Input values.
            timestamp: Optional nanosecond timestamp.

        Returns:
            The computed value.
        """
        value = self.compute(name, inputs)
        payload: dict[str, Any] = {
            "entity_key": entity_key,
            "features": {name: value},
        }
        if timestamp is not None:
            payload["timestamp"] = timestamp
        self._http.post(f"{self._base_url}/v1/features", json=payload)
        return value

    # -- Listing / export ---------------------------------------------------

    def list(self) -> list[dict[str, Any]]:
        """List all registered features.

        Returns:
            List of feature summary dicts.
        """
        return [
            {
                "name": d.name,
                "entity_type": d.entity_type,
                "description": d.description,
                "output_type": d.output_type,
                "tags": d.tags,
                "version": d.version,
                "ttl": d.ttl,
                "inputs": d.inputs,
            }
            for d in self._features.values()
        ]

    def export_schema(self, fmt: str = "json") -> str:
        """Export all feature schemas as JSON or YAML.

        Args:
            fmt: ``"json"`` or ``"yaml"``.

        Returns:
            Serialised schema string.

        Raises:
            ValueError: If *fmt* is not ``"json"`` or ``"yaml"``.
        """
        data = {
            "features": self.list(),
        }

        if fmt == "json":
            return json.dumps(data, indent=2, default=str)
        elif fmt == "yaml":
            # stdlib-only YAML-ish output (no PyYAML dependency).
            lines = ["features:"]
            for feat in data["features"]:
                lines.append(f"  - name: {feat['name']}")
                lines.append(f"    entity_type: {feat['entity_type']}")
                lines.append(f"    description: \"{feat['description']}\"")
                lines.append(f"    output_type: {feat['output_type']}")
                lines.append(f"    version: {feat['version']}")
                if feat["ttl"] is not None:
                    lines.append(f"    ttl: {feat['ttl']}")
                if feat["tags"]:
                    lines.append(f"    tags: [{', '.join(feat['tags'])}]")
                if feat["inputs"]:
                    lines.append("    inputs:")
                    for inp_name, inp_type in feat["inputs"].items():
                        lines.append(f"      {inp_name}: {inp_type}")
            return "\n".join(lines)
        else:
            raise ValueError(f"Unsupported format: {fmt!r}. Use 'json' or 'yaml'.")


# ---------------------------------------------------------------------------
# @on_demand decorator
# ---------------------------------------------------------------------------


@dataclass
class OnDemandDefinition:
    """Metadata captured by the ``@on_demand`` decorator."""

    name: str
    func: Callable[..., Any]
    entity_type: str
    description: str
    source_features: list[str]
    output_type: str = "string"
    cache_ttl: int | None = None
    tags: list[str] = field(default_factory=list)


_GLOBAL_ON_DEMAND: dict[str, OnDemandDefinition] = {}


def on_demand(
    entity_type: str,
    source_features: list[str],
    description: str = "",
    *,
    cache_ttl: int | None = None,
    tags: list[str] | None = None,
) -> Callable[[Callable[..., Any]], Callable[..., Any]]:
    """Decorator that marks a function as an on-demand feature.

    On-demand features are computed at request time from existing stored
    features. The decorated function receives the source feature values as
    keyword arguments and returns a computed value.

    Args:
        entity_type: Entity type (e.g. ``"user"``).
        source_features: List of feature names to fetch before computation.
        description: Human-readable description.
        cache_ttl: Optional TTL in seconds for caching computed values.
        tags: Optional list of tags.

    Example:
        >>> @on_demand(
        ...     entity_type="user",
        ...     source_features=["purchase_count", "return_count"],
        ...     description="Return rate for a user",
        ... )
        ... def return_rate(purchase_count: int, return_count: int) -> float:
        ...     if purchase_count == 0:
        ...         return 0.0
        ...     return return_count / purchase_count
    """

    def decorator(func: Callable[..., Any]) -> Callable[..., Any]:
        try:
            hints = get_type_hints(func)
        except Exception:
            hints = {}

        return_ann = hints.get("return")
        if return_ann is not None:
            try:
                output_type = infer_feather_type(return_ann)
            except TypeError:
                output_type = "string"
        else:
            output_type = "string"

        defn = OnDemandDefinition(
            name=func.__name__,
            func=func,
            entity_type=entity_type,
            description=description,
            source_features=list(source_features),
            output_type=output_type,
            cache_ttl=cache_ttl,
            tags=list(tags) if tags else [],
        )
        func._feather_on_demand = defn  # type: ignore[attr-defined]
        _GLOBAL_ON_DEMAND[defn.name] = defn
        return func

    return decorator


# ---------------------------------------------------------------------------
# FeatureService
# ---------------------------------------------------------------------------


class FeatureService:
    """Serves on-demand features by fetching inputs and computing results.

    Example:
        >>> service = FeatureService(base_url="http://localhost:8080")
        >>> service.discover()
        >>> value = service.compute("return_rate", "user:123")
    """

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        *,
        timeout: float = 30.0,
        http_client: httpx.Client | None = None,
    ) -> None:
        self._base_url = base_url.rstrip("/")
        self._http = http_client or httpx.Client(timeout=timeout)
        self._on_demand: dict[str, OnDemandDefinition] = {}
        self._cache: dict[str, tuple[Any, float]] = {}

    def discover(self) -> list[str]:
        """Discover all ``@on_demand`` decorated functions."""
        discovered: list[str] = []
        for name, defn in _GLOBAL_ON_DEMAND.items():
            self._on_demand[name] = defn
            discovered.append(name)
        return discovered

    def add(self, defn: OnDemandDefinition) -> None:
        """Manually add an on-demand definition."""
        self._on_demand[defn.name] = defn

    def compute(self, name: str, entity_key: str) -> Any:
        """Compute an on-demand feature for the given entity.

        Fetches source features from the server, passes them to the
        decorated function, and returns the result.

        Args:
            name: On-demand feature name.
            entity_key: Entity key (e.g. ``"user:123"``).

        Returns:
            Computed feature value.

        Raises:
            KeyError: If the on-demand feature is not registered.
        """
        import time as _time

        defn = self._on_demand.get(name)
        if defn is None:
            raise KeyError(f"On-demand feature {name!r} not registered")

        # Check cache
        cache_key = f"{name}:{entity_key}"
        if defn.cache_ttl is not None and cache_key in self._cache:
            cached_val, cached_at = self._cache[cache_key]
            if _time.time() - cached_at < defn.cache_ttl:
                return cached_val

        # Fetch source features
        params: dict[str, Any] = {
            "entity": entity_key,
            "feature": defn.source_features,
        }
        resp = self._http.get(
            f"{self._base_url}/v1/features", params=params
        )

        inputs: dict[str, Any] = {}
        if resp.status_code == 200:
            data = resp.json()
            entities = data.get("entities", data.get("data", {}).get("entities", {}))
            entity_data = entities.get(entity_key, {})
            feat_map = entity_data.get("features", {})
            for feat_name, feat_data in feat_map.items():
                if isinstance(feat_data, dict):
                    inputs[feat_name] = feat_data.get("value")
                else:
                    inputs[feat_name] = feat_data

        # Compute
        value = defn.func(**inputs)

        # Cache result
        if defn.cache_ttl is not None:
            self._cache[cache_key] = (value, _time.time())

        return value

    def list(self) -> list[dict[str, Any]]:
        """List all registered on-demand features."""
        return [
            {
                "name": d.name,
                "entity_type": d.entity_type,
                "description": d.description,
                "source_features": d.source_features,
                "output_type": d.output_type,
                "cache_ttl": d.cache_ttl,
                "tags": d.tags,
            }
            for d in self._on_demand.values()
        ]
