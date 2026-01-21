package embedding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultDeduplicationConfig(t *testing.T) {
	config := DefaultDeduplicationConfig()

	assert.True(t, config.Enabled)
	assert.True(t, config.NormalizeContent)
	assert.True(t, config.IncludeModelInHash)
	assert.Equal(t, "sha256", config.HashAlgorithm)
}

func TestNewDeduplicator(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedup := NewDeduplicator(dedupConfig, store)

	require.NotNil(t, dedup)
}

func TestDeduplicator_HashContent(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedup := NewDeduplicator(dedupConfig, store)

	hash1 := dedup.HashContent("test content", "model-1")
	hash2 := dedup.HashContent("test content", "model-1")

	// Same content and model should produce same hash
	assert.Equal(t, hash1, hash2)
	assert.NotEmpty(t, hash1)
}

func TestDeduplicator_HashContent_DifferentModels(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedupConfig.IncludeModelInHash = true
	dedup := NewDeduplicator(dedupConfig, store)

	hash1 := dedup.HashContent("test content", "model-1")
	hash2 := dedup.HashContent("test content", "model-2")

	// Same content but different models should produce different hashes
	assert.NotEqual(t, hash1, hash2)
}

func TestDeduplicator_HashContent_WithoutModelInHash(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedupConfig.IncludeModelInHash = false
	dedup := NewDeduplicator(dedupConfig, store)

	hash1 := dedup.HashContent("test content", "model-1")
	hash2 := dedup.HashContent("test content", "model-2")

	// Same content, different models, without model in hash - same hash
	assert.Equal(t, hash1, hash2)
}

func TestDeduplicator_HashContent_Disabled(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedupConfig.Enabled = false
	dedup := NewDeduplicator(dedupConfig, store)

	hash := dedup.HashContent("test content", "model-1")
	assert.Empty(t, hash)
}

func TestDeduplicator_HashContent_Normalization(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedupConfig.NormalizeContent = true
	dedup := NewDeduplicator(dedupConfig, store)

	// Different whitespace should produce same hash
	hash1 := dedup.HashContent("test  content", "model-1")
	hash2 := dedup.HashContent("test content", "model-1")
	hash3 := dedup.HashContent("TEST CONTENT", "model-1")

	assert.Equal(t, hash1, hash2)
	assert.Equal(t, hash2, hash3)
}

func TestDeduplicator_CheckDuplicate(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedup := NewDeduplicator(dedupConfig, store)

	ctx := context.Background()

	// Store an embedding
	hash := dedup.HashContent("test content", "model-1")
	emb := &Embedding{
		ID:          "emb-1",
		ContentHash: hash,
		Vector:      []float32{0.1, 0.2, 0.3},
		ModelID:     "model-1",
	}
	_ = store.Put(ctx, emb)

	// Check for duplicate
	found, isDuplicate := dedup.CheckDuplicate(ctx, "test content", "model-1")
	assert.True(t, isDuplicate)
	assert.NotNil(t, found)
	assert.Equal(t, "emb-1", found.ID)

	// Check for non-duplicate
	found, isDuplicate = dedup.CheckDuplicate(ctx, "different content", "model-1")
	assert.False(t, isDuplicate)
	assert.Nil(t, found)
}

func TestDeduplicator_CheckDuplicate_Disabled(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedupConfig.Enabled = false
	dedup := NewDeduplicator(dedupConfig, store)

	ctx := context.Background()

	found, isDuplicate := dedup.CheckDuplicate(ctx, "test content", "model-1")
	assert.False(t, isDuplicate)
	assert.Nil(t, found)
}

func TestDeduplicator_GetOrCreate(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedup := NewDeduplicator(dedupConfig, store)

	ctx := context.Background()

	// First call - no existing embedding
	emb, exists, err := dedup.GetOrCreate(ctx, "test content", "model-1")
	require.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, emb)

	// Store an embedding
	hash := dedup.HashContent("test content", "model-1")
	storedEmb := &Embedding{
		ID:          "emb-1",
		ContentHash: hash,
		Vector:      []float32{0.1, 0.2, 0.3},
		ModelID:     "model-1",
	}
	_ = store.Put(ctx, storedEmb)

	// Second call - existing embedding
	emb, exists, err = dedup.GetOrCreate(ctx, "test content", "model-1")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.NotNil(t, emb)
	assert.Equal(t, "emb-1", emb.ID)
}

func TestDeduplicator_RecordDeduplication(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedup := NewDeduplicator(dedupConfig, store)

	dedup.RecordDeduplication(1024)
	dedup.RecordDeduplication(2048)

	stats := dedup.Stats()
	assert.Equal(t, int64(3072), stats["bytes_deduped"].(int64))
}

func TestDeduplicator_Stats(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedup := NewDeduplicator(dedupConfig, store)

	ctx := context.Background()

	// Store an embedding
	hash := dedup.HashContent("test content", "model-1")
	emb := &Embedding{
		ID:          "emb-1",
		ContentHash: hash,
		Vector:      []float32{0.1},
		ModelID:     "model-1",
	}
	_ = store.Put(ctx, emb)

	// Check for duplicate (found)
	_, _ = dedup.CheckDuplicate(ctx, "test content", "model-1")

	// Check for unique content (not found)
	_, _, _ = dedup.GetOrCreate(ctx, "new content", "model-1")

	stats := dedup.Stats()
	assert.True(t, stats["enabled"].(bool))
	assert.Equal(t, "sha256", stats["hash_algorithm"].(string))
	assert.Equal(t, int64(1), stats["duplicates_found"].(int64))
	assert.Equal(t, int64(1), stats["unique_content"].(int64))
}

func TestNormalizeContent(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello world", "hello world"},
		{"HELLO WORLD", "hello world"},
		{"hello  world", "hello world"},
		{"  hello  world  ", "hello world"},
		{"hello\n\tworld", "hello world"},
		{"Hello   World", "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeContent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContentHasher(t *testing.T) {
	hasher := NewContentHasher("sha256")

	hash1 := hasher.Hash("test content")
	hash2 := hasher.Hash("test content")

	assert.Equal(t, hash1, hash2)
	assert.Len(t, hash1, 64) // SHA256 produces 64 hex characters
}

func TestContentHasher_HashWithModel(t *testing.T) {
	hasher := NewContentHasher("sha256")

	hash1 := hasher.HashWithModel("content", "model-1", "v1")
	hash2 := hasher.HashWithModel("content", "model-1", "v2")

	// Different versions should produce different hashes
	assert.NotEqual(t, hash1, hash2)
}

func TestContentHasher_DefaultAlgorithm(t *testing.T) {
	hasher := NewContentHasher("")

	hash := hasher.Hash("test")
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 64) // Defaults to SHA256
}
