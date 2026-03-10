package anomalydetect

import (
	"testing"
)

func TestIsolationForest_FitAndScore(t *testing.T) {
	config := DefaultIsolationForestConfig()
	config.NumTrees = 50
	config.SampleSize = 64
	forest := NewIsolationForest(config)

	// Train on normal data centered around 50
	data := make([]float64, 500)
	for i := range data {
		data[i] = 50 + float64(i%10) - 5
	}
	forest.Fit(data)

	if !forest.IsFitted() {
		t.Fatal("expected forest to be fitted after Fit()")
	}

	// Normal value should have lower score
	normalScore := forest.Score(50)
	// Outlier should have higher score
	outlierScore := forest.Score(1000)

	if outlierScore <= normalScore {
		t.Errorf("expected outlier score (%.4f) > normal score (%.4f)", outlierScore, normalScore)
	}
}

func TestIsolationForest_IsAnomaly(t *testing.T) {
	config := DefaultIsolationForestConfig()
	config.NumTrees = 50
	config.SampleSize = 64
	config.AnomalyThreshold = 0.6
	forest := NewIsolationForest(config)

	data := make([]float64, 500)
	for i := range data {
		data[i] = float64(i%20) + 40
	}
	forest.Fit(data)

	// Extreme outlier should be anomalous
	if !forest.IsAnomaly(99999) {
		t.Error("expected extreme outlier to be detected as anomaly")
	}
}

func TestIsolationForest_IsFitted(t *testing.T) {
	forest := NewIsolationForest(DefaultIsolationForestConfig())

	if forest.IsFitted() {
		t.Error("expected IsFitted() to be false before Fit()")
	}

	forest.Fit([]float64{1, 2, 3, 4, 5})

	if !forest.IsFitted() {
		t.Error("expected IsFitted() to be true after Fit()")
	}
}

func TestIsolationForest_EmptyData(t *testing.T) {
	forest := NewIsolationForest(DefaultIsolationForestConfig())

	// Score before fitting should return 0
	score := forest.Score(42)
	if score != 0 {
		t.Errorf("expected score 0 for unfitted forest, got %f", score)
	}

	// Fit with empty data
	forest.Fit([]float64{})
	score = forest.Score(42)
	// Should not panic, score may vary
}

func TestIsolationForest_DefaultConfig(t *testing.T) {
	config := DefaultIsolationForestConfig()
	if config.NumTrees != 100 {
		t.Errorf("expected NumTrees=100, got %d", config.NumTrees)
	}
	if config.SampleSize != 256 {
		t.Errorf("expected SampleSize=256, got %d", config.SampleSize)
	}
	if config.MaxDepth != 8 {
		t.Errorf("expected MaxDepth=8, got %d", config.MaxDepth)
	}
	if config.AnomalyThreshold != 0.6 {
		t.Errorf("expected AnomalyThreshold=0.6, got %f", config.AnomalyThreshold)
	}
}

func TestIsolationForest_ZeroConfig(t *testing.T) {
	// Zero config should use defaults
	forest := NewIsolationForest(IsolationForestConfig{})
	if forest.config.NumTrees != 100 {
		t.Errorf("expected default NumTrees=100, got %d", forest.config.NumTrees)
	}
}
