// Package ml provides ML model integration for Feather feature store.
// This file implements the Model Registry which tracks model-feature mappings
// for training-serving consistency validation.
package ml

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Model Registry errors.
var (
	ErrModelNotFound      = errors.New("model not found")
	ErrModelExists        = errors.New("model already exists")
	ErrModelVersionExists = errors.New("model version already exists")
	ErrNoActiveVersion    = errors.New("no active model version")
	ErrFeatureSetEmpty    = errors.New("feature set cannot be empty")
)

// ModelStatus represents the lifecycle status of a model.
type ModelStatus string

// ModelStatus constants for model lifecycle.
const (
	ModelStatusDraft      ModelStatus = "draft"
	ModelStatusStaging    ModelStatus = "staging"
	ModelStatusProduction ModelStatus = "production"
	ModelStatusArchived   ModelStatus = "archived"
)

// Model represents a registered ML model with its feature dependencies.
type Model struct {
	// ID is the unique identifier for the model
	ID string `json:"id"`
	// Name is the human-readable model name
	Name string `json:"name"`
	// Description provides context about the model's purpose
	Description string `json:"description"`
	// Team is the owning team for the model
	Team string `json:"team"`
	// Owner is the individual owner/maintainer
	Owner string `json:"owner"`
	// Tags for categorization and search
	Tags []string `json:"tags"`
	// Metadata for custom key-value pairs
	Metadata map[string]string `json:"metadata"`
	// ActiveVersion is the currently serving version
	ActiveVersion string `json:"active_version"`
	// Versions contains all registered versions of this model
	Versions map[string]*ModelVersion `json:"versions"`
	// CreatedAt is the model registration timestamp
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the last modification timestamp
	UpdatedAt time.Time `json:"updated_at"`
}

// ModelVersion represents a specific version of a model with its feature set.
type ModelVersion struct {
	// Version is the version identifier (e.g., "v1.0.0", "2024-01-15")
	Version string `json:"version"`
	// Status is the lifecycle status of this version
	Status ModelStatus `json:"status"`
	// Features is the list of feature names required by this model version
	Features []string `json:"features"`
	// FeatureSet is a set for O(1) feature lookup
	FeatureSet map[string]bool `json:"-"`
	// TrainingSnapshot references the training-time feature distribution snapshot
	TrainingSnapshotID string `json:"training_snapshot_id,omitempty"`
	// ServingEndpoint is the inference endpoint for this version
	ServingEndpoint string `json:"serving_endpoint,omitempty"`
	// Description provides version-specific notes
	Description string `json:"description"`
	// Metadata for version-specific key-value pairs
	Metadata map[string]string `json:"metadata"`
	// CreatedAt is when this version was registered
	CreatedAt time.Time `json:"created_at"`
	// ActivatedAt is when this version was promoted to production
	ActivatedAt *time.Time `json:"activated_at,omitempty"`
	// ArchivedAt is when this version was archived
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
}

// ModelRegistry manages registered ML models and their feature dependencies.
type ModelRegistry struct {
	mu sync.RWMutex
	// models maps model ID to Model
	models map[string]*Model
	// modelsByName maps model name to model ID for lookup
	modelsByName map[string]string
	// featureToModels maps feature name to list of model IDs that use it
	featureToModels map[string][]string
	// callbacks for model lifecycle events
	onModelRegistered    []func(*Model)
	onVersionActivated   []func(*Model, *ModelVersion)
	onVersionDeactivated []func(*Model, *ModelVersion)
}

// NewModelRegistry creates a new model registry.
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		models:          make(map[string]*Model),
		modelsByName:    make(map[string]string),
		featureToModels: make(map[string][]string),
	}
}

// RegisterModel registers a new model in the registry.
func (r *ModelRegistry) RegisterModel(model *Model) error {
	if model.ID == "" {
		return errors.New("model ID is required")
	}
	if model.Name == "" {
		return errors.New("model name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if model already exists
	if _, exists := r.models[model.ID]; exists {
		return fmt.Errorf("%w: %s", ErrModelExists, model.ID)
	}
	if existingID, exists := r.modelsByName[model.Name]; exists {
		return fmt.Errorf("%w: name '%s' already used by model %s", ErrModelExists, model.Name, existingID)
	}

	now := time.Now()
	model.CreatedAt = now
	model.UpdatedAt = now

	if model.Versions == nil {
		model.Versions = make(map[string]*ModelVersion)
	}
	if model.Metadata == nil {
		model.Metadata = make(map[string]string)
	}

	r.models[model.ID] = model
	r.modelsByName[model.Name] = model.ID

	// Notify callbacks
	for _, cb := range r.onModelRegistered {
		cb(model)
	}

	return nil
}

// RegisterVersion registers a new version for an existing model.
func (r *ModelRegistry) RegisterVersion(modelID string, version *ModelVersion) error {
	if version.Version == "" {
		return errors.New("version is required")
	}
	if len(version.Features) == 0 {
		return ErrFeatureSetEmpty
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	model, exists := r.models[modelID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrModelNotFound, modelID)
	}

	if _, exists := model.Versions[version.Version]; exists {
		return fmt.Errorf("%w: %s", ErrModelVersionExists, version.Version)
	}

	// Build feature set for O(1) lookup
	version.FeatureSet = make(map[string]bool, len(version.Features))
	for _, f := range version.Features {
		version.FeatureSet[f] = true
	}

	if version.Status == "" {
		version.Status = ModelStatusDraft
	}
	version.CreatedAt = time.Now()

	if version.Metadata == nil {
		version.Metadata = make(map[string]string)
	}

	model.Versions[version.Version] = version
	model.UpdatedAt = time.Now()

	// Update feature-to-model index
	for _, feature := range version.Features {
		r.featureToModels[feature] = appendUnique(r.featureToModels[feature], modelID)
	}

	return nil
}

// ActivateVersion promotes a version to production status.
func (r *ModelRegistry) ActivateVersion(modelID, version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	model, exists := r.models[modelID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrModelNotFound, modelID)
	}

	ver, exists := model.Versions[version]
	if !exists {
		return fmt.Errorf("version not found: %s", version)
	}

	// Deactivate current active version
	if model.ActiveVersion != "" && model.ActiveVersion != version {
		if prevVer, ok := model.Versions[model.ActiveVersion]; ok {
			prevVer.Status = ModelStatusArchived
			now := time.Now()
			prevVer.ArchivedAt = &now

			for _, cb := range r.onVersionDeactivated {
				cb(model, prevVer)
			}
		}
	}

	// Activate new version
	ver.Status = ModelStatusProduction
	now := time.Now()
	ver.ActivatedAt = &now
	model.ActiveVersion = version
	model.UpdatedAt = now

	// Notify callbacks
	for _, cb := range r.onVersionActivated {
		cb(model, ver)
	}

	return nil
}

// GetModel returns a model by ID.
func (r *ModelRegistry) GetModel(modelID string) (*Model, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, exists := r.models[modelID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrModelNotFound, modelID)
	}
	return model, nil
}

// GetModelByName returns a model by name.
func (r *ModelRegistry) GetModelByName(name string) (*Model, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	modelID, exists := r.modelsByName[name]
	if !exists {
		return nil, fmt.Errorf("%w: name '%s'", ErrModelNotFound, name)
	}
	return r.models[modelID], nil
}

// GetVersion returns a specific version of a model.
func (r *ModelRegistry) GetVersion(modelID, version string) (*ModelVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, exists := r.models[modelID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrModelNotFound, modelID)
	}

	ver, exists := model.Versions[version]
	if !exists {
		return nil, fmt.Errorf("version not found: %s", version)
	}
	return ver, nil
}

// GetActiveVersion returns the active version of a model.
func (r *ModelRegistry) GetActiveVersion(modelID string) (*ModelVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, exists := r.models[modelID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrModelNotFound, modelID)
	}

	if model.ActiveVersion == "" {
		return nil, fmt.Errorf("%w: %s", ErrNoActiveVersion, modelID)
	}

	return model.Versions[model.ActiveVersion], nil
}

// GetFeaturesForModel returns the feature set for a model's active version.
func (r *ModelRegistry) GetFeaturesForModel(modelID string) ([]string, error) {
	ver, err := r.GetActiveVersion(modelID)
	if err != nil {
		return nil, err
	}
	return ver.Features, nil
}

// GetModelsUsingFeature returns all models that depend on a given feature.
func (r *ModelRegistry) GetModelsUsingFeature(featureName string) []*Model {
	r.mu.RLock()
	defer r.mu.RUnlock()

	modelIDs := r.featureToModels[featureName]
	models := make([]*Model, 0, len(modelIDs))
	for _, id := range modelIDs {
		if model, ok := r.models[id]; ok {
			models = append(models, model)
		}
	}
	return models
}

// ListModels returns all registered models.
func (r *ModelRegistry) ListModels() []*Model {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]*Model, 0, len(r.models))
	for _, model := range r.models {
		models = append(models, model)
	}
	return models
}

// DeleteModel removes a model from the registry.
func (r *ModelRegistry) DeleteModel(modelID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	model, exists := r.models[modelID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrModelNotFound, modelID)
	}

	// Remove from feature index
	for _, ver := range model.Versions {
		for _, feature := range ver.Features {
			r.featureToModels[feature] = removeFromSlice(r.featureToModels[feature], modelID)
		}
	}

	delete(r.modelsByName, model.Name)
	delete(r.models, modelID)

	return nil
}

// OnModelRegistered registers a callback for model registration events.
func (r *ModelRegistry) OnModelRegistered(cb func(*Model)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onModelRegistered = append(r.onModelRegistered, cb)
}

// OnVersionActivated registers a callback for version activation events.
func (r *ModelRegistry) OnVersionActivated(cb func(*Model, *ModelVersion)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onVersionActivated = append(r.onVersionActivated, cb)
}

// OnVersionDeactivated registers a callback for version deactivation events.
func (r *ModelRegistry) OnVersionDeactivated(cb func(*Model, *ModelVersion)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onVersionDeactivated = append(r.onVersionDeactivated, cb)
}

// Stats returns registry statistics.
func (r *ModelRegistry) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	totalVersions := 0
	activeModels := 0
	for _, model := range r.models {
		totalVersions += len(model.Versions)
		if model.ActiveVersion != "" {
			activeModels++
		}
	}

	return map[string]interface{}{
		"total_models":     len(r.models),
		"active_models":    activeModels,
		"total_versions":   totalVersions,
		"indexed_features": len(r.featureToModels),
	}
}

// MarshalJSON implements json.Marshaler.
func (r *ModelRegistry) MarshalJSON() ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return json.Marshal(map[string]interface{}{
		"models": r.models,
	})
}

// Helper functions

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func removeFromSlice(slice []string, item string) []string {
	for i, s := range slice {
		if s == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}
