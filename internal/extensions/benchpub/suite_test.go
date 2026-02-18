package benchpub

import (
	"errors"
	"testing"
)

func TestNewSuite(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())
	if s == nil {
		t.Fatal("expected non-nil suite")
	}
	results := s.ListResults()
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRunBenchmark(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	result, err := s.Run(BenchmarkConfig{
		Name:        "test-point-lookup",
		Type:        PointLookup,
		NumEntities: 1000,
		NumFeatures: 10,
		Concurrency: 4,
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result.LatencyP50 <= 0 {
		t.Errorf("expected positive LatencyP50, got %f", result.LatencyP50)
	}
	if result.LatencyP95 <= 0 {
		t.Errorf("expected positive LatencyP95, got %f", result.LatencyP95)
	}
	if result.LatencyP99 <= 0 {
		t.Errorf("expected positive LatencyP99, got %f", result.LatencyP99)
	}
	if result.Throughput <= 0 {
		t.Errorf("expected positive Throughput, got %f", result.Throughput)
	}
	if result.Status != "completed" {
		t.Errorf("expected completed status, got %s", result.Status)
	}
}

func TestCompare(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	_, err := s.Run(BenchmarkConfig{Name: "bench-a", Type: PointLookup, Concurrency: 4})
	if err != nil {
		t.Fatalf("Run bench-a failed: %v", err)
	}
	_, err = s.Run(BenchmarkConfig{Name: "bench-b", Type: BatchGet, Concurrency: 4})
	if err != nil {
		t.Fatalf("Run bench-b failed: %v", err)
	}

	report, err := s.Compare([]string{"bench-a", "bench-b"})
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if len(report.Results) != 2 {
		t.Errorf("expected 2 results in comparison, got %d", len(report.Results))
	}
	if report.Summary["best_latency"] == "" {
		t.Error("expected best_latency in summary")
	}
}

func TestCompareNotFound(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	_, err := s.Compare([]string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent benchmark")
	}
	if !errors.Is(err, ErrBenchmarkNotFound) {
		t.Errorf("expected ErrBenchmarkNotFound, got %v", err)
	}
}

func TestListResults(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	_, _ = s.Run(BenchmarkConfig{Name: "r1", Type: PointLookup})
	_, _ = s.Run(BenchmarkConfig{Name: "r2", Type: BatchGet})

	results := s.ListResults()
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestGetResult(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	_, _ = s.Run(BenchmarkConfig{Name: "lookup-test", Type: PointLookup})

	result, err := s.GetResult("lookup-test")
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}
	if result.Config.Name != "lookup-test" {
		t.Errorf("expected name lookup-test, got %s", result.Config.Name)
	}

	_, err = s.GetResult("nonexistent")
	if !errors.Is(err, ErrBenchmarkNotFound) {
		t.Errorf("expected ErrBenchmarkNotFound, got %v", err)
	}
}

func TestStats(t *testing.T) {
	s := NewSuite(DefaultSuiteConfig())

	stats := s.Stats()
	if stats.TotalRuns != 0 {
		t.Errorf("expected 0 runs, got %d", stats.TotalRuns)
	}

	_, _ = s.Run(BenchmarkConfig{Name: "s1", Type: PointLookup, Concurrency: 4})
	_, _ = s.Run(BenchmarkConfig{Name: "s2", Type: BatchGet, Concurrency: 4})

	stats = s.Stats()
	if stats.TotalRuns != 2 {
		t.Errorf("expected 2 runs, got %d", stats.TotalRuns)
	}
	if stats.AvgLatencyP99 <= 0 {
		t.Errorf("expected positive AvgLatencyP99, got %f", stats.AvgLatencyP99)
	}
	if stats.BestThroughput <= 0 {
		t.Errorf("expected positive BestThroughput, got %f", stats.BestThroughput)
	}
}
