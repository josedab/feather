# Feather: Open-Core Commercial Strategy

## Model: Open Source Core + Enterprise Features

Feather follows the proven open-core model: the core feature store engine is free and open-source (Apache 2.0), while enterprise features, hosted services, and support are offered commercially.

## What's Free (Apache 2.0, forever)

Everything in the current repository:
- Core feature store engine (tiered storage, serving, aggregation)
- All 114 handler extensions
- All 7 SDK languages
- CLI tools, Docker images, Helm charts
- Community support via GitHub Issues and Discussions

## Enterprise Tier

### Feather Enterprise (Self-Hosted)

For teams that need enterprise features on their own infrastructure.

| Feature | Community (Free) | Enterprise |
|---------|-----------------|------------|
| Core feature store | ✅ | ✅ |
| All extensions | ✅ | ✅ |
| Multi-region replication | ✅ (manual) | ✅ (automated) |
| SSO / SAML integration | ❌ | ✅ |
| Audit log export to SIEM | Basic (JSON) | ✅ (Splunk, Datadog, ELK) |
| SLA guarantees | ❌ | 99.9% uptime |
| Priority support | Community | 4-hour response SLA |
| Security advisories | Public | Early notification |
| Compliance certifications | ❌ | SOC2 Type II, HIPAA BAA |
| Custom integrations | ❌ | ✅ |
| Training & onboarding | Docs only | Dedicated engineer |

**Pricing:** Contact sales (estimated $2,000-10,000/month based on cluster size)

### Feather Cloud (Managed)

For teams that want zero-ops feature serving.

| Feature | Starter | Pro | Enterprise |
|---------|---------|-----|------------|
| **Instances** | 1 | 5 | Unlimited |
| **Features** | 10K | 100K | Unlimited |
| **Requests/sec** | 1K | 10K | 100K+ |
| **Storage** | 10 GB | 100 GB | 1 TB+ |
| **Regions** | 1 | 3 | Global |
| **Support** | Email | Priority | Dedicated |
| **SLA** | 99.5% | 99.9% | 99.99% |
| **SSO** | ❌ | ✅ | ✅ |
| **Audit logs** | 7 days | 90 days | 1 year |
| **Price** | $49/mo | $299/mo | Custom |

### Professional Services

| Service | Description | Pricing |
|---------|-------------|---------|
| Migration assistance | Feast/Tecton → Feather migration | $5K one-time |
| Architecture review | Production deployment planning | $3K one-time |
| Custom extension development | Build custom handlers for your use case | $200/hr |
| Training workshop | 2-day hands-on training for your team | $5K per session |

## Competitive Pricing

| Competitor | Model | Annual Cost |
|-----------|-------|------------|
| Feast | Free OSS | $0 (infra costs only) |
| Tecton | Managed SaaS | $50K-200K+ |
| Hopsworks | Hybrid | $30K-150K+ |
| **Feather Cloud Pro** | **Managed** | **$3,588** |
| **Feather Enterprise** | **Self-hosted** | **$24K-120K** |

**Positioning**: 10x cheaper than Tecton, 10x faster than Feast, self-hosted option that Tecton doesn't offer.

## Go-to-Market Phases

1. **Months 1-3**: Open-source launch, community building, benchmark publishing
2. **Months 4-6**: Beta cloud service, 3-5 pilot customers, first case study
3. **Months 7-12**: GA cloud service, enterprise tier, first paying customers
4. **Year 2**: Scale to 20+ enterprise customers, CNCF graduated status
