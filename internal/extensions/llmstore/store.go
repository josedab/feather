package llmstore

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// PromptTemplate represents a versioned prompt template.
type PromptTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Template    string            `json:"template"`
	Variables   []string          `json:"variables,omitempty"`
	Model       string            `json:"model,omitempty"` // target LLM model
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
	Version     int               `json:"version"`
	Tags        map[string]string `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Embedding represents a stored vector embedding.
type Embedding struct {
	ID        string    `json:"id"`
	Vector    []float64 `json:"vector"`
	Text      string    `json:"text,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Model     string    `json:"model,omitempty"` // embedding model used
	CreatedAt time.Time `json:"created_at"`
}

// SimilarityResult represents a vector similarity search result.
type SimilarityResult struct {
	ID       string    `json:"id"`
	Score    float64   `json:"score"`
	Text     string    `json:"text,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// RAGPipeline defines a Retrieval-Augmented Generation pipeline.
type RAGPipeline struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	PromptTemplateID string  `json:"prompt_template_id"`
	EmbeddingModel  string   `json:"embedding_model"`
	TopK            int      `json:"top_k"`
	MinScore        float64  `json:"min_score"`
	ChunkSize       int      `json:"chunk_size,omitempty"`
	ChunkOverlap    int      `json:"chunk_overlap,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// RAGRequest represents a RAG query request.
type RAGRequest struct {
	PipelineID string            `json:"pipeline_id"`
	Query      string            `json:"query"`
	Variables  map[string]string `json:"variables,omitempty"`
	TopK       int               `json:"top_k,omitempty"`
}

// RAGResponse represents a RAG query response.
type RAGResponse struct {
	Query          string             `json:"query"`
	Context        []SimilarityResult `json:"context"`
	AugmentedPrompt string            `json:"augmented_prompt"`
	PipelineID     string             `json:"pipeline_id"`
	Duration       time.Duration      `json:"duration_ns"`
}

// StoreConfig configures the LLM store.
type StoreConfig struct {
	MaxPrompts      int `json:"max_prompts"`
	MaxEmbeddings   int `json:"max_embeddings"`
	MaxPipelines    int `json:"max_pipelines"`
	DefaultTopK     int `json:"default_top_k"`
}

// DefaultStoreConfig returns sensible defaults.
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		MaxPrompts:    10000,
		MaxEmbeddings: 1000000,
		MaxPipelines:  1000,
		DefaultTopK:   5,
	}
}

// Store provides unified prompt, embedding, and RAG management.
type Store struct {
	mu         sync.RWMutex
	config     StoreConfig
	prompts    map[string]*PromptTemplate
	embeddings map[string]*Embedding
	pipelines  map[string]*RAGPipeline
	stats      StoreStats
}

// StoreStats holds aggregate store statistics.
type StoreStats struct {
	TotalPrompts    int   `json:"total_prompts"`
	TotalEmbeddings int   `json:"total_embeddings"`
	TotalPipelines  int   `json:"total_pipelines"`
	TotalQueries    int64 `json:"total_queries"`
	TotalSearches   int64 `json:"total_searches"`
}

// NewStore creates a new LLM feature store.
func NewStore(config StoreConfig) *Store {
	if config.MaxPrompts == 0 {
		config = DefaultStoreConfig()
	}
	return &Store{
		config:     config,
		prompts:    make(map[string]*PromptTemplate),
		embeddings: make(map[string]*Embedding),
		pipelines:  make(map[string]*RAGPipeline),
	}
}

// CreatePrompt creates a new prompt template.
func (s *Store) CreatePrompt(p PromptTemplate) (*PromptTemplate, error) {
	if p.ID == "" || p.Name == "" {
		return nil, fmt.Errorf("%w: id and name are required", ErrInvalidConfig)
	}
	if p.Template == "" {
		return nil, fmt.Errorf("%w: template is required", ErrInvalidConfig)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.prompts[p.ID]; exists {
		return nil, ErrPromptExists
	}
	if len(s.prompts) >= s.config.MaxPrompts {
		return nil, fmt.Errorf("max prompts reached (%d)", s.config.MaxPrompts)
	}

	now := time.Now()
	p.Version = 1
	p.CreatedAt = now
	p.UpdatedAt = now

	s.prompts[p.ID] = &p
	s.stats.TotalPrompts = len(s.prompts)
	return &p, nil
}

// GetPrompt returns a prompt template by ID.
func (s *Store) GetPrompt(id string) (*PromptTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, exists := s.prompts[id]
	if !exists {
		return nil, ErrPromptNotFound
	}
	return p, nil
}

// ListPrompts returns all prompt templates.
func (s *Store) ListPrompts() []PromptTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]PromptTemplate, 0, len(s.prompts))
	for _, p := range s.prompts {
		result = append(result, *p)
	}
	return result
}

// UpdatePrompt updates a prompt template, creating a new version.
func (s *Store) UpdatePrompt(p PromptTemplate) (*PromptTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.prompts[p.ID]
	if !exists {
		return nil, ErrPromptNotFound
	}

	p.Version = existing.Version + 1
	p.CreatedAt = existing.CreatedAt
	p.UpdatedAt = time.Now()

	s.prompts[p.ID] = &p
	return &p, nil
}

// DeletePrompt removes a prompt template.
func (s *Store) DeletePrompt(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.prompts[id]; !exists {
		return ErrPromptNotFound
	}
	delete(s.prompts, id)
	s.stats.TotalPrompts = len(s.prompts)
	return nil
}

// StoreEmbedding stores a vector embedding.
func (s *Store) StoreEmbedding(e Embedding) (*Embedding, error) {
	if e.ID == "" {
		return nil, fmt.Errorf("%w: id is required", ErrInvalidConfig)
	}
	if len(e.Vector) == 0 {
		return nil, fmt.Errorf("%w: vector is required", ErrInvalidConfig)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	e.CreatedAt = time.Now()
	s.embeddings[e.ID] = &e
	s.stats.TotalEmbeddings = len(s.embeddings)
	return &e, nil
}

// GetEmbedding returns an embedding by ID.
func (s *Store) GetEmbedding(id string) (*Embedding, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, exists := s.embeddings[id]
	if !exists {
		return nil, ErrEmbeddingNotFound
	}
	return e, nil
}

// SearchSimilar finds the most similar embeddings to a query vector.
func (s *Store) SearchSimilar(queryVector []float64, topK int, minScore float64) []SimilarityResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.mu.RUnlock()
	s.mu.Lock()
	s.stats.TotalSearches++
	s.mu.Unlock()
	s.mu.RLock()

	if topK <= 0 {
		topK = s.config.DefaultTopK
	}

	type scored struct {
		id    string
		score float64
		emb   *Embedding
	}

	var results []scored
	for id, emb := range s.embeddings {
		score := cosineSimilarity(queryVector, emb.Vector)
		if score >= minScore {
			results = append(results, scored{id: id, score: score, emb: emb})
		}
	}

	// Simple top-K selection
	for i := 0; i < len(results) && i < topK; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if topK > len(results) {
		topK = len(results)
	}

	out := make([]SimilarityResult, topK)
	for i := 0; i < topK; i++ {
		out[i] = SimilarityResult{
			ID:       results[i].id,
			Score:    results[i].score,
			Text:     results[i].emb.Text,
			Metadata: results[i].emb.Metadata,
		}
	}
	return out
}

// CreatePipeline creates a new RAG pipeline.
func (s *Store) CreatePipeline(p RAGPipeline) (*RAGPipeline, error) {
	if p.ID == "" || p.Name == "" {
		return nil, fmt.Errorf("%w: id and name are required", ErrInvalidConfig)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.pipelines[p.ID]; exists {
		return nil, ErrPipelineExists
	}

	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.TopK <= 0 {
		p.TopK = s.config.DefaultTopK
	}

	s.pipelines[p.ID] = &p
	s.stats.TotalPipelines = len(s.pipelines)
	return &p, nil
}

// GetPipeline returns a RAG pipeline by ID.
func (s *Store) GetPipeline(id string) (*RAGPipeline, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, exists := s.pipelines[id]
	if !exists {
		return nil, ErrPipelineNotFound
	}
	return p, nil
}

// ListPipelines returns all RAG pipelines.
func (s *Store) ListPipelines() []RAGPipeline {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]RAGPipeline, 0, len(s.pipelines))
	for _, p := range s.pipelines {
		result = append(result, *p)
	}
	return result
}

// QueryRAG executes a RAG query against a pipeline.
func (s *Store) QueryRAG(req RAGRequest) (*RAGResponse, error) {
	s.mu.Lock()
	s.stats.TotalQueries++
	s.mu.Unlock()

	start := time.Now()

	s.mu.RLock()
	pipeline, exists := s.pipelines[req.PipelineID]
	if !exists {
		s.mu.RUnlock()
		return nil, ErrPipelineNotFound
	}
	s.mu.RUnlock()

	topK := req.TopK
	if topK <= 0 {
		topK = pipeline.TopK
	}

	// In a real implementation, this would embed the query and search
	context := s.SearchSimilar(nil, topK, pipeline.MinScore)

	// Build augmented prompt
	augmented := fmt.Sprintf("Context:\n")
	for _, c := range context {
		augmented += fmt.Sprintf("- %s\n", c.Text)
	}
	augmented += fmt.Sprintf("\nQuery: %s", req.Query)

	return &RAGResponse{
		Query:           req.Query,
		Context:         context,
		AugmentedPrompt: augmented,
		PipelineID:      req.PipelineID,
		Duration:        time.Since(start),
	}, nil
}

// Stats returns store statistics.
func (s *Store) Stats() StoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
