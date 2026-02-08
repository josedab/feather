"""Feature computation utilities.

Provides local computation engine that mirrors server-side FCE behavior.
Supports dependency resolution via topological sort and incremental computation.
"""

from __future__ import annotations

import inspect
from collections import defaultdict
from dataclasses import dataclass, field
from typing import Any, Callable


@dataclass
class ComputePlan:
    """Execution plan showing the order features should be computed.

    Uses topological sort to resolve dependencies between features.

    Attributes:
        execution_order: Feature names in the order they should be computed.
        dependencies: Mapping of feature name to its direct dependencies.
    """

    execution_order: list[str]
    dependencies: dict[str, set[str]]

    def __str__(self) -> str:
        lines = ["ComputePlan:"]
        for i, name in enumerate(self.execution_order, 1):
            deps = self.dependencies.get(name, set())
            dep_str = f" (depends on: {', '.join(sorted(deps))})" if deps else ""
            lines.append(f"  {i}. {name}{dep_str}")
        return "\n".join(lines)


class ComputeEngine:
    """Local feature computation engine.

    Executes feature functions locally, resolving dependencies and caching
    intermediate results for incremental computation.

    Example:
        >>> engine = ComputeEngine()
        >>> engine.register("total", lambda price, qty: price * qty)
        >>> engine.register("discounted", lambda total: total * 0.9, deps=["total"])
        >>> result = engine.compute("discounted", {"price": 10.0, "qty": 3})
        >>> print(result)  # 27.0
    """

    def __init__(self) -> None:
        self._functions: dict[str, Callable[..., Any]] = {}
        self._dependencies: dict[str, set[str]] = defaultdict(set)
        self._cache: dict[str, Any] = {}
        self._cache_inputs: dict[str, dict[str, Any]] = {}

    def register(
        self,
        name: str,
        func: Callable[..., Any],
        deps: list[str] | None = None,
    ) -> None:
        """Register a feature function.

        Args:
            name: Feature name.
            func: Callable that computes the feature value.
            deps: Explicit dependency list. If None, inferred from function parameters.
        """
        self._functions[name] = func
        if deps is not None:
            self._dependencies[name] = set(deps)
        else:
            sig = inspect.signature(func)
            params = set(sig.parameters.keys())
            # Dependencies are parameters that are also registered features
            self._dependencies[name] = params

    def plan(self, target: str) -> ComputePlan:
        """Build a computation plan for a target feature.

        Args:
            target: The feature name to compute.

        Returns:
            ComputePlan with topological execution order.

        Raises:
            ValueError: If a dependency cycle is detected or feature is unknown.
        """
        if target not in self._functions:
            raise ValueError(f"Unknown feature: {target}")

        order: list[str] = []
        visited: set[str] = set()
        in_stack: set[str] = set()

        def _visit(name: str) -> None:
            if name in in_stack:
                raise ValueError(f"Dependency cycle detected involving: {name}")
            if name in visited:
                return
            in_stack.add(name)
            for dep in self._dependencies.get(name, set()):
                if dep in self._functions:
                    _visit(dep)
            in_stack.discard(name)
            visited.add(name)
            order.append(name)

        _visit(target)

        relevant_deps = {
            name: self._dependencies.get(name, set()) & set(self._functions.keys())
            for name in order
        }
        return ComputePlan(execution_order=order, dependencies=relevant_deps)

    def compute(self, name: str, inputs: dict[str, Any]) -> Any:
        """Compute a feature value, resolving dependencies.

        Args:
            name: Feature name to compute.
            inputs: Raw input values.

        Returns:
            The computed feature value.

        Raises:
            ValueError: If the feature is not registered.
        """
        if name not in self._functions:
            raise ValueError(f"Unknown feature: {name}")

        execution_plan = self.plan(name)
        computed: dict[str, Any] = dict(inputs)

        for feature_name in execution_plan.execution_order:
            func = self._functions[feature_name]
            sig = inspect.signature(func)
            kwargs: dict[str, Any] = {}
            for param_name in sig.parameters:
                if param_name in computed:
                    kwargs[param_name] = computed[param_name]
            result = func(**kwargs)
            computed[feature_name] = result

        return computed[name]

    def compute_incremental(self, name: str, inputs: dict[str, Any]) -> Any:
        """Compute a feature, reusing cached results when inputs haven't changed.

        Args:
            name: Feature name to compute.
            inputs: Raw input values.

        Returns:
            The computed feature value (possibly from cache).
        """
        if name not in self._functions:
            raise ValueError(f"Unknown feature: {name}")

        execution_plan = self.plan(name)
        computed: dict[str, Any] = dict(inputs)

        for feature_name in execution_plan.execution_order:
            func = self._functions[feature_name]
            sig = inspect.signature(func)
            kwargs: dict[str, Any] = {}
            for param_name in sig.parameters:
                if param_name in computed:
                    kwargs[param_name] = computed[param_name]

            # Check if cached result is still valid
            if feature_name in self._cache and feature_name in self._cache_inputs:
                if self._cache_inputs[feature_name] == kwargs:
                    computed[feature_name] = self._cache[feature_name]
                    continue

            result = func(**kwargs)
            computed[feature_name] = result
            self._cache[feature_name] = result
            self._cache_inputs[feature_name] = dict(kwargs)

        return computed[name]

    def invalidate(self, name: str | None = None) -> None:
        """Invalidate cached computation results.

        Args:
            name: Feature name to invalidate. If None, invalidates all.
        """
        if name is None:
            self._cache.clear()
            self._cache_inputs.clear()
        else:
            self._cache.pop(name, None)
            self._cache_inputs.pop(name, None)

    @property
    def registered_features(self) -> list[str]:
        """List all registered feature names."""
        return list(self._functions.keys())
