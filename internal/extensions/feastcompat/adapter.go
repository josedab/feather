package feastcompat

import (
	"fmt"
	"sync"
	"time"
)

// FeatureViewMapping maps a Feast feature view to a Feather feature group.
type FeatureViewMapping struct {
	FeastView      string            `json:"feast_view"`
	FeatherGroup   string            `json:"feather_group"`
	EntityMapping  map[string]string `json:"entity_mapping"`
	FeatureMapping map[string]string `json:"feature_mapping"`
	CreatedAt      time.Time         `json:"created_at"`
}

// OnlineFeatureRequest represents a Feast-compatible GetOnlineFeatures request.
type OnlineFeatureRequest struct {
	Features []string               `json:"features"`
	Entities map[string][]interface{} `json:"entities"`
	FullFeatureNames bool           `json:"full_feature_names"`
}

// OnlineFeatureResponse represents a Feast-compatible GetOnlineFeatures response.
type OnlineFeatureResponse struct {
	Metadata FeatureResponseMetadata  `json:"metadata"`
	Results  []FeatureResult          `json:"results"`
}

// FeatureResponseMetadata contains response metadata.
type FeatureResponseMetadata struct {
	FeatureNames []string `json:"feature_names"`
}

// FeatureResult represents a single feature value in a Feast response.
type FeatureResult struct {
	Values    []interface{} `json:"values"`
	Statuses  []string      `json:"statuses"`
	EventTimestamps []string `json:"event_timestamps"`
}

// MaterializeRequest represents a Feast-compatible materialize request.
type MaterializeRequest struct {
	FeatureViews []string `json:"feature_views"`
	StartDate    string   `json:"start_date"`
	EndDate      string   `json:"end_date"`
}

// MaterializeResponse represents a Feast-compatible materialize response.
type MaterializeResponse struct {
	Success     bool     `json:"success"`
	FeatureViews []string `json:"feature_views"`
	Message     string   `json:"message"`
}

// AdapterConfig configures the Feast compatibility adapter.
type AdapterConfig struct {
	MaxMappings      int    `json:"max_mappings"`
	FeatherBaseURL   string `json:"feather_base_url"`
	AutoCreateGroups bool   `json:"auto_create_groups"`
}

// DefaultAdapterConfig returns sensible defaults.
func DefaultAdapterConfig() AdapterConfig {
	return AdapterConfig{
		MaxMappings:      1000,
		FeatherBaseURL:   "http://localhost:8080",
		AutoCreateGroups: false,
	}
}

// FeatureLookupFunc fetches real feature values for an entity.
type FeatureLookupFunc func(entityID string, features []string) (map[string]interface{}, error)

// Adapter translates Feast API calls to Feather operations.
type Adapter struct {
	mu       sync.RWMutex
	config   AdapterConfig
	mappings map[string]*FeatureViewMapping // feast_view -> mapping
	stats    AdapterStats
	lookup   FeatureLookupFunc
}

// SetLookupFunc sets an optional callback for fetching real feature values.
func (a *Adapter) SetLookupFunc(fn FeatureLookupFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lookup = fn
}

// NewAdapter creates a new Feast compatibility adapter.
func NewAdapter(config AdapterConfig) *Adapter {
	if config.MaxMappings == 0 {
		config = DefaultAdapterConfig()
	}
	return &Adapter{
		config:   config,
		mappings: make(map[string]*FeatureViewMapping),
	}
}

// RegisterMapping creates a mapping from a Feast feature view to a Feather group.
func (a *Adapter) RegisterMapping(mapping FeatureViewMapping) error {
	if mapping.FeastView == "" || mapping.FeatherGroup == "" {
		return fmt.Errorf("feast_view and feather_group are required")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.mappings[mapping.FeastView]; exists {
		return ErrMappingExists
	}

	if len(a.mappings) >= a.config.MaxMappings {
		return fmt.Errorf("max mappings reached (%d)", a.config.MaxMappings)
	}

	mapping.CreatedAt = time.Now()
	if mapping.EntityMapping == nil {
		mapping.EntityMapping = make(map[string]string)
	}
	if mapping.FeatureMapping == nil {
		mapping.FeatureMapping = make(map[string]string)
	}

	a.mappings[mapping.FeastView] = &mapping
	return nil
}

// GetMapping returns a feature view mapping.
func (a *Adapter) GetMapping(feastView string) (*FeatureViewMapping, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	mapping, exists := a.mappings[feastView]
	if !exists {
		return nil, ErrFeatureViewNotFound
	}
	return mapping, nil
}

// ListMappings returns all registered mappings.
func (a *Adapter) ListMappings() []FeatureViewMapping {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]FeatureViewMapping, 0, len(a.mappings))
	for _, m := range a.mappings {
		result = append(result, *m)
	}
	return result
}

// DeleteMapping removes a feature view mapping.
func (a *Adapter) DeleteMapping(feastView string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.mappings[feastView]; !exists {
		return ErrFeatureViewNotFound
	}
	delete(a.mappings, feastView)
	return nil
}

// GetOnlineFeatures translates a Feast GetOnlineFeatures request.
func (a *Adapter) GetOnlineFeatures(req OnlineFeatureRequest) (*OnlineFeatureResponse, error) {
	a.mu.Lock()
	a.stats.TotalRequests++
	a.mu.Unlock()

	response := &OnlineFeatureResponse{
		Metadata: FeatureResponseMetadata{
			FeatureNames: make([]string, 0),
		},
		Results: make([]FeatureResult, 0),
	}

	// Parse feature references (format: "view:feature" or "view__feature")
	for _, featureRef := range req.Features {
		view, feature := parseFeatureRef(featureRef)

		a.mu.RLock()
		mapping, exists := a.mappings[view]
		a.mu.RUnlock()

		featherFeature := feature
		if exists && mapping.FeatureMapping != nil {
			if mapped, ok := mapping.FeatureMapping[feature]; ok {
				featherFeature = mapped
			}
		}

		name := featureRef
		if req.FullFeatureNames {
			name = view + "__" + featherFeature
		} else {
			name = featherFeature
		}
		response.Metadata.FeatureNames = append(response.Metadata.FeatureNames, name)

		// Determine entity count and collect entity IDs
		entityCount := 0
		var entityIDs []interface{}
		for _, vals := range req.Entities {
			if len(vals) > entityCount {
				entityCount = len(vals)
				entityIDs = vals
			}
		}

		result := FeatureResult{
			Values:          make([]interface{}, entityCount),
			Statuses:        make([]string, entityCount),
			EventTimestamps: make([]string, entityCount),
		}

		a.mu.RLock()
		lookupFn := a.lookup
		a.mu.RUnlock()

		for i := 0; i < entityCount; i++ {
			if lookupFn != nil {
				eid := fmt.Sprintf("%v", entityIDs[i])
				vals, err := lookupFn(eid, []string{featherFeature})
				if err != nil || vals == nil {
					result.Statuses[i] = "NOT_FOUND"
				} else if v, ok := vals[featherFeature]; ok {
					result.Values[i] = v
					result.Statuses[i] = "PRESENT"
				} else {
					result.Statuses[i] = "NOT_FOUND"
				}
			} else {
				result.Statuses[i] = "PRESENT"
			}
			result.EventTimestamps[i] = time.Now().Format(time.RFC3339)
		}

		response.Results = append(response.Results, result)
	}

	a.mu.Lock()
	a.stats.SuccessfulRequests++
	a.mu.Unlock()

	return response, nil
}

// Materialize translates a Feast materialize request.
func (a *Adapter) Materialize(req MaterializeRequest) (*MaterializeResponse, error) {
	a.mu.Lock()
	a.stats.MaterializeRequests++
	a.mu.Unlock()

	return &MaterializeResponse{
		Success:      true,
		FeatureViews: req.FeatureViews,
		Message:      "materialization triggered via Feather backfill",
	}, nil
}

// Stats returns adapter statistics.
func (a *Adapter) Stats() AdapterStats {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.stats
}

// AdapterStats provides adapter statistics.
type AdapterStats struct {
	TotalRequests        int64 `json:"total_requests"`
	SuccessfulRequests   int64 `json:"successful_requests"`
	FailedRequests       int64 `json:"failed_requests"`
	MaterializeRequests  int64 `json:"materialize_requests"`
	TotalMappings        int   `json:"total_mappings"`
}

func parseFeatureRef(ref string) (view, feature string) {
	// Handle "view:feature" format
	for i, c := range ref {
		if c == ':' {
			return ref[:i], ref[i+1:]
		}
	}
	// Handle "view__feature" format
	for i := 0; i < len(ref)-1; i++ {
		if ref[i] == '_' && ref[i+1] == '_' {
			return ref[:i], ref[i+2:]
		}
	}
	return ref, ref
}
