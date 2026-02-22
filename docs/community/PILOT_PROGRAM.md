# Feather Early Adopter Pilot Program

## Overview

We're looking for 3-5 teams to pilot Feather in real ML production environments. Pilot partners get direct support from the core team and help shape the product roadmap.

## What You Get

- **Direct engineering support** — weekly syncs with the core maintainer
- **Priority bug fixes** — issues from pilot partners are triaged first
- **Custom integrations** — help building connectors for your specific data stack
- **Public credit** — featured as a case study on the website and in talks (if desired)
- **Roadmap influence** — your use case drives feature prioritization

## What We're Looking For

### Ideal Pilot Partner
- ML team serving features in real-time (fraud detection, recommendations, personalization)
- Currently using Feast, a custom solution, or evaluating feature stores
- Willing to run Feather alongside existing solution in shadow mode for 4-8 weeks
- Can share anonymized performance metrics (latency, throughput) for a case study

### Technical Requirements
- Kubernetes cluster or Docker-capable infrastructure
- Go 1.22+ or Docker for deployment
- HTTP/gRPC client capability in your ML serving stack

## Pilot Timeline

| Week | Activity |
|------|----------|
| 0 | Kickoff call, architecture review, deployment plan |
| 1-2 | Shadow deployment alongside existing solution |
| 3-4 | Feature parity validation, schema migration |
| 5-6 | Production traffic (canary, 5-10%) |
| 7-8 | Full evaluation, benchmark comparison, case study draft |

## How to Apply

Send an email to **pilots@feather-store.io** with:

1. **Your company** (name, size, industry)
2. **Your ML use case** (what features do you serve? what latency do you need?)
3. **Current setup** (Feast? Custom? What infra?)
4. **Timeline** (when could you start?)
5. **Team size** (who would be involved?)

Or open a [GitHub Discussion](https://github.com/feather-store/feather/discussions) tagged `pilot-program`.

## FAQ

**Q: Is there a cost?**
A: No. The pilot is free. Feather is open-source (Apache 2.0).

**Q: Do we need to go to production?**
A: No. Shadow mode (dual-read comparison) is the primary goal.

**Q: Can we keep our data private?**
A: Yes. We only ask for anonymized latency/throughput metrics for the case study.

**Q: What if Feather doesn't work for our use case?**
A: That's valuable feedback too. We'll help you evaluate and document what didn't work.
