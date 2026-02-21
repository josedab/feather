"""Feather Python Transform SDK.

Write feature transformations in Python that compile to WASM
and deploy to Feather's runtime.
"""

from __future__ import annotations

import functools
from typing import Any, Callable, Dict, Optional


class TransformRegistry:
    """Singleton registry that collects all decorated transform functions."""

    _instance: Optional[TransformRegistry] = None
    _transforms: Dict[str, Dict[str, Any]] = {}

    def __new__(cls) -> TransformRegistry:
        if cls._instance is None:
            cls._instance = super().__new__(cls)
            cls._transforms = {}
        return cls._instance

    def register(
        self,
        name: str,
        func: Callable,
        input_schema: Dict[str, str],
        output_schema: Dict[str, str],
    ) -> None:
        """Register a transform function with its schemas."""
        self._transforms[name] = {
            "func": func,
            "input_schema": input_schema,
            "output_schema": output_schema,
        }

    def get(self, name: str) -> Optional[Dict[str, Any]]:
        """Get a registered transform by name."""
        return self._transforms.get(name)

    def list_transforms(self) -> Dict[str, Dict[str, Any]]:
        """List all registered transforms (without func references)."""
        return {
            name: {
                "input_schema": info["input_schema"],
                "output_schema": info["output_schema"],
            }
            for name, info in self._transforms.items()
        }

    def clear(self) -> None:
        """Clear all registered transforms. Useful for testing."""
        self._transforms.clear()


def feather_transform(
    name: str,
    input_schema: Optional[Dict[str, str]] = None,
    output_schema: Optional[Dict[str, str]] = None,
) -> Callable:
    """Decorator to register a function as a Feather transform.

    Args:
        name: Unique name for this transform.
        input_schema: Mapping of input field names to their types.
        output_schema: Mapping of output field names to their types.

    Returns:
        The original function, registered in the TransformRegistry.

    Example::

        @feather_transform(
            name="normalize_score",
            input_schema={"raw_score": "float", "min_score": "float", "max_score": "float"},
            output_schema={"normalized_score": "float"},
        )
        def normalize_score(input_data):
            raw = input_data["raw_score"]
            min_val = input_data["min_score"]
            max_val = input_data["max_score"]
            if max_val == min_val:
                return {"normalized_score": 0.0}
            return {"normalized_score": (raw - min_val) / (max_val - min_val)}
    """

    def decorator(func: Callable) -> Callable:
        registry = TransformRegistry()
        registry.register(
            name=name,
            func=func,
            input_schema=input_schema or {},
            output_schema=output_schema or {},
        )

        @functools.wraps(func)
        def wrapper(*args: Any, **kwargs: Any) -> Any:
            return func(*args, **kwargs)

        return wrapper

    return decorator


def compile_to_wasm(name: str) -> bytes:
    """Compile a registered transform to WebAssembly.

    This is a stub — actual WASM compilation requires the
    ``feather-wasm-compiler`` toolchain which converts the Python
    function AST into a WASM module. In production this would:

    1. Retrieve the function source from the registry.
    2. Parse the AST and validate against the declared schemas.
    3. Lower to WASM IR via the compiler toolchain.
    4. Emit an optimised ``.wasm`` binary.

    Args:
        name: Name of a registered transform.

    Returns:
        WASM bytes (stub returns empty bytes).

    Raises:
        ValueError: If the transform name is not registered.
    """
    registry = TransformRegistry()
    transform = registry.get(name)
    if transform is None:
        raise ValueError(f"Transform '{name}' is not registered")

    # Stub: WASM compilation would happen here.
    return b""


def test_transform(func: Callable, input_data: Dict[str, Any]) -> Dict[str, Any]:
    """Run a transform locally with test data.

    Args:
        func: The decorated transform function.
        input_data: Dictionary matching the transform's input schema.

    Returns:
        The transform's output dictionary.

    Raises:
        Exception: Propagates any exception raised by the transform.
    """
    return func(input_data)


# ---------------------------------------------------------------------------
# Built-in example transform
# ---------------------------------------------------------------------------

@feather_transform(
    name="normalize_score",
    input_schema={"raw_score": "float", "min_score": "float", "max_score": "float"},
    output_schema={"normalized_score": "float"},
)
def normalize_score(input_data: Dict[str, Any]) -> Dict[str, Any]:
    """Normalize a value to the 0-1 range."""
    raw = input_data["raw_score"]
    min_val = input_data["min_score"]
    max_val = input_data["max_score"]
    if max_val == min_val:
        return {"normalized_score": 0.0}
    return {"normalized_score": (raw - min_val) / (max_val - min_val)}
