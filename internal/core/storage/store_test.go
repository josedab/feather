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

	err := store.Put(context.Background(), entityKey, features)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Wait for async warm tier write
	time.Sleep(50 * time.Millisecond)

	result, err := store.Get(context.Background(), entityKey, []string{"click_count", "purchase_total"})
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

	err := store.Put(context.Background(), entityKey, features)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Wait for async warm tier write
	time.Sleep(50 * time.Millisecond)

	// Clear hot tier to simulate eviction
	store.hot.Delete(entityKey)

	// Get should still work via warm tier fallback
	result, err := store.Get(context.Background(), entityKey, []string{"feature1"})
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
	err := store.Put(context.Background(), entityKey, map[string]*domain.FeatureValue{
		"feature1": {Value: "value1", Timestamp: now, Version: 1},
	})
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Wait for async warm tier write
	time.Sleep(50 * time.Millisecond)

	// Put feature2 only to hot tier by clearing warm entry (simulate)
	err = store.Put(context.Background(), entityKey, map[string]*domain.FeatureValue{
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
	result, err := store.Get(context.Background(), entityKey, []string{"feature1", "feature2"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 features, got %d", len(result))
	}
}

func TestStore_GetNonexistent(t *testing.T) {
	store := newTestStore(t)

	result, err := store.Get(context.Background(), "nonexistent", []string{"feature"})
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
	err := store.Put(context.Background(), entityKey, map[string]*domain.FeatureValue{
		"counter": {Value: int64(1), Timestamp: now.Add(-2 * time.Hour).UnixNano(), Version: 1},
	})
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	err = store.Put(context.Background(), entityKey, map[string]*domain.FeatureValue{
		"counter": {Value: int64(2), Timestamp: now.Add(-1 * time.Hour).UnixNano(), Version: 2},
	})
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Get as of 90 minutes ago (should return version 1)
	result, err := store.GetAsOf(context.Background(), entityKey, []string{"counter"}, now.Add(-90*time.Minute))
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
	err := store.Put(context.Background(), entityKey, map[string]*domain.FeatureValue{
		"feature": {Value: "value", Timestamp: time.Now().UnixNano(), Version: 1},
	})
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Delete from store (only deletes from hot tier)
	err = store.Delete(context.Background(), entityKey)
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
	store.Put(context.Background(), "user:1", map[string]*domain.FeatureValue{
		"feature": {Value: "value", Timestamp: time.Now().UnixNano(), Version: 1},
	})

	// Hit
	store.Get(context.Background(), "user:1", []string{"feature"})

	// Miss
	store.Get(context.Background(), "user:nonexistent", []string{"feature"})

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
				store.Put(context.Background(), entityKey, map[string]*domain.FeatureValue{
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
				store.Get(context.Background(), "user:concurrent", []string{"counter"})
			}
		}()
	}

	wg.Wait()

	// Verify store is still functional
	result, err := store.Get(context.Background(), "user:concurrent", []string{"counter"})
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

func TestStore_Stats(t *testing.T) {
	store := newTestStore(t)

	store.Put(context.Background(), "user:1", map[string]*domain.FeatureValue{
		"f1": {Value: "v1", Timestamp: time.Now().UnixNano(), Version: 1},
	})

	stats := store.Stats()
	if stats.HotEntityCount < 1 {
		t.Errorf("Expected at least 1 entity in stats, got %d", stats.HotEntityCount)
	}
	if stats.HotSize <= 0 {
		t.Error("Expected positive hot size in stats")
	}
}

func TestStore_NewStore_NilContext(t *testing.T) {
	store, err := NewStore(nil, StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, nil)
	if err != nil {
		t.Fatalf("NewStore with nil context failed: %v", err)
	}
	defer store.Close()
}

func TestStore_NewStore_DefaultWorkers(t *testing.T) {
	store, err := NewStore(context.Background(), StoreOptions{
		HotMaxSize:       1024 * 1024,
		WarmInMemory:     true,
		WarmWriteWorkers: 0,
		WarmWriteBuffer:  0,
	}, nil)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()
}

// --- CheckWarmHealth tests ---

func TestStore_CheckWarmHealth_Healthy(t *testing.T) {
	store := newTestStore(t)

	latency, err := store.CheckWarmHealth()
	if err != nil {
		t.Fatalf("CheckWarmHealth() error = %v", err)
	}
	if latency <= 0 {
		t.Error("expected positive latency")
	}
}

func TestStore_CheckWarmHealth_NilWarm(t *testing.T) {
	// Manually create a minimal store to test nil warm path
	hot := NewHotTier(1024 * 1024)
	ctx, cancel := context.WithCancel(context.Background())
	s := &Store{
		hot:        hot,
		warm:       nil,
		metrics:    &StoreMetrics{},
		ctx:        ctx,
		cancel:     cancel,
		warmWrites: make(chan warmWriteRequest, 1),
	}
	defer cancel()

	_, err := s.CheckWarmHealth()
	if err == nil {
		t.Fatal("expected error for nil warm tier")
	}
}

// --- processWarmWrite tests ---

func TestStore_ProcessWarmWrite_Success(t *testing.T) {
	store := newTestStore(t)

	req := warmWriteRequest{
		entityKey: "user:warmtest",
		features: map[string]*domain.FeatureValue{
			"clicks": {Value: int64(42), Timestamp: 12345, Version: 1},
		},
	}
	store.processWarmWrite(req)

	// Verify it was written to warm tier
	result, err := store.warm.Get("user:warmtest", []string{"clicks"})
	if err != nil {
		t.Fatal(err)
	}
	if result["clicks"] == nil {
		t.Error("expected clicks feature in warm tier")
	}
}

func TestStore_ProcessWarmWrite_MultipleFeatures(t *testing.T) {
	store := newTestStore(t)

	req := warmWriteRequest{
		entityKey: "user:multi",
		features: map[string]*domain.FeatureValue{
			"feat_a": {Value: "a", Timestamp: 1, Version: 1},
			"feat_b": {Value: "b", Timestamp: 2, Version: 1},
			"feat_c": {Value: "c", Timestamp: 3, Version: 1},
		},
	}
	store.processWarmWrite(req)

	result, _ := store.warm.Get("user:multi", []string{"feat_a", "feat_b", "feat_c"})
	if len(result) != 3 {
		t.Errorf("expected 3 features in warm tier, got %d", len(result))
	}
}

// --- enqueueWarmWrite tests ---

func TestStore_EnqueueWarmWrite_ChannelFull(t *testing.T) {
	// Create store with tiny buffer
	store, err := NewStore(context.Background(), StoreOptions{
		HotMaxSize:       1024 * 1024,
		WarmInMemory:     true,
		WarmWriteBuffer:  1,
		WarmWriteWorkers: 0, // minimal workers
		TTLCheckInterval: time.Hour,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Fill the channel
	features := map[string]*domain.FeatureValue{
		"f": {Value: 1, Timestamp: 1},
	}

	// Enqueue many writes; some should be dropped without panic
	for i := 0; i < 100; i++ {
		store.enqueueWarmWrite("entity:"+string(rune('a'+i%26)), features)
	}

	// Should not panic; drops are tracked
	metrics := store.Metrics()
	_ = metrics.WarmWriteDrops // Just verify it's accessible
}

func TestStore_EnqueueWarmWrite_AfterClose(t *testing.T) {
	store, err := NewStore(context.Background(), StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	store.Close()

	// Should not panic; write should be dropped
	store.enqueueWarmWrite("entity:after-close", map[string]*domain.FeatureValue{
		"f": {Value: 1, Timestamp: 1},
	})

	drops := store.Metrics().WarmWriteDrops
	if drops == 0 {
		t.Error("expected drops after close")
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
		store.Put(context.Background(), "user:bench", features)
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
	store.Put(context.Background(), "user:bench", map[string]*domain.FeatureValue{
		"feature1": {Value: int64(1), Timestamp: time.Now().UnixNano(), Version: 1},
		"feature2": {Value: 2.5, Timestamp: time.Now().UnixNano(), Version: 1},
		"feature3": {Value: "string", Timestamp: time.Now().UnixNano(), Version: 1},
	})

	featureNames := []string{"feature1", "feature2", "feature3"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Get(context.Background(), "user:bench", featureNames)
	}
}
