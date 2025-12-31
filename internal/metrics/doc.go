// Package metrics provides Prometheus metrics for the feature store.
//
// It defines and exports metrics for monitoring feature store operations
// including request latencies, throughput, cache hit rates, storage sizes,
// and error counts. The package exposes a Prometheus-compatible HTTP handler.
//
// Key metrics:
//   - feather_requests_total: Total number of requests by operation and status
//   - feather_request_duration_seconds: Request latency histogram
//   - feather_features_total: Total number of stored features
//   - feather_cache_hits_total: Cache hit/miss counts
package metrics
