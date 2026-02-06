package tenant

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/feather-store/feather/internal/domain"
)

func setupPartitionTest(t *testing.T) (*TenantRegistry, *PartitionedHotTier) {
	t.Helper()

	registry := NewTenantRegistry()

	// Create test tenants
	tenant1 := &Tenant{
		ID:   "tenant-1",
		Name: "Tenant 1",
		Quotas: TenantQuotas{
			MaxHotTierBytes: 10000,
		},
	}
	tenant2 := &Tenant{
		ID:   "tenant-2",
		Name: "Tenant 2",
		Quotas: TenantQuotas{
			MaxHotTierBytes: 5000,
		},
	}
	registry.CreateTenant(tenant1)
	registry.CreateTenant(tenant2)

	hotTier := NewPartitionedHotTier(100000, registry)

	return registry, hotTier
}

func createFeatureValue(value interface{}) *domain.FeatureValue {
	return &domain.FeatureValue{
		Value:     value,
		Version:   1,
		Timestamp: time.Now().UnixNano(),
	}
}

func TestNewPartitionedHotTier(t *testing.T) {
	registry := NewTenantRegistry()
	hotTier := NewPartitionedHotTier(100000, registry)
	assert.NotNil(t, hotTier)
}

func TestPartitionedHotTier_Put_Get(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx := WithTenant(context.Background(), "tenant-1")
	features := map[string]*domain.FeatureValue{
		"feature_a": createFeatureValue(1.5),
		"feature_b": createFeatureValue("hello"),
	}

	err := hotTier.Put(ctx, "entity-1", features)
	require.NoError(t, err)

	// Get specific features
	result, err := hotTier.Get(ctx, "entity-1", []string{"feature_a", "feature_b"})
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, 1.5, result["feature_a"].Value)
	assert.Equal(t, "hello", result["feature_b"].Value)
}

func TestPartitionedHotTier_Put_DefaultTenant(t *testing.T) {
	registry, hotTier := setupPartitionTest(t)

	// Create a default tenant for operations without tenant context
	defaultTenant := &Tenant{
		ID:   "default",
		Name: "Default Tenant",
		Quotas: TenantQuotas{
			MaxHotTierBytes: 10000,
		},
	}
	registry.CreateTenant(defaultTenant)

	ctx := context.Background() // No tenant - will use default
	features := map[string]*domain.FeatureValue{
		"feature_a": createFeatureValue(1.0),
	}

	err := hotTier.Put(ctx, "entity-1", features)
	require.NoError(t, err)

	result, err := hotTier.Get(ctx, "entity-1", []string{"feature_a"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestPartitionedHotTier_Get_EntityNotFound(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx := WithTenant(context.Background(), "tenant-1")
	_, err := hotTier.Get(ctx, "nonexistent", []string{"feature_a"})
	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrEntityNotFound)
}

func TestPartitionedHotTier_Get_PartialFeatures(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx := WithTenant(context.Background(), "tenant-1")
	features := map[string]*domain.FeatureValue{
		"feature_a": createFeatureValue(1.0),
	}
	hotTier.Put(ctx, "entity-1", features)

	// Request features including one that doesn't exist
	result, err := hotTier.Get(ctx, "entity-1", []string{"feature_a", "feature_b"})
	require.NoError(t, err)
	assert.Len(t, result, 1) // Only feature_a exists
}

func TestPartitionedHotTier_GetAll(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx := WithTenant(context.Background(), "tenant-1")
	features := map[string]*domain.FeatureValue{
		"feature_a": createFeatureValue(1.0),
		"feature_b": createFeatureValue(2.0),
		"feature_c": createFeatureValue(3.0),
	}
	hotTier.Put(ctx, "entity-1", features)

	result, err := hotTier.GetAll(ctx, "entity-1")
	require.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestPartitionedHotTier_GetAll_NotFound(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx := WithTenant(context.Background(), "tenant-1")
	_, err := hotTier.GetAll(ctx, "nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrEntityNotFound)
}

func TestPartitionedHotTier_TenantIsolation(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx1 := WithTenant(context.Background(), "tenant-1")
	ctx2 := WithTenant(context.Background(), "tenant-2")

	// Store same entity key for different tenants
	features1 := map[string]*domain.FeatureValue{
		"feature_a": createFeatureValue(1.0),
	}
	features2 := map[string]*domain.FeatureValue{
		"feature_a": createFeatureValue(2.0),
	}

	hotTier.Put(ctx1, "entity-1", features1)
	hotTier.Put(ctx2, "entity-1", features2)

	// Verify isolation
	result1, _ := hotTier.Get(ctx1, "entity-1", []string{"feature_a"})
	result2, _ := hotTier.Get(ctx2, "entity-1", []string{"feature_a"})

	assert.Equal(t, 1.0, result1["feature_a"].Value)
	assert.Equal(t, 2.0, result2["feature_a"].Value)
}

func TestPartitionedHotTier_Delete(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx := WithTenant(context.Background(), "tenant-1")
	features := map[string]*domain.FeatureValue{
		"feature_a": createFeatureValue(1.0),
	}
	hotTier.Put(ctx, "entity-1", features)

	err := hotTier.Delete(ctx, "entity-1")
	require.NoError(t, err)

	_, err = hotTier.Get(ctx, "entity-1", []string{"feature_a"})
	assert.Error(t, err)
}

func TestPartitionedHotTier_Delete_NonExistent(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx := WithTenant(context.Background(), "tenant-1")
	err := hotTier.Delete(ctx, "nonexistent")
	assert.NoError(t, err) // Delete is idempotent
}

func TestPartitionedHotTier_VersionUpdate(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx := WithTenant(context.Background(), "tenant-1")

	// Initial put
	features1 := map[string]*domain.FeatureValue{
		"feature_a": {Value: 1.0, Version: 1, Timestamp: time.Now().UnixNano()},
	}
	hotTier.Put(ctx, "entity-1", features1)

	// Update with newer version
	features2 := map[string]*domain.FeatureValue{
		"feature_a": {Value: 2.0, Version: 2, Timestamp: time.Now().UnixNano()},
	}
	hotTier.Put(ctx, "entity-1", features2)

	result, _ := hotTier.Get(ctx, "entity-1", []string{"feature_a"})
	assert.Equal(t, 2.0, result["feature_a"].Value)

	// Try update with older version (should not update)
	features3 := map[string]*domain.FeatureValue{
		"feature_a": {Value: 0.5, Version: 1, Timestamp: time.Now().UnixNano()},
	}
	hotTier.Put(ctx, "entity-1", features3)

	result, _ = hotTier.Get(ctx, "entity-1", []string{"feature_a"})
	assert.Equal(t, 2.0, result["feature_a"].Value) // Still the newer version
}

func TestPartitionedHotTier_Size(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	// Initially empty
	assert.Equal(t, int64(0), hotTier.Size())

	ctx := WithTenant(context.Background(), "tenant-1")
	features := map[string]*domain.FeatureValue{
		"feature_a": createFeatureValue(1.0),
	}
	hotTier.Put(ctx, "entity-1", features)

	// Should have some size now
	assert.Greater(t, hotTier.Size(), int64(0))
}

func TestPartitionedHotTier_TenantSize(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx := WithTenant(context.Background(), "tenant-1")
	features := map[string]*domain.FeatureValue{
		"feature_a": createFeatureValue(1.0),
	}
	hotTier.Put(ctx, "entity-1", features)

	assert.Greater(t, hotTier.TenantSize("tenant-1"), int64(0))
	assert.Equal(t, int64(0), hotTier.TenantSize("tenant-2")) // No data yet
	assert.Equal(t, int64(0), hotTier.TenantSize("nonexistent"))
}

func TestPartitionedHotTier_EntityCount(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx := WithTenant(context.Background(), "tenant-1")

	// Add multiple entities
	for i := 0; i < 5; i++ {
		features := map[string]*domain.FeatureValue{
			"feature_a": createFeatureValue(float64(i)),
		}
		hotTier.Put(ctx, string(rune('a'+i)), features)
	}

	assert.Equal(t, 5, hotTier.EntityCount())
}

func TestPartitionedHotTier_TenantEntityCount(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx1 := WithTenant(context.Background(), "tenant-1")
	ctx2 := WithTenant(context.Background(), "tenant-2")

	// Add entities for tenant-1
	for i := 0; i < 3; i++ {
		features := map[string]*domain.FeatureValue{
			"feature_a": createFeatureValue(float64(i)),
		}
		hotTier.Put(ctx1, string(rune('a'+i)), features)
	}

	// Add entities for tenant-2
	for i := 0; i < 2; i++ {
		features := map[string]*domain.FeatureValue{
			"feature_a": createFeatureValue(float64(i)),
		}
		hotTier.Put(ctx2, string(rune('x'+i)), features)
	}

	assert.Equal(t, int64(3), hotTier.TenantEntityCount("tenant-1"))
	assert.Equal(t, int64(2), hotTier.TenantEntityCount("tenant-2"))
	assert.Equal(t, int64(0), hotTier.TenantEntityCount("nonexistent"))
}

func TestPartitionedHotTier_ExpireOlderThan(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx := WithTenant(context.Background(), "tenant-1")

	// Add features with old timestamp
	oldFeatures := map[string]*domain.FeatureValue{
		"old_feature": {
			Value:     1.0,
			Version:   1,
			Timestamp: time.Now().Add(-2 * time.Hour).UnixNano(),
		},
	}
	hotTier.Put(ctx, "old-entity", oldFeatures)

	// Add features with recent timestamp
	newFeatures := map[string]*domain.FeatureValue{
		"new_feature": {
			Value:     2.0,
			Version:   1,
			Timestamp: time.Now().UnixNano(),
		},
	}
	hotTier.Put(ctx, "new-entity", newFeatures)

	// Expire features older than 1 hour
	expired := hotTier.ExpireOlderThan(1 * time.Hour)
	assert.Greater(t, expired, 0)

	// Old features should be gone
	_, err := hotTier.Get(ctx, "old-entity", []string{"old_feature"})
	assert.Error(t, err) // Entity removed

	// New features should remain
	result, err := hotTier.Get(ctx, "new-entity", []string{"new_feature"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestPartitionedHotTier_Metrics(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx := WithTenant(context.Background(), "tenant-1")

	// Put and get to generate metrics
	features := map[string]*domain.FeatureValue{
		"feature_a": createFeatureValue(1.0),
	}
	hotTier.Put(ctx, "entity-1", features)
	hotTier.Get(ctx, "entity-1", []string{"feature_a"})
	hotTier.Get(ctx, "entity-1", []string{"nonexistent"})

	metrics := hotTier.Metrics()
	assert.Greater(t, metrics["total_hits"].(int64), int64(0))
	assert.Greater(t, metrics["total_misses"].(int64), int64(0))
	assert.Contains(t, metrics, "partition_count")
	assert.Contains(t, metrics, "total_size")
	assert.Contains(t, metrics, "total_entities")
}

func TestPartitionedHotTier_PartitionStats(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx1 := WithTenant(context.Background(), "tenant-1")
	ctx2 := WithTenant(context.Background(), "tenant-2")

	// Add data to create partitions
	hotTier.Put(ctx1, "e1", map[string]*domain.FeatureValue{"f": createFeatureValue(1.0)})
	hotTier.Put(ctx2, "e2", map[string]*domain.FeatureValue{"f": createFeatureValue(2.0)})

	stats := hotTier.PartitionStats()
	assert.GreaterOrEqual(t, len(stats), 2)

	// Check stats structure
	for _, s := range stats {
		assert.Contains(t, s, "tenant_id")
		assert.Contains(t, s, "current_size")
		assert.Contains(t, s, "max_size")
		assert.Contains(t, s, "entity_count")
		assert.Contains(t, s, "utilization")
	}
}

func TestPartitionedHotTier_ResizePartition(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx := WithTenant(context.Background(), "tenant-1")
	features := map[string]*domain.FeatureValue{
		"feature_a": createFeatureValue(1.0),
	}
	hotTier.Put(ctx, "entity-1", features)

	// Resize partition
	err := hotTier.ResizePartition("tenant-1", 20000)
	assert.NoError(t, err)

	// Check partition stats
	stats := hotTier.PartitionStats()
	for _, s := range stats {
		if s["tenant_id"] == "tenant-1" {
			assert.Equal(t, int64(20000), s["max_size"])
		}
	}
}

func TestPartitionedHotTier_ResizePartition_NotFound(t *testing.T) {
	registry := NewTenantRegistry()
	hotTier := NewPartitionedHotTier(100000, registry)

	err := hotTier.ResizePartition("nonexistent", 10000)
	assert.Error(t, err)
}

func TestPartitionedHotTier_DeletePartition(t *testing.T) {
	_, hotTier := setupPartitionTest(t)

	ctx := WithTenant(context.Background(), "tenant-1")
	features := map[string]*domain.FeatureValue{
		"feature_a": createFeatureValue(1.0),
	}
	hotTier.Put(ctx, "entity-1", features)

	err := hotTier.DeletePartition("tenant-1")
	assert.NoError(t, err)

	// Data should be gone
	_, err = hotTier.Get(ctx, "entity-1", []string{"feature_a"})
	assert.Error(t, err)

	// Partition size should be 0
	assert.Equal(t, int64(0), hotTier.TenantSize("tenant-1"))
}

func TestPartitionedHotTier_DeletePartition_NotFound(t *testing.T) {
	registry := NewTenantRegistry()
	hotTier := NewPartitionedHotTier(100000, registry)

	err := hotTier.DeletePartition("nonexistent")
	assert.Error(t, err)
}

func TestPartitionedHotTier_QuotaCheck(t *testing.T) {
	registry := NewTenantRegistry()
	// Note: Must set MaxFeatures != 0 to prevent defaults from overwriting quotas
	tenant := &Tenant{
		ID:   "limited-tenant",
		Name: "Limited",
		Quotas: TenantQuotas{
			MaxFeatures:     100,
			MaxHotTierBytes: 100, // Very small quota
		},
	}
	registry.CreateTenant(tenant)

	hotTier := NewPartitionedHotTier(100000, registry)
	ctx := WithTenant(context.Background(), "limited-tenant")

	// Update usage to at the limit (estimateSize returns ~100 per feature)
	registry.UpdateUsage("limited-tenant", func(u *TenantUsage) {
		u.HotTierBytes = 100 // At limit
	})

	// Try to put - should fail quota check since usage + estimated > max
	features := map[string]*domain.FeatureValue{
		"feature_a": createFeatureValue(1.0),
		"feature_b": createFeatureValue(2.0),
	}
	err := hotTier.Put(ctx, "entity-1", features)
	assert.Error(t, err) // Quota exceeded
}

func TestPartitionMetrics(t *testing.T) {
	metrics := &PartitionMetrics{
		TenantHits:   make(map[string]*int64),
		TenantMisses: make(map[string]*int64),
	}
	assert.NotNil(t, metrics)
}
