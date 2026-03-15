package gitopsdefs

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// FeatureDefinition describes a desired feature group.
type FeatureDefinition struct {
	Name        string            `json:"name"`
	EntityType  string            `json:"entity_type"`
	Description string            `json:"description"`
	TTL         string            `json:"ttl"`
	Features    []FieldDef        `json:"features"`
	Tags        map[string]string `json:"tags"`
	Owner       string            `json:"owner"`
	Version     string            `json:"version"`
}

// FieldDef describes a single field within a feature group.
type FieldDef struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default"`
}

// ReconcileAction indicates the action taken during reconciliation.
type ReconcileAction string

// ReconcileAction constants.
const (
	ActionCreate ReconcileAction = "create"
	ActionUpdate ReconcileAction = "update"
	ActionDelete ReconcileAction = "delete"
	ActionNoOp   ReconcileAction = "noop"
)

// ReconcileResult captures the outcome of a single reconciliation action.
type ReconcileResult struct {
	Definition string
	Action     ReconcileAction
	Success    bool
	Message    string
	Timestamp  time.Time
}

// DiffEntry describes a pending change for a definition.
type DiffEntry struct {
	Definition string
	Action     ReconcileAction
	Fields     []string
}

// ReconcilerConfig configures the reconciler.
type ReconcilerConfig struct {
	MaxDefinitions int
	DryRun         bool
	StrictMode     bool
}

// DefaultReconcilerConfig returns sensible defaults.
func DefaultReconcilerConfig() ReconcilerConfig {
	return ReconcilerConfig{
		MaxDefinitions: 10000,
		DryRun:         false,
		StrictMode:     false,
	}
}

// ReconcilerStats holds aggregate statistics.
type ReconcilerStats struct {
	TotalDefinitions int
	Applied          int
	Pending          int
	ReconcileCount   int
}

// Reconciler manages desired and applied state for feature definitions.
type Reconciler struct {
	mu          sync.RWMutex
	config      ReconcilerConfig
	definitions map[string]*FeatureDefinition
	applied     map[string]*FeatureDefinition
	history     []ReconcileResult
}

// NewReconciler creates a new reconciler.
func NewReconciler(config ReconcilerConfig) *Reconciler {
	if config.MaxDefinitions == 0 {
		config = DefaultReconcilerConfig()
	}
	return &Reconciler{
		config:      config,
		definitions: make(map[string]*FeatureDefinition),
		applied:     make(map[string]*FeatureDefinition),
		history:     make([]ReconcileResult, 0),
	}
}

// LoadDefinition validates and stores a desired feature definition.
func (r *Reconciler) LoadDefinition(def FeatureDefinition) error {
	if def.Name == "" {
		return fmt.Errorf("name is required: %w", ErrDefinitionInvalid)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.definitions) >= r.config.MaxDefinitions {
		if _, exists := r.definitions[def.Name]; !exists {
			return fmt.Errorf("max definitions (%d) reached: %w", r.config.MaxDefinitions, ErrDefinitionInvalid)
		}
	}

	copy := def
	r.definitions[def.Name] = &copy
	return nil
}

// LoadFromYAML parses simple key-based YAML content into feature definitions.
func (r *Reconciler) LoadFromYAML(yamlContent string) ([]FeatureDefinition, error) {
	var defs []FeatureDefinition
	var current *FeatureDefinition

	lines := strings.Split(yamlContent, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.Contains(trimmed, ":") {
			parts := strings.SplitN(trimmed, ":", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])

			if key == "name" {
				if current != nil {
					defs = append(defs, *current)
				}
				current = &FeatureDefinition{
					Name: val,
					Tags: make(map[string]string),
				}
			} else if current != nil {
				switch key {
				case "entity_type":
					current.EntityType = val
				case "description":
					current.Description = val
				case "ttl":
					current.TTL = val
				case "owner":
					current.Owner = val
				case "version":
					current.Version = val
				}
			}
		}
	}

	if current != nil {
		defs = append(defs, *current)
	}

	// Load all parsed definitions
	for _, def := range defs {
		if err := r.LoadDefinition(def); err != nil {
			return nil, fmt.Errorf("loading %q: %w", def.Name, err)
		}
	}

	return defs, nil
}

// ListDefinitions returns all desired definitions.
func (r *Reconciler) ListDefinitions() []FeatureDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]FeatureDefinition, 0, len(r.definitions))
	for _, def := range r.definitions {
		out = append(out, *def)
	}
	return out
}

// GetDefinition returns a definition by name.
func (r *Reconciler) GetDefinition(name string) (*FeatureDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	def, exists := r.definitions[name]
	if !exists {
		return nil, ErrDefinitionNotFound
	}
	copy := *def
	return &copy, nil
}

// DeleteDefinition removes a desired definition.
func (r *Reconciler) DeleteDefinition(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.definitions[name]; !exists {
		return ErrDefinitionNotFound
	}
	delete(r.definitions, name)
	return nil
}

// Reconcile compares desired vs applied state and generates actions.
func (r *Reconciler) Reconcile() []ReconcileResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	var results []ReconcileResult
	now := time.Now()

	// Check desired definitions against applied
	for name, desired := range r.definitions {
		applied, exists := r.applied[name]
		if !exists {
			action := ReconcileResult{
				Definition: name,
				Action:     ActionCreate,
				Success:    true,
				Message:    "created new definition",
				Timestamp:  now,
			}
			if !r.config.DryRun {
				copy := *desired
				r.applied[name] = &copy
			}
			results = append(results, action)
		} else if !definitionsEqual(desired, applied) {
			action := ReconcileResult{
				Definition: name,
				Action:     ActionUpdate,
				Success:    true,
				Message:    "updated definition",
				Timestamp:  now,
			}
			if !r.config.DryRun {
				copy := *desired
				r.applied[name] = &copy
			}
			results = append(results, action)
		} else {
			results = append(results, ReconcileResult{
				Definition: name,
				Action:     ActionNoOp,
				Success:    true,
				Message:    "no changes",
				Timestamp:  now,
			})
		}
	}

	// Check for applied definitions that no longer exist in desired
	for name := range r.applied {
		if _, exists := r.definitions[name]; !exists {
			action := ReconcileResult{
				Definition: name,
				Action:     ActionDelete,
				Success:    true,
				Message:    "deleted removed definition",
				Timestamp:  now,
			}
			if !r.config.DryRun {
				delete(r.applied, name)
			}
			results = append(results, action)
		}
	}

	r.history = append(r.history, results...)

	return results
}

// GetHistory returns recent reconciliation results.
func (r *Reconciler) GetHistory(limit int) []ReconcileResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 || limit > len(r.history) {
		limit = len(r.history)
	}

	start := len(r.history) - limit
	out := make([]ReconcileResult, limit)
	copy(out, r.history[start:])
	return out
}

// Diff shows what would change without applying.
func (r *Reconciler) Diff() []DiffEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var entries []DiffEntry

	for name, desired := range r.definitions {
		applied, exists := r.applied[name]
		if !exists {
			entries = append(entries, DiffEntry{
				Definition: name,
				Action:     ActionCreate,
				Fields:     []string{"all"},
			})
		} else if !definitionsEqual(desired, applied) {
			fields := diffFields(desired, applied)
			entries = append(entries, DiffEntry{
				Definition: name,
				Action:     ActionUpdate,
				Fields:     fields,
			})
		}
	}

	for name := range r.applied {
		if _, exists := r.definitions[name]; !exists {
			entries = append(entries, DiffEntry{
				Definition: name,
				Action:     ActionDelete,
				Fields:     []string{"all"},
			})
		}
	}

	return entries
}

// Stats returns aggregate statistics.
func (r *Reconciler) Stats() ReconcilerStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pending := 0
	for name, desired := range r.definitions {
		applied, exists := r.applied[name]
		if !exists || !definitionsEqual(desired, applied) {
			pending++
		}
	}

	reconcileCount := 0
	for _, h := range r.history {
		if h.Action != ActionNoOp {
			reconcileCount++
		}
	}

	return ReconcilerStats{
		TotalDefinitions: len(r.definitions),
		Applied:          len(r.applied),
		Pending:          pending,
		ReconcileCount:   reconcileCount,
	}
}

// definitionsEqual compares two definitions for equality.
func definitionsEqual(a, b *FeatureDefinition) bool {
	if a.Name != b.Name || a.EntityType != b.EntityType ||
		a.Description != b.Description || a.TTL != b.TTL ||
		a.Owner != b.Owner || a.Version != b.Version {
		return false
	}
	if len(a.Features) != len(b.Features) {
		return false
	}
	for i := range a.Features {
		if a.Features[i] != b.Features[i] {
			return false
		}
	}
	return true
}

// diffFields returns the names of fields that differ between two definitions.
func diffFields(a, b *FeatureDefinition) []string {
	var fields []string
	if a.EntityType != b.EntityType {
		fields = append(fields, "entity_type")
	}
	if a.Description != b.Description {
		fields = append(fields, "description")
	}
	if a.TTL != b.TTL {
		fields = append(fields, "ttl")
	}
	if a.Owner != b.Owner {
		fields = append(fields, "owner")
	}
	if a.Version != b.Version {
		fields = append(fields, "version")
	}
	if len(a.Features) != len(b.Features) {
		fields = append(fields, "features")
	}
	return fields
}
