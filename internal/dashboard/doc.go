// Package dashboard provides the feature monitoring dashboard backend.
//
// It serves a web UI for monitoring feature store health, drift detection,
// usage analytics, and feature discovery. The dashboard provides real-time
// visibility into feature freshness, data quality, and system performance.
//
// Key features:
//   - Feature catalog with search and filtering
//   - Drift visualization and alerts
//   - Freshness heatmaps
//   - Usage analytics
//   - Alert management
//   - System health overview
//
// # Usage
//
//	dashboard := dashboard.New(dashboard.Config{
//	    Store:        featherStore,
//	    DriftMonitor: driftMonitor,
//	    Metrics:      metricsCollector,
//	})
//	dashboard.RegisterRoutes(mux)
package dashboard
