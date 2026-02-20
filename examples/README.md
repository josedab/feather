# Feather Examples

Runnable examples demonstrating common Feather use cases.
All examples require a running Feather server — start one with `make run-dev`.

## Prerequisites

```bash
# From the repository root
make run-dev
```

## Examples

| Example | Language | Description |
|---------|----------|-------------|
| [ml-pipeline.py](./ml-pipeline.py) | Python | End-to-end ML feature pipeline: ingest, serve, batch, backtest, vector search |
| [fraud-detection.py](./fraud-detection.py) | Python | Real-time fraud detection: store transaction features, retrieve for scoring |
| [go-basic/](./go-basic/) | Go | Basic feature CRUD using the Go SDK |
| [ts-basic/](./ts-basic/) | TypeScript | Basic feature operations with the TypeScript SDK (Node.js 18+) |
| [rust-basic/](./rust-basic/) | Rust | Basic feature operations with the Rust client (Rust 1.75+) |

## Running

```bash
# Python examples (stdlib only — no pip install needed)
python examples/ml-pipeline.py
python examples/fraud-detection.py

# Go example
cd examples/go-basic && go run main.go

# TypeScript example (requires Node.js 18+)
cd examples/ts-basic && npm install && npx ts-node index.ts

# Rust example (requires Rust 1.75+)
cd examples/rust-basic && cargo run

# Run all examples
make examples
```

## Writing Your Own

Each example follows these conventions:
1. Check server health before doing anything
2. Self-contained — no imports beyond stdlib or the SDK
3. Clean up any created resources (vector indexes, etc.)
4. Print clear, labeled output
