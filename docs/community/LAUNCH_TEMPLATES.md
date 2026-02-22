# Social Media Launch Templates

## Hacker News (Show HN)

**Title:** Show HN: Feather – Sub-millisecond feature store for ML, written in Go

**Text:**
Hi HN, I built Feather, an open-source real-time feature store that serves ML features with sub-millisecond P99 latency from a single Go binary.

Key differentiators vs Feast/Tecton:
- 10-100x faster: Go with 256-shard LRU, not Python
- Single binary: `go install` and run, no JVM/Kafka/Redis required
- 114 pluggable feature handlers: from drift detection to LLM prompt management
- 7 SDK languages: Go, Python, TypeScript, Java, Rust, Swift, Kotlin
- Feast-compatible API: migrate without changing client code

Architecture: hot tier (in-memory LRU, <1ms) → warm tier (BadgerDB, <10ms) → offline store (Parquet)

Quick start:
```
git clone https://github.com/feather-store/feather
cd feather && make run-dev
curl localhost:8080/health
```

Benchmarks: [link to docs/benchmarks/]
GitHub: https://github.com/feather-store/feather

Would love feedback on the architecture and API design.

---

## Reddit (r/MachineLearning, r/golang)

**Title:** [P] Feather: Open-source feature store with sub-ms latency, 114 handlers, 7 SDKs — all in a single Go binary

**Body:**
I've been building Feather, a real-time feature store that focuses on performance and breadth:

🪶 **What it is:** Feature store for ML with <1ms P99 latency
⚡ **Why Go:** Single binary, zero external dependencies, 10x faster than Python alternatives
📦 **114 handlers:** Drift detection, anomaly detection, prompt management, embeddings, A/B features, audit logs, and more
🔧 **7 SDKs:** Go, Python, TypeScript, Java, Rust, Swift, Kotlin
🔄 **Feast compatible:** Drop-in replacement API

Links:
- GitHub: https://github.com/feather-store/feather
- Docs: https://feather-store.github.io
- Playground: https://feather-store.github.io/playground

---

## Twitter/X

🪶 Introducing Feather — an open-source feature store for ML with sub-millisecond P99 latency.

✅ Single Go binary — no JVM, no Python, no external deps
✅ 114 pluggable handlers (drift, anomaly, prompts, embeddings)
✅ 7 SDK languages
✅ Feast-compatible API
✅ 255K lines of Go

Try it: github.com/feather-store/feather

#MLOps #MachineLearning #Golang #OpenSource

---

## LinkedIn

**Introducing Feather: A High-Performance Open-Source Feature Store**

After months of development, I'm excited to share Feather — an open-source, real-time feature store written in Go that delivers sub-millisecond feature retrieval for machine learning applications.

In building ML systems, I kept running into the same problems: Python feature stores that couldn't keep up with real-time requirements, complex infrastructure with too many moving parts, and the impossible choice between Feast's flexibility and Tecton's managed experience.

Feather combines the best of both worlds:
• Sub-millisecond P99 latency from a single Go binary
• 114 pluggable feature handlers covering everything from drift detection to LLM prompt management
• Feast-compatible API for zero-code migration
• 7 SDK languages for polyglot teams

I'd love to connect with ML engineers and platform teams who are dealing with feature serving challenges. What's your biggest pain point with feature stores today?

GitHub: https://github.com/feather-store/feather

#MachineLearning #MLOps #OpenSource #FeatureStore #GoLang
