package federation

import (
	"math"
	"testing"
)

func TestDPMechanism_AddNoise(t *testing.T) {
	dp := NewDPMechanism(DefaultDPConfig())

	original := 0.5
	noisyCount := 0
	for i := 0; i < 100; i++ {
		noisy, err := dp.AddNoise(original)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if noisy != original {
			noisyCount++
		}
	}

	if noisyCount < 90 {
		t.Fatalf("expected noise in most cases, only %d/100 had noise", noisyCount)
	}
}

func TestDPMechanism_AddNoiseLaplace(t *testing.T) {
	config := DefaultDPConfig()
	config.NoiseType = "laplace"
	dp := NewDPMechanism(config)

	original := 0.5
	noisyCount := 0
	for i := 0; i < 100; i++ {
		noisy, err := dp.AddNoise(original)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if noisy != original {
			noisyCount++
		}
	}

	if noisyCount < 90 {
		t.Fatalf("expected noise in most cases, only %d/100 had noise", noisyCount)
	}
}

func TestDPMechanism_AddNoiseVector(t *testing.T) {
	dp := NewDPMechanism(DefaultDPConfig())

	values := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	noisy, err := dp.AddNoiseVector(values)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(noisy) != len(values) {
		t.Fatalf("expected %d values, got %d", len(values), len(noisy))
	}

	// At least some should differ
	diffCount := 0
	for i := range values {
		if noisy[i] != values[i] {
			diffCount++
		}
	}
	if diffCount == 0 {
		t.Fatal("expected at least some noisy values to differ")
	}
}

func TestDPMechanism_ClipValue(t *testing.T) {
	dp := NewDPMechanism(DefaultDPConfig()) // bounds [-1, 1]

	tests := []struct {
		input    float64
		expected float64
	}{
		{0.5, 0.5},
		{-0.5, -0.5},
		{2.0, 1.0},
		{-2.0, -1.0},
		{1.0, 1.0},
		{-1.0, -1.0},
	}

	for _, tt := range tests {
		result := dp.ClipValue(tt.input)
		if result != tt.expected {
			t.Errorf("ClipValue(%f) = %f, want %f", tt.input, result, tt.expected)
		}
	}
}

func TestDPMechanism_ClipGradient(t *testing.T) {
	config := DefaultDPConfig()
	config.MaxGradientNorm = 1.0
	dp := NewDPMechanism(config)

	// Gradient within norm should be unchanged
	small := []float64{0.3, 0.4} // norm = 0.5
	clipped := dp.ClipGradient(small)
	if math.Abs(clipped[0]-0.3) > 1e-10 || math.Abs(clipped[1]-0.4) > 1e-10 {
		t.Errorf("small gradient should not be clipped: got %v", clipped)
	}

	// Gradient exceeding norm should be scaled down
	large := []float64{3.0, 4.0} // norm = 5.0
	clipped = dp.ClipGradient(large)
	norm := math.Sqrt(clipped[0]*clipped[0] + clipped[1]*clipped[1])
	if math.Abs(norm-1.0) > 1e-10 {
		t.Errorf("clipped gradient norm should be 1.0, got %f", norm)
	}
}

func TestDPMechanism_ClipGradient_Empty(t *testing.T) {
	dp := NewDPMechanism(DefaultDPConfig())
	result := dp.ClipGradient([]float64{})
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestDefaultDPConfig(t *testing.T) {
	config := DefaultDPConfig()

	if config.Epsilon != 1.0 {
		t.Errorf("expected Epsilon 1.0, got %f", config.Epsilon)
	}
	if config.Delta != 1e-5 {
		t.Errorf("expected Delta 1e-5, got %e", config.Delta)
	}
	if config.NoiseType != "gaussian" {
		t.Errorf("expected NoiseType gaussian, got %s", config.NoiseType)
	}
	if config.MaxGradientNorm != 1.0 {
		t.Errorf("expected MaxGradientNorm 1.0, got %f", config.MaxGradientNorm)
	}
}

func TestDPBudget_Consume(t *testing.T) {
	budget := NewDPBudget(5.0, 1e-3)

	err := budget.Consume(2.0, 1e-4)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	remEps, remDelta := budget.Remaining()
	if math.Abs(remEps-3.0) > 1e-10 {
		t.Errorf("expected remaining epsilon 3.0, got %f", remEps)
	}
	if math.Abs(remDelta-(1e-3-1e-4)) > 1e-15 {
		t.Errorf("expected remaining delta %e, got %e", 1e-3-1e-4, remDelta)
	}
}

func TestDPBudget_Exceed(t *testing.T) {
	budget := NewDPBudget(2.0, 1e-3)

	err := budget.Consume(1.5, 1e-4)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	err = budget.Consume(1.0, 1e-4)
	if err == nil {
		t.Fatal("expected error when exceeding budget")
	}
}

func TestDPBudget_IsExhausted(t *testing.T) {
	budget := NewDPBudget(2.0, 1e-3)

	if budget.IsExhausted() {
		t.Fatal("budget should not be exhausted initially")
	}

	_ = budget.Consume(2.0, 0)
	if !budget.IsExhausted() {
		t.Fatal("budget should be exhausted")
	}
}

func TestDPBudget_Reset(t *testing.T) {
	budget := NewDPBudget(5.0, 1e-3)
	_ = budget.Consume(3.0, 5e-4)

	budget.Reset()

	remEps, remDelta := budget.Remaining()
	if remEps != 5.0 {
		t.Errorf("expected full epsilon after reset, got %f", remEps)
	}
	if remDelta != 1e-3 {
		t.Errorf("expected full delta after reset, got %e", remDelta)
	}

	if budget.IsExhausted() {
		t.Fatal("budget should not be exhausted after reset")
	}
}

func TestPrivacyAccountant_Track(t *testing.T) {
	budget := NewDPBudget(10.0, 1e-3)
	pa := NewPrivacyAccountant(budget)

	pa.Track("q-1", 1.0, 1e-5)
	pa.Track("q-2", 0.5, 1e-5)

	report := pa.GetReport()
	if report.TotalQueries != 2 {
		t.Errorf("expected 2 queries, got %d", report.TotalQueries)
	}
	if math.Abs(report.TotalEpsilon-1.5) > 1e-10 {
		t.Errorf("expected total epsilon 1.5, got %f", report.TotalEpsilon)
	}
	if len(report.Queries) != 2 {
		t.Errorf("expected 2 query records, got %d", len(report.Queries))
	}
}

func TestPrivacyAccountant_EmptyReport(t *testing.T) {
	budget := NewDPBudget(10.0, 1e-3)
	pa := NewPrivacyAccountant(budget)

	report := pa.GetReport()
	if report.TotalQueries != 0 {
		t.Errorf("expected 0 queries, got %d", report.TotalQueries)
	}
	if report.TotalEpsilon != 0 {
		t.Errorf("expected 0 epsilon, got %f", report.TotalEpsilon)
	}
}
