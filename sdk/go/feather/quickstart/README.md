# Feather Go Quickstart

Get started with Feather in 30 seconds.

## Prerequisites

- Go 1.21+
- Docker (for running Feather server)

## Step 1: Start Feather

```bash
# From source (no Docker needed)
cd /path/to/feather && make run-dev

# Or with Docker
docker run -d --name feather -p 8080:8080 ghcr.io/feather-store/feather:latest
```

## Step 2: Run the Quickstart

```bash
go run main.go
```

## What This Does

1. Connects to Feather
2. Stores features for a user entity
3. Retrieves the features back
4. Demonstrates point-in-time queries

## Next Steps

- Check out the [full documentation](https://feather-store.dev/docs)
- Explore [vector similarity search](https://feather-store.dev/docs/vectors)
- Learn about [real-time aggregations](https://feather-store.dev/docs/aggregations)
