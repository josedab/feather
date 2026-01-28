package llm

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/storage"
)

// Pipeline errors.
var (
	ErrProviderNotConfigured = errors.New("embedding provider not configured")
	ErrEmptyInput            = errors.New("empty input text")
	ErrChunkingFailed        = errors.New("text chunking failed")
	ErrEmbeddingFailed       = errors.New("embedding generation failed")
	ErrCacheFull             = errors.New("embedding cache full")
)

// PipelineConfig configures the LLM feature pipeline.
type PipelineConfig struct {
	// Provider is the embedding provider.
	Provider Provider `json:"-" yaml:"-"`

	// ProviderType is used for serialization.
	ProviderType ProviderType `json:"provider_type" yaml:"provider_type"`

	// Chunker configures text chunking.
	Chunker ChunkerConfig `json:"chunker" yaml:"chunker"`

	// BatchSize is the max batch size for embedding requests.
	BatchSize int `json:"batch_size" yaml:"batch_size"`

	// CacheEnabled enables embedding caching.
	CacheEnabled bool `json:"cache_enabled" yaml:"cache_enabled"`

	// CacheMaxSize is the maximum cache entries.
	CacheMaxSize int `json:"cache_max_size" yaml:"cache_max_size"`

	// CacheTTL is the cache entry TTL.
	CacheTTL time.Duration `json:"cache_ttl" yaml:"cache_ttl"`

	// StoreChunks stores individual chunk embeddings.
	StoreChunks bool `json:"store_chunks" yaml:"store_chunks"`

	// AggregateMethod defines how to aggregate chunk embeddings.
	AggregateMethod AggregateMethod `json:"aggregate_method" yaml:"aggregate_method"`

	// AutoRegisterFeatures automatically registers embedding features.
	AutoRegisterFeatures bool `json:"auto_register" yaml:"auto_register"`

	// FeaturePrefix is the prefix for generated feature names.
	FeaturePrefix string `json:"feature_prefix" yaml:"feature_prefix"`
}

// AggregateMethod defines how to combine multiple chunk embeddings.
type AggregateMethod string

const (
	// AggregateMean averages all chunk embeddings.
	AggregateMean AggregateMethod = "mean"
	// AggregateFirst uses only the first chunk's embedding.
	AggregateFirst AggregateMethod = "first"
	// AggregateMax takes element-wise max across chunks.
	AggregateMax AggregateMethod = "max"
	// AggregateWeighted uses position-weighted averaging.
	AggregateWeighted AggregateMethod = "weighted"
)

// DefaultPipelineConfig returns the default pipeline configuration.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		Chunker:         DefaultChunkerConfig(),
		BatchSize:       32,
		CacheEnabled:    true,
		CacheMaxSize:    10000,
		CacheTTL:        24 * time.Hour,
		StoreChunks:     false,
		AggregateMethod: AggregateMean,
		FeaturePrefix:   "emb_",
	}
}

// Pipeline manages text-to-embedding feature generation.
type Pipeline struct {
	config   PipelineConfig
	provider Provider
	chunker  *Chunker
	cache    *embeddingCache
	store    *storage.Store

	// Metrics
	processCount    int64
	chunkCount      int64
	embeddingCount  int64
	cacheHits       int64
	cacheMisses     int64
	processingTime  int64 // microseconds
	mu              sync.RWMutex
}

// NewPipeline creates a new LLM feature pipeline.
func NewPipeline(config PipelineConfig, store *storage.Store) *Pipeline {
	if config.BatchSize == 0 {
		config.BatchSize = 32
	}
	if config.AggregateMethod == "" {
		config.AggregateMethod = AggregateMean
	}
	if config.FeaturePrefix == "" {
		config.FeaturePrefix = "emb_"
	}

	p := &Pipeline{
		config:   config,
		provider: config.Provider,
		chunker:  NewChunker(config.Chunker),
		store:    store,
	}

	if config.CacheEnabled {
		p.cache = newEmbeddingCache(config.CacheMaxSize, config.CacheTTL)
	}

	return p
}

// SetProvider sets the embedding provider.
func (p *Pipeline) SetProvider(provider Provider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.provider = provider
}

// Process generates embeddings for text and stores as a feature.
func (p *Pipeline) Process(ctx context.Context, entityKey, featureName, text string) (*EmbeddingResult, error) {
	start := time.Now()
	defer func() {
		atomic.AddInt64(&p.processingTime, time.Since(start).Microseconds())
		atomic.AddInt64(&p.processCount, 1)
	}()

	if p.provider == nil {
		return nil, ErrProviderNotConfigured
	}

	if len(text) == 0 {
		return nil, ErrEmptyInput
	}

	// Check cache first
	contentHash := ContentHash(text)
	if p.cache != nil {
		if cached, ok := p.cache.Get(contentHash); ok {
			atomic.AddInt64(&p.cacheHits, 1)
			return p.storeResult(ctx, entityKey, featureName, cached, contentHash)
		}
		atomic.AddInt64(&p.cacheMisses, 1)
	}

	// Chunk text
	chunks := p.chunker.Split(text)
	if len(chunks) == 0 {
		return nil, ErrChunkingFailed
	}
	atomic.AddInt64(&p.chunkCount, int64(len(chunks)))

	// Generate embeddings for chunks
	chunkTexts := make([]string, len(chunks))
	for i, c := range chunks {
		chunkTexts[i] = c.Text
	}

	embeddings, err := p.embedBatched(ctx, chunkTexts)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEmbeddingFailed, err)
	}
	atomic.AddInt64(&p.embeddingCount, int64(len(embeddings)))

	// Aggregate embeddings
	aggregated := p.aggregate(embeddings)

	// Cache result
	if p.cache != nil {
		p.cache.Put(contentHash, aggregated)
	}

	// Store result
	return p.storeResult(ctx, entityKey, featureName, aggregated, contentHash)
}

// ProcessBatch processes multiple texts for an entity.
func (p *Pipeline) ProcessBatch(ctx context.Context, entityKey string, features map[string]string) ([]*EmbeddingResult, error) {
	results := make([]*EmbeddingResult, 0, len(features))

	for featureName, text := range features {
		result, err := p.Process(ctx, entityKey, featureName, text)
		if err != nil {
			return results, fmt.Errorf("processing %s: %w", featureName, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// Embed generates an embedding without storing.
func (p *Pipeline) Embed(ctx context.Context, text string) ([]float32, error) {
	if p.provider == nil {
		return nil, ErrProviderNotConfigured
	}

	if len(text) == 0 {
		return nil, ErrEmptyInput
	}

	// Check cache
	contentHash := ContentHash(text)
	if p.cache != nil {
		if cached, ok := p.cache.Get(contentHash); ok {
			return cached, nil
		}
	}

	// Chunk and embed
	chunks := p.chunker.Split(text)
	if len(chunks) == 0 {
		return nil, ErrChunkingFailed
	}

	chunkTexts := make([]string, len(chunks))
	for i, c := range chunks {
		chunkTexts[i] = c.Text
	}

	embeddings, err := p.embedBatched(ctx, chunkTexts)
	if err != nil {
		return nil, err
	}

	aggregated := p.aggregate(embeddings)

	// Cache
	if p.cache != nil {
		p.cache.Put(contentHash, aggregated)
	}

	return aggregated, nil
}

// EmbedChunks generates embeddings for each chunk separately.
func (p *Pipeline) EmbedChunks(ctx context.Context, text string) ([]ChunkEmbedding, error) {
	if p.provider == nil {
		return nil, ErrProviderNotConfigured
	}

	chunks := p.chunker.Split(text)
	if len(chunks) == 0 {
		return nil, ErrChunkingFailed
	}

	chunkTexts := make([]string, len(chunks))
	for i, c := range chunks {
		chunkTexts[i] = c.Text
	}

	embeddings, err := p.embedBatched(ctx, chunkTexts)
	if err != nil {
		return nil, err
	}

	result := make([]ChunkEmbedding, len(chunks))
	for i, chunk := range chunks {
		result[i] = ChunkEmbedding{
			Chunk:     chunk,
			Embedding: embeddings[i],
		}
	}

	return result, nil
}

func (p *Pipeline) embedBatched(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) <= p.config.BatchSize {
		return p.provider.EmbedBatch(ctx, texts)
	}

	// Process in batches
	var allEmbeddings [][]float32
	for i := 0; i < len(texts); i += p.config.BatchSize {
		end := i + p.config.BatchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		embeddings, err := p.provider.EmbedBatch(ctx, batch)
		if err != nil {
			return nil, err
		}
		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

func (p *Pipeline) aggregate(embeddings [][]float32) []float32 {
	if len(embeddings) == 0 {
		return nil
	}

	if len(embeddings) == 1 {
		return embeddings[0]
	}

	dim := len(embeddings[0])

	switch p.config.AggregateMethod {
	case AggregateFirst:
		return embeddings[0]

	case AggregateMax:
		result := make([]float32, dim)
		copy(result, embeddings[0])
		for i := 1; i < len(embeddings); i++ {
			for j := 0; j < dim; j++ {
				if embeddings[i][j] > result[j] {
					result[j] = embeddings[i][j]
				}
			}
		}
		return result

	case AggregateWeighted:
		// Give higher weight to earlier chunks
		result := make([]float32, dim)
		totalWeight := float32(0)
		for i, emb := range embeddings {
			weight := 1.0 / float32(i+1)
			totalWeight += weight
			for j := 0; j < dim; j++ {
				result[j] += emb[j] * weight
			}
		}
		for j := 0; j < dim; j++ {
			result[j] /= totalWeight
		}
		return result

	case AggregateMean:
		fallthrough
	default:
		result := make([]float32, dim)
		for _, emb := range embeddings {
			for j := 0; j < dim; j++ {
				result[j] += emb[j]
			}
		}
		n := float32(len(embeddings))
		for j := 0; j < dim; j++ {
			result[j] /= n
		}
		return result
	}
}

func (p *Pipeline) storeResult(ctx context.Context, entityKey, featureName string, embedding []float32, contentHash string) (*EmbeddingResult, error) {
	result := &EmbeddingResult{
		EntityKey:   entityKey,
		FeatureName: p.config.FeaturePrefix + featureName,
		Embedding:   embedding,
		Dimension:   len(embedding),
		ModelID:     p.provider.ModelID(),
		ContentHash: contentHash,
		Timestamp:   time.Now(),
	}

	if p.store != nil {
		// Store as vector feature
		err := p.store.Put(entityKey, map[string]*domain.FeatureValue{
			result.FeatureName: {
				Value:     embedding,
				Timestamp: time.Now().UnixNano(),
				Version:   1,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("storing feature: %w", err)
		}
	}

	return result, nil
}

// Stats returns pipeline statistics.
func (p *Pipeline) Stats() PipelineStats {
	return PipelineStats{
		ProcessCount:   atomic.LoadInt64(&p.processCount),
		ChunkCount:     atomic.LoadInt64(&p.chunkCount),
		EmbeddingCount: atomic.LoadInt64(&p.embeddingCount),
		CacheHits:      atomic.LoadInt64(&p.cacheHits),
		CacheMisses:    atomic.LoadInt64(&p.cacheMisses),
		AvgProcessingTimeUs: func() int64 {
			count := atomic.LoadInt64(&p.processCount)
			if count == 0 {
				return 0
			}
			return atomic.LoadInt64(&p.processingTime) / count
		}(),
		ProviderModelID: func() string {
			if p.provider != nil {
				return p.provider.ModelID()
			}
			return ""
		}(),
		Dimension: func() int {
			if p.provider != nil {
				return p.provider.Dimension()
			}
			return 0
		}(),
	}
}

// ClearCache clears the embedding cache.
func (p *Pipeline) ClearCache() {
	if p.cache != nil {
		p.cache.Clear()
	}
}

// EmbeddingResult represents the result of processing text.
type EmbeddingResult struct {
	EntityKey   string    `json:"entity_key"`
	FeatureName string    `json:"feature_name"`
	Embedding   []float32 `json:"embedding"`
	Dimension   int       `json:"dimension"`
	ModelID     string    `json:"model_id"`
	ContentHash string    `json:"content_hash"`
	Timestamp   time.Time `json:"timestamp"`
}

// ChunkEmbedding pairs a chunk with its embedding.
type ChunkEmbedding struct {
	Chunk     Chunk     `json:"chunk"`
	Embedding []float32 `json:"embedding"`
}

// PipelineStats contains pipeline statistics.
type PipelineStats struct {
	ProcessCount        int64  `json:"process_count"`
	ChunkCount          int64  `json:"chunk_count"`
	EmbeddingCount      int64  `json:"embedding_count"`
	CacheHits           int64  `json:"cache_hits"`
	CacheMisses         int64  `json:"cache_misses"`
	AvgProcessingTimeUs int64  `json:"avg_processing_time_us"`
	ProviderModelID     string `json:"provider_model_id"`
	Dimension           int    `json:"dimension"`
}

// Embedding cache implementation
type embeddingCache struct {
	mu       sync.RWMutex
	entries  map[string]*cacheEntry
	maxSize  int
	ttl      time.Duration
	evictCh  chan struct{}
}

type cacheEntry struct {
	embedding  []float32
	expiration time.Time
}

func newEmbeddingCache(maxSize int, ttl time.Duration) *embeddingCache {
	c := &embeddingCache{
		entries: make(map[string]*cacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
		evictCh: make(chan struct{}),
	}
	go c.evictionLoop()
	return c
}

func (c *embeddingCache) Get(key string) ([]float32, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok || time.Now().After(entry.expiration) {
		return nil, false
	}

	return entry.embedding, true
}

func (c *embeddingCache) Put(key string, embedding []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if full
	if len(c.entries) >= c.maxSize {
		// Simple random eviction
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}

	c.entries[key] = &cacheEntry{
		embedding:  embedding,
		expiration: time.Now().Add(c.ttl),
	}
}

func (c *embeddingCache) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]*cacheEntry)
	c.mu.Unlock()
}

func (c *embeddingCache) evictionLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-c.evictCh:
			return
		case <-ticker.C:
			c.evictExpired()
		}
	}
}

func (c *embeddingCache) evictExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for k, v := range c.entries {
		if now.After(v.expiration) {
			delete(c.entries, k)
		}
	}
}

func (c *embeddingCache) Close() {
	close(c.evictCh)
}
