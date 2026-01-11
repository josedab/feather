---
sidebar_position: 16
title: Dashboard & Web UI
description: Monitor, explore, and manage your feature store through Feather's web interface.
---

# Dashboard & Web UI

Monitor, explore, and manage your feature store through a web interface.

## Overview

The Feather Dashboard provides a web-based interface for:

- **Feature Catalog**: Browse and search all feature definitions
- **Monitoring**: Real-time metrics, drift alerts, and freshness status
- **Analytics**: Usage patterns, popular features, and access logs
- **Administration**: Manage feature groups, indexes, and configuration

### Quick Start

```bash
# Enable dashboard in configuration
cat >> configs/feather.yaml << EOF
ui:
  enabled: true
  port: 3000
EOF

# Start server
./bin/feather -config configs/feather.yaml

# Access dashboard
open http://localhost:3000
```

## Features

### Feature Catalog

Browse and search your feature definitions:

- **Search**: Full-text search across feature names and descriptions
- **Filter**: By group, entity type, data type, or tags
- **Details**: View schema, statistics, and usage information
- **Lineage**: See data sources and downstream dependencies

### Drift Monitoring

Visualize data drift and quality metrics:

- **Drift Scores**: KL divergence, JS divergence, PSI for each feature
- **Distribution Plots**: Compare reference vs current distributions
- **Trend Charts**: Drift evolution over time
- **Alerts**: Configurable thresholds with notifications

### Freshness Heatmaps

Monitor feature freshness across entities:

- **Heatmap View**: Color-coded freshness by feature and time
- **SLA Tracking**: Features meeting/missing freshness targets
- **Stale Detection**: Identify features that haven't updated
- **Entity Drill-down**: Investigate specific entity freshness

### Usage Analytics

Understand how features are being used:

- **Access Patterns**: Which features are most requested
- **Latency Distribution**: P50, P95, P99 by endpoint
- **Error Rates**: Track failures and timeouts
- **Entity Coverage**: Features available per entity type

### Alert Management

Configure and manage alerting:

- **Create Rules**: Define conditions and thresholds
- **View History**: Past alerts with resolution status
- **Acknowledge**: Mark alerts as acknowledged/resolved
- **Integrations**: Slack, PagerDuty, email notifications

## Installation

### Docker

```bash
# Run with dashboard enabled
docker run -d \
  --name feather \
  -p 8080:8080 \
  -p 3000:3000 \
  -e FEATHER_UI_ENABLED=true \
  feather:latest
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: feather
spec:
  template:
    spec:
      containers:
        - name: feather
          ports:
            - containerPort: 8080
              name: api
            - containerPort: 3000
              name: ui
          env:
            - name: FEATHER_UI_ENABLED
              value: "true"
---
apiVersion: v1
kind: Service
metadata:
  name: feather-ui
spec:
  ports:
    - port: 80
      targetPort: 3000
```

## Configuration

### Full Configuration

```yaml
ui:
  enabled: true
  port: 3000

  # Authentication
  auth:
    enabled: true
    provider: "oauth2"  # or "basic", "ldap", "saml"
    oauth2:
      client_id: "${OAUTH_CLIENT_ID}"
      client_secret: "${OAUTH_CLIENT_SECRET}"
      issuer_url: "https://auth.example.com"
      redirect_url: "http://localhost:3000/callback"
      scopes: ["openid", "profile", "email"]

  # Session management
  session:
    secret: "${SESSION_SECRET}"
    max_age: "24h"
    secure_cookie: true

  # Rate limiting
  rate_limit:
    enabled: true
    requests_per_minute: 100

  # Features
  features:
    catalog: true
    drift: true
    freshness: true
    analytics: true
    alerts: true
    admin: false  # Disable admin features

  # Branding
  branding:
    title: "Feather Feature Store"
    logo_url: "/assets/logo.svg"
    primary_color: "#1976D2"
    dark_mode: true
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `FEATHER_UI_ENABLED` | Enable dashboard | `false` |
| `FEATHER_UI_PORT` | Dashboard port | `3000` |
| `FEATHER_UI_AUTH_ENABLED` | Require authentication | `false` |
| `FEATHER_UI_ADMIN_ENABLED` | Enable admin features | `false` |
| `SESSION_SECRET` | Session encryption key | (random) |

## UI Components

### Navigation Structure

```
Dashboard
├── Home (Overview)
├── Catalog
│   ├── Feature Groups
│   ├── Features
│   └── Entity Types
├── Monitoring
│   ├── Metrics
│   ├── Drift Detection
│   └── Freshness
├── Analytics
│   ├── Usage
│   ├── Latency
│   └── Errors
├── Alerts
│   ├── Active
│   ├── History
│   └── Rules
└── Settings (Admin)
    ├── Configuration
    ├── Users
    └── Integrations
```

### Home Dashboard

The overview page displays key metrics:

```
┌─────────────────────────────────────────────────────────────────┐
│  FEATHER DASHBOARD                                    [User ▼]  │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   Features   │  │   Entities   │  │   Requests   │          │
│  │     152      │  │    5.2M      │  │   45K/sec    │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │  Cache Hit   │  │  P99 Latency │  │    Alerts    │          │
│  │    94.2%     │  │    0.8ms     │  │      3       │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│                                                                  │
│  Request Volume (24h)                                           │
│  ▁▂▃▄▅▆▇█▇▆▅▄▃▄▅▆▇█▇▆▅▄▃▂▁▂▃▄▅▆▇█▇▆▅▄▃▂▁▂▃▄▅▆▇█               │
│                                                                  │
│  Recent Alerts                              [View All →]        │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ ⚠ HIGH  purchase_total drift detected       2 hours ago    ││
│  │ ⚠ MED   click_count freshness SLA breach   4 hours ago    ││
│  │ ✓ LOW   user_segment validation warning    1 day ago       ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

### Feature Catalog View

Browse and search features:

```
┌─────────────────────────────────────────────────────────────────┐
│  FEATURE CATALOG                                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Search: [purchase behavior________________] [Filter ▼] [⚙]    │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Group: user-engagement                    [12 features]     ││
│  ├─────────────────────────────────────────────────────────────┤│
│  │  ◉ clicks_last_hour          int64     [sum] [1h window]   ││
│  │    Click count in the last hour                             ││
│  │                                                              ││
│  │  ◉ purchase_total_7d         float64   [sum] [7d window]   ││
│  │    Total purchase amount in the last 7 days                 ││
│  │                                                              ││
│  │  ◉ engagement_score          float64   computed            ││
│  │    Composite engagement metric                              ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Group: user-profile                       [8 features]      ││
│  ├─────────────────────────────────────────────────────────────┤│
│  │  ◉ age_bucket                string     categorical        ││
│  │  ◉ location_region           string     categorical        ││
│  │  ◉ user_embedding            vector     [384 dims]         ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

### Drift Monitoring View

```
┌─────────────────────────────────────────────────────────────────┐
│  DRIFT DETECTION                                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Feature: purchase_total_7d                 Status: ⚠ DRIFTING │
│                                                                  │
│  ┌───────────────────────────┐  ┌───────────────────────────┐  │
│  │ Reference Distribution    │  │ Current Distribution      │  │
│  │          ▁▃▅▇▅▃▁          │  │        ▁▂▅▇█▇▅▂▁         │  │
│  │  μ=125.50  σ=45.2        │  │  μ=168.75  σ=52.8        │  │
│  └───────────────────────────┘  └───────────────────────────┘  │
│                                                                  │
│  Metrics                                                         │
│  ┌─────────────────┬─────────────────┬─────────────────┐       │
│  │ KL Divergence   │ JS Divergence   │ PSI             │       │
│  │ 0.35            │ 0.18            │ 0.42            │       │
│  │ Threshold: 0.1  │ Threshold: 0.1  │ Threshold: 0.25 │       │
│  └─────────────────┴─────────────────┴─────────────────┘       │
│                                                                  │
│  Drift History (30 days)                                        │
│  ▁▁▁▁▁▁▁▂▂▂▃▃▄▄▅▅▆▆▇▇████████                                  │
│  Jan 1     Jan 8     Jan 15    Jan 22    Jan 29                 │
│                                                                  │
│  [Reset Reference] [Configure Alert] [View History]            │
└─────────────────────────────────────────────────────────────────┘
```

## API Backend

The dashboard backend exposes REST endpoints for UI data:

### Dashboard Endpoints

```
GET  /api/dashboard/overview         # Home page metrics
GET  /api/dashboard/stats            # Real-time statistics
GET  /api/dashboard/health           # System health status
```

### Catalog Endpoints

```
GET  /api/catalog/groups             # List feature groups
GET  /api/catalog/groups/{name}      # Feature group details
GET  /api/catalog/features           # Search features
GET  /api/catalog/features/{name}    # Feature details
GET  /api/catalog/entities           # Entity types
```

### Monitoring Endpoints

```
GET  /api/monitoring/metrics         # Prometheus metrics
GET  /api/monitoring/drift           # Drift status
GET  /api/monitoring/freshness       # Freshness heatmap
GET  /api/monitoring/latency         # Latency percentiles
```

### Analytics Endpoints

```
GET  /api/analytics/usage            # Feature usage stats
GET  /api/analytics/access           # Access patterns
GET  /api/analytics/errors           # Error breakdown
```

### Alert Endpoints

```
GET  /api/alerts                     # List alerts
GET  /api/alerts/{id}                # Alert details
POST /api/alerts/{id}/acknowledge    # Acknowledge alert
POST /api/alerts/rules               # Create alert rule
```

## Authentication

### OAuth2 / OIDC

```yaml
auth:
  provider: oauth2
  oauth2:
    client_id: "feather-dashboard"
    client_secret: "${OAUTH_CLIENT_SECRET}"
    issuer_url: "https://auth.example.com"
    redirect_url: "https://feather.example.com/callback"
    scopes: ["openid", "profile", "email", "groups"]

    # Role mapping
    role_claim: "groups"
    admin_groups: ["platform-admins"]
    viewer_groups: ["data-scientists", "ml-engineers"]
```

### LDAP

```yaml
auth:
  provider: ldap
  ldap:
    server: "ldap://ldap.example.com:389"
    bind_dn: "cn=readonly,dc=example,dc=com"
    bind_password: "${LDAP_PASSWORD}"
    user_search_base: "ou=users,dc=example,dc=com"
    user_search_filter: "(uid={username})"
    group_search_base: "ou=groups,dc=example,dc=com"
    group_search_filter: "(member={dn})"
```

### Basic Auth

```yaml
auth:
  provider: basic
  basic:
    users:
      - username: admin
        password_hash: "$2a$10$..."  # bcrypt hash
        role: admin
      - username: viewer
        password_hash: "$2a$10$..."
        role: viewer
```

### Roles & Permissions

| Role | Catalog | Monitoring | Analytics | Alerts | Admin |
|------|---------|------------|-----------|--------|-------|
| `viewer` | Read | Read | Read | Read | - |
| `editor` | Read/Write | Read | Read | Manage | - |
| `admin` | Full | Full | Full | Full | Full |

## Customization

### Custom Themes

```yaml
branding:
  title: "Acme Feature Store"
  logo_url: "/custom/logo.svg"
  favicon_url: "/custom/favicon.ico"

  theme:
    primary_color: "#1976D2"
    secondary_color: "#424242"
    background: "#FAFAFA"
    surface: "#FFFFFF"
    error: "#D32F2F"
    warning: "#FFA000"
    success: "#388E3C"

  dark_mode:
    enabled: true
    default: false  # Light mode by default
```

### Custom Pages

Add custom documentation or links:

```yaml
custom_pages:
  - title: "Getting Started"
    path: "/docs/getting-started"
    content_url: "https://docs.example.com/feather/getting-started"

  - title: "Support"
    path: "/support"
    external_url: "https://support.example.com"
```

### Webhooks

Configure webhooks for dashboard events:

```yaml
webhooks:
  - event: "alert.created"
    url: "https://slack.example.com/webhook"
    secret: "${SLACK_WEBHOOK_SECRET}"

  - event: "feature.created"
    url: "https://api.example.com/feature-events"
    headers:
      Authorization: "Bearer ${API_TOKEN}"
```

## Related Documentation

- [Observability Guide](./observability) - Metrics and monitoring
- [Freshness Guide](./freshness) - Freshness SLAs
- [Drift Detection](./drift-detection) - Drift monitoring
