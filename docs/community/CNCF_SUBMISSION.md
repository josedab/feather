# CNCF Sandbox Submission — Feather

## Project Description

**Feather** is a high-performance, real-time feature store written in Go. It provides sub-millisecond feature retrieval through a tiered storage architecture (hot LRU cache + warm BadgerDB), real-time sliding-window aggregations, and multiple serving APIs (gRPC, HTTP REST). Feather is designed for ML engineering teams that need consistent, low-latency access to feature data across training and serving environments.

Key capabilities:
- **Sub-millisecond feature serving** via in-memory hot tier with automatic warm-tier spill
- **114 API handlers** covering core serving, vector search, drift detection, marketplace, FeatherQL, geo-routing, A/B rollouts, edge runtime, and more
- **7 SDK skeletons** (Go, Python, TypeScript, Java, Ruby, Rust, .NET)
- **GenAI-native** with LLM response caching and HNSW vector similarity search
- **Feast compatibility** for drop-in migration from existing pipelines
- **Single static binary** with zero external runtime dependencies

## Alignment with CNCF

Feather aligns with the Cloud Native Computing Foundation's mission across multiple dimensions:

| CNCF Principle | Feather Implementation |
|----------------|----------------------|
| **Cloud-native architecture** | Stateless HTTP/gRPC servers; horizontally scalable; 12-factor configuration via env vars and YAML |
| **Kubernetes-ready** | Health (`/health`), readiness (`/ready`), and liveness (`/live`) probes; Helm chart and Kubernetes manifests in `deploy/` |
| **Observability** | Native Prometheus metrics endpoint (port 9090); OpenTelemetry tracing with OTLP export; structured logging via `slog` |
| **Interoperability** | gRPC + REST APIs; Feast-compatible endpoints; Kafka ingestion; dbt/Spark/Flink integration stubs |
| **Security** | TLS support; API key authentication; rate limiting; RBAC-ready platform layer |
| **Containerised** | Multi-stage Dockerfile; Docker Compose for dev and production; minimal final image |

## License

✅ **Apache License 2.0** — fully compliant with CNCF requirements.

## Current Maturity

| Dimension | Status |
|-----------|--------|
| Production deployments | Early adopters in staging/pre-prod |
| Test coverage | Unit + integration tests; race-detector CI |
| Documentation | README, CLAUDE.md (AI-assisted dev guide), API docs, package guide |
| Community | Open source from day one; CONTRIBUTING.md, CODE_OF_CONDUCT.md, SECURITY.md |
| Releases | Semantic versioning; changelog maintained |
| CI/CD | GitHub Actions (lint, test, build, Docker) |

## Sponsors Needed

CNCF Sandbox projects require **two sponsors** from the CNCF Technical Oversight Committee (TOC) or from CNCF member organisations.

If you are interested in sponsoring Feather's Sandbox application, please reach out:

- **GitHub:** [github.com/feather-store/feather](https://github.com/feather-store/feather)
- **Email:** maintainers@feather.dev

### What Sponsors Do

1. Review the project against CNCF Sandbox criteria.
2. Present the project to the TOC for a vote.
3. Provide feedback on governance, security, and community readiness.

## Links

| Resource | URL |
|----------|-----|
| Repository | https://github.com/feather-store/feather |
| Documentation | https://feather.dev/docs |
| Roadmap | https://github.com/feather-store/feather/blob/main/docs/ROADMAP.md |
| Contributing Guide | https://github.com/feather-store/feather/blob/main/CONTRIBUTING.md |
| Code of Conduct | https://github.com/feather-store/feather/blob/main/CODE_OF_CONDUCT.md |
| Security Policy | https://github.com/feather-store/feather/blob/main/SECURITY.md |
| License | https://github.com/feather-store/feather/blob/main/LICENSE |
| Community (Discord) | https://discord.gg/feather |

---

*This document follows the [CNCF Sandbox application template](https://github.com/cncf/toc/blob/main/process/sandbox.md).*
