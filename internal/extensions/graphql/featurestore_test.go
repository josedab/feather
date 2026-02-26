package graphql

import (
	"context"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

func newTestStoreAndRegistry(t *testing.T) (*storage.Store, *storage.Registry) {
	t.Helper()
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:       1024 * 1024 * 10,
		WarmInMemory:     true,
		TTLCheckInterval: time.Hour,
	}, nil)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	registry := storage.NewRegistry()
	return store, registry
}

func TestNewFeatureStoreSchema(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)

	fs, err := NewFeatureStoreSchema(store, registry)
	if err != nil {
		t.Fatalf("NewFeatureStoreSchema() error = %v", err)
	}
	if fs == nil {
		t.Fatal("expected non-nil schema")
	}
	if fs.store != store {
		t.Error("store not set")
	}
	if fs.registry != registry {
		t.Error("registry not set")
	}
}

func TestNewFeatureStoreSchema_NilRegistry(t *testing.T) {
	t.Parallel()
	store, _ := newTestStoreAndRegistry(t)

	fs, err := NewFeatureStoreSchema(store, nil)
	if err != nil {
		t.Fatalf("NewFeatureStoreSchema() error = %v", err)
	}
	if fs == nil {
		t.Fatal("expected non-nil schema")
	}
}

// --- resolveFeature tests ---

func TestResolveFeature_Existing(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	ctx := context.Background()
	store.Put(ctx, "user:1", map[string]*domain.FeatureValue{
		"clicks": {Value: int64(42), Timestamp: time.Now().UnixNano(), Version: 1},
	})

	result, err := fs.resolveFeature(ctx, nil, map[string]interface{}{
		"entity":  "user:1",
		"feature": "clicks",
	})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}
	if m["entity"] != "user:1" {
		t.Errorf("expected entity user:1, got %v", m["entity"])
	}
	if m["value"] != int64(42) {
		t.Errorf("expected value 42, got %v", m["value"])
	}
}

func TestResolveFeature_MissingEntity(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	_, err := fs.resolveFeature(context.Background(), nil, map[string]interface{}{
		"entity":  "user:999",
		"feature": "clicks",
	})
	if err == nil {
		t.Fatal("expected error for missing entity/feature")
	}
}

func TestResolveFeature_MissingFeatureName(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	_, err := fs.resolveFeature(context.Background(), nil, map[string]interface{}{
		"entity": "user:1",
	})
	if err == nil {
		t.Fatal("expected error for missing feature name")
	}
}

func TestResolveFeature_MissingEntityArg(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	_, err := fs.resolveFeature(context.Background(), nil, map[string]interface{}{
		"feature": "clicks",
	})
	if err == nil {
		t.Fatal("expected error for missing entity arg")
	}
}

// --- resolveFeatures tests ---

func TestResolveFeatures_Multiple(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	ctx := context.Background()
	store.Put(ctx, "user:1", map[string]*domain.FeatureValue{
		"clicks":    {Value: int64(10), Timestamp: time.Now().UnixNano(), Version: 1},
		"purchases": {Value: int64(5), Timestamp: time.Now().UnixNano(), Version: 1},
	})

	result, err := fs.resolveFeatures(ctx, nil, map[string]interface{}{
		"entity":   "user:1",
		"features": []interface{}{"clicks", "purchases"},
	})
	if err != nil {
		t.Fatal(err)
	}
	list, ok := result.([]interface{})
	if !ok {
		t.Fatal("expected list result")
	}
	if len(list) != 2 {
		t.Errorf("expected 2 features, got %d", len(list))
	}
}

func TestResolveFeatures_EmptyList(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	result, err := fs.resolveFeatures(context.Background(), nil, map[string]interface{}{
		"entity":   "user:1",
		"features": []interface{}{},
	})
	if err != nil {
		t.Fatal(err)
	}
	list := result.([]interface{})
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

func TestResolveFeatures_MissingEntityArg(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	_, err := fs.resolveFeatures(context.Background(), nil, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing entity")
	}
}

// --- resolveFeatureHistory tests ---

func TestResolveFeatureHistory_Valid(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	ctx := context.Background()
	store.Put(ctx, "user:1", map[string]*domain.FeatureValue{
		"clicks": {Value: int64(10), Timestamp: time.Now().UnixNano(), Version: 1},
	})
	time.Sleep(50 * time.Millisecond)

	result, err := fs.resolveFeatureHistory(ctx, nil, map[string]interface{}{
		"entity":    "user:1",
		"feature":   "clicks",
		"startTime": time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestResolveFeatureHistory_MissingEntity(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	_, err := fs.resolveFeatureHistory(context.Background(), nil, map[string]interface{}{
		"feature": "clicks",
	})
	if err == nil {
		t.Fatal("expected error for missing entity")
	}
}

func TestResolveFeatureHistory_MissingFeatureArg(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	_, err := fs.resolveFeatureHistory(context.Background(), nil, map[string]interface{}{
		"entity": "user:1",
	})
	if err == nil {
		t.Fatal("expected error for missing feature")
	}
}

// --- resolveFeatureGroups tests ---

func TestResolveFeatureGroups_Populated(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)

	registry.RegisterGroup(&domain.FeatureGroup{
		Name:        "user_features",
		Description: "User features",
		Features:    []domain.FeatureSpec{{Name: "clicks", DataType: domain.DataTypeInt64}},
	})

	fs, _ := NewFeatureStoreSchema(store, registry)

	result, err := fs.resolveFeatureGroups(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	list := result.([]interface{})
	if len(list) != 1 {
		t.Errorf("expected 1 group, got %d", len(list))
	}
	m := list[0].(map[string]interface{})
	if m["name"] != "user_features" {
		t.Errorf("expected name user_features, got %v", m["name"])
	}
}

func TestResolveFeatureGroups_NilRegistry(t *testing.T) {
	t.Parallel()
	store, _ := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, nil)

	result, err := fs.resolveFeatureGroups(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	list := result.([]interface{})
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

func TestResolveFeatureGroups_Empty(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	result, err := fs.resolveFeatureGroups(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	list := result.([]interface{})
	if len(list) != 0 {
		t.Errorf("expected 0 groups, got %d", len(list))
	}
}

// --- resolveAggregation tests ---

func TestResolveAggregation_WithFeature(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	ctx := context.Background()
	store.Put(ctx, "user:1", map[string]*domain.FeatureValue{
		"clicks": {Value: float64(42), Timestamp: time.Now().UnixNano(), Version: 1},
	})

	functions := []string{"count", "sum", "avg", "min", "max"}
	for _, fn := range functions {
		result, err := fs.resolveAggregation(ctx, nil, map[string]interface{}{
			"entity":   "user:1",
			"feature":  "clicks",
			"function": fn,
		})
		if err != nil {
			t.Errorf("resolveAggregation(%s) error = %v", fn, err)
			continue
		}
		m := result.(map[string]interface{})
		if m["function"] != fn {
			t.Errorf("expected function %s, got %v", fn, m["function"])
		}
		if m["window"] != "1h" {
			t.Errorf("expected default window 1h, got %v", m["window"])
		}
	}
}

func TestResolveAggregation_CustomWindow(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	ctx := context.Background()
	store.Put(ctx, "user:1", map[string]*domain.FeatureValue{
		"clicks": {Value: float64(1), Timestamp: time.Now().UnixNano(), Version: 1},
	})

	result, err := fs.resolveAggregation(ctx, nil, map[string]interface{}{
		"entity":   "user:1",
		"feature":  "clicks",
		"function": "sum",
		"window":   "24h",
	})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]interface{})
	if m["window"] != "24h" {
		t.Errorf("expected window 24h, got %v", m["window"])
	}
}

func TestResolveAggregation_MissingArgs(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	_, err := fs.resolveAggregation(context.Background(), nil, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

// --- resolveSetFeature tests ---

func TestResolveSetFeature_Valid(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	result, err := fs.resolveSetFeature(context.Background(), nil, map[string]interface{}{
		"entity":  "user:1",
		"feature": "clicks",
		"value":   int64(42),
	})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]interface{})
	if m["value"] != int64(42) {
		t.Errorf("expected value 42, got %v", m["value"])
	}

	// Verify stored
	got, _ := store.Get(context.Background(), "user:1", []string{"clicks"})
	if got["clicks"] == nil {
		t.Fatal("expected stored feature")
	}
}

func TestResolveSetFeature_MissingEntity(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	_, err := fs.resolveSetFeature(context.Background(), nil, map[string]interface{}{
		"feature": "clicks",
		"value":   int64(1),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveSetFeature_MissingFeature(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	_, err := fs.resolveSetFeature(context.Background(), nil, map[string]interface{}{
		"entity": "user:1",
		"value":  int64(1),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveSetFeature_WithTimestamp(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	result, err := fs.resolveSetFeature(context.Background(), nil, map[string]interface{}{
		"entity":    "user:1",
		"feature":   "clicks",
		"value":     int64(10),
		"timestamp": ts,
	})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]interface{})
	if m["timestamp"] != ts {
		t.Errorf("expected timestamp to be preserved")
	}
}

// --- resolveCreateFeatureGroup tests ---

func TestResolveCreateFeatureGroup_Valid(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	result, err := fs.resolveCreateFeatureGroup(context.Background(), nil, map[string]interface{}{
		"name":        "test_group",
		"description": "A test group",
	})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]interface{})
	if m["name"] != "test_group" {
		t.Errorf("expected name test_group, got %v", m["name"])
	}
}

func TestResolveCreateFeatureGroup_DuplicateName(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	args := map[string]interface{}{"name": "dup_group"}
	fs.resolveCreateFeatureGroup(context.Background(), nil, args)

	_, err := fs.resolveCreateFeatureGroup(context.Background(), nil, args)
	if err == nil {
		t.Fatal("expected error for duplicate group name")
	}
}

func TestResolveCreateFeatureGroup_NilRegistry(t *testing.T) {
	t.Parallel()
	store, _ := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, nil)

	_, err := fs.resolveCreateFeatureGroup(context.Background(), nil, map[string]interface{}{
		"name": "test",
	})
	if err == nil {
		t.Fatal("expected error for nil registry")
	}
}

func TestResolveCreateFeatureGroup_MissingName(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	_, err := fs.resolveCreateFeatureGroup(context.Background(), nil, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

// --- resolveHealthCheck tests ---

func TestResolveHealthCheck(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	result, err := fs.resolveHealthCheck(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]interface{})
	if m["status"] != "healthy" {
		t.Errorf("expected healthy, got %v", m["status"])
	}
}

// --- resolveDeleteFeature tests ---

func TestResolveDeleteFeature(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	ctx := context.Background()
	store.Put(ctx, "user:1", map[string]*domain.FeatureValue{
		"clicks": {Value: int64(1), Timestamp: time.Now().UnixNano(), Version: 1},
	})

	result, err := fs.resolveDeleteFeature(ctx, nil, map[string]interface{}{
		"entity":  "user:1",
		"feature": "clicks",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != true {
		t.Errorf("expected true, got %v", result)
	}
}

func TestResolveDeleteFeature_MissingArgs(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	_, err := fs.resolveDeleteFeature(context.Background(), nil, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

// --- resolveSetFeatures tests ---

func TestResolveSetFeatures(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	result, err := fs.resolveSetFeatures(context.Background(), nil, map[string]interface{}{
		"features": []interface{}{
			map[string]interface{}{
				"entity":  "user:1",
				"feature": "clicks",
				"value":   int64(10),
			},
			map[string]interface{}{
				"entity":  "user:1",
				"feature": "views",
				"value":   int64(100),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	list := result.([]interface{})
	if len(list) != 2 {
		t.Errorf("expected 2 results, got %d", len(list))
	}
}

func TestResolveSetFeatures_InvalidType(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	_, err := fs.resolveSetFeatures(context.Background(), nil, map[string]interface{}{
		"features": "not a list",
	})
	if err == nil {
		t.Fatal("expected error for invalid features type")
	}
}

// --- resolveFeatureGroup tests ---

func TestResolveFeatureGroup(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)

	registry.RegisterGroup(&domain.FeatureGroup{
		Name:        "user_features",
		Description: "User features",
		Features:    []domain.FeatureSpec{{Name: "clicks", DataType: domain.DataTypeInt64}},
	})

	fs, _ := NewFeatureStoreSchema(store, registry)

	result, err := fs.resolveFeatureGroup(context.Background(), nil, map[string]interface{}{
		"name": "user_features",
	})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]interface{})
	if m["name"] != "user_features" {
		t.Errorf("expected user_features, got %v", m["name"])
	}
}

func TestResolveFeatureGroup_NotFound(t *testing.T) {
	t.Parallel()
	store, registry := newTestStoreAndRegistry(t)
	fs, _ := NewFeatureStoreSchema(store, registry)

	_, err := fs.resolveFeatureGroup(context.Background(), nil, map[string]interface{}{
		"name": "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent group")
	}
}
