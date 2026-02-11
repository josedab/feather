package contract

import (
	"testing"
	"time"
)

func TestValidateDefinition_Valid(t *testing.T) {
	def := &ContractDefinition{
		Version:      "v1",
		Name:         "user-features-contract",
		FeatureGroup: "user_features",
		Mode:         ModeEnforce,
		Rules: []RuleDefinition{
			{Type: RuleFreshness, Severity: SeverityError, MaxStaleness: "1h"},
			{Type: RuleCompleteness, Severity: SeverityWarning, MinCompleteness: 0.95},
		},
	}
	result := ValidateDefinition(def)
	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}
}

func TestValidateDefinition_MissingName(t *testing.T) {
	def := &ContractDefinition{
		FeatureGroup: "group",
		Rules:        []RuleDefinition{{Type: RuleFreshness, MaxStaleness: "5m"}},
	}
	result := ValidateDefinition(def)
	if result.Valid {
		t.Error("expected invalid for missing name")
	}
}

func TestValidateDefinition_NoRules(t *testing.T) {
	def := &ContractDefinition{
		Name:         "test",
		FeatureGroup: "group",
	}
	result := ValidateDefinition(def)
	if result.Valid {
		t.Error("expected invalid for no rules")
	}
}

func TestValidateDefinition_InvalidDuration(t *testing.T) {
	def := &ContractDefinition{
		Name:         "test",
		FeatureGroup: "group",
		Rules: []RuleDefinition{
			{Type: RuleFreshness, MaxStaleness: "not-a-duration"},
		},
	}
	result := ValidateDefinition(def)
	if result.Valid {
		t.Error("expected invalid for bad duration")
	}
}

func TestValidateDefinition_InvalidMode(t *testing.T) {
	def := &ContractDefinition{
		Name:         "test",
		FeatureGroup: "group",
		Mode:         "invalid",
		Rules:        []RuleDefinition{{Type: RuleFreshness, MaxStaleness: "5m"}},
	}
	result := ValidateDefinition(def)
	if result.Valid {
		t.Error("expected invalid for bad mode")
	}
}

func TestToSpec(t *testing.T) {
	def := &ContractDefinition{
		Name:         "test-contract",
		FeatureGroup: "user_features",
		Mode:         ModeWarn,
		Owner:        "ml-team",
		Rules: []RuleDefinition{
			{Type: RuleFreshness, Severity: SeverityError, MaxStaleness: "30m"},
			{Type: RuleCompleteness, MinCompleteness: 0.9},
		},
	}

	spec, err := def.ToSpec()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Name != "test-contract" {
		t.Errorf("expected name 'test-contract', got %q", spec.Name)
	}
	if len(spec.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(spec.Rules))
	}
	if spec.Rules[0].MaxStaleness != 30*time.Minute {
		t.Errorf("expected 30m staleness, got %v", spec.Rules[0].MaxStaleness)
	}
	// Second rule should get default severity.
	if spec.Rules[1].Severity != SeverityWarning {
		t.Errorf("expected default severity 'warning', got %q", spec.Rules[1].Severity)
	}
}

func TestToSpec_Invalid(t *testing.T) {
	def := &ContractDefinition{
		Name: "test",
	}
	_, err := def.ToSpec()
	if err == nil {
		t.Error("expected error for invalid definition")
	}
}

func TestDiffContracts(t *testing.T) {
	old := &Spec{
		Name:         "test",
		FeatureGroup: "group_a",
		Rules: []Rule{
			{Type: RuleFreshness, Severity: SeverityError, MaxStaleness: 30 * time.Minute},
			{Type: RuleCompleteness, Severity: SeverityWarning, MinCompleteness: 0.95},
		},
	}
	newSpec := &Spec{
		Name:         "test",
		FeatureGroup: "group_a",
		Rules: []Rule{
			{Type: RuleFreshness, Severity: SeverityError, MaxStaleness: 15 * time.Minute},
			{Type: RuleSchema, Severity: SeverityError, ExpectedType: "int64"},
		},
	}

	diff := DiffContracts(old, newSpec)
	if len(diff.Added) != 1 {
		t.Errorf("expected 1 added rule, got %d", len(diff.Added))
	}
	if len(diff.Removed) != 1 {
		t.Errorf("expected 1 removed rule, got %d", len(diff.Removed))
	}
}

func TestDiffContracts_Breaking(t *testing.T) {
	old := &Spec{
		Name:         "test",
		FeatureGroup: "group_a",
		Rules:        []Rule{{Type: RuleFreshness, Severity: SeverityCritical}},
	}
	newSpec := &Spec{
		Name:         "test",
		FeatureGroup: "group_b",
	}
	diff := DiffContracts(old, newSpec)
	if !diff.Breaking {
		t.Error("expected breaking change when feature_group changes")
	}
}

func TestGenerateReport(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig(), nil)
	spec1 := &Spec{
		Name:         "c1",
		FeatureGroup: "g1",
		Rules:        []Rule{{Type: RuleFreshness, Severity: SeverityWarning, MaxStaleness: time.Hour}},
	}
	spec2 := &Spec{
		Name:         "c2",
		FeatureGroup: "g2",
		Rules:        []Rule{{Type: RuleCompleteness, Severity: SeverityError, MinCompleteness: 0.9}},
	}
	if err := mgr.CreateContract(spec1); err != nil {
		t.Fatal(err)
	}
	if err := mgr.CreateContract(spec2); err != nil {
		t.Fatal(err)
	}

	report := mgr.GenerateReport()
	if report.TotalContracts != 2 {
		t.Errorf("expected 2 contracts, got %d", report.TotalContracts)
	}
}
