"""
Decorator-based feature definitions for type-safe feature registration.

Example usage:
    @feature_group("user_features", entity="user_id")
    class UserFeatures:
        age: int
        name: str
        score: float
        is_active: bool
"""

from __future__ import annotations

import datetime
from dataclasses import dataclass, field
from typing import Any, Optional, get_type_hints


@dataclass
class FeatureDefinition:
    """Metadata for a single feature defined via decorator."""

    name: str
    data_type: str
    description: str = ""
    default: Any = None
    tags: list[str] = field(default_factory=list)
    validation: Optional[dict[str, Any]] = None


@dataclass
class FeatureGroupDefinition:
    """A complete feature group created via @feature_group decorator."""

    name: str
    entity: str
    description: str = ""
    ttl_seconds: int = 0
    features: list[FeatureDefinition] = field(default_factory=list)
    tags: list[str] = field(default_factory=list)

    def to_dict(self) -> dict[str, Any]:
        """Serialize for Feather API registration."""
        result: dict[str, Any] = {
            "name": self.name,
            "entity_type": self.entity,
            "features": [
                {
                    "name": f.name,
                    "data_type": f.data_type,
                }
                for f in self.features
            ],
        }
        if self.description:
            result["description"] = self.description
        if self.ttl_seconds > 0:
            result["ttl"] = self.ttl_seconds
        return result

    def feature_names(self) -> list[str]:
        """Return list of feature names."""
        return [f.name for f in self.features]


# Registry of all defined feature groups
_registry: dict[str, FeatureGroupDefinition] = {}


def feature_group(
    name: str,
    entity: str,
    description: str = "",
    ttl_seconds: int = 0,
    tags: Optional[list[str]] = None,
):
    """Class decorator to define a feature group from type annotations.

    Inspects class annotations to extract feature definitions.
    Maps Python types: int->int64, float->float64, str->string, bool->bool,
    datetime.datetime->timestamp, list[float]->vector.
    """

    def decorator(cls: type) -> type:
        hints = get_type_hints(cls)
        feature_defs: list[FeatureDefinition] = []

        for attr_name, attr_type in hints.items():
            if attr_name.startswith("_"):
                continue
            feather_type = _python_type_to_feather(attr_type)

            # Check for a default value on the class
            default_val = getattr(cls, attr_name, None)

            # Check for a docstring-style description via __doc__ or class var
            feat_description = ""

            feature_defs.append(
                FeatureDefinition(
                    name=attr_name,
                    data_type=feather_type,
                    description=feat_description,
                    default=default_val,
                )
            )

        group_def = FeatureGroupDefinition(
            name=name,
            entity=entity,
            description=description,
            ttl_seconds=ttl_seconds,
            features=feature_defs,
            tags=list(tags) if tags else [],
        )

        _registry[name] = group_def

        # Attach the definition to the class for introspection
        cls.__feather_group__ = group_def  # type: ignore[attr-defined]
        return cls

    return decorator


def get_registry() -> dict[str, FeatureGroupDefinition]:
    """Return all registered feature groups."""
    return dict(_registry)


def clear_registry() -> None:
    """Clear the registry (useful in tests)."""
    _registry.clear()


def _python_type_to_feather(python_type: Any) -> str:
    """Map Python type annotation to Feather data type."""
    # Handle basic types directly
    if python_type is int:
        return "int64"
    if python_type is float:
        return "float64"
    if python_type is str:
        return "string"
    if python_type is bool:
        return "bool"
    if python_type is datetime.datetime:
        return "timestamp"

    # Handle generic types like list[float]
    origin = getattr(python_type, "__origin__", None)
    if origin is list:
        args = getattr(python_type, "__args__", ())
        if args and args[0] is float:
            return "vector"
        return "string"

    # Fallback
    return "string"
