package rag

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
)

// Indexer errors.
var (
	ErrDimensionMismatch = errors.New("vector dimension mismatch")
	ErrEmptyVector       = errors.New("empty vector")
)

// SearchResult represents a single vector search hit.
type SearchResult struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
}

// Indexer is an in-memory vector index supporting cosine similarity search.
type Indexer struct {
	vectors   map[string][]float32
	dimension int
	metric    string
	mu        sync.RWMutex
}

// NewIndexer creates a new vector indexer with the given dimension and metric.
func NewIndexer(dimension int, metric string) *Indexer {
	if metric == "" {
		metric = "cosine"
	}
	return &Indexer{
		vectors:   make(map[string][]float32),
		dimension: dimension,
		metric:    metric,
	}
}

// Add inserts or updates a vector in the index.
func (idx *Indexer) Add(id string, vector []float32) error {
	if len(vector) == 0 {
		return ErrEmptyVector
	}
	if len(vector) != idx.dimension {
		return fmt.Errorf("expected dimension %d, got %d: %w", idx.dimension, len(vector), ErrDimensionMismatch)
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Store a copy to avoid external mutation.
	v := make([]float32, len(vector))
	copy(v, vector)
	idx.vectors[id] = v
	return nil
}

// Delete removes a vector from the index.
func (idx *Indexer) Delete(id string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	delete(idx.vectors, id)
	return nil
}

// Search finds the top-K most similar vectors to the query.
func (idx *Indexer) Search(query []float32, topK int) []*SearchResult {
	if len(query) != idx.dimension || topK <= 0 {
		return nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	results := make([]*SearchResult, 0, len(idx.vectors))
	for id, vec := range idx.vectors {
		score := cosineSimilarity(query, vec)
		results = append(results, &SearchResult{
			ID:    id,
			Score: score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > len(results) {
		topK = len(results)
	}
	return results[:topK]
}

// Count returns the number of indexed vectors.
func (idx *Indexer) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.vectors)
}

// cosineSimilarity computes the cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
