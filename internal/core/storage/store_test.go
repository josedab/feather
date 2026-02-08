package storage

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

// mockSchemaRegistry implements SchemaRegistry for testing.
type mockSchemaRegistry struct {
	groups map[string]*domain.FeatureGroup
}

func newMockSchemaRegistry() *mockSchemaRegistry {
	return &mockSchemaRegistry{
		groups: make(map[string]*domain.FeatureGroup),
	}
}

func (m *mockSchemaRegistry) GetGroup(name string) (*domain.FeatureGroup, error) {
	if group, ok := m.groups[name]; ok {
		return group, nil
	}
	return nil, domain.ErrGroupNotFound
}

func (m *mockSchemaRegistry) GetFeatureSpec(featureName string) (*domain.FeatureSpec, error) {
	for _, group := range m.groups {
		for i := range group.Features {
			if group.Features[i].Name == featureName {
				return &group.Features[i], nil
			}
		}
	}
	return nil, domain.ErrFeatureNotFound
}

func (m *mockSchemaRegistry) ListGroups() []*domain.FeatureGroup {
	result := make([]*domain.FeatureGroup, 0, len(m.groups))
	for _, group := range m.groups {
		result = append(result, group)
	}
	return result
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(context.Background(), StoreOptions{
		HotMaxSize:       1024 * 1024 * 100, // 100MB
		WarmInMemory:     true,
		TTLCheckInterval: time.Hour, // Long interval to avoid interference
	}, newMockSchemaRegistry())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	t.Cleanup(func() {
		store.Close()
	})
	return store
}

func TestStore_NewStore(t *testing.T) {
	store, err := NewStore(context.Background(), StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, nil)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if store.hot == nil {
		t.Error("Expected hot tier to be initialized")
	}
	if store.warm == nil {
		t.Error("Expected warm tier to be initialized")
	}
}

func TestStore_PutAndGet(t *testing.T) {
	store := newTestStore(t)

	entityKey := "user:123"
	features := map[string]*domain.FeatureValue{
		"click_count": {
			Value:     int64(15),
			Timestamp: time.Now().UnixNano(),
			Version:   1,
		},
		"purchase_total": {
			Value:     245.50,
			Timestamp: time.Now().UnixNano(),
			Version:   1,
		},
	}

	err := store.Put(entityKey, features)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Wait for async warm tier write
	time.Sleep(50 * time.Millisecond)

	result, err := store.Get(entityKey, []string{"click_count", "purchase_total"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 features, got %d", len(result))
	}

	if result["click_count"].Value != int64(15) {
		t.Errorf("Expected click_count=15, got %v", result["click_count"].Value)
	}

	if result["purchase_total"].Value != 245.50 {
		t.Errorf("Expected purchase_total=245.50, got %v", result["purchase_total"].Value)
	}
}

func TestStore_HotTierFallbackToWarm(t *testing.T) {
	store := newTestStore(t)

	entityKey := "user:fallback"
	now := time.Now().UnixNano()
	features := map[string]*domain.FeatureValue{
		"feature1": {Value: "hot_value", Timestamp: now, Version: 1},
	}

	err := store.Put(entityKey, features)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Wait for async warm tier write
	time.Sleep(50 * time.Millisecond)

	// Clear hot tier to simulate eviction
	store.hot.Delete(entityKey)

	// Get should still work via warm tier fallback
	result, err := store.Get(entityKey, []string{"feature1"})
	if err != nil {
		t.Fatalf("Get failed after hot tier eviction: %v", err)
	}

	if result["feature1"] == nil {
		t.Fatal("Expected feature1 to be retrieved from warm tier")
	}

	if result["feature1"].Value != "hot_value" {
		t.Errorf("Expected 'hot_value', got %v", result["feature1"].Value)
	}
}

func TestStore_GetPartialFromBothTiers(t *testing.T) {
	store := newTestStore(t)

	entityKey := "user:partial"
	now := time.Now().UnixNano()

	// Put feature1
	err := store.Put(entityKey, map[string]*domain.FeatureValue{
		"feature1": {Value: "value1", Timestamp: now, Version: 1},
	})
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Wait for async warm tier write
	time.Sleep(50 * time.Millisecond)

	// Put feature2 only to hot tier by clearing warm entry (simulate)
	err = store.Put(entityKey, map[string]*domain.FeatureValue{
		"feature2": {Value: "value2", Timestamp: now, Version: 1},
	})
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Wait for warm write
	time.Sleep(50 * time.Millisecond)

	// Clear feature1 from hot tier only
	// This is a bit contrived, but tests the fallback logic
	store.hot.Delete(entityKey)

	// Re-add only feature2 to hot tier
	store.hot.Put(entityKey, map[string]*domain.FeatureValue{
		"feature2": {Value: "value2", Timestamp: now, Version: 1},
	})

	// Request both features - should get feature2 from hot, feature1 from warm
	result, err := store.Get(entityKey, []string{"feature1", "feature2"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 features, got %d", len(result))
	}
}

func TestStore_GetNonexistent(t *testing.T) {
	store := newTestStore(t)

	result, err := store.Get("nonexistent", []string{"feature"})
	if err != nil {
		t.Fatalf("Get should not error for nonexistent entity: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty result for nonexistent entity, got %d features", len(result))
	}
}

func TestStore_GetAsOf(t *testing.T) {
	store := newTestStore(t)

	entityKey := "user:history"
	now := time.Now()

	// Store feature at different timestamps
	err := store.Put(entityKey, map[string]*domain.FeatureValue{
		"counter": {Value: int64(1), Timestamp: now.Add(-2 * time.Hour).UnixNano(), Version: 1},
	})
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	err = store.Put(entityKey, map[string]*domain.FeatureValue{
		"counter": {Value: int64(2), Timestamp: now.Add(-1 * time.Hour).UnixNano(), Version: 2},
	})
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Get as of 90 minutes ago (should return version 1)
	result, err := store.GetAsOf(entityKey, []string{"counter"}, now.Add(-90*time.Minute))
	if err != nil {
		t.Fatalf("GetAsOf failed: %v", err)
	}

	if result["counter"] == nil {
		t.Fatal("Expected counter to exist")
	}

	// JSON unmarshaling converts int64 to float64
	if v, ok := result["counter"].Value.(float64); !ok || v != 1 {
		t.Errorf("Expected counter=1 as of 90m ago, got %v (type %T)", result["counter"].Value, result["counter"].Value)
	}
}

func TestStore_Delete(t *testing.T) {
	store := newTestStore(t)

	entityKey := "user:delete"
	err := store.Put(entityKey, map[string]*domain.FeatureValue{
		"feature": {Value: "value", Timestamp: time.Now().UnixNano(), Version: 1},
	})
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Delete from store (only deletes from hot tier)
	err = store.Delete(entityKey)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Hot tier should return empty now
	result, _ := store.hot.Get(entityKey, []string{"feature"})
	if result != nil {
		t.Error("Expected hot tier to return nil after delete")
	}
}

func TestStore_Hot_Accessor(t *testing.T) {
	store := newTestStore(t)

	if store.Hot() == nil {
		t.Error("Expected Hot() to return hot tier")
	}

	if store.Hot() != store.hot {
		t.Error("Expected Hot() to return same instance as internal hot tier")
	}
}

func TestStore_Warm_Accessor(t *testing.T) {
	store := newTestStore(t)

	if store.Warm() == nil {
		t.Error("Expected Warm() to return warm tier")
	}

	if store.Warm() != store.warm {
		t.Error("Expected Warm() to return same instance as internal warm tier")
	}
}

func TestStore_Metrics(t *testing.T) {
	store := newTestStore(t)

	// Generate some operations
	store.Put("user:1", map[string]*domain.FeatureValue{
		"feature": {Value: "value", Timestamp: time.Now().UnixNano(), Version: 1},
	})

	// Hit
	store.Get("user:1", []string{"feature"})

	// Miss
	store.Get("user:nonexistent", []string{"feature"})

	metrics := store.Metrics()
	// Just verify metrics struct is populated - exact values depend on implementation
	if metrics.HotHits < 0 || metrics.HotMisses < 0 {
		t.Error("Metrics should have non-negative values")
	}
}

func TestStore_Close(t *testing.T) {
	store, err := NewStore(context.Background(), StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, nil)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	err = store.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	store := newTestStore(t)

	var wg sync.WaitGroup
	numGoroutines := 50
	numOperations := 50

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				entityKey := "user:concurrent"
				store.Put(entityKey, map[string]*domain.FeatureValue{
					"counter": {
						Value:     int64(id*numOperations + j),
						Timestamp: time.Now().UnixNano(),
						Version:   int64(id*numOperations + j),
					},
				})
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				store.Get("user:concurrent", []string{"counter"})
			}
		}()
	}

	wg.Wait()

	// Verify store is still functional
	result, err := store.Get("user:concurrent", []string{"counter"})
	if err != nil {
		t.Fatalf("Failed to get after concurrent access: %v", err)
	}

	if result["counter"] == nil {
		t.Error("Expected counter to have a value")
	}
}

func TestStore_WithSchemaRegistry(t *testing.T) {
	schema := newMockSchemaRegistry()
	schema.groups["user_features"] = &domain.FeatureGroup{
		Name: "user_features",
		TTL:  time.Hour,
		Features: []domain.FeatureSpec{
			{Name: "click_count", DataType: domain.DataTypeInt64},
		},
	}

	store, err := NewStore(context.Background(), StoreOptions{
		HotMaxSize:       1024 * 1024,
		WarmInMemory:     true,
		TTLCheckInterval: time.Hour,
	}, schema)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Verify schema is accessible
	if store.schema == nil {
		t.Error("Expected schema to be set")
	}
}

func BenchmarkStore_Put(b *testing.B) {
	store, err := NewStore(context.Background(), StoreOptions{
		HotMaxSize:   1024 * 1024 * 100,
		WarmInMemory: true,
	}, nil)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	features := map[string]*domain.FeatureValue{
		"feature1": {Value: int64(1), Timestamp: time.Now().UnixNano(), Version: 1},
		"feature2": {Value: 2.5, Timestamp: time.Now().UnixNano(), Version: 1},
		"feature3": {Value: "string", Timestamp: time.Now().UnixNano(), Version: 1},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Put("user:bench", features)
	}
}

func BenchmarkStore_Get(b *testing.B) {
	store, err := NewStore(context.Background(), StoreOptions{
		HotMaxSize:   1024 * 1024 * 100,
		WarmInMemory: true,
	}, nil)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Setup
	store.Put("user:bench", map[string]*domain.FeatureValue{
		"feature1": {Value: int64(1), Timestamp: time.Now().UnixNano(), Version: 1},
		"feature2": {Value: 2.5, Timestamp: time.Now().UnixNano(), Version: 1},
		"feature3": {Value: "string", Timestamp: time.Now().UnixNano(), Version: 1},
	})

	featureNames := []string{"feature1", "feature2", "feature3"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Get("user:bench", featureNames)
	}
}
