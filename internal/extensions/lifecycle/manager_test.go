package lifecycle

import (
	"testing"
	"time"
)

func TestManager_TrackAndAccess(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	m.TrackFeature("user_age")
	m.RecordAccess("user_age", "model_v1")
	m.RecordAccess("user_age", "model_v1")
	m.RecordAccess("user_age", "model_v2")

	fu, err := m.GetFeature("user_age")
	if err != nil {
		t.Fatalf("GetFeature: %v", err)
	}
	if fu.AccessCount != 3 {
		t.Fatalf("expected 3 accesses, got %d", fu.AccessCount)
	}
	if len(fu.Consumers) != 2 {
		t.Fatalf("expected 2 consumers, got %d", len(fu.Consumers))
	}
}

func TestManager_AutoRegister(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	m.RecordAccess("new_feature", "consumer1")

	fu, err := m.GetFeature("new_feature")
	if err != nil {
		t.Fatalf("GetFeature: %v", err)
	}
	if fu.AccessCount != 1 {
		t.Fatalf("expected 1 access, got %d", fu.AccessCount)
	}
}

func TestManager_UpdateMetrics(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	m.TrackFeature("f1")
	err := m.UpdateMetrics("f1", 0.15, 0.9, 1024*1024*100)
	if err != nil {
		t.Fatalf("UpdateMetrics: %v", err)
	}

	fu, _ := m.GetFeature("f1")
	if fu.DriftScore != 0.15 {
		t.Fatalf("expected drift 0.15, got %f", fu.DriftScore)
	}
	if fu.StorageBytes != 1024*1024*100 {
		t.Fatalf("expected 100MB storage, got %d", fu.StorageBytes)
	}
}

func TestManager_UpdateMetrics_NotFound(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	err := m.UpdateMetrics("nonexistent", 0, 0, 0)
	if err == nil {
		t.Fatal("expected error for non-tracked feature")
	}
}

func TestManager_AddAndListRules(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	err := m.AddRule(LifecycleRule{
		ID:   "r1",
		Name: "Deprecate unused",
		Condition: RuleCondition{Type: "unused_days", Threshold: 30, Operator: "gt"},
		Action:    RuleAction{Type: "deprecate", Message: "unused > 30 days"},
	})
	if err != nil {
		t.Fatalf("AddRule: %v", err)
	}

	rules := m.ListRules()
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
}

func TestManager_AddRule_Validation(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	err := m.AddRule(LifecycleRule{})
	if err == nil {
		t.Fatal("expected error for empty rule")
	}
}

func TestManager_RemoveRule(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	m.AddRule(LifecycleRule{ID: "r1", Name: "test"})
	if err := m.RemoveRule("r1"); err != nil {
		t.Fatalf("RemoveRule: %v", err)
	}
	if err := m.RemoveRule("r1"); err == nil {
		t.Fatal("expected error for non-existent rule")
	}
}

func TestManager_Evaluate_RuleTriggered(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	m.TrackFeature("stale_feature")

	// Simulate high drift
	m.UpdateMetrics("stale_feature", 0.5, 0.3, 1024)

	m.AddRule(LifecycleRule{
		ID:   "drift_alert",
		Name: "Alert on drift",
		Condition: RuleCondition{Type: "drift_threshold", Threshold: 0.2, Operator: "gt"},
		Action:    RuleAction{Type: "alert", Message: "high drift detected"},
		Enabled:   true,
	})

	events := m.Evaluate()
	found := false
	for _, e := range events {
		if e.Feature == "stale_feature" && e.Action == "alert" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected drift alert event for stale_feature")
	}
}

func TestManager_AutoDeprecation(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.DeprecationThresholdDays = 0 // immediate deprecation for testing
	m := NewManager(cfg)

	m.TrackFeature("old_feature")
	m.RecordAccess("old_feature", "test")

	// Set last access to the past
	m.mu.Lock()
	m.features["old_feature"].LastAccessed = time.Now().Add(-24 * time.Hour)
	m.mu.Unlock()

	events := m.Evaluate()

	found := false
	for _, e := range events {
		if e.Feature == "old_feature" && e.Action == "auto_deprecate" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected auto-deprecation event")
	}

	fu, _ := m.GetFeature("old_feature")
	if fu.State != StateDeprecated {
		t.Fatalf("expected deprecated state, got %s", fu.State)
	}
}

func TestManager_CostReport(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	m.TrackFeature("f1")
	m.TrackFeature("f2")
	m.UpdateMetrics("f1", 0, 1.0, 1024*1024*1024) // 1GB
	m.UpdateMetrics("f2", 0, 1.0, 512*1024*1024)   // 0.5GB

	report := m.CostReport(10)
	if report.TotalFeatures != 2 {
		t.Fatalf("expected 2 features, got %d", report.TotalFeatures)
	}
	if report.TotalStorageGB < 1.4 {
		t.Fatalf("expected ~1.5GB, got %f", report.TotalStorageGB)
	}
	if report.MonthlyEstUSD <= 0 {
		t.Fatal("expected positive monthly cost")
	}
	if len(report.TopCostly) != 2 {
		t.Fatalf("expected 2 costly features, got %d", len(report.TopCostly))
	}
	// f1 should be costliest
	if report.TopCostly[0].Name != "f1" {
		t.Fatalf("expected f1 as most costly, got %q", report.TopCostly[0].Name)
	}
}

func TestManager_Stats(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	m.TrackFeature("f1")
	m.TrackFeature("f2")
	m.RecordAccess("f1", "consumer1")

	stats := m.Stats()
	if stats.TotalFeatures != 2 {
		t.Fatalf("expected 2 features, got %d", stats.TotalFeatures)
	}
	if stats.ActiveFeatures != 2 {
		t.Fatalf("expected 2 active, got %d", stats.ActiveFeatures)
	}
	if stats.TotalConsumers != 1 {
		t.Fatalf("expected 1 consumer, got %d", stats.TotalConsumers)
	}
}

func TestManager_ListFeatures(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	m.TrackFeature("a")
	m.TrackFeature("b")

	list := m.ListFeatures()
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}
}

func TestManager_GetEvents(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	events := m.GetEvents(10)
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}
