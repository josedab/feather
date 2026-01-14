package llmfeature

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// FeatureType identifies the kind of LLM feature.
type FeatureType string

const (
	TypePromptTemplate FeatureType = "prompt_template"
	TypeCompletion     FeatureType = "completion"
	TypeTokenUsage     FeatureType = "token_usage"
	TypeEmbedding      FeatureType = "embedding"
	TypeRAGContext     FeatureType = "rag_context"
)

var (
	ErrTemplateNotFound   = errors.New("prompt template not found")
	ErrTemplateExists     = errors.New("prompt template already exists")
	ErrCompletionNotFound = errors.New("completion not found")
	ErrInvalidTemplate    = errors.New("invalid template")
)

// PromptTemplate stores a reusable prompt with variable interpolation.
type PromptTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Template    string            `json:"template"`
	Variables   []string          `json:"variables"`
	Model       string            `json:"model,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Version     int               `json:"version"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// CompletionRecord stores a cached LLM completion.
type CompletionRecord struct {
	ID            string    `json:"id"`
	TemplateID    string    `json:"template_id,omitempty"`
	Prompt        string    `json:"prompt"`
	Completion    string    `json:"completion"`
	Model         string    `json:"model"`
	TokensPrompt  int       `json:"tokens_prompt"`
	TokensCompletion int   `json:"tokens_completion"`
	TokensTotal   int       `json:"tokens_total"`
	CostUSD       float64   `json:"cost_usd"`
	LatencyMs     int64     `json:"latency_ms"`
	CacheHit      bool      `json:"cache_hit"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
}

// TokenUsage tracks per-model token consumption.
type TokenUsage struct {
	Model           string  `json:"model"`
	TotalPrompt     int64   `json:"total_prompt_tokens"`
	TotalCompletion int64   `json:"total_completion_tokens"`
	TotalCost       float64 `json:"total_cost_usd"`
	RequestCount    int64   `json:"request_count"`
	CacheHits       int64   `json:"cache_hits"`
	CacheSavings    float64 `json:"cache_savings_usd"`
}

// ModelPricing defines per-token costs for a model.
type ModelPricing struct {
	Model            string  `json:"model"`
	PromptCostPer1K  float64 `json:"prompt_cost_per_1k"`
	CompletionCostPer1K float64 `json:"completion_cost_per_1k"`
}

// StoreConfig configures the LLM feature store.
type StoreConfig struct {
	MaxTemplates    int
	MaxCompletions  int
	CompletionTTL   time.Duration
	DefaultPricing  map[string]*ModelPricing
}

// DefaultStoreConfig returns sensible defaults.
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		MaxTemplates:   1000,
		MaxCompletions: 10000,
		CompletionTTL:  time.Hour,
		DefaultPricing: map[string]*ModelPricing{
			"gpt-4":       {Model: "gpt-4", PromptCostPer1K: 0.03, CompletionCostPer1K: 0.06},
			"gpt-3.5":     {Model: "gpt-3.5", PromptCostPer1K: 0.0005, CompletionCostPer1K: 0.0015},
			"claude-3":    {Model: "claude-3", PromptCostPer1K: 0.015, CompletionCostPer1K: 0.075},
		},
	}
}

// Store manages LLM feature types.
type Store struct {
	mu          sync.RWMutex
	templates   map[string]*PromptTemplate
	completions map[string]*CompletionRecord
	usage       map[string]*TokenUsage
	pricing     map[string]*ModelPricing
	config      StoreConfig

	// Atomic counters
	totalRequests atomic.Int64
	totalTokens   atomic.Int64
	cacheHits     atomic.Int64
}

// NewStore creates a new LLM feature store.
func NewStore(config StoreConfig) *Store {
	if config.MaxTemplates == 0 {
		config = DefaultStoreConfig()
	}
	return &Store{
		templates:   make(map[string]*PromptTemplate),
		completions: make(map[string]*CompletionRecord),
		usage:       make(map[string]*TokenUsage),
		pricing:     config.DefaultPricing,
		config:      config,
	}
}

// CreateTemplate registers a new prompt template.
func (s *Store) CreateTemplate(tmpl *PromptTemplate) error {
	if tmpl.Name == "" || tmpl.Template == "" {
		return fmt.Errorf("%w: name and template required", ErrInvalidTemplate)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.templates[tmpl.ID]; exists {
		return ErrTemplateExists
	}

	now := time.Now()
	tmpl.CreatedAt = now
	tmpl.UpdatedAt = now
	tmpl.Version = 1
	s.templates[tmpl.ID] = tmpl
	return nil
}

// UpdateTemplate updates an existing template, incrementing the version.
func (s *Store) UpdateTemplate(tmpl *PromptTemplate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.templates[tmpl.ID]
	if !ok {
		return ErrTemplateNotFound
	}

	tmpl.Version = existing.Version + 1
	tmpl.CreatedAt = existing.CreatedAt
	tmpl.UpdatedAt = time.Now()
	s.templates[tmpl.ID] = tmpl
	return nil
}

// GetTemplate retrieves a prompt template.
func (s *Store) GetTemplate(id string) (*PromptTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tmpl, ok := s.templates[id]
	if !ok {
		return nil, ErrTemplateNotFound
	}
	return tmpl, nil
}

// ListTemplates returns all prompt templates.
func (s *Store) ListTemplates() []*PromptTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*PromptTemplate, 0, len(s.templates))
	for _, t := range s.templates {
		result = append(result, t)
	}
	return result
}

// DeleteTemplate removes a template.
func (s *Store) DeleteTemplate(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.templates[id]; !ok {
		return ErrTemplateNotFound
	}
	delete(s.templates, id)
	return nil
}

// StoreCompletion caches a completion and tracks token usage.
func (s *Store) StoreCompletion(rec *CompletionRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rec.ExpiresAt.IsZero() {
		rec.ExpiresAt = time.Now().Add(s.config.CompletionTTL)
	}
	rec.CreatedAt = time.Now()

	s.completions[rec.ID] = rec

	// Track usage
	s.trackUsageLocked(rec)

	// Evict old completions
	if len(s.completions) > s.config.MaxCompletions {
		s.evictOldestCompletionLocked()
	}

	s.totalRequests.Add(1)
	s.totalTokens.Add(int64(rec.TokensTotal))
	if rec.CacheHit {
		s.cacheHits.Add(1)
	}
}

// GetCompletion retrieves a cached completion.
func (s *Store) GetCompletion(id string) (*CompletionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.completions[id]
	if !ok {
		return nil, ErrCompletionNotFound
	}
	if !rec.ExpiresAt.IsZero() && time.Now().After(rec.ExpiresAt) {
		return nil, ErrCompletionNotFound
	}
	return rec, nil
}

// GetUsage returns token usage for a model.
func (s *Store) GetUsage(model string) *TokenUsage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if u, ok := s.usage[model]; ok {
		return u
	}
	return &TokenUsage{Model: model}
}

// GetAllUsage returns usage for all models.
func (s *Store) GetAllUsage() []*TokenUsage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*TokenUsage, 0, len(s.usage))
	for _, u := range s.usage {
		result = append(result, u)
	}
	return result
}

// Stats returns aggregate statistics.
func (s *Store) Stats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalCost float64
	for _, u := range s.usage {
		totalCost += u.TotalCost
	}

	return map[string]interface{}{
		"total_templates":   len(s.templates),
		"total_completions": len(s.completions),
		"total_requests":    s.totalRequests.Load(),
		"total_tokens":      s.totalTokens.Load(),
		"cache_hits":        s.cacheHits.Load(),
		"total_cost_usd":    totalCost,
		"models_tracked":    len(s.usage),
	}
}

// SetPricing updates pricing for a model.
func (s *Store) SetPricing(p *ModelPricing) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pricing[p.Model] = p
}

func (s *Store) trackUsageLocked(rec *CompletionRecord) {
	usage, ok := s.usage[rec.Model]
	if !ok {
		usage = &TokenUsage{Model: rec.Model}
		s.usage[rec.Model] = usage
	}

	usage.RequestCount++
	usage.TotalPrompt += int64(rec.TokensPrompt)
	usage.TotalCompletion += int64(rec.TokensCompletion)

	// Calculate cost
	if pricing, ok := s.pricing[rec.Model]; ok {
		cost := float64(rec.TokensPrompt)/1000*pricing.PromptCostPer1K +
			float64(rec.TokensCompletion)/1000*pricing.CompletionCostPer1K
		usage.TotalCost += cost
		rec.CostUSD = cost

		if rec.CacheHit {
			usage.CacheHits++
			usage.CacheSavings += cost
		}
	}
}

func (s *Store) evictOldestCompletionLocked() {
	var oldestKey string
	var oldestTime time.Time
	for k, v := range s.completions {
		if oldestKey == "" || v.CreatedAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.CreatedAt
		}
	}
	if oldestKey != "" {
		delete(s.completions, oldestKey)
	}
}
