package diffprivacy

import (
	"testing"
	"time"
)

func newTestEngine(features map[string]FeaturePrivacyConfig) *Engine {
	e := NewEngine(DefaultConfig())
	for name, cfg := range features {
		_ = e.RegisterFeature(name, cfg)
	}
	return e
}

func TestComplianceReporter_GDPRReport(t *testing.T) {
	e := newTestEngine(map[string]FeaturePrivacyConfig{
		"clicks": {Epsilon: 1.0, Delta: 1e-5, Mechanism: MechanismLaplace, Sensitivity: 1.0, MaxBudget: 10.0},
		"views":  {Epsilon: 0.5, Delta: 1e-5, Mechanism: MechanismGaussian, Sensitivity: 1.0, MaxBudget: 5.0},
	})

	reporter := NewComplianceReporter(e, nil)
	period := ReportPeriod{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
	}

	report := reporter.GenerateReport(FrameworkGDPR, period)

	if report.Framework != FrameworkGDPR {
		t.Errorf("Framework = %s, want GDPR", report.Framework)
	}
	if report.Summary.TotalFeatures != 2 {
		t.Errorf("TotalFeatures = %d, want 2", report.Summary.TotalFeatures)
	}
	if report.Summary.ProtectedFeatures != 2 {
		t.Errorf("ProtectedFeatures = %d, want 2", report.Summary.ProtectedFeatures)
	}
	if report.Summary.UnprotectedFeatures != 0 {
		t.Errorf("UnprotectedFeatures = %d, want 0", report.Summary.UnprotectedFeatures)
	}
	if len(report.FeatureDetails) != 2 {
		t.Errorf("len(FeatureDetails) = %d, want 2", len(report.FeatureDetails))
	}
	if len(report.Recommendations) == 0 {
		t.Error("expected at least one recommendation for GDPR")
	}

	// GDPR should include data subject access request recommendation
	found := false
	for _, r := range report.Recommendations {
		if r == "Ensure data subject access requests include privacy budget usage" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected GDPR data subject access request recommendation")
	}
}

func TestComplianceReporter_CCPAReport(t *testing.T) {
	e := newTestEngine(map[string]FeaturePrivacyConfig{
		"clicks": {Epsilon: 1.0, Delta: 1e-5, Mechanism: MechanismLaplace, Sensitivity: 1.0, MaxBudget: 10.0},
	})

	reporter := NewComplianceReporter(e, nil)
	period := ReportPeriod{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
	}

	report := reporter.GenerateReport(FrameworkCCPA, period)

	if report.Framework != FrameworkCCPA {
		t.Errorf("Framework = %s, want CCPA", report.Framework)
	}

	// CCPA should include consumer privacy notice recommendation
	found := false
	for _, r := range report.Recommendations {
		if r == "Document privacy mechanisms in consumer privacy notice" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected CCPA consumer privacy notice recommendation")
	}
}

func TestComplianceReporter_UnprotectedFeatures(t *testing.T) {
	e := NewEngine(DefaultConfig())
	// Register a feature with zero max budget (unprotected)
	cfg := FeaturePrivacyConfig{
		Epsilon:     1.0,
		Delta:       1e-5,
		Mechanism:   MechanismLaplace,
		Sensitivity: 1.0,
		MaxBudget:   0, // will be set to default by RegisterFeature
	}
	_ = e.RegisterFeature("unprotected", cfg)

	// Directly manipulate to simulate unprotected feature
	e.mu.Lock()
	e.features["unprotected"].config.MaxBudget = 0
	e.mu.Unlock()

	reporter := NewComplianceReporter(e, nil)
	period := ReportPeriod{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
	}

	report := reporter.GenerateReport(FrameworkGDPR, period)

	if report.Summary.UnprotectedFeatures != 1 {
		t.Errorf("UnprotectedFeatures = %d, want 1", report.Summary.UnprotectedFeatures)
	}

	// Find the violation feature
	foundViolation := false
	for _, fd := range report.FeatureDetails {
		if fd.Feature == "unprotected" && fd.Status == "violation" {
			foundViolation = true
			break
		}
	}
	if !foundViolation {
		t.Error("expected violation status for unprotected feature")
	}
}

func TestComplianceReporter_ComplianceScore(t *testing.T) {
	e := newTestEngine(map[string]FeaturePrivacyConfig{
		"f1": {Epsilon: 1.0, Delta: 1e-5, Mechanism: MechanismLaplace, Sensitivity: 1.0, MaxBudget: 10.0},
		"f2": {Epsilon: 1.0, Delta: 1e-5, Mechanism: MechanismLaplace, Sensitivity: 1.0, MaxBudget: 10.0},
	})

	reporter := NewComplianceReporter(e, nil)
	period := ReportPeriod{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
	}

	report := reporter.GenerateReport(FrameworkGDPR, period)

	// All features protected, no consumption -> high score
	if report.Summary.ComplianceScore < 80 {
		t.Errorf("ComplianceScore = %f, want >= 80 for all-protected features", report.Summary.ComplianceScore)
	}
	if report.Summary.Status != "compliant" {
		t.Errorf("Status = %s, want compliant", report.Summary.Status)
	}
}

func TestComplianceReporter_WithBudgetManager(t *testing.T) {
	e := newTestEngine(map[string]FeaturePrivacyConfig{
		"clicks": {Epsilon: 1.0, Delta: 1e-5, Mechanism: MechanismLaplace, Sensitivity: 1.0, MaxBudget: 10.0},
	})

	bm := NewBudgetManager(DefaultBudgetManagerConfig())
	key := BudgetKey{Feature: "clicks", EntityType: "user"}
	_ = bm.RegisterBudget(key, 10.0, 1e-5)
	_ = bm.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count")

	reporter := NewComplianceReporter(e, bm)
	period := ReportPeriod{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
	}

	report := reporter.GenerateReport(FrameworkGDPR, period)

	// Should have enriched feature details with entity type
	foundEntityType := false
	for _, fd := range report.FeatureDetails {
		if fd.Feature == "clicks" && fd.EntityType == "user" {
			foundEntityType = true
			break
		}
	}
	if !foundEntityType {
		t.Error("expected entity_type from budget manager in feature details")
	}
}

func TestComplianceReporter_WithBudgetManagerExtraFeature(t *testing.T) {
	e := newTestEngine(map[string]FeaturePrivacyConfig{
		"clicks": {Epsilon: 1.0, Delta: 1e-5, Mechanism: MechanismLaplace, Sensitivity: 1.0, MaxBudget: 10.0},
	})

	bm := NewBudgetManager(DefaultBudgetManagerConfig())
	// Register a budget for a feature not in the engine
	extraKey := BudgetKey{Feature: "extra_feature", EntityType: "device"}
	_ = bm.RegisterBudget(extraKey, 5.0, 1e-5)

	reporter := NewComplianceReporter(e, bm)
	period := ReportPeriod{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
	}

	report := reporter.GenerateReport(FrameworkGDPR, period)

	// Should include the extra feature from budget manager
	foundExtra := false
	for _, fd := range report.FeatureDetails {
		if fd.Feature == "extra_feature" && fd.EntityType == "device" {
			foundExtra = true
			break
		}
	}
	if !foundExtra {
		t.Error("expected extra_feature from budget manager in feature details")
	}
}

func TestComplianceReporter_WarningStatus(t *testing.T) {
	e := NewEngine(DefaultConfig())
	cfg := FeaturePrivacyConfig{
		Epsilon:     1.0,
		Delta:       1e-5,
		Mechanism:   MechanismLaplace,
		Sensitivity: 1.0,
		MaxBudget:   10.0,
	}
	_ = e.RegisterFeature("high_usage", cfg)

	// Consume 90%+ of budget
	for i := 0; i < 9; i++ {
		_, _ = e.AddNoise("high_usage", 42.0)
	}

	reporter := NewComplianceReporter(e, nil)
	period := ReportPeriod{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
	}

	report := reporter.GenerateReport(FrameworkGDPR, period)

	foundWarning := false
	for _, fd := range report.FeatureDetails {
		if fd.Feature == "high_usage" && fd.Status == "warning" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("expected warning status for high-usage feature")
	}
}

func TestComplianceReporter_HIPAARecommendations(t *testing.T) {
	e := newTestEngine(map[string]FeaturePrivacyConfig{
		"f1": {Epsilon: 1.0, Delta: 1e-5, Mechanism: MechanismLaplace, Sensitivity: 1.0, MaxBudget: 10.0},
	})

	reporter := NewComplianceReporter(e, nil)
	period := ReportPeriod{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
	}

	report := reporter.GenerateReport(FrameworkHIPAA, period)

	found := false
	for _, r := range report.Recommendations {
		if r == "Verify de-identification meets Safe Harbor or Expert Determination requirements" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected HIPAA de-identification recommendation")
	}
}

func TestComplianceReporter_RejectedQueriesRecommendation(t *testing.T) {
	e := newTestEngine(map[string]FeaturePrivacyConfig{
		"f1": {Epsilon: 1.0, Delta: 1e-5, Mechanism: MechanismLaplace, Sensitivity: 1.0, MaxBudget: 10.0},
	})

	cfg := DefaultBudgetManagerConfig()
	cfg.AutoReject = true
	bm := NewBudgetManager(cfg)
	key := BudgetKey{Feature: "f1", EntityType: "user"}
	_ = bm.RegisterBudget(key, 1.0, 1e-5)
	_ = bm.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count")
	_ = bm.ConsumeAndCheck(key, 1.0, 1e-6, MechanismLaplace, "count") // rejected

	reporter := NewComplianceReporter(e, bm)
	period := ReportPeriod{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
	}

	report := reporter.GenerateReport(FrameworkGDPR, period)

	if report.Summary.RejectedQueries != 1 {
		t.Errorf("RejectedQueries = %d, want 1", report.Summary.RejectedQueries)
	}

	foundRejectedRec := false
	for _, r := range report.Recommendations {
		if len(r) > 0 && r[0:6] == "Review" {
			foundRejectedRec = true
			break
		}
	}
	if !foundRejectedRec {
		t.Error("expected recommendation about rejected queries")
	}
}
