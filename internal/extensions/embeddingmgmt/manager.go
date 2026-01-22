package embeddingmgmt

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// EmbeddingModel represents a registered embedding model.
type EmbeddingModel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	Dimensions int    `json:"dimensions"`
	Version    string `json:"version"`
}

// Collection represents a named collection of embeddings.
type Collection struct {
	Name       string          `json:"name"`
	ModelID    string          `json:"model_id"`
	Dimensions int             `json:"dimensions"`
	Count      int             `json:"count"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// Embedding represents a stored embedding vector.
type Embedding struct {
	ID       string            `json:"id"`
	Vector   []float64         `json:"vector"`
	Metadata map[string]string `json:"metadata,omitempty"`
	ModelID  string            `json:"model_id"`
	StoredAt time.Time         `json:"stored_at"`
}

// SearchResult represents a similarity search result.
type SearchResult struct {
	ID       string            `json:"id"`
	Score    float64           `json:"score"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type embeddingStore struct {
	embeddings map[string]*Embedding
}

// ManagerConfig configures the embedding manager.
type ManagerConfig struct {
	MaxCollections int `json:"max_collections"`
	MaxEmbeddings  int `json:"max_embeddings_per_collection"`
	DefaultTopK    int `json:"default_top_k"`
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		MaxCollections: 100,
		MaxEmbeddings:  1000000,
		DefaultTopK:    10,
	}
}

// Manager orchestrates the embedding lifecycle.
type Manager struct {
	mu          sync.RWMutex
	config      ManagerConfig
	models      map[string]*EmbeddingModel
	collections map[string]*Collection
	stores      map[string]*embeddingStore
}

// NewManager creates a new embedding manager.
func NewManager(config ManagerConfig) *Manager {
	if config.MaxCollections == 0 {
		config = DefaultManagerConfig()
	}
	return &Manager{
		config:      config,
		models:      make(map[string]*EmbeddingModel),
		collections: make(map[string]*Collection),
		stores:      make(map[string]*embeddingStore),
	}
}

// RegisterModel registers an embedding model.
func (m *Manager) RegisterModel(model EmbeddingModel) error {
	if model.ID == "" || model.Dimensions <= 0 {
		return fmt.Errorf("model id and positive dimensions required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.models[model.ID] = &model
	return nil
}

// ListModels returns all registered models.
func (m *Manager) ListModels() []EmbeddingModel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]EmbeddingModel, 0, len(m.models))
	for _, model := range m.models {
		result = append(result, *model)
	}
	return result
}

// CreateCollection creates a new embedding collection.
func (m *Manager) CreateCollection(name, modelID string, metadata map[string]string) (*Collection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.collections[name]; exists {
		return nil, ErrCollectionExists
	}

	model, exists := m.models[modelID]
	if !exists {
		return nil, ErrModelNotFound
	}

	if len(m.collections) >= m.config.MaxCollections {
		return nil, fmt.Errorf("max collections reached (%d)", m.config.MaxCollections)
	}

	now := time.Now()
	col := &Collection{
		Name:       name,
		ModelID:    modelID,
		Dimensions: model.Dimensions,
		CreatedAt:  now,
		UpdatedAt:  now,
		Metadata:   metadata,
	}

	m.collections[name] = col
	m.stores[name] = &embeddingStore{
		embeddings: make(map[string]*Embedding),
	}

	return col, nil
}

// GetCollection returns a collection.
func (m *Manager) GetCollection(name string) (*Collection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	col, exists := m.collections[name]
	if !exists {
		return nil, ErrCollectionNotFound
	}
	return col, nil
}

// ListCollections returns all collections.
func (m *Manager) ListCollections() []Collection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Collection, 0, len(m.collections))
	for _, col := range m.collections {
		result = append(result, *col)
	}
	return result
}

// DeleteCollection removes a collection and all its embeddings.
func (m *Manager) DeleteCollection(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.collections[name]; !exists {
		return ErrCollectionNotFound
	}

	delete(m.collections, name)
	delete(m.stores, name)
	return nil
}

// Upsert stores or updates an embedding in a collection.
func (m *Manager) Upsert(collection string, emb Embedding) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	col, exists := m.collections[collection]
	if !exists {
		return ErrCollectionNotFound
	}

	if len(emb.Vector) != col.Dimensions {
		return fmt.Errorf("%w: expected %d, got %d", ErrDimensionMismatch, col.Dimensions, len(emb.Vector))
	}

	store := m.stores[collection]
	if _, exists := store.embeddings[emb.ID]; !exists {
		if len(store.embeddings) >= m.config.MaxEmbeddings {
			return fmt.Errorf("max embeddings reached (%d)", m.config.MaxEmbeddings)
		}
		col.Count++
	}

	emb.StoredAt = time.Now()
	emb.ModelID = col.ModelID
	store.embeddings[emb.ID] = &emb
	col.UpdatedAt = time.Now()

	return nil
}

// Get retrieves an embedding by ID.
func (m *Manager) Get(collection, id string) (*Embedding, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	store, exists := m.stores[collection]
	if !exists {
		return nil, ErrCollectionNotFound
	}

	emb, exists := store.embeddings[id]
	if !exists {
		return nil, fmt.Errorf("embedding %q not found", id)
	}
	return emb, nil
}

// Search performs approximate nearest neighbor search.
func (m *Manager) Search(collection string, query []float64, topK int) ([]SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	col, exists := m.collections[collection]
	if !exists {
		return nil, ErrCollectionNotFound
	}

	if len(query) != col.Dimensions {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrDimensionMismatch, col.Dimensions, len(query))
	}

	if topK <= 0 {
		topK = m.config.DefaultTopK
	}

	store := m.stores[collection]

	// Brute-force cosine similarity (production would use HNSW)
	type scored struct {
		id       string
		score    float64
		metadata map[string]string
	}
	var results []scored

	for _, emb := range store.embeddings {
		score := cosineSimilarity(query, emb.Vector)
		results = append(results, scored{id: emb.ID, score: score, metadata: emb.Metadata})
	}

	// Sort by score descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if len(results) > topK {
		results = results[:topK]
	}

	searchResults := make([]SearchResult, len(results))
	for i, r := range results {
		searchResults[i] = SearchResult{ID: r.id, Score: r.score, Metadata: r.metadata}
	}
	return searchResults, nil
}

// Stats returns aggregate statistics.
func (m *Manager) Stats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var stats ManagerStats
	stats.TotalModels = len(m.models)
	stats.TotalCollections = len(m.collections)
	for _, col := range m.collections {
		stats.TotalEmbeddings += col.Count
	}
	return stats
}

// ManagerStats provides aggregate statistics.
type ManagerStats struct {
	TotalModels      int `json:"total_models"`
	TotalCollections int `json:"total_collections"`
	TotalEmbeddings  int `json:"total_embeddings"`
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
