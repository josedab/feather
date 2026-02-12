package costopt

import (
	"testing"
	"time"
)

func TestDefaultAnalyzerConfig(t *testing.T) {
	cfg := DefaultAnalyzerConfig()
	if cfg.AnalysisWindow != 24*time.Hour {
		t.Errorf("expected 24h window, got %v", cfg.AnalysisWindow)
	}
	if cfg.MinSamples != 100 {
		t.Errorf("expected 100 min samples, got %d", cfg.MinSamples)
	}
}

func TestRecordAndGetPattern(t *testing.T) {
	a := NewAnalyzer(DefaultAnalyzerConfig())

	for i := 0; i < 10; i++ {
		a.RecordAccess("user_features", "user-1", "hot", time.Millisecond, false)
	}
	a.RecordAccess("user_features", "user-2", "hot", 2*time.Millisecond, true)

	p := a.GetPattern("user_features")
	if p == nil {
		t.Fatal("expected pattern, got nil")
	}
	if p.AccessCount != 11 {
		t.Errorf("expected 11 accesses, got %d", p.AccessCount)
	}
	if p.ReadWriteRatio != 10 {
		t.Errorf("expected R/W ratio 10, got %f", p.ReadWriteRatio)
	}
}

func TestGetPatternMissing(t *testing.T) {
	a := NewAnalyzer(DefaultAnalyzerConfig())
	if p := a.GetPattern("nonexistent"); p != nil {
		t.Error("expected nil for unknown group")
	}
}

func TestListPatterns(t *testing.T) {
	a := NewAnalyzer(DefaultAnalyzerConfig())
	for i := 0; i < 5; i++ {
		a.RecordAccess("low", "e", "hot", time.Millisecond, false)
	}
	for i := 0; i < 20; i++ {
		a.RecordAccess("high", "e", "warm", time.Millisecond, false)
	}

	patterns := a.ListPatterns()
	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(patterns))
	}
	if patterns[0].FeatureGroup != "high" {
		t.Error("expected patterns sorted by access count descending")
	}
}

func TestAnalyzeAll(t *testing.T) {
	a := NewAnalyzer(DefaultAnalyzerConfig())
	a.RecordAccess("g1", "e1", "hot", time.Millisecond, false)
	patterns := a.AnalyzeAll()
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}
}

func TestComputeP99(t *testing.T) {
	latencies := make([]time.Duration, 100)
	for i := range latencies {
		latencies[i] = time.Duration(i+1) * time.Millisecond
	}
	p99 := computeP99(latencies)
	if p99 != 100*time.Millisecond {
		t.Errorf("expected 100ms p99, got %v", p99)
	}
	if computeP99(nil) != 0 {
		t.Error("expected 0 for empty slice")
	}
}
