package storage

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/domain"
)

// Object pools for reducing allocations in hot paths.
var (
	// Pool for feature name slices used in batch operations
	featureNameSlicePool = sync.Pool{
		New: func() interface{} {
			s := make([]string, 0, 32)
			return &s
		},
	}

	// Pool for entity key slices
	entityKeySlicePool = sync.Pool{
		New: func() interface{} {
			s := make([]string, 0, 64)
			return &s
		},
	}
)

// entityData holds all features for a single entity.
type entityData struct {
	features map[string]*domain.FeatureValue
	mu       sync.RWMutex
}

// HotTier provides in-memory feature storage optimized for low-latency access.
type HotTier struct {
	// Sharded map for reduced lock contention
	shards    [256]*shard
	arena     *Arena
	maxSize   int64
	curSize   int64
	evictChan chan string
	metrics   *HotTierMetrics
}

type shard struct {
	data map[string]*entityData
	mu   sync.RWMutex
}

// HotTierMetrics tracks hot tier performance.
type HotTierMetrics struct {
	Hits       int64
	Misses     int64
	Evictions  int64
	TotalReads int64
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
