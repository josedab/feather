package mlflow

import (
	"testing"
	"time"
)

func newTestTracker() *Tracker {
	return NewTracker(DefaultConfig())
}

// --- StartRun ---

func TestStartRun_Valid(t *testing.T) {
	tr := newTestTracker()
	run, err := tr.StartRun("my-run", "exp1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Name != "my-run" {
		t.Fatalf("expected name my-run, got %s", run.Name)
	}
	if run.ExperimentID != "exp1" {
		t.Fatalf("expected experiment exp1, got %s", run.ExperimentID)
	}
	if run.Status != "running" {
		t.Fatalf("expected status running, got %s", run.Status)
	}
	if run.ID == "" {
		t.Fatal("expected non-empty run ID")
	}
}

func TestStartRun_DefaultExperiment(t *testing.T) {
	tr := newTestTracker()
	run, err := tr.StartRun("my-run", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.ExperimentID != "default" {
		t.Fatalf("expected default experiment, got %s", run.ExperimentID)
	}
}

func TestStartRun_EmptyName(t *testing.T) {
	tr := newTestTracker()
	_, err := tr.StartRun("", "exp1")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

// --- EndRun ---

func TestEndRun_Completed(t *testing.T) {
	tr := newTestTracker()
	run, _ := tr.StartRun("run1", "")
	err := tr.EndRun(run.ID, "completed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Status != "completed" {
		t.Fatalf("expected completed, got %s", run.Status)
	}
	if run.EndTime.IsZero() {
		t.Fatal("expected end time to be set")
	}
}

func TestEndRun_Failed(t *testing.T) {
	tr := newTestTracker()
	run, _ := tr.StartRun("run1", "")
	err := tr.EndRun(run.ID, "failed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Status != "failed" {
		t.Fatalf("expected failed, got %s", run.Status)
	}
}

func TestEndRun_InvalidStatus(t *testing.T) {
	tr := newTestTracker()
	run, _ := tr.StartRun("run1", "")
	err := tr.EndRun(run.ID, "invalid")
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestEndRun_NotFound(t *testing.T) {
	tr := newTestTracker()
	err := tr.EndRun("nonexistent", "completed")
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
}

func TestEndRun_AlreadyEnded(t *testing.T) {
	tr := newTestTracker()
	run, _ := tr.StartRun("run1", "")
	_ = tr.EndRun(run.ID, "completed")
	err := tr.EndRun(run.ID, "failed")
	if err == nil {
		t.Fatal("expected error for already ended run")
	}
}

// --- LogFeatureUsage ---

func TestLogFeatureUsage_Valid(t *testing.T) {
	tr := newTestTracker()
	run, _ := tr.StartRun("run1", "")
	err := tr.LogFeatureUsage(run.ID, []string{"f1", "f2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.FeaturesUsed) != 2 {
		t.Fatalf("expected 2 features, got %d", len(run.FeaturesUsed))
	}
}

func TestLogFeatureUsage_NotFound(t *testing.T) {
	tr := newTestTracker()
	err := tr.LogFeatureUsage("nonexistent", []string{"f1"})
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
}

func TestLogFeatureUsage_CreatesLineage(t *testing.T) {
	tr := newTestTracker()
	run, _ := tr.StartRun("run1", "")
	_ = tr.LogFeatureUsage(run.ID, []string{"f1", "f2"})

	lineage := tr.GetLineage("f1")
	if len(lineage) != 1 {
		t.Fatalf("expected 1 lineage entry for f1, got %d", len(lineage))
	}
	if lineage[0].RunID != run.ID {
		t.Fatalf("expected run ID %s, got %s", run.ID, lineage[0].RunID)
	}
}

func TestLogFeatureUsage_MultipleRuns(t *testing.T) {
	tr := newTestTracker()
	run1, _ := tr.StartRun("run1", "")
	time.Sleep(time.Millisecond)
	run2, _ := tr.StartRun("run2", "")

	_ = tr.LogFeatureUsage(run1.ID, []string{"f1"})
	_ = tr.LogFeatureUsage(run2.ID, []string{"f1"})

	lineage := tr.GetLineage("f1")
	if len(lineage) != 2 {
		t.Fatalf("expected 2 lineage entries for f1, got %d", len(lineage))
	}
}

// --- LogMetrics ---

func TestLogMetrics_Valid(t *testing.T) {
	tr := newTestTracker()
	run, _ := tr.StartRun("run1", "")
	err := tr.LogMetrics(run.ID, map[string]float64{"accuracy": 0.95, "loss": 0.05})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Metrics["accuracy"] != 0.95 {
		t.Fatalf("expected accuracy 0.95, got %f", run.Metrics["accuracy"])
	}
}

func TestLogMetrics_NotFound(t *testing.T) {
	tr := newTestTracker()
	err := tr.LogMetrics("nonexistent", map[string]float64{"x": 1})
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
}

func TestLogMetrics_Overwrite(t *testing.T) {
	tr := newTestTracker()
	run, _ := tr.StartRun("run1", "")
	_ = tr.LogMetrics(run.ID, map[string]float64{"accuracy": 0.8})
	_ = tr.LogMetrics(run.ID, map[string]float64{"accuracy": 0.95})
	if run.Metrics["accuracy"] != 0.95 {
		t.Fatalf("expected overwritten accuracy 0.95, got %f", run.Metrics["accuracy"])
	}
}

// --- GetRun ---

func TestGetRun_Found(t *testing.T) {
	tr := newTestTracker()
	run, _ := tr.StartRun("run1", "")
	got, err := tr.GetRun(run.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != run.ID {
		t.Fatalf("expected run %s, got %s", run.ID, got.ID)
	}
}

func TestGetRun_NotFound(t *testing.T) {
	tr := newTestTracker()
	_, err := tr.GetRun("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
}

// --- ListRuns ---

func TestListRuns(t *testing.T) {
	tr := newTestTracker()
	tr.StartRun("run1", "")
	time.Sleep(time.Millisecond)
	tr.StartRun("run2", "")

	runs := tr.ListRuns()
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
}

func TestListRuns_Empty(t *testing.T) {
	tr := newTestTracker()
	runs := tr.ListRuns()
	if len(runs) != 0 {
		t.Fatalf("expected 0 runs, got %d", len(runs))
	}
}

// --- GetLineage ---

func TestGetLineage_NoEntries(t *testing.T) {
	tr := newTestTracker()
	lineage := tr.GetLineage("nonexistent")
	if len(lineage) != 0 {
		t.Fatalf("expected 0 lineage entries, got %d", len(lineage))
	}
}

// --- Stats ---

func TestStats(t *testing.T) {
	tr := newTestTracker()
	run1, _ := tr.StartRun("run1", "")
	time.Sleep(time.Millisecond)
	tr.StartRun("run2", "")

	_ = tr.LogFeatureUsage(run1.ID, []string{"f1", "f2"})
	_ = tr.EndRun(run1.ID, "completed")

	stats := tr.Stats()
	if stats.TotalRuns != 2 {
		t.Fatalf("expected 2 total runs, got %d", stats.TotalRuns)
	}
	if stats.ActiveRuns != 1 {
		t.Fatalf("expected 1 active run, got %d", stats.ActiveRuns)
	}
	if stats.FeaturesTracked != 2 {
		t.Fatalf("expected 2 features tracked, got %d", stats.FeaturesTracked)
	}
}

func TestStats_Empty(t *testing.T) {
	tr := newTestTracker()
	stats := tr.Stats()
	if stats.TotalRuns != 0 || stats.ActiveRuns != 0 {
		t.Fatal("expected all zeros for empty tracker")
	}
}

// --- DuplicateRunNames (allowed) ---

func TestDuplicateRunNames(t *testing.T) {
	tr := newTestTracker()
	r1, err1 := tr.StartRun("same-name", "")
	time.Sleep(time.Millisecond)
	r2, err2 := tr.StartRun("same-name", "")
	if err1 != nil || err2 != nil {
		t.Fatal("duplicate names should be allowed")
	}
	if r1.ID == r2.ID {
		t.Fatal("expected different IDs for same-name runs")
	}
}
