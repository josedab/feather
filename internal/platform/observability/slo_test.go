package observability

import (
	"context"
	"testing"
	"time"
)

func TestSLOTracker_RegisterAndStatus(t *testing.T) {
	tracker := NewSLOTracker()

	def := &SLODefinition{
		Name:      "test_availability",
		Type:      SLOTypeAvailability,
		Target:    99.9,
		Window:    24 * time.Hour,
		Component: "test",
	}

	tracker.RegisterSLO(def)

	// Record some operations
	for i := 0; i < 1000; i++ {
		tracker.RecordSuccess("test_availability", 5.0)
	}

	status := tracker.GetStatus("test_availability")
	if status == nil {
		t.Fatal("expected status, got nil")
	}

	if status.CurrentValue != 100.0 {
		t.Errorf("expected 100%% availability, got %.2f%%", status.CurrentValue)
	}

	if !status.InCompliance {
		t.Error("expected SLO to be in compliance")
	}
}

func TestSLOTracker_AvailabilitySLO(t *testing.T) {
	tracker := NewSLOTracker()

	def := &SLODefinition{
		Name:      "availability_test",
		Type:      SLOTypeAvailability,
		Target:    99.0,
		Window:    time.Hour,
		Component: "test",
	}

	tracker.RegisterSLO(def)

	// Record 99 successes and 1 failure = 99% availability
	for i := 0; i < 99; i++ {
		tracker.RecordSuccess("availability_test", 5.0)
	}
	tracker.RecordFailure("availability_test")

	status := tracker.GetStatus("availability_test")
	if status == nil {
		t.Fatal("expected status, got nil")
	}

	if status.CurrentValue != 99.0 {
		t.Errorf("expected 99%% availability, got %.2f%%", status.CurrentValue)
	}

	if !status.InCompliance {
		t.Error("expected SLO to be in compliance at exactly 99%")
	}
}

func TestSLOTracker_AvailabilitySLOBreach(t *testing.T) {
	tracker := NewSLOTracker()

	def := &SLODefinition{
		Name:      "breach_test",
		Type:      SLOTypeAvailability,
		Target:    99.9,
		Window:    time.Hour,
		Component: "test",
	}

	tracker.RegisterSLO(def)

	// Record 98 successes and 2 failures = 98% availability (breach)
	for i := 0; i < 98; i++ {
		tracker.RecordSuccess("breach_test", 5.0)
	}
	tracker.RecordFailure("breach_test")
	tracker.RecordFailure("breach_test")

	status := tracker.GetStatus("breach_test")
	if status == nil {
		t.Fatal("expected status, got nil")
	}

	if status.InCompliance {
		t.Error("expected SLO to be breached")
	}

	if status.CurrentValue != 98.0 {
		t.Errorf("expected 98%% availability, got %.2f%%", status.CurrentValue)
	}
}

func TestSLOTracker_LatencySLO(t *testing.T) {
	tracker := NewSLOTracker()

	def := &SLODefinition{
		Name:      "latency_test",
		Type:      SLOTypeLatency,
		Target:    99.0,
		Threshold: 10, // 10ms
		Window:    time.Hour,
		Component: "test",
	}

	tracker.RegisterSLO(def)

	// Record 99 fast requests and 1 slow request
	for i := 0; i < 99; i++ {
		tracker.RecordSuccess("latency_test", 5.0) // 5ms, under threshold
	}
	tracker.RecordSuccess("latency_test", 15.0) // 15ms, over threshold

	status := tracker.GetStatus("latency_test")
	if status == nil {
		t.Fatal("expected status, got nil")
	}

	if status.CurrentValue != 99.0 {
		t.Errorf("expected 99%% within threshold, got %.2f%%", status.CurrentValue)
	}

	if !status.InCompliance {
		t.Error("expected SLO to be in compliance")
	}
}

func TestSLOTracker_ErrorRateSLO(t *testing.T) {
	tracker := NewSLOTracker()

	def := &SLODefinition{
		Name:      "error_rate_test",
		Type:      SLOTypeErrorRate,
		Target:    99.0, // 99% success rate (1% error rate)
		Window:    time.Hour,
		Component: "test",
	}

	tracker.RegisterSLO(def)

	// Record 99 successes and 1 failure
	for i := 0; i < 99; i++ {
		tracker.RecordSuccess("error_rate_test", 5.0)
	}
	tracker.RecordFailure("error_rate_test")

	status := tracker.GetStatus("error_rate_test")
	if status == nil {
		t.Fatal("expected status, got nil")
	}

	if status.CurrentValue != 99.0 {
		t.Errorf("expected 99%% success rate, got %.2f%%", status.CurrentValue)
	}

	if !status.InCompliance {
		t.Error("expected SLO to be in compliance")
	}
}

func TestSLOTracker_GetAllStatus(t *testing.T) {
	tracker := NewSLOTracker()

	defs := DefaultSLOs()
	for _, def := range defs {
		tracker.RegisterSLO(def)
	}

	statuses := tracker.GetAllStatus()

	if len(statuses) != len(defs) {
		t.Errorf("expected %d statuses, got %d", len(defs), len(statuses))
	}
}

func TestSLOTracker_CheckBreaches(t *testing.T) {
	tracker := NewSLOTracker()

	def := &SLODefinition{
		Name:      "breach_check_test",
		Type:      SLOTypeAvailability,
		Target:    99.9,
		Window:    time.Hour,
		Component: "test",
	}

	tracker.RegisterSLO(def)

	// Create a breach condition
	for i := 0; i < 90; i++ {
		tracker.RecordSuccess("breach_check_test", 5.0)
	}
	for i := 0; i < 10; i++ {
		tracker.RecordFailure("breach_check_test")
	}

	breaches := tracker.CheckBreaches(context.Background())

	if len(breaches) == 0 {
		t.Error("expected at least one breach")
	}

	if len(breaches) > 0 {
		if breaches[0].SLOName != "breach_check_test" {
			t.Errorf("expected breach for 'breach_check_test', got '%s'", breaches[0].SLOName)
		}
	}
}

func TestSLOTracker_GetBreaches(t *testing.T) {
	tracker := NewSLOTracker()

	def := &SLODefinition{
		Name:      "get_breaches_test",
		Type:      SLOTypeAvailability,
		Target:    99.9,
		Window:    time.Hour,
		Component: "test",
	}

	tracker.RegisterSLO(def)

	// Create a breach
	for i := 0; i < 10; i++ {
		tracker.RecordFailure("get_breaches_test")
	}

	// Check breaches to record them
	tracker.CheckBreaches(context.Background())

	// Get breaches from the past hour
	since := time.Now().Add(-time.Hour)
	breaches := tracker.GetBreaches(since)

	if len(breaches) == 0 {
		t.Error("expected at least one breach")
	}

	// Get breaches from the future (should be none)
	future := time.Now().Add(time.Hour)
	noneBreaches := tracker.GetBreaches(future)

	if len(noneBreaches) != 0 {
		t.Errorf("expected no breaches from future, got %d", len(noneBreaches))
	}
}

func TestSLOTracker_ErrorBudget(t *testing.T) {
	tracker := NewSLOTracker()

	def := &SLODefinition{
		Name:      "budget_test",
		Type:      SLOTypeAvailability,
		Target:    99.0, // 1% error budget
		Window:    time.Hour,
		Component: "test",
	}

	tracker.RegisterSLO(def)

	// 100% availability = 100% error budget remaining
	for i := 0; i < 100; i++ {
		tracker.RecordSuccess("budget_test", 5.0)
	}

	status := tracker.GetStatus("budget_test")
	if status.ErrorBudget != 100.0 {
		t.Errorf("expected 100%% error budget, got %.2f%%", status.ErrorBudget)
	}

	// Now add some failures
	for i := 0; i < 2; i++ {
		tracker.RecordFailure("budget_test")
	}

	status = tracker.GetStatus("budget_test")
	// With 2 failures out of 102, we have 98.04% availability
	// Error budget should be reduced
	if status.ErrorBudget >= 100.0 {
		t.Errorf("expected error budget to be reduced, got %.2f%%", status.ErrorBudget)
	}
}

func TestDefaultSLOs(t *testing.T) {
	slos := DefaultSLOs()

	if len(slos) == 0 {
		t.Error("expected default SLOs to be defined")
	}

	// Verify each SLO has required fields
	for _, slo := range slos {
		if slo.Name == "" {
			t.Error("SLO name should not be empty")
		}
		if slo.Target <= 0 || slo.Target > 100 {
			t.Errorf("SLO target should be between 0 and 100, got %.2f", slo.Target)
		}
		if slo.Window <= 0 {
			t.Error("SLO window should be positive")
		}
	}
}

func TestCalculateErrorBudget(t *testing.T) {
	tests := []struct {
		name    string
		current float64
		target  float64
		want    float64
	}{
		{"100% at 99% target", 100.0, 99.0, 100.0},
		{"99% at 99% target", 99.0, 99.0, 0.0},
		{"99.5% at 99% target", 99.5, 99.0, 50.0},
		{"98% at 99% target (over budget)", 98.0, 99.0, 0.0},
		{"100% at 100% target", 100.0, 100.0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateErrorBudget(tt.current, tt.target)
			if got != tt.want {
				t.Errorf("calculateErrorBudget(%.2f, %.2f) = %.2f, want %.2f",
					tt.current, tt.target, got, tt.want)
			}
		})
	}
}
