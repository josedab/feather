# Feature Flags Reference

Feather uses feature flags (`HTTPServerFeatureConfig`) to toggle optional HTTP handler groups on the server. Each flag controls whether a set of API routes is registered at startup. Flags are defined in `internal/server/http.go` and configured in `cmd/feather/main.go`.

## Flag Reference

### Core

| Flag | Description | External Dependencies | Default |
|------|-------------|----------------------|---------|
| `EnableGroups` | Registers feature group management routes | None | `true` |
| `EnableBackfill` | Registers backfill routes; requires `storage.Store` | None (uses store) | `true` |
| `EnableStreaming` | Registers streaming feature delivery routes | None | `true` |
| `EnableCatalog` | Registers feature catalog browsing routes | None | `true` |
| `EnableAuth` | Registers authentication routes | None | `true` |
| `EnableCORS` | Enables CORS middleware on all routes (not a handler group; wraps the root handler) | `HTTPServerCoreConfig.CORS` | `false` |
| `EnableCache` | Registers cache management routes; operates on `storage.Store` | None (uses store) | `true` |
| `EnableConsistency` | Registers consistency-check routes; operates on `storage.Store` | None (uses store) | `true` |
| `EnableImpact` | Registers impact analysis routes | None | `true` |
| `EnableTenant` | Registers multi-tenant isolation routes; uses `TenantMaxBytes` from core config (default 4 GB) | None | `false` |

### Data Management

| Flag | Description | External Dependencies | Default |
|------|-------------|----------------------|---------|
| `EnableTransform` | Registers feature transformation routes; operates on `storage.Store` | None (uses store) | `true` |
| `EnableDrift` | Registers drift detection routes using `drift.NewDetector` | None (self-contained) | `false` |
| `EnableLineage` | Registers feature lineage tracking routes using `lineage.NewTracker` | None (self-contained) | `false` |
| `EnableQuality` | Registers data quality validation routes using `quality.NewValidator` | None (self-contained) | `false` |
| `EnableFreshness` | Registers adaptive feature freshness routes using `freshness.NewManager` | None (self-contained) | `false` |
| `EnableMigration` | Registers Feast migration utility routes using `migration.NewManager` | None (self-contained) | `false` |
| `EnableWarehouse` | Registers cloud data warehouse connector routes; requires `storage.Store` and `storage.Registry` | None (uses store + schema) | `false` |
| `EnableGovernance` | Registers enterprise governance routes | None | `false` |
| `EnableContracts` | Registers declarative feature contract and SLA enforcement routes using `contract.NewManager` | None (self-contained) | `false` |
| `EnableMaterialization` | Registers DAG-based pipeline materialization routes using `materialization.NewEngine` | None (self-contained) | `false` |
| `EnableVersioning` | Registers git-like feature versioning routes using `versioning.NewVersionStore` | None (self-contained) | `false` |
| `EnableValidation` | Registers online/offline feature parity validation routes using `validation.NewValidator` | None (self-contained) | `false` |
| `EnableParity` | Registers online/offline parity checking routes using `parity.NewChecker` | None (self-contained) | `false` |
| `EnableTimeTravel` | Registers time-travel debugging routes using `timetravel.NewDebugger` | None (self-contained) | `false` |

### ML & AI

| Flag | Description | External Dependencies | Default |
|------|-------------|----------------------|---------|
| `EnableML` | Registers ML feature serving routes; operates on `storage.Store` | None (uses store) | `true` |
| `EnableModelServing` | Registers multi-model serving and model registry routes; operates on `storage.Store` | None (uses store) | `false` |
| `EnableSemantic` | Registers semantic feature search routes using `semantic.NewLocalEmbedder` | None (self-contained; uses local embedder) | `false` |
| `EnableEmbedding` | Registers batch embedding processing routes | None | `false` |
| `EnableAutoFE` | Registers automated feature engineering routes (`autofe` package) | None | `false` |
| `EnableLLMCache` | Registers semantic LLM prompt/response caching routes (`llmcache` package) | None | `false` |
| `EnableLLMFeatures` | Registers LLM-specific feature type routes using `llmfeature.NewStore` | None (self-contained) | `false` |
| `EnableRAG` | Registers RAG (Retrieval-Augmented Generation) pipeline routes using `rag.NewPipeline` | None (self-contained) | `false` |
| `EnableExperiment` | Registers A/B testing and experimentation routes using `experiment.NewEngine` | None (self-contained) | `false` |
| `EnableAutogen` | Registers automatic feature generation routes using `autogen.NewGenerator` | None (self-contained) | `false` |

### Platform

| Flag | Description | External Dependencies | Default |
|------|-------------|----------------------|---------|
| `EnableSharding` | Registers shard routing routes (sharding package) | None | `false` |
| `EnableCluster` | Registers cluster management routes; handler works with nil components (returns 503 for unconfigured endpoints) | None (nil-safe) | `false` |
| `EnableConsensus` | Registers Raft-like consensus routes using `consensus.NewRaftNode` and `consensus.NewShardManager` | None (self-contained) | `false` |
| `EnableReplication` | Registers multi-region active-active replication routes using `replication.NewManager` | None (self-contained) | `false` |
| `EnableFederation` | Registers distributed feature store federation routes using `federation.NewFederation` | None (self-contained) | `false` |
| `EnableCloudService` | Registers managed cloud service control plane routes (`cloudservice` package) | None | `false` |
| `EnableControlPlane` | Registers multi-cloud managed control plane routes using `controlplane.NewManager` | None (self-contained) | `false` |
| `EnableGeoRouting` | Registers latency-based geo-routing routes (`georouting` package) | None | `false` |
| `EnableEdgeRuntime` | Registers lightweight edge runtime and offline-first sync routes (`edgeruntime` package) | None | `false` |
| `EnableSaaS` | Registers subscription, billing, and provisioning routes using `saas` package components | None (self-contained) | `false` |
| `EnableMarketplace` | Registers feature marketplace routes for cross-team discovery (`marketplace` package) | None | `false` |
| `EnableABRollout` | Registers canary rollout and traffic-splitting routes (`abrollout` package) | None | `false` |
| `EnableCompute` | Registers feature computation engine routes using `compute.NewComputeEngine` | None (self-contained) | `false` |
| `EnablePushdown` | Registers server-side expression evaluation routes using `pushdown.NewEvaluator` | None (self-contained) | `false` |

### Integrations

| Flag | Description | External Dependencies | Default |
|------|-------------|----------------------|---------|
| `EnableDBT` | Registers dbt sync integration routes | `HTTPServerDependencies.DBTOptions` | `cfg.DBT.Enabled` |
| `EnableWASM` | Registers WebAssembly custom transformation routes using `wasm.NewRuntime` | None (self-contained) | `false` |
| `EnableGraphQL` | Registers GraphQL API routes; requires non-nil `store` and `schema` at registration time | None (uses store + schema) | `false` |
| `EnableFeatherQL` | Registers FeatherQL DSL pipeline routes (`featherql` package) | None | `false` |
| `EnableStreamSQL` | Registers streaming SQL engine routes using `streamsql.NewEngine` | None (self-contained) | `false` |
| `EnableGitOps` | Registers GitOps declarative definition and policy-as-code routes using `gitops` package components | None (self-contained) | `false` |
| `EnablePlugin` | Registers plugin and extension framework routes using `plugin.NewRegistry` | None (self-contained) | `false` |

### Developer Tools

| Flag | Description | External Dependencies | Default |
|------|-------------|----------------------|---------|
| `EnableUI` | Registers embedded web UI for the feature catalog using `ui.NewHandler` | None | `cfg.UI.Enabled` |
| `EnableBenchmark` | Registers benchmarking routes; operates on `storage.Store` | None (uses store) | `true` |
| `EnablePlayground` | Registers feature exploration and query builder routes using `playground.NewService` | None (self-contained) | `false` |
| `EnableDashboardV2` | Registers v2 dashboard routes; requires `storage.Store` and `metrics.Metrics` | None (uses store + metrics) | `false` |
| `EnableComposition` | Registers real-time feature composition routes using `composition.NewEngine` | None (uses store) | `false` |

### Operations

| Flag | Description | External Dependencies | Default |
|------|-------------|----------------------|---------|
| `EnableObservability` | Registers observability routes; operates on `storage.Store` | None (uses store) | `true` |
| `EnableSLA` | Registers SLA tracking and enforcement routes using `sla.NewManager` | None (self-contained) | `false` |
| `EnableScheduler` | Registers cron scheduling routes using `warehouse.NewCronScheduler` | None (self-contained) | `false` |
| `EnableCost` | Registers cost attribution and chargeback routes using `cost` package components | None (self-contained) | `false` |
| `EnableFinOps` | Registers FinOps cost attribution and dashboard routes using `finops.NewManager` | None (self-contained) | `false` |
| `EnableMonitoring` | Registers unified monitoring with alerting and auto-remediation routes using `monitoring.NewManager` | None (self-contained) | `false` |

## External Dependency Activation

The `HTTPServerDependencies` struct (defined in `internal/server/http.go`) provides optional external components to handlers. The following dependency fields exist:

| Dependency Field | Used By |
|-----------------|---------|
| `DBTOptions` | `EnableDBT` — passed to `NewDBTHandler` |
| `DriftDetector` | Declared but not consumed; `EnableDrift` creates its own `drift.NewDetector` |
| `LineageTracker` | Declared but not consumed; `EnableLineage` creates its own `lineage.NewTracker` |
| `SemanticSearch` | Declared but not consumed; `EnableSemantic` creates its own local embedder |
| `WASMRuntime` | Declared but not consumed; `EnableWASM` creates its own `wasm.NewRuntime` |
| `FederationClient` | Declared but not consumed; `EnableFederation` creates its own `federation.NewFederation` |
| `QualityValidator` | Declared but not consumed; `EnableQuality` creates its own `quality.NewValidator` |
| `AutogenGenerator` | Declared but not consumed; `EnableAutogen` creates its own `autogen.NewGenerator` |
| `ExperimentEngine` | Declared but not consumed; `EnableExperiment` creates its own `experiment.NewEngine` |
| `GraphQLSchema` | Declared but not consumed; `EnableGraphQL` builds its own schema from `store` and `schema` |
| `ClusterMembership` | Declared but not consumed; `EnableCluster` initializes nil components |
| `ClusterRing` | Declared but not consumed; `EnableCluster` initializes nil components |
| `ClusterPartitionMap` | Declared but not consumed; `EnableCluster` initializes nil components |
| `ClusterRebalancer` | Declared but not consumed; `EnableCluster` initializes nil components |

> **Note:** In the current codebase, only `DBTOptions` is actively consumed from `HTTPServerDependencies`. All other dependency interfaces are declared in the struct but the corresponding handlers create their own default instances when the flag is enabled. The comment in `cmd/feather/main.go` (lines 331–334) indicates these dependencies are intended for future use when external implementations are available.

## Defaults Summary

The following flags are enabled by default in `cmd/feather/main.go`:

- `EnableGroups`
- `EnableBackfill`
- `EnableStreaming`
- `EnableCatalog`
- `EnableAuth`
- `EnableML`
- `EnableTransform`
- `EnableCache`
- `EnableConsistency`
- `EnableImpact`
- `EnableObservability`
- `EnableBenchmark`

Two flags are conditionally enabled based on configuration:

- `EnableUI` — set to `cfg.UI.Enabled`
- `EnableDBT` — set to `cfg.DBT.Enabled`

All other flags default to `false` (Go zero value for `bool`).
