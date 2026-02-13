package observability

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestAlertManager_Trigger(t *testing.T) {
	m := NewAlertManager()
	ctx := context.Background()

	alert := m.Trigger(ctx, AlertTypePerformance, SeverityWarning, "feat1",
		"High latency", "P99 exceeded", 150.0, 100.0)

	if alert.ID == "" {
		t.Fatal("expected alert ID")
	}
	if alert.Type != AlertTypePerformance {
		t.Fatalf("expected type performance, got %s", alert.Type)
	}
	if alert.Severity != SeverityWarning {
		t.Fatalf("expected severity warning, got %s", alert.Severity)
	}
	if alert.Feature != "feat1" {
		t.Fatalf("expected feature feat1, got %s", alert.Feature)
	}
	if alert.CreatedAt.IsZero() {
		t.Fatal("expected non-zero created_at")
	}
}

func TestAlertManager_TriggerNotifiesHandlers(t *testing.T) {
	m := NewAlertManager()
	ctx := context.Background()

	var called atomic.Int32
	m.AddHandler(AlertHandlerFunc(func(ctx context.Context, alert *Alert) error {
		called.Add(1)
		return nil
	}))

	m.Trigger(ctx, AlertTypeFreshness, SeverityInfo, "f1", "title", "desc", nil, nil)

	// Wait briefly for async handler
	time.Sleep(50 * time.Millisecond)

	if called.Load() != 1 {
		t.Fatalf("expected handler called once, got %d", called.Load())
	}
}

func TestAlertManager_TriggerTruncatesTo1000(t *testing.T) {
	m := NewAlertManager()
	ctx := context.Background()

	for i := 0; i < 1005; i++ {
		m.Trigger(ctx, AlertTypePerformance, SeverityInfo, "f1", "title", "desc", nil, nil)
	}

	alerts := m.GetAlerts("", "", time.Time{})
	if len(alerts) != 1000 {
		t.Fatalf("expected 1000 alerts after truncation, got %d", len(alerts))
	}
}

func TestAlertManager_UniqueIDs(t *testing.T) {
	m := NewAlertManager()
	ctx := context.Background()

	a1 := m.Trigger(ctx, AlertTypePerformance, SeverityInfo, "f1", "t1", "d1", nil, nil)
	a2 := m.Trigger(ctx, AlertTypePerformance, SeverityInfo, "f1", "t2", "d2", nil, nil)
	if a1.ID == a2.ID {
		t.Fatal("expected unique alert IDs")
	}
}

func TestAlertManager_Acknowledge(t *testing.T) {
	m := NewAlertManager()
	ctx := context.Background()

	alert := m.Trigger(ctx, AlertTypePerformance, SeverityInfo, "f1", "t", "d", nil, nil)

	ok := m.Acknowledge(alert.ID, "user1")
	if !ok {
		t.Fatal("expected acknowledge to succeed")
	}
	if alert.AckAt == nil {
		t.Fatal("expected ack_at to be set")
	}
	if alert.AckBy != "user1" {
		t.Fatalf("expected ack_by user1, got %s", alert.AckBy)
	}
}

func TestAlertManager_AcknowledgeNotFound(t *testing.T) {
	m := NewAlertManager()
	ok := m.Acknowledge("nonexistent", "user1")
	if ok {
		t.Fatal("expected acknowledge to fail for nonexistent alert")
	}
}

func TestAlertManager_Resolve(t *testing.T) {
	m := NewAlertManager()
	ctx := context.Background()

	alert := m.Trigger(ctx, AlertTypePerformance, SeverityInfo, "f1", "t", "d", nil, nil)
	ok := m.Resolve(alert.ID)
	if !ok {
		t.Fatal("expected resolve to succeed")
	}
	if alert.ResolvedAt == nil {
		t.Fatal("expected resolved_at to be set")
	}
}

func TestAlertManager_ResolveNotFound(t *testing.T) {
	m := NewAlertManager()
	ok := m.Resolve("nonexistent")
	if ok {
		t.Fatal("expected resolve to fail for nonexistent alert")
	}
}

func TestAlertManager_GetAlertsFilters(t *testing.T) {
	m := NewAlertManager()
	ctx := context.Background()

	m.Trigger(ctx, AlertTypePerformance, SeverityInfo, "f1", "t1", "d1", nil, nil)
	m.Trigger(ctx, AlertTypeFreshness, SeverityInfo, "f2", "t2", "d2", nil, nil)
	m.Trigger(ctx, AlertTypePerformance, SeverityInfo, "f2", "t3", "d3", nil, nil)

	// Filter by type
	alerts := m.GetAlerts(AlertTypePerformance, "", time.Time{})
	if len(alerts) != 2 {
		t.Fatalf("expected 2 performance alerts, got %d", len(alerts))
	}

	// Filter by feature
	alerts = m.GetAlerts("", "f2", time.Time{})
	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts for f2, got %d", len(alerts))
	}

	// Filter by both
	alerts = m.GetAlerts(AlertTypePerformance, "f2", time.Time{})
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
}

func TestAlertManager_AddRemoveRules(t *testing.T) {
	m := NewAlertManager()
	rule := &AlertRule{Name: "r1", Type: AlertTypePerformance, Feature: "f1", Enabled: true}
	m.AddRule(rule)

	rules := m.GetRules()
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	m.RemoveRule("r1")
	rules = m.GetRules()
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules after removal, got %d", len(rules))
	}
}

func TestEvaluateCondition_AllOperators(t *testing.T) {
	m := NewAlertManager()

	tests := []struct {
		cond      string
		value     float64
		threshold float64
		expected  bool
	}{
		{"gt", 10, 5, true},
		{"gt", 5, 10, false},
		{"lt", 5, 10, true},
		{"lt", 10, 5, false},
		{"eq", 5, 5, true},
		{"eq", 5, 6, false},
		{"ne", 5, 6, true},
		{"ne", 5, 5, false},
		{"gte", 5, 5, true},
		{"gte", 6, 5, true},
		{"gte", 4, 5, false},
		{"lte", 5, 5, true},
		{"lte", 4, 5, true},
		{"lte", 6, 5, false},
		{"unknown", 5, 5, false},
	}

	for _, tt := range tests {
		result := m.evaluateCondition(tt.cond, tt.value, tt.threshold)
		if result != tt.expected {
			t.Errorf("evaluateCondition(%q, %f, %f) = %v, want %v",
				tt.cond, tt.value, tt.threshold, result, tt.expected)
		}
	}
}

func TestAlertManager_CheckRules_NoRules(t *testing.T) {
	m := NewAlertManager()
	metrics := NewMetricsCollector()
	ctx := context.Background()

	m.CheckRules(ctx, metrics)
	alerts := m.GetAlerts("", "", time.Time{})
	if len(alerts) != 0 {
		t.Fatalf("expected no alerts with no rules, got %d", len(alerts))
	}
}

func TestAlertManager_CheckRules_DisabledRule(t *testing.T) {
	m := NewAlertManager()
	metrics := NewMetricsCollector()
	ctx := context.Background()

	metrics.RecordRead("f1", time.Millisecond, false, 100)

	m.AddRule(&AlertRule{
		Name:      "disabled_rule",
		Type:      AlertTypePerformance,
		Feature:   "f1",
		Condition: "gt",
		Threshold: 0,
		Severity:  SeverityWarning,
		Enabled:   false,
	})

	m.CheckRules(ctx, metrics)
	alerts := m.GetAlerts("", "", time.Time{})
	if len(alerts) != 0 {
		t.Fatalf("expected no alerts for disabled rule, got %d", len(alerts))
	}
}

func TestAlertManager_CheckRules_MissingMetric(t *testing.T) {
	m := NewAlertManager()
	metrics := NewMetricsCollector()
	ctx := context.Background()

	m.AddRule(&AlertRule{
		Name:      "missing_metric",
		Type:      AlertTypePerformance,
		Feature:   "nonexistent",
		Condition: "gt",
		Threshold: 0,
		Severity:  SeverityWarning,
		Enabled:   true,
	})

	m.CheckRules(ctx, metrics)
	alerts := m.GetAlerts("", "", time.Time{})
	if len(alerts) != 0 {
		t.Fatalf("expected no alerts for missing metric, got %d", len(alerts))
	}
}

func TestAlertManager_CheckRules_Cooldown(t *testing.T) {
	m := NewAlertManager()
	metrics := NewMetricsCollector()
	ctx := context.Background()

	metrics.RecordRead("f1", time.Millisecond, false, 100)

	m.AddRule(&AlertRule{
		Name:      "cooldown_rule",
		Type:      AlertTypePerformance,
		Feature:   "f1",
		Condition: "gte",
		Threshold: 0,
		Severity:  SeverityWarning,
		Enabled:   true,
		Cooldown:  time.Hour,
		LastFired: time.Now(), // Recently fired
	})

	m.CheckRules(ctx, metrics)
	alerts := m.GetAlerts("", "", time.Time{})
	if len(alerts) != 0 {
		t.Fatalf("expected no alerts due to cooldown, got %d", len(alerts))
	}
}

func TestAlertManager_CheckRules_PerformanceAlert(t *testing.T) {
	m := NewAlertManager()
	metrics := NewMetricsCollector()
	ctx := context.Background()

	// Record a read to populate P99 latency
	metrics.RecordRead("f1", 200*time.Microsecond, false, 100)

	m.AddRule(&AlertRule{
		Name:      "perf_rule",
		Type:      AlertTypePerformance,
		Feature:   "f1",
		Condition: "gte",
		Threshold: 0,
		Severity:  SeverityCritical,
		Enabled:   true,
	})

	m.CheckRules(ctx, metrics)
	alerts := m.GetAlerts(AlertTypePerformance, "f1", time.Time{})
	if len(alerts) != 1 {
		t.Fatalf("expected 1 performance alert, got %d", len(alerts))
	}
}

func TestAlertManager_AlertLifecycle(t *testing.T) {
	m := NewAlertManager()
	ctx := context.Background()

	alert := m.Trigger(ctx, AlertTypeAnomaly, SeverityError, "f1", "Anomaly", "desc", nil, nil)

	// Initially no ack/resolve
	if alert.AckAt != nil || alert.ResolvedAt != nil {
		t.Fatal("expected no ack/resolve initially")
	}

	m.Acknowledge(alert.ID, "admin")
	if alert.AckAt == nil {
		t.Fatal("expected ack after acknowledge")
	}

	m.Resolve(alert.ID)
	if alert.ResolvedAt == nil {
		t.Fatal("expected resolve after resolve")
	}
}
