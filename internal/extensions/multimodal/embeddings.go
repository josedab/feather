package multimodal

import (
	"errors"
	"math"
	"sort"
	"sync"
	"time"
)

var (
	ErrEmbeddingNotFound    = errors.New("embedding not found")
	ErrEmbeddingExists      = errors.New("embedding already exists")
	ErrDimensionMismatch    = errors.New("embedding dimension mismatch")
	ErrMaxEmbeddingsReached = errors.New("maximum embeddings reached")
)

// EmbeddingConfig configures the embedding index.
type EmbeddingConfig struct {
	DefaultDimensions int
	SimilarityMetric  string // "cosine", "euclidean", "dot"
	MaxEmbeddings     int
}

// DefaultEmbeddingConfig returns sensible defaults.
func DefaultEmbeddingConfig() EmbeddingConfig {
	return EmbeddingConfig{
		DefaultDimensions: 768,
		SimilarityMetric:  "cosine",
		MaxEmbeddings:     100000,
	}
}

// EmbeddingEntry stores an embedding vector associated with a blob.
type EmbeddingEntry struct {
	BlobID    string    `json:"blob_id"`
	Vector    []float64 `json:"vector"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
}

// SimilarityResult represents a similarity search result.
type SimilarityResult struct {
	BlobID string          `json:"blob_id"`
	Score  float64         `json:"score"`
	Entry  *EmbeddingEntry `json:"entry"`
}

// EmbeddingStats holds statistics about the embedding index.
type EmbeddingStats struct {
	TotalEmbeddings int            `json:"total_embeddings"`
	Dimensions      int            `json:"dimensions"`
	Models          map[string]int `json:"models"`
}

// EmbeddingIndex manages embeddings for multi-modal content.
type EmbeddingIndex struct {
	mu         sync.RWMutex
	config     EmbeddingConfig
	embeddings map[string]*EmbeddingEntry // blobID -> entry
}

// NewEmbeddingIndex creates a new embedding index.
func NewEmbeddingIndex(config EmbeddingConfig) *EmbeddingIndex {
	return &EmbeddingIndex{
		config:     config,
		embeddings: make(map[string]*EmbeddingEntry),
	}
}

// Add adds an embedding for a blob.
func (idx *EmbeddingIndex) Add(blobID string, vector []float64, model string) error {
	if len(vector) != idx.config.DefaultDimensions {
		return ErrDimensionMismatch
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	if len(idx.embeddings) >= idx.config.MaxEmbeddings {
		return ErrMaxEmbeddingsReached
	}

	idx.embeddings[blobID] = &EmbeddingEntry{
		BlobID:    blobID,
		Vector:    vector,
		Model:     model,
		CreatedAt: time.Now(),
	}
	return nil
}

// Get retrieves an embedding by blob ID.
func (idx *EmbeddingIndex) Get(blobID string) (*EmbeddingEntry, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	entry, ok := idx.embeddings[blobID]
	if !ok {
		return nil, ErrEmbeddingNotFound
	}
	return entry, nil
}

// Remove removes an embedding.
func (idx *EmbeddingIndex) Remove(blobID string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, ok := idx.embeddings[blobID]; !ok {
		return ErrEmbeddingNotFound
	}
	delete(idx.embeddings, blobID)
	return nil
}

// Search performs brute-force similarity search returning top-K results.
func (idx *EmbeddingIndex) Search(query []float64, topK int) []*SimilarityResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	results := make([]*SimilarityResult, 0, len(idx.embeddings))
	for _, entry := range idx.embeddings {
		score := cosineSimilarity(query, entry.Vector)
		results = append(results, &SimilarityResult{
			BlobID: entry.BlobID,
			Score:  score,
			Entry:  entry,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > 0 && topK < len(results) {
		results = results[:topK]
	}
	return results
}

// Stats returns embedding index statistics.
func (idx *EmbeddingIndex) Stats() *EmbeddingStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	models := make(map[string]int)
	for _, e := range idx.embeddings {
		models[e.Model]++
	}

	return &EmbeddingStats{
		TotalEmbeddings: len(idx.embeddings),
		Dimensions:      idx.config.DefaultDimensions,
		Models:          models,
	}
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
