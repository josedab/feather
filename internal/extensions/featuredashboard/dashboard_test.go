package featuredashboard

import (
	"testing"
)

func TestNewDashboard(t *testing.T) {
	d := NewDashboard(DefaultDashboardConfig())
	if d == nil {
		t.Fatal("expected non-nil dashboard")
	}
}

func TestTrackAndRecordLatency(t *testing.T) {
	d := NewDashboard(DefaultDashboardConfig())
	d.TrackFeature("user_age", nil)

	for i := 0; i < 100; i++ {
		d.RecordLatency("user_age", float64(i)*0.1)
	}

	health, err := d.GetFeatureHealth("user_age")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if health.Latency.Avg <= 0 {
		t.Error("expected positive average latency")
	}
	if health.Status == StatusUnknown {
		t.Error("expected known status after recording data")
	}
}

func TestSnapshot(t *testing.T) {
	d := NewDashboard(DefaultDashboardConfig())
	d.TrackFeature("f1", []string{"f2"})
	d.TrackFeature("f2", nil)

	d.RecordLatency("f1", 1.0)
	d.RecordLatency("f2", 2.0)

	snapshot := d.TakeSnapshot()
	if snapshot.TotalFeatures != 2 {
		t.Errorf("expected 2 features, got %d", snapshot.TotalFeatures)
	}
	if len(snapshot.Features) != 2 {
		t.Errorf("expected 2 feature health entries, got %d", len(snapshot.Features))
	}
}

func TestDriftScoreAffectsHealth(t *testing.T) {
	d := NewDashboard(DefaultDashboardConfig())
	d.TrackFeature("drifted", nil)
	d.RecordLatency("drifted", 1.0)
	d.UpdateDriftScore("drifted", 0.5)

	health, _ := d.GetFeatureHealth("drifted")
	if health.Status != StatusUnhealthy {
		t.Errorf("expected unhealthy status for high drift, got %s", health.Status)
	}
}

func TestFeatureNotTracked(t *testing.T) {
	d := NewDashboard(DefaultDashboardConfig())
	_, err := d.GetFeatureHealth("nonexistent")
	if err != ErrFeatureNotTracked {
		t.Errorf("expected ErrFeatureNotTracked, got %v", err)
	}
}
