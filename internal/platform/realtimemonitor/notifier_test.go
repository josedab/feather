package realtimemonitor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAddNotifier_Slack(t *testing.T) {
	an := NewAlertNotifier()
	err := an.AddNotifier(NotifierConfig{
		ID:      "slack-1",
		Type:    NotifierSlack,
		Name:    "Slack Channel",
		URL:     "https://hooks.slack.com/test",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	notifiers := an.ListNotifiers()
	if len(notifiers) != 1 {
		t.Fatalf("expected 1 notifier, got %d", len(notifiers))
	}
	if notifiers[0].MinSeverity != SeverityWarning {
		t.Errorf("expected default severity warning, got %s", notifiers[0].MinSeverity)
	}
}

func TestAddNotifier_PagerDuty(t *testing.T) {
	an := NewAlertNotifier()
	err := an.AddNotifier(NotifierConfig{
		ID:      "pd-1",
		Type:    NotifierPagerDuty,
		Name:    "PagerDuty",
		URL:     "https://events.pagerduty.com/v2/enqueue",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddNotifier_Webhook(t *testing.T) {
	an := NewAlertNotifier()
	err := an.AddNotifier(NotifierConfig{
		ID:      "wh-1",
		Type:    NotifierWebhook,
		Name:    "Custom Webhook",
		URL:     "https://example.com/webhook",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddNotifier_MissingID(t *testing.T) {
	an := NewAlertNotifier()
	err := an.AddNotifier(NotifierConfig{URL: "https://example.com"})
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
}

func TestAddNotifier_MissingURL(t *testing.T) {
	an := NewAlertNotifier()
	err := an.AddNotifier(NotifierConfig{ID: "test"})
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
}

func TestAddNotifier_DefaultSeverity(t *testing.T) {
	an := NewAlertNotifier()
	_ = an.AddNotifier(NotifierConfig{
		ID:  "test",
		URL: "https://example.com",
	})
	notifiers := an.ListNotifiers()
	if notifiers[0].MinSeverity != SeverityWarning {
		t.Errorf("expected default SeverityWarning, got %s", notifiers[0].MinSeverity)
	}
}

func TestRemoveNotifier_Existing(t *testing.T) {
	an := NewAlertNotifier()
	_ = an.AddNotifier(NotifierConfig{ID: "test", URL: "https://example.com"})
	err := an.RemoveNotifier("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(an.ListNotifiers()) != 0 {
		t.Error("expected 0 notifiers after removal")
	}
}

func TestRemoveNotifier_NonExistent(t *testing.T) {
	an := NewAlertNotifier()
	err := an.RemoveNotifier("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent notifier")
	}
}

func TestListNotifiers_Empty(t *testing.T) {
	an := NewAlertNotifier()
	notifiers := an.ListNotifiers()
	if len(notifiers) != 0 {
		t.Errorf("expected 0 notifiers, got %d", len(notifiers))
	}
}

func TestListNotifiers_Multiple(t *testing.T) {
	an := NewAlertNotifier()
	_ = an.AddNotifier(NotifierConfig{ID: "a", URL: "https://a.com"})
	_ = an.AddNotifier(NotifierConfig{ID: "b", URL: "https://b.com"})
	if len(an.ListNotifiers()) != 2 {
		t.Errorf("expected 2 notifiers")
	}
}

func TestNotify_SeverityFiltering(t *testing.T) {
	an := NewAlertNotifier()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_ = an.AddNotifier(NotifierConfig{
		ID:          "high-only",
		Type:        NotifierWebhook,
		URL:         server.URL,
		Enabled:     true,
		MinSeverity: SeverityCritical,
	})

	// Info alert should NOT trigger critical-only notifier
	results := an.Notify(Alert{
		ID:       "alert-1",
		Name:     "Low Alert",
		Severity: SeverityInfo,
		Message:  "info message",
		Source:   "test",
		FiredAt:  time.Now(),
	})
	if len(results) != 0 {
		t.Errorf("expected 0 results for info alert on critical notifier, got %d", len(results))
	}
}

func TestNotify_DisabledNotifierSkipped(t *testing.T) {
	an := NewAlertNotifier()
	_ = an.AddNotifier(NotifierConfig{
		ID:      "disabled",
		Type:    NotifierWebhook,
		URL:     "https://example.com",
		Enabled: false,
	})

	results := an.Notify(Alert{
		ID:       "alert-1",
		Severity: SeverityCritical,
		FiredAt:  time.Now(),
	})
	if len(results) != 0 {
		t.Error("disabled notifier should be skipped")
	}
}

func TestNotify_HistoryRecording(t *testing.T) {
	an := NewAlertNotifier()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_ = an.AddNotifier(NotifierConfig{
		ID:      "test",
		Type:    NotifierWebhook,
		URL:     server.URL,
		Enabled: true,
	})

	an.Notify(Alert{
		ID:       "a1",
		Severity: SeverityCritical,
		FiredAt:  time.Now(),
	})

	history := an.GetHistory(10)
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
	if !history[0].Success {
		t.Errorf("expected success, got error: %s", history[0].Error)
	}
}

func TestNotify_HTTPDispatch(t *testing.T) {
	var received bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	an := NewAlertNotifier()
	_ = an.AddNotifier(NotifierConfig{
		ID:      "webhook",
		Type:    NotifierWebhook,
		URL:     server.URL,
		Enabled: true,
	})

	results := an.Notify(Alert{
		ID:       "a1",
		Name:     "Test",
		Severity: SeverityWarning,
		Message:  "test message",
		Source:   "test",
		FiredAt:  time.Now(),
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Errorf("dispatch failed: %s", results[0].Error)
	}
	if !received {
		t.Error("server did not receive request")
	}
}

func TestBuildSlackPayload(t *testing.T) {
	alert := Alert{
		ID:       "a1",
		Name:     "Test Alert",
		Severity: SeverityWarning,
		Message:  "something happened",
		Source:   "test",
		FiredAt:  time.Now(),
	}

	payload, err := buildSlackPayload(alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := parsed["text"]; !ok {
		t.Error("expected 'text' field in Slack payload")
	}
	if _, ok := parsed["blocks"]; !ok {
		t.Error("expected 'blocks' field in Slack payload")
	}
}

func TestBuildPagerDutyPayload(t *testing.T) {
	alert := Alert{
		ID:       "a1",
		Name:     "PD Alert",
		Severity: SeverityCritical,
		Message:  "critical issue",
		Source:   "system",
		FiredAt:  time.Now(),
		Labels:   map[string]string{"env": "prod"},
	}

	payload, err := buildPagerDutyPayload(alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := parsed["routing_key"]; !ok {
		t.Error("expected 'routing_key' in PagerDuty payload")
	}
	if _, ok := parsed["dedup_key"]; !ok {
		t.Error("expected 'dedup_key' in PagerDuty payload")
	}
	p, _ := parsed["payload"].(map[string]interface{})
	if p["severity"] != "critical" {
		t.Errorf("expected critical severity mapping, got %v", p["severity"])
	}
}

func TestGetHistory_Ordering(t *testing.T) {
	an := NewAlertNotifier()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_ = an.AddNotifier(NotifierConfig{
		ID: "test", Type: NotifierWebhook, URL: server.URL, Enabled: true,
	})

	for i := 0; i < 5; i++ {
		an.Notify(Alert{
			ID: "a" + string(rune('0'+i)), Severity: SeverityWarning, FiredAt: time.Now(),
		})
	}

	history := an.GetHistory(3)
	if len(history) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(history))
	}
}

func TestGetHistory_MaxCap(t *testing.T) {
	an := NewAlertNotifier()
	an.maxHistory = 5

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_ = an.AddNotifier(NotifierConfig{
		ID: "test", Type: NotifierWebhook, URL: server.URL, Enabled: true,
	})

	for i := 0; i < 10; i++ {
		an.Notify(Alert{
			ID: "a" + string(rune('0'+i)), Severity: SeverityWarning, FiredAt: time.Now(),
		})
	}

	history := an.GetHistory(100)
	if len(history) > 5 {
		t.Errorf("expected max 5 history entries, got %d", len(history))
	}
}

func TestDashboard_AddNotifier_Integration(t *testing.T) {
	dashboard := NewDashboard(DashboardConfig{MaxAlerts: 100})

	err := dashboard.AddNotifier(NotifierConfig{
		ID:   "n1",
		Type: NotifierWebhook,
		URL:  "https://example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	notifier := dashboard.GetNotifier()
	if notifier == nil {
		t.Fatal("expected non-nil notifier")
	}
	if len(notifier.ListNotifiers()) != 1 {
		t.Error("expected 1 notifier in dashboard")
	}
}
