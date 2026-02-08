package contract

import (
	"context"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

func TestEnforcer_ValidatePut_NoContracts(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig(), nil)
	enforcer := NewEnforcer(mgr, DefaultEnforcerConfig())

	features := map[string]*domain.FeatureValue{
		"click_count": {Value: 42, Timestamp: time.Now().UnixNano()},
	}

	err := enforcer.ValidatePut(context.Background(), "user:1", features)
	if err != nil {
		t.Fatalf("expected no error with no contracts, got %v", err)
	}
}

func TestEnforcer_ValidatePut_SchemaViolation_WarnMode(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig(), nil)
	spec := &Spec{
		Name:         "test-contract",
		FeatureGroup: "users",
		FeatureName:  "click_count",
		Rules: []Rule{
			{Type: RuleSchema, Severity: SeverityError, ExpectedType: "int"},
		},
	}
	if err := mgr.CreateContract(spec); err != nil {
		t.Fatal(err)
	}

	enforcer := NewEnforcer(mgr, EnforcerConfig{DefaultMode: ModeWarn})

	// String value should trigger schema violation but not block
	features := map[string]*domain.FeatureValue{
		"click_count": {Value: "not_a_number", Timestamp: time.Now().UnixNano()},
	}

	err := enforcer.ValidatePut(context.Background(), "user:1", features)
	if err != nil {
		t.Fatalf("warn mode should not return error, got %v", err)
	}
	stats := enforcer.Stats()
	if stats.Warned != 1 {
		t.Errorf("expected 1 warned, got %d", stats.Warned)
	}
}

func TestEnforcer_ValidatePut_SchemaViolation_EnforceMode(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig(), nil)
	spec := &Spec{
		Name:         "test-contract",
		FeatureGroup: "users",
		FeatureName:  "click_count",
		Rules: []Rule{
			{Type: RuleSchema, Severity: SeverityError, ExpectedType: "int"},
		},
	}
	if err := mgr.CreateContract(spec); err != nil {
		t.Fatal(err)
	}

	enforcer := NewEnforcer(mgr, EnforcerConfig{DefaultMode: ModeEnforce})

	// String value should block
	features := map[string]*domain.FeatureValue{
		"click_count": {Value: "bad", Timestamp: time.Now().UnixNano()},
	}

	err := enforcer.ValidatePut(context.Background(), "user:1", features)
	if err == nil {
		t.Fatal("enforce mode should return error for schema violation")
	}
	stats := enforcer.Stats()
	if stats.Blocked != 1 {
		t.Errorf("expected 1 blocked, got %d", stats.Blocked)
	}
}

func TestEnforcer_ValidatePut_DistributionBounds(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig(), nil)
	min := 0.0
	max := 100.0
	spec := &Spec{
		Name:         "range-contract",
		FeatureGroup: "users",
		FeatureName:  "score",
		Rules: []Rule{
			{Type: RuleDistribution, MinValue: &min, MaxValue: &max},
		},
	}
	if err := mgr.CreateContract(spec); err != nil {
		t.Fatal(err)
	}

	enforcer := NewEnforcer(mgr, EnforcerConfig{DefaultMode: ModeEnforce})

	// Value within bounds - should pass
	features := map[string]*domain.FeatureValue{
		"score": {Value: 50.0, Timestamp: time.Now().UnixNano()},
	}
	if err := enforcer.ValidatePut(context.Background(), "user:1", features); err != nil {
		t.Fatalf("expected pass for in-range value, got %v", err)
	}

	// Value out of bounds - should block
	features["score"] = &domain.FeatureValue{Value: 150.0, Timestamp: time.Now().UnixNano()}
	if err := enforcer.ValidatePut(context.Background(), "user:2", features); err == nil {
		t.Fatal("expected error for out-of-range value")
	}
}

func TestEnforcer_ValidatePut_Completeness(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig(), nil)
	spec := &Spec{
		Name:         "complete-contract",
		FeatureGroup: "users",
		FeatureName:  "name",
		Rules: []Rule{
			{Type: RuleCompleteness, MinCompleteness: 1.0},
		},
	}
	if err := mgr.CreateContract(spec); err != nil {
		t.Fatal(err)
	}

	enforcer := NewEnforcer(mgr, EnforcerConfig{DefaultMode: ModeEnforce})

	// Nil value should violate completeness
	features := map[string]*domain.FeatureValue{
		"name": nil,
	}
	if err := enforcer.ValidatePut(context.Background(), "user:1", features); err == nil {
		t.Fatal("expected error for nil value with completeness constraint")
	}
}

func TestValidateContractFiles_EmptyPaths(t *testing.T) {
	result, err := ValidateContractFiles(nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.HasErrors {
		t.Error("expected no errors for empty paths")
	}
}
