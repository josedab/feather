// Package drift provides feature drift detection and monitoring.
//
// It tracks statistical properties of feature values over time and detects
// significant distribution shifts that may indicate data quality issues or
// concept drift. The package supports multiple drift detection algorithms
// and configurable alerting thresholds.
//
// Key components:
//   - Detector: Monitors features for statistical drift
//   - Alert: Represents a detected drift event with severity and metrics
package drift
