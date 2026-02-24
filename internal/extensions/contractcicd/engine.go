package contractcicd

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EngineConfig configures the contract CI/CD engine.
type EngineConfig struct {
	AllowBreakingChanges bool `json:"allow_breaking_changes" yaml:"allow_breaking_changes"`
	AutoMigrate          bool `json:"auto_migrate" yaml:"auto_migrate"`
	StrictMode           bool `json:"strict_mode" yaml:"strict_mode"`
}

// DefaultEngineConfig returns sensible defaults.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		AllowBreakingChanges: false,
		AutoMigrate:          false,
		StrictMode:           true,
	}
}

// Engine is the contract CI/CD engine that compares desired state (contracts)
// against current state and generates migration plans.
type Engine struct {
	config    EngineConfig
	contracts map[string]*Contract
	plans     map[string]*Plan
	mu        sync.RWMutex
	stats     EngineStats
}

// NewEngine creates a new contract CI/CD engine.
func NewEngine(cfg EngineConfig) *Engine {
	return &Engine{
		config:    cfg,
		contracts: make(map[string]*Contract),
		plans:     make(map[string]*Plan),
	}
}

// RegisterContract adds a contract to the engine.
func (e *Engine) RegisterContract(contract *Contract) error {
	if contract == nil || contract.Metadata.Name == "" {
		return ErrInvalidContract
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.contracts[contract.Metadata.Name]; exists {
		return ErrContractExists
	}
	contract.Metadata.CreatedAt = time.Now()
	e.contracts[contract.Metadata.Name] = contract
	e.stats.TotalContracts++
	return nil
}

// UpdateContract updates an existing contract and returns detected changes.
func (e *Engine) UpdateContract(contract *Contract) (*Plan, error) {
	if contract == nil || contract.Metadata.Name == "" {
		return nil, ErrInvalidContract
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, exists := e.contracts[contract.Metadata.Name]
	if !exists {
		return nil, ErrContractNotFound
	}

	changes := detectChanges(existing, contract)
	plan := buildPlan(changes)

	e.plans[plan.ID] = plan
	e.stats.TotalPlans++

	if !e.config.AllowBreakingChanges && plan.HasBreakingChanges() {
		e.stats.BreakingBlocked++
		return plan, ErrBreakingChange
	}

	e.contracts[contract.Metadata.Name] = contract
	return plan, nil
}

// GetContract returns a contract by name.
func (e *Engine) GetContract(name string) (*Contract, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	c, exists := e.contracts[name]
	if !exists {
		return nil, ErrContractNotFound
	}
	return c, nil
}

// ListContracts returns all registered contracts.
func (e *Engine) ListContracts() []*Contract {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*Contract, 0, len(e.contracts))
	for _, c := range e.contracts {
		result = append(result, c)
	}
	return result
}

// PlanFromContracts compares a set of contracts against current state.
func (e *Engine) PlanFromContracts(contracts []*Contract) (*Plan, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var allChanges []Change
	for _, contract := range contracts {
		existing, exists := e.contracts[contract.Metadata.Name]
		if !exists {
			// New contract — all features are additions.
			for _, f := range contract.Spec.Features {
				allChanges = append(allChanges, Change{
					Type:        ChangeTypeAdd,
					Severity:    SeverityInfo,
					Field:       f.Name,
					Description: fmt.Sprintf("new feature %q of type %s", f.Name, f.Type),
				})
			}
			continue
		}
		allChanges = append(allChanges, detectChanges(existing, contract)...)
	}
	return buildPlan(allChanges), nil
}

// Apply executes a plan, applying the recorded changes to contracts.
func (e *Engine) Apply(planID string) (*ApplyResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	plan, exists := e.plans[planID]
	if !exists {
		return nil, ErrPlanNotFound
	}

	if !e.config.AllowBreakingChanges && plan.HasBreakingChanges() {
		return nil, ErrBreakingChange
	}

	result := &ApplyResult{
		PlanID:    planID,
		AppliedAt: time.Now(),
	}

	for _, change := range plan.Changes {
		switch change.Type {
		case ChangeTypeAdd, ChangeTypeModify, ChangeTypeRename:
			result.Applied++
		case ChangeTypeRemove:
			if e.config.AllowBreakingChanges {
				result.Applied++
			} else {
				result.Skipped++
				result.Errors = append(result.Errors, fmt.Sprintf("skipped removal of %q (breaking)", change.Field))
			}
		default:
			result.Skipped++
		}
	}

	e.stats.TotalApplied++
	return result, nil
}

// DeprecateFeature marks a feature as deprecated in a contract.
func (e *Engine) DeprecateFeature(contractName, featureName, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	contract, exists := e.contracts[contractName]
	if !exists {
		return ErrContractNotFound
	}
	for i, f := range contract.Spec.Features {
		if f.Name == featureName {
			contract.Spec.Features[i].Deprecated = true
			contract.Spec.Features[i].Description = reason
			return nil
		}
	}
	return fmt.Errorf("feature %q not found in contract %q", featureName, contractName)
}

// DeleteContract removes a contract.
func (e *Engine) DeleteContract(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.contracts[name]; !exists {
		return ErrContractNotFound
	}
	delete(e.contracts, name)
	e.stats.TotalContracts--
	return nil
}

// DiffSummary returns a human-readable summary of a plan's changes.
func (e *Engine) DiffSummary(planID string) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	plan, exists := e.plans[planID]
	if !exists {
		return "", ErrPlanNotFound
	}

	var summary string
	for _, c := range plan.Changes {
		prefix := "  "
		switch c.Severity {
		case SeverityBreaking:
			prefix = "✗ "
		case SeverityWarning:
			prefix = "⚠ "
		}
		summary += fmt.Sprintf("%s[%s] %s: %s\n", prefix, c.Type, c.Field, c.Description)
	}
	if summary == "" {
		summary = "No changes detected.\n"
	}
	return fmt.Sprintf("Plan %s: %d additions, %d breaking, %d warnings\n%s",
		planID[:8], plan.Additions, plan.BreakingChanges, plan.Warnings, summary), nil
}

// GetPlan returns a plan by ID.
func (e *Engine) GetPlan(id string) (*Plan, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	p, exists := e.plans[id]
	if !exists {
		return nil, ErrPlanNotFound
	}
	return p, nil
}

// GenerateCITemplate generates CI/CD configuration for the given provider.
func (e *Engine) GenerateCITemplate(provider string) CITemplate {
	switch provider {
	case "github":
		return CITemplate{
			Provider: "github",
			Content:  generateGitHubAction(),
		}
	case "gitlab":
		return CITemplate{
			Provider: "gitlab",
			Content:  generateGitLabCI(),
		}
	default:
		return CITemplate{
			Provider: "generic",
			Content:  generateGenericCI(),
		}
	}
}

// Stats returns engine statistics.
func (e *Engine) Stats() EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// Validate checks a contract for correctness without registering it.
func (e *Engine) Validate(contract *Contract) []string {
	var errs []string
	if contract.Metadata.Name == "" {
		errs = append(errs, "metadata.name is required")
	}
	if contract.Spec.EntityType == "" {
		errs = append(errs, "spec.entity_type is required")
	}
	if len(contract.Spec.Features) == 0 {
		errs = append(errs, "spec.features must not be empty")
	}
	validTypes := map[string]bool{
		"int64": true, "float64": true, "string": true, "bool": true,
		"bytes": true, "vector": true, "timestamp": true,
	}
	for _, f := range contract.Spec.Features {
		if f.Name == "" {
			errs = append(errs, "feature name is required")
		}
		if !validTypes[f.Type] {
			errs = append(errs, fmt.Sprintf("feature %q: invalid type %q", f.Name, f.Type))
		}
	}
	return errs
}

func detectChanges(old, new *Contract) []Change {
	var changes []Change
	oldFeatures := make(map[string]FeatureContract)
	for _, f := range old.Spec.Features {
		oldFeatures[f.Name] = f
	}
	newFeatures := make(map[string]FeatureContract)
	for _, f := range new.Spec.Features {
		newFeatures[f.Name] = f
	}

	// Detect removed features (breaking).
	for name := range oldFeatures {
		if _, exists := newFeatures[name]; !exists {
			changes = append(changes, Change{
				Type:        ChangeTypeRemove,
				Severity:    SeverityBreaking,
				Field:       name,
				Description: fmt.Sprintf("feature %q removed", name),
			})
		}
	}

	// Detect added features (info).
	for name, f := range newFeatures {
		if _, exists := oldFeatures[name]; !exists {
			changes = append(changes, Change{
				Type:        ChangeTypeAdd,
				Severity:    SeverityInfo,
				Field:       name,
				Description: fmt.Sprintf("feature %q added with type %s", name, f.Type),
			})
		}
	}

	// Detect type changes (breaking) and deprecation (warning).
	for name, newF := range newFeatures {
		if oldF, exists := oldFeatures[name]; exists {
			if oldF.Type != newF.Type {
				changes = append(changes, Change{
					Type:        ChangeTypeModify,
					Severity:    SeverityBreaking,
					Field:       name,
					Description: fmt.Sprintf("feature %q type changed from %s to %s", name, oldF.Type, newF.Type),
					OldValue:    oldF.Type,
					NewValue:    newF.Type,
				})
			} else if oldF.Required != newF.Required && newF.Required {
				changes = append(changes, Change{
					Type:        ChangeTypeModify,
					Severity:    SeverityWarning,
					Field:       name,
					Description: fmt.Sprintf("feature %q changed to required", name),
				})
			}
			if !oldF.Deprecated && newF.Deprecated {
				changes = append(changes, Change{
					Type:        ChangeTypeModify,
					Severity:    SeverityWarning,
					Field:       name,
					Description: fmt.Sprintf("feature %q deprecated", name),
				})
			}
		}
	}

	return changes
}

func buildPlan(changes []Change) *Plan {
	plan := &Plan{
		ID:        uuid.New().String(),
		Changes:   changes,
		CreatedAt: time.Now(),
	}
	for _, c := range changes {
		switch c.Severity {
		case SeverityBreaking:
			plan.BreakingChanges++
		case SeverityWarning:
			plan.Warnings++
		}
		if c.Type == ChangeTypeAdd {
			plan.Additions++
		}
	}
	// Generate migration steps for each change.
	plan.Migrations = generateMigrations(changes)
	return plan
}

func generateMigrations(changes []Change) []Migration {
	var migrations []Migration
	for _, c := range changes {
		var m Migration
		switch c.Type {
		case ChangeTypeAdd:
			m = Migration{
				ID:          fmt.Sprintf("add_%s", c.Field),
				Description: fmt.Sprintf("Add feature column %q", c.Field),
				SQL:         fmt.Sprintf("ALTER TABLE features ADD COLUMN %s %s;", c.Field, c.NewValue),
				Reversible:  true,
			}
		case ChangeTypeRemove:
			m = Migration{
				ID:          fmt.Sprintf("drop_%s", c.Field),
				Description: fmt.Sprintf("Drop feature column %q", c.Field),
				SQL:         fmt.Sprintf("ALTER TABLE features DROP COLUMN %s;", c.Field),
				Reversible:  false,
			}
		case ChangeTypeModify:
			if c.OldValue != "" && c.NewValue != "" {
				m = Migration{
					ID:          fmt.Sprintf("alter_%s", c.Field),
					Description: fmt.Sprintf("Change %q type from %s to %s", c.Field, c.OldValue, c.NewValue),
					SQL:         fmt.Sprintf("ALTER TABLE features ALTER COLUMN %s TYPE %s;", c.Field, c.NewValue),
					Reversible:  false,
				}
			} else {
				continue
			}
		default:
			continue
		}
		migrations = append(migrations, m)
	}
	return migrations
}

func generateGitHubAction() string {
	return `name: Feather Contract Check
on:
  pull_request:
    paths:
      - 'contracts/**'
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Validate contracts
        run: feather contract validate contracts/
      - name: Plan changes
        run: feather contract plan contracts/
      - name: Check for breaking changes
        run: feather contract check --fail-on-breaking contracts/
`
}

func generateGitLabCI() string {
	return `contract-check:
  stage: validate
  script:
    - feather contract validate contracts/
    - feather contract plan contracts/
    - feather contract check --fail-on-breaking contracts/
  rules:
    - changes:
        - contracts/**
`
}

func generateGenericCI() string {
	return `#!/bin/bash
set -e
feather contract validate contracts/
feather contract plan contracts/
feather contract check --fail-on-breaking contracts/
`
}
