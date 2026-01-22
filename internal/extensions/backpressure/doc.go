// Package backpressure provides automatic detection of ingestion/serving
// backpressure with scaling recommendations based on queue depth, latency
// trends, and error rates.
//
// Key components:
//   - Monitor: Tracks system metrics and evaluates pressure levels
//   - PressureReport: Contains pressure assessment and scaling recommendations
package backpressure
