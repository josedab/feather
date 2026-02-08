// Package cache provides caching strategies for the feature store.
//
// It implements predictive caching that pre-fetches features based on access
// patterns and usage analysis. The cache integrates with the storage tier to
// reduce latency for frequently accessed features.
//
// Key components:
//   - PredictiveCache: Analyzes access patterns to pre-warm cache entries
package cache
