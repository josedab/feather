package diffprivacy

import (
	"math"
	"testing"
)

func defaultFeatureConfig() FeaturePrivacyConfig {
	return FeaturePrivacyConfig{
		Epsilon:     1.0,
		Delta:       1e-5,
		Mechanism:   MechanismLaplace,
		Sensitivity: 1.0,
		MaxBudget:   10.0,
	}
}

func TestNewEngine(t *testing.T) {
	e := NewEngine(DefaultConfig())
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	stats := e.Stats()
	if stats.RegisteredFeatures != 0 {
		t.Errorf("RegisteredFeatures = %d, want 0", stats.RegisteredFeatures)
	}
}

func TestRegisterFeature(t *testing.T) {
	tests := []struct {
		name    string
		fName   string
		cfg     FeaturePrivacyConfig
		wantErr bool
	}{
		{
			name:  "valid laplace",
			fName: "clicks",
			cfg:   defaultFeatureConfig(),
		},
		{
			name:  "valid gaussian",
			fName: "views",
			cfg: FeaturePrivacyConfig{
				Epsilon: 1.0, Delta: 1e-5, Mechanism: MechanismGaussian,
				Sensitivity: 1.0, MaxBudget: 10.0,
			},
		},
		{
			name:    "empty name",
			fName:   "",
			cfg:     defaultFeatureConfig(),
			wantErr: true,
		},
		{
			name:    "zero epsilon",
			fName:   "bad",
			cfg:     FeaturePrivacyConfig{Epsilon: 0, Sensitivity: 1.0},
			wantErr: true,
		},
		{
			name:    "zero sensitivity",
			fName:   "bad",
			cfg:     FeaturePrivacyConfig{Epsilon: 1.0, Sensitivity: 0},
			wantErr: true,
		},
		{
			name:    "gaussian without delta",
			fName:   "bad",
			cfg:     FeaturePrivacyConfig{Epsilon: 1.0, Sensitivity: 1.0, Mechanism: MechanismGaussian, Delta: 0},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEngine(DefaultConfig())
			err := e.RegisterFeature(tt.fName, tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// duplicate registration should fail
			if err := e.RegisterFeature(tt.fName, tt.cfg); err == nil {
				t.Error("expected error for duplicate registration")
			}
		})
	}
}

func TestAddNoise(t *testing.T) {
	tests := []struct {
		name      string
		mechanism Mechanism
		value     float64
	}{
		{"laplace", MechanismLaplace, 100.0},
		{"gaussian", MechanismGaussian, 50.0},
		{"local_dp", MechanismLocalDP, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEngine(DefaultConfig())
			cfg := FeaturePrivacyConfig{
				Epsilon: 1.0, Delta: 1e-5, Mechanism: tt.mechanism,
				Sensitivity: 1.0, MaxBudget: 10.0,
			}
			if err := e.RegisterFeature("f", cfg); err != nil {
				t.Fatal(err)
			}

			noisy, err := e.AddNoise("f", tt.value)
			if err != nil {
				t.Fatalf("AddNoise: %v", err)
			}
			// For laplace/gaussian, noise should differ from original (statistically)
			// For local_dp, output is 0 or 1
			if tt.mechanism == MechanismLocalDP {
				if noisy != 0.0 && noisy != 1.0 {
					t.Errorf("local_dp should return 0 or 1, got %f", noisy)
				}
			}
		})
	}
}

func TestAddNoise_UnregisteredFeature(t *testing.T) {
	e := NewEngine(DefaultConfig())
	_, err := e.AddNoise("nonexistent", 42.0)
	if err == nil {
		t.Fatal("expected error for unregistered feature")
	}
}

func TestNoisyCount(t *testing.T) {
	e := NewEngine(DefaultConfig())
	if err := e.RegisterFeature("f", defaultFeatureConfig()); err != nil {
		t.Fatal(err)
	}

	agg, err := e.NoisyCount("f", 1000)
	if err != nil {
		t.Fatalf("NoisyCount: %v", err)
	}
	if agg.OriginalValue != 1000.0 {
		t.Errorf("OriginalValue = %f, want 1000", agg.OriginalValue)
	}
	if agg.Mechanism != MechanismLaplace {
		t.Errorf("Mechanism = %s, want %s", agg.Mechanism, MechanismLaplace)
	}
}

func TestNoisySum(t *testing.T) {
	e := NewEngine(DefaultConfig())
	if err := e.RegisterFeature("f", defaultFeatureConfig()); err != nil {
		t.Fatal(err)
	}

	agg, err := e.NoisySum("f", 500.0)
	if err != nil {
		t.Fatalf("NoisySum: %v", err)
	}
	if agg.OriginalValue != 500.0 {
		t.Errorf("OriginalValue = %f, want 500", agg.OriginalValue)
	}
	if agg.EpsilonUsed <= 0 {
		t.Error("EpsilonUsed should be positive")
	}
}

func TestNoisyAvg(t *testing.T) {
	tests := []struct {
		name    string
		sum     float64
		count   int64
		wantErr bool
	}{
		{"valid", 100.0, 10, false},
		{"zero count", 100.0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEngine(DefaultConfig())
			if err := e.RegisterFeature("f", defaultFeatureConfig()); err != nil {
				t.Fatal(err)
			}

			agg, err := e.NoisyAvg("f", tt.sum, tt.count)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("NoisyAvg: %v", err)
			}
			expectedOriginal := tt.sum / float64(tt.count)
			if agg.OriginalValue != expectedOriginal {
				t.Errorf("OriginalValue = %f, want %f", agg.OriginalValue, expectedOriginal)
			}
		})
	}
}

func TestBudgetStatus(t *testing.T) {
	e := NewEngine(DefaultConfig())
	cfg := FeaturePrivacyConfig{
		Epsilon: 1.0, Delta: 1e-5, Mechanism: MechanismLaplace,
		Sensitivity: 1.0, MaxBudget: 3.0,
	}
	if err := e.RegisterFeature("f", cfg); err != nil {
		t.Fatal(err)
	}

	info, err := e.BudgetStatus("f")
	if err != nil {
		t.Fatalf("BudgetStatus: %v", err)
	}
	if info.TotalEpsilon != 3.0 {
		t.Errorf("TotalEpsilon = %f, want 3.0", info.TotalEpsilon)
	}
	initialRemaining := info.QueriesRemaining

	// Consume some budget
	if _, err := e.AddNoise("f", 42.0); err != nil {
		t.Fatal(err)
	}

	info2, _ := e.BudgetStatus("f")
	if info2.ConsumedEpsilon <= 0 {
		t.Error("ConsumedEpsilon should be positive after AddNoise")
	}
	if info2.QueriesRemaining >= initialRemaining {
		t.Error("QueriesRemaining should decrease after AddNoise")
	}
}

func TestBudgetExhaustion(t *testing.T) {
	e := NewEngine(DefaultConfig())
	cfg := FeaturePrivacyConfig{
		Epsilon: 1.0, Delta: 1e-5, Mechanism: MechanismLaplace,
		Sensitivity: 1.0, MaxBudget: 2.0,
	}
	if err := e.RegisterFeature("f", cfg); err != nil {
		t.Fatal(err)
	}

	// Should succeed twice (budget = 2.0, each query costs 1.0)
	if _, err := e.AddNoise("f", 1.0); err != nil {
		t.Fatalf("first AddNoise: %v", err)
	}
	if _, err := e.AddNoise("f", 1.0); err != nil {
		t.Fatalf("second AddNoise: %v", err)
	}
	// Third should fail
	if _, err := e.AddNoise("f", 1.0); err == nil {
		t.Error("expected budget exhaustion error")
	}

	stats := e.Stats()
	if stats.BudgetExhaustions < 1 {
		t.Errorf("BudgetExhaustions = %d, want >= 1", stats.BudgetExhaustions)
	}
}

func TestStats(t *testing.T) {
	e := NewEngine(DefaultConfig())
	if err := e.RegisterFeature("f", defaultFeatureConfig()); err != nil {
		t.Fatal(err)
	}
	if _, err := e.NoisyCount("f", 10); err != nil {
		t.Fatal(err)
	}

	stats := e.Stats()
	if stats.RegisteredFeatures != 1 {
		t.Errorf("RegisteredFeatures = %d, want 1", stats.RegisteredFeatures)
	}
	if stats.TotalQueries != 1 {
		t.Errorf("TotalQueries = %d, want 1", stats.TotalQueries)
	}
	if stats.MechanismCounts[MechanismLaplace] != 1 {
		t.Errorf("MechanismCounts[laplace] = %d, want 1", stats.MechanismCounts[MechanismLaplace])
	}
}

func TestSequentialComposition(t *testing.T) {
	eps, delta := SequentialComposition(1.0, 1e-5, 5)
	if eps != 5.0 {
		t.Errorf("totalEpsilon = %f, want 5.0", eps)
	}
	if math.Abs(delta-5e-5) > 1e-10 {
		t.Errorf("totalDelta = %e, want 5e-5", delta)
	}
}

func TestParallelComposition(t *testing.T) {
	eps, delta := ParallelComposition(1.0, 1e-5)
	if eps != 1.0 {
		t.Errorf("totalEpsilon = %f, want 1.0", eps)
	}
	if delta != 1e-5 {
		t.Errorf("totalDelta = %e, want 1e-5", delta)
	}
}

func TestRenyiComposition(t *testing.T) {
	eps, delta := RenyiComposition(1.0, 1e-5, 2.0, 10)
	if eps <= 0 {
		t.Errorf("totalEpsilon should be positive, got %f", eps)
	}
	if delta != 1e-5 {
		t.Errorf("totalDelta = %e, want 1e-5", delta)
	}
}
