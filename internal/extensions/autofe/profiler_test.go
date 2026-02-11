package autofe

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateNormalLike produces values approximating a normal distribution using
// a simple Box-Muller-like approach (sum of uniform randoms via deterministic seed).
func generateNormalLike(n int, mean, stdDev float64) []float64 {
	vals := make([]float64, n)
	for i := range vals {
		// Approximate normal via central limit theorem: sum of 12 uniform [0,1)
		sum := 0.0
		seed := uint64(i*7 + 13)
		for j := 0; j < 12; j++ {
			seed = seed*6364136223846793005 + 1442695040888963407
			sum += float64(seed>>33) / float64(1<<31)
		}
		z := sum - 6.0 // approx standard normal
		vals[i] = mean + stdDev*z
	}
	// Ensure no NaN/Inf.
	for i, v := range vals {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			vals[i] = mean
		}
	}
	return vals
}

func TestProfiler_Profile(t *testing.T) {
	tests := []struct {
		name    string
		dataset *Dataset
		wantErr bool
	}{
		{
			name:    "nil dataset",
			dataset: nil,
			wantErr: true,
		},
		{
			name:    "empty columns",
			dataset: &Dataset{Columns: map[string]*ColumnStats{}},
			wantErr: true,
		},
		{
			name:    "valid dataset",
			dataset: testDataset(),
			wantErr: false,
		},
		{
			name: "single column",
			dataset: &Dataset{
				Rows: 50,
				Columns: map[string]*ColumnStats{
					"x": {
						Name: "x", DataType: "float64", Count: 50,
						Mean: 10, StdDev: 5, Min: 0, Max: 30,
						Values: generateValues(50, 10, 5),
					},
				},
			},
			wantErr: false,
		},
	}

	profiler := NewProfiler(DefaultProfilerConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := profiler.Profile(tt.dataset)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, report)
			assert.Equal(t, len(tt.dataset.Columns), report.TotalColumns)
			assert.Equal(t, tt.dataset.Rows, report.TotalRows)
			assert.Len(t, report.ColumnProfiles, len(tt.dataset.Columns))
			assert.Greater(t, report.DataQualityScore, float64(0))
		})
	}
}

func TestProfiler_ColumnProfile(t *testing.T) {
	profiler := NewProfiler(DefaultProfilerConfig())
	report, err := profiler.Profile(testDataset())
	require.NoError(t, err)

	for _, cp := range report.ColumnProfiles {
		assert.NotEmpty(t, cp.Name)
		assert.NotEmpty(t, cp.DataType)
		assert.NotEmpty(t, cp.DistributionType)
		assert.GreaterOrEqual(t, cp.UniquenessRatio, float64(0))
		assert.LessOrEqual(t, cp.MissingRatio, float64(1))
	}
}

func TestProfiler_Correlations(t *testing.T) {
	profiler := NewProfiler(DefaultProfilerConfig())
	report, err := profiler.Profile(testDataset())
	require.NoError(t, err)

	assert.NotEmpty(t, report.Correlations)
	for _, corr := range report.Correlations {
		assert.InDelta(t, 0, corr, 1.01) // allow small floating-point overshoot
	}
}

func TestProfiler_HighMissing(t *testing.T) {
	profiler := NewProfiler(DefaultProfilerConfig())
	ds := &Dataset{
		Rows: 100,
		Columns: map[string]*ColumnStats{
			"sparse": {
				Name: "sparse", DataType: "float64",
				Count: 100, NullCount: 50,
				Mean: 5, StdDev: 2, Min: 0, Max: 10,
				Values: generateValues(100, 5, 2),
			},
		},
	}
	report, err := profiler.Profile(ds)
	require.NoError(t, err)

	assert.Equal(t, 0.5, report.ColumnProfiles[0].MissingRatio)
	found := false
	for _, rec := range report.Recommendations {
		if len(rec) > 0 {
			found = true
		}
	}
	assert.True(t, found, "expected recommendations for high missing data")
}

func TestDetectDistribution(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   string
	}{
		{name: "too few values", values: []float64{1, 2}, want: "unknown"},
		{name: "empty", values: nil, want: "unknown"},
		{name: "constant", values: []float64{5, 5, 5, 5, 5}, want: "uniform"},
		{name: "normal-like", values: generateNormalLike(1000, 50, 10), want: "normal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectDistribution(tt.values)
			assert.Equal(t, tt.want, got)
		})
	}
}
