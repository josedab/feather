package observability

import (
	"context"
	"testing"
	"time"
)

// newTestQualityMonitor creates a QualityMonitor with a nil store (for rule-only tests).
func newTestQualityMonitor() *QualityMonitor {
	return &QualityMonitor{
		store:      nil,
		rules:      make(map[string][]*QualityRule),
		violations: make([]*QualityViolation, 0),
		scores:     make(map[string]*QualityScore),
	}
}

// --- ValidateValue ---

func TestValidateValue_NoRules(t *testing.T) {
	m := newTestQualityMonitor()
	violations := m.ValidateValue(context.Background(), "feat1", "e1", 42.0)
	if violations != nil {
		t.Fatalf("expected nil violations for no rules, got %v", violations)
	}
}

func TestValidateValue_DisabledRule(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "disabled", Feature: "f1", RuleType: "null_check", Enabled: false, Severity: "warning",
	})
	violations := m.ValidateValue(context.Background(), "f1", "e1", nil)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations for disabled rule, got %d", len(violations))
	}
}

// --- checkRule: null_check ---

func TestCheckRule_NullCheck_Nil(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "null_rule", Feature: "f1", RuleType: "null_check", Enabled: true, Severity: "error",
	})
	violations := m.ValidateValue(context.Background(), "f1", "e1", nil)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Expected != "non-null value" {
		t.Fatalf("unexpected expected value: %s", violations[0].Expected)
	}
}

func TestCheckRule_NullCheck_NonNil(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "null_rule", Feature: "f1", RuleType: "null_check", Enabled: true, Severity: "error",
	})
	violations := m.ValidateValue(context.Background(), "f1", "e1", "hello")
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(violations))
	}
}

// --- checkRule: range ---

func TestCheckRule_Range_BelowMin(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "range_rule", Feature: "f1", RuleType: "range", Enabled: true, Severity: "warning",
		Config: map[string]interface{}{"min": 0.0, "max": 100.0},
	})
	violations := m.ValidateValue(context.Background(), "f1", "e1", -5.0)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
}

func TestCheckRule_Range_AboveMax(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "range_rule", Feature: "f1", RuleType: "range", Enabled: true, Severity: "warning",
		Config: map[string]interface{}{"min": 0.0, "max": 100.0},
	})
	violations := m.ValidateValue(context.Background(), "f1", "e1", 200.0)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
}

func TestCheckRule_Range_InRange(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "range_rule", Feature: "f1", RuleType: "range", Enabled: true, Severity: "warning",
		Config: map[string]interface{}{"min": 0.0, "max": 100.0},
	})
	violations := m.ValidateValue(context.Background(), "f1", "e1", 50.0)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(violations))
	}
}

func TestCheckRule_Range_IntValue(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "range_rule", Feature: "f1", RuleType: "range", Enabled: true, Severity: "warning",
		Config: map[string]interface{}{"min": 0.0, "max": 10.0},
	})
	violations := m.ValidateValue(context.Background(), "f1", "e1", 5)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations for int in range, got %d", len(violations))
	}
}

func TestCheckRule_Range_Int64Value(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "range_rule", Feature: "f1", RuleType: "range", Enabled: true, Severity: "warning",
		Config: map[string]interface{}{"min": 0.0, "max": 10.0},
	})
	violations := m.ValidateValue(context.Background(), "f1", "e1", int64(15))
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation for int64 above max, got %d", len(violations))
	}
}

func TestCheckRule_Range_NonNumeric(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "range_rule", Feature: "f1", RuleType: "range", Enabled: true, Severity: "warning",
		Config: map[string]interface{}{"min": 0.0, "max": 100.0},
	})
	violations := m.ValidateValue(context.Background(), "f1", "e1", "not a number")
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations for non-numeric, got %d", len(violations))
	}
}

func TestCheckRule_Range_OnlyMin(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "range_rule", Feature: "f1", RuleType: "range", Enabled: true, Severity: "warning",
		Config: map[string]interface{}{"min": 10.0},
	})
	violations := m.ValidateValue(context.Background(), "f1", "e1", 5.0)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation for below min, got %d", len(violations))
	}
	// Above min should pass
	violations = m.ValidateValue(context.Background(), "f1", "e1", 20.0)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations for above min, got %d", len(violations))
	}
}

// --- checkRule: enum ---

func TestCheckRule_Enum_InSet(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "enum_rule", Feature: "f1", RuleType: "enum", Enabled: true, Severity: "warning",
		Config: map[string]interface{}{"values": []interface{}{"a", "b", "c"}},
	})
	violations := m.ValidateValue(context.Background(), "f1", "e1", "b")
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(violations))
	}
}

func TestCheckRule_Enum_NotInSet(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "enum_rule", Feature: "f1", RuleType: "enum", Enabled: true, Severity: "warning",
		Config: map[string]interface{}{"values": []interface{}{"a", "b", "c"}},
	})
	violations := m.ValidateValue(context.Background(), "f1", "e1", "z")
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
}

func TestCheckRule_Enum_InvalidConfig(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "enum_rule", Feature: "f1", RuleType: "enum", Enabled: true, Severity: "warning",
		Config: map[string]interface{}{"values": "not-a-list"},
	})
	violations := m.ValidateValue(context.Background(), "f1", "e1", "z")
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations for bad config, got %d", len(violations))
	}
}

// --- checkRule: type_check ---

func TestCheckRule_TypeCheck_Match(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "type_rule", Feature: "f1", RuleType: "type_check", Enabled: true, Severity: "warning",
		Config: map[string]interface{}{"type": "string"},
	})
	violations := m.ValidateValue(context.Background(), "f1", "e1", "hello")
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(violations))
	}
}

func TestCheckRule_TypeCheck_Mismatch(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "type_rule", Feature: "f1", RuleType: "type_check", Enabled: true, Severity: "warning",
		Config: map[string]interface{}{"type": "string"},
	})
	violations := m.ValidateValue(context.Background(), "f1", "e1", 42.0)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
}

func TestCheckRule_TypeCheck_AllTypes(t *testing.T) {
	tests := []struct {
		value    interface{}
		expected string
	}{
		{42.0, "float"},
		{42, "int"},
		{int64(42), "int"},
		{"hello", "string"},
		{true, "bool"},
		{[]interface{}{1, 2}, "array"},
		{map[string]interface{}{"k": "v"}, "object"},
	}

	for _, tt := range tests {
		m := newTestQualityMonitor()
		m.AddRule(&QualityRule{
			Name: "type_rule", Feature: "f1", RuleType: "type_check", Enabled: true, Severity: "warning",
			Config: map[string]interface{}{"type": tt.expected},
		})
		violations := m.ValidateValue(context.Background(), "f1", "e1", tt.value)
		if len(violations) != 0 {
			t.Errorf("type %s: expected 0 violations, got %d", tt.expected, len(violations))
		}
	}
}

// --- Multiple rules ---

func TestValidateValue_MultipleRules(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "null_rule", Feature: "f1", RuleType: "null_check", Enabled: true, Severity: "error",
	})
	m.AddRule(&QualityRule{
		Name: "range_rule", Feature: "f1", RuleType: "range", Enabled: true, Severity: "warning",
		Config: map[string]interface{}{"min": 0.0, "max": 100.0},
	})

	violations := m.ValidateValue(context.Background(), "f1", "e1", 200.0)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation (range), got %d", len(violations))
	}
}

// --- Violations storage ---

func TestGetViolations(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "null_rule", Feature: "f1", RuleType: "null_check", Enabled: true, Severity: "error",
	})

	m.ValidateValue(context.Background(), "f1", "e1", nil)
	m.ValidateValue(context.Background(), "f1", "e2", nil)

	violations := m.GetViolations("f1", time.Time{})
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(violations))
	}

	// Filter by time
	future := time.Now().Add(time.Hour)
	violations = m.GetViolations("f1", future)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations after future time, got %d", len(violations))
	}
}

// --- RemoveRule ---

func TestRemoveRule(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "r1", Feature: "f1", RuleType: "null_check", Enabled: true, Severity: "error",
	})
	m.AddRule(&QualityRule{
		Name: "r2", Feature: "f1", RuleType: "null_check", Enabled: true, Severity: "warning",
	})

	m.RemoveRule("r1")
	rules := m.GetRules("f1")
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule after removal, got %d", len(rules))
	}
	if rules[0].Name != "r2" {
		t.Fatalf("expected r2 to remain, got %s", rules[0].Name)
	}
}

// --- GetScore / GetAllScores ---

func TestGetScore_NoScore(t *testing.T) {
	m := newTestQualityMonitor()
	score := m.GetScore("f1")
	if score != nil {
		t.Fatal("expected nil score")
	}
}

func TestGetAllScores_Empty(t *testing.T) {
	m := newTestQualityMonitor()
	scores := m.GetAllScores()
	if len(scores) != 0 {
		t.Fatalf("expected 0 scores, got %d", len(scores))
	}
}

// --- CalculateScore with nil store ---

func TestCalculateScore_EmptySampleEntities(t *testing.T) {
	m := newTestQualityMonitor()
	score := m.CalculateScore(context.Background(), "f1", []string{})
	if score != nil {
		t.Fatal("expected nil score for empty sample")
	}
}

func TestCalculateScore_NilSampleEntities(t *testing.T) {
	m := newTestQualityMonitor()
	score := m.CalculateScore(context.Background(), "f1", nil)
	if score != nil {
		t.Fatal("expected nil score for nil sample")
	}
}

// --- Violations truncation ---

func TestViolations_Truncation(t *testing.T) {
	m := newTestQualityMonitor()
	m.AddRule(&QualityRule{
		Name: "null_rule", Feature: "f1", RuleType: "null_check", Enabled: true, Severity: "error",
	})

	for i := 0; i < 10005; i++ {
		m.ValidateValue(context.Background(), "f1", "e1", nil)
	}

	violations := m.GetViolations("", time.Time{})
	if len(violations) != 10000 {
		t.Fatalf("expected 10000 violations after truncation, got %d", len(violations))
	}
}

// --- GetRules for non-existent feature ---

func TestGetRules_NonExistent(t *testing.T) {
	m := newTestQualityMonitor()
	rules := m.GetRules("nonexistent")
	if rules != nil {
		t.Fatalf("expected nil rules, got %v", rules)
	}
}
