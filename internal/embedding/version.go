package embedding

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Errors returned by version manager.
var (
	ErrModelNotRegistered   = errors.New("model not registered")
	ErrVersionNotFound      = errors.New("version not found")
	ErrIncompatibleVersion  = errors.New("incompatible model version")
	ErrVersionAlreadyExists = errors.New("version already exists")
)

// ModelInfo represents information about an embedding model.
type ModelInfo struct {
	// ID is the unique model identifier.
	ID string `json:"id"`

	// Name is the human-readable name.
	Name string `json:"name"`

	// Provider is the model provider (openai, cohere, etc).
	Provider string `json:"provider"`

	// Dimension is the embedding dimension.
	Dimension int `json:"dimension"`

	// MaxTokens is the maximum input tokens.
	MaxTokens int `json:"max_tokens"`

	// Description describes the model.
	Description string `json:"description,omitempty"`

	// Metadata contains additional information.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ModelVersion represents a specific version of a model.
type ModelVersion struct {
	// Version is the version string.
	Version string `json:"version"`

	// ModelID is the parent model ID.
	ModelID string `json:"model_id"`

	// Dimension is the embedding dimension for this version.
	Dimension int `json:"dimension"`

	// Compatible lists versions this is compatible with.
	Compatible []string `json:"compatible,omitempty"`

	// ReleasedAt is when this version was released.
	ReleasedAt time.Time `json:"released_at"`

	// DeprecatedAt is when this version was deprecated.
	DeprecatedAt *time.Time `json:"deprecated_at,omitempty"`

	// IsDefault indicates if this is the default version.
	IsDefault bool `json:"is_default"`

	// Metadata contains additional information.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// VersionConfig configures version management.
type VersionConfig struct {
	// AutoMigrate automatically re-embeds when versions change.
	AutoMigrate bool `json:"auto_migrate" yaml:"auto_migrate"`

	// StrictCompatibility enforces strict version compatibility.
	StrictCompatibility bool `json:"strict_compatibility" yaml:"strict_compatibility"`

	// WarnOnDeprecated warns when using deprecated versions.
	WarnOnDeprecated bool `json:"warn_on_deprecated" yaml:"warn_on_deprecated"`
}

// DefaultVersionConfig returns the default version configuration.
func DefaultVersionConfig() VersionConfig {
	return VersionConfig{
		AutoMigrate:         false,
		StrictCompatibility: true,
		WarnOnDeprecated:    true,
	}
}

// VersionManager manages embedding model versions.
type VersionManager struct {
	mu       sync.RWMutex
	config   VersionConfig
	models   map[string]*ModelInfo
	versions map[string]map[string]*ModelVersion // modelID -> version -> ModelVersion

	// Metrics
	versionChecks     int64
	compatibleCount   int64
	incompatibleCount int64
	deprecatedUsage   int64
}

// NewVersionManager creates a new version manager.
func NewVersionManager(config VersionConfig) *VersionManager {
	return &VersionManager{
		config:   config,
		models:   make(map[string]*ModelInfo),
		versions: make(map[string]map[string]*ModelVersion),
	}
}

// RegisterModel registers a new embedding model.
func (m *VersionManager) RegisterModel(model *ModelInfo) error {
	if model.ID == "" {
		return errors.New("model ID is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.models[model.ID] = model
	if m.versions[model.ID] == nil {
		m.versions[model.ID] = make(map[string]*ModelVersion)
	}

	return nil
}

// RegisterVersion registers a new version for a model.
func (m *VersionManager) RegisterVersion(version *ModelVersion) error {
	if version.ModelID == "" || version.Version == "" {
		return errors.New("model ID and version are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.models[version.ModelID] == nil {
		return ErrModelNotRegistered
	}

	if m.versions[version.ModelID] == nil {
		m.versions[version.ModelID] = make(map[string]*ModelVersion)
	}

	if _, exists := m.versions[version.ModelID][version.Version]; exists {
		return ErrVersionAlreadyExists
	}

	if version.ReleasedAt.IsZero() {
		version.ReleasedAt = time.Now()
	}

	// Set dimension from model if not specified
	if version.Dimension == 0 {
		version.Dimension = m.models[version.ModelID].Dimension
	}

	m.versions[version.ModelID][version.Version] = version

	return nil
}

// GetModel returns a registered model.
func (m *VersionManager) GetModel(modelID string) (*ModelInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	model, ok := m.models[modelID]
	if !ok {
		return nil, ErrModelNotRegistered
	}

	return model, nil
}

// GetVersion returns a specific version of a model.
func (m *VersionManager) GetVersion(modelID, version string) (*ModelVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, ok := m.versions[modelID]
	if !ok {
		return nil, ErrModelNotRegistered
	}

	v, ok := versions[version]
	if !ok {
		return nil, ErrVersionNotFound
	}

	return v, nil
}

// GetDefaultVersion returns the default version for a model.
func (m *VersionManager) GetDefaultVersion(modelID string) (*ModelVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, ok := m.versions[modelID]
	if !ok {
		return nil, ErrModelNotRegistered
	}

	for _, v := range versions {
		if v.IsDefault {
			return v, nil
		}
	}

	// Return latest if no default
	var latest *ModelVersion
	for _, v := range versions {
		if latest == nil || v.ReleasedAt.After(latest.ReleasedAt) {
			latest = v
		}
	}

	if latest == nil {
		return nil, ErrVersionNotFound
	}

	return latest, nil
}

// ListVersions returns all versions for a model.
func (m *VersionManager) ListVersions(modelID string) ([]*ModelVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, ok := m.versions[modelID]
	if !ok {
		return nil, ErrModelNotRegistered
	}

	result := make([]*ModelVersion, 0, len(versions))
	for _, v := range versions {
		result = append(result, v)
	}

	return result, nil
}

// ListModels returns all registered models.
func (m *VersionManager) ListModels() []*ModelInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ModelInfo, 0, len(m.models))
	for _, model := range m.models {
		result = append(result, model)
	}

	return result
}

// CheckCompatibility checks if two versions are compatible.
func (m *VersionManager) CheckCompatibility(modelID, version1, version2 string) (bool, error) {
	atomic.AddInt64(&m.versionChecks, 1)

	if version1 == version2 {
		atomic.AddInt64(&m.compatibleCount, 1)
		return true, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, ok := m.versions[modelID]
	if !ok {
		return false, ErrModelNotRegistered
	}

	v1, ok := versions[version1]
	if !ok {
		return false, ErrVersionNotFound
	}

	// Check dimension compatibility
	v2, ok := versions[version2]
	if !ok {
		return false, ErrVersionNotFound
	}

	if v1.Dimension != v2.Dimension {
		atomic.AddInt64(&m.incompatibleCount, 1)
		return false, nil
	}

	// Check explicit compatibility list
	for _, comp := range v1.Compatible {
		if comp == version2 {
			atomic.AddInt64(&m.compatibleCount, 1)
			return true, nil
		}
	}

	// If strict compatibility, versions must be explicitly compatible
	if m.config.StrictCompatibility {
		atomic.AddInt64(&m.incompatibleCount, 1)
		return false, nil
	}

	// Non-strict: same dimension means compatible
	atomic.AddInt64(&m.compatibleCount, 1)
	return true, nil
}

// ValidateEmbedding validates an embedding against version requirements.
func (m *VersionManager) ValidateEmbedding(ctx context.Context, emb *Embedding) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	model, ok := m.models[emb.ModelID]
	if !ok {
		return ErrModelNotRegistered
	}

	versions, ok := m.versions[emb.ModelID]
	if !ok {
		return ErrModelNotRegistered
	}

	version, ok := versions[emb.ModelVersion]
	if !ok {
		return ErrVersionNotFound
	}

	// Check dimension
	if emb.Dimension != version.Dimension && emb.Dimension != model.Dimension {
		return ErrDimensionMismatch
	}

	// Check if deprecated
	if version.DeprecatedAt != nil && m.config.WarnOnDeprecated {
		atomic.AddInt64(&m.deprecatedUsage, 1)
	}

	return nil
}

// IsDeprecated checks if a version is deprecated.
func (m *VersionManager) IsDeprecated(modelID, version string) (bool, error) {
	v, err := m.GetVersion(modelID, version)
	if err != nil {
		return false, err
	}

	return v.DeprecatedAt != nil, nil
}

// DeprecateVersion marks a version as deprecated.
func (m *VersionManager) DeprecateVersion(modelID, version string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions, ok := m.versions[modelID]
	if !ok {
		return ErrModelNotRegistered
	}

	v, ok := versions[version]
	if !ok {
		return ErrVersionNotFound
	}

	now := time.Now()
	v.DeprecatedAt = &now

	return nil
}

// SetDefaultVersion sets the default version for a model.
func (m *VersionManager) SetDefaultVersion(modelID, version string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions, ok := m.versions[modelID]
	if !ok {
		return ErrModelNotRegistered
	}

	if _, ok := versions[version]; !ok {
		return ErrVersionNotFound
	}

	// Clear previous default
	for _, v := range versions {
		v.IsDefault = false
	}

	versions[version].IsDefault = true

	return nil
}

// Stats returns version manager statistics.
func (m *VersionManager) Stats() map[string]interface{} {
	m.mu.RLock()
	modelCount := len(m.models)
	var totalVersions int
	for _, versions := range m.versions {
		totalVersions += len(versions)
	}
	m.mu.RUnlock()

	return map[string]interface{}{
		"model_count":          modelCount,
		"total_versions":       totalVersions,
		"version_checks":       atomic.LoadInt64(&m.versionChecks),
		"compatible_count":     atomic.LoadInt64(&m.compatibleCount),
		"incompatible_count":   atomic.LoadInt64(&m.incompatibleCount),
		"deprecated_usage":     atomic.LoadInt64(&m.deprecatedUsage),
		"strict_compatibility": m.config.StrictCompatibility,
	}
}

// Common embedding models (for convenience).
var (
	ModelOpenAIAda002 = &ModelInfo{
		ID:        "text-embedding-ada-002",
		Name:      "OpenAI text-embedding-ada-002",
		Provider:  "openai",
		Dimension: 1536,
		MaxTokens: 8191,
	}

	ModelOpenAI3Small = &ModelInfo{
		ID:        "text-embedding-3-small",
		Name:      "OpenAI text-embedding-3-small",
		Provider:  "openai",
		Dimension: 1536,
		MaxTokens: 8191,
	}

	ModelOpenAI3Large = &ModelInfo{
		ID:        "text-embedding-3-large",
		Name:      "OpenAI text-embedding-3-large",
		Provider:  "openai",
		Dimension: 3072,
		MaxTokens: 8191,
	}

	ModelCohereEnglish = &ModelInfo{
		ID:        "embed-english-v3.0",
		Name:      "Cohere embed-english-v3.0",
		Provider:  "cohere",
		Dimension: 1024,
		MaxTokens: 512,
	}
)
