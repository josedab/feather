# Feather Python Transform SDK

Write feature transformations in Python that compile to WebAssembly (WASM) and deploy to Feather's runtime. Define your feature logic in familiar Python, test it locally, then compile and deploy to run at sub-millisecond latency inside the Feather feature store.

## Prerequisites

- **Python 3.10+**
- **pip** (package manager)
- Feather server running locally or accessible remotely

## Installation

```bash
pip install feather-transforms
```

## Usage

Define a transform using the `@feather_transform` decorator:

```python
from feather_transforms import feather_transform, TransformRegistry, test_transform

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
```

### List Registered Transforms

```python
registry = TransformRegistry()
for name, info in registry.list_transforms().items():
    print(f"{name}: {info['input_schema']} -> {info['output_schema']}")
```

## Testing Locally

Use the `test_transform` helper to validate your transform before deploying:

```python
from feather_transforms import test_transform

result = test_transform(normalize_score, {
    "raw_score": 75.0,
    "min_score": 0.0,
    "max_score": 100.0,
})
print(result)  # {"normalized_score": 0.75}
```

Run the test suite:

```bash
python -m pytest test_transforms.py
```

## Compiling to WASM

Compile your transforms to WebAssembly for deployment:

```python
from feather_transforms import compile_to_wasm

wasm_bytes = compile_to_wasm("normalize_score")
# Writes normalize_score.wasm to the current directory
```

> **Note:** WASM compilation requires the `feather-wasm-compiler` toolchain. See the [Feather docs](https://feather.dev/docs/wasm) for installation instructions.

## Deploying

Deploy compiled WASM transforms to your Feather instance:

```bash
curl -X POST http://localhost:8080/v1/transforms/deploy \
  -F "wasm=@normalize_score.wasm" \
  -F "name=normalize_score"
```

Or use the Feather CLI:

```bash
feather transform deploy normalize_score.wasm --name normalize_score
```

## API Reference

| Symbol | Description |
|--------|-------------|
| `@feather_transform(name, input_schema, output_schema)` | Decorator to register a transform function |
| `TransformRegistry` | Singleton registry of all decorated transforms |
| `compile_to_wasm(name)` | Compile a registered transform to WASM |
| `test_transform(func, input_data)` | Run a transform locally with test data |
