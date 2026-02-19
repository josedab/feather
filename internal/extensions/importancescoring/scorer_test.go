package importancescoring

import (
	"testing"
)

func TestNewScorer(t *testing.T) {
	cfg := DefaultScorerConfig()
	s := NewScorer(cfg)
	if s == nil {
		t.Fatal("expected non-nil scorer")
	}
	if s.config.MinSamples != 100 {
		t.Errorf("expected MinSamples=100, got %d", s.config.MinSamples)
	}
}

func TestRecordAccess(t *testing.T) {
	s := NewScorer(DefaultScorerConfig())

	for i := 0; i < 10; i++ {
		s.RecordAccess("feature1", float64(i))
	}

	s.mu.RLock()
	fd := s.features["feature1"]
	s.mu.RUnlock()

	if fd == nil {
		t.Fatal("expected feature1 to be tracked")
	}
	if fd.accessCount != 10 {
		t.Errorf("expected 10 accesses, got %d", fd.accessCount)
	}
	if len(fd.values) != 10 {
		t.Errorf("expected 10 values, got %d", len(fd.values))
	}
}

func TestScoreAll(t *testing.T) {
	s := NewScorer(DefaultScorerConfig())

	// Feature A: high access, high variance
	for i := 0; i < 100; i++ {
		s.RecordAccess("featureA", float64(i)*10)
	}
	// Feature B: medium access, low variance
	for i := 0; i < 50; i++ {
		s.RecordAccess("featureB", 5.0)
	}
	// Feature C: low access, medium variance
	for i := 0; i < 10; i++ {
		s.RecordAccess("featureC", float64(i)*5)
	}

	scores := s.ScoreAll()
	if len(scores) != 3 {
		t.Fatalf("expected 3 scores, got %d", len(scores))
	}

	// Scores should be sorted descending
	if scores[0].ImportanceScore < scores[len(scores)-1].ImportanceScore {
		t.Error("scores should be sorted descending by importance")
	}

	// Feature A should rank highest due to high access + high variance
	if scores[0].Name != "featureA" {
		t.Errorf("expected featureA to be top, got %s", scores[0].Name)
	}
}

func TestGetTopK(t *testing.T) {
	s := NewScorer(DefaultScorerConfig())

	for i := 0; i < 50; i++ {
		s.RecordAccess("high", float64(i)*10)
	}
	for i := 0; i < 10; i++ {
		s.RecordAccess("low", 1.0)
	}
	s.ScoreAll()

	top := s.GetTopK(1)
	if len(top) != 1 {
		t.Fatalf("expected 1 result, got %d", len(top))
	}
	if top[0].Name != "high" {
		t.Errorf("expected 'high' as top feature, got %s", top[0].Name)
	}
}

func TestGetDeprecationCandidates(t *testing.T) {
	cfg := DefaultScorerConfig()
	cfg.LowImportanceThreshold = 0.5
	s := NewScorer(cfg)

	// High importance feature
	for i := 0; i < 100; i++ {
		s.RecordAccess("important", float64(i)*100)
	}
	// Low importance feature: very few accesses, no variance
	for i := 0; i < 2; i++ {
		s.RecordAccess("unimportant", 1.0)
	}
	s.ScoreAll()

	candidates := s.GetDeprecationCandidates()
	found := false
	for _, c := range candidates {
		if c.Name == "unimportant" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'unimportant' to be a deprecation candidate")
	}
}

func TestStats(t *testing.T) {
	s := NewScorer(DefaultScorerConfig())

	s.RecordAccess("f1", 1.0)
	s.RecordAccess("f2", 2.0)

	stats := s.Stats()
	if stats.TotalFeatures != 2 {
		t.Errorf("expected 2 total features, got %d", stats.TotalFeatures)
	}
	if stats.ScoredFeatures != 0 {
		t.Errorf("expected 0 scored features before ScoreAll, got %d", stats.ScoredFeatures)
	}

	s.ScoreAll()
	stats = s.Stats()
	if stats.ScoredFeatures != 2 {
		t.Errorf("expected 2 scored features after ScoreAll, got %d", stats.ScoredFeatures)
	}
}
