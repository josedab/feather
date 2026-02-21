# Feather TypeScript Transform SDK

Write feature transformations in TypeScript that compile to WebAssembly (WASM) and deploy to Feather's runtime. Author type-safe feature logic with full IDE support, then compile and deploy for sub-millisecond execution inside the Feather feature store.

## Prerequisites

- **Node.js 18+**
- **npm** or **yarn**
- Feather server running locally or accessible remotely

## Installation

```bash
npm install @feather/transforms
```

## Usage

### Define a Transform

```typescript
import { registerTransform, listTransforms, testTransform } from "@feather/transforms";

registerTransform({
  name: "normalizeScore",
  inputSchema: { raw_score: "float", min_score: "float", max_score: "float" },
  outputSchema: { normalized_score: "float" },
  transform: (input) => {
    const raw = input.raw_score as number;
    const min = input.min_score as number;
    const max = input.max_score as number;
    if (max === min) return { normalized_score: 0.0 };
    return { normalized_score: (raw - min) / (max - min) };
  },
});
```

### List Registered Transforms

```typescript
const transforms = listTransforms();
console.log(transforms);
// [{ name: "normalizeScore", inputSchema: {...}, outputSchema: {...} }]
```

### Test Locally

```typescript
const result = testTransform("normalizeScore", {
  raw_score: 75,
  min_score: 0,
  max_score: 100,
});
console.log(result); // { normalized_score: 0.75 }
```

## Compiling to WASM

```bash
npx feather-compile --transform normalizeScore --out normalizeScore.wasm
```

## Deploying

```bash
curl -X POST http://localhost:8080/v1/transforms/deploy \
  -F "wasm=@normalizeScore.wasm" \
  -F "name=normalizeScore"
```

## API Reference

| Export | Description |
|--------|-------------|
| `FeatherTransform` | Interface for defining a transform |
| `registerTransform(t)` | Register a transform in the global registry |
| `listTransforms()` | List all registered transforms |
| `testTransform(name, input)` | Run a transform locally with test data |
