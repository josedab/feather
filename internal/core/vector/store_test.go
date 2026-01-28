package vector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_CreateAndGetIndex(t *testing.T) {
	store := NewStore(StoreConfig{})

	idx, err := store.CreateIndex("test-idx", 3, DistanceCosine)
	require.NoError(t, err)
	assert.Equal(t, "test-idx", idx.Name)
	assert.Equal(t, 3, idx.Dimension)
	assert.Equal(t, DistanceCosine, idx.DistanceType)

	got, err := store.GetIndex("test-idx")
	require.NoError(t, err)
	assert.Equal(t, idx, got)
}

func TestStore_CreateIndex_Duplicate(t *testing.T) {
	store := NewStore(StoreConfig{})

	_, err := store.CreateIndex("dup", 3, DistanceCosine)
	require.NoError(t, err)

	_, err = store.CreateIndex("dup", 3, DistanceCosine)
	assert.Error(t, err)
}

func TestStore_CreateIndex_DefaultDistance(t *testing.T) {
	store := NewStore(StoreConfig{})

	idx, err := store.CreateIndex("default-dist", 3, "")
	require.NoError(t, err)
	assert.Equal(t, DistanceCosine, idx.DistanceType)
}

func TestStore_GetIndex_NotFound(t *testing.T) {
	store := NewStore(StoreConfig{})

	_, err := store.GetIndex("nonexistent")
	assert.ErrorIs(t, err, ErrIndexNotFound)
}

func TestStore_DeleteIndex(t *testing.T) {
	store := NewStore(StoreConfig{})

	_, err := store.CreateIndex("to-delete", 3, DistanceCosine)
	require.NoError(t, err)

	err = store.DeleteIndex("to-delete")
	require.NoError(t, err)

	_, err = store.GetIndex("to-delete")
	assert.ErrorIs(t, err, ErrIndexNotFound)
}

func TestStore_DeleteIndex_NotFound(t *testing.T) {
	store := NewStore(StoreConfig{})
	err := store.DeleteIndex("nonexistent")
	assert.ErrorIs(t, err, ErrIndexNotFound)
}

func TestStore_ListIndexes(t *testing.T) {
	store := NewStore(StoreConfig{})

	names := store.ListIndexes()
	assert.Empty(t, names)

	store.CreateIndex("idx-a", 3, DistanceCosine)
	store.CreateIndex("idx-b", 3, DistanceEuclidean)

	names = store.ListIndexes()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "idx-a")
	assert.Contains(t, names, "idx-b")
}

func TestIndex_UpsertAndSearch(t *testing.T) {
	store := NewStore(StoreConfig{})
	idx, err := store.CreateIndex("search-test", 3, DistanceCosine)
	require.NoError(t, err)

	err = idx.Upsert("v1", []float32{1, 0, 0}, map[string]interface{}{"label": "x-axis"})
	require.NoError(t, err)

	err = idx.Upsert("v2", []float32{0, 1, 0}, map[string]interface{}{"label": "y-axis"})
	require.NoError(t, err)

	results, err := idx.Search([]float32{1, 0, 0}, 2, nil)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "v1", results[0].ID)
	assert.NotNil(t, results[0].Metadata)
}

func TestIndex_SearchWithOptions(t *testing.T) {
	store := NewStore(StoreConfig{})
	idx, _ := store.CreateIndex("opts-test", 3, DistanceCosine)

	idx.Upsert("v1", []float32{1, 0, 0}, map[string]interface{}{"k": "v"})

	results, err := idx.Search([]float32{1, 0, 0}, 1, &SearchOptions{
		Ef:              50,
		IncludeMetadata: true,
		IncludeVectors:  true,
	})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.NotNil(t, results[0].Vector)
	assert.NotNil(t, results[0].Metadata)
}

func TestIndex_SearchExcludeMetadata(t *testing.T) {
	store := NewStore(StoreConfig{})
	idx, _ := store.CreateIndex("no-meta", 3, DistanceCosine)

	idx.Upsert("v1", []float32{1, 0, 0}, map[string]interface{}{"k": "v"})

	results, err := idx.Search([]float32{1, 0, 0}, 1, &SearchOptions{
		IncludeMetadata: false,
		IncludeVectors:  false,
	})
	require.NoError(t, err)
	assert.Nil(t, results[0].Metadata)
	assert.Nil(t, results[0].Vector)
}

func TestIndex_UpsertBatch(t *testing.T) {
	store := NewStore(StoreConfig{})
	idx, _ := store.CreateIndex("batch", 3, DistanceCosine)

	records := []Record{
		{ID: "b1", Vector: []float32{1, 0, 0}, Metadata: map[string]interface{}{"i": 1}},
		{ID: "b2", Vector: []float32{0, 1, 0}, Metadata: map[string]interface{}{"i": 2}},
		{ID: "b3", Vector: []float32{0, 0, 1}, Metadata: map[string]interface{}{"i": 3}},
	}

	err := idx.UpsertBatch(records)
	require.NoError(t, err)
	assert.Equal(t, 3, idx.Size())
}

func TestIndex_UpsertBatch_DimensionMismatch(t *testing.T) {
	store := NewStore(StoreConfig{})
	idx, _ := store.CreateIndex("bad-batch", 3, DistanceCosine)

	records := []Record{
		{ID: "b1", Vector: []float32{1, 0, 0}},
		{ID: "b2", Vector: []float32{0, 1}}, // wrong dim
	}

	err := idx.UpsertBatch(records)
	assert.Error(t, err)
}

func TestIndex_GetAndDelete(t *testing.T) {
	store := NewStore(StoreConfig{})
	idx, _ := store.CreateIndex("get-del", 3, DistanceCosine)

	idx.Upsert("v1", []float32{1, 0, 0}, map[string]interface{}{"key": "val"})

	rec, err := idx.Get("v1")
	require.NoError(t, err)
	assert.Equal(t, "v1", rec.ID)
	assert.Equal(t, []float32{1, 0, 0}, rec.Vector)
	assert.Equal(t, "val", rec.Metadata["key"])

	err = idx.Delete("v1")
	require.NoError(t, err)

	_, err = idx.Get("v1")
	assert.ErrorIs(t, err, ErrVectorNotFound)
}

func TestIndex_Size(t *testing.T) {
	store := NewStore(StoreConfig{})
	idx, _ := store.CreateIndex("size-test", 2, DistanceCosine)

	assert.Equal(t, 0, idx.Size())

	idx.Upsert("v1", []float32{1, 0}, nil)
	idx.Upsert("v2", []float32{0, 1}, nil)
	assert.Equal(t, 2, idx.Size())
}

func TestStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	store := NewStore(StoreConfig{DataDir: dir})
	idx, _ := store.CreateIndex("persist", 3, DistanceCosine)
	idx.Upsert("v1", []float32{1, 0, 0}, map[string]interface{}{"label": "test"})

	err := store.Save("persist")
	require.NoError(t, err)

	// Verify files exist
	configPath := filepath.Join(dir, "vectors", "persist", "config.json")
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Load into a new store
	store2 := NewStore(StoreConfig{DataDir: dir})
	err = store2.Load("persist")
	require.NoError(t, err)

	idx2, err := store2.GetIndex("persist")
	require.NoError(t, err)
	assert.Equal(t, 1, idx2.Size())
}

func TestStore_Save_InMemory(t *testing.T) {
	store := NewStore(StoreConfig{})
	store.CreateIndex("mem", 3, DistanceCosine)

	// Save should be a no-op for in-memory
	err := store.Save("mem")
	require.NoError(t, err)
}

func TestStore_Save_NotFound(t *testing.T) {
	store := NewStore(StoreConfig{DataDir: t.TempDir()})
	err := store.Save("nonexistent")
	assert.ErrorIs(t, err, ErrIndexNotFound)
}

func TestValidateIndexName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "my-index", false},
		{"valid underscore", "my_index_v2", false},
		{"empty", "", true},
		{"path traversal", "../etc", true},
		{"slash", "a/b", true},
		{"backslash", "a\\b", true},
		{"special chars", "idx@foo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIndexName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStore_InvalidIndexName(t *testing.T) {
	store := NewStore(StoreConfig{})

	_, err := store.CreateIndex("../bad", 3, DistanceCosine)
	assert.Error(t, err)

	_, err = store.GetIndex("../bad")
	assert.Error(t, err)

	err = store.DeleteIndex("../bad")
	assert.Error(t, err)
}

func TestDotProductDistance_Values(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{0, 1}

	dist := dotProductDistance(a, b)
	assert.Equal(t, float32(0), dist) // orthogonal = 0 dot product = -0 distance
}

func TestCosineDistance_ZeroVector(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 0, 0}

	dist := cosineDistance(a, b)
	assert.Equal(t, float32(1), dist) // zero vector should return 1
}
