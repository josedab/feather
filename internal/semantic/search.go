// Package semantic provides semantic feature search using embeddings.
// It allows natural language queries to find features by description similarity.
package semantic

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// Search provides semantic search over features.
type Search struct {
	mu          sync.RWMutex
	features    map[string]*FeatureDocument
	embeddings  map[string][]float32
	embedder    Embedder
	dimension   int
	logger      *slog.Logger
}

// FeatureDocument represents a feature with metadata for search.
type FeatureDocument struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Tags        []string          `json:"tags"`
	Category    string            `json:"category"`
	DataType    string            `json:"data_type"`
	Owner       string            `json:"owner"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Embedder generates embeddings for text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
}

// SearchResult represents a search result.
type SearchResult struct {
	Feature    *FeatureDocument `json:"feature"`
	Score      float32          `json:"score"`
	Similarity float32          `json:"similarity"`
}

// SearchOptions configures search behavior.
type SearchOptions struct {
	Limit       int      `json:"limit"`
	MinScore    float32  `json:"min_score"`
	Categories  []string `json:"categories,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Owner       string   `json:"owner,omitempty"`
	IncludeMetadata bool `json:"include_metadata"`
}

// DefaultSearchOptions returns default search options.
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		Limit:    10,
		MinScore: 0.5,
	}
}

// NewSearch creates a new semantic search instance.
func NewSearch(embedder Embedder, logger *slog.Logger) *Search {
	if logger == nil {
		logger = slog.Default()
	}

	dimension := 384 // Default dimension for small models
	if embedder != nil {
		dimension = embedder.Dimension()
	}

	return &Search{
		features:   make(map[string]*FeatureDocument),
		embeddings: make(map[string][]float32),
		embedder:   embedder,
		dimension:  dimension,
		logger:     logger,
	}
}

// IndexFeature adds or updates a feature in the search index.
func (s *Search) IndexFeature(ctx context.Context, feature *FeatureDocument) error {
	if feature.ID == "" {
		return fmt.Errorf("feature ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate searchable text
	searchText := s.buildSearchText(feature)

	// Generate embedding
	var embedding []float32
	var err error

	if s.embedder != nil {
		embedding, err = s.embedder.Embed(ctx, searchText)
		if err != nil {
			return fmt.Errorf("embedding generation failed: %w", err)
		}
	} else {
		// Use TF-IDF style embedding for fallback
		embedding = s.generateTFIDFEmbedding(searchText)
	}

	now := time.Now()
	if feature.CreatedAt.IsZero() {
		feature.CreatedAt = now
	}
	feature.UpdatedAt = now

	s.features[feature.ID] = feature
	s.embeddings[feature.ID] = embedding

	s.logger.Debug("Indexed feature", "id", feature.ID, "name", feature.Name)

	return nil
}

// IndexBatch indexes multiple features efficiently.
func (s *Search) IndexBatch(ctx context.Context, features []*FeatureDocument) error {
	if len(features) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Build search texts
	texts := make([]string, len(features))
	for i, f := range features {
		texts[i] = s.buildSearchText(f)
	}

	// Generate embeddings
	var embeddings [][]float32
	var err error

	if s.embedder != nil {
		embeddings, err = s.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			return fmt.Errorf("batch embedding failed: %w", err)
		}
	} else {
		embeddings = make([][]float32, len(texts))
		for i, text := range texts {
			embeddings[i] = s.generateTFIDFEmbedding(text)
		}
	}

	// Store
	now := time.Now()
	for i, f := range features {
		if f.ID == "" {
			continue
		}
		if f.CreatedAt.IsZero() {
			f.CreatedAt = now
		}
		f.UpdatedAt = now
		s.features[f.ID] = f
		s.embeddings[f.ID] = embeddings[i]
	}

	s.logger.Info("Indexed batch", "count", len(features))

	return nil
}

// Search performs semantic search over indexed features.
func (s *Search) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 10
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Generate query embedding
	var queryEmbedding []float32
	var err error

	if s.embedder != nil {
		queryEmbedding, err = s.embedder.Embed(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("query embedding failed: %w", err)
		}
	} else {
		queryEmbedding = s.generateTFIDFEmbedding(query)
	}

	// Calculate similarities
	type scoredFeature struct {
		feature *FeatureDocument
		score   float32
	}

	var scored []scoredFeature

	for id, embedding := range s.embeddings {
		feature := s.features[id]

		// Apply filters
		if !s.matchesFilters(feature, opts) {
			continue
		}

		// Calculate cosine similarity
		score := cosineSimilarity(queryEmbedding, embedding)

		if score >= opts.MinScore {
			scored = append(scored, scoredFeature{
				feature: feature,
				score:   score,
			})
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Limit results
	if len(scored) > opts.Limit {
		scored = scored[:opts.Limit]
	}

	// Build results
	results := make([]SearchResult, len(scored))
	for i, sf := range scored {
		results[i] = SearchResult{
			Feature:    sf.feature,
			Score:      sf.score,
			Similarity: sf.score * 100, // Percentage
		}
	}

	return results, nil
}

// Suggest returns similar features to a given feature.
func (s *Search) Suggest(ctx context.Context, featureID string, limit int) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sourceEmbedding, ok := s.embeddings[featureID]
	if !ok {
		return nil, fmt.Errorf("feature not found: %s", featureID)
	}

	if limit <= 0 {
		limit = 5
	}

	type scoredFeature struct {
		feature *FeatureDocument
		score   float32
	}

	var scored []scoredFeature

	for id, embedding := range s.embeddings {
		if id == featureID {
			continue // Skip self
		}

		score := cosineSimilarity(sourceEmbedding, embedding)
		scored = append(scored, scoredFeature{
			feature: s.features[id],
			score:   score,
		})
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Limit results
	if len(scored) > limit {
		scored = scored[:limit]
	}

	// Build results
	results := make([]SearchResult, len(scored))
	for i, sf := range scored {
		results[i] = SearchResult{
			Feature:    sf.feature,
			Score:      sf.score,
			Similarity: sf.score * 100,
		}
	}

	return results, nil
}

// GetFeature returns a feature by ID.
func (s *Search) GetFeature(featureID string) (*FeatureDocument, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	feature, ok := s.features[featureID]
	if !ok {
		return nil, fmt.Errorf("feature not found: %s", featureID)
	}

	return feature, nil
}

// ListFeatures returns all indexed features.
func (s *Search) ListFeatures() []*FeatureDocument {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*FeatureDocument, 0, len(s.features))
	for _, f := range s.features {
		result = append(result, f)
	}
	return result
}

// DeleteFeature removes a feature from the index.
func (s *Search) DeleteFeature(featureID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.features[featureID]; !ok {
		return fmt.Errorf("feature not found: %s", featureID)
	}

	delete(s.features, featureID)
	delete(s.embeddings, featureID)

	return nil
}

// GetStats returns search index statistics.
func (s *Search) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	categories := make(map[string]int)
	tags := make(map[string]int)

	for _, f := range s.features {
		categories[f.Category]++
		for _, tag := range f.Tags {
			tags[tag]++
		}
	}

	return map[string]interface{}{
		"total_features": len(s.features),
		"dimension":      s.dimension,
		"categories":     categories,
		"tags":           tags,
		"has_embedder":   s.embedder != nil,
	}
}

func (s *Search) buildSearchText(feature *FeatureDocument) string {
	var parts []string

	if feature.Name != "" {
		parts = append(parts, feature.Name)
	}
	if feature.Description != "" {
		parts = append(parts, feature.Description)
	}
	if feature.Category != "" {
		parts = append(parts, feature.Category)
	}
	parts = append(parts, feature.Tags...)

	return strings.Join(parts, " ")
}

func (s *Search) matchesFilters(feature *FeatureDocument, opts SearchOptions) bool {
	// Filter by categories
	if len(opts.Categories) > 0 {
		found := false
		for _, cat := range opts.Categories {
			if strings.EqualFold(feature.Category, cat) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by tags
	if len(opts.Tags) > 0 {
		found := false
		for _, optTag := range opts.Tags {
			for _, featureTag := range feature.Tags {
				if strings.EqualFold(featureTag, optTag) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by owner
	if opts.Owner != "" && !strings.EqualFold(feature.Owner, opts.Owner) {
		return false
	}

	return true
}

// generateTFIDFEmbedding creates a simple TF-IDF style embedding as fallback.
func (s *Search) generateTFIDFEmbedding(text string) []float32 {
	// Simple bag of words with hash-based dimensionality reduction
	words := tokenize(text)
	embedding := make([]float32, s.dimension)

	for _, word := range words {
		hash := simpleHash(word)
		idx := hash % uint32(s.dimension)
		embedding[idx] += 1.0
	}

	// Normalize
	normalize(embedding)

	return embedding
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})

	// Remove common stop words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true,
		"to": true, "of": true, "in": true, "for": true, "on": true,
		"with": true, "at": true, "by": true, "from": true, "as": true,
		"it": true, "this": true, "that": true, "these": true, "those": true,
		"and": true, "or": true, "but": true, "if": true, "then": true,
	}

	var filtered []string
	for _, w := range words {
		if len(w) > 1 && !stopWords[w] {
			filtered = append(filtered, w)
		}
	}

	return filtered
}

func simpleHash(s string) uint32 {
	var hash uint32 = 5381
	for _, c := range s {
		hash = ((hash << 5) + hash) + uint32(c)
	}
	return hash
}

func normalize(vec []float32) {
	var sum float32
	for _, v := range vec {
		sum += v * v
	}
	if sum > 0 {
		norm := float32(math.Sqrt(float64(sum)))
		for i := range vec {
			vec[i] /= norm
		}
	}
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
