package timetravel

import (
	"testing"
	"time"
)

func TestDebugger_CreateSession(t *testing.T) {
	d := NewDebugger(DefaultDebuggerConfig())
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()

	session, err := d.CreateSession("s1", "user:123", []string{"age", "score"}, start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.ID != "s1" {
		t.Errorf("expected ID s1, got %s", session.ID)
	}
	if session.EntityKey != "user:123" {
		t.Errorf("expected entity user:123, got %s", session.EntityKey)
	}
	if session.Status != "active" {
		t.Errorf("expected status active, got %s", session.Status)
	}
	if len(session.Features) != 2 {
		t.Errorf("expected 2 features, got %d", len(session.Features))
	}
}

func TestDebugger_CreateSession_InvalidRange(t *testing.T) {
	d := NewDebugger(DefaultDebuggerConfig())
	start := time.Now()
	end := start.Add(-1 * time.Hour) // end before start

	_, err := d.CreateSession("s1", "user:123", []string{"age"}, start, end)
	if err != ErrInvalidRange {
		t.Fatalf("expected ErrInvalidRange, got %v", err)
	}
}

func TestDebugger_AddSnapshot(t *testing.T) {
	d := NewDebugger(DefaultDebuggerConfig())
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()

	_, err := d.CreateSession("s1", "user:123", []string{"score"}, start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap := &Snapshot{
		Timestamp: time.Now(),
		Values:    map[string]interface{}{"score": 42.0},
	}
	if err := d.AddSnapshot("s1", snap); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	session, _ := d.GetSession("s1")
	if len(session.Snapshots) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(session.Snapshots))
	}
}

func TestDebugger_Replay(t *testing.T) {
	d := NewDebugger(DefaultDebuggerConfig())
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()

	_, err := d.CreateSession("s1", "user:123", []string{"score"}, start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Add snapshots out of order
	t3 := time.Now()
	t1 := t3.Add(-2 * time.Hour)
	t2 := t3.Add(-1 * time.Hour)

	d.AddSnapshot("s1", &Snapshot{Timestamp: t3, Values: map[string]interface{}{"score": 30.0}})
	d.AddSnapshot("s1", &Snapshot{Timestamp: t1, Values: map[string]interface{}{"score": 10.0}})
	d.AddSnapshot("s1", &Snapshot{Timestamp: t2, Values: map[string]interface{}{"score": 20.0}})

	result, err := d.Replay("s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PointCount != 3 {
		t.Errorf("expected 3 points, got %d", result.PointCount)
	}

	// Verify sorted order
	for i := 1; i < len(result.Timeline); i++ {
		if result.Timeline[i].Timestamp.Before(result.Timeline[i-1].Timestamp) {
			t.Error("timeline not sorted by timestamp")
		}
	}

	// Verify first value is the earliest
	if result.Timeline[0].Values["score"] != 10.0 {
		t.Errorf("expected first score 10.0, got %v", result.Timeline[0].Values["score"])
	}
}

func TestDebugger_Compare(t *testing.T) {
	d := NewDebugger(DefaultDebuggerConfig())

	now := time.Now()
	windowA := TimeWindow{Start: now.Add(-2 * time.Hour), End: now.Add(-1 * time.Hour)}
	windowB := TimeWindow{Start: now.Add(-1 * time.Hour), End: now}

	snapshots := []*Snapshot{
		{Timestamp: now.Add(-90 * time.Minute), Values: map[string]interface{}{"score": 10.0, "count": 5.0}},
		{Timestamp: now.Add(-80 * time.Minute), Values: map[string]interface{}{"score": 12.0, "count": 6.0}},
		{Timestamp: now.Add(-30 * time.Minute), Values: map[string]interface{}{"score": 20.0, "count": 5.0}},
		{Timestamp: now.Add(-20 * time.Minute), Values: map[string]interface{}{"score": 22.0, "count": 7.0}},
	}

	comparison, err := d.Compare("user:123", windowA, windowB, snapshots)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if comparison.EntityKey != "user:123" {
		t.Errorf("expected entity user:123, got %s", comparison.EntityKey)
	}

	// Verify diffs exist
	if len(comparison.Diffs) == 0 {
		t.Error("expected diffs, got none")
	}

	// Find score diff - should be increased (avg 11 -> avg 21)
	for _, diff := range comparison.Diffs {
		if diff.FeatureName == "score" {
			if diff.ChangeType != "increased" {
				t.Errorf("expected score change type 'increased', got '%s'", diff.ChangeType)
			}
			if diff.ChangePct <= 0 {
				t.Errorf("expected positive change pct, got %f", diff.ChangePct)
			}
		}
	}

	if comparison.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestDebugger_DetectAnomalies(t *testing.T) {
	d := NewDebugger(DefaultDebuggerConfig())

	now := time.Now()
	snapshots := make([]*Snapshot, 0, 21)

	// Add 20 normal values around 10.0
	for i := 0; i < 20; i++ {
		snapshots = append(snapshots, &Snapshot{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Values:    map[string]interface{}{"score": 10.0},
		})
	}

	// Add one extreme outlier
	snapshots = append(snapshots, &Snapshot{
		Timestamp: now.Add(20 * time.Minute),
		Values:    map[string]interface{}{"score": 1000.0},
	})

	anomalies := d.DetectAnomalies(snapshots, "score")
	if len(anomalies) == 0 {
		t.Error("expected at least one anomaly, got none")
	}

	found := false
	for _, a := range anomalies {
		if a.Value == 1000.0 {
			found = true
			if a.FeatureName != "score" {
				t.Errorf("expected feature name 'score', got '%s'", a.FeatureName)
			}
		}
	}
	if !found {
		t.Error("expected anomaly with value 1000.0")
	}
}

func TestDebugger_CloseSession(t *testing.T) {
	d := NewDebugger(DefaultDebuggerConfig())
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()

	d.CreateSession("s1", "user:123", []string{"score"}, start, end)

	if err := d.CloseSession("s1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	session, _ := d.GetSession("s1")
	if session.Status != "completed" {
		t.Errorf("expected status completed, got %s", session.Status)
	}

	// Close non-existent session
	if err := d.CloseSession("nonexistent"); err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestDebugger_MaxSessions(t *testing.T) {
	config := DefaultDebuggerConfig()
	config.MaxSessions = 2
	d := NewDebugger(config)

	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()

	d.CreateSession("s1", "user:1", []string{"a"}, start, end)
	d.CreateSession("s2", "user:2", []string{"b"}, start, end)

	_, err := d.CreateSession("s3", "user:3", []string{"c"}, start, end)
	if err == nil {
		t.Error("expected error when exceeding max sessions")
	}
}
