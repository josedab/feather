// Package realtimemonitor provides a real-time monitoring dashboard for feature
// store observability. It tracks feature freshness, serving latency, drift alerts,
// and pipeline health with configurable alert thresholds and notification channels.
//
// Key components:
//   - Dashboard: Aggregates and serves real-time metrics
//   - FreshnessTracker: Monitors feature data freshness
//   - LatencyTracker: Tracks p50/p95/p99 serving latency
//   - AlertManager: Manages alert rules and notifications
package realtimemonitor
