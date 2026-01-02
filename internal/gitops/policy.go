package gitops

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Policy defines a governance policy for features.
type Policy struct {
	APIVersion string     `json:"apiVersion" yaml:"apiVersion"`
	Kind       string     `json:"kind" yaml:"kind"`
	Metadata   PolicyMeta `json:"metadata" yaml:"metadata"`
	Spec       PolicySpec `json:"spec" yaml:"spec"`
}

// PolicyMeta contains policy metadata.
type PolicyMeta struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Severity    string            `json:"severity,omitempty" yaml:"severity,omitempty"` // error, warning, info
}

// PolicySpec defines what the policy enforces.
type PolicySpec struct {
	Target     PolicyTarget      `json:"target" yaml:"target"`
	Rules      []PolicyRule      `json:"rules" yaml:"rules"`
	Exemptions []PolicyExemption `json:"exemptions,omitempty" yaml:"exemptions,omitempty"`
}

// PolicyTarget defines which resources the policy applies to.
type PolicyTarget struct {
	Kinds      []string          `json:"kinds,omitempty" yaml:"kinds,omitempty"`
	Namespaces []string          `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	Labels     map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Teams      []string          `json:"teams,omitempty" yaml:"teams,omitempty"`
}

// PolicyRule defines a single policy rule.
type PolicyRule struct {
	Name       string      `json:"name" yaml:"name"`
	Type       string      `json:"type" yaml:"type"` // require, forbid, limit, pattern
	Field      string      `json:"field,omitempty" yaml:"field,omitempty"`
	Value      interface{} `json:"value,omitempty" yaml:"value,omitempty"`
	Expression string      `json:"expression,omitempty" yaml:"expression,omitempty"`
	Message    string      `json:"message,omitempty" yaml:"message,omitempty"`
}

// PolicyExemption allows specific resources to bypass a rule.
type PolicyExemption struct {
	Name      string   `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace string   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Rules     []string `json:"rules,omitempty" yaml:"rules,omitempty"`
	Reason    string   `json:"reason,omitempty" yaml:"reason,omitempty"`
	ExpiresAt string   `json:"expiresAt,omitempty" yaml:"expiresAt,omitempty"`
}

// PolicyViolation represents a policy violation.
type PolicyViolation struct {
	Policy    string    `json:"policy"`
	Rule      string    `json:"rule"`
	Resource  string    `json:"resource"`
	Namespace string    `json:"namespace,omitempty"`
	Field     string    `json:"field,omitempty"`
	Message   string    `json:"message"`
	Severity  string    `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
}

// PolicyResult contains the result of policy evaluation.
type PolicyResult struct {
	Passed     bool              `json:"passed"`
	Violations []PolicyViolation `json:"violations,omitempty"`
	Warnings   []PolicyViolation `json:"warnings,omitempty"`
	Evaluated  int               `json:"evaluated"`
	Timestamp  time.Time         `json:"timestamp"`
}

// PolicyEngine evaluates policies against feature definitions.
type PolicyEngine struct {
	mu       sync.RWMutex
	policies map[string]*Policy
}

// NewPolicyEngine creates a new policy engine.
func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{
		policies: make(map[string]*Policy),
	}
}

// RegisterPolicy registers a policy with the engine.
func (e *PolicyEngine) RegisterPolicy(policy *Policy) error {
	if policy.Metadata.Name == "" {
		return fmt.Errorf("policy name is required")
	}
	if len(policy.Spec.Rules) == 0 {
		return fmt.Errorf("policy must have at least one rule")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.policies[policy.Metadata.Name] = policy
	return nil
}

// UnregisterPolicy removes a policy from the engine.
func (e *PolicyEngine) UnregisterPolicy(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.policies, name)
}

// GetPolicy returns a policy by name.
func (e *PolicyEngine) GetPolicy(name string) (*Policy, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policy, exists := e.policies[name]
	return policy, exists
}

// ListPolicies returns all registered policies.
func (e *PolicyEngine) ListPolicies() []*Policy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policies := make([]*Policy, 0, len(e.policies))
	for _, p := range e.policies {
		policies = append(policies, p)
	}
	return policies
}

// Evaluate evaluates all policies against a feature definition.
func (e *PolicyEngine) Evaluate(def *FeatureDefinition) *PolicyResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := &PolicyResult{
		Passed:    true,
		Timestamp: time.Now(),
	}

	for _, policy := range e.policies {
		if !e.matchesTarget(def, &policy.Spec.Target) {
			continue
		}

		result.Evaluated++
		violations := e.evaluatePolicy(def, policy)

		for _, v := range violations {
			if v.Severity == "warning" || v.Severity == "info" {
				result.Warnings = append(result.Warnings, v)
			} else {
				result.Violations = append(result.Violations, v)
				result.Passed = false
			}
		}
	}

	return result
}

// matchesTarget checks if a definition matches a policy target.
func (e *PolicyEngine) matchesTarget(def *FeatureDefinition, target *PolicyTarget) bool {
	// Check kinds
	if len(target.Kinds) > 0 {
		found := false
		for _, kind := range target.Kinds {
			if def.Kind == kind {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check namespaces
	if len(target.Namespaces) > 0 {
		found := false
		for _, ns := range target.Namespaces {
			if def.Metadata.Namespace == ns {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check labels
	for key, value := range target.Labels {
		if def.Metadata.Labels[key] != value {
			return false
		}
	}

	// Check teams
	if len(target.Teams) > 0 {
		found := false
		for _, team := range target.Teams {
			if def.Metadata.Team == team {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// evaluatePolicy evaluates a single policy against a definition.
func (e *PolicyEngine) evaluatePolicy(def *FeatureDefinition, policy *Policy) []PolicyViolation {
	var violations []PolicyViolation

	severity := policy.Metadata.Severity
	if severity == "" {
		severity = "error"
	}

	for _, rule := range policy.Spec.Rules {
		// Check for exemption
		if e.isExempt(def, policy, rule.Name) {
			continue
		}

		if v := e.evaluateRule(def, policy, &rule, severity); v != nil {
			violations = append(violations, *v)
		}
	}

	return violations
}

// isExempt checks if a definition is exempt from a rule.
func (e *PolicyEngine) isExempt(def *FeatureDefinition, policy *Policy, ruleName string) bool {
	for _, exemption := range policy.Spec.Exemptions {
		// Check expiration
		if exemption.ExpiresAt != "" {
			expires, err := time.Parse(time.RFC3339, exemption.ExpiresAt)
			if err == nil && time.Now().After(expires) {
				continue
			}
		}

		// Match name
		if exemption.Name != "" && exemption.Name != def.Metadata.Name {
			continue
		}

		// Match namespace
		if exemption.Namespace != "" && exemption.Namespace != def.Metadata.Namespace {
			continue
		}

		// Check if rule is in exempted list
		if len(exemption.Rules) > 0 {
			found := false
			for _, r := range exemption.Rules {
				if r == ruleName {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		return true
	}
	return false
}

// evaluateRule evaluates a single rule against a definition.
func (e *PolicyEngine) evaluateRule(def *FeatureDefinition, policy *Policy, rule *PolicyRule, severity string) *PolicyViolation {
	switch rule.Type {
	case "require":
		return e.evaluateRequire(def, policy, rule, severity)
	case "forbid":
		return e.evaluateForbid(def, policy, rule, severity)
	case "limit":
		return e.evaluateLimit(def, policy, rule, severity)
	case "pattern":
		return e.evaluatePattern(def, policy, rule, severity)
	default:
		return nil
	}
}

// evaluateRequire checks that a required field is present.
func (e *PolicyEngine) evaluateRequire(def *FeatureDefinition, policy *Policy, rule *PolicyRule, severity string) *PolicyViolation {
	value := e.getFieldValue(def, rule.Field)
	if value == nil || value == "" {
		msg := rule.Message
		if msg == "" {
			msg = fmt.Sprintf("field '%s' is required", rule.Field)
		}
		return &PolicyViolation{
			Policy:    policy.Metadata.Name,
			Rule:      rule.Name,
			Resource:  def.Metadata.Name,
			Namespace: def.Metadata.Namespace,
			Field:     rule.Field,
			Message:   msg,
			Severity:  severity,
			Timestamp: time.Now(),
		}
	}
	return nil
}

// evaluateForbid checks that a forbidden field is not present.
func (e *PolicyEngine) evaluateForbid(def *FeatureDefinition, policy *Policy, rule *PolicyRule, severity string) *PolicyViolation {
	value := e.getFieldValue(def, rule.Field)
	if value != nil && value != "" {
		msg := rule.Message
		if msg == "" {
			msg = fmt.Sprintf("field '%s' is forbidden", rule.Field)
		}
		return &PolicyViolation{
			Policy:    policy.Metadata.Name,
			Rule:      rule.Name,
			Resource:  def.Metadata.Name,
			Namespace: def.Metadata.Namespace,
			Field:     rule.Field,
			Message:   msg,
			Severity:  severity,
			Timestamp: time.Now(),
		}
	}
	return nil
}

// evaluateLimit checks numeric limits.
func (e *PolicyEngine) evaluateLimit(def *FeatureDefinition, policy *Policy, rule *PolicyRule, severity string) *PolicyViolation {
	value := e.getFieldValue(def, rule.Field)

	var numValue float64
	switch v := value.(type) {
	case int:
		numValue = float64(v)
	case int64:
		numValue = float64(v)
	case float64:
		numValue = v
	case []FeatureField:
		numValue = float64(len(v))
	default:
		return nil
	}

	limit, ok := rule.Value.(float64)
	if !ok {
		if intLimit, ok := rule.Value.(int); ok {
			limit = float64(intLimit)
		} else {
			return nil
		}
	}

	if numValue > limit {
		msg := rule.Message
		if msg == "" {
			msg = fmt.Sprintf("field '%s' exceeds limit of %v (actual: %v)", rule.Field, limit, numValue)
		}
		return &PolicyViolation{
			Policy:    policy.Metadata.Name,
			Rule:      rule.Name,
			Resource:  def.Metadata.Name,
			Namespace: def.Metadata.Namespace,
			Field:     rule.Field,
			Message:   msg,
			Severity:  severity,
			Timestamp: time.Now(),
		}
	}
	return nil
}

// evaluatePattern checks string patterns.
func (e *PolicyEngine) evaluatePattern(def *FeatureDefinition, policy *Policy, rule *PolicyRule, severity string) *PolicyViolation {
	value := e.getFieldValue(def, rule.Field)

	strValue, ok := value.(string)
	if !ok {
		return nil
	}

	pattern, ok := rule.Value.(string)
	if !ok {
		return nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	if !re.MatchString(strValue) {
		msg := rule.Message
		if msg == "" {
			msg = fmt.Sprintf("field '%s' does not match pattern '%s'", rule.Field, pattern)
		}
		return &PolicyViolation{
			Policy:    policy.Metadata.Name,
			Rule:      rule.Name,
			Resource:  def.Metadata.Name,
			Namespace: def.Metadata.Namespace,
			Field:     rule.Field,
			Message:   msg,
			Severity:  severity,
			Timestamp: time.Now(),
		}
	}
	return nil
}

// getFieldValue retrieves a field value from a definition using dot notation.
func (e *PolicyEngine) getFieldValue(def *FeatureDefinition, field string) interface{} {
	parts := strings.Split(field, ".")

	switch parts[0] {
	case "metadata":
		if len(parts) < 2 {
			return nil
		}
		switch parts[1] {
		case "name":
			return def.Metadata.Name
		case "namespace":
			return def.Metadata.Namespace
		case "owner":
			return def.Metadata.Owner
		case "team":
			return def.Metadata.Team
		case "labels":
			if len(parts) == 3 {
				return def.Metadata.Labels[parts[2]]
			}
			return def.Metadata.Labels
		case "annotations":
			if len(parts) == 3 {
				return def.Metadata.Annotations[parts[2]]
			}
			return def.Metadata.Annotations
		}
	case "spec":
		if len(parts) < 2 {
			return nil
		}
		switch parts[1] {
		case "entityType":
			return def.Spec.EntityType
		case "description":
			return def.Spec.Description
		case "features":
			return def.Spec.Features
		case "ttl":
			if def.Spec.TTL != nil {
				return def.Spec.TTL.Duration
			}
			return nil
		case "tags":
			return def.Spec.Tags
		case "deprecation":
			if def.Spec.Deprecation != nil && len(parts) == 3 {
				switch parts[2] {
				case "deprecated":
					return def.Spec.Deprecation.Deprecated
				case "message":
					return def.Spec.Deprecation.Message
				}
			}
			return def.Spec.Deprecation
		}
	}
	return nil
}

// CreateStandardPolicies returns a set of commonly used governance policies.
func CreateStandardPolicies() []*Policy {
	return []*Policy{
		{
			APIVersion: "feather.io/v1",
			Kind:       "Policy",
			Metadata: PolicyMeta{
				Name:        "require-owner",
				Description: "All feature definitions must have an owner",
				Severity:    "error",
			},
			Spec: PolicySpec{
				Target: PolicyTarget{
					Kinds: []string{"FeatureGroup"},
				},
				Rules: []PolicyRule{
					{
						Name:    "owner-required",
						Type:    "require",
						Field:   "metadata.owner",
						Message: "All feature definitions must specify an owner",
					},
				},
			},
		},
		{
			APIVersion: "feather.io/v1",
			Kind:       "Policy",
			Metadata: PolicyMeta{
				Name:        "require-description",
				Description: "All features should have descriptions",
				Severity:    "warning",
			},
			Spec: PolicySpec{
				Target: PolicyTarget{
					Kinds: []string{"FeatureGroup"},
				},
				Rules: []PolicyRule{
					{
						Name:    "description-required",
						Type:    "require",
						Field:   "spec.description",
						Message: "Feature groups should have a description",
					},
				},
			},
		},
		{
			APIVersion: "feather.io/v1",
			Kind:       "Policy",
			Metadata: PolicyMeta{
				Name:        "limit-features-per-group",
				Description: "Limit the number of features per group",
				Severity:    "error",
			},
			Spec: PolicySpec{
				Target: PolicyTarget{
					Kinds: []string{"FeatureGroup"},
				},
				Rules: []PolicyRule{
					{
						Name:    "max-features",
						Type:    "limit",
						Field:   "spec.features",
						Value:   100,
						Message: "Feature groups should have at most 100 features",
					},
				},
			},
		},
		{
			APIVersion: "feather.io/v1",
			Kind:       "Policy",
			Metadata: PolicyMeta{
				Name:        "naming-convention",
				Description: "Enforce naming conventions",
				Severity:    "error",
			},
			Spec: PolicySpec{
				Target: PolicyTarget{
					Kinds: []string{"FeatureGroup"},
				},
				Rules: []PolicyRule{
					{
						Name:    "name-format",
						Type:    "pattern",
						Field:   "metadata.name",
						Value:   "^[a-z][a-z0-9_]*$",
						Message: "Names must be lowercase, start with a letter, and contain only letters, numbers, and underscores",
					},
				},
			},
		},
	}
}
