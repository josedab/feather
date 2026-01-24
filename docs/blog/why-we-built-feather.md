# Why We Built Feather

*A sub-millisecond feature store for the real-time ML era.*

## The Problem

Machine learning teams spend an enormous amount of time wrangling features — and most of that time isn't spent on modelling. It's spent fighting inconsistency between training and serving, struggling with latency spikes in real-time pipelines, and managing operational overhead that scales with every new feature.

We lived this pain first-hand. Feature definitions drifted between offline training jobs and online serving paths. Batch pipelines couldn't keep up with the latency demands of fraud detection, recommendation, and personalization models. Every new data source meant another glue script, another deployment headache, and another page at 2 AM.

Existing feature stores asked us to choose: fast *or* flexible, simple *or* scalable. We wanted all of it.

## Why Go

Go gave us three things we couldn't get anywhere else in a single package:

1. **Sub-millisecond performance.** Go's lightweight goroutines and zero-GC-pressure design let Feather serve features in under a millisecond — consistently, not just on a good day.
2. **Single binary deployment.** One `feather` binary, statically compiled. No JVM warm-up, no Python dependency hell, no sidecar containers. Copy it to a server and run.
3. **Zero external runtime dependencies.** Feather starts in seconds with an embedded storage engine. You can run it on a laptop, a Kubernetes pod, or an edge device with the exact same binary.

## What Makes Feather Different

Feather ships with **114 API handlers** spanning core feature serving, vector similarity search, drift detection, automated feature engineering, and more — all behind a three-tier maturity model (stable, beta, experimental) so you always know what's production-ready.

It includes **7 SDK skeletons** (Go, Python, TypeScript, Java, Ruby, Rust, .NET) so every team can integrate in their language of choice. It's **GenAI-native**, with built-in LLM response caching and semantic vector search. And it's **Feast-compatible**, so migrating existing pipelines is a drop-in replacement.

## Architecture Highlights

Feather's **tiered storage** (hot LRU cache + warm BadgerDB) keeps frequently accessed features in memory while persisting historical versions on disk. A **pluggable handler architecture** lets you extend Feather with new endpoints — marketplace, FeatherQL, geo-routing, A/B rollouts — without touching core code. The **three-tier maturity system** (stable → beta → experimental) gives operators confidence about which features are production-safe.

## Quick Start

Get Feather running in three commands:

```bash
# Build from source
make build

# Start the server in dev mode (no external deps)
make run-dev

# Store and retrieve your first feature
curl -X POST http://localhost:8080/v1/features \
  -H "Content-Type: application/json" \
  -d '{"entity":"user:1","group":"profile","name":"age","value":28}'

curl "http://localhost:8080/v1/features?entity=user:1&feature=profile:age"
```

That's it. No Kafka, no Redis, no YAML manifests — just features.

## Get Involved

Feather is Apache 2.0 licensed and we're building in the open. Here's how you can help:

- ⭐ **Star the repo** to show your support.
- 🐛 **Try it out** and file issues — every bug report makes Feather better.
- 🔧 **Contribute** — check out `GOOD_FIRST_ISSUES.md` for beginner-friendly tasks.
- 💬 **Join the community** on Discord to chat with the maintainers.

We believe ML infrastructure shouldn't be a full-time job. Feather is our answer — and we'd love your help making it the standard.
