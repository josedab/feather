package autofe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDataset() *Dataset {
	return &Dataset{
		Target: "target",
		Rows:   100,
		Columns: map[string]*ColumnStats{
			"clicks": {
				Name: "clicks", DataType: "float64", Count: 100,
				Mean: 50, StdDev: 25, Min: 1, Max: 200, Skewness: 1.5,
				Values: generateValues(100, 50, 25),
			},
			"revenue": {
				Name: "revenue", DataType: "float64", Count: 100,
				Mean: 100, StdDev: 50, Min: 0.5, Max: 500, Skewness: 0.7,
				Values: generateValues(100, 100, 50),
			},
			"age": {
				Name: "age", DataType: "int64", Count: 100,
				Mean: 35, StdDev: 10, Min: 18, Max: 80, Skewness: 0.3,
				Values: generateValues(100, 35, 10),
			},
			"target": {
				Name: "target", DataType: "float64", Count: 100,
				Mean: 0.5, StdDev: 0.3, Min: 0, Max: 1,
				Values: generateValues(100, 0.5, 0.3),
			},
		},
	}
}

func generateValues(n int, mean, stdDev float64) []float64 {
	vals := make([]float64, n)
	for i := range vals {
		vals[i] = mean + stdDev*float64(i-n/2)/float64(n)
	}
	return vals
}

func TestEngine_Generate(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	dataset := testDataset()

	candidates, err := engine.Generate(dataset)
	require.NoError(t, err)
	assert.NotEmpty(t, candidates)

	// Should generate log, zscore, minmax, sqrt, bin, interaction, ratio transforms
	transforms := make(map[TransformType]bool)
	for _, c := range candidates {
		transforms[c.Transform] = true
	}
	assert.True(t, transforms[TransformLog], "should generate log transforms")
	assert.True(t, transforms[TransformZScore], "should generate z-score transforms")
	assert.True(t, transforms[TransformInteraction], "should generate interactions")
	assert.True(t, transforms[TransformBin], "should generate bins")
}

func TestEngine_GenerateNilDataset(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	_, err := engine.Generate(nil)
	require.Error(t, err)
}

func TestEngine_GenerateNoInteractions(t *testing.T) {
	cfg := DefaultEngineConfig()
	cfg.EnableInteractions = false
	engine := NewEngine(cfg)

	candidates, err := engine.Generate(testDataset())
	require.NoError(t, err)

	for _, c := range candidates {
		assert.NotEqual(t, TransformInteraction, c.Transform)
		assert.NotEqual(t, TransformRatio, c.Transform)
	}
}

func TestEngine_TopN(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	_, _ = engine.Generate(testDataset())

	top5 := engine.TopN(5)
	assert.Len(t, top5, 5)

	// Verify sorted by score descending
	for i := 1; i < len(top5); i++ {
		assert.GreaterOrEqual(t, top5[i-1].Score, top5[i].Score)
	}
}

func TestEngine_MaxCandidates(t *testing.T) {
	cfg := DefaultEngineConfig()
	cfg.MaxCandidates = 5
	engine := NewEngine(cfg)

	candidates, err := engine.Generate(testDataset())
	require.NoError(t, err)
	assert.LessOrEqual(t, len(candidates), 5)
}

func TestEngine_Stats(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	_, _ = engine.Generate(testDataset())

	stats := engine.Stats()
	assert.Greater(t, stats["total_candidates"], 0)
}

func TestComputeColumnStats(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	stats := ComputeColumnStats("test", "float64", values)

	assert.Equal(t, "test", stats.Name)
	assert.Equal(t, 5, stats.Count)
	assert.InDelta(t, 3.0, stats.Mean, 0.001)
	assert.InDelta(t, 1.0, stats.Min, 0.001)
	assert.InDelta(t, 5.0, stats.Max, 0.001)
	assert.Greater(t, stats.StdDev, float64(0))
}

func TestComputeColumnStats_Empty(t *testing.T) {
	stats := ComputeColumnStats("empty", "float64", nil)
	assert.Equal(t, 0, stats.Count)
}

func TestPearsonCorrelation(t *testing.T) {
	// Perfect positive correlation
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{2, 4, 6, 8, 10}
	assert.InDelta(t, 1.0, pearsonCorrelation(x, y), 0.001)

	// Perfect negative correlation
	y2 := []float64{10, 8, 6, 4, 2}
	assert.InDelta(t, -1.0, pearsonCorrelation(x, y2), 0.001)

	// No correlation
	assert.Equal(t, float64(0), pearsonCorrelation(nil, nil))
}

func TestCandidateScoring(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	candidates, _ := engine.Generate(testDataset())

	for _, c := range candidates {
		assert.GreaterOrEqual(t, c.Score, float64(0))
		assert.LessOrEqual(t, c.Score, float64(1))
	}
}
