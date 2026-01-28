package lineage

import "testing"

func TestImpactAnalyzer_Analyze(t *testing.T) {
	tracker := NewTracker()

	// Register features with dependencies: A -> B -> C, with a consumer on C
	tracker.RegisterFeature(&FeatureLineage{FeatureID: "feat_a", Name: "Feature A"})
	tracker.RegisterFeature(&FeatureLineage{FeatureID: "feat_b", Name: "Feature B", Dependencies: []string{"feat_a"}})
	tracker.RegisterFeature(&FeatureLineage{FeatureID: "feat_c", Name: "Feature C", Dependencies: []string{"feat_b"}})

	tracker.RegisterConsumer(&Consumer{ID: "model_x", Name: "Model X", Type: ConsumerTypeModel})
	tracker.LinkFeatureToConsumer("feat_c", "model_x", "inference")

	analyzer := NewImpactAnalyzer(tracker)
	report, err := analyzer.Analyze("feat_a")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if report.FeatureID != "feat_a" {
		t.Errorf("expected feature_id feat_a, got %s", report.FeatureID)
	}

	// feat_a -> feat_b is a direct dependent
	if len(report.DirectDependents) != 1 || report.DirectDependents[0] != "feat_b" {
		t.Errorf("expected 1 direct dependent (feat_b), got %v", report.DirectDependents)
	}

	// feat_b, feat_c, and model_x (consumer node) are transitive dependents
	if len(report.AllDependents) != 3 {
		t.Errorf("expected 3 total dependents, got %d: %v", len(report.AllDependents), report.AllDependents)
	}

	// model_x is affected via feat_c
	if len(report.AffectedModels) != 1 || report.AffectedModels[0] != "model_x" {
		t.Errorf("expected 1 affected model (model_x), got %v", report.AffectedModels)
	}

	if report.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestImpactAnalyzer_EmptyDependents(t *testing.T) {
	tracker := NewTracker()
	tracker.RegisterFeature(&FeatureLineage{FeatureID: "isolated", Name: "Isolated Feature"})

	analyzer := NewImpactAnalyzer(tracker)
	report, err := analyzer.Analyze("isolated")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(report.DirectDependents) != 0 {
		t.Errorf("expected 0 direct dependents, got %d", len(report.DirectDependents))
	}
	if len(report.AllDependents) != 0 {
		t.Errorf("expected 0 total dependents, got %d", len(report.AllDependents))
	}
	if report.RiskLevel != RiskLow {
		t.Errorf("expected risk level low, got %s", report.RiskLevel)
	}
}

func TestImpactAnalyzer_NotFound(t *testing.T) {
	tracker := NewTracker()
	analyzer := NewImpactAnalyzer(tracker)

	_, err := analyzer.Analyze("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent feature")
	}
}

func TestImpactAnalyzer_RiskLevels(t *testing.T) {
	tests := []struct {
		name         string
		depCount     int
		piiLevel     PIILevel
		expectedRisk RiskLevel
	}{
		{"low_risk_few_deps", 2, PIINone, RiskLow},
		{"medium_risk_several_deps", 5, PIINone, RiskMedium},
		{"high_risk_many_deps", 12, PIINone, RiskHigh},
		{"critical_risk_very_many_deps", 22, PIINone, RiskCritical},
		{"high_risk_medium_pii", 1, PIIMedium, RiskHigh},
		{"critical_risk_high_pii", 1, PIIHigh, RiskCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewTracker()

			// Register root feature
			tracker.RegisterFeature(&FeatureLineage{FeatureID: "root", Name: "Root"})

			// Build chain of dependents
			prev := "root"
			for i := 0; i < tt.depCount; i++ {
				id := "dep_" + string(rune('a'+i))
				if i >= 26 {
					id = "dep_extra_" + string(rune('a'+i-26))
				}
				fl := &FeatureLineage{
					FeatureID:    id,
					Name:         id,
					Dependencies: []string{prev},
					PIILevel:     tt.piiLevel,
				}
				tracker.RegisterFeature(fl)
				prev = id
			}

			analyzer := NewImpactAnalyzer(tracker)
			report, err := analyzer.Analyze("root")
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}

			if report.RiskLevel != tt.expectedRisk {
				t.Errorf("expected risk %s, got %s (dependents: %d)",
					tt.expectedRisk, report.RiskLevel, len(report.AllDependents))
			}
		})
	}
}

func TestImpactAnalyzer_CompareVersions(t *testing.T) {
	tracker := NewTracker()
	tracker.RegisterFeature(&FeatureLineage{FeatureID: "feat_v", Name: "Versioned Feature"})
	tracker.RegisterFeature(&FeatureLineage{FeatureID: "feat_dep", Name: "Dependent", Dependencies: []string{"feat_v"}})

	analyzer := NewImpactAnalyzer(tracker)
	report, err := analyzer.CompareVersions("feat_v", 1, 2)
	if err != nil {
		t.Fatalf("CompareVersions failed: %v", err)
	}

	// Should have breaking changes flagged for version change
	foundVersionChange := false
	for _, bc := range report.BreakingChanges {
		if bc.FeatureID == "feat_dep" && bc.Severity == "medium" {
			foundVersionChange = true
			break
		}
	}
	if !foundVersionChange {
		t.Error("expected breaking change for version update on dependent")
	}
}

func TestImpactAnalyzer_CompareVersions_NotFound(t *testing.T) {
	tracker := NewTracker()
	analyzer := NewImpactAnalyzer(tracker)

	_, err := analyzer.CompareVersions("nonexistent", 1, 2)
	if err == nil {
		t.Fatal("expected error for nonexistent feature")
	}
}

func TestImpactAnalyzer_PIIBreakingChanges(t *testing.T) {
	tracker := NewTracker()

	// Feature with high PII that has downstream dependents
	tracker.RegisterFeature(&FeatureLineage{
		FeatureID: "pii_feat",
		Name:      "PII Feature",
		PIILevel:  PIIHigh,
	})
	tracker.RegisterFeature(&FeatureLineage{
		FeatureID:    "downstream",
		Name:         "Downstream",
		Dependencies: []string{"pii_feat"},
	})

	analyzer := NewImpactAnalyzer(tracker)
	report, err := analyzer.Analyze("pii_feat")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(report.BreakingChanges) == 0 {
		t.Error("expected breaking changes due to high PII upstream")
	}
	if report.BreakingChanges[0].Severity != "high" {
		t.Errorf("expected severity high, got %s", report.BreakingChanges[0].Severity)
	}
}
