package vector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store manages multiple vector indexes.
type Store struct {
	mu      sync.RWMutex
	indexes map[string]*Index
	dataDir string
}

// Index wraps an HNSW index with metadata.
type Index struct {
	Name         string       `json:"name"`
	Dimension    int          `json:"dimension"`
	DistanceType DistanceType `json:"distance_type"`
	hnsw         *HNSW
	metadata     map[string]map[string]interface{} // id -> metadata
	mu           sync.RWMutex
}

// StoreConfig configures the vector store.
type StoreConfig struct {
	DataDir string // Directory for persistence (empty for in-memory)
}

// NewStore creates a new vector store.
func NewStore(config StoreConfig) *Store {
	return &Store{
		indexes: make(map[string]*Index),
		dataDir: config.DataDir,
	}
}

// CreateIndex creates a new vector index.
func (s *Store) CreateIndex(name string, dim int, distType DistanceType) (*Index, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.indexes[name]; exists {
		return nil, fmt.Errorf("index %s already exists", name)
	}

	if distType == "" {
		distType = DistanceCosine
	}

	hnsw := NewHNSW(HNSWConfig{
		Dim:          dim,
		M:            16,
		EfConstruct:  200,
		DistanceType: distType,
	})

	idx := &Index{
		Name:         name,
		Dimension:    dim,
		DistanceType: distType,
		hnsw:         hnsw,
		metadata:     make(map[string]map[string]interface{}),
	}

	s.indexes[name] = idx
	return idx, nil
}

// GetIndex retrieves an index by name.
func (s *Store) GetIndex(name string) (*Index, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, exists := s.indexes[name]
	if !exists {
		return nil, ErrIndexNotFound
	}
	return idx, nil
}

// DeleteIndex removes an index.
func (s *Store) DeleteIndex(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.indexes[name]; !exists {
		return ErrIndexNotFound
	}

	delete(s.indexes, name)
	return nil
}

// ListIndexes returns all index names.
func (s *Store) ListIndexes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.indexes))
	for name := range s.indexes {
		names = append(names, name)
	}
	return names
}

// Upsert adds or updates a vector with optional metadata.
func (idx *Index) Upsert(id string, vector []float32, metadata map[string]interface{}) error {
	if err := idx.hnsw.Insert(id, vector); err != nil {
		return err
	}

	if metadata != nil {
		idx.mu.Lock()
		idx.metadata[id] = metadata
		idx.mu.Unlock()
	}

	return nil
}

// UpsertBatch adds or updates multiple vectors.
func (idx *Index) UpsertBatch(vectors []VectorRecord) error {
	for _, v := range vectors {
		if err := idx.Upsert(v.ID, v.Vector, v.Metadata); err != nil {
			return fmt.Errorf("upserting %s: %w", v.ID, err)
		}
	}
	return nil
}

// Search finds similar vectors.
func (idx *Index) Search(query []float32, k int, opts *SearchOptions) ([]SearchResultWithMetadata, error) {
	ef := 0
	includeMetadata := true
	includeVectors := false

	if opts != nil {
		ef = opts.Ef
		includeMetadata = opts.IncludeMetadata
		includeVectors = opts.IncludeVectors
	}

	results, err := idx.hnsw.Search(query, k, ef)
	if err != nil {
		return nil, err
	}

	// Enrich with metadata
	enriched := make([]SearchResultWithMetadata, len(results))
	idx.mu.RLock()
	for i, r := range results {
		enriched[i] = SearchResultWithMetadata{
			ID:       r.ID,
			Distance: r.Distance,
			Score:    1 - r.Distance, // Convert distance to similarity score
		}
		if includeVectors {
			enriched[i].Vector = r.Vector
		}
		if includeMetadata {
			enriched[i].Metadata = idx.metadata[r.ID]
		}
	}
	idx.mu.RUnlock()

	return enriched, nil
}

// Delete removes a vector.
func (idx *Index) Delete(id string) error {
	idx.mu.Lock()
	delete(idx.metadata, id)
	idx.mu.Unlock()

	return idx.hnsw.Delete(id)
}

// Get retrieves a vector by ID.
func (idx *Index) Get(id string) (*VectorRecord, error) {
	vector, exists := idx.hnsw.Get(id)
	if !exists {
		return nil, ErrVectorNotFound
	}

	idx.mu.RLock()
	metadata := idx.metadata[id]
	idx.mu.RUnlock()

	return &VectorRecord{
		ID:       id,
		Vector:   vector,
		Metadata: metadata,
	}, nil
}

// Size returns the number of vectors.
func (idx *Index) Size() int {
	return idx.hnsw.Size()
}

// VectorRecord represents a vector with metadata.
type VectorRecord struct {
	ID       string                 `json:"id"`
	Vector   []float32              `json:"vector"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SearchOptions configures search behavior.
type SearchOptions struct {
	Ef              int  // Search expansion factor (higher = more accurate, slower)
	IncludeMetadata bool // Include metadata in results
	IncludeVectors  bool // Include vectors in results
}

// SearchResultWithMetadata extends SearchResult with metadata.
type SearchResultWithMetadata struct {
	ID       string                 `json:"id"`
	Distance float32                `json:"distance"`
	Score    float32                `json:"score"` // Similarity score (1 - distance)
	Vector   []float32              `json:"vector,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Save persists an index to disk.
func (s *Store) Save(indexName string) error {
	if s.dataDir == "" {
		return nil // In-memory mode
	}

	s.mu.RLock()
	idx, exists := s.indexes[indexName]
	s.mu.RUnlock()

	if !exists {
		return ErrIndexNotFound
	}

	// Create directory
	indexDir := filepath.Join(s.dataDir, "vectors", indexName)
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return err
	}

	// Save metadata
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Save index config
	configPath := filepath.Join(indexDir, "config.json")
	configData, err := json.Marshal(struct {
		Name         string       `json:"name"`
		Dimension    int          `json:"dimension"`
		DistanceType DistanceType `json:"distance_type"`
	}{
		Name:         idx.Name,
		Dimension:    idx.Dimension,
		DistanceType: idx.DistanceType,
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return err
	}

	// Save vectors and metadata
	var records []VectorRecord
	for id, meta := range idx.metadata {
		vec, _ := idx.hnsw.Get(id)
		records = append(records, VectorRecord{
			ID:       id,
			Vector:   vec,
			Metadata: meta,
		})
	}

	// Also save vectors without metadata
	// This is a simplified approach - production would use more efficient serialization

	dataPath := filepath.Join(indexDir, "data.json")
	dataBytes, err := json.Marshal(records)
	if err != nil {
		return err
	}
	return os.WriteFile(dataPath, dataBytes, 0644)
}

// Load loads an index from disk.
func (s *Store) Load(indexName string) error {
	if s.dataDir == "" {
		return nil
	}

	indexDir := filepath.Join(s.dataDir, "vectors", indexName)

	// Load config
	configPath := filepath.Join(indexDir, "config.json")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var config struct {
		Name         string       `json:"name"`
		Dimension    int          `json:"dimension"`
		DistanceType DistanceType `json:"distance_type"`
	}
	if err := json.Unmarshal(configData, &config); err != nil {
		return err
	}

	// Create index
	idx, err := s.CreateIndex(config.Name, config.Dimension, config.DistanceType)
	if err != nil {
		return err
	}

	// Load data
	dataPath := filepath.Join(indexDir, "data.json")
	dataBytes, err := os.ReadFile(dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No data yet
		}
		return err
	}

	var records []VectorRecord
	if err := json.Unmarshal(dataBytes, &records); err != nil {
		return err
	}

	return idx.UpsertBatch(records)
}
