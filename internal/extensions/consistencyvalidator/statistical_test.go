package consistencyvalidator

import (
	"math"
	"testing"
)

func TestComputePSI(t *testing.T) {
	// Same distribution should have PSI near 0
	same := make([]float64, 1000)
	for i := range same {
		same[i] = float64(i) / 1000.0
	}
	psi := computePSI(same, same, 10)
	if psi > 0.01 {
		t.Fatalf("expected PSI near 0 for same distribution, got %f", psi)
	}

	// Very different distribution: uniform vs concentrated
	uniform := make([]float64, 1000)
	concentrated := make([]float64, 1000)
	for i := range uniform {
		uniform[i] = float64(i) / 1000.0
	}
	for i := range concentrated {
		concentrated[i] = 0.5 + float64(i)/10000.0 // tightly clustered around 0.5
	}
	psi2 := computePSI(uniform, concentrated, 10)
	if psi2 < 0.1 {
		t.Fatalf("expected PSI > 0.1 for different distributions, got %f", psi2)
	}
}

func TestComputeChiSquared(t *testing.T) {
	// Same distribution
	same := make([]float64, 500)
	for i := range same {
		same[i] = float64(i) / 500.0
	}
	chi := computeChiSquared(same, same, 10)
	if chi > 0.01 {
		t.Fatalf("expected chi-squared near 0, got %f", chi)
	}
}

func TestComputeJensenShannon(t *testing.T) {
	// Same distribution should have JSD near 0
	same := make([]float64, 500)
	for i := range same {
		same[i] = float64(i) / 500.0
	}
	jsd := computeJensenShannon(same, same, 10)
	if jsd > 0.01 {
		t.Fatalf("expected JSD near 0, got %f", jsd)
	}

	// JSD should be bounded [0, 1] for the metric form (sqrt)
	shifted := make([]float64, 500)
	for i := range shifted {
		shifted[i] = float64(i)/500.0 + 10.0
	}
	jsd2 := computeJensenShannon(same, shifted, 10)
	if jsd2 < 0 || jsd2 > 1.0 {
		t.Fatalf("expected JSD in [0, 1], got %f", jsd2)
	}
}

func TestRunAllTests(t *testing.T) {
	online := make([]float64, 200)
	offline := make([]float64, 200)
	for i := range online {
		online[i] = float64(i) / 200.0
		offline[i] = float64(i) / 200.0
	}

	cfg := DefaultPerFeatureConfig()
	results := RunAllTests(online, offline, cfg)

	if len(results) != 3 {
		t.Fatalf("expected 3 test results, got %d", len(results))
	}
	for _, r := range results {
		if !r.Passed {
			t.Fatalf("test %s should pass for same distribution, statistic=%f threshold=%f", r.Test, r.Statistic, r.Threshold)
		}
	}
}

func TestRunAllTests_WithChiSquared(t *testing.T) {
	online := make([]float64, 200)
	offline := make([]float64, 200)
	for i := range online {
		online[i] = float64(i) / 200.0
		offline[i] = float64(i) / 200.0
	}

	cfg := PerFeatureConfig{
		KSThreshold:         0.05,
		PSIThreshold:        0.2,
		ChiSquaredThreshold: 0.05,
		JSThreshold:         0.1,
		EnabledTests:        []StatisticalTest{TestKS, TestPSI, TestChiSquared, TestJensenShannon},
	}
	results := RunAllTests(online, offline, cfg)
	if len(results) != 4 {
		t.Fatalf("expected 4 test results, got %d", len(results))
	}
}

func TestTakeSnapshot(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	snap := TakeSnapshot("test_feature", "online", values)

	if snap.Feature != "test_feature" {
		t.Fatalf("expected feature 'test_feature', got %q", snap.Feature)
	}
	if snap.Count != 10 {
		t.Fatalf("expected count 10, got %d", snap.Count)
	}
	if snap.Min != 1.0 {
		t.Fatalf("expected min 1.0, got %f", snap.Min)
	}
	if snap.Max != 10.0 {
		t.Fatalf("expected max 10.0, got %f", snap.Max)
	}
	if math.Abs(snap.Mean-5.5) > 0.01 {
		t.Fatalf("expected mean ~5.5, got %f", snap.Mean)
	}
	if len(snap.Histogram) != 10 {
		t.Fatalf("expected 10 bins, got %d", len(snap.Histogram))
	}
}

func TestTakeSnapshot_Empty(t *testing.T) {
	snap := TakeSnapshot("empty", "online", nil)
	if snap.Count != 0 {
		t.Fatalf("expected count 0, got %d", snap.Count)
	}
}

func TestHistogram(t *testing.T) {
	values := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	hist := histogram(values, 5)
	if len(hist) != 5 {
		t.Fatalf("expected 5 bins, got %d", len(hist))
	}
	// Each bin should have ~0.2 (2 values out of 10)
	sum := 0.0
	for _, h := range hist {
		sum += h
	}
	if math.Abs(sum-1.0) > 0.01 {
		t.Fatalf("expected histogram sum ~1.0, got %f", sum)
	}
}

func TestHistogram_Constant(t *testing.T) {
	values := []float64{5, 5, 5, 5}
	hist := histogram(values, 5)
	if hist[0] != 1.0 {
		t.Fatalf("expected all mass in first bin for constant values, got %f", hist[0])
	}
}

func TestPercentile(t *testing.T) {
	sorted := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	p50 := percentile(sorted, 0.5)
	if math.Abs(p50-5.5) > 0.01 {
		t.Fatalf("expected p50 ~5.5, got %f", p50)
	}
	p95 := percentile(sorted, 0.95)
	if p95 < 9.0 {
		t.Fatalf("expected p95 >= 9.0, got %f", p95)
	}
}

func TestEmptyDistributions(t *testing.T) {
	if psi := computePSI(nil, nil, 10); psi != 0 {
		t.Fatalf("expected 0 for empty, got %f", psi)
	}
	if chi := computeChiSquared(nil, nil, 10); chi != 0 {
		t.Fatalf("expected 0 for empty, got %f", chi)
	}
	if jsd := computeJensenShannon(nil, nil, 10); jsd != 0 {
		t.Fatalf("expected 0 for empty, got %f", jsd)
	}
}
