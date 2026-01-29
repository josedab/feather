package contract

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ContractDefinition represents a YAML-compatible declarative contract definition.
type ContractDefinition struct {
	Version      string            `json:"version" yaml:"version"`
	Name         string            `json:"name" yaml:"name"`
	Description  string            `json:"description,omitempty" yaml:"description,omitempty"`
	FeatureGroup string            `json:"feature_group" yaml:"feature_group"`
	FeatureName  string            `json:"feature_name,omitempty" yaml:"feature_name,omitempty"`
	Owner        string            `json:"owner,omitempty" yaml:"owner,omitempty"`
	Mode         EnforcementMode   `json:"mode" yaml:"mode"`
	Rules        []RuleDefinition  `json:"rules" yaml:"rules"`
	Alerts       []AlertDefinition `json:"alerts,omitempty" yaml:"alerts,omitempty"`
	SchemaPolicy *SchemaPolicyDef  `json:"schema_policy,omitempty" yaml:"schema_policy,omitempty"`
}

// EnforcementMode determines how violations are handled.
type EnforcementMode string

const (
	ModeWarn    EnforcementMode = "warn"
	ModeEnforce EnforcementMode = "enforce"
	ModeAudit   EnforcementMode = "audit"
)

// RuleDefinition is a YAML-friendly rule specification.
type RuleDefinition struct {
	Type     RuleType `json:"type" yaml:"type"`
	Severity Severity `json:"severity" yaml:"severity"`
	// Freshness
	MaxStaleness string `json:"max_staleness,omitempty" yaml:"max_staleness,omitempty"`
	// Completeness
	MinCompleteness float64 `json:"min_completeness,omitempty" yaml:"min_completeness,omitempty"`
	// Distribution
	MinValue *float64 `json:"min_value,omitempty" yaml:"min_value,omitempty"`
	MaxValue *float64 `json:"max_value,omitempty" yaml:"max_value,omitempty"`
	MeanMin  *float64 `json:"mean_min,omitempty" yaml:"mean_min,omitempty"`
	MeanMax  *float64 `json:"mean_max,omitempty" yaml:"mean_max,omitempty"`
	// Schema
	ExpectedType string `json:"expected_type,omitempty" yaml:"expected_type,omitempty"`
	// Custom
	CustomRule string `json:"custom_rule,omitempty" yaml:"custom_rule,omitempty"`
}

// AlertDefinition specifies how violations are reported.
type AlertDefinition struct {
	Channel     string   `json:"channel" yaml:"channel"`
	Endpoint    string   `json:"endpoint" yaml:"endpoint"`
	MinSeverity Severity `json:"min_severity" yaml:"min_severity"`
}

// SchemaPolicyDef defines schema evolution rules.
type SchemaPolicyDef struct {
	// AllowAddFields permits new fields to be added.
	AllowAddFields bool `json:"allow_add_fields" yaml:"allow_add_fields"`
	// AllowRemoveFields permits fields to be removed.
	AllowRemoveFields bool `json:"allow_remove_fields" yaml:"allow_remove_fields"`
	// AllowTypeChanges permits data type changes.
	AllowTypeChanges bool `json:"allow_type_changes" yaml:"allow_type_changes"`
	// RequireBackwardCompat requires new schema to be backward compatible.
	RequireBackwardCompat bool `json:"require_backward_compat" yaml:"require_backward_compat"`
}

// ValidationResult captures the outcome of validating a contract definition.
type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationError `json:"errors,omitempty"`
	Warnings []string          `json:"warnings,omitempty"`
}

// ValidationError is a specific validation failure.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidateDefinition validates a contract definition for correctness.
func ValidateDefinition(def *ContractDefinition) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if def.Name == "" {
		result.addError("name", "name is required")
	}
	if def.FeatureGroup == "" {
		result.addError("feature_group", "feature_group is required")
	}
	if len(def.Rules) == 0 {
		result.addError("rules", "at least one rule is required")
	}

	validModes := map[EnforcementMode]bool{ModeWarn: true, ModeEnforce: true, ModeAudit: true}
	if def.Mode != "" && !validModes[def.Mode] {
		result.addError("mode", fmt.Sprintf("invalid mode %q; must be warn, enforce, or audit", def.Mode))
	}
	if def.Mode == "" {
		result.Warnings = append(result.Warnings, "mode not specified; defaulting to 'warn'")
	}

	for i, rule := range def.Rules {
		prefix := fmt.Sprintf("rules[%d]", i)
		validTypes := map[RuleType]bool{
			RuleFreshness: true, RuleCompleteness: true,
			RuleDistribution: true, RuleSchema: true, RuleCustom: true,
		}
		if !validTypes[rule.Type] {
			result.addError(prefix+".type", fmt.Sprintf("unknown rule type %q", rule.Type))
		}
		if rule.Type == RuleFreshness && rule.MaxStaleness == "" {
			result.addError(prefix+".max_staleness", "max_staleness is required for freshness rules")
		}
		if rule.Type == RuleFreshness && rule.MaxStaleness != "" {
			if _, err := time.ParseDuration(rule.MaxStaleness); err != nil {
				result.addError(prefix+".max_staleness", fmt.Sprintf("invalid duration: %v", err))
			}
		}
		if rule.Type == RuleCompleteness && (rule.MinCompleteness < 0 || rule.MinCompleteness > 1) {
			result.addError(prefix+".min_completeness", "must be between 0 and 1")
		}
		if rule.Type == RuleSchema && rule.ExpectedType == "" {
			result.addError(prefix+".expected_type", "expected_type is required for schema rules")
		}
	}

	return result
}

func (r *ValidationResult) addError(field, message string) {
	r.Valid = false
	r.Errors = append(r.Errors, ValidationError{Field: field, Message: message})
}

// ToSpec converts a ContractDefinition to a Spec for registration.
func (def *ContractDefinition) ToSpec() (*Spec, error) {
	vr := ValidateDefinition(def)
	if !vr.Valid {
		msgs := make([]string, len(vr.Errors))
		for i, e := range vr.Errors {
			msgs[i] = fmt.Sprintf("%s: %s", e.Field, e.Message)
		}
		return nil, fmt.Errorf("invalid contract: %s", strings.Join(msgs, "; "))
	}

	rules := make([]Rule, len(def.Rules))
	for i, rd := range def.Rules {
		rule := Rule{
			Type:            rd.Type,
			Severity:        rd.Severity,
			MinCompleteness: rd.MinCompleteness,
			MinValue:        rd.MinValue,
			MaxValue:        rd.MaxValue,
			MeanMin:         rd.MeanMin,
			MeanMax:         rd.MeanMax,
			ExpectedType:    rd.ExpectedType,
			CustomRule:      rd.CustomRule,
		}
		if rd.MaxStaleness != "" {
			d, _ := time.ParseDuration(rd.MaxStaleness)
			rule.MaxStaleness = d
		}
		if rule.Severity == "" {
			rule.Severity = SeverityWarning
		}
		rules[i] = rule
	}

	mode := def.Mode
	if mode == "" {
		mode = ModeWarn
	}

	now := time.Now()
	return &Spec{
		Name:         def.Name,
		Description:  def.Description,
		FeatureGroup: def.FeatureGroup,
		FeatureName:  def.FeatureName,
		Rules:        rules,
		Owner:        def.Owner,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// CompareContracts identifies differences between two contract specs.
type ContractDiff struct {
	Added    []Rule   `json:"added_rules,omitempty"`
	Removed  []Rule   `json:"removed_rules,omitempty"`
	Changed  []string `json:"changed_fields,omitempty"`
	Breaking bool     `json:"breaking"`
}

// DiffContracts compares an old and new contract spec.
func DiffContracts(old, new *Spec) *ContractDiff {
	diff := &ContractDiff{}

	if old.FeatureGroup != new.FeatureGroup {
		diff.Changed = append(diff.Changed, "feature_group")
		diff.Breaking = true
	}
	if old.FeatureName != new.FeatureName {
		diff.Changed = append(diff.Changed, "feature_name")
	}

	oldRules := make(map[string]Rule)
	for _, r := range old.Rules {
		oldRules[string(r.Type)+r.CustomRule] = r
	}
	newRules := make(map[string]Rule)
	for _, r := range new.Rules {
		newRules[string(r.Type)+r.CustomRule] = r
	}

	for key, r := range newRules {
		if _, exists := oldRules[key]; !exists {
			diff.Added = append(diff.Added, r)
		}
	}
	for key, r := range oldRules {
		if _, exists := newRules[key]; !exists {
			diff.Removed = append(diff.Removed, r)
			if r.Severity == SeverityCritical || r.Severity == SeverityError {
				diff.Breaking = true
			}
		}
	}

	return diff
}

// ContractReport provides a summary of all contracts and their health.
type ContractReport struct {
	TotalContracts int                `json:"total_contracts"`
	Passing        int                `json:"passing"`
	Warning        int                `json:"warning"`
	Breached       int                `json:"breached"`
	Unknown        int                `json:"unknown"`
	TopViolations  []ViolationSummary `json:"top_violations"`
	GeneratedAt    time.Time          `json:"generated_at"`
}

// ViolationSummary summarizes violations for a contract.
type ViolationSummary struct {
	ContractName string   `json:"contract_name"`
	Count        int      `json:"count"`
	Severity     Severity `json:"severity"`
}

// GenerateReport produces a summary report of all contracts.
func (m *Manager) GenerateReport() *ContractReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &ContractReport{
		TotalContracts: len(m.contracts),
		GeneratedAt:    time.Now(),
	}

	for _, status := range m.statuses {
		switch status.Status {
		case StatusPassing:
			report.Passing++
		case StatusWarning:
			report.Warning++
		case StatusBreached:
			report.Breached++
		default:
			report.Unknown++
		}
	}

	violationCounts := make(map[string]*ViolationSummary)
	for _, v := range m.violations {
		key := v.ContractName
		if vs, ok := violationCounts[key]; ok {
			vs.Count++
			if severityRank(v.Severity) > severityRank(vs.Severity) {
				vs.Severity = v.Severity
			}
		} else {
			violationCounts[key] = &ViolationSummary{
				ContractName: v.ContractName,
				Count:        1,
				Severity:     v.Severity,
			}
		}
	}

	for _, vs := range violationCounts {
		report.TopViolations = append(report.TopViolations, *vs)
	}
	sort.Slice(report.TopViolations, func(i, j int) bool {
		return report.TopViolations[i].Count > report.TopViolations[j].Count
	})
	if len(report.TopViolations) > 10 {
		report.TopViolations = report.TopViolations[:10]
	}

	return report
}

func severityRank(s Severity) int {
	switch s {
	case SeverityCritical:
		return 3
	case SeverityError:
		return 2
	case SeverityWarning:
		return 1
	default:
		return 0
	}
}
