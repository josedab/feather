"""Feather Python SDK - Feature transformation definitions and client."""

import json
import urllib.request
from dataclasses import dataclass, field
from typing import Any, Callable, Dict, List, Optional
import inspect


@dataclass
class FieldSchema:
    """Defines a single input or output field."""
    name: str
    dtype: str = "float64"
    required: bool = True
    default: Any = None


@dataclass
class TransformDef:
    """A feature transformation definition."""
    id: str
    name: str
    source_code: str
    entry_point: str = "transform"
    transform_type: str = "on_demand"
    inputs: List[FieldSchema] = field(default_factory=list)
    outputs: List[FieldSchema] = field(default_factory=list)
    dependencies: List[str] = field(default_factory=list)
    feature_group: Optional[str] = None
    tags: Dict[str, str] = field(default_factory=dict)

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "name": self.name,
            "source_code": self.source_code,
            "entry_point": self.entry_point,
            "type": self.transform_type,
            "inputs": [{"name": f.name, "dtype": f.dtype, "required": f.required} for f in self.inputs],
            "outputs": [{"name": f.name, "dtype": f.dtype} for f in self.outputs],
            "dependencies": self.dependencies,
            "feature_group": self.feature_group,
            "tags": self.tags,
        }


def on_demand(inputs: List[str], outputs: List[str], dtype: str = "float64"):
    """Decorator to mark a function as an on-demand feature transform."""
    def decorator(func: Callable) -> TransformDef:
        source = inspect.getsource(func)
        return TransformDef(
            id=func.__name__,
            name=func.__name__,
            source_code=source,
            entry_point=func.__name__,
            transform_type="on_demand",
            inputs=[FieldSchema(name=n, dtype=dtype) for n in inputs],
            outputs=[FieldSchema(name=n, dtype=dtype) for n in outputs],
        )
    return decorator


def batch_transform(inputs: List[str], outputs: List[str], dtype: str = "float64"):
    """Decorator to mark a function as a batch feature transform."""
    def decorator(func: Callable) -> TransformDef:
        source = inspect.getsource(func)
        return TransformDef(
            id=func.__name__,
            name=func.__name__,
            source_code=source,
            entry_point=func.__name__,
            transform_type="batch",
            inputs=[FieldSchema(name=n, dtype=dtype) for n in inputs],
            outputs=[FieldSchema(name=n, dtype=dtype) for n in outputs],
        )
    return decorator


class FeatherClient:
    """Client for interacting with Feather's Python Transform SDK API."""

    def __init__(self, base_url: str = "http://localhost:8080"):
        self.base_url = base_url.rstrip("/")

    def register(self, transform_def: TransformDef) -> dict:
        """Register a transform with Feather."""
        return self._post("/v1/transforms", transform_def.to_dict())

    def get(self, transform_id: str) -> dict:
        """Get a transform by ID."""
        return self._get(f"/v1/transforms/{transform_id}")

    def list(self) -> dict:
        """List all transforms."""
        return self._get("/v1/transforms")

    def execute(self, transform_id: str, inputs: dict) -> dict:
        """Execute a transform."""
        return self._post(f"/v1/transforms/{transform_id}/execute", inputs)

    def deploy(self, transform_id: str) -> dict:
        """Deploy a transform."""
        return self._post(f"/v1/transforms/{transform_id}/deploy", {})

    def validate(self, transform_def: TransformDef) -> dict:
        """Validate a transform definition."""
        return self._post("/v1/transforms/validate", transform_def.to_dict())

    def _get(self, path: str) -> dict:
        url = f"{self.base_url}{path}"
        req = urllib.request.Request(url, method="GET")
        with urllib.request.urlopen(req) as resp:
            return json.loads(resp.read())

    def _post(self, path: str, data: dict) -> dict:
        url = f"{self.base_url}{path}"
        body = json.dumps(data).encode("utf-8")
        req = urllib.request.Request(url, data=body, method="POST",
                                     headers={"Content-Type": "application/json"})
        with urllib.request.urlopen(req) as resp:
            return json.loads(resp.read())
