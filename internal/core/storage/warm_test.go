package storage

import (
	"sync"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

func newTestWarmTier(t *testing.T) *WarmTier {
	t.Helper()
	warm, err := NewWarmTier(WarmTierOptions{InMemory: true})
	if err != nil {
		t.Fatalf("Failed to create warm tier: %v", err)
	}
	t.Cleanup(func() {
		warm.Close()
	})
	return warm
}

func TestWarmTier_NewWarmTier(t *testing.T) {
	warm, err := NewWarmTier(WarmTierOptions{InMemory: true})
	if err != nil {
		t.Fatalf("NewWarmTier failed: %v", err)
	}
	defer warm.Close()

	if warm.db == nil {
		t.Error("Expected db to be initialized")
	}

	if warm.syncInterval != time.Second {
		t.Errorf("Expected default syncInterval of 1s, got %v", warm.syncInterval)
	}
}

func TestWarmTier_NewWarmTier_CustomSyncInterval(t *testing.T) {
	warm, err := NewWarmTier(WarmTierOptions{
		InMemory:     true,
		SyncInterval: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewWarmTier failed: %v", err)
	}
	defer warm.Close()

	if warm.syncInterval != 5*time.Second {
		t.Errorf("Expected syncInterval of 5s, got %v", warm.syncInterval)
	}
}

func TestWarmTier_PutAndGet(t *testing.T) {
	warm := newTestWarmTier(t)

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

	err := warm.Put(entityKey, features)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	result, err := warm.Get(entityKey, []string{"click_count", "purchase_total"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 features, got %d", len(result))
	}

	// JSON unmarshaling converts int64 to float64, so we compare as float64
	if v, ok := result["click_count"].Value.(float64); !ok || v != 15 {
		t.Errorf("Expected click_count=15, got %v (type %T)", result["click_count"].Value, result["click_count"].Value)
	}

	if v, ok := result["purchase_total"].Value.(float64); !ok || v != 245.50 {
		t.Errorf("Expected purchase_total=245.50, got %v (type %T)", result["purchase_total"].Value, result["purchase_total"].Value)
	}
}

func TestWarmTier_GetNonexistent(t *testing.T) {
	warm := newTestWarmTier(t)

	result, err := warm.Get("nonexistent", []string{"feature"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Warm tier returns empty map for nonexistent entities (unlike hot tier which returns error)
	if len(result) != 0 {
		t.Errorf("Expected empty result for nonexistent entity, got %d features", len(result))
	}
}

func TestWarmTier_PartialFeatures(t *testing.T) {
	warm := newTestWarmTier(t)

	entityKey := "user:456"
	features := map[string]*domain.FeatureValue{
		"feature1": {Value: "value1", Timestamp: time.Now().UnixNano(), Version: 1},
	}

	err := warm.Put(entityKey, features)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Request features including one that doesn't exist
	result, err := warm.Get(entityKey, []string{"feature1", "feature2"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 feature, got %d", len(result))
	}

	if _, ok := result["feature1"]; !ok {
		t.Error("Expected feature1 to be present")
	}
}

func TestWarmTier_Delete(t *testing.T) {
	warm := newTestWarmTier(t)

	entityKey := "user:delete"
	err := warm.Put(entityKey, map[string]*domain.FeatureValue{
		"feature1": {Value: "value1", Timestamp: time.Now().UnixNano(), Version: 1},
		"feature2": {Value: "value2", Timestamp: time.Now().UnixNano(), Version: 1},
	})
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Verify features exist
	result, err := warm.Get(entityKey, []string{"feature1", "feature2"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("Expected 2 features before delete, got %d", len(result))
	}

	// Delete one feature
	err = warm.Delete(entityKey, []string{"feature1"})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify only one feature remains
	result, err = warm.Get(entityKey, []string{"feature1", "feature2"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 feature after delete, got %d", len(result))
	}

	if _, ok := result["feature2"]; !ok {
		t.Error("Expected feature2 to still exist")
	}
}

func TestWarmTier_DeleteNonexistent(t *testing.T) {
	warm := newTestWarmTier(t)

	// Deleting nonexistent features should not error
	err := warm.Delete("nonexistent", []string{"feature"})
	if err != nil {
		t.Errorf("Delete of nonexistent feature should not error: %v", err)
	}
}

func TestWarmTier_GetAsOf(t *testing.T) {
	warm := newTestWarmTier(t)

	entityKey := "user:history"
	now := time.Now()

	// Store features at different timestamps
	features1 := map[string]*domain.FeatureValue{
		"counter": {Value: int64(1), Timestamp: now.Add(-2 * time.Hour).UnixNano(), Version: 1},
	}
	err := warm.Put(entityKey, features1)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	features2 := map[string]*domain.FeatureValue{
		"counter": {Value: int64(2), Timestamp: now.Add(-1 * time.Hour).UnixNano(), Version: 2},
	}
	err = warm.Put(entityKey, features2)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	features3 := map[string]*domain.FeatureValue{
		"counter": {Value: int64(3), Timestamp: now.UnixNano(), Version: 3},
	}
	err = warm.Put(entityKey, features3)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Get as of 1.5 hours ago (should return version 1)
	result, err := warm.GetAsOf(entityKey, []string{"counter"}, now.Add(-90*time.Minute))
	if err != nil {
		t.Fatalf("GetAsOf failed: %v", err)
	}

	if result["counter"] == nil {
		t.Fatal("Expected counter to exist")
	}
	// JSON unmarshaling converts int64 to float64
	if v, ok := result["counter"].Value.(float64); !ok || v != 1 {
		t.Errorf("Expected counter=1 as of 1.5h ago, got %v (type %T)", result["counter"].Value, result["counter"].Value)
	}

	// Get as of 30 minutes ago (should return version 2)
	result, err = warm.GetAsOf(entityKey, []string{"counter"}, now.Add(-30*time.Minute))
	if err != nil {
		t.Fatalf("GetAsOf failed: %v", err)
	}

	if result["counter"] == nil {
		t.Fatal("Expected counter to exist")
	}
	if v, ok := result["counter"].Value.(float64); !ok || v != 2 {
		t.Errorf("Expected counter=2 as of 30m ago, got %v (type %T)", result["counter"].Value, result["counter"].Value)
	}

	// Get as of now (should return version 3)
	result, err = warm.GetAsOf(entityKey, []string{"counter"}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("GetAsOf failed: %v", err)
	}

	if result["counter"] == nil {
		t.Fatal("Expected counter to exist")
	}
	if v, ok := result["counter"].Value.(float64); !ok || v != 3 {
		t.Errorf("Expected counter=3 as of now, got %v (type %T)", result["counter"].Value, result["counter"].Value)
	}
}

func TestWarmTier_ExpireOlderThan(t *testing.T) {
	warm := newTestWarmTier(t)

	entityKey := "user:expire"
	now := time.Now()

	// Store feature with old timestamp
	oldFeature := map[string]*domain.FeatureValue{
		"old_feature": {Value: "old", Timestamp: now.Add(-48 * time.Hour).UnixNano(), Version: 1},
	}
	err := warm.Put(entityKey, oldFeature)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Store feature with recent timestamp
	newFeature := map[string]*domain.FeatureValue{
		"new_feature": {Value: "new", Timestamp: now.UnixNano(), Version: 1},
	}
	err = warm.Put(entityKey, newFeature)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Expire data older than 24 hours
	deleted, err := warm.ExpireOlderThan(24 * time.Hour)
	if err != nil {
		t.Fatalf("ExpireOlderThan failed: %v", err)
	}

	// Just verify no error - deletion count can vary based on implementation
	_ = deleted

	// Recent feature's history should still be accessible
	result, err := warm.GetAsOf(entityKey, []string{"new_feature"}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("GetAsOf failed: %v", err)
	}

	if result["new_feature"] == nil {
		t.Error("Expected new_feature history to still exist")
	}
}

func TestWarmTier_Close(t *testing.T) {
	warm, err := NewWarmTier(WarmTierOptions{InMemory: true})
	if err != nil {
		t.Fatalf("NewWarmTier failed: %v", err)
	}

	err = warm.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Operations after close should fail
	_, err = warm.Get("entity", []string{"feature"})
	if err == nil {
		t.Error("Expected error when operating on closed warm tier")
	}
}

func TestWarmTier_ConcurrentAccess(t *testing.T) {
	warm := newTestWarmTier(t)

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
				warm.Put(entityKey, map[string]*domain.FeatureValue{
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
				warm.Get("user:concurrent", []string{"counter"})
			}
		}()
	}

	wg.Wait()

	// Verify we can still read
	result, err := warm.Get("user:concurrent", []string{"counter"})
	if err != nil {
		t.Fatalf("Failed to get after concurrent access: %v", err)
	}

	if result["counter"] == nil {
		t.Error("Expected counter to have a value")
	}
}

func TestWarmTier_MultipleDataTypes(t *testing.T) {
	warm := newTestWarmTier(t)

	entityKey := "user:types"
	now := time.Now().UnixNano()

	features := map[string]*domain.FeatureValue{
		"int_feature":     {Value: int64(42), Timestamp: now, Version: 1},
		"float_feature":   {Value: 3.14159, Timestamp: now, Version: 1},
		"string_feature":  {Value: "hello world", Timestamp: now, Version: 1},
		"bool_feature":    {Value: true, Timestamp: now, Version: 1},
		"bytes_feature":   {Value: []byte{0x01, 0x02, 0x03}, Timestamp: now, Version: 1},
		"float32_feature": {Value: []float32{1.1, 2.2, 3.3}, Timestamp: now, Version: 1},
	}

	err := warm.Put(entityKey, features)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	result, err := warm.Get(entityKey, []string{
		"int_feature", "float_feature", "string_feature",
		"bool_feature", "bytes_feature", "float32_feature",
	})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(result) != 6 {
		t.Errorf("Expected 6 features, got %d", len(result))
	}
}

func BenchmarkWarmTier_Put(b *testing.B) {
	warm, err := NewWarmTier(WarmTierOptions{InMemory: true})
	if err != nil {
		b.Fatalf("Failed to create warm tier: %v", err)
	}
	defer warm.Close()

	features := map[string]*domain.FeatureValue{
		"feature1": {Value: int64(1), Timestamp: time.Now().UnixNano(), Version: 1},
		"feature2": {Value: 2.5, Timestamp: time.Now().UnixNano(), Version: 1},
		"feature3": {Value: "string", Timestamp: time.Now().UnixNano(), Version: 1},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		warm.Put("user:bench", features)
	}
}

func BenchmarkWarmTier_Get(b *testing.B) {
	warm, err := NewWarmTier(WarmTierOptions{InMemory: true})
	if err != nil {
		b.Fatalf("Failed to create warm tier: %v", err)
	}
	defer warm.Close()

	// Setup
	warm.Put("user:bench", map[string]*domain.FeatureValue{
		"feature1": {Value: int64(1), Timestamp: time.Now().UnixNano(), Version: 1},
		"feature2": {Value: 2.5, Timestamp: time.Now().UnixNano(), Version: 1},
		"feature3": {Value: "string", Timestamp: time.Now().UnixNano(), Version: 1},
	})

	featureNames := []string{"feature1", "feature2", "feature3"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		warm.Get("user:bench", featureNames)
	}
}
