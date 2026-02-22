# Maintainers

Feather is maintained by the following people. We welcome new maintainers — see [Contributing Roles](#contributing-roles) below.

## Core Maintainers

| Name | GitHub | Focus Area | Since |
|------|--------|------------|-------|
| Jose David Baena | [@josedab](https://github.com/josedab) | Architecture, Core | 2024 |

## Contributing Roles

We recognize contributions at multiple levels:

### 🟢 Contributor
- Submits PRs that get merged
- Listed in GitHub contributors
- No special permissions needed

### 🔵 Reviewer
- Reviews PRs in a specific area (e.g., storage, extensions, SDKs)
- Added to CODEOWNERS for their area
- Requirements: 3+ merged PRs in the area

### 🟣 Maintainer
- Merge access, release authority, roadmap input
- Public credit in README and all release notes
- Requirements: 5+ merged PRs, demonstrated domain expertise, alignment with project values

## Becoming a Maintainer

We are actively seeking co-maintainers for the following areas:

| Area | Packages | Skills Needed |
|------|----------|---------------|
| **Storage Engine** | `internal/core/storage/` | Go performance, caching, databases |
| **Extensions** | `internal/extensions/` | Go, domain expertise per extension |
| **SDKs** | `sdk/` | Python, TypeScript, Java, Rust, etc. |
| **Documentation** | `docs/` | Technical writing, tutorials |
| **DevOps** | `deploy/`, `.github/` | Docker, K8s, CI/CD, Terraform |
| **Integrations** | `internal/integrations/` | Kafka, Spark, dbt, Airflow |

### How to Apply

1. Start contributing: pick a [Good First Issue](https://github.com/feather-store/feather/labels/good%20first%20issue)
2. Build expertise: submit 3-5 PRs in your area of interest
3. Express interest: open a Discussion titled "Maintainer Application: [Your Name]"
4. Interview: We'll have a brief conversation about the project direction

### What Maintainers Get

- Commit access to the repository
- Input on the project roadmap and architecture decisions
- Co-authorship credit on blog posts and conference talks
- Direct collaboration with the core team
- Experience maintaining a high-visibility open-source project

## Decision Making

- **Routine changes** (bug fixes, minor features): Single maintainer approval
- **Significant changes** (new extensions, API changes): Two maintainer approvals
- **Architectural changes** (storage engine, protocol): RFC + all maintainer consensus
- **Breaking changes**: RFC + all maintainer consensus + deprecation period

## Code of Conduct

All maintainers uphold the project's [Code of Conduct](CODE_OF_CONDUCT.md).
