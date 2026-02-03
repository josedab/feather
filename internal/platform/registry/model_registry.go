package registry

import (
	"fmt"
	"sync"
	"time"
)

// ModelBinding tracks which model uses which features.
type ModelBinding struct {
	ModelID      string    `json:"model_id"`
	ModelName    string    `json:"model_name"`
	ModelVersion string    `json:"model_version"`
	Features     []string  `json:"features"`
	Owner        string    `json:"owner,omitempty"`
	Environment  string    `json:"environment,omitempty"` // dev, staging, production
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// FeatureUsageByModel describes how models use a feature.
type FeatureUsageByModel struct {
	FeatureName string   `json:"feature_name"`
	Models      []string `json:"models"`
	ModelCount  int      `json:"model_count"`
	Environments []string `json:"environments"`
}

// BlastRadius describes the impact of changing or deprecating a feature.
type BlastRadius struct {
	FeatureName     string          `json:"feature_name"`
	AffectedModels  []ModelBinding  `json:"affected_models"`
	TotalModels     int             `json:"total_models"`
	ProductionModels int            `json:"production_models"`
	Severity        string          `json:"severity"` // low, medium, high, critical
}

// DeprecationNotice marks a feature for deprecation.
type DeprecationNotice struct {
	FeatureName  string    `json:"feature_name"`
	Reason       string    `json:"reason"`
	Replacement  string    `json:"replacement,omitempty"`
	DeprecatedAt time.Time `json:"deprecated_at"`
	RemoveBy     time.Time `json:"remove_by,omitempty"`
	AckedBy      []string  `json:"acked_by,omitempty"`
}

// ModelRegistry tracks feature-to-model relationships.
type ModelRegistry struct {
	mu            sync.RWMutex
	models        map[string]*ModelBinding
	deprecations  map[string]*DeprecationNotice
	featureToModels map[string]map[string]bool // feature -> set of model IDs
}

// NewModelRegistry creates a new model registry.
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		models:          make(map[string]*ModelBinding),
		deprecations:    make(map[string]*DeprecationNotice),
		featureToModels: make(map[string]map[string]bool),
	}
}

// RegisterModel binds a model to its feature dependencies.
func (mr *ModelRegistry) RegisterModel(binding ModelBinding) error {
	if binding.ModelID == "" || binding.ModelName == "" {
		return fmt.Errorf("model_id and model_name are required")
	}
	if len(binding.Features) == 0 {
		return fmt.Errorf("at least one feature is required")
	}

	mr.mu.Lock()
	defer mr.mu.Unlock()

	now := time.Now()
	binding.CreatedAt = now
	binding.UpdatedAt = now

	// Remove old feature bindings if model existed
	if old, exists := mr.models[binding.ModelID]; exists {
		for _, f := range old.Features {
			if m, ok := mr.featureToModels[f]; ok {
				delete(m, binding.ModelID)
			}
		}
	}

	mr.models[binding.ModelID] = &binding

	// Update reverse index
	for _, f := range binding.Features {
		if mr.featureToModels[f] == nil {
			mr.featureToModels[f] = make(map[string]bool)
		}
		mr.featureToModels[f][binding.ModelID] = true
	}

	return nil
}

// GetModel returns a model binding by ID.
func (mr *ModelRegistry) GetModel(modelID string) (*ModelBinding, error) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	m, exists := mr.models[modelID]
	if !exists {
		return nil, fmt.Errorf("model %s not found", modelID)
	}
	return m, nil
}

// ListModels returns all model bindings.
func (mr *ModelRegistry) ListModels() []ModelBinding {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	result := make([]ModelBinding, 0, len(mr.models))
	for _, m := range mr.models {
		result = append(result, *m)
	}
	return result
}

// RemoveModel removes a model binding.
func (mr *ModelRegistry) RemoveModel(modelID string) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	m, exists := mr.models[modelID]
	if !exists {
		return fmt.Errorf("model %s not found", modelID)
	}

	for _, f := range m.Features {
		if models, ok := mr.featureToModels[f]; ok {
			delete(models, modelID)
		}
	}
	delete(mr.models, modelID)
	return nil
}

// GetFeatureUsage returns which models use a specific feature.
func (mr *ModelRegistry) GetFeatureUsage(featureName string) *FeatureUsageByModel {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	modelIDs, exists := mr.featureToModels[featureName]
	if !exists || len(modelIDs) == 0 {
		return &FeatureUsageByModel{FeatureName: featureName}
	}

	usage := &FeatureUsageByModel{
		FeatureName: featureName,
		ModelCount:  len(modelIDs),
	}

	envSet := make(map[string]bool)
	for id := range modelIDs {
		if m, ok := mr.models[id]; ok {
			usage.Models = append(usage.Models, m.ModelName)
			if m.Environment != "" {
				envSet[m.Environment] = true
			}
		}
	}
	for env := range envSet {
		usage.Environments = append(usage.Environments, env)
	}
	return usage
}

// AnalyzeBlastRadius calculates the impact of changing a feature.
func (mr *ModelRegistry) AnalyzeBlastRadius(featureName string) *BlastRadius {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	br := &BlastRadius{FeatureName: featureName}

	modelIDs, exists := mr.featureToModels[featureName]
	if !exists {
		br.Severity = "low"
		return br
	}

	for id := range modelIDs {
		if m, ok := mr.models[id]; ok {
			br.AffectedModels = append(br.AffectedModels, *m)
			br.TotalModels++
			if m.Environment == "production" {
				br.ProductionModels++
			}
		}
	}

	// Determine severity
	switch {
	case br.ProductionModels >= 5:
		br.Severity = "critical"
	case br.ProductionModels >= 1:
		br.Severity = "high"
	case br.TotalModels >= 3:
		br.Severity = "medium"
	default:
		br.Severity = "low"
	}

	return br
}

// DeprecateFeature marks a feature for deprecation.
func (mr *ModelRegistry) DeprecateFeature(notice DeprecationNotice) error {
	if notice.FeatureName == "" {
		return fmt.Errorf("feature_name is required")
	}

	mr.mu.Lock()
	defer mr.mu.Unlock()

	notice.DeprecatedAt = time.Now()
	mr.deprecations[notice.FeatureName] = &notice
	return nil
}

// GetDeprecations returns all active deprecation notices.
func (mr *ModelRegistry) GetDeprecations() []DeprecationNotice {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	result := make([]DeprecationNotice, 0, len(mr.deprecations))
	for _, d := range mr.deprecations {
		result = append(result, *d)
	}
	return result
}

// AcknowledgeDeprecation records that a model owner has acknowledged a deprecation.
func (mr *ModelRegistry) AcknowledgeDeprecation(featureName, modelOwner string) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	d, exists := mr.deprecations[featureName]
	if !exists {
		return fmt.Errorf("no deprecation notice for feature %s", featureName)
	}
	d.AckedBy = append(d.AckedBy, modelOwner)
	return nil
}

// ModelRegistryStats provides aggregate statistics.
type ModelRegistryStats struct {
	TotalModels      int            `json:"total_models"`
	TotalFeatures    int            `json:"total_features_tracked"`
	TotalDeprecations int           `json:"total_deprecations"`
	ByEnvironment    map[string]int `json:"by_environment"`
}

// Stats returns aggregate registry statistics.
func (mr *ModelRegistry) Stats() ModelRegistryStats {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	stats := ModelRegistryStats{
		TotalModels:       len(mr.models),
		TotalFeatures:     len(mr.featureToModels),
		TotalDeprecations: len(mr.deprecations),
		ByEnvironment:     make(map[string]int),
	}
	for _, m := range mr.models {
		env := m.Environment
		if env == "" {
			env = "unknown"
		}
		stats.ByEnvironment[env]++
	}
	return stats
}
