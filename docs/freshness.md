# Feature Freshness SLAs

This guide covers Feather's adaptive feature freshness management system, including SLA definitions, TTL policies, ML-driven predictions, and automated remediation.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Freshness Monitor](#freshness-monitor)
- [TTL Policies](#ttl-policies)
- [ML-Driven Predictions](#ml-driven-predictions)
- [SLA Management](#sla-management)
- [Alerting](#alerting)
- [Auto-Remediation](#auto-remediation)
- [API Reference](#api-reference)
- [Configuration](#configuration)
- [Best Practices](#best-practices)

## Overview

Feature freshness is critical in ML systems - serving stale features can degrade model performance and lead to poor predictions. Feather provides a comprehensive freshness management system that:

- **Monitors** access patterns, value changes, and drift for all features
- **Predicts** optimal TTL values using ML-based analysis
- **Enforces** freshness SLAs with configurable thresholds
- **Alerts** when features become stale through multiple channels
- **Remediates** automatically through backfill, recomputation, or fallback

### Key Concepts

| Concept | Description |
|---------|-------------|
| **Freshness** | How recently a feature value was computed or updated |
| **TTL** | Time-to-live - how long a cached value is considered valid |
| **Staleness** | The age of a feature value since last update |
| **Drift** | Statistical deviation from reference distribution |
| **SLA** | Service Level Agreement defining freshness requirements |

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Freshness Management System                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐            │
│  │   Monitor   │───▶│  Predictor  │───▶│  Evaluator  │            │
│  │             │    │    (ML)     │    │             │            │
│  └─────────────┘    └─────────────┘    └─────────────┘            │
│         │                                     │                    │
│         ▼                                     ▼                    │
│  ┌─────────────┐                      ┌─────────────┐             │
│  │   Metrics   │                      │   Policies  │             │
│  │  - Access   │                      │   - Fixed   │             │
│  │  - Change   │                      │   - Adaptive│             │
│  │  - Drift    │                      │   - Time    │             │
│  └─────────────┘                      │   - Threshold│            │
│                                       └─────────────┘             │
│                                              │                     │
│                                              ▼                     │
│                                       ┌─────────────┐             │
│                                       │ SLA Manager │             │
│                                       │             │             │
│                                       └─────────────┘             │
│                                              │                     │
│                              ┌───────────────┼───────────────┐    │
│                              ▼               ▼               ▼    │
│                       ┌──────────┐    ┌──────────┐    ┌──────────┐│
│                       │ Alerting │    │Remediation│   │ Metrics  ││
│                       └──────────┘    └──────────┘    └──────────┘│
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

## Freshness Monitor

The Monitor tracks access patterns and value changes for all features.

### Access Metrics

Track how features are accessed:

```go
// AccessMetrics tracks access patterns for a feature
type AccessMetrics struct {
    FeatureName    string        // Feature identifier
    TotalAccesses  int64         // Total access count
    RecentAccesses int64         // Accesses in current window
    AccessRate     float64       // Accesses per second
    LastAccess     time.Time     // Timestamp of last access
    P50Latency     time.Duration // 50th percentile latency
    P95Latency     time.Duration // 95th percentile latency
    P99Latency     time.Duration // 99th percentile latency
    CacheHitRate   float64       // Cache hit ratio (0-1)
    StaleServes    int64         // Times stale value was served
}
```

### Change Metrics

Track how features change over time:

```go
// ChangeMetrics tracks change patterns for a feature
type ChangeMetrics struct {
    FeatureName        string    // Feature identifier
    TotalUpdates       int64     // Total update count
    RecentUpdates      int64     // Updates in current window
    UpdateRate         float64   // Updates per second
    LastUpdate         time.Time // Timestamp of last update
    AvgChangeMagnitude float64   // Average change size
    Volatility         float64   // Standard deviation of changes
    DriftScore         float64   // Current drift score (0-1)
}
```

### Recording Events

Record access and change events via the HTTP API:

```bash
# Record feature access
curl -X POST http://localhost:8080/v1/freshness/access \
  -H "Content-Type: application/json" \
  -d '{
    "feature": "user_click_count",
    "latency": 1500000,
    "cache_hit": true
  }'

# Record feature value change
curl -X POST http://localhost:8080/v1/freshness/change \
  -H "Content-Type: application/json" \
  -d '{
    "feature": "user_click_count",
    "old_value": 42,
    "new_value": 45
  }'

# Record drift score
curl -X POST http://localhost:8080/v1/freshness/drift \
  -H "Content-Type: application/json" \
  -d '{
    "feature": "user_click_count",
    "drift_score": 0.15
  }'

# Record stale serve
curl -X POST http://localhost:8080/v1/freshness/stale \
  -H "Content-Type: application/json" \
  -d '{
    "feature": "user_click_count"
  }'
```

### Getting Metrics

```bash
# Get metrics for all features
curl http://localhost:8080/v1/freshness/metrics

# Get metrics for a specific feature
curl http://localhost:8080/v1/freshness/metrics/user_click_count
```

Response:

```json
{
  "feature_name": "user_click_count",
  "access": {
    "feature_name": "user_click_count",
    "total_accesses": 15420,
    "recent_accesses": 342,
    "access_rate": 1.14,
    "last_access": "2024-01-15T10:30:00Z",
    "p50_latency": 1500000,
    "p95_latency": 5000000,
    "p99_latency": 12000000,
    "cache_hit_rate": 0.92,
    "stale_serves": 23
  },
  "change": {
    "feature_name": "user_click_count",
    "total_updates": 1542,
    "recent_updates": 28,
    "update_rate": 0.093,
    "last_update": "2024-01-15T10:28:00Z",
    "avg_change_magnitude": 3.2,
    "volatility": 1.8,
    "drift_score": 0.12
  }
}
```

## TTL Policies

Policies define how TTL values are determined for features. Four policy types are supported:

### 1. Fixed Policy

Assigns a constant TTL to matching features:

```bash
curl -X POST http://localhost:8080/v1/freshness/policies \
  -H "Content-Type: application/json" \
  -d '{
    "id": "critical-features",
    "name": "Critical Features Policy",
    "type": "fixed",
    "feature_pattern": "critical_*",
    "priority": 100,
    "enabled": true,
    "config": {
      "fixed_ttl": 60000000000
    }
  }'
```

### 2. Adaptive Policy

Uses ML predictions within configured bounds:

```bash
curl -X POST http://localhost:8080/v1/freshness/policies \
  -H "Content-Type: application/json" \
  -d '{
    "id": "user-features",
    "name": "User Features Policy",
    "type": "adaptive",
    "feature_pattern": "user_*",
    "priority": 50,
    "enabled": true,
    "config": {
      "min_ttl": 10000000000,
      "max_ttl": 3600000000000,
      "access_weight": 0.3,
      "volatility_weight": 0.4,
      "drift_weight": 0.3
    }
  }'
```

### 3. Time-Based Policy

Different TTLs for peak vs off-peak hours:

```bash
curl -X POST http://localhost:8080/v1/freshness/policies \
  -H "Content-Type: application/json" \
  -d '{
    "id": "time-sensitive",
    "name": "Time Sensitive Policy",
    "type": "time",
    "feature_pattern": "realtime_*",
    "priority": 75,
    "enabled": true,
    "config": {
      "peak_hours_start": 9,
      "peak_hours_end": 18,
      "peak_ttl": 30000000000,
      "off_peak_ttl": 300000000000
    }
  }'
```

### 4. Threshold Policy

TTL based on access rate or drift thresholds:

```bash
curl -X POST http://localhost:8080/v1/freshness/policies \
  -H "Content-Type: application/json" \
  -d '{
    "id": "threshold-based",
    "name": "Threshold Policy",
    "type": "threshold",
    "feature_pattern": "model_*",
    "priority": 60,
    "enabled": true,
    "config": {
      "access_rate_threshold": 10.0,
      "high_access_ttl": 60000000000,
      "low_access_ttl": 600000000000,
      "drift_threshold": 0.5,
      "high_drift_ttl": 30000000000
    }
  }'
```

### Policy Matching

- Policies use glob patterns to match feature names
- Supported patterns: `*` (prefix), `*suffix`, exact match
- Higher priority policies take precedence
- Multiple policies can be active simultaneously

### Policy Evaluation

Get the recommended TTL for a feature:

```bash
# Get TTL with explanation
curl http://localhost:8080/v1/freshness/ttl/user_click_count
```

Response:

```json
{
  "feature_name": "user_click_count",
  "ttl": 120000000000,
  "policy_id": "user-features",
  "policy_name": "User Features Policy",
  "policy_type": "adaptive",
  "reason": "adaptive prediction: stable values, high cache hit rate",
  "evaluated_at": "2024-01-15T10:30:00Z"
}
```

## ML-Driven Predictions

The Predictor uses machine learning to recommend optimal TTL values.

### Prediction Factors

| Factor | Weight | Description |
|--------|--------|-------------|
| Access Pattern | 30% | High access + high cache hit = longer TTL |
| Volatility | 40% | High update rate/change magnitude = shorter TTL |
| Drift | 30% | Statistical drift = shorter TTL |

### Prediction Output

```bash
# Get prediction for a feature
curl http://localhost:8080/v1/freshness/predictions/user_click_count
```

Response:

```json
{
  "feature_name": "user_click_count",
  "recommended_ttl": 120000000000,
  "confidence": 0.85,
  "reason": "stable values, high cache hit rate",
  "access_score": 0.78,
  "volatility_score": 0.15,
  "drift_score": 0.12,
  "predicted_at": "2024-01-15T10:30:00Z"
}
```

### Confidence Levels

| Confidence | Meaning |
|------------|---------|
| 0.0 - 0.3 | Low confidence (limited data) |
| 0.3 - 0.6 | Medium confidence |
| 0.6 - 0.8 | High confidence |
| 0.8 - 1.0 | Very high confidence |

### Getting All Predictions

```bash
curl http://localhost:8080/v1/freshness/predictions
```

## SLA Management

SLAs define freshness requirements with alerting and remediation.

### SLA Specification

```go
// SLASpec defines a freshness SLA
type SLASpec struct {
    ID                string              // Unique identifier
    Name              string              // Human-readable name
    Description       string              // Description
    FeaturePattern    string              // Glob pattern for features
    Features          []string            // Explicit feature list
    Thresholds        SLAThresholds       // Staleness thresholds
    AlertConfig       SLAAlertConfig      // Alerting configuration
    RemediationConfig SLARemediationConfig// Auto-remediation config
    Enabled           bool                // Is SLA active
    Priority          int                 // Evaluation order
    Tags              []string            // Organization tags
    Owner             string              // SLA owner
}
```

### Severity Levels

| Severity | Description | Typical Action |
|----------|-------------|----------------|
| `ok` | Feature is fresh | No action |
| `warning` | Approaching staleness | Monitor closely |
| `critical` | Feature is stale | Investigate |
| `breach` | SLA violated | Immediate action |

### Thresholds Configuration

```go
type SLAThresholds struct {
    WarningAge        time.Duration // Trigger warning
    CriticalAge       time.Duration // Trigger critical
    BreachAge         time.Duration // Trigger breach (SLA violation)
    MaxStalePercent   float64       // Max % of stale serves
    MinFreshnessScore float64       // Minimum freshness score (0-100)
}
```

### Example SLA

```go
sla := &freshness.SLASpec{
    ID:          "critical-user-features",
    Name:        "Critical User Features SLA",
    Description: "Ensures user features stay fresh for real-time personalization",
    FeaturePattern: "user_*",
    Thresholds: freshness.SLAThresholds{
        WarningAge:        15 * time.Minute,
        CriticalAge:       30 * time.Minute,
        BreachAge:         1 * time.Hour,
        MaxStalePercent:   5.0,  // Max 5% stale serves
        MinFreshnessScore: 80.0, // Minimum 80% freshness
    },
    AlertConfig: freshness.SLAAlertConfig{
        Channels: []freshness.AlertChannelConfig{
            {
                Type: freshness.AlertChannelSlack,
                URL:  "https://hooks.slack.com/services/xxx",
                Severities: []freshness.SLASeverity{
                    freshness.SeverityWarning,
                    freshness.SeverityCritical,
                    freshness.SeverityBreach,
                },
            },
            {
                Type: freshness.AlertChannelPagerDuty,
                URL:  "https://events.pagerduty.com/v2/enqueue",
                Headers: map[string]string{
                    "routing_key": "your-routing-key",
                },
                Severities: []freshness.SLASeverity{
                    freshness.SeverityBreach,
                },
            },
        },
        CooldownPeriod:     5 * time.Minute,
        EscalationAfter:    15 * time.Minute,
        IncludeMetrics:     true,
    },
    RemediationConfig: freshness.SLARemediationConfig{
        Enabled: true,
        Actions: map[freshness.SLASeverity]freshness.RemediationAction{
            freshness.SeverityWarning:  freshness.RemediationNotify,
            freshness.SeverityCritical: freshness.RemediationRecompute,
            freshness.SeverityBreach:   freshness.RemediationBackfill,
        },
        MaxRetries:   3,
        RetryBackoff: 30 * time.Second,
    },
    Enabled:  true,
    Priority: 100,
    Owner:    "ml-platform-team",
}
```

## Alerting

### Alert Channels

| Channel | Description | Configuration |
|---------|-------------|---------------|
| `webhook` | Generic HTTP webhook | URL, headers |
| `slack` | Slack incoming webhook | Webhook URL |
| `pagerduty` | PagerDuty Events API | Integration key |
| `email` | Email notifications | SMTP config |
| `prometheus` | Prometheus alerting | Metrics labels |

### Slack Alert Format

Alerts sent to Slack include:

- Color-coded severity (green/yellow/orange/red)
- Feature name and SLA ID
- Stale duration and freshness score
- Timestamp

### PagerDuty Integration

```go
AlertChannelConfig{
    Type: freshness.AlertChannelPagerDuty,
    URL:  "https://events.pagerduty.com/v2/enqueue",
    Headers: map[string]string{
        "routing_key": "your-routing-key",
    },
    Severities: []freshness.SLASeverity{
        freshness.SeverityCritical,
        freshness.SeverityBreach,
    },
}
```

### Alert Cooldown

Prevent alert fatigue with cooldown periods:

```go
AlertConfig: SLAAlertConfig{
    CooldownPeriod: 5 * time.Minute,  // Min time between alerts
    EscalationAfter: 15 * time.Minute, // Escalate if unresolved
}
```

### Alert Acknowledgment

Acknowledge alerts to prevent escalation:

```go
err := slaManager.AcknowledgeAlert(alertID, "user@example.com")
```

## Auto-Remediation

### Remediation Actions

| Action | Description | Use Case |
|--------|-------------|----------|
| `none` | No action | Monitoring only |
| `notify` | Send notification | Warning severity |
| `recompute` | Trigger recomputation | Critical features |
| `backfill` | Load from offline store | Major staleness |
| `fallback` | Use fallback value | Extreme cases |

### Backfill Configuration

```go
BackfillRemediationConfig{
    SourcePath: "s3://bucket/features/",
    MaxAge:     24 * time.Hour,
    Priority:   10,
}
```

### Fallback Configuration

```go
FallbackRemediationConfig{
    FallbackFeature: "user_click_count_hourly",  // Use another feature
    DefaultValue:    0,                           // Or use default value
    UseCachedValue:  true,                        // Or use last cached
}
```

### Custom Remediation Callback

```go
slaManager.SetRemediationCallback(func(
    ctx context.Context,
    feature string,
    action freshness.RemediationAction,
    config *freshness.SLARemediationConfig,
) error {
    switch action {
    case freshness.RemediationBackfill:
        return triggerBackfillJob(ctx, feature, config.BackfillConfig)
    case freshness.RemediationRecompute:
        return triggerRecomputeJob(ctx, feature)
    default:
        return nil
    }
})
```

## API Reference

### Metrics Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/freshness/metrics` | Get all feature metrics |
| GET | `/v1/freshness/metrics/{feature}` | Get specific feature metrics |

### TTL Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/freshness/ttl/{feature}` | Get recommended TTL |
| GET | `/v1/freshness/predictions` | Get all predictions |
| GET | `/v1/freshness/predictions/{feature}` | Get specific prediction |
| GET | `/v1/freshness/evaluate` | Evaluate all features |

### Policy Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/freshness/policies` | List all policies |
| POST | `/v1/freshness/policies` | Create policy |
| GET | `/v1/freshness/policies/{id}` | Get policy |
| PUT | `/v1/freshness/policies/{id}` | Update policy |
| DELETE | `/v1/freshness/policies/{id}` | Delete policy |

### Recording Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/freshness/access` | Record access event |
| POST | `/v1/freshness/change` | Record value change |
| POST | `/v1/freshness/drift` | Record drift score |
| POST | `/v1/freshness/stale` | Record stale serve |

### Stats Endpoint

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/freshness/stats` | Get manager statistics |

## Configuration

### Manager Configuration

```go
config := freshness.ManagerConfig{
    Monitor: freshness.MonitorConfig{
        WindowSize:        5 * time.Minute,  // Metrics window
        MaxHistoryEntries: 1000,             // Max entries per feature
        CleanupInterval:   1 * time.Minute,  // Cleanup frequency
    },
    Predictor: freshness.PredictorConfig{
        MinTTL:           1 * time.Second,   // Minimum allowed TTL
        MaxTTL:           24 * time.Hour,    // Maximum allowed TTL
        DefaultTTL:       5 * time.Minute,   // Default TTL
        AccessWeight:     0.3,               // Access pattern weight
        VolatilityWeight: 0.4,               // Volatility weight
        DriftWeight:      0.3,               // Drift weight
        UpdateInterval:   1 * time.Minute,   // Prediction refresh
    },
}

manager := freshness.NewManager(config)
```

### SLA Manager Configuration

```go
slaConfig := freshness.SLAManagerConfig{
    CheckInterval:     time.Minute,       // SLA check frequency
    AlertRetention:    24 * time.Hour,    // Alert history retention
    MaxAlerts:         1000,              // Max alerts to retain
    DefaultCooldown:   5 * time.Minute,   // Default alert cooldown
    EnableRemediation: true,              // Enable auto-remediation
    WebhookTimeout:    10 * time.Second,  // Webhook delivery timeout
}

slaManager := freshness.NewSLAManager(manager, slaConfig, logger)
slaManager.Start(ctx)
```

### Environment Variables

```bash
# Monitor settings
FEATHER_FRESHNESS_WINDOW_SIZE=5m
FEATHER_FRESHNESS_MAX_HISTORY=1000
FEATHER_FRESHNESS_CLEANUP_INTERVAL=1m

# Predictor settings
FEATHER_FRESHNESS_MIN_TTL=1s
FEATHER_FRESHNESS_MAX_TTL=24h
FEATHER_FRESHNESS_DEFAULT_TTL=5m
FEATHER_FRESHNESS_ACCESS_WEIGHT=0.3
FEATHER_FRESHNESS_VOLATILITY_WEIGHT=0.4
FEATHER_FRESHNESS_DRIFT_WEIGHT=0.3

# SLA settings
FEATHER_SLA_CHECK_INTERVAL=1m
FEATHER_SLA_ALERT_RETENTION=24h
FEATHER_SLA_MAX_ALERTS=1000
FEATHER_SLA_ENABLE_REMEDIATION=true
```

## Best Practices

### 1. Start with Monitoring

Before defining SLAs, collect baseline metrics:

```bash
# Monitor for at least a week
curl http://localhost:8080/v1/freshness/stats

# Review access patterns
curl http://localhost:8080/v1/freshness/metrics
```

### 2. Define Appropriate Thresholds

Set thresholds based on business requirements:

| Feature Type | Warning | Critical | Breach |
|--------------|---------|----------|--------|
| Real-time (fraud) | 1m | 5m | 15m |
| User preferences | 15m | 1h | 4h |
| Batch aggregations | 1h | 6h | 24h |

### 3. Use Policy Hierarchy

Layer policies from general to specific:

```go
// Base policy (priority 10)
NewAdaptivePolicy("base", "Base Policy", "*", 1*time.Minute, 1*time.Hour, 10)

// User features (priority 50)
NewAdaptivePolicy("user", "User Policy", "user_*", 30*time.Second, 30*time.Minute, 50)

// Critical features (priority 100)
NewFixedPolicy("critical", "Critical Policy", "critical_*", 10*time.Second, 100)
```

### 4. Configure Multi-Channel Alerting

Route alerts appropriately:

- **Warning**: Slack channel
- **Critical**: Slack + PagerDuty (low urgency)
- **Breach**: PagerDuty (high urgency) + Email

### 5. Test Remediation Actions

Test auto-remediation in staging:

```go
// Dry-run mode
slaConfig.EnableRemediation = false

// Test callback
slaManager.SetRemediationCallback(func(ctx, feature, action, config) error {
    log.Printf("Would remediate %s with %s", feature, action)
    return nil
})
```

### 6. Monitor SLA Compliance

Track compliance over time:

```bash
# Get SLA metrics
curl http://localhost:8080/v1/freshness/sla/metrics

# Export to Prometheus
# feather_sla_compliance_percent
# feather_sla_breaches_total
# feather_sla_alerts_total
```

### 7. Handle Alert Fatigue

Prevent alert storms:

- Use appropriate cooldown periods (5-15 minutes)
- Group alerts by feature group
- Set escalation only for persistent issues
- Review and tune thresholds regularly

### 8. Document SLAs

Maintain an SLA inventory:

| SLA ID | Owner | Features | Breach Threshold | Escalation |
|--------|-------|----------|-----------------|------------|
| user-critical | ML Team | user_* | 1h | PagerDuty |
| model-inputs | Data Team | model_* | 4h | Slack |
| batch-aggs | Analytics | batch_* | 24h | Email |

## Troubleshooting

### High False Positive Alerts

1. Review thresholds - may be too aggressive
2. Check for legitimate access pattern changes
3. Increase cooldown period
4. Add more specific policies

### Low Prediction Confidence

1. Wait for more data collection (min 1 week)
2. Ensure events are being recorded
3. Check for data pipeline issues
4. Use fixed policies temporarily

### Remediation Failures

1. Check remediation callback logs
2. Verify backfill source availability
3. Review retry configuration
4. Check network connectivity for webhooks

### Missing Metrics

1. Verify access recording integration
2. Check change recording for feature updates
3. Review drift detection pipeline
4. Ensure proper feature naming
