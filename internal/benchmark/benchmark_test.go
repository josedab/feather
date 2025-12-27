package benchmark

import (
	"context"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestStore(t testing.TB) *storage.Store {
	t.Helper()
	store, err := storage.NewStore(storage.StoreOptions{
		HotMaxSize:   100 * 1024 * 1024, // 100MB
		WarmInMemory: true,
	}, nil)
	require.NoError(t, err)
	return store
}

func TestNewSuite(t *testing.T) {
	store := createTestStore(t)
	config := DefaultConfig()
	config.NumEntities = 100
	config.NumOperations = 100
	config.Concurrency = 2

	suite := NewSuite(store, config)

	assert.NotNil(t, suite)
	assert.Equal(t, config.NumEntities, suite.config.NumEntities)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 10000, config.NumEntities)
	assert.Equal(t, 10, config.NumFeatures)
	assert.Equal(t, 100000, config.NumOperations)
	assert.Equal(t, 10, config.Concurrency)
	assert.Equal(t, 5*time.Second, config.WarmupDuration)
	assert.Equal(t, 100, config.DataSize)
}

func TestSuite_Run(t *testing.T) {
	store := createTestStore(t)
	config := DefaultConfig()
	config.NumEntities = 100
	config.NumFeatures = 5
	config.NumOperations = 100
	config.Concurrency = 2
	config.WarmupDuration = 100 * time.Millisecond
	config.DataSize = 50

	suite := NewSuite(store, config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := suite.Run(ctx)
	require.NoError(t, err)

	results := suite.GetResults()
	assert.NotEmpty(t, results)

	// Check that we have results for expected benchmarks
	expectedBenchmarks := []string{"Write", "Read", "BatchRead", "MixedWorkload", "ConcurrentWrite", "PointInTimeRead"}
	for _, name := range expectedBenchmarks {
		result, ok := results[name]
		assert.True(t, ok, "expected result for %s", name)
		if ok {
			assert.Greater(t, result.TotalOps, int64(0), "expected ops for %s", name)
			assert.Greater(t, result.OpsPerSecond, float64(0), "expected ops/sec for %s", name)
		}
	}
}

func TestSuite_GetResultsJSON(t *testing.T) {
	store := createTestStore(t)
	config := DefaultConfig()
	config.NumEntities = 50
	config.NumFeatures = 3
	config.NumOperations = 50
	config.Concurrency = 2
	config.WarmupDuration = 0
	config.DataSize = 20

	suite := NewSuite(store, config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := suite.Run(ctx)
	require.NoError(t, err)

	jsonData, err := suite.GetResultsJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)
	assert.Contains(t, string(jsonData), "ops_per_second")
	assert.Contains(t, string(jsonData), "latency_p50")
}

func TestSuite_ContextCancellation(t *testing.T) {
	store := createTestStore(t)
	config := DefaultConfig()
	config.NumEntities = 10000
	config.NumOperations = 1000000
	config.WarmupDuration = 0

	suite := NewSuite(store, config)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	err := suite.Run(ctx)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestCalculateResult(t *testing.T) {
	store := createTestStore(t)
	suite := NewSuite(store, DefaultConfig())

	latencies := []time.Duration{
		1 * time.Microsecond,
		2 * time.Microsecond,
		3 * time.Microsecond,
		4 * time.Microsecond,
		5 * time.Microsecond,
		6 * time.Microsecond,
		7 * time.Microsecond,
		8 * time.Microsecond,
		9 * time.Microsecond,
		10 * time.Microsecond,
	}

	result := suite.calculateResult("Test", latencies, time.Second, 1000, 0)

	assert.Equal(t, "Test", result.Name)
	assert.Equal(t, int64(10), result.TotalOps)
	assert.Equal(t, float64(10), result.OpsPerSecond)
	assert.Equal(t, 1*time.Microsecond, result.LatencyMin)
	assert.Equal(t, 10*time.Microsecond, result.LatencyMax)
	assert.Equal(t, int64(0), result.Errors)
}

func TestCalculateResult_EmptyLatencies(t *testing.T) {
	store := createTestStore(t)
	suite := NewSuite(store, DefaultConfig())

	result := suite.calculateResult("Test", []time.Duration{}, time.Second, 0, 5)

	assert.Equal(t, "Test", result.Name)
	assert.Equal(t, int64(0), result.TotalOps)
	assert.Equal(t, int64(5), result.Errors)
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		n        int64
		expected string
	}{
		{500, "500"},
		{1000, "1.00K"},
		{1500, "1.50K"},
		{1000000, "1.00M"},
		{2500000, "2.50M"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatNumber(tt.n)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func BenchmarkWrite(b *testing.B) {
	store := createTestStore(b)
	config := DefaultConfig()
	config.NumEntities = 1000
	config.NumOperations = b.N
	config.Concurrency = 1
	config.WarmupDuration = 0

	suite := NewSuite(store, config)

	// Seed data
	ctx := context.Background()
	if err := suite.seedData(ctx); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	_, err := suite.benchmarkWrite(ctx)
	if err != nil {
		b.Fatal(err)
	}
}

func BenchmarkRead(b *testing.B) {
	store := createTestStore(b)
	config := DefaultConfig()
	config.NumEntities = 1000
	config.NumOperations = b.N
	config.Concurrency = 1
	config.WarmupDuration = 0

	suite := NewSuite(store, config)

	// Seed data
	ctx := context.Background()
	if err := suite.seedData(ctx); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	_, err := suite.benchmarkRead(ctx)
	if err != nil {
		b.Fatal(err)
	}
}
