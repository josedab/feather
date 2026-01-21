package gitops

import (
	"testing"
	"time"
)

func TestPolicyEngine_RegisterPolicy(t *testing.T) {
	engine := NewPolicyEngine()

	policy := &Policy{
		APIVersion: "feather.io/v1",
		Kind:       "Policy",
		Metadata: PolicyMeta{
			Name: "test-policy",
		},
		Spec: PolicySpec{
			Rules: []PolicyRule{
				{Name: "test-rule", Type: "require", Field: "metadata.name"},
			},
		},
	}

	if err := engine.RegisterPolicy(policy); err != nil {
		t.Fatalf("RegisterPolicy failed: %v", err)
	}

	retrieved, exists := engine.GetPolicy("test-policy")
	if !exists {
		t.Fatal("Expected policy to exist")
	}
	if retrieved.Metadata.Name != "test-policy" {
		t.Errorf("Expected name 'test-policy', got '%s'", retrieved.Metadata.Name)
	}
}

func TestPolicyEngine_RegisterPolicy_Validation(t *testing.T) {
	engine := NewPolicyEngine()

	tests := []struct {
		name    string
		policy  *Policy
		wantErr bool
	}{
		{
			name: "missing name",
			policy: &Policy{
				Spec: PolicySpec{
					Rules: []PolicyRule{{Name: "rule", Type: "require"}},
				},
			},
			wantErr: true,
		},
		{
			name: "no rules",
			policy: &Policy{
				Metadata: PolicyMeta{Name: "test"},
				Spec:     PolicySpec{Rules: []PolicyRule{}},
			},
			wantErr: true,
		},
		{
			name: "valid policy",
			policy: &Policy{
				Metadata: PolicyMeta{Name: "test"},
				Spec: PolicySpec{
					Rules: []PolicyRule{{Name: "rule", Type: "require"}},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.RegisterPolicy(tt.policy)
			if tt.wantErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestPolicyEngine_UnregisterPolicy(t *testing.T) {
	engine := NewPolicyEngine()

	policy := &Policy{
		Metadata: PolicyMeta{Name: "test-policy"},
		Spec: PolicySpec{
			Rules: []PolicyRule{{Name: "rule", Type: "require"}},
		},
	}

	engine.RegisterPolicy(policy)
	engine.UnregisterPolicy("test-policy")

	_, exists := engine.GetPolicy("test-policy")
	if exists {
		t.Error("Expected policy to be removed")
	}
}

func TestPolicyEngine_ListPolicies(t *testing.T) {
	engine := NewPolicyEngine()

	policies := []*Policy{
		{Metadata: PolicyMeta{Name: "policy1"}, Spec: PolicySpec{Rules: []PolicyRule{{Name: "r1", Type: "require"}}}},
		{Metadata: PolicyMeta{Name: "policy2"}, Spec: PolicySpec{Rules: []PolicyRule{{Name: "r2", Type: "require"}}}},
		{Metadata: PolicyMeta{Name: "policy3"}, Spec: PolicySpec{Rules: []PolicyRule{{Name: "r3", Type: "require"}}}},
	}

	for _, p := range policies {
		engine.RegisterPolicy(p)
	}

	listed := engine.ListPolicies()
	if len(listed) != 3 {
		t.Errorf("Expected 3 policies, got %d", len(listed))
	}
}

func TestPolicyEngine_Evaluate_RequireRule(t *testing.T) {
	engine := NewPolicyEngine()

	policy := &Policy{
		APIVersion: "feather.io/v1",
		Kind:       "Policy",
		Metadata: PolicyMeta{
			Name:     "require-owner",
			Severity: "error",
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
					Message: "Owner is required",
				},
			},
		},
	}
	engine.RegisterPolicy(policy)

	// Test with missing owner
	defWithoutOwner := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "test"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	result := engine.Evaluate(defWithoutOwner)
	if result.Passed {
		t.Error("Expected evaluation to fail")
	}
	if len(result.Violations) != 1 {
		t.Errorf("Expected 1 violation, got %d", len(result.Violations))
	}
	if result.Violations[0].Message != "Owner is required" {
		t.Errorf("Expected message 'Owner is required', got '%s'", result.Violations[0].Message)
	}

	// Test with owner
	defWithOwner := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "test", Owner: "ml-team"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	result = engine.Evaluate(defWithOwner)
	if !result.Passed {
		t.Error("Expected evaluation to pass")
	}
}

func TestPolicyEngine_Evaluate_ForbidRule(t *testing.T) {
	engine := NewPolicyEngine()

	policy := &Policy{
		Metadata: PolicyMeta{
			Name:     "forbid-deprecated",
			Severity: "warning",
		},
		Spec: PolicySpec{
			Rules: []PolicyRule{
				{
					Name:    "no-deprecation",
					Type:    "forbid",
					Field:   "spec.deprecation.deprecated",
					Message: "Deprecated features should not be deployed",
				},
			},
		},
	}
	engine.RegisterPolicy(policy)

	// Test with deprecated feature
	defDeprecated := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "test"},
		Spec: FeatureSpec{
			EntityType: "user",
			Features:   []FeatureField{{Name: "age", DataType: "int64"}},
			Deprecation: &DeprecationSpec{
				Deprecated: true,
				Message:    "Use new_features instead",
			},
		},
	}

	result := engine.Evaluate(defDeprecated)
	// The severity is warning, so result.Passed should be true (only errors fail)
	if !result.Passed {
		t.Error("Expected evaluation to pass with warnings")
	}
	// But there should be warnings recorded
	if len(result.Warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(result.Warnings))
	}
}

func TestPolicyEngine_Evaluate_LimitRule(t *testing.T) {
	engine := NewPolicyEngine()

	policy := &Policy{
		Metadata: PolicyMeta{
			Name:     "limit-features",
			Severity: "error",
		},
		Spec: PolicySpec{
			Rules: []PolicyRule{
				{
					Name:    "max-features",
					Type:    "limit",
					Field:   "spec.features",
					Value:   3,
					Message: "Feature groups should have at most 3 features",
				},
			},
		},
	}
	engine.RegisterPolicy(policy)

	// Test within limit
	defSmall := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "test"},
		Spec: FeatureSpec{
			EntityType: "user",
			Features: []FeatureField{
				{Name: "f1", DataType: "int64"},
				{Name: "f2", DataType: "int64"},
			},
		},
	}

	result := engine.Evaluate(defSmall)
	if !result.Passed {
		t.Error("Expected evaluation to pass for small feature group")
	}

	// Test exceeding limit
	defLarge := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "test"},
		Spec: FeatureSpec{
			EntityType: "user",
			Features: []FeatureField{
				{Name: "f1", DataType: "int64"},
				{Name: "f2", DataType: "int64"},
				{Name: "f3", DataType: "int64"},
				{Name: "f4", DataType: "int64"},
			},
		},
	}

	result = engine.Evaluate(defLarge)
	if result.Passed {
		t.Error("Expected evaluation to fail for large feature group")
	}
}

func TestPolicyEngine_Evaluate_PatternRule(t *testing.T) {
	engine := NewPolicyEngine()

	policy := &Policy{
		Metadata: PolicyMeta{
			Name:     "naming-convention",
			Severity: "error",
		},
		Spec: PolicySpec{
			Rules: []PolicyRule{
				{
					Name:    "name-format",
					Type:    "pattern",
					Field:   "metadata.name",
					Value:   "^[a-z][a-z0-9_]*$",
					Message: "Names must be lowercase snake_case",
				},
			},
		},
	}
	engine.RegisterPolicy(policy)

	// Test valid name
	defValid := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "user_features"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	result := engine.Evaluate(defValid)
	if !result.Passed {
		t.Error("Expected evaluation to pass for valid name")
	}

	// Test invalid name
	defInvalid := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "UserFeatures"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	result = engine.Evaluate(defInvalid)
	if result.Passed {
		t.Error("Expected evaluation to fail for invalid name")
	}
}

func TestPolicyEngine_Evaluate_TargetFiltering(t *testing.T) {
	engine := NewPolicyEngine()

	policy := &Policy{
		Metadata: PolicyMeta{
			Name:     "production-policy",
			Severity: "error",
		},
		Spec: PolicySpec{
			Target: PolicyTarget{
				Namespaces: []string{"production"},
				Labels:     map[string]string{"env": "prod"},
			},
			Rules: []PolicyRule{
				{
					Name:  "require-team",
					Type:  "require",
					Field: "metadata.team",
				},
			},
		},
	}
	engine.RegisterPolicy(policy)

	// Test non-matching namespace
	defDev := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "test", Namespace: "development"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	result := engine.Evaluate(defDev)
	if !result.Passed {
		t.Error("Expected policy to not apply to development namespace")
	}
	if result.Evaluated != 0 {
		t.Error("Expected 0 policies evaluated")
	}

	// Test matching namespace but missing label
	defProdNoLabel := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "test", Namespace: "production"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	result = engine.Evaluate(defProdNoLabel)
	if !result.Passed {
		t.Error("Expected policy to not apply without matching labels")
	}

	// Test matching namespace and labels
	defProd := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata: DefinitionMeta{
			Name:      "test",
			Namespace: "production",
			Labels:    map[string]string{"env": "prod"},
		},
		Spec: FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	result = engine.Evaluate(defProd)
	if result.Passed {
		t.Error("Expected policy to fail (missing team)")
	}
	if result.Evaluated != 1 {
		t.Errorf("Expected 1 policy evaluated, got %d", result.Evaluated)
	}
}

func TestPolicyEngine_Evaluate_TeamFiltering(t *testing.T) {
	engine := NewPolicyEngine()

	policy := &Policy{
		Metadata: PolicyMeta{
			Name:     "ml-team-policy",
			Severity: "error",
		},
		Spec: PolicySpec{
			Target: PolicyTarget{
				Teams: []string{"ml-team", "data-team"},
			},
			Rules: []PolicyRule{
				{Name: "require-description", Type: "require", Field: "spec.description"},
			},
		},
	}
	engine.RegisterPolicy(policy)

	// Test non-matching team
	defOtherTeam := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "test", Team: "frontend-team"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	result := engine.Evaluate(defOtherTeam)
	if result.Evaluated != 0 {
		t.Error("Expected policy to not apply to other team")
	}

	// Test matching team
	defMLTeam := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "test", Team: "ml-team"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	result = engine.Evaluate(defMLTeam)
	if result.Passed {
		t.Error("Expected policy to fail (missing description)")
	}
	if result.Evaluated != 1 {
		t.Errorf("Expected 1 policy evaluated, got %d", result.Evaluated)
	}
}

func TestPolicyEngine_Evaluate_Exemptions(t *testing.T) {
	engine := NewPolicyEngine()

	policy := &Policy{
		Metadata: PolicyMeta{
			Name:     "require-owner",
			Severity: "error",
		},
		Spec: PolicySpec{
			Rules: []PolicyRule{
				{Name: "owner-required", Type: "require", Field: "metadata.owner"},
			},
			Exemptions: []PolicyExemption{
				{Name: "legacy_features", Reason: "Legacy system"},
			},
		},
	}
	engine.RegisterPolicy(policy)

	// Test exempted resource
	defExempted := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "legacy_features"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	result := engine.Evaluate(defExempted)
	if !result.Passed {
		t.Error("Expected exempted resource to pass")
	}

	// Test non-exempted resource
	defNotExempted := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "new_features"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	result = engine.Evaluate(defNotExempted)
	if result.Passed {
		t.Error("Expected non-exempted resource to fail")
	}
}

func TestPolicyEngine_Evaluate_ExemptionExpiry(t *testing.T) {
	engine := NewPolicyEngine()

	// Expired exemption
	policy := &Policy{
		Metadata: PolicyMeta{
			Name:     "require-owner",
			Severity: "error",
		},
		Spec: PolicySpec{
			Rules: []PolicyRule{
				{Name: "owner-required", Type: "require", Field: "metadata.owner"},
			},
			Exemptions: []PolicyExemption{
				{
					Name:      "legacy_features",
					ExpiresAt: "2020-01-01T00:00:00Z",
				},
			},
		},
	}
	engine.RegisterPolicy(policy)

	def := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "legacy_features"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	result := engine.Evaluate(def)
	if result.Passed {
		t.Error("Expected expired exemption to not apply")
	}
}

func TestPolicyEngine_Evaluate_RuleSpecificExemption(t *testing.T) {
	engine := NewPolicyEngine()

	policy := &Policy{
		Metadata: PolicyMeta{
			Name:     "multi-rule-policy",
			Severity: "error",
		},
		Spec: PolicySpec{
			Rules: []PolicyRule{
				{Name: "rule1", Type: "require", Field: "metadata.owner"},
				{Name: "rule2", Type: "require", Field: "spec.description"},
			},
			Exemptions: []PolicyExemption{
				{Name: "test_features", Rules: []string{"rule1"}},
			},
		},
	}
	engine.RegisterPolicy(policy)

	def := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "test_features"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	result := engine.Evaluate(def)
	if result.Passed {
		t.Error("Expected rule2 to still fail")
	}
	// Should have 1 violation for rule2, rule1 is exempted
	if len(result.Violations) != 1 {
		t.Errorf("Expected 1 violation, got %d", len(result.Violations))
	}
	if result.Violations[0].Rule != "rule2" {
		t.Errorf("Expected violation for rule2, got %s", result.Violations[0].Rule)
	}
}

func TestCreateStandardPolicies(t *testing.T) {
	policies := CreateStandardPolicies()

	if len(policies) < 4 {
		t.Errorf("Expected at least 4 standard policies, got %d", len(policies))
	}

	// Check that all policies are valid
	engine := NewPolicyEngine()
	for _, p := range policies {
		if err := engine.RegisterPolicy(p); err != nil {
			t.Errorf("Standard policy %s is invalid: %v", p.Metadata.Name, err)
		}
	}

	// Test policies work
	def := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "INVALID_NAME"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	result := engine.Evaluate(def)
	// Should fail naming convention and require-owner
	if result.Passed {
		t.Error("Expected standard policies to catch violations")
	}
}

func TestPolicyViolation_Fields(t *testing.T) {
	v := PolicyViolation{
		Policy:    "test-policy",
		Rule:      "test-rule",
		Resource:  "test-resource",
		Namespace: "test-ns",
		Field:     "metadata.owner",
		Message:   "Owner is required",
		Severity:  "error",
		Timestamp: time.Now(),
	}

	if v.Policy != "test-policy" {
		t.Errorf("Expected policy 'test-policy', got '%s'", v.Policy)
	}
	if v.Severity != "error" {
		t.Errorf("Expected severity 'error', got '%s'", v.Severity)
	}
}

func TestPolicyResult_Fields(t *testing.T) {
	result := PolicyResult{
		Passed:    false,
		Evaluated: 3,
		Violations: []PolicyViolation{
			{Policy: "p1", Rule: "r1", Message: "error 1", Severity: "error"},
		},
		Warnings: []PolicyViolation{
			{Policy: "p2", Rule: "r2", Message: "warning 1", Severity: "warning"},
		},
		Timestamp: time.Now(),
	}

	if result.Passed {
		t.Error("Expected Passed to be false")
	}
	if result.Evaluated != 3 {
		t.Errorf("Expected Evaluated 3, got %d", result.Evaluated)
	}
	if len(result.Violations) != 1 {
		t.Errorf("Expected 1 violation, got %d", len(result.Violations))
	}
	if len(result.Warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(result.Warnings))
	}
}

func TestPolicyEngine_GetFieldValue(t *testing.T) {
	engine := NewPolicyEngine()

	def := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata: DefinitionMeta{
			Name:      "test_features",
			Namespace: "production",
			Owner:     "ml-team",
			Team:      "data-science",
			Labels: map[string]string{
				"env": "prod",
			},
			Annotations: map[string]string{
				"note": "important",
			},
		},
		Spec: FeatureSpec{
			EntityType:  "user",
			Description: "Test features",
			Features: []FeatureField{
				{Name: "age", DataType: "int64"},
				{Name: "name", DataType: "string"},
			},
			Tags: []string{"user", "profile"},
			TTL:  &Duration{Duration: time.Hour},
		},
	}

	tests := []struct {
		field    string
		expected interface{}
	}{
		{"metadata.name", "test_features"},
		{"metadata.namespace", "production"},
		{"metadata.owner", "ml-team"},
		{"metadata.team", "data-science"},
		{"metadata.labels.env", "prod"},
		{"metadata.annotations.note", "important"},
		{"spec.entityType", "user"},
		{"spec.description", "Test features"},
		{"spec.ttl", time.Hour},
	}

	for _, tt := range tests {
		value := engine.getFieldValue(def, tt.field)
		if value != tt.expected {
			t.Errorf("Field %s: expected %v, got %v", tt.field, tt.expected, value)
		}
	}
}

func TestPolicyEngine_Evaluate_KindFiltering(t *testing.T) {
	engine := NewPolicyEngine()

	policy := &Policy{
		Metadata: PolicyMeta{
			Name:     "feature-group-only",
			Severity: "error",
		},
		Spec: PolicySpec{
			Target: PolicyTarget{
				Kinds: []string{"FeatureGroup"},
			},
			Rules: []PolicyRule{
				{Name: "require-owner", Type: "require", Field: "metadata.owner"},
			},
		},
	}
	engine.RegisterPolicy(policy)

	// Test with matching kind
	defGroup := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "test"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	result := engine.Evaluate(defGroup)
	if result.Passed {
		t.Error("Expected policy to apply and fail")
	}

	// Test with non-matching kind
	defOther := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "TransformPipeline",
		Metadata:   DefinitionMeta{Name: "test"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	result = engine.Evaluate(defOther)
	if result.Evaluated != 0 {
		t.Error("Expected policy to not apply to different kind")
	}
}
