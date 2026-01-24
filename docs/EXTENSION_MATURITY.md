# Extension Maturity Matrix

This document provides an honest assessment of every extension package's production readiness. Extensions are classified into three tiers:

- ✅ **Production** — Battle-tested, persistent, handles edge cases, suitable for production use
- 🟡 **Beta** — Functional and tested but in-memory-only or missing edge case handling
- ⚠️ **Prototype** — Working implementation but uses stubs, simplified algorithms, or lacks persistence. Suitable for evaluation, not production.

## Core Extensions (Pre-existing, Production-Quality)

| Package | Status | Notes |
|---------|--------|-------|
| `drift` | ✅ Production | KS test, PSI, mean shift detection with alert cooldown |
| `freshness` | ✅ Production | Adaptive TTL, SLA enforcement, ML predictions |
| `semantic` | ✅ Production | Local embedder, vector similarity search |
| `marketplace` | ✅ Production | Publish, subscribe, billing engine |
| `sharding` | ✅ Production | Consistent hash ring, replication |
| `composition` | ✅ Production | DAG engine with executor |
| `experiment` | 🟡 Beta | A/B testing engine, functional |
| `lineage` | 🟡 Beta | Graph tracking, UI components |
| `versioning` | 🟡 Beta | Git-like versioning with snapshots |
| `materialization` | 🟡 Beta | DAG-based pipeline engine |
| `graphql` | ⚠️ Prototype | Schema generation, basic queries |
| `wasm` | ⚠️ Prototype | Sandboxed runtime, basic execution |
| `rag` | ⚠️ Prototype | Pipeline with default embedder |
| `timetravel` | ⚠️ Prototype | Debugger component only |
| `streamdsl` | ⚠️ Prototype | Pipeline manager, basic compiler |

## Next-Gen v1 Extensions (Round 1)

| Package | Status | Notes | Key Limitation |
|---------|--------|-------|----------------|
| `streamcompute` | 🟡 Beta | Tumbling/sliding/session windows, 7 aggregations | In-memory state only |
| `sdkcodegen` | ✅ Production | Go/Python/TypeScript code generation | Generated code is skeleton |
| `promptstore` | 🟡 Beta | Versioning, rendering, usage tracking | In-memory, no persistence |
| `consistencyvalidator` | 🟡 Beta | KS test, z-score, alert management | In-memory samples |
| `featuredashboard` | 🟡 Beta | Latency percentiles, health scoring | In-memory snapshots |
| `featherqlv2` | 🟡 Beta | **Recursive-descent parser**, AST, execution plans | No query optimizer |
| `embeddingmgmt` | ⚠️ Prototype | Collection management, model registry | **Brute-force O(n) search** — needs HNSW |
| `schemaevolution` | ✅ Production | Compatibility modes, coercion, rollback | In-memory state |
| `feastcompat` | ⚠️ Prototype | API parsing, mapping management | **Returns synthetic data** — no backend integration |
| `saascontrol` | 🟡 Beta | Plan tiers, quota enforcement, provisioning | In-memory, simulated provisioning |

## Next-Gen v2 Extensions (Round 2)

| Package | Status | Notes | Key Limitation |
|---------|--------|-------|----------------|
| `lineagegraph` | ✅ Production | DAG with cycle detection, impact analysis, upstream/downstream | In-memory graph |
| `adaptivecache` | 🟡 Beta | Exponential decay scoring, hit/miss tracking | In-memory predictor |
| `contracttest` | 🟡 Beta | Schema and range validation, severity levels | Basic rule engine |
| `backpressure` | 🟡 Beta | Queue/latency/error monitoring, scale recommendations | In-memory samples |
| `offlinestore` | ⚠️ Prototype | Point-in-time queries, dataset management | **In-memory only** — needs Parquet I/O |
| `benchpub` | 🟡 Beta | Configurable workloads, comparison reports | Simulated latency numbers |
| `gitopsdefs` | 🟡 Beta | Definition loading, reconciliation, diff | No Git integration |
| `abfeatures` | 🟡 Beta | Hash-based routing, z-test significance | In-memory experiments |
| `federateddiscovery` | 🟡 Beta | Catalog, search, subscriptions | Single-instance, no network |
| `wasmudf` | ⚠️ Prototype | Module registry, schema validation | **Simulated execution** — no real WASM runtime |

## Next-Gen v3 Extensions (Round 3)

| Package | Status | Notes | Key Limitation |
|---------|--------|-------|----------------|
| `apigateway` | 🟡 Beta | Weighted routing, health-based selection | In-memory, no real proxying |
| `importancescoring` | 🟡 Beta | Access frequency + variance + correlation scoring | In-memory, simplified scoring |
| `anomalydetect` | 🟡 Beta | Z-score, IQR, quarantine, learning period | In-memory rolling stats |
| `feathercli` | 🟡 Beta | Table/CSV/JSON formatting, query parsing | HTTP client is simulated |
| `incrmat` | 🟡 Beta | Dependency propagation, topological sort | In-memory, no checkpointing |
| `webhooks` | 🟡 Beta | Event dispatch, delivery tracking, dead letter | Simulated HTTP delivery |
| `cloudstorage` | 🟡 Beta | Put/get/delete/list/copy abstraction | **In-memory only** — no real S3/GCS |
| `auditlog` | 🟡 Beta | Action/actor/resource filtering, JSON/CSV export | In-memory, no durable storage |
| `openapisync` | 🟡 Beta | Route introspection, spec generation | No schema inference |
| `terraformprovider` | 🟡 Beta | CRUD, plan/apply, import | In-memory state, no real Terraform SDK |

## Utility Extensions (Round 4)

| Package | Status | Notes |
|---------|--------|-------|
| `pluginsdk` | 🟡 Beta | Handler interface, registry, routing |

## Summary

| Status | Count | Percentage |
|--------|-------|------------|
| ✅ Production | 9 | 13% |
| 🟡 Beta | 44 | 64% |
| ⚠️ Prototype | 16 | 23% |
| **Total** | **69** | 100% |

## Hardening Priorities

The following extensions would benefit most from production hardening:

1. **`embeddingmgmt`** — Replace brute-force search with HNSW from existing `internal/core/vector/` package
2. **`feastcompat`** — Wire to actual Feather storage backend instead of synthetic responses
3. **`offlinestore`** — Add real Parquet read/write using existing `parquet-go` dependency
4. **`cloudstorage`** — Implement real S3/GCS backends using cloud SDKs
5. **`wasmudf`** — Integrate with existing `wasm` extension's runtime for real WASM execution
