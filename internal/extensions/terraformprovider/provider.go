package terraformprovider

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ResourceType identifies the kind of Terraform-managed resource.
type ResourceType string

const (
	ResourceFeatureGroup ResourceType = "feather_feature_group"
	ResourceSchema       ResourceType = "feather_schema"
	ResourceSLA          ResourceType = "feather_sla"
	ResourceContract     ResourceType = "feather_contract"
)

// ResourceState represents the current state of a managed resource.
type ResourceState struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Attributes map[string]interface{} `json:"attributes"`
	Version    int                    `json:"version"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// PlanAction describes the action to take on a resource.
type PlanAction string

const (
	PlanCreate PlanAction = "create"
	PlanUpdate PlanAction = "update"
	PlanDelete PlanAction = "delete"
	PlanNoOp   PlanAction = "no-op"
)

// PlanResult describes the planned change for a single resource.
type PlanResult struct {
	ResourceID string                 `json:"resource_id"`
	Action     PlanAction             `json:"action"`
	Before     map[string]interface{} `json:"before,omitempty"`
	After      map[string]interface{} `json:"after,omitempty"`
	Changes    []string               `json:"changes,omitempty"`
}

// ApplyResult describes the outcome of applying a planned change.
type ApplyResult struct {
	ResourceID string         `json:"resource_id"`
	Action     PlanAction     `json:"action"`
	Success    bool           `json:"success"`
	Error      string         `json:"error,omitempty"`
	State      *ResourceState `json:"state,omitempty"`
}

// ProviderConfig configures the Terraform provider.
type ProviderConfig struct {
	ServerURL    string `json:"server_url"`
	APIKey       string `json:"api_key"`
	MaxResources int    `json:"max_resources"`
}

// DefaultProviderConfig returns sensible defaults.
func DefaultProviderConfig() ProviderConfig {
	return ProviderConfig{
		ServerURL:    "http://localhost:8080",
		MaxResources: 100000,
	}
}

// ProviderStats holds provider statistics.
type ProviderStats struct {
	TotalResources  int            `json:"total_resources"`
	ResourcesByType map[string]int `json:"resources_by_type"`
	TotalPlans      int64          `json:"total_plans"`
	TotalApplies    int64          `json:"total_applies"`
}

// Provider manages Feather resources as Terraform infrastructure.
type Provider struct {
	mu           sync.RWMutex
	config       ProviderConfig
	resources    map[string]*ResourceState
	planResults  []PlanResult
	applyResults []ApplyResult
	totalPlans   atomic.Int64
	totalApplies atomic.Int64
}

// NewProvider creates a new Terraform provider.
func NewProvider(config ProviderConfig) *Provider {
	return &Provider{
		config:       config,
		resources:    make(map[string]*ResourceState),
		planResults:  make([]PlanResult, 0),
		applyResults: make([]ApplyResult, 0),
	}
}

// CreateResource creates a new managed resource.
func (p *Provider) CreateResource(resType ResourceType, id string, attrs map[string]interface{}) (*ResourceState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.resources[id]; exists {
		return nil, fmt.Errorf("resource %s: %w", id, ErrResourceExists)
	}
	if len(p.resources) >= p.config.MaxResources {
		return nil, fmt.Errorf("max resources (%d) reached", p.config.MaxResources)
	}

	now := time.Now()
	state := &ResourceState{
		ID:         id,
		Type:       string(resType),
		Attributes: attrs,
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	p.resources[id] = state

	copy := *state
	return &copy, nil
}

// ReadResource returns the current state of a resource.
func (p *Provider) ReadResource(id string) (*ResourceState, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	res, exists := p.resources[id]
	if !exists {
		return nil, fmt.Errorf("resource %s: %w", id, ErrResourceNotFound)
	}

	copy := *res
	return &copy, nil
}

// UpdateResource updates a resource's attributes and increments its version.
func (p *Provider) UpdateResource(id string, attrs map[string]interface{}) (*ResourceState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	res, exists := p.resources[id]
	if !exists {
		return nil, fmt.Errorf("resource %s: %w", id, ErrResourceNotFound)
	}

	for k, v := range attrs {
		res.Attributes[k] = v
	}
	res.Version++
	res.UpdatedAt = time.Now()

	copy := *res
	return &copy, nil
}

// DeleteResource removes a resource by ID.
func (p *Provider) DeleteResource(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.resources[id]; !exists {
		return fmt.Errorf("resource %s: %w", id, ErrResourceNotFound)
	}

	delete(p.resources, id)
	return nil
}

// ListResources returns all resources of the given type.
func (p *Provider) ListResources(resType ResourceType) []ResourceState {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var results []ResourceState
	for _, res := range p.resources {
		if res.Type == string(resType) {
			results = append(results, *res)
		}
	}
	return results
}

// Plan compares desired state against current state and produces a plan.
func (p *Provider) Plan(desired []ResourceState) []PlanResult {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var results []PlanResult
	seen := make(map[string]bool)

	for _, d := range desired {
		seen[d.ID] = true
		existing, exists := p.resources[d.ID]

		if !exists {
			results = append(results, PlanResult{
				ResourceID: d.ID,
				Action:     PlanCreate,
				After:      d.Attributes,
			})
			continue
		}

		changes := diffAttributes(existing.Attributes, d.Attributes)
		if len(changes) > 0 {
			results = append(results, PlanResult{
				ResourceID: d.ID,
				Action:     PlanUpdate,
				Before:     existing.Attributes,
				After:      d.Attributes,
				Changes:    changes,
			})
		} else {
			results = append(results, PlanResult{
				ResourceID: d.ID,
				Action:     PlanNoOp,
			})
		}
	}

	// Resources that exist but are not in desired state should be deleted
	for id := range p.resources {
		if !seen[id] {
			results = append(results, PlanResult{
				ResourceID: id,
				Action:     PlanDelete,
				Before:     p.resources[id].Attributes,
			})
		}
	}

	p.totalPlans.Add(1)
	return results
}

// Apply executes the plan and returns the results.
func (p *Provider) Apply(plan []PlanResult) []ApplyResult {
	var results []ApplyResult

	for _, pr := range plan {
		switch pr.Action {
		case PlanCreate:
			state, err := p.CreateResource(ResourceFeatureGroup, pr.ResourceID, pr.After)
			if err != nil {
				results = append(results, ApplyResult{
					ResourceID: pr.ResourceID, Action: pr.Action, Error: err.Error(),
				})
			} else {
				results = append(results, ApplyResult{
					ResourceID: pr.ResourceID, Action: pr.Action, Success: true, State: state,
				})
			}

		case PlanUpdate:
			state, err := p.UpdateResource(pr.ResourceID, pr.After)
			if err != nil {
				results = append(results, ApplyResult{
					ResourceID: pr.ResourceID, Action: pr.Action, Error: err.Error(),
				})
			} else {
				results = append(results, ApplyResult{
					ResourceID: pr.ResourceID, Action: pr.Action, Success: true, State: state,
				})
			}

		case PlanDelete:
			err := p.DeleteResource(pr.ResourceID)
			if err != nil {
				results = append(results, ApplyResult{
					ResourceID: pr.ResourceID, Action: pr.Action, Error: err.Error(),
				})
			} else {
				results = append(results, ApplyResult{
					ResourceID: pr.ResourceID, Action: pr.Action, Success: true,
				})
			}

		case PlanNoOp:
			results = append(results, ApplyResult{
				ResourceID: pr.ResourceID, Action: pr.Action, Success: true,
			})
		}
	}

	p.totalApplies.Add(1)
	return results
}

// ImportResource imports an existing resource into the provider state.
func (p *Provider) ImportResource(resType ResourceType, id string) (*ResourceState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.resources[id]; exists {
		return nil, fmt.Errorf("resource %s: %w", id, ErrResourceExists)
	}

	now := time.Now()
	state := &ResourceState{
		ID:         id,
		Type:       string(resType),
		Attributes: make(map[string]interface{}),
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	p.resources[id] = state

	copy := *state
	return &copy, nil
}

// Stats returns provider statistics.
func (p *Provider) Stats() ProviderStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	byType := make(map[string]int)
	for _, res := range p.resources {
		byType[res.Type]++
	}

	return ProviderStats{
		TotalResources:  len(p.resources),
		ResourcesByType: byType,
		TotalPlans:      p.totalPlans.Load(),
		TotalApplies:    p.totalApplies.Load(),
	}
}

func diffAttributes(before, after map[string]interface{}) []string {
	var changes []string
	for k, v := range after {
		if bv, ok := before[k]; !ok || fmt.Sprintf("%v", bv) != fmt.Sprintf("%v", v) {
			changes = append(changes, k)
		}
	}
	return changes
}
