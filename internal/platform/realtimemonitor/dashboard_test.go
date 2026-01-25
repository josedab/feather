package realtimemonitor

import (
	"testing"
	"time"
)

func TestNewDashboard(t *testing.T) {
	d := NewDashboard(DefaultDashboardConfig())
	if d == nil {
		t.Fatal("expected non-nil dashboard")
	}
}

func TestRecordFreshness(t *testing.T) {
	d := NewDashboard(DefaultDashboardConfig())
	d.RecordFreshness("user_age", "user_features", time.Now())
	d.RecordFreshness("stale_feature", "old_group", time.Now().Add(-10*time.Minute))

	snap := d.Snapshot()
	if snap.Summary.TotalFeatures != 2 {
		t.Errorf("expected 2 features, got %d", snap.Summary.TotalFeatures)
	}
	if snap.Summary.StaleFeatures != 1 {
		t.Errorf("expected 1 stale feature, got %d", snap.Summary.StaleFeatures)
	}
}

func TestRecordLatency(t *testing.T) {
	d := NewDashboard(DefaultDashboardConfig())
	for i := 0; i < 100; i++ {
		d.RecordLatency("/v1/features", time.Duration(i)*time.Millisecond, i > 95)
	}

	snap := d.Snapshot()
	if len(snap.Latency) != 1 {
		t.Fatalf("expected 1 latency metric, got %d", len(snap.Latency))
	}
	if snap.Latency[0].Count != 100 {
		t.Errorf("expected 100 requests, got %d", snap.Latency[0].Count)
	}
	if snap.Latency[0].P50 == 0 {
		t.Error("expected non-zero p50")
	}
}

func TestFireAndResolveAlert(t *testing.T) {
	d := NewDashboard(DefaultDashboardConfig())
	alert := d.FireAlert("high_latency", SeverityWarning, "p99 > 100ms", "/v1/features", nil)
	if alert.ID == "" {
		t.Error("expected alert ID")
	}

	alerts := d.GetAlerts("active")
	if len(alerts) != 1 {
		t.Fatalf("expected 1 active alert, got %d", len(alerts))
	}

	if err := d.ResolveAlert(alert.ID); err != nil {
		t.Fatal(err)
	}

	alerts = d.GetAlerts("active")
	if len(alerts) != 0 {
		t.Errorf("expected 0 active alerts, got %d", len(alerts))
	}
}

func TestPipelineHealth(t *testing.T) {
	d := NewDashboard(DefaultDashboardConfig())
	d.UpdatePipelineHealth(PipelineHealth{
		PipelineID: "p1", Status: HealthHealthy, EventsPerSec: 1000,
	})
	d.UpdatePipelineHealth(PipelineHealth{
		PipelineID: "p2", Status: HealthDegraded, EventsPerSec: 50, Lag: 5000,
	})

	snap := d.Snapshot()
	if snap.Summary.TotalPipelines != 2 {
		t.Errorf("expected 2 pipelines, got %d", snap.Summary.TotalPipelines)
	}
	if snap.Summary.HealthyPipelines != 1 {
		t.Errorf("expected 1 healthy pipeline, got %d", snap.Summary.HealthyPipelines)
	}
}

func TestOverallHealth(t *testing.T) {
	d := NewDashboard(DefaultDashboardConfig())

	// Healthy by default
	snap := d.Snapshot()
	if snap.OverallHealth != HealthHealthy {
		t.Errorf("expected healthy, got %s", snap.OverallHealth)
	}

	// Degraded with warning
	d.FireAlert("test", SeverityWarning, "test", "test", nil)
	snap = d.Snapshot()
	if snap.OverallHealth != HealthDegraded {
		t.Errorf("expected degraded, got %s", snap.OverallHealth)
	}

	// Unhealthy with critical
	d.FireAlert("critical", SeverityCritical, "critical", "test", nil)
	snap = d.Snapshot()
	if snap.OverallHealth != HealthUnhealthy {
		t.Errorf("expected unhealthy, got %s", snap.OverallHealth)
	}
}
