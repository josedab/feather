// Package storage provides tiered feature storage for the feature store.
//
// It implements a two-tier storage architecture with a hot tier (in-memory LRU
// cache) for low-latency access and a warm tier (BadgerDB) for persistent storage
// with historical versions. The package handles automatic tier management,
// TTL expiration, and efficient batch operations.
//
// Key components:
//   - Store: Unified interface to hot and warm storage tiers
//   - HotTier: LRU-based in-memory cache with configurable size limits
//   - WarmTier: BadgerDB-backed persistent storage with versioning
//   - Registry: Schema registry for feature group definitions
//
// Example usage:
//
//	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
//	    HotMaxSize: 1 << 30, // 1GB
//	    WarmPath:   "/var/lib/feather/data",
//	}, schema)
//	defer store.Close()
package storage
