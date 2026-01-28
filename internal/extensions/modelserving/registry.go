package modelserving

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var (
	ErrModelNotFound   = errors.New("model not found")
	ErrVersionNotFound = errors.New("model version not found")
	ErrBundleNotFound  = errors.New("feature bundle not found")
	ErrAlreadyExists   = errors.New("model already exists")
)

// ModelStatus indicates the model's lifecycle state.
type ModelStatus string

const (
	ModelStatusActive     ModelStatus = "active"
	ModelStatusStaging    ModelStatus = "staging"
	ModelStatusArchived   ModelStatus = "archived"
	ModelStatusDeprecated ModelStatus = "deprecated"
)

// RegistryConfig configures the model registry.
type RegistryConfig struct {
	MaxModels           int  `json:"max_models"`
	MaxVersionsPerModel int  `json:"max_versions_per_model"`
	EnableCaching       bool `json:"enable_caching"`
}

// DefaultRegistryConfig returns sensible defaults for the registry.
func DefaultRegistryConfig() RegistryConfig {
	return RegistryConfig{
		MaxModels:           1000,
		MaxVersionsPerModel: 100,
		EnableCaching:       true,
	}
}

// Model represents a registered ML model.
type Model struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Framework   string            `json:"framework"`
	Owner       string            `json:"owner,omitempty"`
	Status      ModelStatus       `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// ModelVersion represents a specific version of a model.
type ModelVersion struct {
	ModelID    string            `json:"model_id"`
	Version    int               `json:"version"`
	Features   []string          `json:"features"`
	EntityType string            `json:"entity_type"`
	Endpoint   string            `json:"endpoint,omitempty"`
	Status     ModelStatus       `json:"status"`
	CreatedAt  time.Time         `json:"created_at"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// FeatureBundle is a pre-computed set of features for a model.
type FeatureBundle struct {
	ModelID         string                 `json:"model_id"`
	Version         int                    `json:"version"`
	EntityKey       string                 `json:"entity_key"`
	Features        map[string]interface{} `json:"features"`
	ResolvedAt      time.Time              `json:"resolved_at"`
	CacheHit        bool                   `json:"cache_hit"`
	MissingFeatures []string               `json:"missing_features,omitempty"`
}

// RegistryStats holds summary statistics for the registry.
type RegistryStats struct {
	ModelCount   int     `json:"model_count"`
	VersionCount int     `json:"version_count"`
	BundleCount  int     `json:"bundle_count"`
	CacheHitRate float64 `json:"cache_hit_rate"`
}

// Registry manages model registrations and feature mappings.
type Registry struct {
	mu       sync.RWMutex
	config   RegistryConfig
	models   map[string]*Model
	versions map[string][]*ModelVersion
	bundles  map[string]*FeatureBundle

	cacheHits    int64
	cacheLookups int64
}

// NewRegistry creates a new model registry with the given configuration.
func NewRegistry(config RegistryConfig) *Registry {
	return &Registry{
		config:   config,
		models:   make(map[string]*Model),
		versions: make(map[string][]*ModelVersion),
		bundles:  make(map[string]*FeatureBundle),
	}
}

// RegisterModel adds a model to the registry.
func (r *Registry) RegisterModel(model *Model) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.models[model.ID]; exists {
		return fmt.Errorf("registering model %q: %w", model.ID, ErrAlreadyExists)
	}

	if len(r.models) >= r.config.MaxModels {
		return fmt.Errorf("registry at capacity (%d models)", r.config.MaxModels)
	}

	now := time.Now()
	model.CreatedAt = now
	model.UpdatedAt = now
	r.models[model.ID] = model
	return nil
}

// GetModel retrieves a model by ID.
func (r *Registry) GetModel(id string) (*Model, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	m, ok := r.models[id]
	if !ok {
		return nil, fmt.Errorf("getting model %q: %w", id, ErrModelNotFound)
	}
	return m, nil
}

// ListModels returns all registered models.
func (r *Registry) ListModels() []*Model {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Model, 0, len(r.models))
	for _, m := range r.models {
		result = append(result, m)
	}
	return result
}

// RemoveModel removes a model and all its versions from the registry.
func (r *Registry) RemoveModel(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.models[id]; !ok {
		return fmt.Errorf("removing model %q: %w", id, ErrModelNotFound)
	}

	delete(r.models, id)
	delete(r.versions, id)

	// Remove cached bundles for this model.
	for key, b := range r.bundles {
		if b.ModelID == id {
			delete(r.bundles, key)
		}
	}
	return nil
}

// RegisterVersion adds a version to an existing model.
func (r *Registry) RegisterVersion(version *ModelVersion) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.models[version.ModelID]; !ok {
		return fmt.Errorf("registering version for model %q: %w", version.ModelID, ErrModelNotFound)
	}

	versions := r.versions[version.ModelID]
	if len(versions) >= r.config.MaxVersionsPerModel {
		return fmt.Errorf("model %q at version capacity (%d versions)", version.ModelID, r.config.MaxVersionsPerModel)
	}

	// Check for duplicate version number.
	for _, v := range versions {
		if v.Version == version.Version {
			return fmt.Errorf("version %d of model %q: %w", version.Version, version.ModelID, ErrAlreadyExists)
		}
	}

	version.CreatedAt = time.Now()
	r.versions[version.ModelID] = append(r.versions[version.ModelID], version)
	return nil
}

// GetVersion retrieves a specific version of a model.
func (r *Registry) GetVersion(modelID string, version int) (*ModelVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, ok := r.versions[modelID]
	if !ok {
		return nil, fmt.Errorf("getting version %d of model %q: %w", version, modelID, ErrModelNotFound)
	}

	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}
	return nil, fmt.Errorf("getting version %d of model %q: %w", version, modelID, ErrVersionNotFound)
}

// GetLatestVersion returns the highest version number for a model.
func (r *Registry) GetLatestVersion(modelID string) (*ModelVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, ok := r.versions[modelID]
	if !ok || len(versions) == 0 {
		return nil, fmt.Errorf("getting latest version of model %q: %w", modelID, ErrVersionNotFound)
	}

	latest := versions[0]
	for _, v := range versions[1:] {
		if v.Version > latest.Version {
			latest = v
		}
	}
	return latest, nil
}

// ListVersions returns all versions for a model, sorted by version number.
func (r *Registry) ListVersions(modelID string) []*ModelVersion {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions := r.versions[modelID]
	if len(versions) == 0 {
		return nil
	}

	result := make([]*ModelVersion, len(versions))
	copy(result, versions)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})
	return result
}

// bundleCacheKey builds a cache key for a feature bundle.
func bundleCacheKey(modelID string, version int, entityKey string) string {
	return fmt.Sprintf("%s:%d:%s", modelID, version, entityKey)
}

// ResolveFeatures builds a FeatureBundle from provided feature values for the
// given model and version. Missing features are tracked in the bundle.
func (r *Registry) ResolveFeatures(modelID string, version int, entityKey string, featureValues map[string]interface{}) (*FeatureBundle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check cache first.
	cacheKey := bundleCacheKey(modelID, version, entityKey)
	r.cacheLookups++

	if r.config.EnableCaching {
		if cached, ok := r.bundles[cacheKey]; ok {
			r.cacheHits++
			cached.CacheHit = true
			return cached, nil
		}
	}

	// Lookup the version to know which features are required.
	mv, err := r.getVersionLocked(modelID, version)
	if err != nil {
		return nil, fmt.Errorf("resolving features: %w", err)
	}

	bundle := &FeatureBundle{
		ModelID:    modelID,
		Version:    version,
		EntityKey:  entityKey,
		Features:   make(map[string]interface{}, len(mv.Features)),
		ResolvedAt: time.Now(),
		CacheHit:   false,
	}

	for _, feat := range mv.Features {
		if val, ok := featureValues[feat]; ok {
			bundle.Features[feat] = val
		} else {
			bundle.MissingFeatures = append(bundle.MissingFeatures, feat)
		}
	}

	if r.config.EnableCaching {
		r.bundles[cacheKey] = bundle
	}
	return bundle, nil
}

// getVersionLocked retrieves a version without acquiring locks (caller must hold lock).
func (r *Registry) getVersionLocked(modelID string, version int) (*ModelVersion, error) {
	versions, ok := r.versions[modelID]
	if !ok {
		return nil, fmt.Errorf("getting version %d of model %q: %w", version, modelID, ErrModelNotFound)
	}

	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}
	return nil, fmt.Errorf("getting version %d of model %q: %w", version, modelID, ErrVersionNotFound)
}

// Stats returns summary statistics for the registry.
func (r *Registry) Stats() *RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	totalVersions := 0
	for _, vs := range r.versions {
		totalVersions += len(vs)
	}

	var hitRate float64
	if r.cacheLookups > 0 {
		hitRate = float64(r.cacheHits) / float64(r.cacheLookups)
	}

	return &RegistryStats{
		ModelCount:   len(r.models),
		VersionCount: totalVersions,
		BundleCount:  len(r.bundles),
		CacheHitRate: hitRate,
	}
}
