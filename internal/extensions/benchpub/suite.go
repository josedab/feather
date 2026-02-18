package benchpub

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// BenchmarkType indicates the type of benchmark to run.
type BenchmarkType string

// BenchmarkType constants.
const (
	PointLookup     BenchmarkType = "point_lookup"
	BatchGet        BenchmarkType = "batch_get"
	StreamIngest    BenchmarkType = "stream_ingest"
	HistoricalQuery BenchmarkType = "historical_query"
)

// BenchmarkConfig configures a single benchmark run.
type BenchmarkConfig struct {
	Name         string
	Type         BenchmarkType
	NumEntities  int
	NumFeatures  int
	Concurrency  int
	DurationSecs int
	WarmupSecs   int
}

// BenchmarkResult captures the result of a benchmark run.
type BenchmarkResult struct {
	Config      BenchmarkConfig
	LatencyP50  float64
	LatencyP95  float64
	LatencyP99  float64
	LatencyP999 float64
	LatencyAvg  float64
	LatencyMax  float64
	Throughput  float64
	MemoryMB    int64
	ErrorCount  int64
	StartedAt   time.Time
	CompletedAt time.Time
	DurationMs  float64
	Status      string
}

// ComparisonReport compares results across multiple benchmark runs.
type ComparisonReport struct {
	Name        string
	Results     []BenchmarkResult
	GeneratedAt time.Time
	Summary     map[string]string
}

// SuiteConfig configures the benchmark suite.
type SuiteConfig struct {
	MaxBenchmarks      int
	DefaultDuration    int
	DefaultConcurrency int
}

// DefaultSuiteConfig returns sensible defaults.
func DefaultSuiteConfig() SuiteConfig {
	return SuiteConfig{
		MaxBenchmarks:      100,
		DefaultDuration:    30,
		DefaultConcurrency: 8,
	}
}

// SuiteStats holds aggregate statistics for the suite.
type SuiteStats struct {
	TotalRuns      int
	AvgLatencyP99  float64
	BestThroughput float64
}

// Suite manages benchmark execution and results.
type Suite struct {
	mu      sync.RWMutex
	config  SuiteConfig
	results []BenchmarkResult
	running bool
}

// NewSuite creates a new benchmark suite.
func NewSuite(config SuiteConfig) *Suite {
	if config.MaxBenchmarks == 0 {
		config = DefaultSuiteConfig()
	}
	return &Suite{
		config:  config,
		results: make([]BenchmarkResult, 0),
	}
}

// Run simulates running a benchmark and returns the result.
func (s *Suite) Run(cfg BenchmarkConfig) (*BenchmarkResult, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil, ErrBenchmarkRunning
	}
	if len(s.results) >= s.config.MaxBenchmarks {
		s.mu.Unlock()
		return nil, fmt.Errorf("max benchmarks (%d) reached", s.config.MaxBenchmarks)
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	if cfg.DurationSecs == 0 {
		cfg.DurationSecs = s.config.DefaultDuration
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = s.config.DefaultConcurrency
	}

	startedAt := time.Now()
	baseLatency := baseLatencyForType(cfg.Type)

	result := &BenchmarkResult{
		Config:      cfg,
		LatencyP50:  baseLatency * (0.9 + rand.Float64()*0.2),
		LatencyP95:  baseLatency * (2.0 + rand.Float64()*0.5),
		LatencyP99:  baseLatency * (3.0 + rand.Float64()*1.0),
		LatencyP999: baseLatency * (5.0 + rand.Float64()*2.0),
		LatencyAvg:  baseLatency * (1.0 + rand.Float64()*0.3),
		LatencyMax:  baseLatency * (8.0 + rand.Float64()*4.0),
		Throughput:  float64(cfg.Concurrency) * (1000.0 / baseLatency) * (0.8 + rand.Float64()*0.4),
		MemoryMB:    int64(32 + rand.Intn(128)),
		ErrorCount:  int64(rand.Intn(5)),
		StartedAt:   startedAt,
		CompletedAt: time.Now(),
		DurationMs:  float64(time.Since(startedAt).Milliseconds()),
		Status:      "completed",
	}

	s.mu.Lock()
	s.results = append(s.results, *result)
	s.mu.Unlock()

	return result, nil
}

// ListResults returns all benchmark results.
func (s *Suite) ListResults() []BenchmarkResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]BenchmarkResult, len(s.results))
	copy(out, s.results)
	return out
}

// GetResult returns a benchmark result by name.
func (s *Suite) GetResult(name string) (*BenchmarkResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := len(s.results) - 1; i >= 0; i-- {
		if s.results[i].Config.Name == name {
			r := s.results[i]
			return &r, nil
		}
	}
	return nil, ErrBenchmarkNotFound
}

// Compare creates a comparison report across multiple results.
func (s *Suite) Compare(names []string) (*ComparisonReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []BenchmarkResult
	for _, name := range names {
		found := false
		for i := len(s.results) - 1; i >= 0; i-- {
			if s.results[i].Config.Name == name {
				results = append(results, s.results[i])
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("benchmark %q: %w", name, ErrBenchmarkNotFound)
		}
	}

	summary := make(map[string]string)
	if len(results) >= 2 {
		best := results[0]
		for _, r := range results[1:] {
			if r.LatencyP99 < best.LatencyP99 {
				best = r
			}
		}
		summary["best_latency"] = best.Config.Name
		summary["best_latency_p99"] = fmt.Sprintf("%.3fms", best.LatencyP99)

		bestTP := results[0]
		for _, r := range results[1:] {
			if r.Throughput > bestTP.Throughput {
				bestTP = r
			}
		}
		summary["best_throughput"] = bestTP.Config.Name
		summary["best_throughput_ops"] = fmt.Sprintf("%.0f ops/s", bestTP.Throughput)
	}

	return &ComparisonReport{
		Name:        fmt.Sprintf("comparison-%d", len(names)),
		Results:     results,
		GeneratedAt: time.Now(),
		Summary:     summary,
	}, nil
}

// Stats returns aggregate statistics for the suite.
func (s *Suite) Stats() SuiteStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := SuiteStats{TotalRuns: len(s.results)}
	if len(s.results) == 0 {
		return stats
	}

	var sumP99 float64
	for _, r := range s.results {
		sumP99 += r.LatencyP99
		if r.Throughput > stats.BestThroughput {
			stats.BestThroughput = r.Throughput
		}
	}
	stats.AvgLatencyP99 = sumP99 / float64(len(s.results))

	return stats
}

// baseLatencyForType returns a realistic base latency in milliseconds.
func baseLatencyForType(bt BenchmarkType) float64 {
	switch bt {
	case PointLookup:
		return 0.5
	case BatchGet:
		return 5.0
	case StreamIngest:
		return 1.0
	case HistoricalQuery:
		return 10.0
	default:
		return 1.0
	}
}

// round is a helper to avoid importing additional packages.
func round(val float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(val*p) / p
}
