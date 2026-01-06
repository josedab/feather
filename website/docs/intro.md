---
sidebar_position: 1
slug: /
title: Introduction
description: Feather is a high-performance, real-time feature store for machine learning applications.
---

# Introduction to Feather

Feather is a **high-performance, real-time feature store** designed to serve machine learning features with sub-millisecond latency. It enables ML teams to store, retrieve, and manage features efficiently without the operational complexity of traditional feature stores.

## What is a Feature Store?

A feature store is a centralized repository for storing, managing, and serving machine learning features. Features are the input variables used by ML models to make predictions—things like user click counts, purchase history, or product embeddings.

Feature stores solve several critical problems:

- **Consistency**: Ensure training and serving use the same feature definitions
- **Reusability**: Share features across models and teams
- **Freshness**: Serve up-to-date features in real-time
- **Point-in-time correctness**: Generate training data without data leakage

## Why Feather?

Feather was built with a specific philosophy: **operational simplicity without sacrificing performance**.

### Sub-Millisecond Latency

Feather's two-tier storage architecture delivers P99 latency under 1ms for hot tier reads:

| Tier | Storage | Latency | Use Case |
|------|---------|---------|----------|
| **Hot** | In-memory (256 shards) | < 1ms P99 | Real-time serving |
| **Warm** | BadgerDB (disk) | 1-10ms P99 | Historical queries |

### Single Binary Deployment

Unlike other feature stores that require Redis, PostgreSQL, or cloud services, Feather is a single binary:

```bash
# That's it. No Docker, no databases, no cloud accounts.
./feather
```

### Production Ready

Feather includes everything you need for production:

- **Prometheus metrics** on port 9090
- **OpenTelemetry tracing** with OTLP export
- **Structured logging** with slog
- **Health probes** for Kubernetes (`/health`, `/ready`, `/live`)
- **Graceful shutdown** with 30-second timeout

## Key Features

| Feature | Description |
|---------|-------------|
| **Tiered Storage** | Automatic hot/warm tier management with LRU eviction |
| **Point-in-Time Queries** | Retrieve features as they existed at any timestamp |
| **Real-Time Aggregations** | Sliding window aggregations (count, sum, avg, min, max) |
| **Vector Search** | HNSW-based similarity search for embeddings |
| **Drift Detection** | Statistical drift monitoring with alerts |
| **Multiple APIs** | HTTP REST and gRPC with streaming support |
| **Client SDKs** | Go, Python, Java, Rust, TypeScript |

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Client Applications                      │
│           (ML Models, Training Pipelines, Services)          │
└─────────────────────────────┬───────────────────────────────┘
                              │
               ┌──────────────┴──────────────┐
               ▼                              ▼
        ┌────────────┐                ┌────────────┐
        │ HTTP :8080 │                │gRPC :50051 │
        └─────┬──────┘                └─────┬──────┘
              │                              │
              └──────────────┬───────────────┘
                             ▼
                  ┌─────────────────────┐
                  │   Feature Engine    │
                  │  • Schema Registry  │
                  │  • Aggregations     │
                  │  • Rate Limiting    │
                  └──────────┬──────────┘
                             │
            ┌────────────────┴────────────────┐
            ▼                                  ▼
     ┌─────────────┐                   ┌─────────────┐
     │  Hot Tier   │                   │ Warm Tier   │
     │  (Memory)   │◄─────────────────▶│ (BadgerDB)  │
     │   sub-1ms      │     overflow      │   1-10ms    │
     └─────────────┘                   └─────────────┘
```

## Who Uses Feather?

Feather is designed for:

- **ML Engineers** serving features for model inference
- **Data Scientists** generating training data with point-in-time correctness
- **Platform Teams** building internal ML infrastructure
- **Startups** who need feature serving without operational overhead

## Next Steps

Ready to get started? Here's your learning path:

1. **[Getting Started](./getting-started)** - Run Feather and serve your first feature in 5 minutes
2. **[Core Concepts](./concepts/architecture)** - Understand how Feather works
3. **[API Reference](./api-reference)** - Detailed HTTP and gRPC documentation
4. **[Deployment Guide](./guides/deployment)** - Production deployment patterns

## Getting Help

- **GitHub Issues**: [Report bugs or request features](https://github.com/feather-store/feather/issues)
- **GitHub Discussions**: [Ask questions and share ideas](https://github.com/feather-store/feather/discussions)
- **Contributing**: [Contribute to Feather](./contributing)
