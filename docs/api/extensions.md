# Extension APIs

## Extension APIs

Extension handlers are enabled via the `EnabledFeatures` map in `cmd/feather/main.go`. Run `make api-routes` to see all registered handlers with maturity levels. All extension endpoints use the standard JSON response envelope and error format described in [Error Handling](#error-handling).

> **Maturity levels**: **stable** = production-ready, **beta** = functional but API may change, **experimental** = may be incomplete.

### Sharding & Replication

**Maturity:** stable

Distributed sharding with consistent hashing for horizontal scaling.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/sharding/stats` | Get shard routing statistics |
| `GET` | `/v1/sharding/partition?key=X` | Get partition for a key |
| `GET` | `/v1/sharding/owners?key=X` | Get replica owners for a key |
| `POST` | `/v1/sharding/recompute` | Recompute partition map |

**Example — Get partition for a key:**

```bash
curl "http://localhost:8080/v1/sharding/partition?key=user:123"
```

```json
{
  "success": true,
  "data": {
    "key": "user:123",
    "partition": 42,
    "node": "node-1"
  }
}
```

---

### Feature Marketplace

**Maturity:** stable

Publish, discover, and subscribe to shared feature definitions across teams.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/marketplace/features` | List published features |
| `POST` | `/v1/marketplace/features` | Publish a feature |
| `GET` | `/v1/marketplace/features/{id}` | Get feature details |
| `GET` | `/v1/marketplace/features/{id}/subscribers` | List feature subscribers |
| `POST` | `/v1/marketplace/features/{id}/subscribe` | Subscribe to a feature |
| `DELETE` | `/v1/marketplace/features/{id}/subscribe` | Unsubscribe from a feature |
| `POST` | `/v1/marketplace/features/{id}/deprecate` | Deprecate a feature |
| `GET` | `/v1/marketplace/search?q=X` | Search marketplace |
| `GET` | `/v1/marketplace/stats` | Marketplace statistics |

**Example — Publish a feature:**

```bash
curl -X POST http://localhost:8080/v1/marketplace/features \
  -H "Content-Type: application/json" \
  -d '{
    "name": "user_click_count",
    "description": "Hourly click count per user",
    "entity_type": "user",
    "data_type": "int64",
    "owner": "ml-team",
    "tags": ["engagement", "real-time"]
  }'
```

```json
{
  "success": true,
  "data": {
    "id": "feat_abc123",
    "name": "user_click_count",
    "status": "published"
  }
}
```

---

### Cloud Service

**Maturity:** beta

Managed instance provisioning and scaling.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/cloud/instances` | List managed instances |
| `POST` | `/v1/cloud/instances` | Provision a new instance |
| `GET` | `/v1/cloud/instances/{id}` | Get instance details |
| `GET` | `/v1/cloud/instances/{id}/metrics` | Get instance metrics |
| `POST` | `/v1/cloud/instances/{id}/scale` | Scale an instance |
| `POST` | `/v1/cloud/instances/{id}/autoscale` | Configure autoscaling |
| `DELETE` | `/v1/cloud/instances/{id}` | Terminate an instance |

**Example — Provision an instance:**

```bash
curl -X POST http://localhost:8080/v1/cloud/instances \
  -H "Content-Type: application/json" \
  -d '{
    "name": "prod-us-east",
    "tier": "standard",
    "region": "us-east-1",
    "replicas": 3
  }'
```

---

### FeatherQL (Declarative Pipelines)

**Maturity:** beta

SQL-like DSL for declarative feature pipelines.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/featherql/parse` | Parse a FeatherQL query |
| `POST` | `/v1/featherql/compile` | Compile a FeatherQL pipeline |
| `POST` | `/v1/featherql/execute` | Execute a FeatherQL query |
| `POST` | `/v1/featherql/validate` | Validate a FeatherQL query |
| `GET` | `/v1/featherql/pipelines` | List compiled pipelines |

**v2 endpoints:**

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/featherql/v2/parse` | Parse (v2 engine) |
| `POST` | `/v1/featherql/v2/compile` | Compile (v2 engine) |
| `POST` | `/v1/featherql/v2/execute` | Execute (v2 engine) |
| `GET` | `/v1/featherql/v2/pipelines` | List pipelines (v2) |
| `GET` | `/v1/featherql/v2/pipelines/{id}` | Get pipeline details (v2) |
| `DELETE` | `/v1/featherql/v2/pipelines/{id}` | Delete pipeline (v2) |

**Example — Execute a query:**

```bash
curl -X POST http://localhost:8080/v1/featherql/execute \
  -H "Content-Type: application/json" \
  -d '{
    "query": "SELECT click_count, purchase_total FROM user_features WHERE entity = '\''user:123'\''"
  }'
```

```json
{
  "success": true,
  "data": {
    "columns": ["click_count", "purchase_total"],
    "rows": [
      {"click_count": 42, "purchase_total": 1250.75}
    ]
  }
}
```

---

### LLM Cache

**Maturity:** beta

Semantic caching for LLM prompt/response pairs to reduce latency and cost.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/llm/cache/lookup` | Lookup cached LLM response |
| `POST` | `/v1/llm/cache/store` | Store LLM response |
| `POST` | `/v1/llm/cache/clear` | Clear cache entries |
| `DELETE` | `/v1/llm/cache` | Delete all cache entries |
| `GET` | `/v1/llm/cache/stats` | Cache hit/miss statistics |
| `GET` | `/v1/llm/cache/costs` | Cost savings by provider |

**Example — Lookup cached response:**

```bash
curl -X POST http://localhost:8080/v1/llm/cache/lookup \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "What is the capital of France?",
    "model": "gpt-4",
    "similarity_threshold": 0.95
  }'
```

```json
{
  "success": true,
  "data": {
    "hit": true,
    "response": "The capital of France is Paris.",
    "similarity": 0.99,
    "saved_latency_ms": 850
  }
}
```

---

### AutoFE (Automated Feature Engineering)

**Maturity:** beta

Automated generation and ranking of candidate features.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/autofe/generate` | Generate candidate features |
| `GET` | `/v1/autofe/candidates` | List all candidates |
| `GET` | `/v1/autofe/candidates/top` | Get top candidates by score |
| `GET` | `/v1/autofe/stats` | Generation statistics |

**Example — Generate candidates:**

```bash
curl -X POST http://localhost:8080/v1/autofe/generate \
  -H "Content-Type: application/json" \
  -d '{
    "entity_type": "user",
    "source_features": ["click_count", "purchase_total", "session_duration"],
    "max_candidates": 20
  }'
```

```json
{
  "success": true,
  "data": {
    "candidates": [
      {
        "name": "click_count_rolling_avg_7d",
        "expression": "avg(click_count, 7d)",
        "score": 0.92
      }
    ],
    "total": 15
  }
}
```

---

### Geo-Routing

**Maturity:** beta

Multi-cloud geo-routing with data residency compliance.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/georouting/regions` | List registered regions |
| `POST` | `/v1/georouting/regions` | Add a cloud region |
| `DELETE` | `/v1/georouting/regions/{id}` | Remove a region |
| `GET` | `/v1/georouting/route?entity=X` | Route request to best region |
| `GET` | `/v1/georouting/stats` | Routing statistics |
| `GET` | `/v1/georouting/metrics` | Latency and throughput metrics |

**Example — Route a request:**

```bash
curl "http://localhost:8080/v1/georouting/route?entity=user:123"
```

```json
{
  "success": true,
  "data": {
    "entity": "user:123",
    "region": "eu-west-1",
    "latency_ms": 12,
    "reason": "data_residency"
  }
}
```

---

### A/B Rollout

**Maturity:** beta

Feature versioning with canary rollouts and quality gates.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/rollouts` | List rollouts |
| `POST` | `/v1/rollouts` | Start a canary rollout |
| `GET` | `/v1/rollouts/{id}` | Get rollout details |
| `POST` | `/v1/rollouts/{id}/advance` | Advance to next traffic step |
| `POST` | `/v1/rollouts/{id}/pause` | Pause a rollout |
| `POST` | `/v1/rollouts/{id}/rollback` | Rollback to base version |
| `GET` | `/v1/rollouts/{id}/quality` | Evaluate quality gates |
| `GET` | `/v1/rollouts/resolve?feature=X&entity=Y` | Resolve version for entity |
| `GET` | `/v1/rollouts/stats` | Rollout statistics |

**Example — Start a canary rollout:**

```bash
curl -X POST http://localhost:8080/v1/rollouts \
  -H "Content-Type: application/json" \
  -d '{
    "feature": "user_click_count",
    "base_version": "v1",
    "canary_version": "v2",
    "traffic_steps": [5, 25, 50, 100]
  }'
```

```json
{
  "success": true,
  "data": {
    "id": "rollout_xyz",
    "status": "active",
    "current_step": 0,
    "canary_traffic_pct": 5
  }
}
```

---

### Edge Runtime

**Maturity:** beta

Lightweight edge runtime with offline-first sync and WASM module deployment.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/edge/devices` | List edge devices |
| `POST` | `/v1/edge/devices` | Register a new device |
| `GET` | `/v1/edge/devices/{id}` | Get device details |
| `GET` | `/v1/edge/devices/{id}/stats` | Get device statistics |
| `GET` | `/v1/edge/devices/{id}/pending` | Get pending sync items |
| `POST` | `/v1/edge/devices/{id}/sync` | Trigger sync for device |
| `POST` | `/v1/edge/devices/{id}/heartbeat` | Device heartbeat |
| `POST` | `/v1/edge/devices/{id}/deploy/{moduleId}` | Deploy module to device |
| `GET` | `/v1/edge/modules` | List edge modules |
| `POST` | `/v1/edge/modules` | Register a module |
| `GET` | `/v1/edge/modules/{id}` | Get module details |
| `DELETE` | `/v1/edge/modules/{id}` | Remove a module |
| `GET` | `/v1/edge/stats` | Edge fleet statistics |

**Example — Trigger device sync:**

```bash
curl -X POST http://localhost:8080/v1/edge/devices/device-001/sync
```

```json
{
  "success": true,
  "data": {
    "device_id": "device-001",
    "synced_features": 142,
    "sync_duration_ms": 230
  }
}
```

---

### Additional Extension Handlers

The following handlers are registered but not documented in detail here. Enable them via `EnabledFeatures` and run `make api-routes` to see their maturity levels.

#### Stable Handlers

| Handler | Route Prefix | Description |
|---------|-------------|-------------|
| `groups` | `/v1/schema/groups` | Feature group management |
| `backfill` | `/v1/backfill/` | Historical data backfill jobs |
| `streaming` | `/v1/streaming/` | Real-time streaming pipelines |
| `catalog` | `/v1/catalog/` | Feature catalog and discovery |
| `auth` | `/v1/auth/` | API key management and RBAC |
| `ml` | `/v1/models/` | ML model serving and inference |
| `transform` | `/v1/transforms/` | Feature transformation pipelines |
| `cache` | `/v1/cache/` | Cache management and warming |
| `consistency` | `/v1/consistency/` | Data consistency checks |
| `observability` | `/v1/observability/` | Metrics and tracing configuration |
| `benchmark` | `/v1/benchmarks/` | Performance benchmarking |
| `impact` | `/v1/impact/` | Feature impact analysis |
| `model_serving` | `/v1/models/` | Model deployment and serving |
| `governance` | `/v1/governance/` | Data governance and compliance |
| `freshness` | `/v1/freshness/` | Feature freshness SLAs |
| `sla` | `/v1/sla/` | SLA management and monitoring |
| `drift` | `/v1/drift/` | Statistical drift detection |
| `semantic` | `/v1/semantic/` | Semantic search for features |
| `quality` | `/v1/quality/` | Data quality validation |

#### Beta Handlers

| Handler | Route Prefix | Description |
|---------|-------------|-------------|
| `tenant` | `/v1/tenants/` | Multi-tenant isolation |
| `warehouse` | `/v1/warehouse/` | Data warehouse sync |
| `embedding` | `/v1/embeddings/` | Embedding generation |
| `composition` | `/v1/composition/` | Feature composition DAGs |
| `migration` | `/v1/migrations/` | Schema migration management |
| `saas` | `/v1/saas/` | SaaS provisioning and billing |
| `cost` | `/v1/cost/` | Cost tracking and budgets |
| `scheduler` | `/v1/scheduler/` | Job scheduling (cron) |
| `lineage` | `/v1/lineage/` | Feature lineage tracking |
| `federation` | `/v1/federation/` | Distributed federation |
| `experiment` | `/v1/experiments/` | A/B experiment management |
| `dbt` | `/v1/dbt/` | dbt integration |
| `compute` | `/v1/compute/` | Compute engine |
| `consensus` | `/v1/consensus/` | Raft consensus |
| `stream_sql` | `/v1/stream-sql/` | Stream SQL processing |
| `control_plane` | `/v1/controlplane/` | Control plane management |
| `versioning` | `/v1/versioning/` | Feature versioning (branches, tags) |
| `validation` | `/v1/validation/` | Schema validation rules |
| `billing` | `/v1/billing/` | Usage billing |
| `contracts` | `/v1/contracts/` | Feature contracts |
| `materialization` | `/v1/materialization/` | Feature materialization |
| `replication` | `/v1/replication/` | Data replication |

#### Experimental Handlers

| Handler | Route Prefix | Description |
|---------|-------------|-------------|
| `graphql` | `/v1/graphql` | GraphQL API |
| `wasm` | `/v1/wasm/` | WASM runtime for transforms |
| `rag` | `/v1/rag/` | RAG pipeline |
| `plugin` | `/v1/plugins/` | Plugin system |
| `playground` | `/v1/playground/` | API playground |
| `gitops` | `/v1/gitops/` | GitOps schema management |
| `time_travel` | `/v1/timetravel/` | Time travel debugging |
| `llm_gateway` | `/v1/llm/gateway/` | LLM gateway routing |
| `compute_graph` | `/v1/compute-graph/` | Compute graph engine |

> **Tip:** Run `make api-routes` for a complete list of all handlers with maturity levels and enabled status.

---

