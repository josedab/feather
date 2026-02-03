"""Additional decorators and CLI support for the Feather Python SDK.

Provides @transformation and @aggregation decorators for defining
feature pipelines, plus a CLI entry point for 'feather apply'.

Usage:
    from feather_client.transforms import transformation, aggregation

    @transformation(
        entity_type="user",
        inputs=["raw_clicks", "raw_impressions"],
    )
    def click_through_rate(raw_clicks: int, raw_impressions: int) -> float:
        if raw_impressions == 0:
            return 0.0
        return raw_clicks / raw_impressions

    @aggregation(
        entity_type="user",
        source_feature="purchase_amount",
        function="sum",
        window="24h",
    )
    def daily_spend() -> float:
        ...

CLI:
    python -m feather_client.transforms apply --url http://localhost:8080
    python -m feather_client.transforms validate
    python -m feather_client.transforms export --format json
"""

from __future__ import annotations

import inspect
import json
import sys
from dataclasses import dataclass, field
from typing import Any, Callable, Optional, get_type_hints

from feather_client.declarative import (
    FeatureDefinition,
    FeatureRegistry,
    _GLOBAL_FEATURES,
    infer_feather_type,
)


# ---------------------------------------------------------------------------
# @transformation decorator
# ---------------------------------------------------------------------------

_GLOBAL_TRANSFORMS: dict[str, TransformDefinition] = {}


@dataclass
class TransformDefinition:
    """Metadata captured by the @transformation decorator."""

    name: str
    func: Callable[..., Any]
    entity_type: str
    description: str
    inputs: list[str]
    output_type: str = "float64"
    schedule: str | None = None
    tags: list[str] = field(default_factory=list)
    version: int = 1


def transformation(
    entity_type: str,
    inputs: list[str],
    description: str = "",
    *,
    schedule: str | None = None,
    tags: list[str] | None = None,
    version: int = 1,
) -> Callable[[Callable[..., Any]], Callable[..., Any]]:
    """Decorator that marks a function as a feature transformation.

    Transformations take one or more input features and produce
    a derived feature. They can run on a schedule or on-demand.

    Args:
        entity_type: Entity type (e.g. "user").
        inputs: List of input feature names.
        description: Human-readable description.
        schedule: Optional cron expression or interval (e.g. "1h").
        tags: Optional list of tags.
        version: Schema version.
    """

    def decorator(func: Callable[..., Any]) -> Callable[..., Any]:
        try:
            hints = get_type_hints(func)
        except Exception:
            hints = {}

        return_ann = hints.get("return")
        output_type = "float64"
        if return_ann is not None:
            try:
                output_type = infer_feather_type(return_ann)
            except TypeError:
                output_type = "string"

        defn = TransformDefinition(
            name=func.__name__,
            func=func,
            entity_type=entity_type,
            description=description,
            inputs=list(inputs),
            output_type=output_type,
            schedule=schedule,
            tags=list(tags) if tags else [],
            version=version,
        )
        func._feather_transform = defn  # type: ignore[attr-defined]
        _GLOBAL_TRANSFORMS[defn.name] = defn

        # Also register as a regular feature for auto-discovery
        feat_defn = FeatureDefinition(
            name=func.__name__,
            func=func,
            entity_type=entity_type,
            description=description or f"Transformation: {func.__name__}",
            output_type=output_type,
            tags=list(tags) if tags else [],
            version=version,
            inputs={inp: "float64" for inp in inputs},
            module=func.__module__,
        )
        _GLOBAL_FEATURES[feat_defn.name] = feat_defn

        return func

    return decorator


# ---------------------------------------------------------------------------
# @aggregation decorator
# ---------------------------------------------------------------------------

_GLOBAL_AGGREGATIONS: dict[str, AggregationDefinition] = {}


@dataclass
class AggregationDefinition:
    """Metadata captured by the @aggregation decorator."""

    name: str
    func: Callable[..., Any] | None
    entity_type: str
    description: str
    source_feature: str
    function: str  # sum, avg, count, min, max
    window: str  # e.g. "1h", "24h", "7d"
    slide_by: str | None = None
    output_type: str = "float64"
    tags: list[str] = field(default_factory=list)
    version: int = 1


def aggregation(
    entity_type: str,
    source_feature: str,
    function: str,
    window: str,
    description: str = "",
    *,
    slide_by: str | None = None,
    tags: list[str] | None = None,
    version: int = 1,
) -> Callable[[Callable[..., Any]], Callable[..., Any]]:
    """Decorator that marks a function as a feature aggregation.

    Aggregations compute windowed statistics over a source feature.

    Args:
        entity_type: Entity type (e.g. "user").
        source_feature: Name of the feature to aggregate.
        function: Aggregation function (sum, avg, count, min, max).
        window: Window duration (e.g. "1h", "24h").
        description: Human-readable description.
        slide_by: Slide interval for sliding windows.
        tags: Optional tags.
        version: Schema version.
    """

    def decorator(func: Callable[..., Any]) -> Callable[..., Any]:
        defn = AggregationDefinition(
            name=func.__name__,
            func=func,
            entity_type=entity_type,
            description=description or f"{function}({source_feature}) over {window}",
            source_feature=source_feature,
            function=function,
            window=window,
            slide_by=slide_by,
            tags=list(tags) if tags else [],
            version=version,
        )
        func._feather_aggregation = defn  # type: ignore[attr-defined]
        _GLOBAL_AGGREGATIONS[defn.name] = defn

        # Register as a feature
        feat_defn = FeatureDefinition(
            name=func.__name__,
            func=func,
            entity_type=entity_type,
            description=defn.description,
            output_type="float64",
            tags=list(tags) if tags else [],
            version=version,
            inputs={source_feature: "float64"},
            module=func.__module__,
        )
        _GLOBAL_FEATURES[feat_defn.name] = feat_defn

        return func

    return decorator


# ---------------------------------------------------------------------------
# CLI: feather apply / validate / export
# ---------------------------------------------------------------------------


def _cli_apply(args: list[str]) -> None:
    """Register all discovered features with a Feather server."""
    import argparse

    parser = argparse.ArgumentParser(description="Apply feature definitions to Feather")
    parser.add_argument("--url", default="http://localhost:8080", help="Feather server URL")
    parser.add_argument("--module", default=None, help="Python module to scan")
    parser.add_argument("--dry-run", action="store_true", help="Validate without applying")
    opts = parser.parse_args(args)

    registry = FeatureRegistry(base_url=opts.url)

    # Discover features
    if opts.module:
        import importlib
        mod = importlib.import_module(opts.module)
        discovered = registry.discover(module=mod)
    else:
        discovered = registry.discover()

    print(f"Discovered {len(discovered)} feature(s): {', '.join(discovered)}")

    # Validate
    errors = registry.validate()
    if errors:
        print("Validation errors:")
        for err in errors:
            print(f"  - {err}")
        sys.exit(1)

    print("Validation passed ✓")

    if opts.dry_run:
        print("Dry run mode — skipping registration")
        return

    # Apply
    groups = registry.register_all()
    print(f"Registered {len(groups)} feature group(s)")
    for g in groups:
        print(f"  - {g.name} ({len(g.features)} features)")


def _cli_validate(args: list[str]) -> None:
    """Validate feature definitions without applying."""
    _cli_apply(["--dry-run"] + args)


def _cli_export(args: list[str]) -> None:
    """Export feature schemas."""
    import argparse

    parser = argparse.ArgumentParser(description="Export feature schemas")
    parser.add_argument("--format", default="json", choices=["json", "yaml"])
    parser.add_argument("--module", default=None, help="Python module to scan")
    opts = parser.parse_args(args)

    registry = FeatureRegistry()
    if opts.module:
        import importlib
        mod = importlib.import_module(opts.module)
        registry.discover(module=mod)
    else:
        registry.discover()

    print(registry.export_schema(fmt=opts.format))


def main() -> None:
    """CLI entry point for 'python -m feather_client.transforms'."""
    if len(sys.argv) < 2:
        print("Usage: feather <command> [options]")
        print("Commands: apply, validate, export")
        sys.exit(1)

    command = sys.argv[1]
    remaining = sys.argv[2:]

    commands = {
        "apply": _cli_apply,
        "validate": _cli_validate,
        "export": _cli_export,
    }

    handler = commands.get(command)
    if handler is None:
        print(f"Unknown command: {command}")
        print(f"Available: {', '.join(commands)}")
        sys.exit(1)

    handler(remaining)


if __name__ == "__main__":
    main()
