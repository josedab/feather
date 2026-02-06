package tenant

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/domain"
)

// ErrPartitionNotFound indicates a tenant partition is missing.
var ErrPartitionNotFound = errors.New("partition not found")

// PartitionedHotTier provides tenant-aware hot tier partitioning.
// It wraps the standard hot tier with per-tenant memory quotas and isolation.
type PartitionedHotTier struct {
	mu sync.RWMutex

	// Global config
	totalMaxSize int64

	// Per-tenant partitions
	partitions map[string]*tenantPartition

	// Registry for quota checking
	registry *TenantRegistry

	// Metrics
	metrics *PartitionMetrics
}

// tenantPartition represents a tenant's isolated hot tier partition.
type tenantPartition struct {
	tenantID    string
	data        map[string]*entityData // entityKey -> features
	maxSize     int64
	curSize     int64
	entityCount int64
	mu          sync.RWMutex
}

// entityData holds all features for a single entity.
type entityData struct {
	features map[string]*domain.FeatureValue
	mu       sync.RWMutex
}

// PartitionMetrics tracks metrics for the partitioned hot tier.
type PartitionMetrics struct {
	TotalHits         int64
	TotalMisses       int64
	TotalEvictions    int64
	TenantHits        map[string]*int64
	TenantMisses      map[string]*int64
	CrossTenantDenied int64
	mu                sync.RWMutex
}

// NewPartitionedHotTier creates a new partitioned hot tier.
func NewPartitionedHotTier(totalMaxSize int64, registry *TenantRegistry) *PartitionedHotTier {
	return &PartitionedHotTier{
		totalMaxSize: totalMaxSize,
		partitions:   make(map[string]*tenantPartition),
		registry:     registry,
		metrics: &PartitionMetrics{
			TenantHits:   make(map[string]*int64),
			TenantMisses: make(map[string]*int64),
		},
	}
}

// getOrCreatePartition gets or creates a partition for a tenant.
func (h *PartitionedHotTier) getOrCreatePartition(tenantID string) *tenantPartition {
	h.mu.RLock()
	partition, exists := h.partitions[tenantID]
	h.mu.RUnlock()

	if exists {
		return partition
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Double-check after acquiring write lock
	if partition, exists = h.partitions[tenantID]; exists {
		return partition
	}

	// Get tenant quota
	maxSize := h.totalMaxSize / 10 // Default to 10% of total
	if tenant, err := h.registry.GetTenant(tenantID); err == nil {
		if tenant.Quotas.MaxHotTierBytes > 0 {
			maxSize = tenant.Quotas.MaxHotTierBytes
		}
	}

	partition = &tenantPartition{
		tenantID: tenantID,
		data:     make(map[string]*entityData),
		maxSize:  maxSize,
	}

	h.partitions[tenantID] = partition

	// Initialize metrics
	h.metrics.mu.Lock()
	hits := int64(0)
	misses := int64(0)
	h.metrics.TenantHits[tenantID] = &hits
	h.metrics.TenantMisses[tenantID] = &misses
	h.metrics.mu.Unlock()

	return partition
}

// Get retrieves features for an entity within a tenant partition.
func (h *PartitionedHotTier) Get(ctx context.Context, entityKey string, features []string) (map[string]*domain.FeatureValue, error) {
	tenantID := TenantFromContext(ctx)
	if tenantID == "" {
		tenantID = "default"
	}

	partition := h.getOrCreatePartition(tenantID)

	partition.mu.RLock()
	entity, ok := partition.data[entityKey]
	partition.mu.RUnlock()

	if !ok {
		atomic.AddInt64(&h.metrics.TotalMisses, 1)
		h.incrementTenantMisses(tenantID)
		return nil, domain.ErrEntityNotFound
	}

	entity.mu.RLock()
	defer entity.mu.RUnlock()

	result := make(map[string]*domain.FeatureValue, len(features))
	for _, f := range features {
		if val, ok := entity.features[f]; ok {
			result[f] = val
			atomic.AddInt64(&h.metrics.TotalHits, 1)
			h.incrementTenantHits(tenantID)
		} else {
			atomic.AddInt64(&h.metrics.TotalMisses, 1)
			h.incrementTenantMisses(tenantID)
		}
	}

	return result, nil
}

// GetAll retrieves all features for an entity.
func (h *PartitionedHotTier) GetAll(ctx context.Context, entityKey string) (map[string]*domain.FeatureValue, error) {
	tenantID := TenantFromContext(ctx)
	if tenantID == "" {
		tenantID = "default"
	}

	partition := h.getOrCreatePartition(tenantID)

	partition.mu.RLock()
	entity, ok := partition.data[entityKey]
	partition.mu.RUnlock()

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

// Put stores features for an entity within a tenant partition.
func (h *PartitionedHotTier) Put(ctx context.Context, entityKey string, features map[string]*domain.FeatureValue) error {
	tenantID := TenantFromContext(ctx)
	if tenantID == "" {
		tenantID = "default"
	}

	// Check tenant quota
	if err := h.registry.CheckQuota(tenantID, "hot_tier", estimateSize(features)); err != nil {
		return err
	}

	partition := h.getOrCreatePartition(tenantID)

	partition.mu.Lock()
	entity, ok := partition.data[entityKey]
	if !ok {
		entity = &entityData{
			features: make(map[string]*domain.FeatureValue),
		}
		partition.data[entityKey] = entity
		atomic.AddInt64(&partition.entityCount, 1)
	}
	partition.mu.Unlock()

	entity.mu.Lock()
	defer entity.mu.Unlock()

	for name, val := range features {
		existing, exists := entity.features[name]
		if !exists || val.Version > existing.Version {
			entity.features[name] = val
			// Track size
			atomic.AddInt64(&partition.curSize, 100)
		}
	}

	// Check if eviction needed
	if atomic.LoadInt64(&partition.curSize) > partition.maxSize {
		go h.evictPartition(partition)
	}

	return nil
}

// Delete removes an entity from a tenant partition.
func (h *PartitionedHotTier) Delete(ctx context.Context, entityKey string) error {
	tenantID := TenantFromContext(ctx)
	if tenantID == "" {
		tenantID = "default"
	}

	partition := h.getOrCreatePartition(tenantID)

	partition.mu.Lock()
	defer partition.mu.Unlock()

	if entity, ok := partition.data[entityKey]; ok {
		atomic.AddInt64(&partition.curSize, -int64(len(entity.features)*100))
		atomic.AddInt64(&partition.entityCount, -1)
		delete(partition.data, entityKey)
		atomic.AddInt64(&h.metrics.TotalEvictions, 1)
	}

	return nil
}

// ExpireOlderThan removes features older than maxAge from all partitions.
func (h *PartitionedHotTier) ExpireOlderThan(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge).UnixNano()
	expired := 0

	h.mu.RLock()
	partitions := make([]*tenantPartition, 0, len(h.partitions))
	for _, p := range h.partitions {
		partitions = append(partitions, p)
	}
	h.mu.RUnlock()

	for _, partition := range partitions {
		expired += h.expirePartition(partition, cutoff)
	}

	return expired
}

func (h *PartitionedHotTier) expirePartition(partition *tenantPartition, cutoff int64) int {
	expired := 0

	partition.mu.Lock()
	defer partition.mu.Unlock()

	for entityKey, entity := range partition.data {
		entity.mu.Lock()
		for featureName, val := range entity.features {
			if val.Timestamp < cutoff {
				delete(entity.features, featureName)
				expired++
			}
		}
		if len(entity.features) == 0 {
			delete(partition.data, entityKey)
			atomic.AddInt64(&partition.entityCount, -1)
		}
		entity.mu.Unlock()
	}

	return expired
}

func (h *PartitionedHotTier) evictPartition(partition *tenantPartition) {
	partition.mu.Lock()
	defer partition.mu.Unlock()

	// Simple LRU approximation - remove oldest 10%
	targetSize := partition.maxSize * 9 / 10
	if atomic.LoadInt64(&partition.curSize) <= targetSize {
		return
	}

	// Find entities to evict (simplified - just remove some)
	toRemove := len(partition.data) / 10
	if toRemove < 1 {
		toRemove = 1
	}

	removed := 0
	for entityKey, entity := range partition.data {
		if removed >= toRemove {
			break
		}
		atomic.AddInt64(&partition.curSize, -int64(len(entity.features)*100))
		atomic.AddInt64(&partition.entityCount, -1)
		delete(partition.data, entityKey)
		atomic.AddInt64(&h.metrics.TotalEvictions, 1)
		removed++
	}
}

func (h *PartitionedHotTier) incrementTenantHits(tenantID string) {
	h.metrics.mu.RLock()
	counter := h.metrics.TenantHits[tenantID]
	h.metrics.mu.RUnlock()

	if counter != nil {
		atomic.AddInt64(counter, 1)
	}
}

func (h *PartitionedHotTier) incrementTenantMisses(tenantID string) {
	h.metrics.mu.RLock()
	counter := h.metrics.TenantMisses[tenantID]
	h.metrics.mu.RUnlock()

	if counter != nil {
		atomic.AddInt64(counter, 1)
	}
}

// Size returns total current size across all partitions.
func (h *PartitionedHotTier) Size() int64 {
	var total int64

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, partition := range h.partitions {
		total += atomic.LoadInt64(&partition.curSize)
	}
	return total
}

// TenantSize returns current size for a specific tenant.
func (h *PartitionedHotTier) TenantSize(tenantID string) int64 {
	h.mu.RLock()
	partition, exists := h.partitions[tenantID]
	h.mu.RUnlock()

	if !exists {
		return 0
	}
	return atomic.LoadInt64(&partition.curSize)
}

// EntityCount returns total entity count across all partitions.
func (h *PartitionedHotTier) EntityCount() int {
	var total int64

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, partition := range h.partitions {
		total += atomic.LoadInt64(&partition.entityCount)
	}
	return int(total)
}

// TenantEntityCount returns entity count for a specific tenant.
func (h *PartitionedHotTier) TenantEntityCount(tenantID string) int64 {
	h.mu.RLock()
	partition, exists := h.partitions[tenantID]
	h.mu.RUnlock()

	if !exists {
		return 0
	}
	return atomic.LoadInt64(&partition.entityCount)
}

// Metrics returns current metrics.
func (h *PartitionedHotTier) Metrics() map[string]interface{} {
	h.metrics.mu.RLock()
	tenantHits := make(map[string]int64)
	tenantMisses := make(map[string]int64)
	for k, v := range h.metrics.TenantHits {
		tenantHits[k] = atomic.LoadInt64(v)
	}
	for k, v := range h.metrics.TenantMisses {
		tenantMisses[k] = atomic.LoadInt64(v)
	}
	h.metrics.mu.RUnlock()

	return map[string]interface{}{
		"total_hits":          atomic.LoadInt64(&h.metrics.TotalHits),
		"total_misses":        atomic.LoadInt64(&h.metrics.TotalMisses),
		"total_evictions":     atomic.LoadInt64(&h.metrics.TotalEvictions),
		"cross_tenant_denied": atomic.LoadInt64(&h.metrics.CrossTenantDenied),
		"tenant_hits":         tenantHits,
		"tenant_misses":       tenantMisses,
		"partition_count":     len(h.partitions),
		"total_size":          h.Size(),
		"total_entities":      h.EntityCount(),
	}
}

// PartitionStats returns stats for all partitions.
func (h *PartitionedHotTier) PartitionStats() []map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := make([]map[string]interface{}, 0, len(h.partitions))
	for tenantID, partition := range h.partitions {
		stats = append(stats, map[string]interface{}{
			"tenant_id":    tenantID,
			"current_size": atomic.LoadInt64(&partition.curSize),
			"max_size":     partition.maxSize,
			"entity_count": atomic.LoadInt64(&partition.entityCount),
			"utilization":  float64(atomic.LoadInt64(&partition.curSize)) / float64(partition.maxSize) * 100,
		})
	}
	return stats
}

// ResizePartition resizes a tenant's partition.
func (h *PartitionedHotTier) ResizePartition(tenantID string, newMaxSize int64) error {
	h.mu.Lock()
	partition, exists := h.partitions[tenantID]
	h.mu.Unlock()

	if !exists {
		return errors.New("partition not found")
	}

	partition.mu.Lock()
	partition.maxSize = newMaxSize
	partition.mu.Unlock()

	// Trigger eviction if over new limit
	if atomic.LoadInt64(&partition.curSize) > newMaxSize {
		go h.evictPartition(partition)
	}

	return nil
}

// DeletePartition removes a tenant's partition entirely.
func (h *PartitionedHotTier) DeletePartition(tenantID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.partitions[tenantID]; !exists {
		return fmt.Errorf("%w: %s", ErrPartitionNotFound, tenantID)
	}

	delete(h.partitions, tenantID)

	h.metrics.mu.Lock()
	delete(h.metrics.TenantHits, tenantID)
	delete(h.metrics.TenantMisses, tenantID)
	h.metrics.mu.Unlock()

	return nil
}

func estimateSize(features map[string]*domain.FeatureValue) int64 {
	// Approximate 100 bytes per feature value
	return int64(len(features) * 100)
}
