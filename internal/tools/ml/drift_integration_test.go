package ml

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestModelDriftMonitor_RecordAlert(t *testing.T) {
	monitor := NewModelDriftMonitor()

	alert := &ModelDriftAlert{
		ModelID:      "model-1",
		ModelVersion: "v1.0",
		Feature:      "feature-1",
		DriftType:    "ks",
		Severity:     "medium",
		Score:        0.15,
		Threshold:    0.1,
		Message:      "Feature drift detected",
	}

	err := monitor.RecordAlert(alert)
	if err != nil {
		t.Fatalf("RecordAlert error: %v", err)
	}

	if alert.AlertID == "" {
		t.Error("expected alert ID to be set")
	}
	if alert.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestModelDriftMonitor_GetAlert(t *testing.T) {
	monitor := NewModelDriftMonitor()

	alert := &ModelDriftAlert{
		AlertID:      "alert-123",
		ModelID:      "model-1",
		ModelVersion: "v1.0",
		Feature:      "feature-1",
		DriftType:    "ks",
	}

	monitor.RecordAlert(alert)

	retrieved, err := monitor.GetAlert("alert-123")
	if err != nil {
		t.Fatalf("GetAlert error: %v", err)
	}

	if retrieved.ModelID != "model-1" {
		t.Errorf("expected model ID 'model-1', got '%s'", retrieved.ModelID)
	}
}

func TestModelDriftMonitor_GetAlert_NotFound(t *testing.T) {
	monitor := NewModelDriftMonitor()

	_, err := monitor.GetAlert("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent alert")
	}
}

func TestModelDriftMonitor_GetAlertsForModel(t *testing.T) {
	monitor := NewModelDriftMonitor()

	// Add alerts for different models
	monitor.RecordAlert(&ModelDriftAlert{
		ModelID:      "model-1",
		ModelVersion: "v1.0",
		Feature:      "feature-1",
	})
	// Wait to avoid cooldown
	time.Sleep(1 * time.Millisecond)
	monitor.alertCooldown = 0 // Disable cooldown for testing

	monitor.RecordAlert(&ModelDriftAlert{
		ModelID:      "model-1",
		ModelVersion: "v1.0",
		Feature:      "feature-2",
	})
	monitor.RecordAlert(&ModelDriftAlert{
		ModelID:      "model-2",
		ModelVersion: "v1.0",
		Feature:      "feature-1",
	})

	alerts := monitor.GetAlertsForModel("model-1", "v1.0")
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts, got %d", len(alerts))
	}
}

func TestModelDriftMonitor_GetAlertsForFeature(t *testing.T) {
	monitor := NewModelDriftMonitor()
	monitor.alertCooldown = 0

	monitor.RecordAlert(&ModelDriftAlert{
		ModelID:      "model-1",
		ModelVersion: "v1.0",
		Feature:      "feature-1",
	})
	monitor.RecordAlert(&ModelDriftAlert{
		ModelID:      "model-2",
		ModelVersion: "v1.0",
		Feature:      "feature-1",
	})
	monitor.RecordAlert(&ModelDriftAlert{
		ModelID:      "model-1",
		ModelVersion: "v1.0",
		Feature:      "feature-2",
	})

	alerts := monitor.GetAlertsForFeature("feature-1")
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts, got %d", len(alerts))
	}
}

func TestModelDriftMonitor_GetRecentAlerts(t *testing.T) {
	monitor := NewModelDriftMonitor()
	monitor.alertCooldown = 0

	start := time.Now()

	monitor.RecordAlert(&ModelDriftAlert{
		ModelID:   "model-1",
		Feature:   "feature-1",
		Timestamp: start.Add(-10 * time.Minute),
	})
	monitor.RecordAlert(&ModelDriftAlert{
		ModelID: "model-1",
		Feature: "feature-2",
	})

	alerts := monitor.GetRecentAlerts(start.Add(-5 * time.Minute))
	if len(alerts) != 1 {
		t.Errorf("expected 1 recent alert, got %d", len(alerts))
	}
}

func TestModelDriftMonitor_GetUnacknowledgedAlerts(t *testing.T) {
	monitor := NewModelDriftMonitor()
	monitor.alertCooldown = 0

	monitor.RecordAlert(&ModelDriftAlert{
		AlertID:      "alert-1",
		ModelID:      "model-1",
		Feature:      "feature-1",
		Acknowledged: false,
	})
	monitor.RecordAlert(&ModelDriftAlert{
		AlertID:      "alert-2",
		ModelID:      "model-1",
		Feature:      "feature-2",
		Acknowledged: true,
	})

	alerts := monitor.GetUnacknowledgedAlerts()
	if len(alerts) != 1 {
		t.Errorf("expected 1 unacknowledged alert, got %d", len(alerts))
	}
}

func TestModelDriftMonitor_AcknowledgeAlert(t *testing.T) {
	monitor := NewModelDriftMonitor()

	monitor.RecordAlert(&ModelDriftAlert{
		AlertID: "alert-1",
		ModelID: "model-1",
		Feature: "feature-1",
	})

	err := monitor.AcknowledgeAlert("alert-1", "user@example.com")
	if err != nil {
		t.Fatalf("AcknowledgeAlert error: %v", err)
	}

	alert, _ := monitor.GetAlert("alert-1")
	if !alert.Acknowledged {
		t.Error("expected alert to be acknowledged")
	}
	if alert.AcknowledgedBy != "user@example.com" {
		t.Errorf("expected acknowledged by 'user@example.com', got '%s'", alert.AcknowledgedBy)
	}
	if alert.AcknowledgedAt == nil {
		t.Error("expected acknowledged_at to be set")
	}
}

func TestModelDriftMonitor_AcknowledgeAlert_NotFound(t *testing.T) {
	monitor := NewModelDriftMonitor()

	err := monitor.AcknowledgeAlert("nonexistent", "user")
	if err == nil {
		t.Error("expected error for nonexistent alert")
	}
}

func TestModelDriftMonitor_DeleteAlert(t *testing.T) {
	monitor := NewModelDriftMonitor()

	monitor.RecordAlert(&ModelDriftAlert{
		AlertID:      "alert-1",
		ModelID:      "model-1",
		ModelVersion: "v1.0",
		Feature:      "feature-1",
	})

	err := monitor.DeleteAlert("alert-1")
	if err != nil {
		t.Fatalf("DeleteAlert error: %v", err)
	}

	_, err = monitor.GetAlert("alert-1")
	if err == nil {
		t.Error("expected error after deletion")
	}

	// Verify indexes are cleaned up
	alerts := monitor.GetAlertsForModel("model-1", "v1.0")
	if len(alerts) != 0 {
		t.Error("expected no alerts for model after deletion")
	}

	alerts = monitor.GetAlertsForFeature("feature-1")
	if len(alerts) != 0 {
		t.Error("expected no alerts for feature after deletion")
	}
}

func TestModelDriftMonitor_DeleteAlert_NotFound(t *testing.T) {
	monitor := NewModelDriftMonitor()

	err := monitor.DeleteAlert("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent alert")
	}
}

func TestModelDriftMonitor_OnAlert(t *testing.T) {
	monitor := NewModelDriftMonitor()
	monitor.alertCooldown = 0

	var called atomic.Bool
	alertCh := make(chan *ModelDriftAlert, 1)

	monitor.OnAlert(func(alert *ModelDriftAlert) {
		called.Store(true)
		alertCh <- alert
	})

	monitor.RecordAlert(&ModelDriftAlert{
		ModelID: "model-1",
		Feature: "feature-1",
	})

	received := <-alertCh
	if !called.Load() {
		t.Error("expected callback to be called")
	}
	if received == nil || received.ModelID != "model-1" {
		t.Error("expected correct alert in callback")
	}
}

func TestModelDriftMonitor_Stats(t *testing.T) {
	monitor := NewModelDriftMonitor()
	monitor.alertCooldown = 0

	monitor.RecordAlert(&ModelDriftAlert{
		ModelID:  "model-1",
		Feature:  "feature-1",
		Severity: "high",
	})
	monitor.RecordAlert(&ModelDriftAlert{
		ModelID:  "model-2",
		Feature:  "feature-2",
		Severity: "low",
	})

	stats := monitor.Stats()

	if stats["total_alerts"].(int) != 2 {
		t.Errorf("expected 2 total alerts, got %v", stats["total_alerts"])
	}
	if stats["unacknowledged"].(int) != 2 {
		t.Errorf("expected 2 unacknowledged, got %v", stats["unacknowledged"])
	}
}

func TestModelDriftMonitor_Cooldown(t *testing.T) {
	monitor := NewModelDriftMonitor()
	monitor.alertCooldown = 1 * time.Hour // Long cooldown

	// First alert should succeed
	monitor.RecordAlert(&ModelDriftAlert{
		ModelID:      "model-1",
		ModelVersion: "v1.0",
		Feature:      "feature-1",
	})

	// Second alert with same key should be skipped due to cooldown
	monitor.RecordAlert(&ModelDriftAlert{
		ModelID:      "model-1",
		ModelVersion: "v1.0",
		Feature:      "feature-1",
	})

	alerts := monitor.GetAlertsForModel("model-1", "v1.0")
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert due to cooldown, got %d", len(alerts))
	}
}

func TestModelDriftMonitor_TrimOldAlerts(t *testing.T) {
	monitor := NewModelDriftMonitor()
	monitor.maxAlerts = 5
	monitor.alertCooldown = 0

	// Add more than max alerts
	for i := 0; i < 10; i++ {
		monitor.RecordAlert(&ModelDriftAlert{
			ModelID: "model-1",
			Feature: string(rune('a' + i)),
		})
	}

	if len(monitor.alerts) > 5 {
		t.Errorf("expected at most 5 alerts, got %d", len(monitor.alerts))
	}
}

// Tests for DriftDetectorBridge

func TestDriftDetectorBridge_RegisterModel(t *testing.T) {
	registry := NewModelRegistry()
	monitor := NewModelDriftMonitor()
	bridge := NewDriftDetectorBridge(registry, monitor)

	bridge.RegisterModel("model-1", []string{"feature-a", "feature-b"})

	features := bridge.GetFeaturesForModel("model-1")
	if len(features) != 2 {
		t.Errorf("expected 2 features, got %d", len(features))
	}

	models := bridge.GetModelsForFeature("feature-a")
	if len(models) != 1 || models[0] != "model-1" {
		t.Error("expected model-1 to be registered for feature-a")
	}
}

func TestDriftDetectorBridge_UnregisterModel(t *testing.T) {
	registry := NewModelRegistry()
	monitor := NewModelDriftMonitor()
	bridge := NewDriftDetectorBridge(registry, monitor)

	bridge.RegisterModel("model-1", []string{"feature-a", "feature-b"})
	bridge.UnregisterModel("model-1")

	features := bridge.GetFeaturesForModel("model-1")
	if len(features) != 0 {
		t.Errorf("expected 0 features after unregister, got %d", len(features))
	}

	models := bridge.GetModelsForFeature("feature-a")
	if len(models) != 0 {
		t.Errorf("expected 0 models after unregister, got %d", len(models))
	}
}

func TestDriftDetectorBridge_OnDriftDetected(t *testing.T) {
	registry := NewModelRegistry()
	monitor := NewModelDriftMonitor()
	bridge := NewDriftDetectorBridge(registry, monitor)

	// Register a model
	registry.RegisterModel(&Model{
		ID:   "model-1",
		Name: "Test Model",
	})
	registry.RegisterVersion("model-1", &ModelVersion{
		Version:  "v1.0",
		Features: []string{"feature-a", "feature-b"},
	})
	registry.ActivateVersion("model-1", "v1.0")

	bridge.RegisterModel("model-1", []string{"feature-a", "feature-b"})
	bridge.SetCheckCooldown(0) // Disable cooldown for testing

	// Simulate drift detection
	bridge.OnDriftDetected("feature-a", "ks", 0.15, 0.1)

	alerts := monitor.GetAlertsForModel("model-1", "v1.0")
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(alerts))
	}

	if alerts[0].Feature != "feature-a" {
		t.Errorf("expected feature 'feature-a', got '%s'", alerts[0].Feature)
	}
	if alerts[0].DriftType != "ks" {
		t.Errorf("expected drift type 'ks', got '%s'", alerts[0].DriftType)
	}
	if alerts[0].Severity != "medium" {
		t.Errorf("expected severity 'medium', got '%s'", alerts[0].Severity)
	}
}

func TestDriftDetectorBridge_OnDriftDetected_MultipleModels(t *testing.T) {
	registry := NewModelRegistry()
	monitor := NewModelDriftMonitor()
	monitor.alertCooldown = 0
	bridge := NewDriftDetectorBridge(registry, monitor)
	bridge.SetCheckCooldown(0)

	// Register two models using the same feature
	registry.RegisterModel(&Model{ID: "model-1", Name: "Model 1"})
	registry.RegisterVersion("model-1", &ModelVersion{Version: "v1.0", Features: []string{"shared-feature"}})
	registry.ActivateVersion("model-1", "v1.0")

	registry.RegisterModel(&Model{ID: "model-2", Name: "Model 2"})
	registry.RegisterVersion("model-2", &ModelVersion{Version: "v1.0", Features: []string{"shared-feature"}})
	registry.ActivateVersion("model-2", "v1.0")

	bridge.RegisterModel("model-1", []string{"shared-feature"})
	bridge.RegisterModel("model-2", []string{"shared-feature"})

	// Detect drift on shared feature
	bridge.OnDriftDetected("shared-feature", "ks", 0.25, 0.1)

	alerts1 := monitor.GetAlertsForModel("model-1", "v1.0")
	alerts2 := monitor.GetAlertsForModel("model-2", "v1.0")

	if len(alerts1) != 1 {
		t.Errorf("expected 1 alert for model-1, got %d", len(alerts1))
	}
	if len(alerts2) != 1 {
		t.Errorf("expected 1 alert for model-2, got %d", len(alerts2))
	}
}

func TestDriftDetectorBridge_SeverityLevels(t *testing.T) {
	registry := NewModelRegistry()
	monitor := NewModelDriftMonitor()
	bridge := NewDriftDetectorBridge(registry, monitor)

	tests := []struct {
		score    float64
		expected string
	}{
		{0.05, "low"},
		{0.15, "medium"},
		{0.25, "high"},
		{0.35, "critical"},
	}

	for _, tt := range tests {
		severity := bridge.scoreToseverity(tt.score)
		if severity != tt.expected {
			t.Errorf("score %.2f: expected severity '%s', got '%s'", tt.score, tt.expected, severity)
		}
	}
}

func TestDriftDetectorBridge_Cooldown(t *testing.T) {
	registry := NewModelRegistry()
	monitor := NewModelDriftMonitor()
	monitor.alertCooldown = 0
	bridge := NewDriftDetectorBridge(registry, monitor)
	bridge.SetCheckCooldown(1 * time.Hour) // Long cooldown

	registry.RegisterModel(&Model{ID: "model-1", Name: "Model 1"})
	registry.RegisterVersion("model-1", &ModelVersion{Version: "v1.0", Features: []string{"feature-a"}})
	registry.ActivateVersion("model-1", "v1.0")

	bridge.RegisterModel("model-1", []string{"feature-a"})

	// First drift should create alert
	bridge.OnDriftDetected("feature-a", "ks", 0.15, 0.1)

	// Second drift should be skipped due to cooldown
	bridge.OnDriftDetected("feature-a", "ks", 0.20, 0.1)

	alerts := monitor.GetAlertsForModel("model-1", "v1.0")
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert due to cooldown, got %d", len(alerts))
	}
}

func TestDriftDetectorBridge_Stats(t *testing.T) {
	registry := NewModelRegistry()
	monitor := NewModelDriftMonitor()
	bridge := NewDriftDetectorBridge(registry, monitor)

	bridge.RegisterModel("model-1", []string{"feature-a", "feature-b"})
	bridge.RegisterModel("model-2", []string{"feature-a"})

	stats := bridge.Stats()

	if stats["monitored_models"].(int) != 2 {
		t.Errorf("expected 2 monitored models, got %v", stats["monitored_models"])
	}
	if stats["monitored_features"].(int) != 2 {
		t.Errorf("expected 2 monitored features, got %v", stats["monitored_features"])
	}
}

func TestDriftDetectorBridge_ReregisterModel(t *testing.T) {
	registry := NewModelRegistry()
	monitor := NewModelDriftMonitor()
	bridge := NewDriftDetectorBridge(registry, monitor)

	// Initial registration
	bridge.RegisterModel("model-1", []string{"feature-a", "feature-b"})

	// Re-register with different features
	bridge.RegisterModel("model-1", []string{"feature-c", "feature-d"})

	features := bridge.GetFeaturesForModel("model-1")
	if len(features) != 2 {
		t.Errorf("expected 2 features, got %d", len(features))
	}

	// Old features should not be mapped
	models := bridge.GetModelsForFeature("feature-a")
	if len(models) != 0 {
		t.Error("expected feature-a to be unmapped from model-1")
	}

	// New features should be mapped
	models = bridge.GetModelsForFeature("feature-c")
	if len(models) != 1 {
		t.Error("expected feature-c to be mapped to model-1")
	}
}

func TestModelServingOrchestrator_Creation(t *testing.T) {
	registry := NewModelRegistry()
	snapshotStore := NewSnapshotStore()
	config := DefaultValidatorConfig()

	orchestrator := NewModelServingOrchestrator(registry, snapshotStore, config)

	if orchestrator.Registry() != registry {
		t.Error("expected registry to be accessible")
	}
	if orchestrator.SnapshotStore() != snapshotStore {
		t.Error("expected snapshot store to be accessible")
	}
	if orchestrator.Validator() == nil {
		t.Error("expected validator to be created")
	}
	if orchestrator.DriftMonitor() == nil {
		t.Error("expected drift monitor to be created")
	}
}

func TestModelServingOrchestrator_Stats(t *testing.T) {
	registry := NewModelRegistry()
	snapshotStore := NewSnapshotStore()
	config := DefaultValidatorConfig()

	orchestrator := NewModelServingOrchestrator(registry, snapshotStore, config)

	stats := orchestrator.Stats()

	if _, ok := stats["registry"]; !ok {
		t.Error("expected registry stats")
	}
	if _, ok := stats["snapshots"]; !ok {
		t.Error("expected snapshots stats")
	}
	if _, ok := stats["validator"]; !ok {
		t.Error("expected validator stats")
	}
	if _, ok := stats["drift_monitor"]; !ok {
		t.Error("expected drift_monitor stats")
	}
}
