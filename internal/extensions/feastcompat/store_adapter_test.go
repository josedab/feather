package feastcompat

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

// mockStore implements FeatureStoreBackend for testing.
type mockStore struct {
	mu       sync.RWMutex
	features map[string]map[string]*domain.FeatureValue
}

func newMockStore() *mockStore {
	return &mockStore{
		features: make(map[string]map[string]*domain.FeatureValue),
	}
}

func (m *mockStore) Get(_ context.Context, entityKey string, features []string) (map[string]*domain.FeatureValue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stored, ok := m.features[entityKey]
	if !ok {
		return map[string]*domain.FeatureValue{}, nil
	}
	result := make(map[string]*domain.FeatureValue, len(features))
	for _, f := range features {
		if v, exists := stored[f]; exists {
			result[f] = v
		}
	}
	return result, nil
}

func (m *mockStore) GetAsOf(_ context.Context, entityKey string, features []string, _ time.Time) (map[string]*domain.FeatureValue, error) {
	return m.Get(context.Background(), entityKey, features)
}

func (m *mockStore) Put(_ context.Context, entityKey string, features map[string]*domain.FeatureValue) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.features[entityKey]; !ok {
		m.features[entityKey] = make(map[string]*domain.FeatureValue)
	}
	for k, v := range features {
		m.features[entityKey][k] = v
	}
	return nil
}

func TestStoreLookupAdapter_LookupFunc(t *testing.T) {
	store := newMockStore()
	store.features["user:1"] = map[string]*domain.FeatureValue{
		"click_count": {Value: 42, Timestamp: time.Now().UnixNano(), Version: 1},
		"age":         {Value: 25, Timestamp: time.Now().UnixNano(), Version: 1},
	}

	adapter := NewStoreLookupAdapter(store)
	lookup := adapter.LookupFunc()

	vals, err := lookup("user:1", []string{"click_count", "age"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vals["click_count"] != 42 {
		t.Errorf("expected click_count=42, got %v", vals["click_count"])
	}
	if vals["age"] != 25 {
		t.Errorf("expected age=25, got %v", vals["age"])
	}
}

func TestStoreLookupAdapter_LookupFunc_MissingEntity(t *testing.T) {
	store := newMockStore()
	adapter := NewStoreLookupAdapter(store)
	lookup := adapter.LookupFunc()

	vals, err := lookup("nonexistent", []string{"feature"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vals) != 0 {
		t.Errorf("expected empty result for missing entity, got %v", vals)
	}
}

func TestStoreLookupAdapter_PushToStore(t *testing.T) {
	store := newMockStore()
	adapter := NewStoreLookupAdapter(store)

	rows := []map[string]interface{}{
		{"entity_id": "user:1", "click_count": 10, "age": 30},
		{"entity_id": "user:2", "click_count": 20},
	}

	ingested, err := adapter.PushToStore("test_source", rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ingested != 2 {
		t.Errorf("expected 2 ingested, got %d", ingested)
	}

	// Verify features stored
	stored := store.features["user:1"]
	if stored == nil {
		t.Fatal("expected features for user:1")
	}
	if stored["click_count"].Value != 10 {
		t.Errorf("expected click_count=10, got %v", stored["click_count"].Value)
	}
	if stored["age"].Value != 30 {
		t.Errorf("expected age=30, got %v", stored["age"].Value)
	}

	stored2 := store.features["user:2"]
	if stored2 == nil {
		t.Fatal("expected features for user:2")
	}
	if stored2["click_count"].Value != 20 {
		t.Errorf("expected click_count=20, got %v", stored2["click_count"].Value)
	}
}

func TestStoreLookupAdapter_PushToStore_NoEntityID(t *testing.T) {
	store := newMockStore()
	adapter := NewStoreLookupAdapter(store)

	rows := []map[string]interface{}{
		{"click_count": 10},
	}

	ingested, err := adapter.PushToStore("src", rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ingested != 1 {
		t.Errorf("expected 1 ingested, got %d", ingested)
	}

	// Should use generated entity key
	key := fmt.Sprintf("src_%d", 0)
	if store.features[key] == nil {
		t.Errorf("expected features for generated key %q", key)
	}
}

func TestStoreLookupAdapter_MaterializeFromStore(t *testing.T) {
	store := newMockStore()
	store.features["user:1"] = map[string]*domain.FeatureValue{
		"click_count": {Value: 42, Timestamp: time.Now().UnixNano(), Version: 1},
	}

	adapter := NewStoreLookupAdapter(store)

	rows, err := adapter.MaterializeFromStore(
		[]string{"user:1"},
		[]string{"click_count"},
		time.Now(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows != 1 {
		t.Errorf("expected 1 row written, got %d", rows)
	}
}

func TestGateway_Push_WithStoreAdapter(t *testing.T) {
	store := newMockStore()
	adapter := NewAdapter(DefaultAdapterConfig())
	gw := NewGateway(adapter)

	storeAdapter := NewStoreLookupAdapter(store)
	gw.SetStoreAdapter(storeAdapter)

	resp, err := gw.Push(PushRequest{
		PushSourceName: "test",
		DfData: []map[string]interface{}{
			{"entity_id": "user:1", "click_count": 99},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success")
	}
	if resp.RowsIngested != 1 {
		t.Errorf("expected 1 row ingested, got %d", resp.RowsIngested)
	}

	// Verify stored in real store
	stored := store.features["user:1"]
	if stored == nil {
		t.Fatal("expected features for user:1 in store")
	}
	if stored["click_count"].Value != 99 {
		t.Errorf("expected click_count=99, got %v", stored["click_count"].Value)
	}
}

func TestGateway_Push_WithoutStoreAdapter(t *testing.T) {
	adapter := NewAdapter(DefaultAdapterConfig())
	gw := NewGateway(adapter)

	resp, err := gw.Push(PushRequest{
		PushSourceName: "test",
		DfData: []map[string]interface{}{
			{"entity_id": "user:1", "click_count": 99},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success")
	}
	if resp.RowsIngested != 1 {
		t.Errorf("expected 1 row ingested, got %d", resp.RowsIngested)
	}
}

func TestGAGateway_SetStoreAdapter(t *testing.T) {
	store := newMockStore()
	store.features["user123"] = map[string]*domain.FeatureValue{
		"click_count": {Value: 77, Timestamp: time.Now().UnixNano(), Version: 1},
	}

	gaGw := NewGAGateway(DefaultGAConfig())
	storeAdapter := NewStoreLookupAdapter(store)
	gaGw.SetStoreAdapter(storeAdapter)

	result, err := gaGw.GetOnlineFeatures(
		[]map[string]interface{}{{"user_id": "user123"}},
		[]string{"click_count"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results, ok := result["results"].([]map[string]interface{})
	if !ok || len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0]["click_count"] != 77 {
		t.Errorf("expected click_count=77 from store, got %v", results[0]["click_count"])
	}
}
