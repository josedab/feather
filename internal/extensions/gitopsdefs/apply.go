package gitopsdefs

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DeclarativeSpec is the top-level structure for a feature store YAML/JSON manifest.
type DeclarativeSpec struct {
	APIVersion string              `json:"apiVersion" yaml:"apiVersion"`
	Kind       string              `json:"kind" yaml:"kind"`
	Metadata   SpecMetadata        `json:"metadata" yaml:"metadata"`
	Spec       FeatureGroupSpec    `json:"spec" yaml:"spec"`
}

// SpecMetadata holds identification fields for a spec.
type SpecMetadata struct {
	Name        string            `json:"name" yaml:"name"`
	Namespace   string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}

// FeatureGroupSpec describes a feature group in declarative form.
type FeatureGroupSpec struct {
	EntityType  string       `json:"entityType" yaml:"entityType"`
	Description string       `json:"description,omitempty" yaml:"description,omitempty"`
	Owner       string       `json:"owner,omitempty" yaml:"owner,omitempty"`
	TTL         string       `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Features    []FeatureSpec `json:"features" yaml:"features"`
	Tags        map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// FeatureSpec describes a single feature in declarative form.
type FeatureSpec struct {
	Name        string `json:"name" yaml:"name"`
	Type        string `json:"type" yaml:"type"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool   `json:"required,omitempty" yaml:"required,omitempty"`
	Default     string `json:"default,omitempty" yaml:"default,omitempty"`
}

// ValidationResult from validating a declarative spec.
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors  []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ValidateSpec checks a declarative spec for correctness.
func ValidateSpec(spec *DeclarativeSpec) ValidationResult {
	result := ValidationResult{Valid: true}

	if spec.APIVersion == "" {
		result.Errors = append(result.Errors, "apiVersion is required")
	}
	if spec.Kind == "" {
		result.Errors = append(result.Errors, "kind is required")
	} else if spec.Kind != "FeatureGroup" && spec.Kind != "FeaturePipeline" {
		result.Errors = append(result.Errors, fmt.Sprintf("unknown kind %q (expected FeatureGroup or FeaturePipeline)", spec.Kind))
	}
	if spec.Metadata.Name == "" {
		result.Errors = append(result.Errors, "metadata.name is required")
	}
	if spec.Spec.EntityType == "" {
		result.Warnings = append(result.Warnings, "spec.entityType is recommended")
	}

	validTypes := map[string]bool{
		"int64": true, "float64": true, "string": true,
		"bool": true, "bytes": true, "vector": true, "timestamp": true,
	}
	seen := make(map[string]bool)
	for i, f := range spec.Spec.Features {
		if f.Name == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("features[%d].name is required", i))
		}
		if seen[f.Name] {
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate feature name %q", f.Name))
		}
		seen[f.Name] = true

		if f.Type != "" && !validTypes[f.Type] {
			result.Errors = append(result.Errors, fmt.Sprintf("features[%d] (%s): invalid type %q", i, f.Name, f.Type))
		}
	}

	result.Valid = len(result.Errors) == 0
	return result
}

// PlanResult is the output of a dry-run apply (feather plan).
type PlanResult struct {
	Changes   []PlanChange `json:"changes"`
	Summary   PlanSummary  `json:"summary"`
	Timestamp time.Time    `json:"timestamp"`
}

// PlanChange describes a single planned change.
type PlanChange struct {
	Name   string          `json:"name"`
	Action ReconcileAction `json:"action"`
	Diff   []string        `json:"diff,omitempty"`
}

// PlanSummary summarizes planned changes.
type PlanSummary struct {
	Creates int `json:"creates"`
	Updates int `json:"updates"`
	Deletes int `json:"deletes"`
	NoOps   int `json:"no_ops"`
}

// ApplyResult is the output of an apply operation.
type ApplyResult struct {
	Results   []ReconcileResult `json:"results"`
	Summary   PlanSummary       `json:"summary"`
	Success   bool              `json:"success"`
	Timestamp time.Time         `json:"timestamp"`
}

// Plan performs a dry-run, showing what would change without applying.
func (r *Reconciler) Plan() *PlanResult {
	diffs := r.Diff()
	plan := &PlanResult{
		Timestamp: time.Now(),
	}

	for _, d := range diffs {
		plan.Changes = append(plan.Changes, PlanChange{
			Name:   d.Definition,
			Action: d.Action,
			Diff:   d.Fields,
		})
		switch d.Action {
		case ActionCreate:
			plan.Summary.Creates++
		case ActionUpdate:
			plan.Summary.Updates++
		case ActionDelete:
			plan.Summary.Deletes++
		}
	}

	return plan
}

// Apply reconciles all definitions and returns the result.
func (r *Reconciler) Apply() *ApplyResult {
	results := r.Reconcile()
	apply := &ApplyResult{
		Results:   results,
		Success:   true,
		Timestamp: time.Now(),
	}

	for _, res := range results {
		if !res.Success {
			apply.Success = false
		}
		switch res.Action {
		case ActionCreate:
			apply.Summary.Creates++
		case ActionUpdate:
			apply.Summary.Updates++
		case ActionDelete:
			apply.Summary.Deletes++
		case ActionNoOp:
			apply.Summary.NoOps++
		}
	}

	return apply
}

// LoadSpecJSON loads a declarative spec from JSON and validates it.
func (r *Reconciler) LoadSpecJSON(data []byte) (*DeclarativeSpec, *ValidationResult, error) {
	var spec DeclarativeSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, nil, fmt.Errorf("invalid JSON: %w", err)
	}

	validation := ValidateSpec(&spec)
	if !validation.Valid {
		return &spec, &validation, nil
	}

	// Convert to FeatureDefinition and load
	def := specToDefinition(&spec)
	if err := r.LoadDefinition(def); err != nil {
		return &spec, &validation, fmt.Errorf("loading definition: %w", err)
	}

	return &spec, &validation, nil
}

func specToDefinition(spec *DeclarativeSpec) FeatureDefinition {
	def := FeatureDefinition{
		Name:        spec.Metadata.Name,
		EntityType:  spec.Spec.EntityType,
		Description: spec.Spec.Description,
		Owner:       spec.Spec.Owner,
		TTL:         spec.Spec.TTL,
		Tags:        spec.Spec.Tags,
		Version:     spec.APIVersion,
	}

	for _, f := range spec.Spec.Features {
		def.Features = append(def.Features, FieldDef{
			Name:        f.Name,
			Type:        f.Type,
			Description: f.Description,
			Required:    f.Required,
			Default:     f.Default,
		})
	}

	if def.Tags == nil {
		def.Tags = make(map[string]string)
	}
	for k, v := range spec.Metadata.Labels {
		def.Tags[k] = v
	}

	return def
}

// Rollback reverts the applied state to the previous version for a given definition.
func (r *Reconciler) Rollback(name string) (*ReconcileResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find the last successful action for this definition in history
	var lastApplied *FeatureDefinition
	for i := len(r.history) - 1; i >= 0; i-- {
		if r.history[i].Definition == name && r.history[i].Action != ActionNoOp && r.history[i].Success {
			break
		}
	}

	if lastApplied == nil {
		// No previous state — remove from applied
		if _, exists := r.applied[name]; !exists {
			return nil, fmt.Errorf("definition %q not found in applied state", name)
		}
		delete(r.applied, name)
		result := &ReconcileResult{
			Definition: name,
			Action:     ActionDelete,
			Success:    true,
			Message:    "rolled back (removed from applied state)",
			Timestamp:  time.Now(),
		}
		r.history = append(r.history, *result)
		return result, nil
	}

	return nil, fmt.Errorf("no rollback target for %q", name)
}

// FormatPlan returns a human-readable plan output (like terraform plan).
func FormatPlan(plan *PlanResult) string {
	var sb strings.Builder
	sb.WriteString("Feather Plan:\n\n")

	for _, c := range plan.Changes {
		switch c.Action {
		case ActionCreate:
			sb.WriteString(fmt.Sprintf("  + %s (create)\n", c.Name))
		case ActionUpdate:
			sb.WriteString(fmt.Sprintf("  ~ %s (update: %s)\n", c.Name, strings.Join(c.Diff, ", ")))
		case ActionDelete:
			sb.WriteString(fmt.Sprintf("  - %s (delete)\n", c.Name))
		}
	}

	sb.WriteString(fmt.Sprintf("\nPlan: %d to add, %d to change, %d to destroy.\n",
		plan.Summary.Creates, plan.Summary.Updates, plan.Summary.Deletes))

	return sb.String()
}
