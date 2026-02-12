package qualityscore

import (
	"testing"
	"time"
)

func TestDefaultScoringConfig(t *testing.T) {
	cfg := DefaultScoringConfig()
	if len(cfg.Weights) != 6 {
		t.Errorf("expected 6 weights, got %d", len(cfg.Weights))
	}
	if cfg.GradeThresholds.A != 0.9 {
		t.Errorf("grade A threshold = %f, want 0.9", cfg.GradeThresholds.A)
	}
}

func TestNewScorer(t *testing.T) {
	s := NewScorer(DefaultScoringConfig())
	if s == nil {
		t.Fatal("NewScorer returned nil")
	}
}

func TestScorer_RecordSignalAndScore(t *testing.T) {
	s := NewScorer(DefaultScoringConfig())

	s.RecordSignal("feature-1", &Signal{
		Type:      SignalUsage,
		Value:     0.95,
		Weight:    1.0,
		Timestamp: time.Now(),
	})
	s.RecordSignal("feature-1", &Signal{
		Type:      SignalStability,
		Value:     0.8,
		Weight:    1.0,
		Timestamp: time.Now(),
	})

	score, err := s.Score("feature-1")
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if score.OverallScore <= 0 {
		t.Error("expected positive overall score")
	}
	if score.FeatureID != "feature-1" {
		t.Errorf("FeatureID = %q, want %q", score.FeatureID, "feature-1")
	}
	if score.Grade == "" {
		t.Error("expected non-empty grade")
	}
}

func TestScorer_Score_NoSignals(t *testing.T) {
	s := NewScorer(DefaultScoringConfig())
	_, err := s.Score("nonexistent")
	if err == nil {
		t.Error("expected error for feature with no signals")
	}
}

func TestScorer_GetScore(t *testing.T) {
	s := NewScorer(DefaultScoringConfig())

	s.RecordSignal("feature-1", &Signal{Type: SignalUsage, Value: 0.9, Timestamp: time.Now()})
	_, _ = s.Score("feature-1")

	score, err := s.GetScore("feature-1")
	if err != nil {
		t.Fatalf("GetScore: %v", err)
	}
	if score.FeatureID != "feature-1" {
		t.Errorf("FeatureID = %q, want %q", score.FeatureID, "feature-1")
	}
}

func TestScorer_GetScore_NotComputed(t *testing.T) {
	s := NewScorer(DefaultScoringConfig())
	_, err := s.GetScore("not-computed")
	if err == nil {
		t.Error("expected error for uncomputed score")
	}
}

func TestScorer_ScoreAll(t *testing.T) {
	s := NewScorer(DefaultScoringConfig())

	for _, id := range []string{"f1", "f2", "f3"} {
		s.RecordSignal(id, &Signal{Type: SignalUsage, Value: 0.8, Timestamp: time.Now()})
		s.RecordSignal(id, &Signal{Type: SignalStability, Value: 0.7, Timestamp: time.Now()})
	}

	scores := s.ScoreAll()
	if len(scores) != 3 {
		t.Errorf("ScoreAll returned %d scores, want 3", len(scores))
	}

	// Should be sorted descending
	for i := 1; i < len(scores); i++ {
		if scores[i].OverallScore > scores[i-1].OverallScore {
			t.Error("scores should be sorted descending")
		}
	}
}

func TestScorer_TopN(t *testing.T) {
	s := NewScorer(DefaultScoringConfig())

	s.RecordSignal("high", &Signal{Type: SignalUsage, Value: 0.99, Timestamp: time.Now()})
	s.RecordSignal("mid", &Signal{Type: SignalUsage, Value: 0.5, Timestamp: time.Now()})
	s.RecordSignal("low", &Signal{Type: SignalUsage, Value: 0.1, Timestamp: time.Now()})
	s.ScoreAll()

	top := s.TopN(2)
	if len(top) != 2 {
		t.Errorf("TopN(2) returned %d, want 2", len(top))
	}
}

func TestScorer_TopN_MoreThanAvailable(t *testing.T) {
	s := NewScorer(DefaultScoringConfig())
	s.RecordSignal("f1", &Signal{Type: SignalUsage, Value: 0.9, Timestamp: time.Now()})
	s.ScoreAll()

	top := s.TopN(100)
	if len(top) != 1 {
		t.Errorf("TopN(100) returned %d, want 1", len(top))
	}
}

func TestScorer_BottomN(t *testing.T) {
	s := NewScorer(DefaultScoringConfig())

	s.RecordSignal("high", &Signal{Type: SignalUsage, Value: 0.99, Timestamp: time.Now()})
	s.RecordSignal("low", &Signal{Type: SignalUsage, Value: 0.1, Timestamp: time.Now()})
	s.ScoreAll()

	bottom := s.BottomN(1)
	if len(bottom) != 1 {
		t.Errorf("BottomN(1) returned %d, want 1", len(bottom))
	}
}

func TestScorer_GetDeprecationCandidates(t *testing.T) {
	s := NewScorer(DefaultScoringConfig())

	s.RecordSignal("good", &Signal{Type: SignalUsage, Value: 0.95, Timestamp: time.Now()})
	s.RecordSignal("bad", &Signal{Type: SignalUsage, Value: 0.1, Timestamp: time.Now()})
	s.ScoreAll()

	candidates := s.GetDeprecationCandidates(0.5)
	if len(candidates) != 1 {
		t.Errorf("expected 1 deprecation candidate, got %d", len(candidates))
	}
	if len(candidates) > 0 && candidates[0].FeatureID != "bad" {
		t.Errorf("expected 'bad' as candidate, got %q", candidates[0].FeatureID)
	}
}

func TestScorer_Stats(t *testing.T) {
	s := NewScorer(DefaultScoringConfig())

	// Empty stats
	stats := s.Stats()
	if stats.FeaturesScored != 0 {
		t.Errorf("FeaturesScored = %d, want 0", stats.FeaturesScored)
	}
	if stats.AverageScore != 0 {
		t.Errorf("AverageScore = %f, want 0", stats.AverageScore)
	}

	// With scores
	s.RecordSignal("f1", &Signal{Type: SignalUsage, Value: 0.9, Timestamp: time.Now()})
	s.RecordSignal("f2", &Signal{Type: SignalUsage, Value: 0.3, Timestamp: time.Now()})
	s.ScoreAll()

	stats = s.Stats()
	if stats.FeaturesScored != 2 {
		t.Errorf("FeaturesScored = %d, want 2", stats.FeaturesScored)
	}
	if stats.AverageScore <= 0 {
		t.Error("expected positive AverageScore")
	}
}

func TestScorer_Grades(t *testing.T) {
	s := NewScorer(DefaultScoringConfig())

	tests := []struct {
		name  string
		value float64
		grade string
	}{
		{"A grade", 0.95, "A"},
		{"B grade", 0.8, "B"},
		{"C grade", 0.65, "C"},
		{"D grade", 0.45, "D"},
		{"F grade", 0.2, "F"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grade := s.computeGrade(tt.value)
			if grade != tt.grade {
				t.Errorf("computeGrade(%f) = %q, want %q", tt.value, grade, tt.grade)
			}
		})
	}
}

func TestScorer_Recommendations(t *testing.T) {
	s := NewScorer(DefaultScoringConfig())

	signals := map[SignalType]*Signal{
		SignalUsage:     {Type: SignalUsage, Value: 0.3},
		SignalFreshness: {Type: SignalFreshness, Value: 0.2},
	}

	recs := s.generateRecommendations(signals)
	if len(recs) == 0 {
		t.Error("expected recommendations for low signals")
	}
}

func TestScorer_ValueClamping(t *testing.T) {
	s := NewScorer(DefaultScoringConfig())

	// Values above 1 should be clamped
	s.RecordSignal("clamped", &Signal{Type: SignalUsage, Value: 1.5, Timestamp: time.Now()})
	s.RecordSignal("clamped", &Signal{Type: SignalStability, Value: -0.5, Timestamp: time.Now()})

	score, err := s.Score("clamped")
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if score.OverallScore < 0 || score.OverallScore > 1 {
		t.Errorf("OverallScore = %f, expected in [0,1]", score.OverallScore)
	}
}

func TestSignalType_Constants(t *testing.T) {
	types := []SignalType{SignalUsage, SignalStability, SignalFreshness, SignalCompleteness, SignalUniqueness, SignalDrift}
	if len(types) != 6 {
		t.Errorf("expected 6 signal types, got %d", len(types))
	}
}
