package finops

import (
	"testing"
	"time"
)

func TestCostAttributor_RecordFeatureRead(t *testing.T) {
	ca := NewCostAttributor()

	ca.RecordFeatureRead("user_age", "user_features", "team-a")
	ca.RecordFeatureRead("user_age", "user_features", "team-a")
	ca.RecordFeatureRead("user_age", "user_features", "team-a")

	fc, ok := ca.GetFeatureCost("user_age")
	if !ok {
		t.Fatal("expected feature cost to exist")
	}
	if fc.ReadCount != 3 {
		t.Errorf("expected ReadCount=3, got %d", fc.ReadCount)
	}
	if fc.ServingCost <= 0 {
		t.Error("expected positive serving cost")
	}
	if fc.TotalCost <= 0 {
		t.Error("expected positive total cost")
	}
	if fc.CostPerRead <= 0 {
		t.Error("expected positive cost per read")
	}
	if fc.FeatureName != "user_age" {
		t.Errorf("expected FeatureName=user_age, got %s", fc.FeatureName)
	}
	if fc.GroupName != "user_features" {
		t.Errorf("expected GroupName=user_features, got %s", fc.GroupName)
	}
	if fc.Team != "team-a" {
		t.Errorf("expected Team=team-a, got %s", fc.Team)
	}
	if fc.LastAccessed.IsZero() {
		t.Error("expected LastAccessed to be set")
	}
}

func TestCostAttributor_RecordFeatureWrite(t *testing.T) {
	ca := NewCostAttributor()

	ca.RecordFeatureWrite("user_age", "user_features", "team-a")
	ca.RecordFeatureWrite("user_age", "user_features", "team-a")

	fc, ok := ca.GetFeatureCost("user_age")
	if !ok {
		t.Fatal("expected feature cost to exist")
	}
	if fc.WriteCount != 2 {
		t.Errorf("expected WriteCount=2, got %d", fc.WriteCount)
	}
	if fc.ComputeCost <= 0 {
		t.Error("expected positive compute cost")
	}
	if fc.TotalCost <= 0 {
		t.Error("expected positive total cost")
	}
}

func TestCostAttributor_RecordPrediction(t *testing.T) {
	ca := NewCostAttributor()

	ca.RecordPrediction("model-1", 5, 12.5)
	ca.RecordPrediction("model-1", 5, 7.5)

	pc, ok := ca.GetPredictionCost("model-1")
	if !ok {
		t.Fatal("expected prediction cost to exist")
	}
	if pc.TotalQueries != 2 {
		t.Errorf("expected TotalQueries=2, got %d", pc.TotalQueries)
	}
	if pc.FeatureCount != 5 {
		t.Errorf("expected FeatureCount=5, got %d", pc.FeatureCount)
	}
	if pc.TotalCost <= 0 {
		t.Error("expected positive total cost")
	}
	if pc.CostPerQuery <= 0 {
		t.Error("expected positive cost per query")
	}
	if pc.AvgLatencyMs != 10.0 {
		t.Errorf("expected AvgLatencyMs=10.0, got %f", pc.AvgLatencyMs)
	}
	if pc.ModelID != "model-1" {
		t.Errorf("expected ModelID=model-1, got %s", pc.ModelID)
	}
}

func TestCostAttributor_GenerateOptimizations_Unused(t *testing.T) {
	ca := NewCostAttributor()

	// Create a feature with an old last accessed time
	ca.mu.Lock()
	ca.featureCosts["stale_feature"] = &FeatureCost{
		FeatureName:  "stale_feature",
		GroupName:    "old_group",
		Team:         "team-a",
		StorageCost:  1.0,
		ComputeCost:  0.5,
		ServingCost:  0.5,
		TotalCost:    2.0,
		ReadCount:    10,
		WriteCount:   5,
		LastAccessed: time.Now().Add(-45 * 24 * time.Hour), // 45 days ago
		Period:       "monthly",
	}
	ca.mu.Unlock()

	suggestions := ca.GenerateOptimizations()

	found := false
	for _, s := range suggestions {
		if s.Type == "deprecation" && s.Feature == "stale_feature" {
			found = true
			if s.EstSavings <= 0 {
				t.Error("expected positive estimated savings")
			}
			if s.Priority != "high" {
				t.Errorf("expected priority=high, got %s", s.Priority)
			}
		}
	}
	if !found {
		t.Error("expected deprecation suggestion for stale_feature")
	}
}

func TestCostAttributor_GenerateOptimizations_Empty(t *testing.T) {
	ca := NewCostAttributor()

	suggestions := ca.GenerateOptimizations()
	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions for empty attributor, got %d", len(suggestions))
	}
}

func TestCostAttributor_GetCostSummary(t *testing.T) {
	ca := NewCostAttributor()

	// Record some activity
	ca.RecordFeatureRead("feat1", "group1", "team-a")
	ca.RecordFeatureWrite("feat1", "group1", "team-a")
	ca.RecordFeatureRead("feat2", "group2", "team-b")
	ca.RecordPrediction("model-1", 3, 5.0)

	summary := ca.GetCostSummary()

	if summary.TotalFeatures != 2 {
		t.Errorf("expected TotalFeatures=2, got %d", summary.TotalFeatures)
	}
	if summary.TotalModels != 1 {
		t.Errorf("expected TotalModels=1, got %d", summary.TotalModels)
	}
	if summary.GrandTotal <= 0 {
		t.Error("expected positive grand total")
	}
	if summary.TotalServingCost <= 0 {
		t.Error("expected positive serving cost")
	}
}

func TestCostAttributor_SetRates(t *testing.T) {
	ca := NewCostAttributor()

	ca.SetRates(0.05, 0.001, 0.002)

	// Record with new rates
	ca.RecordFeatureRead("feat1", "group1", "team-a")

	fc, ok := ca.GetFeatureCost("feat1")
	if !ok {
		t.Fatal("expected feature cost to exist")
	}
	// Serving cost should use new rate 0.002
	expectedServing := 0.002
	if fc.ServingCost != expectedServing {
		t.Errorf("expected ServingCost=%f, got %f", expectedServing, fc.ServingCost)
	}
}

func TestCostAttributor_GetAllFeatureCosts(t *testing.T) {
	ca := NewCostAttributor()

	ca.RecordFeatureRead("feat1", "g1", "t1")
	ca.RecordFeatureRead("feat2", "g2", "t2")

	all := ca.GetAllFeatureCosts()
	if len(all) != 2 {
		t.Errorf("expected 2 feature costs, got %d", len(all))
	}
}

func TestCostAttributor_GetAllPredictionCosts(t *testing.T) {
	ca := NewCostAttributor()

	ca.RecordPrediction("m1", 3, 5.0)
	ca.RecordPrediction("m2", 5, 10.0)

	all := ca.GetAllPredictionCosts()
	if len(all) != 2 {
		t.Errorf("expected 2 prediction costs, got %d", len(all))
	}
}
