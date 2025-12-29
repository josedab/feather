package storage

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/domain"
)

// Object pools reduce allocations in hot paths by reusing slice memory.
// These pools are used for temporary slices during batch operations.
var (
	// featureNameSlicePool provides reusable slices for feature name lists.
	featureNameSlicePool = sync.Pool{
		New: func() interface{} {
			s := make([]string, 0, 32)
			return &s
		},
	}

	// entityKeySlicePool provides reusable slices for entity key lists.
	entityKeySlicePool = sync.Pool{
		New: func() interface{} {
			s := make([]string, 0, 64)
			return &s
		},
	}
)

// entityData holds all features for a single entity with its own mutex.
// This enables fine-grained locking at the entity level within a shard.
type entityData struct {
	features map[string]*domain.FeatureValue
	mu       sync.RWMutex
}

// HotTier provides in-memory feature storage optimized for sub-millisecond access.
//
// # Sharding Strategy
//
// The hot tier uses 256 shards to minimize lock contention. Entity keys are
// distributed across shards using FNV-1a hashing. Each shard has its own
// RWMutex, allowing concurrent reads across different shards.
//
// # Lock Hierarchy
//
// Two levels of locking are used:
//  1. Shard-level RWMutex: Protects the entity map within the shard
//  2. Entity-level RWMutex: Protects the feature map within the entity
//
// This allows multiple goroutines to read different entities concurrently,
// even within the same shard.
//
// # Memory Management
//
// Memory usage is tracked approximately (100 bytes per feature value).
// When the configured maximum size is exceeded, the eviction channel is
// signaled to trigger LRU eviction.
//
// # Metrics
//
// All metrics (hits, misses, evictions) are tracked using atomic counters,
// providing lock-free observability without impacting read latency.
type HotTier struct {
	// shards contains 256 independent hash buckets for concurrent access.
	// The shard index is computed as: fnvHash(entityKey) % 256
	shards [256]*shard

	// arena provides pooled memory allocation for reducing GC pressure.
	arena *Arena

	// maxSize is the maximum allowed memory usage in bytes.
	maxSize int64

	// curSize tracks the current approximate memory usage (atomic).
	curSize int64

	// evictChan receives entity keys when memory limit is exceeded.
	evictChan chan string

	// metrics contains atomic counters for observability.
	metrics *HotTierMetrics
}

// shard represents one of 256 hash buckets in the hot tier.
// Each shard has independent locking for concurrent access.
type shard struct {
	data map[string]*entityData
	mu   sync.RWMutex
}

// HotTierMetrics contains performance counters for the hot tier.
// All fields are updated atomically and safe to read during operation.
type HotTierMetrics struct {
	// Hits is the number of successful feature lookups.
	Hits int64
	// Misses is the number of feature lookup misses.
	Misses int64
	// Evictions is the number of entities removed due to memory pressure.
	Evictions int64
	// TotalReads is the total number of Get operations.
	TotalReads int64
	// DroppedEvictionSignals is the number of eviction signals dropped due to channel full.
	DroppedEvictionSignals int64
}

// NewHotTier creates a new hot tier with the given max memory size.
func NewHotTier(maxSize int64) *HotTier {
	h := &HotTier{
		arena:     NewArena(1024 * 1024), // 1MB chunks
		maxSize:   maxSize,
		evictChan: make(chan string, 1000),
		metrics:   &HotTierMetrics{},
	}

	// Initialize shards
	for i := range h.shards {
		h.shards[i] = &shard{
			data: make(map[string]*entityData),
		}
	}

	return h
}

// getShard returns the shard for the given entity key.
func (h *HotTier) getShard(entityKey string) *shard {
	hash := fnvHash(entityKey)
	return h.shards[hash%256]
}

// fnvHash computes FNV-1a hash of a string.
func fnvHash(s string) uint32 {
	hash := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		hash ^= uint32(s[i])
		hash *= 16777619
	}
	return hash
}

// Get retrieves features for an entity.
func (h *HotTier) Get(entityKey string, features []string) (map[string]*domain.FeatureValue, error) {
	atomic.AddInt64(&h.metrics.TotalReads, 1)

	shard := h.getShard(entityKey)
	shard.mu.RLock()
	entity, ok := shard.data[entityKey]
	shard.mu.RUnlock()

	if !ok {
		atomic.AddInt64(&h.metrics.Misses, 1)
		return nil, domain.ErrEntityNotFound
	}

	entity.mu.RLock()
	defer entity.mu.RUnlock()

	result := make(map[string]*domain.FeatureValue, len(features))
	for _, f := range features {
		if val, ok := entity.features[f]; ok {
			result[f] = val
			atomic.AddInt64(&h.metrics.Hits, 1)
		} else {
			atomic.AddInt64(&h.metrics.Misses, 1)
		}
	}

	return result, nil
}

// GetAll retrieves all features for an entity.
func (h *HotTier) GetAll(entityKey string) (map[string]*domain.FeatureValue, error) {
	shard := h.getShard(entityKey)
	shard.mu.RLock()
	entity, ok := shard.data[entityKey]
	shard.mu.RUnlock()

	if !ok {
		return nil, domain.ErrEntityNotFound
	}

	entity.mu.RLock()
	defer entity.mu.RUnlock()

	result := make(map[string]*domain.FeatureValue, len(entity.features))
	for k, v := range entity.features {
		result[k] = v
	}

	return result, nil
}

// Put stores features for an entity.
func (h *HotTier) Put(entityKey string, features map[string]*domain.FeatureValue) error {
	shard := h.getShard(entityKey)

	shard.mu.Lock()
	entity, ok := shard.data[entityKey]
	if !ok {
		entity = &entityData{
			features: make(map[string]*domain.FeatureValue),
		}
		shard.data[entityKey] = entity
	}
	shard.mu.Unlock()

	entity.mu.Lock()
	defer entity.mu.Unlock()

	for name, val := range features {
		existing, exists := entity.features[name]
		if !exists || val.Version > existing.Version {
			entity.features[name] = val
			// Track size growth (approximate)
			atomic.AddInt64(&h.curSize, 100)
		}
	}

	// Check if we need to trigger eviction
	if atomic.LoadInt64(&h.curSize) > h.maxSize {
		// Signal eviction needed
		select {
		case h.evictChan <- entityKey:
		default:
			// Channel full, track dropped signal
			atomic.AddInt64(&h.metrics.DroppedEvictionSignals, 1)
		}
	}

	return nil
}

// Delete removes an entity from the hot tier.
func (h *HotTier) Delete(entityKey string) error {
	shard := h.getShard(entityKey)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if entity, ok := shard.data[entityKey]; ok {
		// Approximate size reduction
		atomic.AddInt64(&h.curSize, -int64(len(entity.features)*100))
		delete(shard.data, entityKey)
		atomic.AddInt64(&h.metrics.Evictions, 1)
	}

	return nil
}

// ExpireOlderThan removes features older than the given duration.
func (h *HotTier) ExpireOlderThan(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge).UnixNano()
	expired := 0

	for _, shard := range h.shards {
		shard.mu.Lock()
		for entityKey, entity := range shard.data {
			entity.mu.Lock()
			for featureName, val := range entity.features {
				if val.Timestamp < cutoff {
					delete(entity.features, featureName)
					expired++
				}
			}
			// Remove entity if no features left
			if len(entity.features) == 0 {
				delete(shard.data, entityKey)
			}
			entity.mu.Unlock()
		}
		shard.mu.Unlock()
	}

	return expired
}

// Size returns the approximate current size in bytes.
func (h *HotTier) Size() int64 {
	return atomic.LoadInt64(&h.curSize)
}

// EntityCount returns the total number of entities.
func (h *HotTier) EntityCount() int {
	count := 0
	for _, shard := range h.shards {
		shard.mu.RLock()
		count += len(shard.data)
		shard.mu.RUnlock()
	}
	return count
}

// Metrics returns current metrics.
func (h *HotTier) Metrics() HotTierMetrics {
	return HotTierMetrics{
		Hits:       atomic.LoadInt64(&h.metrics.Hits),
		Misses:     atomic.LoadInt64(&h.metrics.Misses),
		Evictions:  atomic.LoadInt64(&h.metrics.Evictions),
		TotalReads: atomic.LoadInt64(&h.metrics.TotalReads),
	}
}
