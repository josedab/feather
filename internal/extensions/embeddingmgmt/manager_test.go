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

func TestRegisterModelInvalidInput(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	// Empty ID
	err := m.RegisterModel(EmbeddingModel{ID: "", Dimensions: 3})
	if err == nil {
		t.Error("expected error for empty model ID")
	}

	// Zero dimensions
	err = m.RegisterModel(EmbeddingModel{ID: "m1", Dimensions: 0})
	if err == nil {
		t.Error("expected error for zero dimensions")
	}
}

func TestCreateCollectionUnknownModel(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_, err := m.CreateCollection("test", "nonexistent-model", nil)
	if err != ErrModelNotFound {
		t.Fatalf("expected ErrModelNotFound, got %v", err)
	}
}

func TestUpsertToNonexistentCollection(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	err := m.Upsert("nonexistent", Embedding{ID: "e1", Vector: []float64{1, 0, 0}})
	if err != ErrCollectionNotFound {
		t.Fatalf("expected ErrCollectionNotFound, got %v", err)
	}
}

func TestSearchEmptyCollection(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_ = m.RegisterModel(EmbeddingModel{ID: "m1", Dimensions: 3})
	_, _ = m.CreateCollection("empty", "m1", nil)

	results, err := m.Search("empty", []float64{1, 0, 0}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results in empty collection, got %d", len(results))
	}
}

func TestSearchNonexistentCollection(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_, err := m.Search("nope", []float64{1, 0, 0}, 5)
	if err != ErrCollectionNotFound {
		t.Fatalf("expected ErrCollectionNotFound, got %v", err)
	}
}

func TestListCollections(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_ = m.RegisterModel(EmbeddingModel{ID: "m1", Dimensions: 3})
	_, _ = m.CreateCollection("col-a", "m1", nil)
	_, _ = m.CreateCollection("col-b", "m1", nil)

	cols := m.ListCollections()
	if len(cols) != 2 {
		t.Errorf("expected 2 collections, got %d", len(cols))
	}
}

func TestDeleteNonexistentCollection(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	err := m.DeleteCollection("nonexistent")
	if err != ErrCollectionNotFound {
		t.Fatalf("expected ErrCollectionNotFound, got %v", err)
	}
}

func TestMaxCollectionsLimit(t *testing.T) {
	m := NewManager(ManagerConfig{MaxCollections: 2, MaxEmbeddings: 100, DefaultTopK: 10})
	_ = m.RegisterModel(EmbeddingModel{ID: "m1", Dimensions: 3})
	_, _ = m.CreateCollection("col-1", "m1", nil)
	_, _ = m.CreateCollection("col-2", "m1", nil)

	_, err := m.CreateCollection("col-3", "m1", nil)
	if err == nil {
		t.Error("expected error when exceeding max collections")
	}
}

func TestUpsertOverwrite(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	_ = m.RegisterModel(EmbeddingModel{ID: "m1", Dimensions: 3})
	_, _ = m.CreateCollection("test", "m1", nil)

	_ = m.Upsert("test", Embedding{ID: "e1", Vector: []float64{1, 0, 0}})
	_ = m.Upsert("test", Embedding{ID: "e1", Vector: []float64{0, 1, 0}})

	results, err := m.Search("test", []float64{0, 1, 0}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ID != "e1" {
		t.Error("expected overwritten embedding to be found")
	}
}
