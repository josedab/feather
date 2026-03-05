# Package Guide

This guide categorizes every package under `internal/` by maturity level and purpose.
Use it to understand which packages are load-bearing production code versus experimental extensions.

## Maturity Levels

| Level | Meaning |
|-------|---------|
| **Stable** | Production-ready, well-tested, breaking changes follow semver |
| **Beta** | Functional and tested, API may change between minor releases |
| **Experimental** | Working implementation, may be incomplete or change significantly |

---

## Core (`internal/core/`) — Stable

These packages form the essential feature store. All are **stable** and required for the server to run.

| Package | Description |
|---------|-------------|
| `aggregation` | Real-time sliding window aggregations (count, sum, avg, min, max) |
| `config` | YAML and environment variable configuration loading and validation |
| `domain` | Core types (`FeatureValue`, `FeatureGroup`), error codes, and constants |
| `export` | Training data export (CSV, JSON, JSONL, Parquet) |
| `ingestion` | Kafka consumer with circuit breaker, HTTP push endpoint |
| `logging` | Structured logging with `slog` (JSON/text) |
| `metrics` | Prometheus metrics collection and exposition |
| `server` | HTTP REST and gRPC servers, health checks, handler registration |
| `storage` | Tiered storage engine (hot LRU cache + warm BadgerDB), schema registry |
| `tracing` | OpenTelemetry distributed tracing |
| `vector` | HNSW-based vector similarity search |

---

## Extensions (`internal/extensions/`) — Mixed Maturity

Optional feature modules registered through the [pluggable handler system](./adr/0008-pluggable-http-handlers.md).
Enable or disable via the feature flag map in `HTTPServerFeatureConfig.EnabledFeatures`.

### Stable Extensions

| Package | Description |
|---------|-------------|
| `drift` | Feature drift detection with KL divergence, JS divergence, and PSI metrics |
| `freshness` | Adaptive TTL management, ML-driven freshness predictions, SLA enforcement |
| `marketplace` | Cross-team feature publishing, discovery, and subscription |
| `semantic` | AI-powered feature discovery with semantic search and NL queries |
| `sharding` | Distributed sharding with consistent hashing and quorum R/W |

### Beta Extensions

| Package | Description |
|---------|-------------|
| `abrollout` | Feature versioning with A/B canary rollout traffic management |
| `abfeatures` | A/B feature flag management and targeting |
| `activeactive` | CRDT-based active-active replication with gossip protocol |
| `autofe` | Automated feature engineering with candidate scoring |
| `backfillengine` | Advanced backfill orchestration engine |
| `backpressure` | Backpressure management for ingestion pipelines |
| `cache` | Advanced caching strategies (write-through, write-behind) |
| `cloudservice` | Managed cloud control plane with auto-scaling |
| `composition` | Real-time feature composition from multiple sources |
| `consistencyvalidator` | Consistency validation rules and checks |
| `contractcicd` | Feature contract CI/CD integration |
| `contracttest` | Feature contract testing framework |
| `edgeruntime` | Lightweight edge runtime with offline-first sync |
| `embedding` | Batch embedding processing for feature vectors |
| `embeddingmgmt` | Embedding lifecycle and version management |
| `experiment` | A/B testing and experimentation framework |
| `featherql` | SQL-like DSL for declarative feature pipelines |
| `featherqlv2` | FeatherQL v2 engine with enhanced capabilities |
| `featuredashboard` | Feature analytics and monitoring dashboard |
| `ftl` | Feature Transformation Language — SQL-like DSL compiled to in-memory transforms |
| `georouting` | Multi-cloud geo-routing with data residency compliance |
| `gitopsdefs` | GitOps schema definitions and management |
| `importancescoring` | Feature importance scoring and ranking |
| `incrmat` | Incremental materialization engine |
| `joins` | Feature join engine for combining features across entities |
| `lifecycle` | Feature lifecycle management |
| `lineage` | Feature lineage tracking and visualization |
| `lineageanalysis` | Lineage impact analysis |
| `lineagegraph` | Lineage graph visualization |
| `llmcache` | Semantic LLM prompt/response caching with cost tracking |
| `llmstore` | LLM feature storage and retrieval |
| `materialization` | DAG-based pipeline engine for feature materialization |
| `mesh` | Service mesh for distributed feature serving |
| `mobilesync` | Mobile SDK sync protocol with delta sync and conflict resolution |
| `multimodal` | Multi-modal feature storage and embedding index |
| `obsconsole` | Observability console UI backend |
| `promptstore` | LLM prompt management and versioning |
| `qualityscore` | Automated multi-signal feature quality scoring engine |
| `saascontrol` | SaaS control plane management |
| `schemaevolution` | Schema evolution and migration management |
| `sdkcodegen` | SDK code generation for multiple languages |
| `streamcompute` | Stream computation engine |
| `versioning` | Git-like versioning for feature definitions and values |

### Experimental Extensions

| Package | Description |
|---------|-------------|
| `adaptivecache` | ML-driven adaptive caching strategies |
| `anomalydetect` | Anomaly detection on feature values |
| `apigateway` | API gateway and routing layer |
| `arrowflight` | Zero-copy columnar data transport via Arrow Flight protocol |
| `auditlog` | Audit logging for feature operations |
| `audittrail` | Event-sourced audit trail with Merkle tree hash chaining |
| `autogen` | Automatic feature generation and schema inference |
| `benchpub` | Benchmark result publishing |
| `cloudstorage` | Cloud storage connectors (S3, GCS, Azure Blob) |
| `compression` | Intelligent tiered compression with ML-based strategy selection |
| `computegraph` | Computation graph engine for complex feature DAGs |
| `diffprivacy` | Differential privacy engine with Laplace/Gaussian noise and budget tracking |
| `feastcompat` | Feast compatibility adapter |
| `feathercli` | CLI tool package |
| `federateddiscovery` | Federated feature discovery across clusters |
| `fedlearning` | Federated learning adapter with secure aggregation protocol |
| `graphql` | GraphQL API for the feature store |
| `llm` | LLM-powered feature generation pipelines |
| `llmfeature` | First-class LLM-specific feature types and storage |
| `llmgateway` | Multi-provider LLM gateway with routing and rate limiting |
| `modelserving` | Multi-model feature serving with model registry |
| `notebooksdk` | Server-side Jupyter/Colab integration with magic commands |
| `offlinestore` | Offline feature store for batch workloads |
| `openapisync` | OpenAPI spec synchronization |
| `playgroundv2` | Enhanced browser-based feature playground with simulation and deploy |
| `pluginsdk` | Plugin SDK for third-party extensions |
| `prefetch` | Predictive feature pre-fetching using ML-based access patterns |
| `pythonsdk` | Python SDK package |
| `qualitygates` | CI/CD quality gates with schema validation and merge-blocking rules |
| `queryplanner` | Self-optimizing cost-based query planner with adaptive replanning |
| `rag` | Native RAG pipeline for retrieval-augmented generation |
| `skewdetect` | Online/offline feature skew detection |
| `starlarkudf` | Starlark user-defined function runtime |
| `streamdsl` | Stream processing DSL with pipeline compiler |
| `terraformprovider` | Terraform provider for Feather resources |
| `timetravel` | Time-travel debugging for feature values |
| `wasm` | WebAssembly runtime for custom transformations |
| `wasmruntime` | WASM runtime management |
| `wasmudf` | WASM user-defined function execution |
| `webhooks` | Webhook event delivery |

---

## Integrations (`internal/integrations/`) — Beta

External system connectors for batch and streaming workloads.

| Package | Description |
|---------|-------------|
| `airflow` | Apache Airflow provider with DAG operators and freshness sensors |
| `dbt` | Converts dbt models to Feather feature groups |
| `flink` | Apache Flink integration for streaming feature computation |
| `flinkpipeline` | Apache Flink pipeline management |
| `kubeflow` | Kubeflow Pipelines components for feature retrieval |
| `mlflow` | MLflow tracking integration with feature lineage |
| `offlinesync` | Offline/online synchronization for batch training data |
| `spark` | Apache Spark connector for batch feature export |
| `streaming` | Real-time streaming pipelines with windowing and CEP |
| `streamsql` | SQL engine for defining real-time feature transformations |
| `warehouse` | Cloud data warehouse connectors (BigQuery, Redshift, Snowflake) |

---

## Platform (`internal/platform/`) — Mixed Maturity

Cross-cutting infrastructure for enterprise deployments.

### Stable Platform

| Package | Description |
|---------|-------------|
| `auth` | API key authentication with SHA256 hashing and RBAC |
| `clientip` | Secure client IP resolution for HTTP requests |
| `governance` | Access control lists and data governance policies |
| `groups` | Feature group management and operations |
| `tenant` | Multi-tenant isolation with per-tenant resource limits |

### Beta Platform

| Package | Description |
|---------|-------------|
| `cloudcontrol` | Cloud control plane management |
| `cluster` | Distributed cluster management with consistent hashing |
| `consensus` | Simplified Raft-like consensus for leader election |
| `consistency` | Online/offline data consistency verification |
| `contract` | Declarative feature contracts and SLA enforcement |
| `controlplane` | Multi-cloud managed control plane |
| `cost` | Cost tracking, budget management, and alerting |
| `federation` | Distributed feature store federation |
| `finops` | Cost attribution, FinOps dashboards, and chargeback |
| `impact` | Feature impact analysis and dependency tracking |
| `migration` | Feast-to-Feather migration utilities |
| `monitoring` | Unified monitoring with alerting and auto-remediation |
| `multiregion` | Multi-region deployment management |
| `observability` | Unified observability dashboards and exporters |
| `operator` | Kubernetes operator with custom resources |
| `quality` | Data quality monitoring and validation |
| `realtimemonitor` | Real-time feature monitoring |
| `registry` | Feature catalog and discovery services |
| `replication` | Multi-region active-active replication |
| `sla` | Service Level Agreement tracking and enforcement |
| `transform` | Feature transformation pipelines |
| `urlvalidation` | URL validation utilities |
| `validation` | Statistical online/offline parity validation |

### Experimental Platform

| Package | Description |
|---------|-------------|
| `autoscaler` | Kubernetes-aware auto-scaling for feature serving |
| `cloud` | Multi-cloud control plane for managed deployments |
| `costopt` | Cost optimization with forecasting and recommendations |
| `gitops` | Git-based declarative feature governance |
| `parity` | Online/offline feature parity validation |
| `plugin` | Plugin and extension framework |
| `pushdown` | Server-side expression evaluation for computed features |
| `saas` | Subscription, billing, and provisioning |

---

## Tools (`internal/tools/`) — Beta

Developer and operational utilities.

| Package | Description |
|---------|-------------|
| `backfill` | Historical feature data backfilling with DAG orchestration |
| `benchmark` | Performance benchmarking harness |
| `benchsuite` | Extended benchmark suite with configurable workloads |
| `catalog` | Feature catalog UI service |
| `compute` | Feature Computation Engine (FCE) with expression evaluator |
| `dashboard` | Feature monitoring dashboard backend with explorer |
| `mcp` | Model Context Protocol server for AI agent integration |
| `migrationcli` | CLI tool for Feast-to-Feather migration |
| `ml` | Machine learning model integration |
| `pipelinebuilder` | Code generation and templates for feature pipelines |
| `playground` | Feature exploration and interactive query builder |
| `ui` | Embedded web UI for browsing and managing features |

---

## Adding a New Package

See the [Extension Development Guide](./extension-development.md) for detailed instructions.

1. Create the package under the appropriate directory (`extensions/`, `platform/`, etc.)
2. Add a `doc.go` with a package-level comment describing its purpose
3. Register its HTTP handler in `internal/core/server/features.go` (if it exposes an API)
4. Add it to this guide with the appropriate maturity level
5. Add tests in a `*_test.go` file alongside the implementation
