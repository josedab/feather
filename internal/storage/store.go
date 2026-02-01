// Package storage implements Feather's tiered storage architecture.
//
// The storage layer uses a two-tier design optimized for both latency and durability:
//
//   - Hot Tier: In-memory LRU cache with 256 shards for sub-millisecond access (<1ms P99)
//   - Warm Tier: BadgerDB-backed persistent storage with historical versioning (<10ms P99)
//
// # Architecture Overview
//
// The [Store] type coordinates reads and writes across both tiers:
//
//	┌─────────────────────────────────────────────────────────────┐
//	│                         Store                                │
//	│  ┌─────────────────────┐    ┌─────────────────────────────┐ │
//	│  │     Hot Tier        │    │        Warm Tier            │ │
//	│  │  (Memory, 256 shards)│    │  (BadgerDB, persistent)    │ │
//	│  │  • <1ms latency     │    │  • Historical versions     │ │
//	│  │  • LRU eviction     │    │  • Point-in-time queries   │ │
//	│  └─────────────────────┘    └─────────────────────────────┘ │
//	└─────────────────────────────────────────────────────────────┘
//
// # Read Path
//
// Reads first check the hot tier, falling back to warm tier on cache miss:
//
//	features, err := store.Get("user:123", []string{"click_count", "purchase_total"})
//
// # Write Path
//
// Writes go to the hot tier synchronously and warm tier asynchronously:
//
//	err := store.Put("user:123", map[string]*domain.FeatureValue{
//	    "click_count": {Value: 42, Timestamp: time.Now().UnixNano()},
//	})
//
// # Point-in-Time Queries
//
// Historical feature values can be retrieved using GetAsOf:
//
//	features, err := store.GetAsOf("user:123", []string{"click_count"}, time.Now().Add(-24*time.Hour))
package storage

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/domain"
)

// Store provides unified access to Feather's tiered storage architecture.
//
// Store coordinates reads and writes between the hot (memory) and warm (disk) tiers,
// implementing read-through caching and async background writes for optimal latency.
//
// The store is safe for concurrent use from multiple goroutines.
type Store struct {
	hot     *HotTier
	warm    *WarmTier
	schema  SchemaRegistry
	metrics *StoreMetrics

	warmWrites      chan warmWriteRequest
	warmWriteErrors int64
	warmWriteDrops  int64
	wg              sync.WaitGroup
	ctx             context.Context
	cancel          context.CancelFunc
	closed          int32
	logger          *slog.Logger
}

// SchemaRegistry provides access to feature group schemas for validation.
//
// Implementations must be safe for concurrent access.
type SchemaRegistry interface {
	// GetGroup returns the feature group with the given name.
	GetGroup(name string) (*domain.FeatureGroup, error)
	// GetFeatureSpec returns the specification for a feature by name.
	GetFeatureSpec(featureName string) (*domain.FeatureSpec, error)
	// ListGroups returns all registered feature groups.
	ListGroups() []*domain.FeatureGroup
}

// StoreMetrics contains aggregate performance statistics for the store.
//
// All counters are updated atomically and safe to read during operation.
type StoreMetrics struct {
	// HotHits is the number of successful hot tier lookups.
	HotHits int64
	// HotMisses is the number of hot tier cache misses.
	HotMisses int64
	// WarmHits is the number of successful warm tier lookups.
	WarmHits int64
	// WarmMisses is the number of warm tier misses (entity not found).
	WarmMisses int64
	// Writes is the total number of write operations.
	Writes int64
	// WarmWriteErrors is the number of warm tier write errors.
	WarmWriteErrors int64
	// WarmWriteDrops is the number of warm tier writes dropped due to shutdown.
	WarmWriteDrops int64
}

// StoreOptions configures the store's behavior and resource limits.
type StoreOptions struct {
	// HotMaxSize is the maximum memory (in bytes) for the hot tier cache.
	// Features are evicted using LRU when this limit is exceeded.
	HotMaxSize int64
	// WarmPath is the filesystem path for BadgerDB storage.
	WarmPath string
	// WarmSyncInterval controls how often BadgerDB syncs to disk.
	WarmSyncInterval time.Duration
	// WarmInMemory enables in-memory mode for testing (data not persisted).
	WarmInMemory bool
	// TTLCheckInterval is how often to scan for and remove expired features.
	// Defaults to 1 minute if not specified.
	TTLCheckInterval time.Duration

	// WarmWriteWorkers is the number of background workers for warm tier writes.
	// Defaults to 4 if not specified.
	WarmWriteWorkers int
	// WarmWriteBuffer is the size of the warm tier write queue.
	// Defaults to 1024 if not specified.
	WarmWriteBuffer int
}

type warmWriteRequest struct {
	entityKey string
	features  map[string]*domain.FeatureValue
}

// NewStore creates a new feature store.
func NewStore(ctx context.Context, opts StoreOptions, schema SchemaRegistry) (*Store, error) { //nolint:contextcheck
	hot := NewHotTier(opts.HotMaxSize)

	warm, err := NewWarmTier(WarmTierOptions{
		Path:         opts.WarmPath,
		SyncInterval: opts.WarmSyncInterval,
		InMemory:     opts.WarmInMemory,
	})
	if err != nil {
		return nil, err
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &Store{
		hot:     hot,
		warm:    warm,
		schema:  schema,
		metrics: &StoreMetrics{},
		ctx:     ctx,
		cancel:  cancel,
		logger:  slog.Default(),
	}

	// Start background tasks
	ttlInterval := opts.TTLCheckInterval
	if ttlInterval == 0 {
		ttlInterval = time.Minute
	}
	s.wg.Add(1)
	go s.ttlLoop(ctx, ttlInterval)

	workerCount := opts.WarmWriteWorkers
	if workerCount <= 0 {
		workerCount = 4
	}
	bufferSize := opts.WarmWriteBuffer
	if bufferSize <= 0 {
		bufferSize = 1024
	}
	s.warmWrites = make(chan warmWriteRequest, bufferSize)
	for i := 0; i < workerCount; i++ {
		s.wg.Add(1)
		go s.warmWriteLoop(ctx)
	}

	return s, nil
}

// Get retrieves features for an entity.
func (s *Store) Get(entityKey string, features []string) (map[string]*domain.FeatureValue, error) {
	// Try hot tier first
	result, err := s.hot.Get(entityKey, features)
	if err != nil && !errors.Is(err, domain.ErrEntityNotFound) {
		return nil, err
	}

	// Get missing slice from pool to reduce allocations
	missingPtr, ok := featureNameSlicePool.Get().(*[]string)
	if !ok {
		missing := make([]string, 0, len(features))
		missingPtr = &missing
	}
	missing := (*missingPtr)[:0]
	defer func() {
		*missingPtr = missing[:0]
		featureNameSlicePool.Put(missingPtr)
	}()

	if result == nil {
		result = make(map[string]*domain.FeatureValue)
		missing = append(missing, features...)
	} else {
		for _, f := range features {
			if _, ok := result[f]; !ok {
				missing = append(missing, f)
			}
		}
	}

	// Check warm tier for missing features
	if len(missing) > 0 {
		warmResult, err := s.warm.Get(entityKey, missing)
		if err != nil {
			return nil, err
		}
		for k, v := range warmResult {
			result[k] = v
		}
		if len(warmResult) > 0 {
			_ = s.hot.Put(entityKey, warmResult)
		}
	}

	return result, nil
}

// GetAsOf retrieves features as of a specific timestamp.
func (s *Store) GetAsOf(entityKey string, features []string, asOf time.Time) (map[string]*domain.FeatureValue, error) {
	return s.warm.GetAsOf(entityKey, features, asOf)
}

// Put stores features for an entity in both tiers.
func (s *Store) Put(entityKey string, features map[string]*domain.FeatureValue) error {
	// Write to hot tier first
	if err := s.hot.Put(entityKey, features); err != nil {
		return err
	}

	// Write to warm tier asynchronously with tracking
	s.enqueueWarmWrite(entityKey, features)

	return nil
}

// Delete removes an entity from both tiers.
func (s *Store) Delete(entityKey string) error {
	if err := s.hot.Delete(entityKey); err != nil {
		return err
	}
	// Note: We don't delete from warm tier to preserve history
	return nil
}

// Hot returns the hot tier for direct access.
func (s *Store) Hot() *HotTier {
	return s.hot
}

// Warm returns the warm tier for direct access.
func (s *Store) Warm() *WarmTier {
	return s.warm
}

// Metrics returns current metrics.
func (s *Store) Metrics() StoreMetrics {
	hotMetrics := s.hot.Metrics()
	return StoreMetrics{
		HotHits:         hotMetrics.Hits,
		HotMisses:       hotMetrics.Misses,
		WarmWriteErrors: atomic.LoadInt64(&s.warmWriteErrors),
		WarmWriteDrops:  atomic.LoadInt64(&s.warmWriteDrops),
	}
}

// ttlLoop periodically checks for and removes expired features.
func (s *Store) ttlLoop(ctx context.Context, interval time.Duration) {
	defer s.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.schema != nil {
				for _, group := range s.schema.ListGroups() {
					if group.TTL > 0 {
						s.hot.ExpireOlderThan(group.TTL)
					}
				}
			}
		}
	}
}

// Close shuts down the store.
func (s *Store) Close() error {
	if atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		s.cancel()
	}
	s.wg.Wait()
	s.hot.Close()
	return s.warm.Close()
}

func (s *Store) enqueueWarmWrite(entityKey string, features map[string]*domain.FeatureValue) {
	if atomic.LoadInt32(&s.closed) == 1 {
		atomic.AddInt64(&s.warmWriteDrops, 1)
		return
	}
	select {
	case s.warmWrites <- warmWriteRequest{entityKey: entityKey, features: features}:
	case <-s.ctx.Done():
		atomic.AddInt64(&s.warmWriteDrops, 1)
	default:
		atomic.AddInt64(&s.warmWriteDrops, 1)
	}
}

func (s *Store) warmWriteLoop(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case req := <-s.warmWrites:
			if err := s.warm.Put(req.entityKey, req.features); err != nil {
				atomic.AddInt64(&s.warmWriteErrors, 1)
				if s.logger != nil {
					s.logger.Error("warm tier write failed", "entity_key", req.entityKey, "error", err)
				}
			}
		}
	}
}
