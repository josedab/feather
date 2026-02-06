package embedding

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Errors returned by the embedding store.
var (
	ErrEmbeddingNotFound = errors.New("embedding not found")
	ErrEmbeddingExists   = errors.New("embedding already exists")
	ErrStoreClosed       = errors.New("embedding store is closed")
	ErrInvalidEmbedding  = errors.New("invalid embedding")
	ErrDimensionMismatch = errors.New("embedding dimension mismatch")
	ErrCapacityExceeded  = errors.New("store capacity exceeded")
)

// Embedding represents a cached embedding vector.
type Embedding struct {
	// ID is the unique embedding identifier.
	ID string `json:"id"`

	// ContentHash is the hash of the original content.
	ContentHash string `json:"content_hash"`

	// Vector is the embedding vector.
	Vector []float32 `json:"vector"`

	// Dimension is the vector dimension.
	Dimension int `json:"dimension"`

	// ModelID is the embedding model identifier.
	ModelID string `json:"model_id"`

	// ModelVersion is the model version.
	ModelVersion string `json:"model_version"`

	// Content is the original text (optional, for debugging).
	Content string `json:"content,omitempty"`

	// Metadata contains additional information.
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// CreatedAt is when the embedding was created.
	CreatedAt time.Time `json:"created_at"`

	// LastAccessedAt is when the embedding was last accessed.
	LastAccessedAt time.Time `json:"last_accessed_at"`

	// AccessCount is the number of times accessed.
	AccessCount int64 `json:"access_count"`

	// ExpiresAt is when the embedding expires (zero means no expiration).
	ExpiresAt time.Time `json:"expires_at,omitempty"`

	// ByteSize is the estimated memory size.
	ByteSize int64 `json:"byte_size"`
}

// StoreConfig configures the embedding store.
type StoreConfig struct {
	// MaxCapacity is the maximum number of embeddings.
	MaxCapacity int `json:"max_capacity" yaml:"max_capacity"`

	// MaxMemoryBytes is the maximum memory usage.
	MaxMemoryBytes int64 `json:"max_memory_bytes" yaml:"max_memory_bytes"`

	// DefaultTTL is the default time-to-live for embeddings.
	DefaultTTL time.Duration `json:"default_ttl" yaml:"default_ttl"`

	// EvictionPolicy is the eviction policy (lru, lfu, ttl).
	EvictionPolicy string `json:"eviction_policy" yaml:"eviction_policy"`

	// CleanupInterval is how often to run cleanup.
	CleanupInterval time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"`

	// PersistPath is the path for persistent storage (optional).
	PersistPath string `json:"persist_path" yaml:"persist_path"`
}

// DefaultStoreConfig returns the default store configuration.
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		MaxCapacity:     100000,
		MaxMemoryBytes:  4 * 1024 * 1024 * 1024, // 4GB
		DefaultTTL:      24 * time.Hour,
		EvictionPolicy:  "lru",
		CleanupInterval: 5 * time.Minute,
	}
}

// Store is the embedding cache store.
type Store struct {
	mu         sync.RWMutex
	config     StoreConfig
	embeddings map[string]*Embedding
	byHash     map[string]string   // contentHash -> embeddingID
	byModel    map[string][]string // modelID -> []embeddingID
	accessList []*accessEntry      // for LRU eviction

	// Metrics
	totalEmbeddings int64
	totalBytes      int64
	cacheHits       int64
	cacheMisses     int64
	evictions       int64

	// State
	closed    bool
	closeCh   chan struct{}
	cleanupWg sync.WaitGroup
}

type accessEntry struct {
	id         string
	accessTime time.Time
}

// NewStore creates a new embedding store.
func NewStore(config StoreConfig) *Store {
	s := &Store{
		config:     config,
		embeddings: make(map[string]*Embedding),
		byHash:     make(map[string]string),
		byModel:    make(map[string][]string),
		accessList: make([]*accessEntry, 0),
		closeCh:    make(chan struct{}),
	}

	// Start cleanup goroutine
	if config.CleanupInterval > 0 {
		s.cleanupWg.Add(1)
		go s.cleanupLoop()
	}

	return s
}

// cleanupLoop periodically removes expired embeddings.
func (s *Store) cleanupLoop() {
	defer s.cleanupWg.Done()

	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.closeCh:
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

// cleanup removes expired embeddings.
func (s *Store) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var toDelete []string

	for id, emb := range s.embeddings {
		if !emb.ExpiresAt.IsZero() && now.After(emb.ExpiresAt) {
			toDelete = append(toDelete, id)
		}
	}

	for _, id := range toDelete {
		s.deleteEmbedding(id)
		atomic.AddInt64(&s.evictions, 1)
	}
}

// Put stores an embedding.
func (s *Store) Put(ctx context.Context, emb *Embedding) error {
	if s.closed {
		return ErrStoreClosed
	}

	if emb == nil || len(emb.Vector) == 0 {
		return ErrInvalidEmbedding
	}

	// Calculate byte size
	emb.ByteSize = int64(len(emb.Vector)*4 + len(emb.Content) + len(emb.ID) + len(emb.ContentHash))

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check capacity
	if len(s.embeddings) >= s.config.MaxCapacity {
		if err := s.evict(); err != nil {
			return err
		}
	}

	// Check memory
	if s.totalBytes+emb.ByteSize > s.config.MaxMemoryBytes {
		if err := s.evictByMemory(emb.ByteSize); err != nil {
			return err
		}
	}

	// Set defaults
	now := time.Now()
	if emb.CreatedAt.IsZero() {
		emb.CreatedAt = now
	}
	emb.LastAccessedAt = now
	emb.Dimension = len(emb.Vector)

	// Set TTL
	if emb.ExpiresAt.IsZero() && s.config.DefaultTTL > 0 {
		emb.ExpiresAt = now.Add(s.config.DefaultTTL)
	}

	// Store embedding
	s.embeddings[emb.ID] = emb
	atomic.AddInt64(&s.totalEmbeddings, 1)
	atomic.AddInt64(&s.totalBytes, emb.ByteSize)

	// Index by content hash
	if emb.ContentHash != "" {
		s.byHash[emb.ContentHash] = emb.ID
	}

	// Index by model
	s.byModel[emb.ModelID] = append(s.byModel[emb.ModelID], emb.ID)

	// Add to access list
	s.accessList = append(s.accessList, &accessEntry{
		id:         emb.ID,
		accessTime: now,
	})

	return nil
}

// Get retrieves an embedding by ID.
func (s *Store) Get(ctx context.Context, id string) (*Embedding, error) {
	if s.closed {
		return nil, ErrStoreClosed
	}

	s.mu.RLock()
	emb, ok := s.embeddings[id]
	s.mu.RUnlock()

	if !ok {
		atomic.AddInt64(&s.cacheMisses, 1)
		return nil, ErrEmbeddingNotFound
	}

	// Check expiration
	if !emb.ExpiresAt.IsZero() && time.Now().After(emb.ExpiresAt) {
		atomic.AddInt64(&s.cacheMisses, 1)
		return nil, ErrEmbeddingNotFound
	}

	atomic.AddInt64(&s.cacheHits, 1)

	// Update access time
	s.mu.Lock()
	emb.LastAccessedAt = time.Now()
	emb.AccessCount++
	s.mu.Unlock()

	return emb, nil
}

// GetByHash retrieves an embedding by content hash.
func (s *Store) GetByHash(ctx context.Context, contentHash string) (*Embedding, error) {
	s.mu.RLock()
	id, ok := s.byHash[contentHash]
	s.mu.RUnlock()

	if !ok {
		atomic.AddInt64(&s.cacheMisses, 1)
		return nil, ErrEmbeddingNotFound
	}

	return s.Get(ctx, id)
}

// GetByModel retrieves all embeddings for a model.
func (s *Store) GetByModel(ctx context.Context, modelID string) ([]*Embedding, error) {
	if s.closed {
		return nil, ErrStoreClosed
	}

	s.mu.RLock()
	ids := s.byModel[modelID]
	s.mu.RUnlock()

	embeddings := make([]*Embedding, 0, len(ids))
	for _, id := range ids {
		emb, err := s.Get(ctx, id)
		if err == nil {
			embeddings = append(embeddings, emb)
		}
	}

	return embeddings, nil
}

// Delete removes an embedding by ID.
func (s *Store) Delete(ctx context.Context, id string) error {
	if s.closed {
		return ErrStoreClosed
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.embeddings[id]; !ok {
		return ErrEmbeddingNotFound
	}

	s.deleteEmbedding(id)
	return nil
}

// deleteEmbedding removes an embedding (must hold lock).
func (s *Store) deleteEmbedding(id string) {
	emb, ok := s.embeddings[id]
	if !ok {
		return
	}

	// Remove from hash index
	if emb.ContentHash != "" {
		delete(s.byHash, emb.ContentHash)
	}

	// Remove from model index
	if ids, ok := s.byModel[emb.ModelID]; ok {
		for i, eid := range ids {
			if eid == id {
				s.byModel[emb.ModelID] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}

	// Update metrics
	atomic.AddInt64(&s.totalEmbeddings, -1)
	atomic.AddInt64(&s.totalBytes, -emb.ByteSize)

	delete(s.embeddings, id)
}

// evict removes embeddings based on eviction policy.
func (s *Store) evict() error {
	switch s.config.EvictionPolicy {
	case "lru":
		return s.evictLRU(1)
	case "lfu":
		return s.evictLFU(1)
	case "ttl":
		return s.evictTTL(1)
	default:
		return s.evictLRU(1)
	}
}

// evictByMemory evicts until enough memory is free.
func (s *Store) evictByMemory(needed int64) error {
	for s.totalBytes+needed > s.config.MaxMemoryBytes && len(s.embeddings) > 0 {
		if err := s.evict(); err != nil {
			return err
		}
	}
	return nil
}

// evictLRU removes the least recently used embeddings.
func (s *Store) evictLRU(count int) error {
	// Find oldest accessed embeddings
	var oldest *Embedding
	for _, emb := range s.embeddings {
		if oldest == nil || emb.LastAccessedAt.Before(oldest.LastAccessedAt) {
			oldest = emb
		}
	}

	if oldest != nil {
		s.deleteEmbedding(oldest.ID)
		atomic.AddInt64(&s.evictions, 1)
	}

	return nil
}

// evictLFU removes the least frequently used embeddings.
func (s *Store) evictLFU(count int) error {
	var lfu *Embedding
	for _, emb := range s.embeddings {
		if lfu == nil || emb.AccessCount < lfu.AccessCount {
			lfu = emb
		}
	}

	if lfu != nil {
		s.deleteEmbedding(lfu.ID)
		atomic.AddInt64(&s.evictions, 1)
	}

	return nil
}

// evictTTL removes embeddings closest to expiration.
func (s *Store) evictTTL(count int) error {
	now := time.Now()
	var closest *Embedding

	for _, emb := range s.embeddings {
		if emb.ExpiresAt.IsZero() {
			continue
		}
		remaining := emb.ExpiresAt.Sub(now)
		if closest == nil || remaining < closest.ExpiresAt.Sub(now) {
			closest = emb
		}
	}

	if closest != nil {
		s.deleteEmbedding(closest.ID)
		atomic.AddInt64(&s.evictions, 1)
	}

	return nil
}

// Clear removes all embeddings.
func (s *Store) Clear(ctx context.Context) error {
	if s.closed {
		return ErrStoreClosed
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.embeddings = make(map[string]*Embedding)
	s.byHash = make(map[string]string)
	s.byModel = make(map[string][]string)
	s.accessList = make([]*accessEntry, 0)

	atomic.StoreInt64(&s.totalEmbeddings, 0)
	atomic.StoreInt64(&s.totalBytes, 0)

	return nil
}

// Count returns the number of embeddings.
func (s *Store) Count() int64 {
	return atomic.LoadInt64(&s.totalEmbeddings)
}

// Stats returns store statistics.
func (s *Store) Stats() map[string]interface{} {
	s.mu.RLock()
	modelCount := len(s.byModel)
	s.mu.RUnlock()

	return map[string]interface{}{
		"total_embeddings": atomic.LoadInt64(&s.totalEmbeddings),
		"total_bytes":      atomic.LoadInt64(&s.totalBytes),
		"cache_hits":       atomic.LoadInt64(&s.cacheHits),
		"cache_misses":     atomic.LoadInt64(&s.cacheMisses),
		"evictions":        atomic.LoadInt64(&s.evictions),
		"model_count":      modelCount,
		"max_capacity":     s.config.MaxCapacity,
		"max_memory_bytes": s.config.MaxMemoryBytes,
		"eviction_policy":  s.config.EvictionPolicy,
	}
}

// Close closes the store.
func (s *Store) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	close(s.closeCh)
	s.mu.Unlock()

	s.cleanupWg.Wait()
	return nil
}

// List returns all embeddings with optional filtering.
func (s *Store) List(ctx context.Context, modelID string, limit, offset int) ([]*Embedding, error) {
	if s.closed {
		return nil, ErrStoreClosed
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Embedding, 0, len(s.embeddings))
	count := 0
	skipped := 0

	for _, emb := range s.embeddings {
		// Filter by model
		if modelID != "" && emb.ModelID != modelID {
			continue
		}

		// Skip expired
		if !emb.ExpiresAt.IsZero() && time.Now().After(emb.ExpiresAt) {
			continue
		}

		// Apply offset
		if skipped < offset {
			skipped++
			continue
		}

		result = append(result, emb)
		count++

		// Apply limit
		if limit > 0 && count >= limit {
			break
		}
	}

	return result, nil
}

// Touch updates the last accessed time for an embedding.
func (s *Store) Touch(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	emb, ok := s.embeddings[id]
	if !ok {
		return ErrEmbeddingNotFound
	}

	emb.LastAccessedAt = time.Now()
	emb.AccessCount++

	return nil
}

// Exists checks if an embedding exists.
func (s *Store) Exists(ctx context.Context, id string) bool {
	s.mu.RLock()
	_, ok := s.embeddings[id]
	s.mu.RUnlock()
	return ok
}

// ExistsByHash checks if an embedding exists by content hash.
func (s *Store) ExistsByHash(ctx context.Context, contentHash string) bool {
	s.mu.RLock()
	_, ok := s.byHash[contentHash]
	s.mu.RUnlock()
	return ok
}
