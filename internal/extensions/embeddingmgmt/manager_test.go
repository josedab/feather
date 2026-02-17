package embeddingmgmt

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestModelRegistration(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	err := m.RegisterModel(EmbeddingModel{
		ID:         "openai-ada",
		Name:       "text-embedding-ada-002",
		Provider:   "openai",
		Dimensions: 1536,
		Version:    "2",
	})
	if err != nil {
		t.Fatal(err)
	}

	models := m.ListModels()
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
}

func TestCollectionLifecycle(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_ = m.RegisterModel(EmbeddingModel{ID: "m1", Dimensions: 3})

	col, err := m.CreateCollection("docs", "m1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if col.Dimensions != 3 {
		t.Errorf("expected 3 dimensions, got %d", col.Dimensions)
	}

	// Duplicate
	_, err = m.CreateCollection("docs", "m1", nil)
	if err != ErrCollectionExists {
		t.Fatalf("expected ErrCollectionExists, got %v", err)
	}

	// Delete
	if err := m.DeleteCollection("docs"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetCollection("docs"); err != ErrCollectionNotFound {
		t.Fatalf("expected ErrCollectionNotFound, got %v", err)
	}
}

func TestUpsertAndSearch(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_ = m.RegisterModel(EmbeddingModel{ID: "m1", Dimensions: 3})
	_, _ = m.CreateCollection("test", "m1", nil)

	_ = m.Upsert("test", Embedding{ID: "e1", Vector: []float64{1, 0, 0}})
	_ = m.Upsert("test", Embedding{ID: "e2", Vector: []float64{0, 1, 0}})
	_ = m.Upsert("test", Embedding{ID: "e3", Vector: []float64{1, 1, 0}})

	results, err := m.Search("test", []float64{1, 0, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "e1" {
		t.Errorf("expected e1 as top result, got %s", results[0].ID)
	}
}

func TestDimensionMismatch(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_ = m.RegisterModel(EmbeddingModel{ID: "m1", Dimensions: 3})
	_, _ = m.CreateCollection("test", "m1", nil)

	err := m.Upsert("test", Embedding{ID: "e1", Vector: []float64{1, 0}})
	if err == nil {
		t.Fatal("expected dimension mismatch error")
	}
}
