package embedding

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Errors returned by batch processing.
var (
	ErrBatchTooLarge     = errors.New("batch size exceeds limit")
	ErrBatchEmpty        = errors.New("batch is empty")
	ErrProviderError     = errors.New("embedding provider error")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrBatchCancelled    = errors.New("batch processing cancelled")
)

// BatchRequest represents a batch embedding request.
type BatchRequest struct {
	// ID is the unique request ID.
	ID string `json:"id"`

	// Contents are the texts to embed.
	Contents []string `json:"contents"`

	// ModelID is the embedding model to use.
	ModelID string `json:"model_id"`

	// ModelVersion is the model version (optional).
	ModelVersion string `json:"model_version,omitempty"`

	// Priority affects processing order (higher = first).
	Priority int `json:"priority"`

	// Metadata contains additional information.
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// SubmittedAt is when the request was submitted.
	SubmittedAt time.Time `json:"submitted_at"`
}

// BatchResult represents the result of a batch request.
type BatchResult struct {
	// RequestID is the original request ID.
	RequestID string `json:"request_id"`

	// Embeddings are the generated embeddings.
	Embeddings []*Embedding `json:"embeddings"`

	// Errors contains any per-item errors.
	Errors []error `json:"errors,omitempty"`

	// ProcessedAt is when processing completed.
	ProcessedAt time.Time `json:"processed_at"`

	// Duration is how long processing took.
	Duration time.Duration `json:"duration"`

	// CacheHits is the number of cache hits.
	CacheHits int `json:"cache_hits"`

	// APICalls is the number of API calls made.
	APICalls int `json:"api_calls"`
}

// EmbeddingProvider is the interface for embedding generation.
type EmbeddingProvider interface {
	// GenerateEmbeddings generates embeddings for the given texts.
	GenerateEmbeddings(ctx context.Context, texts []string, modelID string) ([][]float32, error)

	// GetDimension returns the embedding dimension for a model.
	GetDimension(modelID string) (int, error)

	// Name returns the provider name.
	Name() string
}

// BatchConfig configures the batch processor.
type BatchConfig struct {
	// MaxBatchSize is the maximum items per batch.
	MaxBatchSize int `json:"max_batch_size" yaml:"max_batch_size"`

	// MaxConcurrency is the maximum concurrent batches.
	MaxConcurrency int `json:"max_concurrency" yaml:"max_concurrency"`

	// RequestsPerSecond is the rate limit.
	RequestsPerSecond float64 `json:"requests_per_second" yaml:"requests_per_second"`

	// MaxRetries is the maximum retry attempts.
	MaxRetries int `json:"max_retries" yaml:"max_retries"`

	// RetryDelay is the delay between retries.
	RetryDelay time.Duration `json:"retry_delay" yaml:"retry_delay"`

	// Timeout is the request timeout.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// UseCache enables cache lookup before API calls.
	UseCache bool `json:"use_cache" yaml:"use_cache"`
}

// DefaultBatchConfig returns the default batch configuration.
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		MaxBatchSize:      100,
		MaxConcurrency:    5,
		RequestsPerSecond: 10.0,
		MaxRetries:        3,
		RetryDelay:        time.Second,
		Timeout:           60 * time.Second,
		UseCache:          true,
	}
}

// BatchProcessor handles batch embedding requests.
type BatchProcessor struct {
	mu       sync.RWMutex
	config   BatchConfig
	store    *Store
	dedup    *Deduplicator
	provider EmbeddingProvider

	// Rate limiting
	lastRequest time.Time
	tokenBucket float64

	// Queue
	queue     []*BatchRequest
	queueCond *sync.Cond

	// Metrics
	requestsProcessed int64
	itemsProcessed    int64
	cacheHits         int64
	apiCalls          int64
	errors            int64
	retries           int64

	// State
	running bool
	closeCh chan struct{}
	wg      sync.WaitGroup
}

// NewBatchProcessor creates a new batch processor.
func NewBatchProcessor(config BatchConfig, store *Store, dedup *Deduplicator, provider EmbeddingProvider) *BatchProcessor {
	p := &BatchProcessor{
		config:   config,
		store:    store,
		dedup:    dedup,
		provider: provider,
		queue:    make([]*BatchRequest, 0),
		closeCh:  make(chan struct{}),
	}
	p.queueCond = sync.NewCond(&p.mu)

	return p
}

// Start starts the batch processor.
func (p *BatchProcessor) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()

	for i := 0; i < p.config.MaxConcurrency; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop stops the batch processor.
func (p *BatchProcessor) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	close(p.closeCh)
	p.queueCond.Broadcast()
	p.mu.Unlock()

	p.wg.Wait()
}

// worker processes batch requests.
func (p *BatchProcessor) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.closeCh:
			return
		default:
		}

		p.mu.Lock()
		for len(p.queue) == 0 && p.running {
			p.queueCond.Wait()
		}

		if !p.running {
			p.mu.Unlock()
			return
		}

		// Get next request (priority queue)
		req := p.dequeue()
		p.mu.Unlock()

		if req == nil {
			continue
		}

		// Process request
		ctx, cancel := context.WithTimeout(context.Background(), p.config.Timeout)
		_, _ = p.processRequest(ctx, req)
		cancel()
	}
}

// dequeue removes and returns the highest priority request.
func (p *BatchProcessor) dequeue() *BatchRequest {
	if len(p.queue) == 0 {
		return nil
	}

	// Find highest priority
	maxIdx := 0
	for i, req := range p.queue {
		if req.Priority > p.queue[maxIdx].Priority {
			maxIdx = i
		}
	}

	req := p.queue[maxIdx]
	p.queue = append(p.queue[:maxIdx], p.queue[maxIdx+1:]...)

	return req
}

// Submit submits a batch request for processing.
func (p *BatchProcessor) Submit(req *BatchRequest) error {
	if len(req.Contents) == 0 {
		return ErrBatchEmpty
	}

	if len(req.Contents) > p.config.MaxBatchSize {
		return ErrBatchTooLarge
	}

	if req.SubmittedAt.IsZero() {
		req.SubmittedAt = time.Now()
	}

	p.mu.Lock()
	p.queue = append(p.queue, req)
	p.queueCond.Signal()
	p.mu.Unlock()

	return nil
}

// ProcessSync processes a batch synchronously.
func (p *BatchProcessor) ProcessSync(ctx context.Context, req *BatchRequest) (*BatchResult, error) {
	if len(req.Contents) == 0 {
		return nil, ErrBatchEmpty
	}

	if len(req.Contents) > p.config.MaxBatchSize {
		return nil, ErrBatchTooLarge
	}

	return p.processRequest(ctx, req)
}

// processRequest processes a single batch request.
func (p *BatchProcessor) processRequest(ctx context.Context, req *BatchRequest) (*BatchResult, error) {
	start := time.Now()

	result := &BatchResult{
		RequestID:   req.ID,
		Embeddings:  make([]*Embedding, len(req.Contents)),
		Errors:      make([]error, len(req.Contents)),
		ProcessedAt: time.Now(),
	}

	// Check cache for each item
	var uncachedIndices []int
	var uncachedContents []string

	if p.config.UseCache && p.dedup != nil {
		for i, content := range req.Contents {
			emb, found := p.dedup.CheckDuplicate(ctx, content, req.ModelID)
			if found {
				result.Embeddings[i] = emb
				result.CacheHits++
				atomic.AddInt64(&p.cacheHits, 1)
			} else {
				uncachedIndices = append(uncachedIndices, i)
				uncachedContents = append(uncachedContents, content)
			}
		}
	} else {
		for i := range req.Contents {
			uncachedIndices = append(uncachedIndices, i)
		}
		uncachedContents = req.Contents
	}

	// Generate embeddings for uncached content
	if len(uncachedContents) > 0 && p.provider != nil {
		// Rate limiting
		p.waitForRateLimit()

		var vectors [][]float32
		var err error

		// Retry logic
		for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
			vectors, err = p.provider.GenerateEmbeddings(ctx, uncachedContents, req.ModelID)
			if err == nil {
				break
			}

			if attempt < p.config.MaxRetries {
				atomic.AddInt64(&p.retries, 1)
				select {
				case <-ctx.Done():
					return nil, ErrBatchCancelled
				case <-time.After(p.config.RetryDelay * time.Duration(attempt+1)):
				}
			}
		}

		if err != nil {
			atomic.AddInt64(&p.errors, 1)
			for _, idx := range uncachedIndices {
				result.Errors[idx] = err
			}
			return result, nil
		}

		atomic.AddInt64(&p.apiCalls, 1)
		result.APICalls++

		// Store embeddings
		for i, idx := range uncachedIndices {
			if i >= len(vectors) {
				continue
			}

			contentHash := ""
			if p.dedup != nil {
				contentHash = p.dedup.HashContent(uncachedContents[i], req.ModelID)
			}

			emb := &Embedding{
				ID:           generateID(),
				ContentHash:  contentHash,
				Vector:       vectors[i],
				Dimension:    len(vectors[i]),
				ModelID:      req.ModelID,
				ModelVersion: req.ModelVersion,
				Content:      uncachedContents[i],
				CreatedAt:    time.Now(),
			}

			// Store in cache
			if p.store != nil {
				_ = p.store.Put(ctx, emb)
			}

			result.Embeddings[idx] = emb
		}
	}

	result.Duration = time.Since(start)

	atomic.AddInt64(&p.requestsProcessed, 1)
	atomic.AddInt64(&p.itemsProcessed, int64(len(req.Contents)))

	return result, nil
}

// waitForRateLimit waits to satisfy rate limiting.
func (p *BatchProcessor) waitForRateLimit() {
	if p.config.RequestsPerSecond <= 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(p.lastRequest).Seconds()
	p.tokenBucket += elapsed * p.config.RequestsPerSecond

	if p.tokenBucket > p.config.RequestsPerSecond {
		p.tokenBucket = p.config.RequestsPerSecond
	}

	if p.tokenBucket < 1 {
		waitTime := time.Duration((1-p.tokenBucket)/p.config.RequestsPerSecond*1000) * time.Millisecond
		p.mu.Unlock()
		time.Sleep(waitTime)
		p.mu.Lock()
		p.tokenBucket = 0
	} else {
		p.tokenBucket--
	}

	p.lastRequest = time.Now()
}

// QueueLength returns the current queue length.
func (p *BatchProcessor) QueueLength() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.queue)
}

// Stats returns batch processor statistics.
func (p *BatchProcessor) Stats() map[string]interface{} {
	p.mu.RLock()
	queueLen := len(p.queue)
	p.mu.RUnlock()

	return map[string]interface{}{
		"requests_processed": atomic.LoadInt64(&p.requestsProcessed),
		"items_processed":    atomic.LoadInt64(&p.itemsProcessed),
		"cache_hits":         atomic.LoadInt64(&p.cacheHits),
		"api_calls":          atomic.LoadInt64(&p.apiCalls),
		"errors":             atomic.LoadInt64(&p.errors),
		"retries":            atomic.LoadInt64(&p.retries),
		"queue_length":       queueLen,
		"max_batch_size":     p.config.MaxBatchSize,
		"max_concurrency":    p.config.MaxConcurrency,
	}
}

// generateID generates a unique ID.
func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomHex(8)
}

// randomHex generates a random hex string.
func randomHex(n int) string {
	b := make([]byte, (n+1)/2)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}

// MockProvider is a mock embedding provider for testing.
type MockProvider struct {
	dimension int
	delay     time.Duration
	failRate  float64
}

// NewMockProvider creates a new mock provider.
func NewMockProvider(dimension int, delay time.Duration) *MockProvider {
	return &MockProvider{
		dimension: dimension,
		delay:     delay,
	}
}

// GenerateEmbeddings generates mock embeddings.
func (p *MockProvider) GenerateEmbeddings(ctx context.Context, texts []string, modelID string) ([][]float32, error) {
	if p.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(p.delay):
		}
	}

	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = make([]float32, p.dimension)
		// Fill with deterministic values based on text
		for j := 0; j < p.dimension; j++ {
			if j < len(texts[i]) {
				result[i][j] = float32(texts[i][j]) / 255.0
			}
		}
	}

	return result, nil
}

// GetDimension returns the dimension.
func (p *MockProvider) GetDimension(modelID string) (int, error) {
	return p.dimension, nil
}

// Name returns the provider name.
func (p *MockProvider) Name() string {
	return "mock"
}
