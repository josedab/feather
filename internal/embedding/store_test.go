package embedding

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultStoreConfig(t *testing.T) {
	config := DefaultStoreConfig()

	assert.Equal(t, 100000, config.MaxCapacity)
	assert.Equal(t, int64(4*1024*1024*1024), config.MaxMemoryBytes)
	assert.Equal(t, 24*time.Hour, config.DefaultTTL)
	assert.Equal(t, "lru", config.EvictionPolicy)
	assert.Equal(t, 5*time.Minute, config.CleanupInterval)
}

func TestNewStore(t *testing.T) {
	config := DefaultStoreConfig()
	config.CleanupInterval = 0 // Disable cleanup for tests

	store := NewStore(config)
	require.NotNil(t, store)

	defer store.Close()

	assert.Equal(t, int64(0), store.Count())
}

func TestStore_Put_Get(t *testing.T) {
	config := DefaultStoreConfig()
	config.CleanupInterval = 0
	store := NewStore(config)
	defer store.Close()

	ctx := context.Background()

	emb := &Embedding{
		ID:          "emb-1",
		ContentHash: "hash-1",
		Vector:      []float32{0.1, 0.2, 0.3},
		ModelID:     "model-1",
	}

	err := store.Put(ctx, emb)
	require.NoError(t, err)

	assert.Equal(t, int64(1), store.Count())

	// Retrieve by ID
	retrieved, err := store.Get(ctx, "emb-1")
	require.NoError(t, err)
	assert.Equal(t, emb.ID, retrieved.ID)
	assert.Equal(t, emb.Vector, retrieved.Vector)
	assert.Equal(t, 3, retrieved.Dimension)
}

func TestStore_Put_InvalidEmbedding(t *testing.T) {
	config := DefaultStoreConfig()
	config.CleanupInterval = 0
	store := NewStore(config)
	defer store.Close()

	ctx := context.Background()

	// Nil embedding
	err := store.Put(ctx, nil)
	assert.ErrorIs(t, err, ErrInvalidEmbedding)

	// Empty vector
	err = store.Put(ctx, &Embedding{ID: "empty", Vector: []float32{}})
	assert.ErrorIs(t, err, ErrInvalidEmbedding)
}

func TestStore_Get_NotFound(t *testing.T) {
	config := DefaultStoreConfig()
	config.CleanupInterval = 0
	store := NewStore(config)
	defer store.Close()

	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrEmbeddingNotFound)
}

func TestStore_GetByHash(t *testing.T) {
	config := DefaultStoreConfig()
	config.CleanupInterval = 0
	store := NewStore(config)
	defer store.Close()

	ctx := context.Background()

	emb := &Embedding{
		ID:          "emb-1",
		ContentHash: "unique-hash",
		Vector:      []float32{0.1, 0.2, 0.3},
		ModelID:     "model-1",
	}

	err := store.Put(ctx, emb)
	require.NoError(t, err)

	// Retrieve by hash
	retrieved, err := store.GetByHash(ctx, "unique-hash")
	require.NoError(t, err)
	assert.Equal(t, "emb-1", retrieved.ID)
}

func TestStore_GetByHash_NotFound(t *testing.T) {
	config := DefaultStoreConfig()
	config.CleanupInterval = 0
	store := NewStore(config)
	defer store.Close()

	ctx := context.Background()

	_, err := store.GetByHash(ctx, "nonexistent-hash")
	assert.ErrorIs(t, err, ErrEmbeddingNotFound)
}

func TestStore_GetByModel(t *testing.T) {
	config := DefaultStoreConfig()
	config.CleanupInterval = 0
	store := NewStore(config)
	defer store.Close()

	ctx := context.Background()

	// Add embeddings for two models
	_ = store.Put(ctx, &Embedding{ID: "emb-1", Vector: []float32{0.1}, ModelID: "model-a"})
	_ = store.Put(ctx, &Embedding{ID: "emb-2", Vector: []float32{0.2}, ModelID: "model-a"})
	_ = store.Put(ctx, &Embedding{ID: "emb-3", Vector: []float32{0.3}, ModelID: "model-b"})

	// Get by model
	embeddings, err := store.GetByModel(ctx, "model-a")
	require.NoError(t, err)
	assert.Len(t, embeddings, 2)

	embeddings, err = store.GetByModel(ctx, "model-b")
	require.NoError(t, err)
	assert.Len(t, embeddings, 1)
}

func TestStore_Delete(t *testing.T) {
	config := DefaultStoreConfig()
	config.CleanupInterval = 0
	store := NewStore(config)
	defer store.Close()

	ctx := context.Background()

	emb := &Embedding{
		ID:          "emb-1",
		ContentHash: "hash-1",
		Vector:      []float32{0.1, 0.2, 0.3},
		ModelID:     "model-1",
	}

	_ = store.Put(ctx, emb)
	assert.Equal(t, int64(1), store.Count())

	err := store.Delete(ctx, "emb-1")
	require.NoError(t, err)
	assert.Equal(t, int64(0), store.Count())

	// Hash index should be cleared
	_, err = store.GetByHash(ctx, "hash-1")
	assert.ErrorIs(t, err, ErrEmbeddingNotFound)
}

func TestStore_Delete_NotFound(t *testing.T) {
	config := DefaultStoreConfig()
	config.CleanupInterval = 0
	store := NewStore(config)
	defer store.Close()

	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrEmbeddingNotFound)
}

func TestStore_Expiration(t *testing.T) {
	config := DefaultStoreConfig()
	config.CleanupInterval = 0
	config.DefaultTTL = 0 // Disable default TTL
	store := NewStore(config)
	defer store.Close()

	ctx := context.Background()

	// Add expired embedding
	emb := &Embedding{
		ID:        "emb-1",
		Vector:    []float32{0.1},
		ModelID:   "model-1",
		ExpiresAt: time.Now().Add(-time.Hour), // Already expired
	}
	_ = store.Put(ctx, emb)

	// Should not be retrievable
	_, err := store.Get(ctx, "emb-1")
	assert.ErrorIs(t, err, ErrEmbeddingNotFound)
}

func TestStore_Clear(t *testing.T) {
	config := DefaultStoreConfig()
	config.CleanupInterval = 0
	store := NewStore(config)
	defer store.Close()

	ctx := context.Background()

	_ = store.Put(ctx, &Embedding{ID: "emb-1", Vector: []float32{0.1}, ModelID: "model"})
	_ = store.Put(ctx, &Embedding{ID: "emb-2", Vector: []float32{0.2}, ModelID: "model"})

	assert.Equal(t, int64(2), store.Count())

	err := store.Clear(ctx)
	require.NoError(t, err)

	assert.Equal(t, int64(0), store.Count())
}

func TestStore_Eviction_LRU(t *testing.T) {
	config := StoreConfig{
		MaxCapacity:     3,
		MaxMemoryBytes:  1024 * 1024,
		EvictionPolicy:  "lru",
		CleanupInterval: 0,
	}
	store := NewStore(config)
	defer store.Close()

	ctx := context.Background()

	// Add 3 embeddings
	_ = store.Put(ctx, &Embedding{ID: "emb-1", Vector: []float32{0.1}, ModelID: "model"})
	time.Sleep(10 * time.Millisecond)
	_ = store.Put(ctx, &Embedding{ID: "emb-2", Vector: []float32{0.2}, ModelID: "model"})
	time.Sleep(10 * time.Millisecond)
	_ = store.Put(ctx, &Embedding{ID: "emb-3", Vector: []float32{0.3}, ModelID: "model"})

	// Access emb-1 to make it recently used
	_, _ = store.Get(ctx, "emb-1")

	// Add 4th embedding - should evict emb-2 (least recently used)
	_ = store.Put(ctx, &Embedding{ID: "emb-4", Vector: []float32{0.4}, ModelID: "model"})

	assert.Equal(t, int64(3), store.Count())

	// emb-1 should still exist (recently accessed)
	_, err := store.Get(ctx, "emb-1")
	assert.NoError(t, err)

	// emb-4 should exist
	_, err = store.Get(ctx, "emb-4")
	assert.NoError(t, err)
}

func TestStore_Eviction_LFU(t *testing.T) {
	config := StoreConfig{
		MaxCapacity:     3,
		MaxMemoryBytes:  1024 * 1024,
		EvictionPolicy:  "lfu",
		CleanupInterval: 0,
	}
	store := NewStore(config)
	defer store.Close()

	ctx := context.Background()

	// Add 3 embeddings
	_ = store.Put(ctx, &Embedding{ID: "emb-1", Vector: []float32{0.1}, ModelID: "model"})
	_ = store.Put(ctx, &Embedding{ID: "emb-2", Vector: []float32{0.2}, ModelID: "model"})
	_ = store.Put(ctx, &Embedding{ID: "emb-3", Vector: []float32{0.3}, ModelID: "model"})

	// Access emb-2 and emb-3 multiple times
	for i := 0; i < 5; i++ {
		_, _ = store.Get(ctx, "emb-2")
		_, _ = store.Get(ctx, "emb-3")
	}

	// Add 4th embedding - should evict emb-1 (least frequently used)
	_ = store.Put(ctx, &Embedding{ID: "emb-4", Vector: []float32{0.4}, ModelID: "model"})

	assert.Equal(t, int64(3), store.Count())

	// emb-2 and emb-3 should still exist (frequently accessed)
	_, err := store.Get(ctx, "emb-2")
	assert.NoError(t, err)

	_, err = store.Get(ctx, "emb-3")
	assert.NoError(t, err)
}

func TestStore_List(t *testing.T) {
	config := DefaultStoreConfig()
	config.CleanupInterval = 0
	store := NewStore(config)
	defer store.Close()

	ctx := context.Background()

	_ = store.Put(ctx, &Embedding{ID: "emb-1", Vector: []float32{0.1}, ModelID: "model-a"})
	_ = store.Put(ctx, &Embedding{ID: "emb-2", Vector: []float32{0.2}, ModelID: "model-a"})
	_ = store.Put(ctx, &Embedding{ID: "emb-3", Vector: []float32{0.3}, ModelID: "model-b"})

	// List all
	embeddings, err := store.List(ctx, "", 0, 0)
	require.NoError(t, err)
	assert.Len(t, embeddings, 3)

	// List by model
	embeddings, err = store.List(ctx, "model-a", 0, 0)
	require.NoError(t, err)
	assert.Len(t, embeddings, 2)

	// List with limit
	embeddings, err = store.List(ctx, "", 2, 0)
	require.NoError(t, err)
	assert.Len(t, embeddings, 2)

	// List with offset
	embeddings, err = store.List(ctx, "", 10, 2)
	require.NoError(t, err)
	assert.Len(t, embeddings, 1)
}

func TestStore_Touch(t *testing.T) {
	config := DefaultStoreConfig()
	config.CleanupInterval = 0
	store := NewStore(config)
	defer store.Close()

	ctx := context.Background()

	emb := &Embedding{
		ID:      "emb-1",
		Vector:  []float32{0.1},
		ModelID: "model",
	}
	_ = store.Put(ctx, emb)

	originalAccess := emb.LastAccessedAt
	originalCount := emb.AccessCount

	time.Sleep(10 * time.Millisecond)

	err := store.Touch(ctx, "emb-1")
	require.NoError(t, err)

	retrieved, _ := store.Get(ctx, "emb-1")
	assert.True(t, retrieved.LastAccessedAt.After(originalAccess))
	assert.Greater(t, retrieved.AccessCount, originalCount)
}

func TestStore_Exists(t *testing.T) {
	config := DefaultStoreConfig()
	config.CleanupInterval = 0
	store := NewStore(config)
	defer store.Close()

	ctx := context.Background()

	_ = store.Put(ctx, &Embedding{ID: "emb-1", Vector: []float32{0.1}, ModelID: "model", ContentHash: "hash-1"})

	assert.True(t, store.Exists(ctx, "emb-1"))
	assert.False(t, store.Exists(ctx, "nonexistent"))

	assert.True(t, store.ExistsByHash(ctx, "hash-1"))
	assert.False(t, store.ExistsByHash(ctx, "nonexistent"))
}

func TestStore_Stats(t *testing.T) {
	config := DefaultStoreConfig()
	config.CleanupInterval = 0
	store := NewStore(config)
	defer store.Close()

	ctx := context.Background()

	_ = store.Put(ctx, &Embedding{ID: "emb-1", Vector: []float32{0.1}, ModelID: "model-1"})
	_ = store.Put(ctx, &Embedding{ID: "emb-2", Vector: []float32{0.2}, ModelID: "model-2"})

	// Cache hit
	_, _ = store.Get(ctx, "emb-1")

	// Cache miss
	_, _ = store.Get(ctx, "nonexistent")

	stats := store.Stats()
	assert.Equal(t, int64(2), stats["total_embeddings"].(int64))
	assert.Equal(t, int64(1), stats["cache_hits"].(int64))
	assert.Equal(t, int64(1), stats["cache_misses"].(int64))
	assert.Equal(t, 2, stats["model_count"].(int))
}

func TestStore_Close(t *testing.T) {
	config := DefaultStoreConfig()
	config.CleanupInterval = time.Second
	store := NewStore(config)

	err := store.Close()
	require.NoError(t, err)

	// Operations should fail after close
	ctx := context.Background()
	err = store.Put(ctx, &Embedding{ID: "test", Vector: []float32{0.1}, ModelID: "model"})
	assert.ErrorIs(t, err, ErrStoreClosed)

	_, err = store.Get(ctx, "test")
	assert.ErrorIs(t, err, ErrStoreClosed)
}

func TestEmbedding_Fields(t *testing.T) {
	now := time.Now()
	emb := &Embedding{
		ID:             "emb-1",
		ContentHash:    "hash-123",
		Vector:         []float32{0.1, 0.2, 0.3},
		Dimension:      3,
		ModelID:        "text-embedding-ada-002",
		ModelVersion:   "v1",
		Content:        "test content",
		Metadata:       map[string]interface{}{"key": "value"},
		CreatedAt:      now,
		LastAccessedAt: now,
		AccessCount:    5,
		ExpiresAt:      now.Add(time.Hour),
		ByteSize:       100,
	}

	assert.Equal(t, "emb-1", emb.ID)
	assert.Equal(t, 3, emb.Dimension)
	assert.Equal(t, int64(5), emb.AccessCount)
}
