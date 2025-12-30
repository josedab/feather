package impact

import (
	"testing"
	"time"
)

func TestImpactTracker_RecordAccess(t *testing.T) {
	tracker := NewImpactTracker()

	// Record some accesses
	tracker.RecordAccess("user_age", 1.5, false, false)
	tracker.RecordAccess("user_age", 2.0, false, false)
	tracker.RecordAccess("user_age", 1.0, true, false) // error
	tracker.RecordAccess("user_age", 0.5, false, true) // null

	usage := tracker.GetFeatureUsage("user_age")
	if usage == nil {
		t.Fatal("expected feature usage to exist")
	}

	if usage.AccessCount != 4 {
		t.Errorf("expected access count 4, got %d", usage.AccessCount)
	}

	if usage.ErrorCount != 1 {
		t.Errorf("expected error count 1, got %d", usage.ErrorCount)
	}

	if usage.NullCount != 1 {
		t.Errorf("expected null count 1, got %d", usage.NullCount)
	}

	if usage.AvgLatencyMs <= 0 {
		t.Error("expected positive average latency")
	}
}

func TestImpactTracker_RegisterModel(t *testing.T) {
	tracker := NewImpactTracker()

	model := &ModelUsage{
		ModelID:      "fraud_model",
		ModelVersion: "v1.0",
		Features:     []string{"user_age", "transaction_amount", "location"},
		Environment:  "prod",
		Endpoint:     "/predict/fraud",
	}

	tracker.RegisterModel(model)

	// Verify model was registered
	registeredModel := tracker.GetModelUsage("fraud_model")
	if registeredModel == nil {
		t.Fatal("expected model to be registered")
	}

	if len(registeredModel.Features) != 3 {
		t.Errorf("expected 3 features, got %d", len(registeredModel.Features))
	}

	// Verify features are linked to model
	for _, feature := range model.Features {
		usage := tracker.GetFeatureUsage(feature)
		if usage == nil {
			t.Errorf("expected feature %s to exist", feature)
			continue
		}
		if len(usage.Models) != 1 || usage.Models[0] != "fraud_model" {
			t.Errorf("expected feature %s to be linked to fraud_model", feature)
		}
	}
}

func TestImpactTracker_SetDependencies(t *testing.T) {
	tracker := NewImpactTracker()

	// Set up dependencies: derived_feature depends on base_feature_1 and base_feature_2
	tracker.SetDependencies("derived_feature", []string{"base_feature_1", "base_feature_2"})

	// Verify dependencies are set
	usage := tracker.GetFeatureUsage("derived_feature")
	if usage == nil {
		t.Fatal("expected feature to exist")
	}

	if len(usage.Dependencies) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(usage.Dependencies))
	}

	// Verify dependents are updated
	base1 := tracker.GetFeatureUsage("base_feature_1")
	if base1 == nil {
		t.Fatal("expected base_feature_1 to exist")
	}

	if len(base1.Dependents) != 1 || base1.Dependents[0] != "derived_feature" {
		t.Error("expected base_feature_1 to have derived_feature as dependent")
	}
}

func TestImpactTracker_CalculateImpactScore(t *testing.T) {
	tracker := NewImpactTracker()

	// Create some usage data
	for i := 0; i < 100; i++ {
		tracker.RecordAccess("popular_feature", 0.5, false, false)
	}

	// Register a model using the feature
	model := &ModelUsage{
		ModelID:     "test_model",
		Features:    []string{"popular_feature"},
		Environment: "prod",
	}
	tracker.RegisterModel(model)

	score := tracker.CalculateImpactScore("popular_feature")
	if score == nil {
		t.Fatal("expected impact score")
	}

	if score.OverallScore <= 0 {
		t.Error("expected positive overall score")
	}

	if score.UsageScore <= 0 {
		t.Error("expected positive usage score")
	}

	if !score.CriticalPath {
		t.Error("expected feature to be on critical path (used by prod model)")
	}
}

func TestImpactTracker_GetTopFeaturesByImpact(t *testing.T) {
	tracker := NewImpactTracker()

	// Create features with different usage levels
	for i := 0; i < 100; i++ {
		tracker.RecordAccess("high_usage", 0.5, false, false)
	}
	for i := 0; i < 10; i++ {
		tracker.RecordAccess("medium_usage", 0.5, false, false)
	}
	tracker.RecordAccess("low_usage", 0.5, false, false)

	topFeatures := tracker.GetTopFeaturesByImpact(2)

	if len(topFeatures) != 2 {
		t.Errorf("expected 2 features, got %d", len(topFeatures))
	}

	// Verify ordering (highest impact first)
	if topFeatures[0].Feature != "high_usage" {
		t.Errorf("expected high_usage to be first, got %s", topFeatures[0].Feature)
	}
}

func TestImpactTracker_GetUnusedFeatures(t *testing.T) {
	tracker := NewImpactTracker()

	// Create a feature with old access time
	tracker.RecordAccess("old_feature", 0.5, false, false)

	// Manually set last access to 40 days ago
	tracker.mu.Lock()
	if usage, ok := tracker.featureUsage["old_feature"]; ok {
		usage.LastAccess = time.Now().Add(-40 * 24 * time.Hour)
	}
	tracker.mu.Unlock()

	// Create a recently used feature
	tracker.RecordAccess("recent_feature", 0.5, false, false)

	// Get features unused in last 30 days
	since := time.Now().Add(-30 * 24 * time.Hour)
	unused := tracker.GetUnusedFeatures(since)

	if len(unused) != 1 {
		t.Errorf("expected 1 unused feature, got %d", len(unused))
	}

	if len(unused) > 0 && unused[0].Feature != "old_feature" {
		t.Errorf("expected old_feature to be unused, got %s", unused[0].Feature)
	}
}

func TestImpactTracker_Deprecation(t *testing.T) {
	tracker := NewImpactTracker()

	// Create a feature first
	tracker.RecordAccess("deprecated_feature", 0.5, false, false)

	// Request deprecation
	req := &DeprecationRequest{
		Feature:     "deprecated_feature",
		Reason:      "Replaced by better_feature",
		RequestedBy: "engineer@example.com",
		Replacement: "better_feature",
	}

	err := tracker.RequestDeprecation(req)
	if err != nil {
		t.Fatalf("failed to request deprecation: %v", err)
	}

	// Verify deprecation request
	depReq := tracker.GetDeprecationRequest("deprecated_feature")
	if depReq == nil {
		t.Fatal("expected deprecation request to exist")
	}

	if depReq.Status != "pending" {
		t.Errorf("expected status pending, got %s", depReq.Status)
	}

	// Approve deprecation
	err = tracker.ApproveDeprecation("deprecated_feature")
	if err != nil {
		t.Fatalf("failed to approve deprecation: %v", err)
	}

	// Verify feature is marked deprecated
	usage := tracker.GetFeatureUsage("deprecated_feature")
	if !usage.Deprecated {
		t.Error("expected feature to be marked as deprecated")
	}

	depReq = tracker.GetDeprecationRequest("deprecated_feature")
	if depReq.Status != "approved" {
		t.Errorf("expected status approved, got %s", depReq.Status)
	}
}

func TestImpactTracker_FeatureLineage(t *testing.T) {
	tracker := NewImpactTracker()

	// Create a dependency chain: level3 -> level2 -> level1
	tracker.SetDependencies("level2", []string{"level1"})
	tracker.SetDependencies("level3", []string{"level2"})

	lineage := tracker.GetFeatureLineage("level2")
	if lineage == nil {
		t.Fatal("expected lineage to exist")
	}

	// Verify upstream (dependencies)
	if len(lineage.Upstream) != 1 || lineage.Upstream[0] != "level1" {
		t.Errorf("expected level1 in upstream, got %v", lineage.Upstream)
	}

	// Verify downstream (dependents)
	if len(lineage.Downstream) != 1 || lineage.Downstream[0] != "level3" {
		t.Errorf("expected level3 in downstream, got %v", lineage.Downstream)
	}
}

func TestImpactTracker_GenerateReport(t *testing.T) {
	tracker := NewImpactTracker()

	// Create some test data
	for i := 0; i < 50; i++ {
		tracker.RecordAccess("feature1", 0.5, false, false)
	}
	tracker.RecordAccess("feature2", 0.5, false, false)

	model := &ModelUsage{
		ModelID:     "model1",
		Features:    []string{"feature1"},
		Environment: "prod",
	}
	tracker.RegisterModel(model)

	report := tracker.GenerateReport()

	if report.TotalFeatures != 2 {
		t.Errorf("expected 2 features, got %d", report.TotalFeatures)
	}

	if report.TotalModels != 1 {
		t.Errorf("expected 1 model, got %d", report.TotalModels)
	}

	if len(report.TopFeatures) == 0 {
		t.Error("expected top features in report")
	}

	if report.CriticalFeatures != 1 {
		t.Errorf("expected 1 critical feature, got %d", report.CriticalFeatures)
	}
}

func TestImpactTracker_RecordInference(t *testing.T) {
	tracker := NewImpactTracker()

	// Register a model
	model := &ModelUsage{
		ModelID:  "test_model",
		Features: []string{"feature1"},
	}
	tracker.RegisterModel(model)

	// Record inferences
	tracker.RecordInference("test_model", 10.0, false)
	tracker.RecordInference("test_model", 12.0, false)
	tracker.RecordInference("test_model", 100.0, true) // error

	registeredModel := tracker.GetModelUsage("test_model")
	if registeredModel == nil {
		t.Fatal("expected model to exist")
	}

	if registeredModel.InferenceCount != 3 {
		t.Errorf("expected 3 inferences, got %d", registeredModel.InferenceCount)
	}

	if registeredModel.AvgLatencyMs <= 0 {
		t.Error("expected positive average latency")
	}

	if registeredModel.ErrorRate <= 0 {
		t.Error("expected positive error rate")
	}
}
