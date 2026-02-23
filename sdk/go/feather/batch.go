package feather

import (
	"context"
	"errors"
	"sync"
	"time"
)

// BatchClient provides batched operations for efficiency.
type BatchClient struct {
	client        *Client
	batchSize     int
	flushInterval time.Duration
	pending       []batchItem
	mu            sync.Mutex
	ctx           context.Context
	cancel        context.CancelFunc
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

type batchItem struct {
	entityID string
	features map[string]interface{}
	done     chan error
}

// NewBatchClient creates a new batch client.
func NewBatchClient(client *Client, batchSize int, flushInterval time.Duration) *BatchClient {
	return NewBatchClientWithContext(context.Background(), client, batchSize, flushInterval)
}

// NewBatchClientWithContext creates a new batch client with a parent context.
func NewBatchClientWithContext(ctx context.Context, client *Client, batchSize int, flushInterval time.Duration) *BatchClient {
	ctx, cancel := context.WithCancel(ctx)
	bc := &BatchClient{
		client:        client,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		pending:       make([]batchItem, 0, batchSize),
		ctx:           ctx,
		cancel:        cancel,
		stopCh:        make(chan struct{}),
	}

	bc.wg.Add(1)
	go bc.flushLoop()

	return bc
}

// Put queues a feature update for batched writing.
func (bc *BatchClient) Put(ctx context.Context, entityID string, features map[string]interface{}) error {
	done := make(chan error, 1)

	bc.mu.Lock()
	bc.pending = append(bc.pending, batchItem{
		entityID: entityID,
		features: features,
		done:     done,
	})

	if len(bc.pending) >= bc.batchSize {
		bc.wg.Add(1)
		go func(ctx context.Context) {
			defer bc.wg.Done()
			bc.flush(ctx)
		}(ctx)
	}
	bc.mu.Unlock()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (bc *BatchClient) flushLoop() {
	defer bc.wg.Done()

	ticker := time.NewTicker(bc.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bc.flush(bc.ctx)
		case <-bc.stopCh:
			bc.flush(bc.ctx) // Final flush
			return
		}
	}
}

func (bc *BatchClient) flush(ctx context.Context) {
	bc.mu.Lock()
	if len(bc.pending) == 0 {
		bc.mu.Unlock()
		return
	}

	items := bc.pending
	bc.pending = make([]batchItem, 0, bc.batchSize)
	bc.mu.Unlock()

	// Group by entity
	byEntity := make(map[string]map[string]interface{})
	itemsByEntity := make(map[string][]batchItem)

	for _, item := range items {
		if byEntity[item.entityID] == nil {
			byEntity[item.entityID] = make(map[string]interface{})
		}
		for k, v := range item.features {
			byEntity[item.entityID][k] = v
		}
		itemsByEntity[item.entityID] = append(itemsByEntity[item.entityID], item)
	}

	// Send batched requests
	for entityID, features := range byEntity {
		if ctx.Err() != nil {
			for _, item := range itemsByEntity[entityID] {
				item.done <- ctx.Err()
			}
			continue
		}
		err := bc.client.Features.Put(ctx, &PutRequest{
			EntityID: entityID,
			Features: features,
		})

		// Notify all items for this entity
		for _, item := range itemsByEntity[entityID] {
			item.done <- err
		}
	}
}

// Close stops the batch client and flushes pending items.
func (bc *BatchClient) Close() {
	bc.CloseWithContext(bc.ctx)
}

// CloseWithContext stops the batch client using the provided context for the final flush.
func (bc *BatchClient) CloseWithContext(ctx context.Context) {
	bc.cancel()
	close(bc.stopCh)
	bc.wg.Wait()
}

// AsyncClient provides asynchronous operations.
type AsyncClient struct {
	client *Client
}

// NewAsyncClient creates a new async client wrapper.
func NewAsyncClient(client *Client) *AsyncClient {
	return &AsyncClient{client: client}
}

// AsyncResult represents the result of an async operation.
type AsyncResult[T any] struct {
	Value T
	Err   error
}

// GetAsync retrieves features asynchronously.
func (ac *AsyncClient) GetAsync(ctx context.Context, entityID string, features []string) <-chan AsyncResult[*GetResponse] {
	ch := make(chan AsyncResult[*GetResponse], 1)

	go func() {
		resp, err := ac.client.Features.Get(ctx, entityID, features)
		ch <- AsyncResult[*GetResponse]{Value: resp, Err: err}
		close(ch)
	}()

	return ch
}

// GetBatchAsync retrieves features for multiple entities asynchronously.
func (ac *AsyncClient) GetBatchAsync(ctx context.Context, entityIDs []string, features []string) <-chan AsyncResult[map[string]*GetResponse] {
	ch := make(chan AsyncResult[map[string]*GetResponse], 1)

	go func() {
		resp, err := ac.client.Features.GetBatch(ctx, entityIDs, features)
		ch <- AsyncResult[map[string]*GetResponse]{Value: resp, Err: err}
		close(ch)
	}()

	return ch
}

// PutAsync stores features asynchronously.
func (ac *AsyncClient) PutAsync(ctx context.Context, req *PutRequest) <-chan error {
	ch := make(chan error, 1)

	go func() {
		ch <- ac.client.Features.Put(ctx, req)
		close(ch)
	}()

	return ch
}

// ParallelGet retrieves features for multiple entities in parallel.
func (ac *AsyncClient) ParallelGet(ctx context.Context, requests []GetRequest) map[string]*GetResponse {
	results := make(map[string]*GetResponse)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, req := range requests {
		wg.Add(1)
		go func(r GetRequest) {
			defer wg.Done()

			resp, err := ac.client.Features.Get(ctx, r.EntityID, r.Features)
			if err == nil {
				mu.Lock()
				results[r.EntityID] = resp
				mu.Unlock()
			}
		}(req)
	}

	wg.Wait()
	return results
}

// RetryConfig configures retry behavior.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts before giving up. Default: 3.
	MaxRetries int
	// InitialBackoff is the delay before the first retry. Default: 100ms.
	InitialBackoff time.Duration
	// MaxBackoff is the upper bound on backoff delay. Default: 10s.
	MaxBackoff time.Duration
	// Multiplier is the exponential backoff factor applied after each retry. Default: 2.0.
	Multiplier float64
}

// DefaultRetryConfig returns default retry configuration.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
	}
}

// WithRetry executes a function with exponential backoff retry.
func WithRetry[T any](ctx context.Context, config *RetryConfig, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	backoff := config.InitialBackoff

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(backoff):
			}

			backoff = time.Duration(float64(backoff) * config.Multiplier)
			if backoff > config.MaxBackoff {
				backoff = config.MaxBackoff
			}
		}

		result, lastErr = fn()
		if lastErr == nil {
			return result, nil
		}

		// Don't retry on non-retryable errors
		var apiErr *APIError
		if errors.As(lastErr, &apiErr) {
			if apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
				return result, lastErr
			}
		}
	}

	return result, lastErr
}

// CacheConfig configures the client-side cache.
type CacheConfig struct {
	// MaxSize is the maximum number of entries in the cache. Default: 1000.
	MaxSize int
	// TTL is how long cached entries remain valid before eviction. Default: 5m.
	TTL time.Duration
	// Enabled controls whether client-side caching is active.
	Enabled bool
}

// CachedClient wraps a client with local caching.
type CachedClient struct {
	client *Client
	cache  map[string]*cacheEntry
	config *CacheConfig
	mu     sync.RWMutex
	stopCh chan struct{}
}

type cacheEntry struct {
	value     *GetResponse
	expiresAt time.Time
}

// NewCachedClient creates a client with local caching.
func NewCachedClient(client *Client, config *CacheConfig) *CachedClient {
	cc := &CachedClient{
		client: client,
		cache:  make(map[string]*cacheEntry),
		config: config,
		stopCh: make(chan struct{}),
	}

	// Start cleanup goroutine
	go cc.cleanup()

	return cc
}

// Get retrieves features with caching.
func (cc *CachedClient) Get(ctx context.Context, entityID string, features []string) (*GetResponse, error) {
	if !cc.config.Enabled {
		return cc.client.Features.Get(ctx, entityID, features)
	}

	cacheKey := cc.cacheKey(entityID, features)

	// Check cache
	cc.mu.RLock()
	entry, ok := cc.cache[cacheKey]
	cc.mu.RUnlock()

	if ok && entry.expiresAt.After(time.Now()) {
		return entry.value, nil
	}

	// Fetch from server
	resp, err := cc.client.Features.Get(ctx, entityID, features)
	if err != nil {
		return nil, err
	}

	// Update cache
	cc.mu.Lock()
	if len(cc.cache) >= cc.config.MaxSize {
		// Simple eviction: remove oldest
		var oldest string
		var oldestTime time.Time
		for k, v := range cc.cache {
			if oldest == "" || v.expiresAt.Before(oldestTime) {
				oldest = k
				oldestTime = v.expiresAt
			}
		}
		delete(cc.cache, oldest)
	}

	cc.cache[cacheKey] = &cacheEntry{
		value:     resp,
		expiresAt: time.Now().Add(cc.config.TTL),
	}
	cc.mu.Unlock()

	return resp, nil
}

func (cc *CachedClient) cacheKey(entityID string, features []string) string {
	key := entityID
	for _, f := range features {
		key += ":" + f
	}
	return key
}

func (cc *CachedClient) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cc.mu.Lock()
			now := time.Now()
			for k, v := range cc.cache {
				if v.expiresAt.Before(now) {
					delete(cc.cache, k)
				}
			}
			cc.mu.Unlock()
		case <-cc.stopCh:
			return
		}
	}
}

// Invalidate invalidates a cache entry.
func (cc *CachedClient) Invalidate(entityID string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	for k := range cc.cache {
		if len(k) >= len(entityID) && k[:len(entityID)] == entityID {
			delete(cc.cache, k)
		}
	}
}

// Close stops the background cleanup goroutine and releases resources.
func (cc *CachedClient) Close() {
	close(cc.stopCh)
}
