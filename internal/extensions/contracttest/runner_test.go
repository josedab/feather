package contracttest

import (
	"errors"
	"testing"
)

func TestNewRunner(t *testing.T) {
	r := NewRunner(DefaultRunnerConfig())
	if r == nil {
		t.Fatal("NewRunner returned nil")
	}

	if len(r.contracts) != 0 {
		t.Errorf("expected empty contracts, got %d", len(r.contracts))
	}

	// Zero config should use defaults
	r2 := NewRunner(RunnerConfig{})
	if r2.config.MaxContracts != 10000 {
		t.Errorf("expected default MaxContracts 10000, got %d", r2.config.MaxContracts)
	}
}

func TestRegisterContract(t *testing.T) {
	r := NewRunner(DefaultRunnerConfig())

	c := Contract{
		ID:           "schema-1",
		Name:         "User Schema",
		FeatureGroup: "users",
		Type:         SchemaContract,
		Rules: map[string]interface{}{
			"user_id": "string",
			"age":     "int",
		},
	}

	if err := r.RegisterContract(c); err != nil {
		t.Fatalf("RegisterContract failed: %v", err)
	}

	contracts := r.ListContracts()
	if len(contracts) != 1 {
		t.Fatalf("expected 1 contract, got %d", len(contracts))
	}

	// Invalid contract
	if err := r.RegisterContract(Contract{}); err == nil {
		t.Error("expected error for empty contract")
	} else if !errors.Is(err, ErrInvalidContract) {
		t.Errorf("expected ErrInvalidContract, got %v", err)
	}
}

func TestValidateSchema(t *testing.T) {
	r := NewRunner(DefaultRunnerConfig())

	c := Contract{
		ID:           "schema-1",
		Name:         "User Schema",
		FeatureGroup: "users",
		Type:         SchemaContract,
		Rules: map[string]interface{}{
			"user_id": "string",
			"age":     "int",
			"email":   "string",
		},
	}
	_ = r.RegisterContract(c)

	// Valid schema
	result := r.ValidateSchema("schema-1", map[string]string{
		"user_id": "string",
		"age":     "int",
		"email":   "string",
	})

	if !result.Passed {
		t.Errorf("expected validation to pass, got violations: %v", result.Violations)
	}

	// Missing field
	result = r.ValidateSchema("schema-1", map[string]string{
		"user_id": "string",
		"age":     "int",
	})

	if result.Passed {
		t.Error("expected validation to fail for missing email field")
	}

	if len(result.Violations) != 1 {
		t.Errorf("expected 1 violation, got %d", len(result.Violations))
	}

	// Wrong type
	result = r.ValidateSchema("schema-1", map[string]string{
		"user_id": "string",
		"age":     "string", // Wrong type
		"email":   "string",
	})

	if result.Passed {
		t.Error("expected validation to fail for wrong type")
	}

	// Non-existent contract
	result = r.ValidateSchema("nonexistent", map[string]string{})
	if result.Passed {
		t.Error("expected validation to fail for nonexistent contract")
	}
}

func TestValidateRange(t *testing.T) {
	r := NewRunner(DefaultRunnerConfig())

	c := Contract{
		ID:           "range-1",
		Name:         "Age Range",
		FeatureGroup: "users",
		Type:         RangeContract,
		Rules: map[string]interface{}{
			"age": map[string]interface{}{
				"min": float64(0),
				"max": float64(150),
			},
			"score": map[string]interface{}{
				"min": float64(0),
				"max": float64(1.0),
			},
		},
	}
	_ = r.RegisterContract(c)

	// Valid values
	result := r.ValidateRange("range-1", map[string]float64{
		"age":   25,
		"score": 0.85,
	})

	if !result.Passed {
		t.Errorf("expected validation to pass, got violations: %v", result.Violations)
	}

	// Below minimum
	result = r.ValidateRange("range-1", map[string]float64{
		"age": -5,
	})

	if result.Passed {
		t.Error("expected validation to fail for negative age")
	}

	// Above maximum
	result = r.ValidateRange("range-1", map[string]float64{
		"score": 1.5,
	})

	if result.Passed {
		t.Error("expected validation to fail for score > 1.0")
	}
}

func TestDeleteContract(t *testing.T) {
	r := NewRunner(DefaultRunnerConfig())

	c := Contract{
		ID:   "del-1",
		Name: "To Delete",
		Type: SchemaContract,
	}
	_ = r.RegisterContract(c)

	if err := r.DeleteContract("del-1"); err != nil {
		t.Fatalf("DeleteContract failed: %v", err)
	}

	if _, err := r.GetContract("del-1"); err == nil {
		t.Error("expected error after deletion")
	}

	// Delete non-existent
	if err := r.DeleteContract("nonexistent"); !errors.Is(err, ErrContractNotFound) {
		t.Errorf("expected ErrContractNotFound, got %v", err)
	}
}

func TestStats(t *testing.T) {
	r := NewRunner(DefaultRunnerConfig())

	c := Contract{
		ID:       "stats-1",
		Name:     "Stats Schema",
		Type:     SchemaContract,
		Severity: "block",
		Rules: map[string]interface{}{
			"required_field": "string",
		},
	}
	_ = r.RegisterContract(c)

	// One pass, one fail
	r.ValidateSchema("stats-1", map[string]string{"required_field": "string"})
	r.ValidateSchema("stats-1", map[string]string{})

	stats := r.Stats()

	if stats.TotalContracts != 1 {
		t.Errorf("expected 1 contract, got %d", stats.TotalContracts)
	}

	if stats.TotalRuns != 2 {
		t.Errorf("expected 2 runs, got %d", stats.TotalRuns)
	}

	if stats.PassRate != 0.5 {
		t.Errorf("expected 50%% pass rate, got %f", stats.PassRate)
	}

	if stats.BlockingViolations != 1 {
		t.Errorf("expected 1 blocking violation, got %d", stats.BlockingViolations)
	}
}
