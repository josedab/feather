package federation

import (
	"math"
	"testing"
)

func TestPrivacyEngine_RegisterOrg(t *testing.T) {
	pe := NewPrivacyEngine(DefaultPrivacyConfig())

	err := pe.RegisterOrg("org-1", 5.0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Duplicate registration should fail.
	err = pe.RegisterOrg("org-1", 5.0)
	if err != ErrOrgAlreadyRegistered {
		t.Fatalf("expected ErrOrgAlreadyRegistered, got %v", err)
	}
}

func TestPrivacyEngine_BudgetTracking(t *testing.T) {
	pe := NewPrivacyEngine(DefaultPrivacyConfig())
	_ = pe.RegisterOrg("org-1", 5.0)

	pe.ConsumeBudget("org-1", 2.0)

	budget, err := pe.GetBudget("org-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if budget.UsedEpsilon != 2.0 {
		t.Fatalf("expected used epsilon 2.0, got %f", budget.UsedEpsilon)
	}
	remaining := budget.TotalEpsilon - budget.UsedEpsilon
	if remaining != 3.0 {
		t.Fatalf("expected remaining 3.0, got %f", remaining)
	}
	if budget.QueryCount != 1 {
		t.Fatalf("expected query count 1, got %d", budget.QueryCount)
	}
}

func TestPrivacyEngine_BudgetExceeded(t *testing.T) {
	pe := NewPrivacyEngine(DefaultPrivacyConfig())
	_ = pe.RegisterOrg("org-1", 3.0)

	pe.ConsumeBudget("org-1", 2.5)

	err := pe.CheckBudget("org-1", 1.0)
	if err != ErrBudgetExceeded {
		t.Fatalf("expected ErrBudgetExceeded, got %v", err)
	}

	// Should still allow within budget.
	err = pe.CheckBudget("org-1", 0.4)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestPrivacyEngine_LaplaceNoise(t *testing.T) {
	pe := NewPrivacyEngine(DefaultPrivacyConfig())

	original := 100.0
	noisyCount := 0
	for i := 0; i < 100; i++ {
		noisy := pe.AddLaplaceNoise(original, 1.0, 1.0)
		if noisy != original {
			noisyCount++
		}
	}

	// Noise should be added in virtually all cases.
	if noisyCount < 90 {
		t.Fatalf("expected noise to be added in most cases, only %d/100 had noise", noisyCount)
	}
}

func TestPrivacyEngine_GaussianNoise(t *testing.T) {
	pe := NewPrivacyEngine(DefaultPrivacyConfig())

	original := 50.0
	noisyCount := 0
	for i := 0; i < 100; i++ {
		noisy := pe.AddGaussianNoise(original, 1.0, 1.0, 1e-5)
		if noisy != original {
			noisyCount++
		}
	}

	if noisyCount < 90 {
		t.Fatalf("expected noise to be added in most cases, only %d/100 had noise", noisyCount)
	}
}

func TestPrivacyEngine_KAnonymity(t *testing.T) {
	pe := NewPrivacyEngine(DefaultPrivacyConfig())

	// Default MinKAnonymity is 5.
	err := pe.CheckKAnonymity(3)
	if err != ErrKAnonymityViolation {
		t.Fatalf("expected ErrKAnonymityViolation, got %v", err)
	}

	err = pe.CheckKAnonymity(5)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	err = pe.CheckKAnonymity(10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestPrivacyEngine_ExecuteQuery(t *testing.T) {
	pe := NewPrivacyEngine(DefaultPrivacyConfig())
	_ = pe.RegisterOrg("org-1", 10.0)

	req := &FederatedQueryRequest{
		QueryID:     "q-1",
		OrgID:       "org-1",
		Features:    []string{"feature-a"},
		Aggregation: "sum",
		Epsilon:     1.0,
	}

	rawValues := map[string][]float64{
		"feature-a": {10.0, 20.0, 30.0, 40.0, 50.0},
	}

	result, err := pe.ExecutePrivateQuery(req, rawValues)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.QueryID != "q-1" {
		t.Fatalf("expected query ID q-1, got %s", result.QueryID)
	}
	if !result.NoiseAdded {
		t.Fatal("expected noise to be added")
	}
	if result.KAnonymity != 5 {
		t.Fatalf("expected k-anonymity 5, got %d", result.KAnonymity)
	}

	// The true sum is 150; result should be in a reasonable range.
	val, ok := result.Results["feature-a"]
	if !ok {
		t.Fatal("expected feature-a in results")
	}
	if math.Abs(val-150.0) > 500 {
		t.Fatalf("noisy result %f is too far from true sum 150", val)
	}

	// Budget should be consumed.
	budget, _ := pe.GetBudget("org-1")
	if budget.UsedEpsilon != 1.0 {
		t.Fatalf("expected used epsilon 1.0, got %f", budget.UsedEpsilon)
	}
}

func TestPrivacyEngine_ExecuteQuery_BudgetExceeded(t *testing.T) {
	pe := NewPrivacyEngine(DefaultPrivacyConfig())
	_ = pe.RegisterOrg("org-1", 0.5)

	req := &FederatedQueryRequest{
		QueryID:     "q-1",
		OrgID:       "org-1",
		Features:    []string{"feature-a"},
		Aggregation: "count",
		Epsilon:     1.0,
	}

	rawValues := map[string][]float64{
		"feature-a": {1, 2, 3, 4, 5},
	}

	_, err := pe.ExecutePrivateQuery(req, rawValues)
	if err != ErrBudgetExceeded {
		t.Fatalf("expected ErrBudgetExceeded, got %v", err)
	}
}

func TestPrivacyEngine_ExecuteQuery_KAnonymityViolation(t *testing.T) {
	pe := NewPrivacyEngine(DefaultPrivacyConfig())
	_ = pe.RegisterOrg("org-1", 10.0)

	req := &FederatedQueryRequest{
		QueryID:     "q-1",
		OrgID:       "org-1",
		Features:    []string{"feature-a"},
		Aggregation: "sum",
		Epsilon:     1.0,
	}

	// Only 2 values, less than MinKAnonymity (5).
	rawValues := map[string][]float64{
		"feature-a": {10.0, 20.0},
	}

	_, err := pe.ExecutePrivateQuery(req, rawValues)
	if err != ErrKAnonymityViolation {
		t.Fatalf("expected ErrKAnonymityViolation, got %v", err)
	}
}

func TestPrivacyEngine_AuditLog(t *testing.T) {
	pe := NewPrivacyEngine(DefaultPrivacyConfig())
	_ = pe.RegisterOrg("org-1", 10.0)

	req := &FederatedQueryRequest{
		QueryID:     "q-1",
		OrgID:       "org-1",
		Features:    []string{"feature-a"},
		Aggregation: "count",
		Epsilon:     1.0,
	}

	rawValues := map[string][]float64{
		"feature-a": {1, 2, 3, 4, 5},
	}

	_, _ = pe.ExecutePrivateQuery(req, rawValues)

	log := pe.GetAuditLog()
	if len(log) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(log))
	}
	if log[0].OrgID != "org-1" {
		t.Fatalf("expected org-1, got %s", log[0].OrgID)
	}
	if log[0].Action != "query_executed" {
		t.Fatalf("expected action query_executed, got %s", log[0].Action)
	}
	if !log[0].Approved {
		t.Fatal("expected approved to be true")
	}
}

func TestPrivacyEngine_ResetBudget(t *testing.T) {
	pe := NewPrivacyEngine(DefaultPrivacyConfig())
	_ = pe.RegisterOrg("org-1", 5.0)

	pe.ConsumeBudget("org-1", 3.0)
	pe.ResetBudget("org-1")

	budget, _ := pe.GetBudget("org-1")
	if budget.UsedEpsilon != 0 {
		t.Fatalf("expected used epsilon 0 after reset, got %f", budget.UsedEpsilon)
	}
	if budget.QueryCount != 0 {
		t.Fatalf("expected query count 0 after reset, got %d", budget.QueryCount)
	}
}

func TestPrivacyEngine_InvalidAggregation(t *testing.T) {
	pe := NewPrivacyEngine(DefaultPrivacyConfig())
	_ = pe.RegisterOrg("org-1", 10.0)

	req := &FederatedQueryRequest{
		QueryID:     "q-1",
		OrgID:       "org-1",
		Features:    []string{"feature-a"},
		Aggregation: "median",
		Epsilon:     1.0,
	}

	rawValues := map[string][]float64{
		"feature-a": {1, 2, 3, 4, 5},
	}

	_, err := pe.ExecutePrivateQuery(req, rawValues)
	if err != ErrInvalidAggregation {
		t.Fatalf("expected ErrInvalidAggregation, got %v", err)
	}
}

func TestPrivacyEngine_GetBudget_NotFound(t *testing.T) {
	pe := NewPrivacyEngine(DefaultPrivacyConfig())

	_, err := pe.GetBudget("nonexistent")
	if err != ErrOrgNotFound {
		t.Fatalf("expected ErrOrgNotFound, got %v", err)
	}
}
