# Conference Talk: Sub-Microsecond Feature Serving in Go

## CFP Abstracts

### KubeCon NA 2026

**Title:** From 50ms to 0.2µs: Building a Cloud-Native Feature Store in Go

**Abstract (300 words):**

Feature stores are critical infrastructure for production machine learning, but existing solutions force teams to choose between Python's flexibility (Feast) and Java's performance (Hopsworks), while managed solutions (Tecton) lock teams into expensive SaaS contracts.

We built Feather, an open-source feature store in Go that serves ML features with sub-microsecond P99 latency from a single binary — no JVM, no Python runtime, no external dependencies. In this talk, we'll share the architectural decisions that make this possible:

1. **Tiered Storage Architecture**: A 256-shard LRU hot tier delivers 207ns point lookups, while BadgerDB provides warm-tier persistence. We'll explain how async warm-tier writes with backpressure give us both speed and durability.

2. **Pluggable Handler Pattern**: With 114 feature handlers spanning drift detection, anomaly monitoring, LLM prompt management, and more — all registered via a factory pattern with maturity levels — we'll show how Go interfaces enable massive extensibility without abstraction overhead.

3. **Cloud-Native Operations**: Prometheus metrics, OpenTelemetry tracing, Kubernetes CRDs, and Terraform provider — we'll demo the full operational stack from kubectl apply to Grafana dashboard.

Live demo: We'll deploy Feather to a Kind cluster, ingest 1M features, serve them at <1ms P99, detect drift in real-time, and trigger a webhook alert — all in under 10 minutes.

Attendees will learn: how to architect high-performance Go services for ML infrastructure, when tiered storage beats cache-aside patterns, and how pluggable handler registries enable "one binary, 114 features" without code bloat.

**Track:** Cloud Native Application Runtime
**Session Type:** Session (35 min)
**Level:** Intermediate

---

### MLOps Community Meetup

**Title:** Why We Rewrote Our Feature Store in Go (And Got 100x Faster)

**Abstract (150 words):**

Every ML team needs a feature store. Most use Feast (Python, flexible, slow) or Tecton (managed, fast, expensive). We wanted both: fast AND self-hosted.

Feather is an open-source feature store written in Go that achieves sub-microsecond latency for feature lookups — 100x faster than Feast's Redis-backed serving — from a single 18MB binary.

In this talk, I'll share:
- Why Go beats Python for ML infrastructure (hint: it's not just speed)
- The tiered storage trick that eliminates cache miss penalties
- How we serve 18M features/second with zero-allocation writes
- Building a Feast-compatible API that lets teams migrate without code changes

I'll do a live demo showing real-time fraud detection features served at <1ms from a laptop.

**Duration:** 20 minutes
**Level:** All levels

---

## Talk Outline (35-minute version)

### Part 1: The Problem (5 min)
- Feature stores 101 — what they are, why ML teams need them
- The impossible triad: fast vs flexible vs free
- Our journey: from Feast + Redis to "can we do better?"

### Part 2: Architecture Deep Dive (12 min)
- Why Go: goroutines, zero-cost abstractions, single binary
- Tiered storage: hot (LRU, 207ns) → warm (BadgerDB, 4µs) → offline (Parquet)
- The 256-shard design: why we don't use sync.Map
- Async warm writes with backpressure and drain-on-shutdown
- Zero-allocation hot path: how we achieved 0 allocs/op on Put

### Part 3: Extensibility at Scale (8 min)
- The handler registry pattern: init() + factory + maturity levels
- From 0 to 114 handlers: how interfaces keep things clean
- Demo: enabling/disabling features with a config map
- Maturity tiers: stable → beta → experimental

### Part 4: Live Demo (8 min)
1. `make run-dev` — start Feather (2 seconds)
2. Store 100K features via batch API
3. Retrieve features at sub-ms latency
4. Open Grafana dashboard, show latency heatmap
5. Trigger drift detection, show webhook alert
6. Run FeatherQL query: `SELECT avg(amount) OVER (WINDOW '1h') FROM transactions`

### Part 5: What's Next (2 min)
- Community growth: from 1 to 3+ maintainers
- Production hardening priorities
- CNCF sandbox submission
- Call to action: try it, star it, contribute

## Speaker Bio

Jose David Baena is a software engineer specializing in ML infrastructure and high-performance systems. He built Feather, an open-source feature store serving ML features at sub-microsecond latency, to bridge the gap between Python's flexibility and enterprise performance requirements. He is passionate about Go, real-time systems, and making ML infrastructure accessible to every team.
