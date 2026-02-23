package consistencyvalidator

import (
	"math"
	"math/rand"
	"testing"
)

func TestKolmogorovSmirnov_IdenticalDistributions(t *testing.T) {
	data := make([]float64, 200)
	for i := range data {
		data[i] = float64(i) / 200.0
	}

	result := KolmogorovSmirnov(data, data, 0.05)
	if !result.Passed {
		t.Errorf("expected identical distributions to pass, got statistic=%f", result.Statistic)
	}
	if result.Statistic > 0.01 {
		t.Errorf("expected near-zero D for identical samples, got %f", result.Statistic)
	}
}

func TestKolmogorovSmirnov_DifferentDistributions(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	a := make([]float64, 500)
	b := make([]float64, 500)
	for i := range a {
		a[i] = rng.NormFloat64()       // mean=0
		b[i] = rng.NormFloat64() + 3.0 // mean=3, clearly shifted
	}

	result := KolmogorovSmirnov(a, b, 0.05)
	if result.Passed {
		t.Errorf("expected different distributions to fail, got statistic=%f", result.Statistic)
	}
	if result.PValue >= 0.05 {
		t.Errorf("expected small p-value, got %f", result.PValue)
	}
}

func TestKolmogorovSmirnov_EmptyInputs(t *testing.T) {
	tests := []struct {
		name    string
		online  []float64
		offline []float64
	}{
		{"both empty", nil, nil},
		{"online empty", nil, []float64{1, 2, 3}},
		{"offline empty", []float64{1, 2, 3}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := KolmogorovSmirnov(tc.online, tc.offline, 0.05)
			if !result.Passed {
				t.Error("expected pass for empty input")
			}
		})
	}
}

func TestKolmogorovSmirnov_SampleSizes(t *testing.T) {
	a := []float64{1, 2, 3}
	b := []float64{4, 5}
	result := KolmogorovSmirnov(a, b, 1.0)
	if result.SampleSize[0] != 3 || result.SampleSize[1] != 2 {
		t.Errorf("unexpected sample sizes: %v", result.SampleSize)
	}
}

func TestPopulationStabilityIndex_StableDistributions(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	a := make([]float64, 1000)
	b := make([]float64, 1000)
	for i := range a {
		a[i] = rng.NormFloat64()
		b[i] = rng.NormFloat64()
	}

	result := PopulationStabilityIndex(a, b, 10, 0.2)
	if !result.Passed {
		t.Errorf("expected stable distributions to pass, PSI=%f", result.Statistic)
	}
}

func TestPopulationStabilityIndex_ShiftedDistribution(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	a := make([]float64, 1000)
	b := make([]float64, 1000)
	for i := range a {
		a[i] = rng.NormFloat64()
		b[i] = rng.NormFloat64() + 5.0
	}

	result := PopulationStabilityIndex(a, b, 10, 0.2)
	if result.Passed {
		t.Errorf("expected shifted distribution to fail, PSI=%f", result.Statistic)
	}
}

func TestPopulationStabilityIndex_EmptyInputs(t *testing.T) {
	result := PopulationStabilityIndex(nil, []float64{1}, 10, 0.2)
	if !result.Passed {
		t.Error("expected pass for empty input")
	}
}

func TestChiSquaredTest_MatchingCategories(t *testing.T) {
	obs := map[string]int{"a": 50, "b": 50, "c": 50}
	exp := map[string]int{"a": 50, "b": 50, "c": 50}

	result := ChiSquaredTest(obs, exp, 10.0)
	if !result.Passed {
		t.Errorf("expected matching distributions to pass, chi2=%f", result.Statistic)
	}
	if result.Statistic > 0.001 {
		t.Errorf("expected near-zero statistic, got %f", result.Statistic)
	}
}

func TestChiSquaredTest_DifferentCategories(t *testing.T) {
	obs := map[string]int{"a": 100, "b": 10, "c": 10}
	exp := map[string]int{"a": 10, "b": 10, "c": 100}

	result := ChiSquaredTest(obs, exp, 10.0)
	if result.Passed {
		t.Errorf("expected different distributions to fail, chi2=%f", result.Statistic)
	}
}

func TestChiSquaredTest_EmptyMaps(t *testing.T) {
	result := ChiSquaredTest(map[string]int{}, map[string]int{"a": 5}, 10.0)
	if !result.Passed {
		t.Error("expected pass for empty observed")
	}
}

func TestChiSquaredTest_DisjointCategories(t *testing.T) {
	obs := map[string]int{"x": 50}
	exp := map[string]int{"y": 50}

	result := ChiSquaredTest(obs, exp, 10.0)
	if result.Statistic == 0 {
		t.Error("expected non-zero statistic for disjoint categories")
	}
}

func TestJensenShannonDivergence_SimilarDistributions(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	a := make([]float64, 1000)
	b := make([]float64, 1000)
	for i := range a {
		a[i] = rng.NormFloat64()
		b[i] = rng.NormFloat64()
	}

	result := JensenShannonDivergence(a, b, 10, 0.2)
	if !result.Passed {
		t.Errorf("expected similar distributions to pass, JSD=%f", result.Statistic)
	}
}

func TestJensenShannonDivergence_DifferentDistributions(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	a := make([]float64, 1000)
	b := make([]float64, 1000)
	for i := range a {
		a[i] = rng.NormFloat64()
		b[i] = rng.NormFloat64() + 10.0
	}

	result := JensenShannonDivergence(a, b, 10, 0.1)
	if result.Passed {
		t.Errorf("expected different distributions to fail, JSD=%f", result.Statistic)
	}
}

func TestJensenShannonDivergence_EmptyInputs(t *testing.T) {
	result := JensenShannonDivergence(nil, []float64{1}, 10, 0.1)
	if !result.Passed {
		t.Error("expected pass for empty input")
	}
}

func TestCaptureDistribution_Correctness(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	snap := CaptureDistribution(values)

	if snap.Count != 10 {
		t.Errorf("expected count=10, got %d", snap.Count)
	}
	if snap.Min != 1.0 {
		t.Errorf("expected min=1, got %f", snap.Min)
	}
	if snap.Max != 10.0 {
		t.Errorf("expected max=10, got %f", snap.Max)
	}
	expectedMean := 5.5
	if math.Abs(snap.Mean-expectedMean) > 0.001 {
		t.Errorf("expected mean=%f, got %f", expectedMean, snap.Mean)
	}
	if snap.Variance <= 0 {
		t.Error("expected positive variance")
	}
	if snap.P50 < 5.0 || snap.P50 > 6.0 {
		t.Errorf("expected p50 near 5.5, got %f", snap.P50)
	}
	if snap.P90 < 9.0 {
		t.Errorf("expected p90 >= 9, got %f", snap.P90)
	}
	if snap.P95 < 9.0 {
		t.Errorf("expected p95 >= 9, got %f", snap.P95)
	}
	if len(snap.Histogram) != 10 {
		t.Errorf("expected 10 histogram bins, got %d", len(snap.Histogram))
	}
}

func TestCaptureDistribution_Empty(t *testing.T) {
	snap := CaptureDistribution(nil)
	if snap.Count != 0 {
		t.Errorf("expected count=0 for empty input, got %d", snap.Count)
	}
	if snap.Mean != 0 || snap.Variance != 0 {
		t.Error("expected zero mean and variance for empty input")
	}
}

func TestCaptureDistribution_SingleValue(t *testing.T) {
	snap := CaptureDistribution([]float64{42.0})
	if snap.Count != 1 {
		t.Errorf("expected count=1, got %d", snap.Count)
	}
	if snap.Mean != 42.0 {
		t.Errorf("expected mean=42, got %f", snap.Mean)
	}
	if snap.Variance != 0 {
		t.Errorf("expected variance=0, got %f", snap.Variance)
	}
	if snap.Skewness != 0 || snap.Kurtosis != 0 {
		t.Error("expected zero skewness/kurtosis for single value")
	}
}

func TestCaptureDistribution_IdenticalValues(t *testing.T) {
	values := []float64{5, 5, 5, 5, 5}
	snap := CaptureDistribution(values)
	if snap.Variance != 0 {
		t.Errorf("expected variance=0 for identical values, got %f", snap.Variance)
	}
	if snap.Min != snap.Max {
		t.Error("expected min == max for identical values")
	}
}

func TestCaptureDistribution_Skewness(t *testing.T) {
	// Right-skewed distribution
	values := make([]float64, 1000)
	rng := rand.New(rand.NewSource(123))
	for i := range values {
		values[i] = math.Abs(rng.NormFloat64())
	}
	snap := CaptureDistribution(values)
	if snap.Skewness <= 0 {
		t.Errorf("expected positive skewness for right-skewed data, got %f", snap.Skewness)
	}
}

func TestKolmogorovSmirnov_PValueRange(t *testing.T) {
	rng := rand.New(rand.NewSource(77))
	a := make([]float64, 100)
	b := make([]float64, 100)
	for i := range a {
		a[i] = rng.NormFloat64()
		b[i] = rng.NormFloat64()
	}

	result := KolmogorovSmirnov(a, b, 1.0)
	if result.PValue < 0 || result.PValue > 1 {
		t.Errorf("p-value out of range [0,1]: %f", result.PValue)
	}
}
