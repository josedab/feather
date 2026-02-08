package contract

import (
	"context"
	"testing"
	"time"
)

type mockProvider struct {
	metrics *FeatureMetrics
	err     error
}

func (m *mockProvider) GetFeatureMetrics(_ context.Context, _, _ string) (*FeatureMetrics, error) {
	return m.metrics, m.err
}

type mockAlertHandler struct {
	violations []Violation
}

func (m *mockAlertHandler) HandleViolation(_ context.Context, v Violation) error {
	m.violations = append(m.violations, v)
	return nil
}

func TestManager_CreateContract(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig(), nil)

	tests := []struct {
		name    string
		spec    *Spec
		wantErr error
	}{
		{
			name: "valid contract",
			spec: &Spec{
				Name:         "test-contract",
				FeatureGroup: "user_features",
				Rules: []Rule{
					{Type: RuleFreshness, MaxStaleness: 5 * time.Minute, Severity: SeverityError},
				},
			},
			wantErr: nil,
		},
		{
			name: "missing name",
			spec: &Spec{
				FeatureGroup: "user_features",
				Rules:        []Rule{{Type: RuleFreshness}},
			},
			wantErr: ErrInvalidContract,
		},
		{
			name: "missing group",
			spec: &Spec{
				Name:  "test",
				Rules: []Rule{{Type: RuleFreshness}},
			},
			wantErr: ErrInvalidContract,
		},
		{
			name: "no rules",
			spec: &Spec{
				Name:         "test",
				FeatureGroup: "group",
				Rules:        []Rule{},
			},
			wantErr: ErrNoRules,
		},
		{
			name: "invalid rule type",
			spec: &Spec{
				Name:         "test",
				FeatureGroup: "group",
				Rules:        []Rule{{Type: "bogus"}},
			},
			wantErr: ErrInvalidRuleType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.CreateContract(tt.spec)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error containing %v, got nil", tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestManager_DuplicateContract(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig(), nil)
	spec := &Spec{
		Name:         "dup",
		FeatureGroup: "group",
		Rules:        []Rule{{Type: RuleFreshness, MaxStaleness: time.Minute}},
	}
	if err := mgr.CreateContract(spec); err != nil {
		t.Fatal(err)
	}
	err := mgr.CreateContract(spec)
	if err != ErrContractExists {
		t.Fatalf("expected ErrContractExists, got %v", err)
	}
}

func TestManager_CRUD(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig(), nil)
	spec := &Spec{
		Name:         "crud-test",
		FeatureGroup: "group",
		Rules:        []Rule{{Type: RuleCompleteness, MinCompleteness: 0.95}},
	}

	// Create
	if err := mgr.CreateContract(spec); err != nil {
		t.Fatal(err)
	}

	// Get
	got, err := mgr.GetContract("crud-test")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "crud-test" {
		t.Fatalf("expected crud-test, got %s", got.Name)
	}

	// List
	all := mgr.ListContracts()
	if len(all) != 1 {
		t.Fatalf("expected 1 contract, got %d", len(all))
	}

	// Update
	spec.Rules = append(spec.Rules, Rule{Type: RuleFreshness, MaxStaleness: time.Minute})
	if err := mgr.UpdateContract(spec); err != nil {
		t.Fatal(err)
	}

	// Delete
	if err := mgr.DeleteContract("crud-test"); err != nil {
		t.Fatal(err)
	}
	_, err = mgr.GetContract("crud-test")
	if err != ErrContractNotFound {
		t.Fatalf("expected ErrContractNotFound, got %v", err)
	}
}

func TestManager_EvaluateFreshnessViolation(t *testing.T) {
	provider := &mockProvider{
		metrics: &FeatureMetrics{
			LastUpdated:  time.Now().Add(-10 * time.Minute),
			Completeness: 1.0,
			Mean:         50,
			Min:          0,
			Max:          100,
			DataType:     "float64",
			SampleCount:  1000,
		},
	}
	alertHandler := &mockAlertHandler{}
	mgr := NewManager(DefaultManagerConfig(), provider)
	mgr.RegisterAlert(alertHandler)

	spec := &Spec{
		Name:         "freshness-check",
		FeatureGroup: "user_features",
		FeatureName:  "clicks",
		Rules: []Rule{
			{Type: RuleFreshness, MaxStaleness: 5 * time.Minute, Severity: SeverityCritical},
		},
	}
	if err := mgr.CreateContract(spec); err != nil {
		t.Fatal(err)
	}

	mgr.EvaluateAll(context.Background())

	status, err := mgr.GetStatus("freshness-check")
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != StatusBreached {
		t.Fatalf("expected breached, got %s", status.Status)
	}
	if len(status.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(status.Violations))
	}
	if status.Violations[0].RuleType != RuleFreshness {
		t.Fatalf("expected freshness violation, got %s", status.Violations[0].RuleType)
	}
	if len(alertHandler.violations) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alertHandler.violations))
	}
}

func TestManager_EvaluateCompletenessViolation(t *testing.T) {
	provider := &mockProvider{
		metrics: &FeatureMetrics{
			LastUpdated:  time.Now(),
			Completeness: 0.80,
			DataType:     "float64",
		},
	}
	mgr := NewManager(DefaultManagerConfig(), provider)
	spec := &Spec{
		Name:         "completeness-check",
		FeatureGroup: "g",
		Rules: []Rule{
			{Type: RuleCompleteness, MinCompleteness: 0.95, Severity: SeverityWarning},
		},
	}
	if err := mgr.CreateContract(spec); err != nil {
		t.Fatal(err)
	}

	mgr.EvaluateAll(context.Background())

	status, _ := mgr.GetStatus("completeness-check")
	if status.Status != StatusWarning {
		t.Fatalf("expected warning, got %s", status.Status)
	}
}

func TestManager_EvaluateDistributionViolation(t *testing.T) {
	minVal := 0.0
	maxVal := 100.0
	provider := &mockProvider{
		metrics: &FeatureMetrics{
			LastUpdated: time.Now(),
			Min:         -5,
			Max:         200,
			Mean:        50,
			DataType:    "float64",
		},
	}
	mgr := NewManager(DefaultManagerConfig(), provider)
	spec := &Spec{
		Name:         "dist-check",
		FeatureGroup: "g",
		Rules: []Rule{
			{Type: RuleDistribution, MinValue: &minVal, MaxValue: &maxVal, Severity: SeverityError},
		},
	}
	if err := mgr.CreateContract(spec); err != nil {
		t.Fatal(err)
	}

	mgr.EvaluateAll(context.Background())

	status, _ := mgr.GetStatus("dist-check")
	if status.Status != StatusBreached {
		t.Fatalf("expected breached, got %s", status.Status)
	}
}

func TestManager_EvaluateSchemaViolation(t *testing.T) {
	provider := &mockProvider{
		metrics: &FeatureMetrics{
			LastUpdated: time.Now(),
			DataType:    "string",
		},
	}
	mgr := NewManager(DefaultManagerConfig(), provider)
	spec := &Spec{
		Name:         "schema-check",
		FeatureGroup: "g",
		Rules: []Rule{
			{Type: RuleSchema, ExpectedType: "float64", Severity: SeverityCritical},
		},
	}
	if err := mgr.CreateContract(spec); err != nil {
		t.Fatal(err)
	}

	mgr.EvaluateAll(context.Background())

	status, _ := mgr.GetStatus("schema-check")
	if status.Status != StatusBreached {
		t.Fatalf("expected breached, got %s", status.Status)
	}
}

func TestManager_EvaluatePassing(t *testing.T) {
	provider := &mockProvider{
		metrics: &FeatureMetrics{
			LastUpdated:  time.Now(),
			Completeness: 1.0,
			Min:          0,
			Max:          100,
			Mean:         50,
			DataType:     "float64",
		},
	}
	mgr := NewManager(DefaultManagerConfig(), provider)
	minVal := 0.0
	maxVal := 200.0
	spec := &Spec{
		Name:         "passing-check",
		FeatureGroup: "g",
		Rules: []Rule{
			{Type: RuleFreshness, MaxStaleness: 5 * time.Minute},
			{Type: RuleCompleteness, MinCompleteness: 0.95},
			{Type: RuleDistribution, MinValue: &minVal, MaxValue: &maxVal},
			{Type: RuleSchema, ExpectedType: "float64"},
		},
	}
	if err := mgr.CreateContract(spec); err != nil {
		t.Fatal(err)
	}

	mgr.EvaluateAll(context.Background())

	status, _ := mgr.GetStatus("passing-check")
	if status.Status != StatusPassing {
		t.Fatalf("expected passing, got %s", status.Status)
	}
	if status.RulesPassing != 4 {
		t.Fatalf("expected 4 rules passing, got %d", status.RulesPassing)
	}
}

func TestManager_GetViolations(t *testing.T) {
	provider := &mockProvider{
		metrics: &FeatureMetrics{
			LastUpdated: time.Now().Add(-10 * time.Minute),
		},
	}
	mgr := NewManager(DefaultManagerConfig(), provider)
	spec := &Spec{
		Name:         "v-test",
		FeatureGroup: "g",
		Rules:        []Rule{{Type: RuleFreshness, MaxStaleness: time.Minute}},
	}
	_ = mgr.CreateContract(spec)

	before := time.Now().Add(-time.Second)
	mgr.EvaluateAll(context.Background())

	violations := mgr.GetViolations(before)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
}
