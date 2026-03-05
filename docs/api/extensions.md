# Extension APIs

## Extension APIs

Extension handlers are enabled via the `EnabledFeatures` map in `cmd/feather/main.go`. Run `make api-routes` to see all registered handlers with maturity levels. All extension endpoints use the standard JSON response envelope and error format described in [Error Handling](#error-handling).

> **Maturity levels**: **stable** = production-ready, **beta** = functional but API may change, **experimental** = may be incomplete.

### Sharding & Replication

**Maturity:** stable

> **⚠️ Not yet implemented.** The sharding handler is registered but currently returns `nil` (see `features_core.go`). These endpoints are not reachable and will return 404. The `internal/extensions/sharding` package exists but is not wired into the handler. This section documents the planned API.

Distributed sharding with consistent hashing for horizontal scaling.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/sharding/stats` | Get shard routing statistics |
| `GET` | `/v1/sharding/partition?key=X` | Get partition for a key |
| `GET` | `/v1/sharding/owners?key=X` | Get replica owners for a key |
| `POST` | `/v1/sharding/recompute` | Recompute partition map |

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

**Example — Search the marketplace:**

```bash
curl "http://localhost:8080/v1/marketplace/search?q=click+engagement"
```

```json
{
  "success": true,
  "data": {
    "results": [
      {
        "id": "feat_abc123",
        "name": "user_click_count",
        "description": "Hourly click count per user",
        "owner": "ml-team",
        "subscribers": 12
      }
    ],
    "total": 1
  }
}
```

**Example — Subscribe to a feature:**

```bash
curl -X POST http://localhost:8080/v1/marketplace/features/feat_abc123/subscribe
```

```json
{
  "success": true,
  "data": {
    "feature_id": "feat_abc123",
    "subscribed": true
  }
}
```

**Example — Get marketplace statistics:**

```bash
curl http://localhost:8080/v1/marketplace/stats
```

```json
{
  "success": true,
  "data": {
    "total_features": 142,
    "total_subscribers": 85,
    "active_publishers": 12
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

```json
{
  "success": true,
  "data": {
    "id": "inst_xyz789",
    "name": "prod-us-east",
    "status": "provisioning",
    "tier": "standard",
    "region": "us-east-1",
    "replicas": 3
  }
}
```

**Example — Scale an instance:**

```bash
curl -X POST http://localhost:8080/v1/cloud/instances/inst_xyz789/scale \
  -H "Content-Type: application/json" \
  -d '{"replicas": 5}'
```

```json
{
  "success": true,
  "data": {
    "id": "inst_xyz789",
    "replicas": 5,
    "status": "scaling"
  }
}
```

**Example — List instances:**

```bash
curl http://localhost:8080/v1/cloud/instances
```

```json
{
  "success": true,
  "data": [
    {
      "id": "inst_xyz789",
      "name": "prod-us-east",
      "status": "running",
      "tier": "standard",
      "region": "us-east-1",
      "replicas": 3
    }
  ]
}
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

### Semantic Search

**Maturity:** stable

Semantic feature discovery with TF-IDF search, hybrid ranking, similarity suggestions, model recommendations, and explainability.

#### Search

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/semantic/search` | Search features by natural language query |
| `GET` | `/v1/semantic/search?q=X` | Search features via query parameter |

**POST `/v1/semantic/search`** — Search features by semantic similarity.

```bash
curl -X POST http://localhost:8080/v1/semantic/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "user click behavior",
    "limit": 5,
    "min_score": 0.3,
    "categories": ["engagement"],
    "tags": ["real-time"],
    "owner": "ml-team"
  }'
```

```json
{
  "results": [
    {
      "feature": {
        "id": "user_click_count",
        "name": "user_click_count",
        "description": "Hourly click count per user",
        "tags": ["engagement", "real-time"],
        "category": "engagement",
        "data_type": "int64",
        "owner": "ml-team"
      },
      "score": 0.92,
      "similarity": 0.88
    }
  ],
  "count": 1,
  "query": "user click behavior"
}
```

**GET `/v1/semantic/search`** — Query parameters: `q` (required), `limit`, `min_score`, `category` (repeatable), `tag` (repeatable), `owner`.

```bash
curl "http://localhost:8080/v1/semantic/search?q=click+behavior&limit=5&min_score=0.5"
```

#### Feature CRUD

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/semantic/features` | List all indexed features |
| `POST` | `/v1/semantic/features` | Index a feature with metadata |
| `POST` | `/v1/semantic/features/batch` | Batch index multiple features |
| `GET` | `/v1/semantic/features/{id}` | Get a feature by ID |
| `DELETE` | `/v1/semantic/features/{id}` | Delete a feature from the index |

**POST `/v1/semantic/features`** — Index a feature with enhanced metadata.

```bash
curl -X POST http://localhost:8080/v1/semantic/features \
  -H "Content-Type: application/json" \
  -d '{
    "id": "user_click_count",
    "name": "user_click_count",
    "description": "Hourly click count per user",
    "category": "engagement",
    "tags": ["engagement", "real-time"],
    "data_type": "int64",
    "entity_type": "user",
    "domain": "marketing",
    "owner": "ml-team",
    "team": "data-eng",
    "quality_score": 0.95
  }'
```

```json
{
  "success": true,
  "feature_id": "user_click_count"
}
```

**POST `/v1/semantic/features/batch`** — Batch index. Request body is a JSON array of feature objects (same schema as above). Response:

```json
{
  "success": true,
  "indexed": 3,
  "total": 4,
  "errors": ["skipped feature with missing id or name"]
}
```

#### Enriched Metadata

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/semantic/features/{id}/enriched` | Get enriched feature with all metadata |
| `GET` | `/v1/semantic/features/{id}/metadata` | Get feature metadata |
| `PUT` | `/v1/semantic/features/{id}/metadata` | Update feature metadata |
| `PUT` | `/v1/semantic/features/{id}/statistics` | Set feature statistics |
| `PUT` | `/v1/semantic/features/{id}/lineage` | Set feature lineage |
| `PUT` | `/v1/semantic/features/{id}/usage` | Set feature usage metrics |

**PUT `/v1/semantic/features/{id}/statistics`** — Set statistical summary for a feature.

```bash
curl -X PUT http://localhost:8080/v1/semantic/features/user_click_count/statistics \
  -H "Content-Type: application/json" \
  -d '{
    "mean": 42.5,
    "stddev": 12.3,
    "min": 0,
    "max": 500,
    "null_percentage": 0.02
  }'
```

**PUT `/v1/semantic/features/{id}/lineage`** — Set data lineage (sources, transformations).

```bash
curl -X PUT http://localhost:8080/v1/semantic/features/user_click_count/lineage \
  -H "Content-Type: application/json" \
  -d '{
    "sources": ["clickstream_raw"],
    "transformations": ["count", "window_1h"]
  }'
```

#### Ranking & Suggestions

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/semantic/rank` | Hybrid ranking with multi-signal scoring |
| `GET` | `/v1/semantic/suggest/{id}?limit=N` | Get similar feature suggestions |
| `POST` | `/v1/semantic/suggest` | Get suggestions via request body |
| `POST` | `/v1/semantic/recommend` | Recommend features for a model |

**POST `/v1/semantic/rank`** — Rank features using hybrid scoring (semantic + quality + freshness).

```bash
curl -X POST http://localhost:8080/v1/semantic/rank \
  -H "Content-Type: application/json" \
  -d '{
    "query": "user engagement metrics",
    "domains": ["marketing"],
    "min_quality": 0.7,
    "only_fresh": true,
    "limit": 10
  }'
```

**POST `/v1/semantic/recommend`** — Recommend complementary features for a model.

```bash
curl -X POST http://localhost:8080/v1/semantic/recommend \
  -H "Content-Type: application/json" \
  -d '{
    "existing_features": ["user_click_count", "session_duration"],
    "model_use_case": "churn_prediction",
    "limit": 5
  }'
```

#### Explainability

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/semantic/explain` | Explain why a feature matched a query |
| `POST` | `/v1/semantic/explain/batch` | Batch explain multiple search results |

**POST `/v1/semantic/explain`** — Explain a search result score.

```bash
curl -X POST http://localhost:8080/v1/semantic/explain \
  -H "Content-Type: application/json" \
  -d '{
    "feature_id": "user_click_count",
    "query": "user engagement",
    "score": 0.92
  }'
```

**POST `/v1/semantic/explain/batch`** — Batch explanation request body:

```json
{
  "results": [
    {"feature_id": "user_click_count", "score": 0.92},
    {"feature_id": "session_duration", "score": 0.85}
  ],
  "query": "user engagement"
}
```

#### Discovery

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/semantic/discover/popular?limit=N` | Discover most popular features |
| `GET` | `/v1/semantic/discover/quality?min_score=N` | Discover high-quality features |
| `GET` | `/v1/semantic/discover/domain/{domain}` | Discover features by domain |
| `GET` | `/v1/semantic/discover/entity/{entityType}` | Discover features by entity type |
| `GET` | `/v1/semantic/discover/usecase/{usecase}` | Discover features by use case |

**Example — Discover popular features:**

```bash
curl "http://localhost:8080/v1/semantic/discover/popular?limit=5"
```

```json
{
  "features": [...],
  "count": 5
}
```

#### Stats

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/semantic/stats` | Get search index and indexer statistics |

```bash
curl http://localhost:8080/v1/semantic/stats
```

---

### Freshness SLA

**Maturity:** stable

Adaptive feature freshness monitoring with access/change tracking, TTL predictions, policy-based SLA management, and drift recording.

#### Metrics

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/freshness/metrics` | Get metrics for all tracked features |
| `GET` | `/v1/freshness/metrics/{feature}` | Get metrics for a specific feature |

**Example — Get feature metrics:**

```bash
curl http://localhost:8080/v1/freshness/metrics/user_click_count
```

```json
{
  "feature_name": "user_click_count",
  "access": {
    "total_accesses": 15234,
    "cache_hit_rate": 0.87,
    "avg_latency_ms": 2.3
  },
  "change": {
    "total_changes": 842,
    "avg_magnitude": 3.14
  }
}
```

#### TTL & Predictions

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/freshness/ttl/{feature}` | Get recommended TTL with reasoning |
| `GET` | `/v1/freshness/predictions` | Get predictions for all tracked features |
| `GET` | `/v1/freshness/predictions/{feature}` | Get staleness prediction for a feature |

**Example — Get TTL recommendation:**

```bash
curl http://localhost:8080/v1/freshness/ttl/user_click_count
```

#### SLA Policies

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/freshness/policies` | List all freshness policies |
| `POST` | `/v1/freshness/policies` | Create a new policy |
| `GET` | `/v1/freshness/policies/{id}` | Get policy by ID |
| `PUT` | `/v1/freshness/policies/{id}` | Update an existing policy |
| `DELETE` | `/v1/freshness/policies/{id}` | Delete a policy |

**POST `/v1/freshness/policies`** — Create a freshness SLA policy.

```bash
curl -X POST http://localhost:8080/v1/freshness/policies \
  -H "Content-Type: application/json" \
  -d '{
    "id": "realtime-features",
    "name": "Real-time Feature SLA",
    "type": "max_staleness",
    "feature_pattern": "user_*",
    "priority": 10,
    "enabled": true,
    "config": {
      "max_staleness": "5m",
      "alert_threshold": "3m"
    }
  }'
```

Error responses: `409 Conflict` if policy ID already exists, `400 Bad Request` if policy is invalid.

#### Event Recording

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/freshness/access` | Record a feature access event |
| `POST` | `/v1/freshness/change` | Record a feature value change |
| `POST` | `/v1/freshness/drift` | Record a drift score |
| `POST` | `/v1/freshness/stale` | Record a stale serve event |

**POST `/v1/freshness/access`** — Record an access event.

```bash
curl -X POST http://localhost:8080/v1/freshness/access \
  -H "Content-Type: application/json" \
  -d '{"feature": "user_click_count", "latency": 2500000, "cache_hit": true}'
```

**POST `/v1/freshness/change`** — Record a value change.

```bash
curl -X POST http://localhost:8080/v1/freshness/change \
  -H "Content-Type: application/json" \
  -d '{"feature": "user_click_count", "old_value": 41.0, "new_value": 42.0}'
```

**POST `/v1/freshness/drift`** — Record a drift score.

```bash
curl -X POST http://localhost:8080/v1/freshness/drift \
  -H "Content-Type: application/json" \
  -d '{"feature": "user_click_count", "drift_score": 0.15}'
```

**POST `/v1/freshness/stale`** — Record a stale serve.

```bash
curl -X POST http://localhost:8080/v1/freshness/stale \
  -H "Content-Type: application/json" \
  -d '{"feature": "user_click_count"}'
```

All recording endpoints return `202 Accepted` on success.

#### Stats & Evaluation

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/freshness/stats` | Get freshness manager statistics |
| `GET` | `/v1/freshness/evaluate` | Evaluate freshness for all tracked features |

```bash
curl http://localhost:8080/v1/freshness/stats
curl http://localhost:8080/v1/freshness/evaluate
```

---

### Additional Extension Handlers

The following handlers are registered but not documented in detail here. Enable them via `EnabledFeatures` and run `make api-routes` to see their maturity levels.

#### Stable Handlers

| Handler | Route Prefix | Maturity | Description |
|---------|-------------|----------|-------------|
| `groups` | `/v1/schema/groups` | stable | Feature group management |
| `backfill` | `/v1/backfill/` | stable | Historical data backfill jobs |
| `streaming` | `/v1/streaming/` | stable | Real-time streaming pipelines |
| `catalog` | `/v1/catalog/` | stable | Feature catalog and discovery |
| `auth` | `/v1/auth/` | stable | API key management and RBAC |
| `ml` | `/v1/models/` | stable | ML model serving and inference |
| `transform` | `/v1/transforms/` | stable | Feature transformation pipelines |
| `cache` | `/v1/cache/` | stable | Cache management and warming |
| `consistency` | `/v1/consistency/` | stable | Data consistency checks |
| `observability` | `/v1/observability/` | stable | Metrics and tracing configuration |
| `benchmark` | `/v1/benchmarks/` | stable | Performance benchmarking |
| `impact` | `/v1/impact/` | stable | Feature impact analysis |
| `model_serving` | `/v1/models/` | stable | Model deployment and serving |
| `governance` | `/v1/governance/` | stable | Data governance and compliance |
| `freshness` | `/v1/freshness/` | stable | Feature freshness SLAs (see [Freshness SLA](#freshness-sla)) |
| `sla` | `/v1/sla/` | stable | SLA management and monitoring |
| `drift` | `/v1/drift/` | stable | Statistical drift detection |
| `semantic` | `/v1/semantic/` | stable | Semantic search for features (see [Semantic Search](#semantic-search)) |
| `quality` | `/v1/quality/` | stable | Data quality validation |
| `feast_ga` | `/v1/feast/` | stable | Feast compatibility layer (GA) |

#### Beta Handlers

| Handler | Route Prefix | Maturity | Description |
|---------|-------------|----------|-------------|
| `tenant` | `/v1/tenants/` | beta | Multi-tenant isolation |
| `warehouse` | `/v1/warehouse/` | beta | Data warehouse sync |
| `embedding` | `/v1/embeddings/` | beta | Embedding generation |
| `composition` | `/v1/composition/` | beta | Feature composition DAGs |
| `migration` | `/v1/migrations/` | beta | Schema migration management |
| `saas` | `/v1/saas/` | beta | SaaS provisioning and billing |
| `cost` | `/v1/cost/` | beta | Cost tracking and budgets |
| `scheduler` | `/v1/scheduler/` | beta | Job scheduling (cron) |
| `lineage` | `/v1/lineage/` | beta | Feature lineage tracking |
| `federation` | `/v1/federation/` | beta | Distributed federation |
| `experiment` | `/v1/experiments/` | beta | A/B experiment management |
| `dbt` | `/v1/dbt/` | beta | dbt integration |
| `compute` | `/v1/compute/` | beta | Compute engine |
| `consensus` | `/v1/consensus/` | beta | Raft consensus |
| `cluster` | — | beta | ⚠️ Not implemented (handler returns nil) |
| `stream_sql` | `/v1/stream-sql/` | beta | Stream SQL processing |
| `control_plane` | `/v1/controlplane/` | beta | Control plane management |
| `versioning` | `/v1/versioning/` | beta | Feature versioning (branches, tags) |
| `validation` | `/v1/validation/` | beta | Schema validation rules |
| `billing` | `/v1/billing/` | beta | Usage billing |
| `contracts` | `/v1/contracts/` | beta | Feature contracts |
| `materialization` | `/v1/materialization/` | beta | Feature materialization |
| `replication` | `/v1/replication/` | beta | Data replication |
| `ab_features` | `/v1/ab/features/` | beta | A/B feature testing |
| `auto_sharding` | `/v1/autosharding/` | beta | Automatic shard management |
| `backfill_engine` | `/v1/backfill-engine/` | beta | Advanced backfill orchestration |
| `backpressure` | `/v1/backpressure/` | beta | Backpressure management |
| `bench_suite` | `/v1/bench-suite/` | beta | Extended benchmark suite |
| `cdc_materialization` | `/v1/cdc/` | beta | CDC-based materialization |
| `cloud_control` | `/v1/cloud-control/` | beta | Cloud control plane management |
| `consistency_validator` | `/v1/consistency-validator/` | beta | Consistency validation rules |
| `contract_cicd` | `/v1/contracts/cicd/` | beta | Contract CI/CD integration |
| `contract_test` | `/v1/contracts/test/` | beta | Contract testing |
| `dashboard_v2` | `/v1/dashboard/` | beta | Feature monitoring dashboard (v2) |
| `declarative_graph` | `/v1/declarative-graph/` | beta | Declarative computation graphs |
| `explorer` | `/v1/explorer/` | beta | Feature explorer UI backend |
| `feast_gateway` | `/v1/feast/gateway/` | beta | Feast-compatible gateway |
| `feature_dashboard` | `/v1/feature-dashboard/` | beta | Feature analytics dashboard |
| `featherql_v2` | `/v1/featherql/v2/` | beta | FeatherQL v2 engine |
| `finops` | `/v1/finops/` | beta | FinOps dashboards and chargeback |
| `flight_endpoint` | `/v1/flight/` | beta | Arrow Flight endpoint |
| `flink_pipeline` | `/v1/flink/` | beta | Apache Flink pipeline integration |
| `ftl` | `/v1/ftl/` | beta | Feature Transformation Language |
| `gitops_defs` | `/v1/gitops/defs/` | beta | GitOps schema definitions |
| `importance_scoring` | `/v1/importance/` | beta | Feature importance scoring |
| `incr_materialization` | `/v1/incr-materialization/` | beta | Incremental materialization |
| `k8s_autoscaler` | `/v1/autoscaler/` | beta | Kubernetes-aware autoscaling |
| `lineage_analysis` | `/v1/lineage/analysis/` | beta | Lineage impact analysis |
| `lineage_graph` | `/v1/lineage/graph/` | beta | Lineage graph visualization |
| `llm_store` | `/v1/llm/store/` | beta | LLM feature storage |
| `ml_integrations` | `/v1/ml/integrations/` | beta | ML framework integrations |
| `mobile_sync` | `/v1/mobile/sync/` | beta | Mobile SDK sync protocol |
| `model_registry` | `/v1/models/registry/` | beta | Model registry management |
| `monitoring` | `/v1/monitoring/` | beta | Unified monitoring and alerting |
| `multi_region` | `/v1/multi-region/` | beta | Multi-region deployment management |
| `multi_tenant_metering` | `/v1/metering/` | beta | Multi-tenant usage metering |
| `obs_console` | `/v1/obs-console/` | beta | Observability console |
| `offline_store_sync` | `/v1/offline/sync/` | beta | Offline store synchronization |
| `orchestrator` | `/v1/orchestrator/` | beta | Pipeline orchestration |
| `parity` | `/v1/parity/` | beta | Online/offline parity validation |
| `prompt_store` | `/v1/prompts/` | beta | LLM prompt management |
| `python_sidecar` | `/v1/python/sidecar/` | beta | Python sidecar transforms |
| `python_transforms` | `/v1/python/transforms/` | beta | Python transform pipelines |
| `quality_score` | `/v1/quality/score/` | beta | Automated quality scoring |
| `realtime_monitor` | `/v1/realtime-monitor/` | beta | Real-time monitoring |
| `region_federation` | `/v1/region-federation/` | beta | Cross-region federation |
| `saas_control` | `/v1/saas/control/` | beta | SaaS control plane |
| `schema_evolution` | `/v1/schema/evolution/` | beta | Schema evolution management |
| `sdk_codegen` | `/v1/sdk/codegen/` | beta | SDK code generation |
| `stream_compute` | `/v1/stream-compute/` | beta | Stream computation engine |

#### Experimental Handlers

| Handler | Route Prefix | Maturity | Description |
|---------|-------------|----------|-------------|
| `graphql` | `/v1/graphql` | experimental | GraphQL API |
| `wasm` | `/v1/wasm/` | experimental | WASM runtime for transforms |
| `rag` | `/v1/rag/` | experimental | RAG pipeline |
| `plugin` | `/v1/plugins/` | experimental | Plugin system |
| `playground` | `/v1/playground/` | experimental | API playground |
| `gitops` | `/v1/gitops/` | experimental | GitOps schema management |
| `time_travel` | `/v1/timetravel/` | experimental | Time travel debugging |
| `llm_gateway` | `/v1/llm/gateway/` | experimental | LLM gateway routing |
| `compute_graph` | `/v1/compute-graph/` | experimental | Compute graph engine |
| `adaptive_cache` | `/v1/adaptive-cache/` | experimental | ML-driven adaptive caching |
| `anomaly_detect` | `/v1/anomaly/` | experimental | Anomaly detection on features |
| `api_gateway` | `/v1/gateway/` | experimental | API gateway and routing |
| `arrow_batch` | `/v1/arrow/batch/` | experimental | Arrow batch data transport |
| `arrow_flight` | `/v1/arrow/flight/` | experimental | Arrow Flight data transport |
| `audit_log` | `/v1/audit/` | experimental | Audit logging |
| `audit_trail` | `/v1/audit/trail/` | experimental | Event-sourced audit trail |
| `autogen` | `/v1/autogen/` | experimental | Automatic feature generation |
| `bench_pub` | `/v1/bench-pub/` | experimental | Benchmark publishing |
| `catalog_ui` | `/v1/catalog-ui/` | experimental | Catalog UI service |
| `cloud_storage` | `/v1/cloud-storage/` | experimental | Cloud storage connectors |
| `compression` | `/v1/compression/` | experimental | Intelligent tiered compression |
| `compute_graph_v2` | `/v1/compute-graph/v2/` | experimental | Compute graph engine (v2) |
| `consistency_advanced` | `/v1/consistency/advanced/` | experimental | Advanced consistency checks |
| `diff_privacy` | `/v1/privacy/` | experimental | Differential privacy engine |
| `embedding_lifecycle` | `/v1/embeddings/lifecycle/` | experimental | Embedding lifecycle management |
| `embedding_mgmt` | `/v1/embeddings/mgmt/` | experimental | Embedding management |
| `feast_compat` | `/v1/feast/compat/` | experimental | Feast compatibility adapter |
| `feast_enhanced` | `/v1/feast/enhanced/` | experimental | Enhanced Feast compatibility |
| `feather_cli` | — | experimental | CLI tool handler |
| `feather_cloud` | `/v1/cloud/` | experimental | Cloud control plane |
| `fed_learning` | `/v1/federated-learning/` | experimental | Federated learning adapter |
| `federated_discovery` | `/v1/federated/discovery/` | experimental | Federated feature discovery |
| `federation_cross_org` | `/v1/federation/cross-org/` | experimental | Cross-organization federation |
| `gitops_manifests` | `/v1/gitops/manifests/` | experimental | GitOps manifest management |
| `lifecycle_manager` | `/v1/lifecycle/` | experimental | Feature lifecycle management |
| `llm_features` | `/v1/llm/features/` | experimental | LLM-specific feature types |
| `model_gateway` | `/v1/models/gateway/` | experimental | Model gateway routing |
| `nl_discovery` | `/v1/nl-discovery/` | experimental | Natural language feature discovery |
| `notebook_sdk` | `/v1/notebook/` | experimental | Jupyter/Colab integration |
| `offline_store` | `/v1/offline/` | experimental | Offline feature store |
| `openapi_sync` | `/v1/openapi/` | experimental | OpenAPI spec synchronization |
| `playground_v2` | `/v1/playground/v2/` | experimental | Enhanced playground (v2) |
| `predictive_warming` | `/v1/predictive-warming/` | experimental | Predictive cache warming |
| `prefetch` | `/v1/prefetch/` | experimental | ML-based feature pre-fetching |
| `pushdown` | `/v1/pushdown/` | experimental | Server-side expression evaluation |
| `python_runtime` | `/v1/python/runtime/` | experimental | Python runtime environment |
| `quality_gates` | `/v1/quality/gates/` | experimental | CI/CD quality gates |
| `query_planner` | `/v1/query-planner/` | experimental | Self-optimizing query planner |
| `sdk_languages` | `/v1/sdk/languages/` | experimental | Multi-language SDK support |
| `semantic_catalog` | `/v1/semantic/catalog/` | experimental | Semantic feature catalog |
| `skew_detect` | `/v1/skew/` | experimental | Online/offline skew detection |
| `smpc` | `/v1/smpc/` | experimental | Secure multi-party computation |
| `starlark_udf` | `/v1/starlark/` | experimental | Starlark UDF runtime |
| `stream_advanced` | `/v1/stream/advanced/` | experimental | Advanced stream processing |
| `stream_dsl` | `/v1/stream-dsl/` | experimental | Stream processing DSL |
| `terraform_provider` | `/v1/terraform/` | experimental | Terraform provider |
| `ui` | `/ui/` | experimental | Embedded web UI |
| `wasm_runtime` | `/v1/wasm/runtime/` | experimental | WASM runtime management |
| `wasm_udf` | `/v1/wasm/udf/` | experimental | WASM user-defined functions |
| `webhook_events` | `/v1/webhooks/` | experimental | Webhook event delivery |

> **Tip:** Run `make api-routes` for a complete list of all handlers with maturity levels and enabled status.

---

