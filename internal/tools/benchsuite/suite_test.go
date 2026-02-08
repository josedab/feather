package benchsuite

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultSuiteConfig(t *testing.T) {
	cfg := DefaultSuiteConfig()

	assert.Equal(t, 5*time.Second, cfg.Warmup)
	assert.Equal(t, 30*time.Second, cfg.Duration)
	assert.Equal(t, 10, cfg.Concurrency)
	assert.Equal(t, 100, cfg.NumFeatures)
	assert.Equal(t, 10000, cfg.NumEntities)
	assert.Equal(t, "json", cfg.ReportFormat)
}

func TestNewSuite(t *testing.T) {
	cfg := DefaultSuiteConfig()
	s := NewSuite(cfg)

	assert.NotNil(t, s)
	assert.Equal(t, cfg, s.config)
	assert.NotNil(t, s.benchmarks)
}

func TestCreateRun(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	run, err := s.CreateRun("test-read", WorkloadReadHeavy)
	require.NoError(t, err)
	assert.Equal(t, "bench-1", run.ID)
	assert.Equal(t, "test-read", run.Name)
	assert.Equal(t, WorkloadReadHeavy, run.Workload)
	assert.Equal(t, StatusPending, run.Status)
}

func TestCreateRun_EmptyName(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	_, err := s.CreateRun("", WorkloadReadHeavy)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestCreateRun_InvalidWorkload(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	_, err := s.CreateRun("test", WorkloadType("invalid"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid workload type")
}

func TestRunBenchmark_AllWorkloads(t *testing.T) {
	workloads := []WorkloadType{
		WorkloadReadHeavy,
		WorkloadWriteHeavy,
		WorkloadMixed,
		WorkloadBurst,
		WorkloadLatency,
		WorkloadThroughput,
	}

	for _, wl := range workloads {
		t.Run(string(wl), func(t *testing.T) {
			cfg := DefaultSuiteConfig()
			cfg.Duration = 1 * time.Second
			cfg.Concurrency = 2
			s := NewSuite(cfg)

			run, err := s.CreateRun("bench-"+string(wl), wl)
			require.NoError(t, err)

			results, err := s.RunBenchmark(run.ID)
			require.NoError(t, err)
			require.NotNil(t, results)

			// Verify basic result sanity
			assert.Greater(t, results.TotalOps, int64(0))
			assert.Greater(t, results.OpsPerSecond, float64(0))
			assert.Equal(t, results.ReadOps+results.WriteOps, results.TotalOps)
			assert.GreaterOrEqual(t, results.Errors, int64(0))

			// Verify latency stats ordering: P50 <= P95 <= P99 <= P999 <= Max
			assert.LessOrEqual(t, results.Latency.Min, results.Latency.P50)
			assert.LessOrEqual(t, results.Latency.P50, results.Latency.P95)
			assert.LessOrEqual(t, results.Latency.P95, results.Latency.P99)
			assert.LessOrEqual(t, results.Latency.P99, results.Latency.P999)
			assert.LessOrEqual(t, results.Latency.P999, results.Latency.Max)
			assert.Greater(t, results.Latency.P99, results.Latency.P50)

			// Read latency should be populated if read ops exist
			if results.ReadOps > 0 {
				assert.Greater(t, results.ReadLatency.P50, time.Duration(0))
				assert.LessOrEqual(t, results.ReadLatency.P50, results.ReadLatency.P99)
			}
			if results.WriteOps > 0 {
				assert.Greater(t, results.WriteLatency.P50, time.Duration(0))
				assert.LessOrEqual(t, results.WriteLatency.P50, results.WriteLatency.P99)
			}

			// Throughput
			assert.Greater(t, results.Throughput.AvgOpsPerSec, float64(0))
			assert.Greater(t, results.Throughput.PeakOpsPerSec, float64(0))

			// Run should be marked completed
			saved, err := s.GetRun(run.ID)
			require.NoError(t, err)
			assert.Equal(t, StatusCompleted, saved.Status)
			assert.NotNil(t, saved.Results)
		})
	}
}

func TestRunBenchmark_ReadHeavyDistribution(t *testing.T) {
	cfg := DefaultSuiteConfig()
	cfg.Duration = 1 * time.Second
	cfg.Concurrency = 2
	s := NewSuite(cfg)

	run, err := s.CreateRun("read-heavy-test", WorkloadReadHeavy)
	require.NoError(t, err)

	results, err := s.RunBenchmark(run.ID)
	require.NoError(t, err)

	// 90% reads
	assert.Greater(t, results.ReadOps, results.WriteOps)
	readRatio := float64(results.ReadOps) / float64(results.TotalOps)
	assert.InDelta(t, 0.9, readRatio, 0.01)
}

func TestRunBenchmark_WriteHeavyDistribution(t *testing.T) {
	cfg := DefaultSuiteConfig()
	cfg.Duration = 1 * time.Second
	cfg.Concurrency = 2
	s := NewSuite(cfg)

	run, err := s.CreateRun("write-heavy-test", WorkloadWriteHeavy)
	require.NoError(t, err)

	results, err := s.RunBenchmark(run.ID)
	require.NoError(t, err)

	// 90% writes
	assert.Greater(t, results.WriteOps, results.ReadOps)
	writeRatio := float64(results.WriteOps) / float64(results.TotalOps)
	assert.InDelta(t, 0.9, writeRatio, 0.01)
}

func TestRunBenchmark_NotFound(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	_, err := s.RunBenchmark("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetRun(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	run, err := s.CreateRun("test", WorkloadMixed)
	require.NoError(t, err)

	got, err := s.GetRun(run.ID)
	require.NoError(t, err)
	assert.Equal(t, run.ID, got.ID)
	assert.Equal(t, "test", got.Name)
}

func TestGetRun_NotFound(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	_, err := s.GetRun("nonexistent")
	assert.Error(t, err)
}

func TestListRuns(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	_, _ = s.CreateRun("run-a", WorkloadReadHeavy)
	_, _ = s.CreateRun("run-b", WorkloadMixed)
	_, _ = s.CreateRun("run-c", WorkloadLatency)

	runs := s.ListRuns()
	assert.Len(t, runs, 3)
	// Sorted by ID
	assert.Equal(t, "bench-1", runs[0].ID)
	assert.Equal(t, "bench-2", runs[1].ID)
	assert.Equal(t, "bench-3", runs[2].ID)
}

func TestDeleteRun(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	run, _ := s.CreateRun("to-delete", WorkloadMixed)
	err := s.DeleteRun(run.ID)
	require.NoError(t, err)

	_, err = s.GetRun(run.ID)
	assert.Error(t, err)
	assert.Len(t, s.ListRuns(), 0)
}

func TestDeleteRun_NotFound(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	err := s.DeleteRun("nonexistent")
	assert.Error(t, err)
}

func TestCompare(t *testing.T) {
	cfg := DefaultSuiteConfig()
	cfg.Duration = 1 * time.Second
	cfg.Concurrency = 2
	s := NewSuite(cfg)

	run1, _ := s.CreateRun("read-bench", WorkloadReadHeavy)
	run2, _ := s.CreateRun("write-bench", WorkloadWriteHeavy)
	run3, _ := s.CreateRun("mixed-bench", WorkloadMixed)

	_, _ = s.RunBenchmark(run1.ID)
	_, _ = s.RunBenchmark(run2.ID)
	_, _ = s.RunBenchmark(run3.ID)

	report, err := s.Compare([]string{run1.ID, run2.ID, run3.ID})
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Len(t, report.Runs, 3)
	assert.Len(t, report.Rankings, 3)

	// Rankings should be ordered by score descending
	for i := 0; i < len(report.Rankings)-1; i++ {
		assert.GreaterOrEqual(t, report.Rankings[i].Score, report.Rankings[i+1].Score)
	}

	// Rank values should be sequential
	for i, r := range report.Rankings {
		assert.Equal(t, i+1, r.Rank)
		assert.Greater(t, r.Score, float64(0))
		assert.LessOrEqual(t, r.Score, float64(100))
	}

	// Summary should be populated
	assert.NotEmpty(t, report.Summary.FastestP99)
	assert.NotEmpty(t, report.Summary.HighestQPS)
	assert.NotEmpty(t, report.Summary.LowestErrors)
	assert.NotEmpty(t, report.Summary.BestOverall)
}

func TestCompare_TooFewRuns(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	run, _ := s.CreateRun("solo", WorkloadMixed)

	_, err := s.Compare([]string{run.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least 2")
}

func TestCompare_IncompleteRun(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	run1, _ := s.CreateRun("pending", WorkloadMixed)
	run2, _ := s.CreateRun("also-pending", WorkloadMixed)

	_, err := s.Compare([]string{run1.ID, run2.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not completed")
}

func TestGenerateReport(t *testing.T) {
	cfg := DefaultSuiteConfig()
	cfg.Duration = 1 * time.Second
	cfg.Concurrency = 2
	s := NewSuite(cfg)

	run, _ := s.CreateRun("report-test", WorkloadReadHeavy)
	_, _ = s.RunBenchmark(run.ID)

	report, err := s.GenerateReport(run.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, report)

	// Report should contain key sections
	assert.Contains(t, report, "report-test")
	assert.Contains(t, report, "read_heavy")
	assert.Contains(t, report, "Ops/sec")
	assert.Contains(t, report, "P99")
	assert.Contains(t, report, "Read Latency")
	assert.Contains(t, report, "Write Latency")
	assert.Contains(t, report, "Throughput")
}

func TestGenerateReport_NotFound(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	_, err := s.GenerateReport("nonexistent")
	assert.Error(t, err)
}

func TestGenerateReport_NoResults(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	run, _ := s.CreateRun("pending", WorkloadMixed)
	_, err := s.GenerateReport(run.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no results")
}

func TestStats(t *testing.T) {
	cfg := DefaultSuiteConfig()
	cfg.Duration = 1 * time.Second
	cfg.Concurrency = 2
	s := NewSuite(cfg)

	r1, _ := s.CreateRun("a", WorkloadReadHeavy)
	r2, _ := s.CreateRun("b", WorkloadWriteHeavy)
	_, _ = s.CreateRun("c", WorkloadMixed) // remains pending

	_, _ = s.RunBenchmark(r1.ID)
	_, _ = s.RunBenchmark(r2.ID)

	stats := s.Stats()
	assert.Equal(t, 3, stats.TotalRuns)
	assert.Equal(t, 2, stats.CompletedRuns)
	assert.Equal(t, 1, stats.ByWorkload[string(WorkloadReadHeavy)])
	assert.Equal(t, 1, stats.ByWorkload[string(WorkloadWriteHeavy)])
	assert.Equal(t, 1, stats.ByWorkload[string(WorkloadMixed)])
	assert.Equal(t, 2, stats.ByStatus[string(StatusCompleted)])
	assert.Equal(t, 1, stats.ByStatus[string(StatusPending)])
	assert.Greater(t, stats.AvgOpsPerSec, float64(0))
	assert.NotEmpty(t, stats.BestP99)
}

func TestStats_Empty(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())
	stats := s.Stats()

	assert.Equal(t, 0, stats.TotalRuns)
	assert.Equal(t, 0, stats.CompletedRuns)
	assert.Equal(t, float64(0), stats.AvgOpsPerSec)
	assert.Empty(t, stats.BestP99)
}

func TestComputeLatencyStats(t *testing.T) {
	samples := make([]time.Duration, 1000)
	for i := range samples {
		samples[i] = time.Duration(i+1) * time.Microsecond
	}

	stats := computeLatencyStats(samples)

	assert.Equal(t, 1*time.Microsecond, stats.Min)
	assert.Equal(t, 1000*time.Microsecond, stats.Max)
	assert.LessOrEqual(t, stats.P50, stats.P95)
	assert.LessOrEqual(t, stats.P95, stats.P99)
	assert.LessOrEqual(t, stats.P99, stats.P999)
	assert.Greater(t, stats.StdDev, time.Duration(0))
	assert.Greater(t, stats.Mean, time.Duration(0))
}

func TestComputeLatencyStats_Empty(t *testing.T) {
	stats := computeLatencyStats(nil)
	assert.Equal(t, LatencyStats{}, stats)
}

func TestDeterministicResults(t *testing.T) {
	cfg := DefaultSuiteConfig()
	cfg.Duration = 1 * time.Second
	cfg.Concurrency = 2

	s1 := NewSuite(cfg)
	r1, _ := s1.CreateRun("det-1", WorkloadReadHeavy)
	res1, _ := s1.RunBenchmark(r1.ID)

	s2 := NewSuite(cfg)
	r2, _ := s2.CreateRun("det-2", WorkloadReadHeavy)
	res2, _ := s2.RunBenchmark(r2.ID)

	// Same workload + config should produce identical results
	assert.Equal(t, res1.TotalOps, res2.TotalOps)
	assert.Equal(t, res1.ReadOps, res2.ReadOps)
	assert.Equal(t, res1.Latency.P99, res2.Latency.P99)
	assert.Equal(t, res1.OpsPerSecond, res2.OpsPerSecond)
}
