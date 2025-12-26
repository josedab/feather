package quality

import (
	"context"
	"testing"
	"time"
)

func TestValidator_AddRule(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name    string
		rule    *ValidationRule
		wantErr bool
	}{
		{
			name: "valid not null rule",
			rule: &ValidationRule{
				ID:        "rule-1",
				Name:      "Not Null Check",
				Type:      RuleTypeNotNull,
				FeatureID: "feature-1",
				Severity:  SeverityError,
			},
			wantErr: false,
		},
		{
			name: "valid range rule",
			rule: &ValidationRule{
				ID:        "rule-2",
				Name:      "Range Check",
				Type:      RuleTypeRange,
				FeatureID: "feature-1",
				Severity:  SeverityWarning,
				Config: map[string]interface{}{
					"min":           0.0,
					"max":           100.0,
					"min_inclusive": true,
					"max_inclusive": true,
				},
			},
			wantErr: false,
		},
		{
			name: "valid pattern rule",
			rule: &ValidationRule{
				ID:        "rule-3",
				Name:      "Email Pattern",
				Type:      RuleTypePattern,
				FeatureID: "feature-1",
				Severity:  SeverityError,
				Config: map[string]interface{}{
					"regex": `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
				},
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			rule: &ValidationRule{
				Name:      "No ID",
				Type:      RuleTypeNotNull,
				FeatureID: "feature-1",
			},
			wantErr: true,
		},
		{
			name: "missing feature ID",
			rule: &ValidationRule{
				ID:   "rule-4",
				Name: "No Feature",
				Type: RuleTypeNotNull,
			},
			wantErr: true,
		},
		{
			name: "invalid regex pattern",
			rule: &ValidationRule{
				ID:        "rule-5",
				Name:      "Bad Pattern",
				Type:      RuleTypePattern,
				FeatureID: "feature-1",
				Config: map[string]interface{}{
					"regex": `[invalid`,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.AddRule(tt.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_RemoveRule(t *testing.T) {
	v := NewValidator()

	rule := &ValidationRule{
		ID:        "rule-1",
		Name:      "Test Rule",
		Type:      RuleTypeNotNull,
		FeatureID: "feature-1",
		GroupID:   "group-1",
	}
	v.AddRule(rule)

	// Verify rule exists
	if _, err := v.GetRule("rule-1"); err != nil {
		t.Fatalf("rule should exist: %v", err)
	}

	// Remove rule
	if err := v.RemoveRule("rule-1"); err != nil {
		t.Fatalf("RemoveRule() error = %v", err)
	}

	// Verify rule is gone
	if _, err := v.GetRule("rule-1"); err == nil {
		t.Error("rule should not exist after removal")
	}

	// Remove non-existent rule
	if err := v.RemoveRule("nonexistent"); err == nil {
		t.Error("expected error for nonexistent rule")
	}
}

func TestValidator_ValidateValue_NotNull(t *testing.T) {
	v := NewValidator()

	v.AddRule(&ValidationRule{
		ID:        "not-null",
		Name:      "Not Null",
		Type:      RuleTypeNotNull,
		FeatureID: "feature-1",
		Severity:  SeverityError,
	})

	ctx := context.Background()

	// Valid value
	results := v.ValidateValue(ctx, "feature-1", "hello", nil)
	if len(results) != 1 || !results[0].Passed {
		t.Error("expected validation to pass for non-null value")
	}

	// Null value
	results = v.ValidateValue(ctx, "feature-1", nil, nil)
	if len(results) != 1 || results[0].Passed {
		t.Error("expected validation to fail for null value")
	}
}

func TestValidator_ValidateValue_Range(t *testing.T) {
	v := NewValidator()

	v.AddRule(&ValidationRule{
		ID:        "range",
		Name:      "Range Check",
		Type:      RuleTypeRange,
		FeatureID: "feature-1",
		Severity:  SeverityWarning,
		Config: map[string]interface{}{
			"min":           0.0,
			"max":           100.0,
			"min_inclusive": true,
			"max_inclusive": true,
		},
	})

	ctx := context.Background()

	tests := []struct {
		name   string
		value  interface{}
		passed bool
	}{
		{"in range", 50.0, true},
		{"at min", 0.0, true},
		{"at max", 100.0, true},
		{"below min", -1.0, false},
		{"above max", 101.0, false},
		{"non-numeric", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := v.ValidateValue(ctx, "feature-1", tt.value, nil)
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			if results[0].Passed != tt.passed {
				t.Errorf("expected passed=%v, got %v", tt.passed, results[0].Passed)
			}
		})
	}
}

func TestValidator_ValidateValue_Pattern(t *testing.T) {
	v := NewValidator()

	v.AddRule(&ValidationRule{
		ID:        "email",
		Name:      "Email Pattern",
		Type:      RuleTypePattern,
		FeatureID: "feature-1",
		Severity:  SeverityError,
		Config: map[string]interface{}{
			"regex": `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
		},
	})

	ctx := context.Background()

	tests := []struct {
		name   string
		value  string
		passed bool
	}{
		{"valid email", "test@example.com", true},
		{"valid email with dots", "test.user@example.co.uk", true},
		{"invalid no @", "testexample.com", false},
		{"invalid no domain", "test@", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := v.ValidateValue(ctx, "feature-1", tt.value, nil)
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			if results[0].Passed != tt.passed {
				t.Errorf("expected passed=%v, got %v", tt.passed, results[0].Passed)
			}
		})
	}
}

func TestValidator_ValidateValue_Enum(t *testing.T) {
	v := NewValidator()

	v.AddRule(&ValidationRule{
		ID:        "status",
		Name:      "Status Enum",
		Type:      RuleTypeEnum,
		FeatureID: "feature-1",
		Severity:  SeverityError,
		Config: map[string]interface{}{
			"allowed_values": []interface{}{"active", "inactive", "pending"},
			"case_sensitive": false,
		},
	})

	ctx := context.Background()

	tests := []struct {
		name   string
		value  string
		passed bool
	}{
		{"valid lowercase", "active", true},
		{"valid uppercase", "ACTIVE", true},
		{"valid mixed case", "Active", true},
		{"invalid value", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := v.ValidateValue(ctx, "feature-1", tt.value, nil)
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			if results[0].Passed != tt.passed {
				t.Errorf("expected passed=%v, got %v", tt.passed, results[0].Passed)
			}
		})
	}
}

func TestValidator_ValidateValue_Freshness(t *testing.T) {
	v := NewValidator()

	v.AddRule(&ValidationRule{
		ID:        "freshness",
		Name:      "Freshness Check",
		Type:      RuleTypeFreshness,
		FeatureID: "feature-1",
		Severity:  SeverityWarning,
		Config: map[string]interface{}{
			"max_age": "1h",
		},
	})

	ctx := context.Background()

	// Fresh data
	results := v.ValidateValue(ctx, "feature-1", "value", map[string]interface{}{
		"timestamp": time.Now().Add(-30 * time.Minute),
	})
	if len(results) != 1 || !results[0].Passed {
		t.Error("expected validation to pass for fresh data")
	}

	// Stale data
	results = v.ValidateValue(ctx, "feature-1", "value", map[string]interface{}{
		"timestamp": time.Now().Add(-2 * time.Hour),
	})
	if len(results) != 1 || results[0].Passed {
		t.Error("expected validation to fail for stale data")
	}
}

func TestValidator_ValidateValue_Custom(t *testing.T) {
	v := NewValidator()

	v.AddRule(&ValidationRule{
		ID:        "positive",
		Name:      "Positive Check",
		Type:      RuleTypeCustom,
		FeatureID: "feature-1",
		Severity:  SeverityError,
		Config: map[string]interface{}{
			"expression": "positive",
		},
	})

	ctx := context.Background()

	// Positive value
	results := v.ValidateValue(ctx, "feature-1", 10.0, nil)
	if len(results) != 1 || !results[0].Passed {
		t.Error("expected validation to pass for positive value")
	}

	// Zero
	results = v.ValidateValue(ctx, "feature-1", 0.0, nil)
	if len(results) != 1 || results[0].Passed {
		t.Error("expected validation to fail for zero")
	}

	// Negative
	results = v.ValidateValue(ctx, "feature-1", -5.0, nil)
	if len(results) != 1 || results[0].Passed {
		t.Error("expected validation to fail for negative value")
	}
}

func TestValidator_ValidateBatch(t *testing.T) {
	v := NewValidator()

	v.AddRule(&ValidationRule{
		ID:        "not-null",
		Name:      "Not Null",
		Type:      RuleTypeNotNull,
		FeatureID: "feature-1",
		Severity:  SeverityError,
	})

	v.AddRule(&ValidationRule{
		ID:        "range",
		Name:      "Range",
		Type:      RuleTypeRange,
		FeatureID: "feature-1",
		Severity:  SeverityWarning,
		Config: map[string]interface{}{
			"min":           0.0,
			"max":           100.0,
			"min_inclusive": true,
			"max_inclusive": true,
		},
	})

	ctx := context.Background()
	values := []interface{}{10.0, 50.0, 75.0, 150.0, nil}

	report := v.ValidateBatch(ctx, "feature-1", values, nil)

	if report.TotalRules != 2 {
		t.Errorf("expected 2 total rules, got %d", report.TotalRules)
	}

	// Both rules should have some failures
	if report.FailedRules == 0 {
		t.Error("expected some rules to fail")
	}

	if report.Duration == 0 {
		t.Error("expected duration to be set")
	}

	// Check failure rates
	for _, result := range report.Results {
		if result.SampleSize != 5 {
			t.Errorf("expected sample size 5, got %d", result.SampleSize)
		}
	}
}

func TestValidator_DisabledRules(t *testing.T) {
	v := NewValidator()

	rule := &ValidationRule{
		ID:        "disabled",
		Name:      "Disabled Rule",
		Type:      RuleTypeNotNull,
		FeatureID: "feature-1",
		Severity:  SeverityError,
	}
	v.AddRule(rule)

	// Disable the rule
	v.mu.Lock()
	v.rules["disabled"].Enabled = false
	v.mu.Unlock()

	ctx := context.Background()
	results := v.ValidateValue(ctx, "feature-1", nil, nil)

	// Should return no results for disabled rules
	if len(results) != 0 {
		t.Errorf("expected 0 results for disabled rule, got %d", len(results))
	}
}

func TestValidator_GetRulesForFeature(t *testing.T) {
	v := NewValidator()

	v.AddRule(&ValidationRule{
		ID:        "rule-1",
		Name:      "Rule 1",
		Type:      RuleTypeNotNull,
		FeatureID: "feature-1",
	})

	v.AddRule(&ValidationRule{
		ID:        "rule-2",
		Name:      "Rule 2",
		Type:      RuleTypeRange,
		FeatureID: "feature-1",
	})

	v.AddRule(&ValidationRule{
		ID:        "rule-3",
		Name:      "Rule 3",
		Type:      RuleTypeNotNull,
		FeatureID: "feature-2",
	})

	rules := v.GetRulesForFeature("feature-1")
	if len(rules) != 2 {
		t.Errorf("expected 2 rules for feature-1, got %d", len(rules))
	}

	rules = v.GetRulesForFeature("feature-2")
	if len(rules) != 1 {
		t.Errorf("expected 1 rule for feature-2, got %d", len(rules))
	}

	rules = v.GetRulesForFeature("nonexistent")
	if len(rules) != 0 {
		t.Errorf("expected 0 rules for nonexistent feature, got %d", len(rules))
	}
}

func TestValidator_CalculateQualityScore(t *testing.T) {
	v := NewValidator()

	// Add some rules
	v.AddRule(&ValidationRule{
		ID:        "rule-1",
		Name:      "Not Null",
		Type:      RuleTypeNotNull,
		FeatureID: "feature-1",
		Severity:  SeverityError,
	})

	v.AddRule(&ValidationRule{
		ID:        "rule-2",
		Name:      "Freshness",
		Type:      RuleTypeFreshness,
		FeatureID: "feature-1",
		Severity:  SeverityWarning,
		Config: map[string]interface{}{
			"max_age": "1h",
		},
	})

	ctx := context.Background()

	// Run some validations
	v.ValidateBatch(ctx, "feature-1", []interface{}{1, 2, 3}, map[string]interface{}{
		"timestamp": time.Now(),
	})

	score := v.CalculateQualityScore("feature-1")

	if score.FeatureID != "feature-1" {
		t.Errorf("expected feature ID feature-1, got %s", score.FeatureID)
	}

	if score.OverallScore < 0 || score.OverallScore > 1 {
		t.Errorf("quality score should be between 0 and 1, got %f", score.OverallScore)
	}

	if score.LastUpdated.IsZero() {
		t.Error("expected LastUpdated to be set")
	}
}

func TestValidator_GetStats(t *testing.T) {
	v := NewValidator()

	v.AddRule(&ValidationRule{
		ID:        "rule-1",
		Type:      RuleTypeNotNull,
		FeatureID: "feature-1",
	})
	v.AddRule(&ValidationRule{
		ID:        "rule-2",
		Type:      RuleTypeNotNull,
		FeatureID: "feature-2",
	})
	v.AddRule(&ValidationRule{
		ID:        "rule-3",
		Type:      RuleTypeRange,
		FeatureID: "feature-1",
	})

	stats := v.GetStats()

	if stats["total_rules"].(int) != 3 {
		t.Errorf("expected 3 total rules, got %v", stats["total_rules"])
	}

	if stats["features_covered"].(int) != 2 {
		t.Errorf("expected 2 features covered, got %v", stats["features_covered"])
	}

	rulesByType := stats["rules_by_type"].(map[string]int)
	if rulesByType["not_null"] != 2 {
		t.Errorf("expected 2 not_null rules, got %d", rulesByType["not_null"])
	}
}

func TestValidator_ListRules(t *testing.T) {
	v := NewValidator()

	for i := 0; i < 5; i++ {
		v.AddRule(&ValidationRule{
			ID:        string(rune('a' + i)),
			Type:      RuleTypeNotNull,
			FeatureID: "feature-1",
		})
	}

	rules := v.ListRules()
	if len(rules) != 5 {
		t.Errorf("expected 5 rules, got %d", len(rules))
	}
}

func TestValidator_GetQualityHistory(t *testing.T) {
	v := NewValidator()

	v.AddRule(&ValidationRule{
		ID:        "rule-1",
		Type:      RuleTypeNotNull,
		FeatureID: "feature-1",
	})

	ctx := context.Background()

	// Generate some history
	for i := 0; i < 10; i++ {
		v.ValidateBatch(ctx, "feature-1", []interface{}{1, 2, 3}, nil)
	}

	history := v.GetQualityHistory(5)
	if len(history) != 5 {
		t.Errorf("expected 5 history items, got %d", len(history))
	}

	// Get all
	history = v.GetQualityHistory(0)
	if len(history) != 10 {
		t.Errorf("expected 10 history items, got %d", len(history))
	}
}

func TestValidateConsistency(t *testing.T) {
	v := NewValidator()

	v.AddRule(&ValidationRule{
		ID:        "consistency",
		Name:      "Consistency Check",
		Type:      RuleTypeConsistency,
		FeatureID: "feature-1",
		Severity:  SeverityError,
		Config: map[string]interface{}{
			"reference_field": "expected",
			"comparator":      "eq",
		},
	})

	ctx := context.Background()

	// Matching values
	results := v.ValidateValue(ctx, "feature-1", 100, map[string]interface{}{
		"expected": 100,
	})
	if len(results) != 1 || !results[0].Passed {
		t.Error("expected validation to pass for matching values")
	}

	// Non-matching values
	results = v.ValidateValue(ctx, "feature-1", 100, map[string]interface{}{
		"expected": 200,
	})
	if len(results) != 1 || results[0].Passed {
		t.Error("expected validation to fail for non-matching values")
	}
}

func TestEqualFold(t *testing.T) {
	tests := []struct {
		s1, s2 string
		want   bool
	}{
		{"hello", "hello", true},
		{"Hello", "hello", true},
		{"HELLO", "hello", true},
		{"hello", "world", false},
		{"", "", true},
		{"a", "ab", false},
	}

	for _, tt := range tests {
		if got := equalFold(tt.s1, tt.s2); got != tt.want {
			t.Errorf("equalFold(%q, %q) = %v, want %v", tt.s1, tt.s2, got, tt.want)
		}
	}
}
