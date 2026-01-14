package parity

import (
	"testing"
	"time"
)

func TestChecker_RecordPair_Match(t *testing.T) {
	c := NewChecker(DefaultConfig())

	c.RecordPair("feature1", "user:1", 10.0, 10.0)

	status := c.GetStatus("feature1")
	if status == nil {
		t.Fatal("expected status")
	}
	if status.TotalPairs != 1 {
		t.Errorf("expected 1 pair, got %d", status.TotalPairs)
	}
	if status.MatchRate != 1.0 {
		t.Errorf("expected match rate 1.0, got %f", status.MatchRate)
	}
}

func TestChecker_RecordPair_Mismatch(t *testing.T) {
	c := NewChecker(Config{
		MaxSamples:        1000,
		AbsoluteTolerance: 0.001,
		RelativeTolerance: 0.001,
		AlertThreshold:    0.05,
	})

	c.RecordPair("feature1", "user:1", 10.0, 20.0)

	status := c.GetStatus("feature1")
	if status.MismatchCount != 1 {
		t.Errorf("expected 1 mismatch, got %d", status.MismatchCount)
	}
	if status.MaxAbsDiff != 10.0 {
		t.Errorf("expected max abs diff 10.0, got %f", status.MaxAbsDiff)
	}
}

func TestChecker_WithinTolerance(t *testing.T) {
	c := NewChecker(Config{
		MaxSamples:        1000,
		AbsoluteTolerance: 0.01,
		RelativeTolerance: 0.05,
		AlertThreshold:    0.1,
	})

	// Should match within absolute tolerance
	c.RecordPair("f1", "e1", 100.0, 100.005)
	status := c.GetStatus("f1")
	if status.MatchCount != 1 {
		t.Errorf("expected match within absolute tolerance")
	}
}

func TestChecker_StringValues(t *testing.T) {
	c := NewChecker(DefaultConfig())

	c.RecordPair("category", "item:1", "electronics", "electronics")
	c.RecordPair("category", "item:2", "books", "electronics")

	status := c.GetStatus("category")
	if status.MatchCount != 1 {
		t.Errorf("expected 1 match, got %d", status.MatchCount)
	}
	if status.MismatchCount != 1 {
		t.Errorf("expected 1 mismatch, got %d", status.MismatchCount)
	}
}

func TestChecker_GetAllStatuses(t *testing.T) {
	c := NewChecker(DefaultConfig())

	c.RecordPair("f1", "e1", 1.0, 1.0)
	c.RecordPair("f2", "e1", 2.0, 3.0)

	statuses := c.GetAllStatuses()
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
}

func TestChecker_Alerts(t *testing.T) {
	c := NewChecker(Config{
		MaxSamples:        10000,
		AbsoluteTolerance: 0.001,
		RelativeTolerance: 0.001,
		AlertThreshold:    0.05,
	})

	// Generate enough mismatches to trigger alert
	for i := 0; i < 200; i++ {
		c.RecordPair("bad_feature", "user:1", float64(i), float64(i+100))
	}

	alerts := c.GetAlerts(time.Time{})
	if len(alerts) == 0 {
		t.Error("expected at least one alert")
	}
}

func TestChecker_Summary(t *testing.T) {
	c := NewChecker(DefaultConfig())

	c.RecordPair("good", "e1", 1.0, 1.0)
	c.RecordPair("bad", "e1", 1.0, 100.0)

	summary := c.GetSummary()
	if summary.TotalFeatures != 2 {
		t.Errorf("expected 2 features, got %d", summary.TotalFeatures)
	}
}

func TestChecker_Reset(t *testing.T) {
	c := NewChecker(DefaultConfig())

	c.RecordPair("f1", "e1", 1.0, 1.0)
	if err := c.Reset("f1"); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}
	if c.GetStatus("f1") != nil {
		t.Error("expected nil status after reset")
	}

	if err := c.Reset("nonexistent"); err == nil {
		t.Error("expected error for non-existent feature")
	}
}

func TestChecker_MaxSamples(t *testing.T) {
	c := NewChecker(Config{
		MaxSamples:        5,
		AbsoluteTolerance: 0.001,
		RelativeTolerance: 0.01,
		AlertThreshold:    0.5,
	})

	for i := 0; i < 10; i++ {
		c.RecordPair("f1", "e1", float64(i), float64(i))
	}

	// Should have retained only MaxSamples pairs in the window
	c.mu.RLock()
	pairCount := len(c.states["f1"].pairs)
	c.mu.RUnlock()

	if pairCount != 5 {
		t.Errorf("expected 5 retained pairs, got %d", pairCount)
	}
}
