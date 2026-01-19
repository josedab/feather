"""
Feather Feature Definition SDK

Decorator-based DSL for defining features as Python functions with automatic
dependency resolution, type inference, and schema generation.

Example usage:
    from feather.definitions import feature, window, FeatureStore

    store = FeatureStore("http://localhost:8080")

    @feature(entity="user", name="age_bucket")
    def user_age_bucket(age: float) -> str:
        if age < 18: return "minor"
        elif age < 35: return "young_adult"
        elif age < 55: return "middle_aged"
        return "senior"

    @feature(entity="user", name="purchase_total", freshness="1h")
    @window(duration="24h", aggregation="sum")
    def daily_purchase_total(amount: float) -> float:
        return amount

    # Generate schema
    schema = store.generate_schema()

    # Validate all features
    results = store.validate()

    # Export to YAML
    store.export_yaml("features.yaml")
"""

from __future__ import annotations

import functools
import inspect
import json
import re
import sys
import time
import typing
from dataclasses import dataclass, field
from datetime import datetime, timedelta
from enum import Enum
from typing import Any, Callable, Dict, List, Optional, Type, get_type_hints


# ---------------------------------------------------------------------------
# Data types
# ---------------------------------------------------------------------------


class DataType(Enum):
    """Supported Feather data types."""

    STRING = "string"
    INT = "int64"
    FLOAT = "float64"
    BOOL = "bool"
    TIMESTAMP = "timestamp"
    LIST = "list"
    MAP = "map"


# Mapping from Python types to Feather DataType.
_PYTHON_TYPE_MAP: Dict[type, DataType] = {
    str: DataType.STRING,
    int: DataType.INT,
    float: DataType.FLOAT,
    bool: DataType.BOOL,
    datetime: DataType.TIMESTAMP,
}


# ---------------------------------------------------------------------------
# Core dataclasses
# ---------------------------------------------------------------------------


@dataclass
class FeatureDefinition:
    """Metadata for a single feature defined via decorator."""

    name: str
    entity: str
    description: str = ""
    data_type: DataType = DataType.STRING
    freshness: Optional[str] = None
    owner: str = ""
    tags: List[str] = field(default_factory=list)
    dependencies: List[str] = field(default_factory=list)
    window_duration: Optional[str] = None
    window_aggregation: Optional[str] = None
    func: Optional[Callable] = None
    created_at: datetime = field(default_factory=datetime.utcnow)


@dataclass
class FeatureGroup:
    """A logical grouping of features for the same entity."""

    name: str
    entity: str
    features: List[FeatureDefinition] = field(default_factory=list)
    owner: str = ""
    description: str = ""


@dataclass
class ValidationResult:
    """Result of validating a single feature definition."""

    feature_name: str
    valid: bool
    errors: List[str] = field(default_factory=list)
    warnings: List[str] = field(default_factory=list)


@dataclass
class SchemaExport:
    """Exported schema containing all feature groups."""

    version: str = "1.0"
    groups: List[Dict] = field(default_factory=list)
    generated_at: str = ""


# ---------------------------------------------------------------------------
# Type inference
# ---------------------------------------------------------------------------


def _infer_type(func: Callable) -> DataType:
    """Infer Feather DataType from a function's return type annotation.

    Falls back to ``DataType.STRING`` when no annotation is present or the
    type is not recognised.
    """
    try:
        hints = get_type_hints(func)
    except Exception:
        return DataType.STRING

    return_type = hints.get("return")
    if return_type is None:
        return DataType.STRING

    # Check for bool before int (bool is a subclass of int in Python)
    if return_type is bool:
        return DataType.BOOL

    if return_type in _PYTHON_TYPE_MAP:
        return _PYTHON_TYPE_MAP[return_type]

    origin = getattr(return_type, "__origin__", None)
    if origin is list:
        return DataType.LIST
    if origin is dict:
        return DataType.MAP

    return DataType.STRING


# ---------------------------------------------------------------------------
# Freshness validation
# ---------------------------------------------------------------------------

_FRESHNESS_RE = re.compile(r"^(\d+)(s|m|h|d)$")


def _validate_freshness(value: str) -> bool:
    """Return ``True`` if *value* is a valid freshness string (e.g. ``'1h'``)."""
    return _FRESHNESS_RE.match(value) is not None


# ---------------------------------------------------------------------------
# Global registry
# ---------------------------------------------------------------------------

_registry: Dict[str, FeatureDefinition] = {}


def _clear_registry() -> None:
    """Clear the global registry (useful in tests)."""
    _registry.clear()


# ---------------------------------------------------------------------------
# Decorators
# ---------------------------------------------------------------------------


def feature(
    entity: str,
    name: Optional[str] = None,
    description: str = "",
    freshness: Optional[str] = None,
    owner: str = "",
    tags: Optional[List[str]] = None,
) -> Callable[[Callable[..., Any]], Callable[..., Any]]:
    """Decorator that marks a function as a Feather feature definition.

    The function's parameters become input dependencies and the return type
    is used to infer the Feather data type.

    Args:
        entity: Entity type this feature belongs to (e.g. ``"user"``).
        name: Feature name. Defaults to the function name.
        description: Human-readable description.
        freshness: Maximum staleness (e.g. ``"1h"``, ``"30m"``).
        owner: Owner team or person.
        tags: Optional list of tags.

    Returns:
        The original function with ``_feather_definition`` metadata attached.
    """

    def decorator(func: Callable[..., Any]) -> Callable[..., Any]:
        feat_name = name or func.__name__

        # Infer data type from return annotation
        data_type = _infer_type(func)

        # Extract dependencies from parameter names
        sig = inspect.signature(func)
        dependencies = list(sig.parameters.keys())

        defn = FeatureDefinition(
            name=feat_name,
            entity=entity,
            description=description or (func.__doc__ or "").strip(),
            data_type=data_type,
            freshness=freshness,
            owner=owner,
            tags=list(tags) if tags else [],
            dependencies=dependencies,
            func=func,
        )

        # Pick up window config set by @window applied before @feature
        win = getattr(func, "_feather_window", None)
        if win is not None:
            defn.window_duration = win["duration"]
            defn.window_aggregation = win["aggregation"]

        # Pick up explicit dependencies set by @depends_on applied before @feature
        explicit_deps = getattr(func, "_feather_depends", None)
        if explicit_deps is not None:
            defn.dependencies = explicit_deps

        @functools.wraps(func)
        def wrapper(*args: Any, **kwargs: Any) -> Any:
            return func(*args, **kwargs)

        wrapper._feather_definition = defn  # type: ignore[attr-defined]
        _registry[feat_name] = defn
        return wrapper

    return decorator


def window(
    duration: str,
    aggregation: str = "sum",
) -> Callable[[Callable[..., Any]], Callable[..., Any]]:
    """Decorator that adds sliding-window configuration to a feature.

    Must be applied **before** (i.e. below) the ``@feature`` decorator so
    that ``_feather_definition`` already exists on the function.

    Args:
        duration: Window size (e.g. ``"24h"``, ``"7d"``).
        aggregation: Aggregation function (``"sum"``, ``"avg"``, ``"count"``,
            ``"min"``, ``"max"``).
    """

    def decorator(func: Callable[..., Any]) -> Callable[..., Any]:
        defn: Optional[FeatureDefinition] = getattr(func, "_feather_definition", None)
        if defn is not None:
            defn.window_duration = duration
            defn.window_aggregation = aggregation
        else:
            # Store for later attachment when @feature is applied above
            func._feather_window = {"duration": duration, "aggregation": aggregation}  # type: ignore[attr-defined]
        return func

    return decorator


def depends_on(
    *feature_names: str,
) -> Callable[[Callable[..., Any]], Callable[..., Any]]:
    """Decorator that specifies explicit feature dependencies.

    Args:
        feature_names: Names of features this feature depends on.
    """

    def decorator(func: Callable[..., Any]) -> Callable[..., Any]:
        defn: Optional[FeatureDefinition] = getattr(func, "_feather_definition", None)
        if defn is not None:
            defn.dependencies = list(feature_names)
        else:
            func._feather_depends = list(feature_names)  # type: ignore[attr-defined]
        return func

    return decorator


# ---------------------------------------------------------------------------
# FeatureStore
# ---------------------------------------------------------------------------


class FeatureStore:
    """Central store that manages feature definitions, validation, and export.

    Example:
        >>> store = FeatureStore("http://localhost:8080")
        >>> store.auto_discover()
        >>> results = store.validate()
        >>> store.export_json("features.json")
    """

    def __init__(self, base_url: str = "http://localhost:8080") -> None:
        self._base_url = base_url.rstrip("/")
        self._features: Dict[str, FeatureDefinition] = {}

    # -- Registration -------------------------------------------------------

    def register(self, func_or_definition: Any) -> None:
        """Register a feature by decorated function or ``FeatureDefinition``.

        Args:
            func_or_definition: A ``@feature``-decorated callable **or** a
                ``FeatureDefinition`` instance.

        Raises:
            ValueError: If the argument is not a valid feature.
        """
        if isinstance(func_or_definition, FeatureDefinition):
            self._features[func_or_definition.name] = func_or_definition
            return

        defn: Optional[FeatureDefinition] = getattr(
            func_or_definition, "_feather_definition", None
        )
        if defn is not None:
            self._features[defn.name] = defn
            return

        raise ValueError(
            "Argument must be a @feature-decorated function or a FeatureDefinition"
        )

    # -- Query --------------------------------------------------------------

    def list_features(self) -> List[FeatureDefinition]:
        """Return all registered feature definitions."""
        return list(self._features.values())

    def get_feature(self, name: str) -> FeatureDefinition:
        """Return a single feature definition by name.

        Raises:
            KeyError: If the feature is not registered.
        """
        if name not in self._features:
            raise KeyError(f"Feature {name!r} not registered")
        return self._features[name]

    # -- Validation ---------------------------------------------------------

    def validate(self) -> List[ValidationResult]:
        """Validate all registered feature definitions.

        Checks performed:
        - Name uniqueness (enforced by dict storage)
        - Type annotations present on the function
        - Dependencies reference registered features
        - No circular dependencies
        - Freshness format valid (e.g. ``"1h"``, ``"30m"``)

        Returns:
            List of ``ValidationResult`` objects.
        """
        results: List[ValidationResult] = []

        for name, defn in self._features.items():
            errors: List[str] = []
            warnings: List[str] = []

            # Check type annotation
            if defn.func is not None:
                try:
                    hints = get_type_hints(defn.func)
                except Exception:
                    hints = {}
                if "return" not in hints:
                    warnings.append("Missing return type annotation")

            # Check dependencies exist
            for dep in defn.dependencies:
                if dep not in self._features:
                    warnings.append(f"Dependency '{dep}' is not a registered feature")

            # Check freshness format
            if defn.freshness is not None and not _validate_freshness(defn.freshness):
                errors.append(
                    f"Invalid freshness format '{defn.freshness}'; "
                    "expected pattern like '1h', '30m', '7d'"
                )

            results.append(
                ValidationResult(
                    feature_name=name,
                    valid=len(errors) == 0,
                    errors=errors,
                    warnings=warnings,
                )
            )

        # Check circular dependencies
        circular = self._find_cycles()
        for cycle_name in circular:
            for r in results:
                if r.feature_name == cycle_name:
                    r.errors.append("Circular dependency detected")
                    r.valid = False

        return results

    def _find_cycles(self) -> List[str]:
        """Detect features involved in circular dependencies."""
        visited: set[str] = set()
        in_stack: set[str] = set()
        cycle_nodes: List[str] = []

        def _dfs(name: str) -> bool:
            if name in in_stack:
                return True
            if name in visited:
                return False
            visited.add(name)
            in_stack.add(name)
            defn = self._features.get(name)
            if defn:
                for dep in defn.dependencies:
                    if dep in self._features and _dfs(dep):
                        cycle_nodes.append(name)
                        return True
            in_stack.discard(name)
            return False

        for feat_name in self._features:
            visited.clear()
            in_stack.clear()
            _dfs(feat_name)

        return cycle_nodes

    # -- Schema generation --------------------------------------------------

    def generate_schema(self) -> SchemaExport:
        """Generate a ``SchemaExport`` grouping features by entity.

        Returns:
            A ``SchemaExport`` with one group per entity type.
        """
        groups_map: Dict[str, List[FeatureDefinition]] = {}
        for defn in self._features.values():
            groups_map.setdefault(defn.entity, []).append(defn)

        groups: List[Dict] = []
        for entity, defns in groups_map.items():
            groups.append(
                {
                    "name": f"{entity}_features",
                    "entity": entity,
                    "features": [
                        {
                            "name": d.name,
                            "data_type": d.data_type.value,
                            "description": d.description,
                            "freshness": d.freshness,
                            "owner": d.owner,
                            "tags": d.tags,
                            "dependencies": d.dependencies,
                            "window_duration": d.window_duration,
                            "window_aggregation": d.window_aggregation,
                        }
                        for d in defns
                    ],
                }
            )

        return SchemaExport(
            version="1.0",
            groups=groups,
            generated_at=datetime.utcnow().isoformat(),
        )

    # -- Export -------------------------------------------------------------

    def export_yaml(self, path: str) -> None:
        """Export all feature definitions to a YAML file.

        Uses a simple dict-based serializer (no PyYAML dependency).

        Args:
            path: Destination file path.
        """
        schema = self.generate_schema()
        lines = [f"version: \"{schema.version}\"", f"generated_at: \"{schema.generated_at}\"", "groups:"]
        for group in schema.groups:
            lines.append(f"  - name: {group['name']}")
            lines.append(f"    entity: {group['entity']}")
            lines.append("    features:")
            for feat in group["features"]:
                lines.append(f"      - name: {feat['name']}")
                lines.append(f"        data_type: {feat['data_type']}")
                if feat["description"]:
                    lines.append(f"        description: \"{feat['description']}\"")
                if feat["freshness"]:
                    lines.append(f"        freshness: {feat['freshness']}")
                if feat["owner"]:
                    lines.append(f"        owner: {feat['owner']}")
                if feat["tags"]:
                    lines.append(f"        tags: [{', '.join(feat['tags'])}]")
                if feat["dependencies"]:
                    lines.append(f"        dependencies: [{', '.join(feat['dependencies'])}]")
                if feat["window_duration"]:
                    lines.append(f"        window_duration: {feat['window_duration']}")
                if feat["window_aggregation"]:
                    lines.append(f"        window_aggregation: {feat['window_aggregation']}")

        with open(path, "w") as f:
            f.write("\n".join(lines) + "\n")

    def export_json(self, path: str) -> None:
        """Export all feature definitions to a JSON file.

        Args:
            path: Destination file path.
        """
        schema = self.generate_schema()
        data = {
            "version": schema.version,
            "generated_at": schema.generated_at,
            "groups": schema.groups,
        }
        with open(path, "w") as f:
            json.dump(data, f, indent=2, default=str)

    # -- Dependency graph ---------------------------------------------------

    def dependency_graph(self) -> Dict[str, List[str]]:
        """Return the dependency graph as an adjacency list.

        Returns:
            Dict mapping each feature name to its list of dependencies.
        """
        return {
            name: list(defn.dependencies) for name, defn in self._features.items()
        }

    # -- Local compute ------------------------------------------------------

    def compute(self, name: str, **kwargs: Any) -> Any:
        """Execute a feature function locally with the given inputs.

        Args:
            name: Feature name.
            **kwargs: Input values passed to the feature function.

        Returns:
            Computed feature value.

        Raises:
            KeyError: If the feature is not registered.
            TypeError: If the feature has no callable function.
        """
        defn = self.get_feature(name)
        if defn.func is None:
            raise TypeError(f"Feature {name!r} has no callable function")
        return defn.func(**kwargs)

    # -- Auto-discovery -----------------------------------------------------

    def auto_discover(self) -> List[str]:
        """Find all ``@feature``-decorated functions in the global registry.

        Returns:
            List of newly discovered feature names.
        """
        discovered: List[str] = []
        for name, defn in _registry.items():
            if name not in self._features:
                self._features[name] = defn
                discovered.append(name)
        return discovered
