package storage

import (
	"fmt"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

// Registry implements SchemaRegistry and stores feature group definitions.
type Registry struct {
	groups       map[string]*domain.FeatureGroup
	featureIndex map[string]*featureRef
	featureNames map[string]string
	mu           sync.RWMutex
}

type featureRef struct {
	group   *domain.FeatureGroup
	feature *domain.FeatureSpec
}

// NewRegistry creates a new schema registry.
func NewRegistry() *Registry {
	return &Registry{
		groups:       make(map[string]*domain.FeatureGroup),
		featureIndex: make(map[string]*featureRef),
		featureNames: make(map[string]string),
	}
}

// RegisterGroup registers a feature group.
func (r *Registry) RegisterGroup(group *domain.FeatureGroup) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.groups[group.Name]; exists {
		return fmt.Errorf("%w: group %s", domain.ErrAlreadyExists, group.Name)
	}

	if err := r.validateFeatureNames(group, ""); err != nil {
		return err
	}

	r.groups[group.Name] = group

	// Index features
	for i := range group.Features {
		featureName := group.Features[i].Name
		r.featureIndex[featureName] = &featureRef{
			group:   group,
			feature: &group.Features[i],
		}
		r.featureNames[featureName] = group.Name
	}

	return nil
}

// UpdateGroup updates a feature group.
func (r *Registry) UpdateGroup(group *domain.FeatureGroup) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.groups[group.Name]
	if !exists {
		return fmt.Errorf("%w: group %s", domain.ErrGroupNotFound, group.Name)
	}

	if err := r.validateFeatureNames(group, group.Name); err != nil {
		return err
	}

	// Remove old feature indexes
	for _, feature := range existing.Features {
		delete(r.featureIndex, feature.Name)
		delete(r.featureNames, feature.Name)
	}

	// Update group and re-index
	r.groups[group.Name] = group
	for i := range group.Features {
		featureName := group.Features[i].Name
		r.featureIndex[featureName] = &featureRef{
			group:   group,
			feature: &group.Features[i],
		}
		r.featureNames[featureName] = group.Name
	}

	return nil
}

// RemoveGroup removes a feature group.
func (r *Registry) RemoveGroup(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	group, exists := r.groups[name]
	if !exists {
		return fmt.Errorf("%w: group %s", domain.ErrGroupNotFound, name)
	}

	// Remove feature indexes
	for _, feature := range group.Features {
		delete(r.featureIndex, feature.Name)
		delete(r.featureNames, feature.Name)
	}

	delete(r.groups, name)
	return nil
}

// GetGroup returns a feature group by name.
func (r *Registry) GetGroup(name string) (*domain.FeatureGroup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	group, exists := r.groups[name]
	if !exists {
		return nil, fmt.Errorf("%w: group %s", domain.ErrGroupNotFound, name)
	}

	return group, nil
}

// GetFeatureSpec returns a feature specification by name.
func (r *Registry) GetFeatureSpec(featureName string) (*domain.FeatureSpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ref, exists := r.featureIndex[featureName]
	if !exists {
		return nil, fmt.Errorf("%w: feature %s", domain.ErrFeatureNotFound, featureName)
	}

	return ref.feature, nil
}

// GetFeatureGroup returns the group containing a feature.
func (r *Registry) GetFeatureGroup(featureName string) (*domain.FeatureGroup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ref, exists := r.featureIndex[featureName]
	if !exists {
		return nil, fmt.Errorf("%w: feature %s", domain.ErrFeatureNotFound, featureName)
	}

	return ref.group, nil
}

// ListGroups returns all registered groups.
func (r *Registry) ListGroups() []*domain.FeatureGroup {
	r.mu.RLock()
	defer r.mu.RUnlock()

	groups := make([]*domain.FeatureGroup, 0, len(r.groups))
	for _, group := range r.groups {
		groups = append(groups, group)
	}

	return groups
}

// ListFeatures returns all registered feature names.
func (r *Registry) ListFeatures() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	features := make([]string, 0, len(r.featureIndex))
	for name := range r.featureIndex {
		features = append(features, name)
	}

	return features
}

// ListEntityTypes returns all registered entity types.
func (r *Registry) ListEntityTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	types := make([]string, 0)
	for _, group := range r.groups {
		if !seen[group.EntityType] {
			seen[group.EntityType] = true
			types = append(types, group.EntityType)
		}
	}
	return types
}

// ListFeaturesForEntityType returns feature names available for a given entity type.
func (r *Registry) ListFeaturesForEntityType(entityType string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var features []string
	for _, group := range r.groups {
		if group.EntityType == entityType {
			for _, f := range group.Features {
				features = append(features, f.Name)
			}
		}
	}
	return features
}

// Validate validates a feature value against its schema.
func (r *Registry) Validate(featureName string, value interface{}) error {
	spec, err := r.GetFeatureSpec(featureName)
	if err != nil {
		return err
	}

	return validateValue(spec, value)
}

func validateValue(spec *domain.FeatureSpec, value interface{}) error {
	if spec.Validation == nil {
		return nil
	}

	v := spec.Validation

	if v.NotNull && value == nil {
		return &domain.ValidationError{
			Field:   spec.Name,
			Message: "value cannot be null",
		}
	}

	if value == nil {
		return nil
	}

	// Type-specific validation
	switch spec.DataType {
	case domain.DataTypeFloat64, domain.DataTypeInt64:
		var numVal float64
		switch n := value.(type) {
		case float64:
			numVal = n
		case int64:
			numVal = float64(n)
		case int:
			numVal = float64(n)
		default:
			return &domain.ValidationError{
				Field:   spec.Name,
				Message: "expected numeric value",
			}
		}

		if v.Min != nil && numVal < *v.Min {
			return &domain.ValidationError{
				Field:   spec.Name,
				Message: fmt.Sprintf("value %f is less than minimum %f", numVal, *v.Min),
			}
		}
		if v.Max != nil && numVal > *v.Max {
			return &domain.ValidationError{
				Field:   spec.Name,
				Message: fmt.Sprintf("value %f is greater than maximum %f", numVal, *v.Max),
			}
		}

	case domain.DataTypeString:
		strVal, ok := value.(string)
		if !ok {
			return &domain.ValidationError{
				Field:   spec.Name,
				Message: "expected string value",
			}
		}

		if len(v.OneOf) > 0 {
			found := false
			for _, allowed := range v.OneOf {
				if strVal == allowed {
					found = true
					break
				}
			}
			if !found {
				return &domain.ValidationError{
					Field:   spec.Name,
					Message: fmt.Sprintf("value %q not in allowed values", strVal),
				}
			}
		}
	case domain.DataTypeBool:
		if _, ok := value.(bool); !ok {
			return &domain.ValidationError{
				Field:   spec.Name,
				Message: "expected boolean value",
			}
		}
	case domain.DataTypeBytes:
		if _, ok := value.([]byte); !ok {
			return &domain.ValidationError{
				Field:   spec.Name,
				Message: "expected bytes value",
			}
		}
	case domain.DataTypeVector:
		if _, ok := value.([]float32); !ok {
			return &domain.ValidationError{
				Field:   spec.Name,
				Message: "expected vector value",
			}
		}
	case domain.DataTypeTimestamp:
		switch value.(type) {
		case time.Time, int64, string:
			return nil
		default:
			return &domain.ValidationError{
				Field:   spec.Name,
				Message: "expected timestamp value",
			}
		}
	}

	return nil
}

func (r *Registry) validateFeatureNames(group *domain.FeatureGroup, allowGroup string) error {
	for i := range group.Features {
		featureName := group.Features[i].Name
		if existingGroup, exists := r.featureNames[featureName]; exists && existingGroup != allowGroup {
			return fmt.Errorf("%w: feature %s already defined in group %s", domain.ErrAlreadyExists, featureName, existingGroup)
		}
	}
	return nil
}
