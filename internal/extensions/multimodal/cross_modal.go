package multimodal

import (
	"fmt"
	"math"
	"sort"
	"sync"
)

// CrossModalResult represents a search result across modalities.
type CrossModalResult struct {
	BlobID   string       `json:"blob_id"`
	Modality ModalityType `json:"modality"`
	Score    float64      `json:"score"`
	Model    string       `json:"model"`
}

// CrossModalSearchConfig configures cross-modal search.
type CrossModalSearchConfig struct {
	MaxResults      int     `json:"max_results" yaml:"max_results"`
	MinScore        float64 `json:"min_score" yaml:"min_score"`
	EnableReranking bool    `json:"enable_reranking" yaml:"enable_reranking"`
}

// DefaultCrossModalSearchConfig returns sensible defaults.
func DefaultCrossModalSearchConfig() CrossModalSearchConfig {
	return CrossModalSearchConfig{
		MaxResults:      10,
		MinScore:        0.0,
		EnableReranking: false,
	}
}

// CrossModalSearch provides unified search across all modalities.
type CrossModalSearch struct {
	mu       sync.RWMutex
	config   CrossModalSearchConfig
	index    *EmbeddingIndex
	store    *MultiModalStore
	pipeline *EmbeddingPipeline
}

// NewCrossModalSearch creates a new cross-modal search engine.
func NewCrossModalSearch(config CrossModalSearchConfig, index *EmbeddingIndex, store *MultiModalStore, pipeline *EmbeddingPipeline) *CrossModalSearch {
	return &CrossModalSearch{
		config:   config,
		index:    index,
		store:    store,
		pipeline: pipeline,
	}
}

// SearchByVector performs cross-modal search using a query vector.
func (s *CrossModalSearch) SearchByVector(query []float64, topK int) ([]CrossModalResult, error) {
	if s.index == nil {
		return nil, fmt.Errorf("embedding index not configured")
	}

	if topK <= 0 {
		topK = s.config.MaxResults
	}

	results := s.index.Search(query, topK)

	var crossResults []CrossModalResult
	for _, r := range results {
		if r.Score < s.config.MinScore {
			continue
		}

		modality := ModalityType("unknown")
		model := ""
		if r.Entry != nil {
			model = r.Entry.Model
		}
		if s.store != nil {
			meta, err := s.store.GetMetadata(r.BlobID)
			if err == nil {
				modality = meta.Modality
			}
		}

		crossResults = append(crossResults, CrossModalResult{
			BlobID:   r.BlobID,
			Modality: modality,
			Score:    r.Score,
			Model:    model,
		})
	}

	sort.Slice(crossResults, func(i, j int) bool {
		return crossResults[i].Score > crossResults[j].Score
	})

	return crossResults, nil
}

// SearchByText performs cross-modal search from text input.
func (s *CrossModalSearch) SearchByText(text string, topK int) ([]CrossModalResult, error) {
	if s.pipeline == nil {
		return nil, fmt.Errorf("embedding pipeline not configured")
	}

	embedding := s.pipeline.generateEmbedding([]byte(text), s.pipeline.config.DefaultDimension)
	return s.SearchByVector(embedding, topK)
}

// SearchByBlob performs cross-modal search from an existing blob.
func (s *CrossModalSearch) SearchByBlob(blobID string, topK int) ([]CrossModalResult, error) {
	if s.index == nil {
		return nil, fmt.Errorf("embedding index not configured")
	}

	entry, err := s.index.Get(blobID)
	if err != nil {
		return nil, fmt.Errorf("getting embedding for blob %s: %w", blobID, err)
	}

	results, err := s.SearchByVector(entry.Vector, topK+1) // +1 to exclude self
	if err != nil {
		return nil, err
	}

	var filtered []CrossModalResult
	for _, r := range results {
		if r.BlobID != blobID {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) > topK {
		filtered = filtered[:topK]
	}

	return filtered, nil
}

// CosineSimilarity computes the cosine similarity between two vectors.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA < 1e-10 || normB < 1e-10 {
		return 0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
