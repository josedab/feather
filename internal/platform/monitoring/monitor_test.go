package monitoring

import (
	"errors"
	"testing"
	"time"
)

func TestManager_RegisterMonitor(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	mon := &FeatureMonitor{
		ID:          "mon-1",
		FeatureName: "user_age",
		Type:        MonitorTypeDrift,
		Threshold:   0.5,
		Enabled:     true,
	}

	if err := m.RegisterMonitor(mon); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := m.GetMonitor("mon-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.FeatureName != "user_age" {
		t.Errorf("got feature name %q, want %q", got.FeatureName, "user_age")
	}
	if got.Status != StatusUnknown {
		t.Errorf("got status %q, want %q", got.Status, StatusUnknown)
	}
}

func TestManager_RegisterMonitor_Duplicate(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	mon := &FeatureMonitor{ID: "mon-dup", FeatureName: "feat1", Enabled: true}
	if err := m.RegisterMonitor(mon); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err := m.RegisterMonitor(&FeatureMonitor{ID: "mon-dup", FeatureName: "feat2"})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("got error %v, want ErrAlreadyExists", err)
	}
}

func TestManager_RecordValue_Healthy(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	mon := &FeatureMonitor{
		ID:          "mon-healthy",
		FeatureName: "click_rate",
		Type:        MonitorTypeValue,
		Threshold:   1.0,
		Enabled:     true,
	}
	if err := m.RegisterMonitor(mon); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Add a rule that triggers above 1.0
	rule := &AlertRule{
		ID:        "rule-1",
		MonitorID: "mon-healthy",
		Severity:  SeverityWarning,
		Condition: "above",
		Threshold: 1.0,
		Enabled:   true,
	}
	if err := m.AddRule(rule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Record a value within threshold
	if err := m.RecordValue("mon-healthy", 0.5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := m.GetMonitor("mon-healthy")
	if got.Status != StatusHealthy {
		t.Errorf("got status %q, want %q", got.Status, StatusHealthy)
	}
	if got.CheckCount != 1 {
		t.Errorf("got check count %d, want 1", got.CheckCount)
	}

	alerts := m.GetAlerts(time.Time{})
	if len(alerts) != 0 {
		t.Errorf("got %d alerts, want 0", len(alerts))
	}
}

func TestManager_RecordValue_Alert(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	mon := &FeatureMonitor{
		ID:          "mon-alert",
		FeatureName: "latency_p99",
		Type:        MonitorTypeValue,
		Enabled:     true,
	}
	if err := m.RegisterMonitor(mon); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rule := &AlertRule{
		ID:        "rule-crit",
		MonitorID: "mon-alert",
		Severity:  SeverityCritical,
		Condition: "above",
		Threshold: 100.0,
		Cooldown:  0, // no cooldown for test
		Enabled:   true,
	}
	if err := m.AddRule(rule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := m.RecordValue("mon-alert", 150.0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := m.GetMonitor("mon-alert")
	if got.Status != StatusCritical {
		t.Errorf("got status %q, want %q", got.Status, StatusCritical)
	}

	alerts := m.GetAlerts(time.Time{})
	if len(alerts) != 1 {
		t.Fatalf("got %d alerts, want 1", len(alerts))
	}
	if alerts[0].Severity != SeverityCritical {
		t.Errorf("got severity %q, want %q", alerts[0].Severity, SeverityCritical)
	}
	if alerts[0].Value != 150.0 {
		t.Errorf("got value %.2f, want 150.00", alerts[0].Value)
	}
}

func TestManager_AlertCooldown(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.AlertCooldown = 1 * time.Hour
	m := NewManager(cfg)

	mon := &FeatureMonitor{
		ID:          "mon-cool",
		FeatureName: "error_rate",
		Type:        MonitorTypeValue,
		Enabled:     true,
	}
	if err := m.RegisterMonitor(mon); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rule := &AlertRule{
		ID:        "rule-cool",
		MonitorID: "mon-cool",
		Severity:  SeverityWarning,
		Condition: "above",
		Threshold: 0.1,
		Enabled:   true,
		// Cooldown will default to cfg.AlertCooldown (1 hour)
	}
	if err := m.AddRule(rule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First record should trigger alert
	if err := m.RecordValue("mon-cool", 0.5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	alerts := m.GetAlerts(time.Time{})
	if len(alerts) != 1 {
		t.Fatalf("got %d alerts after first record, want 1", len(alerts))
	}

	// Second record should NOT trigger (cooldown not elapsed)
	if err := m.RecordValue("mon-cool", 0.6); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	alerts = m.GetAlerts(time.Time{})
	if len(alerts) != 1 {
		t.Errorf("got %d alerts after second record, want 1 (cooldown should prevent)", len(alerts))
	}
}

func TestManager_AcknowledgeAlert(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	mon := &FeatureMonitor{ID: "mon-ack", FeatureName: "feat", Enabled: true}
	_ = m.RegisterMonitor(mon)

	rule := &AlertRule{
		ID:        "rule-ack",
		MonitorID: "mon-ack",
		Severity:  SeverityWarning,
		Condition: "above",
		Threshold: 0.0,
		Cooldown:  0,
		Enabled:   true,
	}
	_ = m.AddRule(rule)
	_ = m.RecordValue("mon-ack", 1.0)

	alerts := m.GetAlerts(time.Time{})
	if len(alerts) == 0 {
		t.Fatal("expected at least one alert")
	}

	alertID := alerts[0].ID
	if err := m.AcknowledgeAlert(alertID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify acknowledged
	alerts = m.GetAlerts(time.Time{})
	if !alerts[0].Acknowledged {
		t.Error("alert should be acknowledged")
	}

	// Acknowledge non-existent alert
	if err := m.AcknowledgeAlert("nonexistent"); err == nil {
		t.Error("expected error for nonexistent alert")
	}
}

func TestManager_Summary(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	// Add monitors with various statuses
	monitors := []struct {
		id     string
		status MonitorStatus
	}{
		{"m1", StatusHealthy},
		{"m2", StatusHealthy},
		{"m3", StatusWarning},
		{"m4", StatusCritical},
	}
	for _, tc := range monitors {
		_ = m.RegisterMonitor(&FeatureMonitor{
			ID:      tc.id,
			Status:  tc.status,
			Enabled: true,
		})
	}

	// Add a rule
	_ = m.AddRule(&AlertRule{ID: "r1", MonitorID: "m4", Severity: SeverityCritical, Condition: "above", Threshold: 0, Enabled: true})

	// Fire an alert
	_ = m.RecordValue("m4", 1.0)

	m.AddNotifier(NewLogNotifier("test-log"))

	s := m.Summary()
	if s.TotalMonitors != 4 {
		t.Errorf("got total monitors %d, want 4", s.TotalMonitors)
	}
	if s.HealthyCount != 2 {
		t.Errorf("got healthy %d, want 2", s.HealthyCount)
	}
	if s.WarningCount != 1 {
		t.Errorf("got warning %d, want 1", s.WarningCount)
	}
	if s.CriticalCount != 1 {
		t.Errorf("got critical %d, want 1", s.CriticalCount)
	}
	if s.TotalAlerts != 1 {
		t.Errorf("got total alerts %d, want 1", s.TotalAlerts)
	}
	if s.UnackedAlerts != 1 {
		t.Errorf("got unacked alerts %d, want 1", s.UnackedAlerts)
	}
	if s.RuleCount != 1 {
		t.Errorf("got rule count %d, want 1", s.RuleCount)
	}
	if s.NotifierCount != 1 {
		t.Errorf("got notifier count %d, want 1", s.NotifierCount)
	}
}

func TestRemediationEngine_Evaluate(t *testing.T) {
	re := NewRemediationEngine()

	actions := []*RemediationAction{
		{
			ID:          "act-1",
			Name:        "notify-slack",
			Trigger:     SeverityCritical,
			MonitorType: MonitorTypeDrift,
			ActionType:  ActionNotify,
			Enabled:     true,
		},
		{
			ID:          "act-2",
			Name:        "fallback-default",
			Trigger:     SeverityCritical,
			MonitorType: MonitorTypeValue,
			ActionType:  ActionFallback,
			Enabled:     true,
		},
		{
			ID:          "act-3",
			Name:        "disabled-action",
			Trigger:     SeverityCritical,
			MonitorType: MonitorTypeValue,
			ActionType:  ActionDisable,
			Enabled:     false,
		},
		{
			ID:          "act-4",
			Name:        "info-action",
			Trigger:     SeverityInfo,
			MonitorType: MonitorTypeVolume,
			ActionType:  ActionNotify,
			Enabled:     true,
		},
	}

	for _, a := range actions {
		if err := re.RegisterAction(a); err != nil {
			t.Fatalf("unexpected error registering action %s: %v", a.ID, err)
		}
	}

	// Evaluate critical alert
	alert := Alert{Severity: SeverityCritical, FeatureName: "test_feat"}
	matched := re.Evaluate(alert)
	if len(matched) != 2 {
		t.Errorf("got %d matched actions for critical, want 2", len(matched))
	}

	// Evaluate info alert
	infoAlert := Alert{Severity: SeverityInfo, FeatureName: "test_feat"}
	matched = re.Evaluate(infoAlert)
	if len(matched) != 1 {
		t.Errorf("got %d matched actions for info, want 1", len(matched))
	}

	// Execute an action
	if err := re.Execute(actions[0], alert); err != nil {
		t.Fatalf("unexpected error executing action: %v", err)
	}
	if actions[0].ExecutionCount != 1 {
		t.Errorf("got execution count %d, want 1", actions[0].ExecutionCount)
	}
}

func TestWebhookNotifier_Name(t *testing.T) {
	wh := NewWebhookNotifier("slack-webhook", "https://hooks.example.com/test")
	if wh.Name() != "slack-webhook" {
		t.Errorf("got name %q, want %q", wh.Name(), "slack-webhook")
	}
}

func TestLogNotifier_Notify(t *testing.T) {
	ln := NewLogNotifier("test-logger")
	if ln.Name() != "test-logger" {
		t.Errorf("got name %q, want %q", ln.Name(), "test-logger")
	}

	alert := Alert{
		ID:          "alert-1",
		FeatureName: "test_feature",
		Severity:    SeverityWarning,
		Message:     "test alert message",
		Value:       1.5,
		Threshold:   1.0,
		Timestamp:   time.Now(),
	}

	if err := ln.Notify(alert); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
