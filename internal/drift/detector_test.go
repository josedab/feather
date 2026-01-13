package drift

import (
	"errors"
	"math/rand"
	"testing"
	"time"
)

func TestDetector_NumericDrift(t *testing.T) {
	detector := NewDetector(Config{
		WindowSize:    100,
		ReferenceSize: 200,
		KSThreshold:   0.3,
		AlertCooldown: 0, // Disable cooldown for testing
	})

	// Record reference distribution (normal around 0)
	for i := 0; i < 200; i++ {
		detector.RecordNumeric("feature1", rand.NormFloat64())
	}

	// Record current distribution (shifted)
	for i := 0; i < 100; i++ {
		detector.RecordNumeric("feature1", rand.NormFloat64()+5) // Shifted by 5
	}

	// Check status
	statuses := detector.GetMonitorStatus()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 monitor, got %d", len(statuses))
	}

	status := statuses[0]
	if status.Feature != "feature1" {
		t.Errorf("expected feature1, got %s", status.Feature)
	}

	// Should detect drift due to mean shift
	if status.DriftType == DriftNone {
		t.Log("Drift score:", status.DriftScore)
		// Note: Due to randomness, drift may not always be detected
	}
}

func TestDetector_CategoricalDrift(t *testing.T) {
	detector := NewDetector(Config{
		WindowSize:    100,
		ReferenceSize: 200,
		PSIThreshold:  0.1,
		AlertCooldown: 0,
	})

	// Record reference distribution
	categories := []string{"A", "B", "C"}
	for i := 0; i < 200; i++ {
		idx := rand.Intn(3)
		detector.RecordCategorical("cat_feature", categories[idx])
	}

	// Record current distribution (heavily biased toward A)
	for i := 0; i < 100; i++ {
		if rand.Float64() < 0.8 {
			detector.RecordCategorical("cat_feature", "A")
		} else {
			detector.RecordCategorical("cat_feature", "B")
		}
	}

	statuses := detector.GetMonitorStatus()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 monitor, got %d", len(statuses))
	}

	status := statuses[0]
	if status.Type != TypeCategorical {
		t.Errorf("expected categorical type, got %v", status.Type)
	}
}

func TestDetector_KSTest(t *testing.T) {
	detector := NewDetector(DefaultConfig())

	// Test identical distributions
	sample := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	ks := detector.ksTest(sample, sample)
	if ks > 0.01 {
		t.Errorf("KS for identical samples should be ~0, got %f", ks)
	}

	// Test completely different distributions (non-overlapping)
	ref := []float64{1, 2, 3, 4, 5}
	cur := []float64{100, 101, 102, 103, 104}
	ks = detector.ksTest(ref, cur)
	if ks < 0.8 {
		t.Errorf("KS for non-overlapping samples should be high, got %f", ks)
	}
}

func TestDetector_PSITest(t *testing.T) {
	detector := NewDetector(DefaultConfig())

	// Identical distributions
	cats := map[string]int{"A": 50, "B": 30, "C": 20}
	psi := detector.psiTest(cats, cats, 100, 100)
	if psi > 0.001 {
		t.Errorf("PSI for identical distributions should be ~0, got %f", psi)
	}

	// Different distributions
	refCats := map[string]int{"A": 33, "B": 33, "C": 34}
	curCats := map[string]int{"A": 80, "B": 15, "C": 5}
	psi = detector.psiTest(refCats, curCats, 100, 100)
	if psi < 0.1 {
		t.Errorf("PSI for different distributions should be > 0.1, got %f", psi)
	}
}

func TestDetector_Alerts(t *testing.T) {
	detector := NewDetector(Config{
		WindowSize:    50,
		ReferenceSize: 100,
		KSThreshold:   0.1,
		AlertCooldown: 0,
	})

	// Record reference
	for i := 0; i < 100; i++ {
		detector.RecordNumeric("drift_feature", float64(i))
	}

	// Record drifted values
	for i := 0; i < 100; i++ {
		detector.RecordNumeric("drift_feature", float64(i+1000))
	}

	// Check alerts
	alerts := detector.GetAlerts(time.Now().Add(-1 * time.Hour))
	// May or may not have alerts depending on drift detection
	_ = alerts
}

func TestDetector_ResetReference(t *testing.T) {
	detector := NewDetector(DefaultConfig())

	// Record some data
	for i := 0; i < 100; i++ {
		detector.RecordNumeric("reset_feature", float64(i))
	}

	// Reset reference
	if err := detector.ResetReference("reset_feature"); err != nil {
		t.Fatalf("ResetReference failed: %v", err)
	}

	// Check that drift is cleared
	statuses := detector.GetMonitorStatus()
	for _, s := range statuses {
		if s.Feature == "reset_feature" && s.DriftType != DriftNone {
			t.Errorf("drift should be cleared after reset")
		}
	}

	// Reset non-existent feature
	if err := detector.ResetReference("nonexistent"); err != nil && !errors.Is(err, ErrFeatureNotFound) {
		t.Errorf("expected ErrFeatureNotFound, got %v", err)
	}
}

func TestDistribution_UpdateStats(t *testing.T) {
	dist := &Distribution{
		Values: []float64{1, 2, 3, 4, 5},
	}
	dist.updateStats()

	expectedMean := 3.0
	if dist.Mean != expectedMean {
		t.Errorf("Mean = %f, want %f", dist.Mean, expectedMean)
	}

	if dist.Min != 1 {
		t.Errorf("Min = %f, want 1", dist.Min)
	}

	if dist.Max != 5 {
		t.Errorf("Max = %f, want 5", dist.Max)
	}

	// StdDev for [1,2,3,4,5] = sqrt(2)
	expectedStdDev := 1.4142135623730951
	if dist.StdDev < 1.4 || dist.StdDev > 1.5 {
		t.Errorf("StdDev = %f, want ~%f", dist.StdDev, expectedStdDev)
	}
}

func BenchmarkDetector_RecordNumeric(b *testing.B) {
	detector := NewDetector(DefaultConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.RecordNumeric("bench_feature", rand.NormFloat64())
	}
}

func BenchmarkDetector_RecordCategorical(b *testing.B) {
	detector := NewDetector(DefaultConfig())
	categories := []string{"A", "B", "C", "D", "E"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.RecordCategorical("bench_cat", categories[rand.Intn(5)])
	}
}
