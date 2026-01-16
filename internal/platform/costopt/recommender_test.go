package costopt

import (
	"testing"
	"time"
)

func TestDefaultRecommenderConfig(t *testing.T) {
	cfg := DefaultRecommenderConfig()
	if cfg.MinSavingsThreshold != 5.0 {
		t.Errorf("unexpected min savings: %f", cfg.MinSavingsThreshold)
	}
}

func TestGenerateRecommendations_LowAccess(t *testing.T) {
	a := NewAnalyzer(DefaultAnalyzerConfig())
	// Record fewer than MinSamples accesses in hot tier
	for i := 0; i < 5; i++ {
		a.RecordAccess("sparse_group", "e1", "hot", time.Millisecond, false)
	}

	r := NewRecommender(a, DefaultRecommenderConfig())
	recs := r.GenerateRecommendations()

	found := false
	for _, rec := range recs {
		if rec.Type == RecommendTierMove && rec.RecommendedState == "warm" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected tier_move recommendation to warm for low-access hot feature")
	}
}

func TestGenerateRecommendations_HighLatency(t *testing.T) {
	a := NewAnalyzer(DefaultAnalyzerConfig())
	for i := 0; i < 10; i++ {
		a.RecordAccess("slow_group", "e1", "hot", 100*time.Millisecond, false)
	}

	r := NewRecommender(a, DefaultRecommenderConfig())
	recs := r.GenerateRecommendations()

	found := false
	for _, rec := range recs {
		if rec.Type == RecommendScaling {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected scaling recommendation for high P99 latency")
	}
}

func TestApplyAndDismissRecommendation(t *testing.T) {
	a := NewAnalyzer(DefaultAnalyzerConfig())
	a.RecordAccess("g1", "e1", "hot", time.Millisecond, false)
	r := NewRecommender(a, DefaultRecommenderConfig())
	recs := r.GenerateRecommendations()

	if len(recs) == 0 {
		t.Fatal("expected at least one recommendation")
	}
	id := recs[0].ID

	if err := r.ApplyRecommendation(id); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	rec, _ := r.GetRecommendation(id)
	if !rec.Applied {
		t.Error("expected recommendation to be applied")
	}

	if err := r.DismissRecommendation(id); err != nil {
		t.Fatalf("dismiss failed: %v", err)
	}
	rec, _ = r.GetRecommendation(id)
	if !rec.Dismissed {
		t.Error("expected recommendation to be dismissed")
	}
}

func TestGetRecommendation_NotFound(t *testing.T) {
	r := NewRecommender(NewAnalyzer(DefaultAnalyzerConfig()), DefaultRecommenderConfig())
	if _, err := r.GetRecommendation("nonexistent"); err == nil {
		t.Error("expected error for missing recommendation")
	}
}

func TestApplyRecommendation_NotFound(t *testing.T) {
	r := NewRecommender(NewAnalyzer(DefaultAnalyzerConfig()), DefaultRecommenderConfig())
	if err := r.ApplyRecommendation("bad"); err == nil {
		t.Error("expected error")
	}
}

func TestDismissRecommendation_NotFound(t *testing.T) {
	r := NewRecommender(NewAnalyzer(DefaultAnalyzerConfig()), DefaultRecommenderConfig())
	if err := r.DismissRecommendation("bad"); err == nil {
		t.Error("expected error")
	}
}

func TestStats(t *testing.T) {
	a := NewAnalyzer(DefaultAnalyzerConfig())
	for i := 0; i < 3; i++ {
		a.RecordAccess("g1", "e1", "hot", time.Millisecond, false)
	}
	r := NewRecommender(a, DefaultRecommenderConfig())
	recs := r.GenerateRecommendations()

	if len(recs) == 0 {
		t.Fatal("expected recommendations")
	}
	_ = r.ApplyRecommendation(recs[0].ID)

	stats := r.Stats()
	if stats.TotalRecommendations != len(recs) {
		t.Errorf("expected %d total, got %d", len(recs), stats.TotalRecommendations)
	}
	if stats.Applied != 1 {
		t.Errorf("expected 1 applied, got %d", stats.Applied)
	}
}
