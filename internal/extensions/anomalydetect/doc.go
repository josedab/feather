// Package anomalydetect provides real-time streaming anomaly detection on
// feature values using Z-score, IQR, and configurable thresholds with
// auto-quarantine capability.
//
// Key components:
//   - Detector: Monitors feature values and detects anomalies in real time
//   - AnomalyResult: Contains detection outcome with type, score, and metadata
package anomalydetect
