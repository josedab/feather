package anomalydetect

import (
	"testing"
	"time"
)

func TestRemediationEngine_AddRemovePolicy(t *testing.T) {
	engine := NewRemediationEngine()

	policy := RemediationPolicy{
		Name:    "test-policy",
		Feature: "latency",
		Actions: []RemediationAction{ActionCacheFlush},
		Enabled: true,
	}

	if err := engine.AddPolicy(policy); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	policies := engine.ListPolicies()
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
	if policies[0].Name != "test-policy" {
		t.Errorf("expected policy name 'test-policy', got %q", policies[0].Name)
	}

	if err := engine.RemovePolicy("test-policy"); err != nil {
		t.Fatalf("unexpected error removing policy: %v", err)
	}
	if len(engine.ListPolicies()) != 0 {
		t.Error("expected 0 policies after removal")
	}
}

func TestRemediationEngine_AddPolicy_Validation(t *testing.T) {
	engine := NewRemediationEngine()

	tests := []struct {
		name   string
		policy RemediationPolicy
	}{
		{"empty name", RemediationPolicy{Feature: "f", Actions: []RemediationAction{ActionNoOp}}},
		{"empty feature", RemediationPolicy{Name: "p", Actions: []RemediationAction{ActionNoOp}}},
		{"no actions", RemediationPolicy{Name: "p", Feature: "f"}},
	}

	for _, tt := range tests {
		if err := engine.AddPolicy(tt.policy); err == nil {
			t.Errorf("%s: expected error, got nil", tt.name)
		}
	}
}

func TestRemediationEngine_RemovePolicy_NotFound(t *testing.T) {
	engine := NewRemediationEngine()
	if err := engine.RemovePolicy("nonexistent"); err == nil {
		t.Error("expected error removing nonexistent policy")
	}
}

func TestRemediationEngine_Evaluate_MatchingPolicy(t *testing.T) {
	engine := NewRemediationEngine()

	_ = engine.AddPolicy(RemediationPolicy{
		Name:    "latency-alert",
		Feature: "latency",
		Condition: PolicyCondition{
			MinScore: 1.0,
		},
		Actions: []RemediationAction{ActionAlertEscalate},
		Enabled: true,
	})

	anomaly := AnomalyResult{
		Feature:   "latency",
		IsAnomaly: true,
		Score:     5.0,
		Type:      AnomalyZScore,
		Timestamp: time.Now(),
	}

	events := engine.Evaluate(anomaly)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].PolicyName != "latency-alert" {
		t.Errorf("expected policy 'latency-alert', got %q", events[0].PolicyName)
	}
	if !events[0].Success {
		t.Error("expected event to be successful")
	}
}

func TestRemediationEngine_Evaluate_NonMatchingPolicy(t *testing.T) {
	engine := NewRemediationEngine()

	_ = engine.AddPolicy(RemediationPolicy{
		Name:    "other-policy",
		Feature: "throughput",
		Condition: PolicyCondition{
			MinScore: 1.0,
		},
		Actions: []RemediationAction{ActionCacheFlush},
		Enabled: true,
	})

	anomaly := AnomalyResult{
		Feature:   "latency",
		IsAnomaly: true,
		Score:     5.0,
		Timestamp: time.Now(),
	}

	events := engine.Evaluate(anomaly)
	if len(events) != 0 {
		t.Errorf("expected 0 events for non-matching feature, got %d", len(events))
	}
}

func TestRemediationEngine_Evaluate_NonAnomaly(t *testing.T) {
	engine := NewRemediationEngine()

	_ = engine.AddPolicy(RemediationPolicy{
		Name:    "p",
		Feature: "latency",
		Actions: []RemediationAction{ActionNoOp},
		Enabled: true,
	})

	result := AnomalyResult{
		Feature:   "latency",
		IsAnomaly: false,
		Timestamp: time.Now(),
	}

	events := engine.Evaluate(result)
	if events != nil {
		t.Error("expected nil events for non-anomaly")
	}
}

func TestRemediationEngine_CooldownPeriod(t *testing.T) {
	engine := NewRemediationEngine()

	_ = engine.AddPolicy(RemediationPolicy{
		Name:           "cooldown-test",
		Feature:        "latency",
		Condition:      PolicyCondition{MinScore: 1.0},
		Actions:        []RemediationAction{ActionCacheFlush},
		CooldownPeriod: 1 * time.Hour,
		Enabled:        true,
	})

	anomaly := AnomalyResult{
		Feature:   "latency",
		IsAnomaly: true,
		Score:     5.0,
		Timestamp: time.Now(),
	}

	// First evaluation should trigger
	events := engine.Evaluate(anomaly)
	if len(events) != 1 {
		t.Fatalf("expected 1 event on first evaluation, got %d", len(events))
	}

	// Second evaluation should be blocked by cooldown
	events = engine.Evaluate(anomaly)
	if len(events) != 0 {
		t.Errorf("expected 0 events during cooldown, got %d", len(events))
	}
}

func TestRemediationEngine_ConsecutiveCount(t *testing.T) {
	engine := NewRemediationEngine()

	_ = engine.AddPolicy(RemediationPolicy{
		Name:    "consecutive-test",
		Feature: "latency",
		Condition: PolicyCondition{
			MinScore:         1.0,
			ConsecutiveCount: 3,
		},
		Actions: []RemediationAction{ActionCircuitBreak},
		Enabled: true,
	})

	anomaly := AnomalyResult{
		Feature:   "latency",
		IsAnomaly: true,
		Score:     5.0,
		Timestamp: time.Now(),
	}

	// First two should not trigger
	for i := 0; i < 2; i++ {
		events := engine.Evaluate(anomaly)
		if len(events) != 0 {
			t.Errorf("iteration %d: expected 0 events before consecutive threshold, got %d", i, len(events))
		}
	}

	// Third should trigger
	events := engine.Evaluate(anomaly)
	if len(events) != 1 {
		t.Fatalf("expected 1 event at consecutive threshold, got %d", len(events))
	}
}

func TestRemediationEngine_WildcardFeature(t *testing.T) {
	engine := NewRemediationEngine()

	_ = engine.AddPolicy(RemediationPolicy{
		Name:      "wildcard",
		Feature:   "*",
		Condition: PolicyCondition{MinScore: 1.0},
		Actions:   []RemediationAction{ActionAlertEscalate},
		Enabled:   true,
	})

	anomaly := AnomalyResult{
		Feature:   "any-feature",
		IsAnomaly: true,
		Score:     5.0,
		Timestamp: time.Now(),
	}

	events := engine.Evaluate(anomaly)
	if len(events) != 1 {
		t.Fatalf("expected wildcard policy to match, got %d events", len(events))
	}
}

func TestRemediationEngine_Stats(t *testing.T) {
	engine := NewRemediationEngine()

	_ = engine.AddPolicy(RemediationPolicy{
		Name:      "stats-test",
		Feature:   "latency",
		Condition: PolicyCondition{MinScore: 1.0},
		Actions:   []RemediationAction{ActionCacheFlush, ActionAlertEscalate},
		Enabled:   true,
	})

	anomaly := AnomalyResult{
		Feature:   "latency",
		IsAnomaly: true,
		Score:     5.0,
		Timestamp: time.Now(),
	}

	engine.Evaluate(anomaly)

	stats := engine.Stats()
	if stats.TotalPolicies != 1 {
		t.Errorf("expected 1 policy, got %d", stats.TotalPolicies)
	}
	if stats.TotalEvents != 1 {
		t.Errorf("expected 1 total event, got %d", stats.TotalEvents)
	}
	if stats.SuccessfulEvents != 1 {
		t.Errorf("expected 1 successful event, got %d", stats.SuccessfulEvents)
	}
	if stats.ActionCounts[ActionCacheFlush] != 1 {
		t.Errorf("expected 1 cache_flush action, got %d", stats.ActionCounts[ActionCacheFlush])
	}
	if stats.ActionCounts[ActionAlertEscalate] != 1 {
		t.Errorf("expected 1 alert_escalation action, got %d", stats.ActionCounts[ActionAlertEscalate])
	}
}

func TestRemediationEngine_GetEvents(t *testing.T) {
	engine := NewRemediationEngine()

	_ = engine.AddPolicy(RemediationPolicy{
		Name:      "events-test",
		Feature:   "*",
		Condition: PolicyCondition{MinScore: 0},
		Actions:   []RemediationAction{ActionNoOp},
		Enabled:   true,
	})

	for _, f := range []string{"a", "b", "a"} {
		engine.Evaluate(AnomalyResult{
			Feature:   f,
			IsAnomaly: true,
			Score:     1.0,
			Timestamp: time.Now(),
		})
	}

	all := engine.GetEvents("", 0)
	if len(all) != 3 {
		t.Errorf("expected 3 events, got %d", len(all))
	}

	filtered := engine.GetEvents("a", 0)
	if len(filtered) != 2 {
		t.Errorf("expected 2 events for feature 'a', got %d", len(filtered))
	}

	limited := engine.GetEvents("", 1)
	if len(limited) != 1 {
		t.Errorf("expected 1 event with limit=1, got %d", len(limited))
	}
}

func TestRemediationEngine_DisabledPolicy(t *testing.T) {
	engine := NewRemediationEngine()

	_ = engine.AddPolicy(RemediationPolicy{
		Name:    "disabled",
		Feature: "latency",
		Actions: []RemediationAction{ActionNoOp},
		Enabled: false,
	})

	events := engine.Evaluate(AnomalyResult{
		Feature:   "latency",
		IsAnomaly: true,
		Score:     5.0,
		Timestamp: time.Now(),
	})

	if len(events) != 0 {
		t.Errorf("expected 0 events for disabled policy, got %d", len(events))
	}
}
