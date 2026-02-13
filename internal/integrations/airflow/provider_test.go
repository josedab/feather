package airflow

import (
	"testing"
)

func newTestProvider() *Provider {
	return NewProvider(DefaultConfig())
}

func validOperator(id, name, opType string) *DAGOperator {
	return &DAGOperator{
		ID:   id,
		Name: name,
		Type: opType,
	}
}

// --- RegisterOperator ---

func TestRegisterOperator_Valid(t *testing.T) {
	p := newTestProvider()
	err := p.RegisterOperator(validOperator("op1", "Operator 1", "feature_compute"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegisterOperator_AllTypes(t *testing.T) {
	p := newTestProvider()
	for _, typ := range []string{"feature_compute", "feature_backfill", "freshness_sensor"} {
		err := p.RegisterOperator(validOperator("op-"+typ, "Op", typ))
		if err != nil {
			t.Fatalf("unexpected error for type %s: %v", typ, err)
		}
	}
}

func TestRegisterOperator_InvalidType(t *testing.T) {
	p := newTestProvider()
	err := p.RegisterOperator(validOperator("op1", "Op", "invalid_type"))
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestRegisterOperator_MissingID(t *testing.T) {
	p := newTestProvider()
	err := p.RegisterOperator(&DAGOperator{ID: "", Name: "Op", Type: "feature_compute"})
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
}

func TestRegisterOperator_MissingName(t *testing.T) {
	p := newTestProvider()
	err := p.RegisterOperator(&DAGOperator{ID: "op1", Name: "", Type: "feature_compute"})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestRegisterOperator_Duplicate(t *testing.T) {
	p := newTestProvider()
	_ = p.RegisterOperator(validOperator("op1", "Op1", "feature_compute"))
	err := p.RegisterOperator(validOperator("op1", "Op2", "feature_compute"))
	if err == nil {
		t.Fatal("expected error for duplicate operator")
	}
}

func TestRegisterOperator_SetsCreatedAt(t *testing.T) {
	p := newTestProvider()
	op := validOperator("op1", "Op1", "feature_compute")
	_ = p.RegisterOperator(op)
	if op.CreatedAt.IsZero() {
		t.Fatal("expected created_at to be set")
	}
}

// --- GetOperator ---

func TestGetOperator_Found(t *testing.T) {
	p := newTestProvider()
	_ = p.RegisterOperator(validOperator("op1", "Op1", "feature_compute"))
	op, err := p.GetOperator("op1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op.ID != "op1" {
		t.Fatalf("expected op1, got %s", op.ID)
	}
}

func TestGetOperator_NotFound(t *testing.T) {
	p := newTestProvider()
	_, err := p.GetOperator("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent operator")
	}
}

// --- EnableOperator / DisableOperator ---

func TestEnableDisableOperator(t *testing.T) {
	p := newTestProvider()
	_ = p.RegisterOperator(validOperator("op1", "Op1", "feature_compute"))

	err := p.EnableOperator("op1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	op, _ := p.GetOperator("op1")
	if !op.Enabled {
		t.Fatal("expected operator to be enabled")
	}

	err = p.DisableOperator("op1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	op, _ = p.GetOperator("op1")
	if op.Enabled {
		t.Fatal("expected operator to be disabled")
	}
}

func TestEnableOperator_NotFound(t *testing.T) {
	p := newTestProvider()
	err := p.EnableOperator("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent operator")
	}
}

func TestDisableOperator_NotFound(t *testing.T) {
	p := newTestProvider()
	err := p.DisableOperator("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent operator")
	}
}

// --- ListOperators ---

func TestListOperators(t *testing.T) {
	p := newTestProvider()
	_ = p.RegisterOperator(validOperator("op1", "Op1", "feature_compute"))
	_ = p.RegisterOperator(validOperator("op2", "Op2", "feature_backfill"))

	ops := p.ListOperators()
	if len(ops) != 2 {
		t.Fatalf("expected 2 operators, got %d", len(ops))
	}
}

func TestListOperators_Empty(t *testing.T) {
	p := newTestProvider()
	ops := p.ListOperators()
	if len(ops) != 0 {
		t.Fatalf("expected 0 operators, got %d", len(ops))
	}
}

// --- CheckFreshness ---

func TestCheckFreshness_NoSensor(t *testing.T) {
	p := newTestProvider()
	result, err := p.CheckFreshness("feature1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsFresh {
		t.Fatal("expected fresh when no sensor")
	}
	if result.OperatorID != "" {
		t.Fatalf("expected empty operator ID, got %s", result.OperatorID)
	}
}

func TestCheckFreshness_WithSensor(t *testing.T) {
	p := newTestProvider()
	op := &DAGOperator{
		ID:         "sensor1",
		Name:       "Freshness Sensor",
		Type:       "freshness_sensor",
		FeatureIDs: []string{"feature1"},
		Enabled:    true,
	}
	_ = p.RegisterOperator(op)
	_ = p.EnableOperator("sensor1")

	result, err := p.CheckFreshness("feature1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OperatorID != "sensor1" {
		t.Fatalf("expected operator sensor1, got %s", result.OperatorID)
	}
	if !result.IsFresh {
		t.Fatal("expected fresh when no last run")
	}
}

func TestCheckFreshness_NonexistentFeature(t *testing.T) {
	p := newTestProvider()
	result, err := p.CheckFreshness("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsFresh {
		t.Fatal("expected fresh for nonexistent feature (no sensor)")
	}
}

// --- ListSensorResults ---

func TestListSensorResults(t *testing.T) {
	p := newTestProvider()
	_, _ = p.CheckFreshness("f1")
	_, _ = p.CheckFreshness("f2")

	results := p.ListSensorResults()
	if len(results) != 2 {
		t.Fatalf("expected 2 sensor results, got %d", len(results))
	}
}

// --- Stats ---

func TestStats(t *testing.T) {
	p := newTestProvider()
	_ = p.RegisterOperator(validOperator("op1", "Op1", "feature_compute"))
	_ = p.RegisterOperator(validOperator("op2", "Op2", "freshness_sensor"))
	_ = p.EnableOperator("op1")

	_, _ = p.CheckFreshness("f1")

	stats := p.Stats()
	if stats.TotalOperators != 2 {
		t.Fatalf("expected 2 total operators, got %d", stats.TotalOperators)
	}
	if stats.ActiveOperators != 1 {
		t.Fatalf("expected 1 active operator, got %d", stats.ActiveOperators)
	}
	if stats.SensorChecks != 1 {
		t.Fatalf("expected 1 sensor check, got %d", stats.SensorChecks)
	}
}
