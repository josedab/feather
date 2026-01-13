package embedding

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultBatchConfig(t *testing.T) {
	config := DefaultBatchConfig()

	assert.Equal(t, 100, config.MaxBatchSize)
	assert.Equal(t, 5, config.MaxConcurrency)
	assert.Equal(t, 10.0, config.RequestsPerSecond)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, time.Second, config.RetryDelay)
	assert.Equal(t, 60*time.Second, config.Timeout)
	assert.True(t, config.UseCache)
}

func TestNewBatchProcessor(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedup := NewDeduplicator(dedupConfig, store)

	batchConfig := DefaultBatchConfig()
	provider := NewMockProvider(1536, 0)

	processor := NewBatchProcessor(batchConfig, store, dedup, provider)
	require.NotNil(t, processor)
}

func TestBatchProcessor_Submit(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedup := NewDeduplicator(dedupConfig, store)

	batchConfig := DefaultBatchConfig()
	provider := NewMockProvider(1536, 0)

	processor := NewBatchProcessor(batchConfig, store, dedup, provider)

	req := &BatchRequest{
		ID:       "req-1",
		Contents: []string{"text 1", "text 2"},
		ModelID:  "model-1",
	}

	err := processor.Submit(req)
	require.NoError(t, err)

	assert.Equal(t, 1, processor.QueueLength())
}

func TestBatchProcessor_Submit_EmptyBatch(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	batchConfig := DefaultBatchConfig()
	processor := NewBatchProcessor(batchConfig, store, nil, nil)

	req := &BatchRequest{
		ID:       "req-1",
		Contents: []string{},
		ModelID:  "model-1",
	}

	err := processor.Submit(req)
	assert.ErrorIs(t, err, ErrBatchEmpty)
}

func TestBatchProcessor_Submit_BatchTooLarge(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	batchConfig := DefaultBatchConfig()
	batchConfig.MaxBatchSize = 5
	processor := NewBatchProcessor(batchConfig, store, nil, nil)

	contents := make([]string, 10)
	for i := range contents {
		contents[i] = "text"
	}

	req := &BatchRequest{
		ID:       "req-1",
		Contents: contents,
		ModelID:  "model-1",
	}

	err := processor.Submit(req)
	assert.ErrorIs(t, err, ErrBatchTooLarge)
}

func TestBatchProcessor_ProcessSync(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedup := NewDeduplicator(dedupConfig, store)

	batchConfig := DefaultBatchConfig()
	provider := NewMockProvider(1536, 10*time.Millisecond)

	processor := NewBatchProcessor(batchConfig, store, dedup, provider)

	req := &BatchRequest{
		ID:       "req-1",
		Contents: []string{"text 1", "text 2", "text 3"},
		ModelID:  "model-1",
	}

	ctx := context.Background()
	result, err := processor.ProcessSync(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, "req-1", result.RequestID)
	assert.Len(t, result.Embeddings, 3)
	assert.Equal(t, 1, result.APICalls)

	for _, emb := range result.Embeddings {
		assert.NotNil(t, emb)
		assert.Equal(t, 1536, emb.Dimension)
		assert.Equal(t, "model-1", emb.ModelID)
	}
}

func TestBatchProcessor_ProcessSync_WithCache(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedup := NewDeduplicator(dedupConfig, store)

	batchConfig := DefaultBatchConfig()
	batchConfig.UseCache = true
	provider := NewMockProvider(1536, 0)

	processor := NewBatchProcessor(batchConfig, store, dedup, provider)
	ctx := context.Background()

	// First request
	req1 := &BatchRequest{
		ID:       "req-1",
		Contents: []string{"text 1", "text 2"},
		ModelID:  "model-1",
	}
	result1, err := processor.ProcessSync(ctx, req1)
	require.NoError(t, err)
	assert.Equal(t, 0, result1.CacheHits)
	assert.Equal(t, 1, result1.APICalls)

	// Second request with same content
	req2 := &BatchRequest{
		ID:       "req-2",
		Contents: []string{"text 1", "text 2"},
		ModelID:  "model-1",
	}
	result2, err := processor.ProcessSync(ctx, req2)
	require.NoError(t, err)
	assert.Equal(t, 2, result2.CacheHits)
	assert.Equal(t, 0, result2.APICalls)
}

func TestBatchProcessor_ProcessSync_CacheDisabled(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedup := NewDeduplicator(dedupConfig, store)

	batchConfig := DefaultBatchConfig()
	batchConfig.UseCache = false
	provider := NewMockProvider(1536, 0)

	processor := NewBatchProcessor(batchConfig, store, dedup, provider)
	ctx := context.Background()

	// First request
	req := &BatchRequest{
		ID:       "req-1",
		Contents: []string{"text 1"},
		ModelID:  "model-1",
	}
	result, err := processor.ProcessSync(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, 0, result.CacheHits)
	assert.Equal(t, 1, result.APICalls)
}

func TestBatchProcessor_ProcessSync_Empty(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	batchConfig := DefaultBatchConfig()
	processor := NewBatchProcessor(batchConfig, store, nil, nil)

	req := &BatchRequest{
		ID:       "req-1",
		Contents: []string{},
		ModelID:  "model-1",
	}

	ctx := context.Background()
	_, err := processor.ProcessSync(ctx, req)
	assert.ErrorIs(t, err, ErrBatchEmpty)
}

func TestBatchProcessor_StartStop(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	batchConfig := DefaultBatchConfig()
	batchConfig.MaxConcurrency = 2
	provider := NewMockProvider(1536, 10*time.Millisecond)

	processor := NewBatchProcessor(batchConfig, store, nil, provider)

	processor.Start()
	defer processor.Stop()

	// Submit some requests
	for i := 0; i < 3; i++ {
		req := &BatchRequest{
			ID:       "req-" + string(rune('a'+i)),
			Contents: []string{"text"},
			ModelID:  "model-1",
		}
		_ = processor.Submit(req)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Queue should be empty or nearly empty
	assert.LessOrEqual(t, processor.QueueLength(), 1)
}

func TestBatchProcessor_Priority(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	batchConfig := DefaultBatchConfig()
	batchConfig.MaxConcurrency = 1
	provider := NewMockProvider(1536, 50*time.Millisecond)

	processor := NewBatchProcessor(batchConfig, store, nil, provider)

	// Submit low priority first
	_ = processor.Submit(&BatchRequest{
		ID:       "low",
		Contents: []string{"text"},
		ModelID:  "model-1",
		Priority: 1,
	})

	// Submit high priority
	_ = processor.Submit(&BatchRequest{
		ID:       "high",
		Contents: []string{"text"},
		ModelID:  "model-1",
		Priority: 100,
	})

	// High priority should be processed first when worker starts
	processor.Start()
	defer processor.Stop()

	time.Sleep(100 * time.Millisecond)
}

func TestBatchProcessor_Stats(t *testing.T) {
	storeConfig := DefaultStoreConfig()
	storeConfig.CleanupInterval = 0
	store := NewStore(storeConfig)
	defer store.Close()

	dedupConfig := DefaultDeduplicationConfig()
	dedup := NewDeduplicator(dedupConfig, store)

	batchConfig := DefaultBatchConfig()
	provider := NewMockProvider(1536, 0)

	processor := NewBatchProcessor(batchConfig, store, dedup, provider)
	ctx := context.Background()

	req := &BatchRequest{
		ID:       "req-1",
		Contents: []string{"text 1", "text 2"},
		ModelID:  "model-1",
	}
	_, _ = processor.ProcessSync(ctx, req)

	stats := processor.Stats()
	assert.Equal(t, int64(1), stats["requests_processed"].(int64))
	assert.Equal(t, int64(2), stats["items_processed"].(int64))
	assert.GreaterOrEqual(t, stats["api_calls"].(int64), int64(1))
	assert.Equal(t, 100, stats["max_batch_size"].(int))
}

func TestBatchRequest_Fields(t *testing.T) {
	req := &BatchRequest{
		ID:       "req-1",
		Contents: []string{"text 1", "text 2"},
		Priority: 10,
	}

	assert.Equal(t, "req-1", req.ID)
	assert.Len(t, req.Contents, 2)
	assert.Equal(t, 10, req.Priority)
}

func TestBatchResult_Fields(t *testing.T) {
	result := &BatchResult{
		RequestID:  "req-1",
		Embeddings: []*Embedding{{ID: "emb-1"}},
		CacheHits:  5,
		APICalls:   2,
	}

	assert.Equal(t, "req-1", result.RequestID)
	assert.Len(t, result.Embeddings, 1)
	assert.Equal(t, 5, result.CacheHits)
	assert.Equal(t, 2, result.APICalls)
}

func TestMockProvider(t *testing.T) {
	provider := NewMockProvider(1536, 0)

	assert.Equal(t, "mock", provider.Name())

	dim, err := provider.GetDimension("any-model")
	require.NoError(t, err)
	assert.Equal(t, 1536, dim)
}

func TestMockProvider_GenerateEmbeddings(t *testing.T) {
	provider := NewMockProvider(768, 0)

	ctx := context.Background()
	texts := []string{"hello", "world"}

	vectors, err := provider.GenerateEmbeddings(ctx, texts, "model-1")
	require.NoError(t, err)
	assert.Len(t, vectors, 2)

	for _, vec := range vectors {
		assert.Len(t, vec, 768)
	}
}

func TestMockProvider_GenerateEmbeddings_WithDelay(t *testing.T) {
	provider := NewMockProvider(128, 50*time.Millisecond)

	ctx := context.Background()
	start := time.Now()

	vectors, err := provider.GenerateEmbeddings(ctx, []string{"text"}, "model")
	require.NoError(t, err)
	assert.Len(t, vectors, 1)

	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
}

func TestMockProvider_GenerateEmbeddings_ContextCancelled(t *testing.T) {
	provider := NewMockProvider(128, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := provider.GenerateEmbeddings(ctx, []string{"text"}, "model")
	assert.Error(t, err)
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	// IDs should be unique (with high probability)
	assert.NotEqual(t, id1, id2)
}

func TestRandomHex(t *testing.T) {
	hex1 := randomHex(8)
	hex2 := randomHex(8)

	assert.Len(t, hex1, 8)
	assert.Len(t, hex2, 8)

	// All characters should be valid hex
	for _, c := range hex1 {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'))
	}
}
