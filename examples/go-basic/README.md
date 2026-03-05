# Go Basic Example

A minimal Go example demonstrating Feather feature store operations using only the standard library.

## Prerequisites

- Go 1.24+
- A running Feather server (`make run-dev`)

## Run

```bash
cd examples/go-basic && go run main.go
```

## What it does

1. Checks server health
2. Stores feature values for an entity
3. Retrieves features by entity ID
4. Performs batch feature retrieval
