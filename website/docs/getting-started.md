---
sidebar_position: 2
title: Getting Started
description: Get Feather running and serve your first feature in under 5 minutes.
---

# Getting Started

This guide will have you serving ML features in under 5 minutes. No Docker required, no external databases, just a single binary.

## Prerequisites

- Linux, macOS, or Windows with WSL2
- curl (for installation)
- Optional: Go 1.24+ (for building from source)

## Quick Install

### Option 1: Download Binary

```bash
# Linux (amd64)
curl -sSL https://github.com/feather-store/feather/releases/latest/download/feather-linux-amd64 -o feather

# Linux (arm64)
curl -sSL https://github.com/feather-store/feather/releases/latest/download/feather-linux-arm64 -o feather

# macOS (Apple Silicon)
curl -sSL https://github.com/feather-store/feather/releases/latest/download/feather-darwin-arm64 -o feather

# macOS (Intel)
curl -sSL https://github.com/feather-store/feather/releases/latest/download/feather-darwin-amd64 -o feather

# Make executable
chmod +x feather
```

### Option 2: Docker

```bash
docker run -d \
  --name feather \
  -p 8080:8080 \
  -p 50051:50051 \
  -p 9090:9090 \
  -v feather-data:/var/lib/feather/data \
  ghcr.io/feather-store/feather:latest
```

### Option 3: Build from Source

```bash
git clone https://github.com/feather-store/feather.git
cd feather
make build
./bin/feather
```

## Start Feather

Run Feather with default settings:

```bash
./feather
```

You should see:

```
2024/01/15 10:30:00 INFO Starting Feather Feature Store version=1.0.0
2024/01/15 10:30:00 INFO HTTP server listening addr=:8080
2024/01/15 10:30:00 INFO gRPC server listening addr=:50051
2024/01/15 10:30:00 INFO Metrics server listening addr=:9090
2024/01/15 10:30:00 INFO Feather is ready
```

## Verify Installation

Check that Feather is running:

```bash
curl http://localhost:8080/health
```

Expected response:

```json
{
  "status": "healthy",
  "components": {
    "hot_tier": "healthy",
    "warm_tier": "healthy"
  }
}
```

## Store Your First Feature

Let's store a user's click count:

```bash
curl -X POST http://localhost:8080/v1/features \
  -H "Content-Type: application/json" \
  -d '{
    "entity_key": "user:123",
    "features": {
      "click_count": 42,
      "last_purchase": 149.99,
      "is_premium": true
    }
  }'
```

Response:

```json
{
  "success": true,
  "request_id": "req-a1b2c3d4"
}
```

## Retrieve Features

Get the features you just stored:

```bash
curl "http://localhost:8080/v1/features?entity=user:123&feature=click_count&feature=last_purchase"
```

Response:

```json
{
  "success": true,
  "data": {
    "entities": {
      "user:123": {
        "features": {
          "click_count": {
            "value": 42,
            "timestamp": 1705315800000000000
          },
          "last_purchase": {
            "value": 149.99,
            "timestamp": 1705315800000000000
          }
        }
      }
    }
  },
  "request_id": "req-e5f6g7h8"
}
```

## Batch Retrieval

Retrieve features for multiple entities at once:

```bash
curl -X POST http://localhost:8080/v1/features/batch \
  -H "Content-Type: application/json" \
  -d '{
    "entities": ["user:123", "user:456", "user:789"],
    "features": ["click_count", "last_purchase"]
  }'
```

## Point-in-Time Query

Retrieve features as they existed at a specific time (essential for training data):

```bash
# First, update the feature
curl -X POST http://localhost:8080/v1/features \
  -H "Content-Type: application/json" \
  -d '{
    "entity_key": "user:123",
    "features": {"click_count": 100}
  }'

# Now query the historical value
curl "http://localhost:8080/v1/features/history?entity=user:123&feature=click_count&as_of=2024-01-15T10:30:00Z"
```

The response returns the value that existed at that timestamp, not the current value.

## Using the Python SDK

Install the SDK:

```bash
pip install feather-client
```

Use it in your code:

```python
from feather import FeatherClient

# Connect to Feather
client = FeatherClient("localhost:8080")

# Store features
client.put_features("user:123", {
    "click_count": 42,
    "last_purchase": 149.99,
    "is_premium": True
})

# Retrieve features
features = client.get_features("user:123", ["click_count", "last_purchase"])
print(f"Click count: {features['click_count']}")

# Point-in-time retrieval
historical = client.get_features_as_of(
    "user:123",
    ["click_count"],
    as_of="2024-01-15T10:30:00Z"
)
```

## Using the Go SDK

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/feather-store/feather/sdk/go/feather"
)

func main() {
    client, err := feather.NewClient("localhost:8080")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Store features
    err = client.PutFeatures(ctx, "user:123", map[string]interface{}{
        "click_count":   42,
        "last_purchase": 149.99,
        "is_premium":    true,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Retrieve features
    features, err := client.GetFeatures(ctx, "user:123", []string{"click_count", "last_purchase"})
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Click count: %v\n", features["click_count"].Value)
}
```

## Configuration

Create a config file for more control:

```yaml title="feather.yaml"
server:
  http:
    port: 8080
  grpc:
    port: 50051

storage:
  hot:
    max_memory: "4GB"
    ttl: "1h"
  warm:
    path: "/var/lib/feather/data"

observability:
  metrics:
    enabled: true
    port: 9090
  logging:
    level: "info"
    format: "json"
```

Run with the config file:

```bash
./feather -config feather.yaml
```

## What's Next?

You've successfully:
- ✅ Installed Feather
- ✅ Stored and retrieved features
- ✅ Performed batch operations
- ✅ Used point-in-time queries

Continue learning:

- **[Architecture Overview](./concepts/architecture)** - Understand how Feather works under the hood
- **[Tiered Storage](./concepts/tiered-storage)** - Learn about hot and warm tiers
- **[Deployment Guide](./guides/deployment)** - Deploy Feather in production
- **[API Reference](./api-reference)** - Complete API documentation

## Troubleshooting

### Port Already in Use

If port 8080 is taken, use environment variables:

```bash
FEATHER_HTTP_PORT=8081 ./feather
```

### Permission Denied

Make sure the binary is executable:

```bash
chmod +x feather
```

### Data Directory Issues

Feather needs write access to the data directory:

```bash
# Use a custom path
FEATHER_WARM_PATH=./data ./feather
```

For more help, see the [Troubleshooting Guide](./troubleshooting).
