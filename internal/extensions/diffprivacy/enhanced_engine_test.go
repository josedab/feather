package diffprivacy

import (
	"testing"
	"time"
)

func TestEnhancedEngine_RegisterAndTrack(t *testing.T) {
	ee := NewEnhancedEngine(DefaultConfig(), DefaultBudgetManagerConfig())

	err := ee.RegisterFeature("clicks", "user", FeaturePrivacyConfig{
		Epsilon:     1.0,
		Delta:       1e-5,
		Mechanism:   MechanismLaplace,
		Sensitivity: 1.0,
		MaxBudget:   5.0,
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// Apply noise with tracking
	noisy, err := ee.AddNoiseTracked("clicks", "user", 100.0)
	if err != nil {
		t.Fatalf("add noise tracked failed: %v", err)
	}
	if noisy == 0 {
		t.Error("noisy value should not be exactly 0")
	}

	// Check budget was consumed
	budget, err := ee.BudgetManager().GetBudget(BudgetKey{Feature: "clicks", EntityType: "user"})
	if err != nil {
		t.Fatalf("get budget failed: %v", err)
	}
	if budget.ConsumedEpsilon == 0 {
		t.Error("budget should have been consumed")
	}
}

func TestEnhancedEngine_BudgetExhaustion(t *testing.T) {
	ee := NewEnhancedEngine(DefaultConfig(), BudgetManagerConfig{
		DefaultMaxEpsilon: 2.0,
		DefaultAlertAt:    0.8,
		AutoReject:        true,
	})

	err := ee.RegisterFeature("clicks", "user", FeaturePrivacyConfig{
		Epsilon:     1.0,
		Delta:       1e-5,
		Mechanism:   MechanismLaplace,
		Sensitivity: 1.0,
		MaxBudget:   2.0,
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// First two queries should succeed (budget = 2.0, each costs 1.0)
	for i := 0; i < 2; i++ {
		_, err := ee.AddNoiseTracked("clicks", "user", 100.0)
		if err != nil {
			t.Fatalf("query %d should succeed: %v", i+1, err)
		}
	}

	// Third query should be rejected by budget manager
	_, err = ee.AddNoiseTracked("clicks", "user", 100.0)
	if err == nil {
		t.Fatal("third query should have been rejected")
	}
}

func TestEnhancedEngine_NoisyCountTracked(t *testing.T) {
	ee := NewEnhancedEngine(DefaultConfig(), DefaultBudgetManagerConfig())

	err := ee.RegisterFeature("views", "page", FeaturePrivacyConfig{
		Epsilon:     0.5,
		Delta:       1e-5,
		Mechanism:   MechanismLaplace,
		Sensitivity: 1.0,
		MaxBudget:   10.0,
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	result, err := ee.NoisyCountTracked("views", "page", 1000)
	if err != nil {
		t.Fatalf("noisy count tracked failed: %v", err)
	}
	if result.EpsilonUsed != 0.5 {
		t.Errorf("expected epsilon 0.5, got %f", result.EpsilonUsed)
	}
}

func TestEnhancedEngine_GenerateReport(t *testing.T) {
	ee := NewEnhancedEngine(DefaultConfig(), DefaultBudgetManagerConfig())

	_ = ee.RegisterFeature("f1", "user", FeaturePrivacyConfig{
		Epsilon: 1.0, Delta: 1e-5, Mechanism: MechanismLaplace,
		Sensitivity: 1.0, MaxBudget: 10.0,
	})

	report := ee.GenerateReport(FrameworkGDPR, ReportPeriod{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
	})
	if report.Framework != FrameworkGDPR {
		t.Errorf("expected GDPR framework, got %s", report.Framework)
	}
	if report.Summary.TotalFeatures != 1 {
		t.Errorf("expected 1 feature, got %d", report.Summary.TotalFeatures)
	}
}
