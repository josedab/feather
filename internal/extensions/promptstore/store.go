package promptstore

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// PromptTemplate represents a versioned LLM prompt template.
type PromptTemplate struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	Template      string            `json:"template"`
	Variables     []string          `json:"variables,omitempty"`
	Model         string            `json:"model,omitempty"`
	MaxTokens     int               `json:"max_tokens,omitempty"`
	Temperature   float64           `json:"temperature,omitempty"`
	Version       int               `json:"version"`
	Tags          map[string]string `json:"tags,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	TrafficWeight float64           `json:"traffic_weight"`
}

// PromptUsage tracks usage statistics for a prompt template.
type PromptUsage struct {
	PromptID     string  `json:"prompt_id"`
	Version      int     `json:"version"`
	Invocations  int64   `json:"invocations"`
	TotalTokens  int64   `json:"total_tokens"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	ErrorCount   int64   `json:"error_count"`
	AvgScore     float64 `json:"avg_score"`
	ScoreCount   int64   `json:"score_count"`
}

// RenderResult represents a rendered prompt with metadata.
type RenderResult struct {
	PromptID    string `json:"prompt_id"`
	Version     int    `json:"version"`
	Rendered    string `json:"rendered"`
	TokenEstimate int  `json:"token_estimate"`
}

// StoreConfig configures the prompt store.
type StoreConfig struct {
	MaxVersions    int `json:"max_versions"`
	MaxPrompts     int `json:"max_prompts"`
	MaxTemplateLen int `json:"max_template_len"`
}

// DefaultStoreConfig returns sensible defaults.
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		MaxVersions:    100,
		MaxPrompts:     10000,
		MaxTemplateLen: 100000,
	}
}

// Store manages LLM prompt templates with versioning and tracking.
type Store struct {
	mu       sync.RWMutex
	config   StoreConfig
	prompts  map[string][]*PromptTemplate // ID -> versions (sorted)
	usage    map[string]*PromptUsage       // "id:version" -> usage
}

// NewStore creates a new prompt store.
func NewStore(config StoreConfig) *Store {
	if config.MaxPrompts == 0 {
		config = DefaultStoreConfig()
	}
	return &Store{
		config:  config,
		prompts: make(map[string][]*PromptTemplate),
		usage:   make(map[string]*PromptUsage),
	}
}

// Create creates a new prompt template.
func (s *Store) Create(tmpl PromptTemplate) (*PromptTemplate, error) {
	if tmpl.ID == "" || tmpl.Template == "" {
		return nil, fmt.Errorf("%w: id and template are required", ErrInvalidTemplate)
	}
	if len(tmpl.Template) > s.config.MaxTemplateLen {
		return nil, fmt.Errorf("%w: template exceeds max length", ErrInvalidTemplate)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.prompts[tmpl.ID]; exists {
		return nil, ErrPromptExists
	}

	if len(s.prompts) >= s.config.MaxPrompts {
		return nil, fmt.Errorf("max prompts reached (%d)", s.config.MaxPrompts)
	}

	now := time.Now()
	tmpl.Version = 1
	tmpl.CreatedAt = now
	tmpl.UpdatedAt = now
	if tmpl.TrafficWeight == 0 {
		tmpl.TrafficWeight = 1.0
	}
	if tmpl.Variables == nil {
		tmpl.Variables = extractVariables(tmpl.Template)
	}

	s.prompts[tmpl.ID] = []*PromptTemplate{&tmpl}
	return &tmpl, nil
}

// Update creates a new version of an existing prompt.
func (s *Store) Update(id string, tmpl PromptTemplate) (*PromptTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	versions, exists := s.prompts[id]
	if !exists {
		return nil, ErrPromptNotFound
	}

	latest := versions[len(versions)-1]
	newVersion := *latest
	newVersion.Version = latest.Version + 1
	newVersion.UpdatedAt = time.Now()

	if tmpl.Template != "" {
		newVersion.Template = tmpl.Template
		newVersion.Variables = extractVariables(tmpl.Template)
	}
	if tmpl.Name != "" {
		newVersion.Name = tmpl.Name
	}
	if tmpl.Description != "" {
		newVersion.Description = tmpl.Description
	}
	if tmpl.Model != "" {
		newVersion.Model = tmpl.Model
	}
	if tmpl.MaxTokens > 0 {
		newVersion.MaxTokens = tmpl.MaxTokens
	}
	if tmpl.Temperature > 0 {
		newVersion.Temperature = tmpl.Temperature
	}
	if tmpl.TrafficWeight > 0 {
		newVersion.TrafficWeight = tmpl.TrafficWeight
	}

	s.prompts[id] = append(versions, &newVersion)

	if len(s.prompts[id]) > s.config.MaxVersions {
		s.prompts[id] = s.prompts[id][1:]
	}

	return &newVersion, nil
}

// Get returns the latest version of a prompt.
func (s *Store) Get(id string) (*PromptTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, exists := s.prompts[id]
	if !exists {
		return nil, ErrPromptNotFound
	}

	latest := versions[len(versions)-1]
	result := *latest
	return &result, nil
}

// GetVersion returns a specific version of a prompt.
func (s *Store) GetVersion(id string, version int) (*PromptTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, exists := s.prompts[id]
	if !exists {
		return nil, ErrPromptNotFound
	}

	for _, v := range versions {
		if v.Version == version {
			result := *v
			return &result, nil
		}
	}
	return nil, ErrVersionNotFound
}

// List returns all prompts (latest versions).
func (s *Store) List() []PromptTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]PromptTemplate, 0, len(s.prompts))
	for _, versions := range s.prompts {
		if len(versions) > 0 {
			result = append(result, *versions[len(versions)-1])
		}
	}
	return result
}

// ListVersions returns all versions of a prompt.
func (s *Store) ListVersions(id string) ([]PromptTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, exists := s.prompts[id]
	if !exists {
		return nil, ErrPromptNotFound
	}

	result := make([]PromptTemplate, len(versions))
	for i, v := range versions {
		result[i] = *v
	}
	return result, nil
}

// Delete removes a prompt and all its versions.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.prompts[id]; !exists {
		return ErrPromptNotFound
	}

	delete(s.prompts, id)
	return nil
}

// Render renders a prompt template with the given variables.
func (s *Store) Render(id string, vars map[string]string) (*RenderResult, error) {
	tmpl, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	rendered := tmpl.Template
	for k, v := range vars {
		rendered = strings.ReplaceAll(rendered, "{{"+k+"}}", v)
	}

	s.RecordUsage(id, tmpl.Version, 0, 0, nil)

	return &RenderResult{
		PromptID:      id,
		Version:       tmpl.Version,
		Rendered:      rendered,
		TokenEstimate: estimateTokens(rendered),
	}, nil
}

// RecordUsage records a prompt invocation with metrics.
func (s *Store) RecordUsage(id string, version int, tokens int64, latencyMs float64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%d", id, version)
	usage, exists := s.usage[key]
	if !exists {
		usage = &PromptUsage{PromptID: id, Version: version}
		s.usage[key] = usage
	}

	usage.Invocations++
	usage.TotalTokens += tokens
	// Rolling average for latency
	if latencyMs > 0 {
		usage.AvgLatencyMs = usage.AvgLatencyMs + (latencyMs-usage.AvgLatencyMs)/float64(usage.Invocations)
	}
	if err != nil {
		usage.ErrorCount++
	}
}

// RecordScore records a quality score for a prompt invocation.
func (s *Store) RecordScore(id string, version int, score float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%d", id, version)
	usage, exists := s.usage[key]
	if !exists {
		usage = &PromptUsage{PromptID: id, Version: version}
		s.usage[key] = usage
	}

	usage.ScoreCount++
	usage.AvgScore = usage.AvgScore + (score-usage.AvgScore)/float64(usage.ScoreCount)
}

// GetUsage returns usage statistics for a prompt.
func (s *Store) GetUsage(id string) []PromptUsage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []PromptUsage
	for key, u := range s.usage {
		if strings.HasPrefix(key, id+":") {
			result = append(result, *u)
		}
	}
	return result
}

// Stats returns aggregate store statistics.
func (s *Store) Stats() StoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var stats StoreStats
	stats.TotalPrompts = len(s.prompts)
	for _, versions := range s.prompts {
		stats.TotalVersions += len(versions)
	}
	for _, u := range s.usage {
		stats.TotalInvocations += u.Invocations
		stats.TotalTokens += u.TotalTokens
	}
	return stats
}

// StoreStats provides aggregate statistics.
type StoreStats struct {
	TotalPrompts     int   `json:"total_prompts"`
	TotalVersions    int   `json:"total_versions"`
	TotalInvocations int64 `json:"total_invocations"`
	TotalTokens      int64 `json:"total_tokens"`
}

func extractVariables(template string) []string {
	var vars []string
	seen := make(map[string]bool)
	for {
		start := strings.Index(template, "{{")
		if start == -1 {
			break
		}
		end := strings.Index(template[start:], "}}")
		if end == -1 {
			break
		}
		v := strings.TrimSpace(template[start+2 : start+end])
		if v != "" && !seen[v] {
			vars = append(vars, v)
			seen[v] = true
		}
		template = template[start+end+2:]
	}
	return vars
}

func estimateTokens(text string) int {
	// Rough estimate: ~4 chars per token for English text
	return (len(text) + 3) / 4
}
