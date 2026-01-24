package storage

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

func TestHotTier_PutAndGet(t *testing.T) {
	hot := NewHotTier(1024 * 1024 * 100) // 100MB

	// Put some features
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

	err := hot.Put(entityKey, features)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Get features back
	result, err := hot.Get(entityKey, []string{"click_count", "purchase_total"})
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

func TestHotTier_EntityNotFound(t *testing.T) {
	hot := NewHotTier(1024 * 1024)

	_, err := hot.Get("nonexistent", []string{"feature"})
	if !errors.Is(err, domain.ErrEntityNotFound) {
		t.Errorf("Expected ErrEntityNotFound, got %v", err)
	}
}

func TestHotTier_PartialFeatures(t *testing.T) {
	hot := NewHotTier(1024 * 1024)

	entityKey := "user:456"
	features := map[string]*domain.FeatureValue{
		"feature1": {Value: "value1", Timestamp: time.Now().UnixNano(), Version: 1},
	}

	hot.Put(entityKey, features)

	// Request features including one that doesn't exist
	result, err := hot.Get(entityKey, []string{"feature1", "feature2"})
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

func TestHotTier_VersionConflict(t *testing.T) {
	hot := NewHotTier(1024 * 1024)

	entityKey := "user:789"

	// Put version 2
	hot.Put(entityKey, map[string]*domain.FeatureValue{
		"feature": {Value: "version2", Timestamp: time.Now().UnixNano(), Version: 2},
	})

	// Try to put version 1 (should be ignored)
	hot.Put(entityKey, map[string]*domain.FeatureValue{
		"feature": {Value: "version1", Timestamp: time.Now().UnixNano(), Version: 1},
	})

	result, _ := hot.Get(entityKey, []string{"feature"})
	if result["feature"].Value != "version2" {
		t.Errorf("Expected version2, got %v", result["feature"].Value)
	}

	// Put version 3 (should succeed)
	hot.Put(entityKey, map[string]*domain.FeatureValue{
		"feature": {Value: "version3", Timestamp: time.Now().UnixNano(), Version: 3},
	})

	result, _ = hot.Get(entityKey, []string{"feature"})
	if result["feature"].Value != "version3" {
		t.Errorf("Expected version3, got %v", result["feature"].Value)
	}
}

func TestHotTier_Delete(t *testing.T) {
	hot := NewHotTier(1024 * 1024)

	entityKey := "user:delete"
	hot.Put(entityKey, map[string]*domain.FeatureValue{
		"feature": {Value: "value", Timestamp: time.Now().UnixNano(), Version: 1},
	})

	// Verify it exists
	_, err := hot.Get(entityKey, []string{"feature"})
	if err != nil {
		t.Fatalf("Expected feature to exist")
	}

	// Delete
	hot.Delete(entityKey)

	// Verify it's gone
	_, err = hot.Get(entityKey, []string{"feature"})
	if !errors.Is(err, domain.ErrEntityNotFound) {
		t.Errorf("Expected ErrEntityNotFound after delete, got %v", err)
	}
}

func TestHotTier_ConcurrentAccess(t *testing.T) {
	hot := NewHotTier(1024 * 1024 * 100)

	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				entityKey := "user:concurrent"
				hot.Put(entityKey, map[string]*domain.FeatureValue{
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
				hot.Get("user:concurrent", []string{"counter"})
			}
		}()
	}

	wg.Wait()

	// Verify we can still read
	result, err := hot.Get("user:concurrent", []string{"counter"})
	if err != nil {
		t.Fatalf("Failed to get after concurrent access: %v", err)
	}

	if result["counter"] == nil {
		t.Error("Expected counter to have a value")
	}
}

func TestHotTier_Metrics(t *testing.T) {
	hot := NewHotTier(1024 * 1024)

	// Generate some hits and misses
	hot.Put("user:1", map[string]*domain.FeatureValue{
		"feature": {Value: "value", Timestamp: time.Now().UnixNano(), Version: 1},
	})

	// Hit
	hot.Get("user:1", []string{"feature"})

	// Miss (feature doesn't exist)
	hot.Get("user:1", []string{"nonexistent"})

	// Miss (entity doesn't exist)
	hot.Get("user:nonexistent", []string{"feature"})

	metrics := hot.Metrics()
	if metrics.TotalReads < 3 {
		t.Errorf("Expected at least 3 total reads, got %d", metrics.TotalReads)
	}
}

func TestHotTier_GetAll(t *testing.T) {
	hot := NewHotTier(1024 * 1024)

	entityKey := "user:all"
	hot.Put(entityKey, map[string]*domain.FeatureValue{
		"f1": {Value: "v1", Timestamp: time.Now().UnixNano(), Version: 1},
		"f2": {Value: "v2", Timestamp: time.Now().UnixNano(), Version: 1},
		"f3": {Value: "v3", Timestamp: time.Now().UnixNano(), Version: 1},
	})

	result, err := hot.GetAll(entityKey)
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("Expected 3 features, got %d", len(result))
	}
}

func TestHotTier_GetAll_NotFound(t *testing.T) {
	hot := NewHotTier(1024 * 1024)

	_, err := hot.GetAll("nonexistent")
	if !errors.Is(err, domain.ErrEntityNotFound) {
		t.Errorf("Expected ErrEntityNotFound, got %v", err)
	}
}

func TestHotTier_Size(t *testing.T) {
	hot := NewHotTier(1024 * 1024)

	if hot.Size() != 0 {
		t.Errorf("Expected initial size 0, got %d", hot.Size())
	}

	hot.Put("user:1", map[string]*domain.FeatureValue{
		"f1": {Value: "v1", Timestamp: time.Now().UnixNano(), Version: 1},
	})

	if hot.Size() <= 0 {
		t.Error("Expected positive size after put")
	}
}

func TestHotTier_EntityCount(t *testing.T) {
	hot := NewHotTier(1024 * 1024)

	if hot.EntityCount() != 0 {
		t.Error("Expected initial entity count 0")
	}

	hot.Put("user:1", map[string]*domain.FeatureValue{
		"f1": {Value: "v1", Timestamp: time.Now().UnixNano(), Version: 1},
	})
	hot.Put("user:2", map[string]*domain.FeatureValue{
		"f1": {Value: "v1", Timestamp: time.Now().UnixNano(), Version: 1},
	})

	if hot.EntityCount() != 2 {
		t.Errorf("Expected 2 entities, got %d", hot.EntityCount())
	}
}

func TestHotTier_ExpireOlderThan(t *testing.T) {
	hot := NewHotTier(1024 * 1024)

	// Put an old feature
	oldTs := time.Now().Add(-2 * time.Hour).UnixNano()
	hot.Put("user:old", map[string]*domain.FeatureValue{
		"f1": {Value: "old", Timestamp: oldTs, Version: 1},
	})

	// Put a recent feature
	hot.Put("user:new", map[string]*domain.FeatureValue{
		"f1": {Value: "new", Timestamp: time.Now().UnixNano(), Version: 1},
	})

	expired := hot.ExpireOlderThan(time.Hour)
	if expired < 1 {
		t.Errorf("Expected at least 1 expired feature, got %d", expired)
	}

	// Old entity should be gone (only had expired features)
	_, err := hot.Get("user:old", []string{"f1"})
	if !errors.Is(err, domain.ErrEntityNotFound) {
		t.Error("Expected old entity to be removed")
	}

	// New entity should still exist
	result, err := hot.Get("user:new", []string{"f1"})
	if err != nil {
		t.Fatalf("New entity should still exist: %v", err)
	}
	if result["f1"].Value != "new" {
		t.Error("New entity feature value mismatch")
	}
}

func TestHotTier_LRUEviction(t *testing.T) {
	// Very small max size to trigger eviction
	hot := NewHotTier(200)

	// Put multiple entities to exceed max size
	for i := 0; i < 10; i++ {
		entityKey := "user:" + string(rune('A'+i))
		hot.Put(entityKey, map[string]*domain.FeatureValue{
			"f1": {Value: "value", Timestamp: time.Now().UnixNano(), Version: 1},
		})
	}

	// Wait for async eviction
	time.Sleep(100 * time.Millisecond)

	// Some entities should have been evicted
	metrics := hot.Metrics()
	if metrics.Evictions == 0 {
		t.Error("Expected some evictions with small max size")
	}
}

func TestHotTier_Close(t *testing.T) {
	hot := NewHotTier(1024 * 1024)
	hot.Put("user:1", map[string]*domain.FeatureValue{
		"f1": {Value: "v1", Timestamp: time.Now().UnixNano(), Version: 1},
	})

	// Close should not panic and should be idempotent
	hot.Close()
	hot.Close() // second close via stopOnce
}

func TestHotTier_DeleteNonExistent(t *testing.T) {
	hot := NewHotTier(1024 * 1024)

	// Deleting non-existent entity should not error
	err := hot.Delete("nonexistent")
	if err != nil {
		t.Errorf("Delete of nonexistent entity should not error: %v", err)
	}
}

func BenchmarkHotTier_Put(b *testing.B) {
	hot := NewHotTier(1024 * 1024 * 100)

	features := map[string]*domain.FeatureValue{
		"feature1": {Value: int64(1), Timestamp: time.Now().UnixNano(), Version: 1},
		"feature2": {Value: 2.5, Timestamp: time.Now().UnixNano(), Version: 1},
		"feature3": {Value: "string", Timestamp: time.Now().UnixNano(), Version: 1},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hot.Put("user:bench", features)
	}
}

func BenchmarkHotTier_Get(b *testing.B) {
	hot := NewHotTier(1024 * 1024 * 100)

	// Setup
	hot.Put("user:bench", map[string]*domain.FeatureValue{
		"feature1": {Value: int64(1), Timestamp: time.Now().UnixNano(), Version: 1},
		"feature2": {Value: 2.5, Timestamp: time.Now().UnixNano(), Version: 1},
		"feature3": {Value: "string", Timestamp: time.Now().UnixNano(), Version: 1},
	})

	featureNames := []string{"feature1", "feature2", "feature3"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hot.Get("user:bench", featureNames)
	}
}
