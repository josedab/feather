package benchsuite

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// WorkloadType defines the type of benchmark workload.
type WorkloadType string

const (
	WorkloadReadHeavy  WorkloadType = "read_heavy"  // 90% read, 10% write
	WorkloadWriteHeavy WorkloadType = "write_heavy"  // 10% read, 90% write
	WorkloadMixed      WorkloadType = "mixed"         // 50% read, 50% write
	WorkloadBurst      WorkloadType = "burst"         // periodic high load
	WorkloadLatency    WorkloadType = "latency"       // focus on p99
	WorkloadThroughput WorkloadType = "throughput"    // focus on max QPS
)

var validWorkloads = map[WorkloadType]bool{
	WorkloadReadHeavy:  true,
	WorkloadWriteHeavy: true,
	WorkloadMixed:      true,
	WorkloadBurst:      true,
	WorkloadLatency:    true,
	WorkloadThroughput: true,
}

// BenchmarkStatus represents the lifecycle status of a benchmark run.
type BenchmarkStatus string

const (
	StatusPending   BenchmarkStatus = "pending"
	StatusRunning   BenchmarkStatus = "running"
	StatusCompleted BenchmarkStatus = "completed"
	StatusFailed    BenchmarkStatus = "failed"
)

// SuiteConfig configures the benchmark suite.
type SuiteConfig struct {
	Warmup       time.Duration `json:"warmup"`
	Duration     time.Duration `json:"duration"`
	Concurrency  int           `json:"concurrency"`
	NumFeatures  int           `json:"num_features"`
	NumEntities  int           `json:"num_entities"`
	ReportFormat string        `json:"report_format"` // "json", "text"
}

// DefaultSuiteConfig returns sensible defaults for benchmarking.
func DefaultSuiteConfig() SuiteConfig {
	return SuiteConfig{
		Warmup:       5 * time.Second,
		Duration:     30 * time.Second,
		Concurrency:  10,
		NumFeatures:  100,
		NumEntities:  10000,
		ReportFormat: "json",
	}
}

// Suite manages benchmark runs with standardized workloads.
type Suite struct {
	config     SuiteConfig
	mu         sync.RWMutex
	benchmarks map[string]*BenchmarkRun
	idCounter  int64
}

// BenchmarkRun represents a single benchmark execution.
type BenchmarkRun struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Workload    WorkloadType      `json:"workload"`
	Config      SuiteConfig       `json:"config"`
	Status      BenchmarkStatus   `json:"status"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt time.Time         `json:"completed_at,omitempty"`
	Results     *BenchmarkResults `json:"results,omitempty"`
	Error       string            `json:"error,omitempty"`
}

// BenchmarkResults holds all metrics from a completed benchmark run.
type BenchmarkResults struct {
	TotalOps     int64           `json:"total_operations"`
	Duration     time.Duration   `json:"duration"`
	OpsPerSecond float64         `json:"ops_per_second"`
	ReadOps      int64           `json:"read_ops"`
	WriteOps     int64           `json:"write_ops"`
	Errors       int64           `json:"errors"`
	ErrorRate    float64         `json:"error_rate_pct"`
	Latency      LatencyStats    `json:"latency"`
	ReadLatency  LatencyStats    `json:"read_latency"`
	WriteLatency LatencyStats    `json:"write_latency"`
	Throughput   ThroughputStats `json:"throughput"`
	MemoryUsed   int64           `json:"memory_used_bytes"`
}

// LatencyStats holds percentile-based latency statistics.
type LatencyStats struct {
	Min    time.Duration `json:"min"`
	Max    time.Duration `json:"max"`
	Mean   time.Duration `json:"mean"`
	P50    time.Duration `json:"p50"`
	P95    time.Duration `json:"p95"`
	P99    time.Duration `json:"p99"`
	P999   time.Duration `json:"p999"`
	StdDev time.Duration `json:"std_dev"`
}

// ThroughputStats holds throughput metrics.
type ThroughputStats struct {
	AvgOpsPerSec  float64   `json:"avg_ops_per_sec"`
	PeakOpsPerSec float64   `json:"peak_ops_per_sec"`
	Samples       []float64 `json:"-"` // don't serialize raw samples
}

// ComparisonReport compares multiple benchmark runs.
type ComparisonReport struct {
	Runs        []*BenchmarkRun   `json:"runs"`
	Rankings    []RankEntry       `json:"rankings"`
	Summary     ComparisonSummary `json:"summary"`
	GeneratedAt time.Time         `json:"generated_at"`
}

// RankEntry represents a ranked benchmark run.
type RankEntry struct {
	RunID     string  `json:"run_id"`
	Name      string  `json:"name"`
	Score     float64 `json:"score"` // composite score 0-100
	Rank      int     `json:"rank"`
	P99Ms     float64 `json:"p99_ms"`
	OpsPerSec float64 `json:"ops_per_sec"`
	ErrorRate float64 `json:"error_rate_pct"`
}

// ComparisonSummary provides quick lookup for best-in-class runs.
type ComparisonSummary struct {
	FastestP99   string `json:"fastest_p99"`
	HighestQPS   string `json:"highest_qps"`
	LowestErrors string `json:"lowest_errors"`
	BestOverall  string `json:"best_overall"`
}

// SuiteStats provides aggregate statistics across all benchmark runs.
type SuiteStats struct {
	TotalRuns     int            `json:"total_runs"`
	CompletedRuns int            `json:"completed_runs"`
	ByWorkload    map[string]int `json:"by_workload"`
	ByStatus      map[string]int `json:"by_status"`
	AvgOpsPerSec  float64        `json:"avg_ops_per_sec"`
	BestP99       string         `json:"best_p99"`
}

// NewSuite creates a new benchmark suite with the given configuration.
func NewSuite(cfg SuiteConfig) *Suite {
	return &Suite{
		config:     cfg,
		benchmarks: make(map[string]*BenchmarkRun),
	}
}

// CreateRun creates a new benchmark run with pending status.
func (s *Suite) CreateRun(name string, workload WorkloadType) (*BenchmarkRun, error) {
	if name == "" {
		return nil, fmt.Errorf("benchmark name is required")
	}
	if !validWorkloads[workload] {
		return nil, fmt.Errorf("invalid workload type: %s", workload)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.idCounter++
	id := fmt.Sprintf("bench-%d", s.idCounter)

	run := &BenchmarkRun{
		ID:       id,
		Name:     name,
		Workload: workload,
		Config:   s.config,
		Status:   StatusPending,
	}

	s.benchmarks[id] = run
	return run, nil
}

// RunBenchmark executes the benchmark identified by id.
func (s *Suite) RunBenchmark(id string) (*BenchmarkResults, error) {
	s.mu.Lock()
	run, ok := s.benchmarks[id]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("benchmark run not found: %s", id)
	}
	run.Status = StatusRunning
	run.StartedAt = time.Now()
	s.mu.Unlock()

	results, err := s.simulateWorkload(run.Workload, run.Config)
	if err != nil {
		s.mu.Lock()
		run.Status = StatusFailed
		run.Error = err.Error()
		s.mu.Unlock()
		return nil, err
	}

	s.mu.Lock()
	run.Status = StatusCompleted
	run.CompletedAt = time.Now()
	run.Results = results
	s.mu.Unlock()

	return results, nil
}

// GetRun returns the benchmark run identified by id.
func (s *Suite) GetRun(id string) (*BenchmarkRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	run, ok := s.benchmarks[id]
	if !ok {
		return nil, fmt.Errorf("benchmark run not found: %s", id)
	}
	return run, nil
}

// ListRuns returns all benchmark runs sorted by ID.
func (s *Suite) ListRuns() []*BenchmarkRun {
	s.mu.RLock()
	defer s.mu.RUnlock()

	runs := make([]*BenchmarkRun, 0, len(s.benchmarks))
	for _, r := range s.benchmarks {
		runs = append(runs, r)
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].ID < runs[j].ID
	})
	return runs
}

// DeleteRun removes a benchmark run by id.
func (s *Suite) DeleteRun(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.benchmarks[id]; !ok {
		return fmt.Errorf("benchmark run not found: %s", id)
	}
	delete(s.benchmarks, id)
	return nil
}

// Compare generates a comparison report for the given run IDs.
func (s *Suite) Compare(ids []string) (*ComparisonReport, error) {
	if len(ids) < 2 {
		return nil, fmt.Errorf("at least 2 runs required for comparison")
	}

	s.mu.RLock()
	var runs []*BenchmarkRun
	for _, id := range ids {
		run, ok := s.benchmarks[id]
		if !ok {
			s.mu.RUnlock()
			return nil, fmt.Errorf("benchmark run not found: %s", id)
		}
		if run.Status != StatusCompleted || run.Results == nil {
			s.mu.RUnlock()
			return nil, fmt.Errorf("benchmark run %s is not completed", id)
		}
		runs = append(runs, run)
	}
	s.mu.RUnlock()

	// Collect metrics for normalization
	var maxOps, maxP99, maxErrRate float64
	var minP99 float64 = math.MaxFloat64
	type runMetrics struct {
		ops     float64
		p99     float64
		errRate float64
		stddev  float64
	}
	metrics := make([]runMetrics, len(runs))

	for i, run := range runs {
		m := runMetrics{
			ops:     run.Results.OpsPerSecond,
			p99:     float64(run.Results.Latency.P99) / float64(time.Millisecond),
			errRate: run.Results.ErrorRate,
			stddev:  float64(run.Results.Latency.StdDev),
		}
		metrics[i] = m
		if m.ops > maxOps {
			maxOps = m.ops
		}
		if m.p99 > maxP99 {
			maxP99 = m.p99
		}
		if m.p99 < minP99 {
			minP99 = m.p99
		}
		if m.errRate > maxErrRate {
			maxErrRate = m.errRate
		}
	}

	// Build rankings with composite score
	rankings := make([]RankEntry, len(runs))
	for i, run := range runs {
		m := metrics[i]

		normalizedOps := 0.0
		if maxOps > 0 {
			normalizedOps = m.ops / maxOps
		}
		normalizedP99Inv := 0.0
		if maxP99 > 0 {
			normalizedP99Inv = 1.0 - (m.p99 / maxP99)
		}
		normalizedErrRateInv := 1.0
		if maxErrRate > 0 {
			normalizedErrRateInv = 1.0 - (m.errRate / maxErrRate)
		}
		// Consistency: inverse of normalized stddev
		consistency := 1.0
		if m.p99 > 0 {
			consistency = 1.0 - math.Min(m.stddev/float64(run.Results.Latency.P99), 1.0)
		}

		score := (0.40*normalizedOps + 0.30*normalizedP99Inv + 0.20*normalizedErrRateInv + 0.10*consistency) * 100

		rankings[i] = RankEntry{
			RunID:     run.ID,
			Name:      run.Name,
			Score:     math.Round(score*100) / 100,
			P99Ms:     math.Round(m.p99*1000) / 1000,
			OpsPerSec: math.Round(m.ops*100) / 100,
			ErrorRate: math.Round(m.errRate*1000) / 1000,
		}
	}

	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Score > rankings[j].Score
	})
	for i := range rankings {
		rankings[i].Rank = i + 1
	}

	// Build summary
	summary := ComparisonSummary{}
	bestP99Idx, bestQPSIdx, bestErrIdx := 0, 0, 0
	for i := range runs {
		if metrics[i].p99 < metrics[bestP99Idx].p99 {
			bestP99Idx = i
		}
		if metrics[i].ops > metrics[bestQPSIdx].ops {
			bestQPSIdx = i
		}
		if metrics[i].errRate < metrics[bestErrIdx].errRate {
			bestErrIdx = i
		}
	}
	summary.FastestP99 = runs[bestP99Idx].Name
	summary.HighestQPS = runs[bestQPSIdx].Name
	summary.LowestErrors = runs[bestErrIdx].Name
	summary.BestOverall = rankings[0].Name

	return &ComparisonReport{
		Runs:        runs,
		Rankings:    rankings,
		Summary:     summary,
		GeneratedAt: time.Now(),
	}, nil
}

// GenerateReport produces a text-format report for the given run.
func (s *Suite) GenerateReport(id string) (string, error) {
	s.mu.RLock()
	run, ok := s.benchmarks[id]
	s.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("benchmark run not found: %s", id)
	}
	if run.Results == nil {
		return "", fmt.Errorf("benchmark run %s has no results", id)
	}

	r := run.Results
	var b strings.Builder

	b.WriteString("╔══════════════════════════════════════════════════════╗\n")
	b.WriteString(fmt.Sprintf("║  Benchmark Report: %-34s ║\n", run.Name))
	b.WriteString("╠══════════════════════════════════════════════════════╣\n")
	b.WriteString(fmt.Sprintf("║  ID:       %-42s ║\n", run.ID))
	b.WriteString(fmt.Sprintf("║  Workload: %-42s ║\n", run.Workload))
	b.WriteString(fmt.Sprintf("║  Status:   %-42s ║\n", run.Status))
	b.WriteString("╠══════════════════════════════════════════════════════╣\n")
	b.WriteString("║  Operations                                        ║\n")
	b.WriteString("╠══════════════════════════════════════════════════════╣\n")
	b.WriteString(fmt.Sprintf("║  Total Ops:     %-37d ║\n", r.TotalOps))
	b.WriteString(fmt.Sprintf("║  Read Ops:      %-37d ║\n", r.ReadOps))
	b.WriteString(fmt.Sprintf("║  Write Ops:     %-37d ║\n", r.WriteOps))
	b.WriteString(fmt.Sprintf("║  Errors:        %-37d ║\n", r.Errors))
	b.WriteString(fmt.Sprintf("║  Error Rate:    %-37.3f ║\n", r.ErrorRate))
	b.WriteString(fmt.Sprintf("║  Ops/sec:       %-37.2f ║\n", r.OpsPerSecond))
	b.WriteString("╠══════════════════════════════════════════════════════╣\n")
	b.WriteString("║  Overall Latency                                   ║\n")
	b.WriteString("╠══════════════════════════════════════════════════════╣\n")
	writeLatencySection(&b, r.Latency)
	b.WriteString("╠══════════════════════════════════════════════════════╣\n")
	b.WriteString("║  Read Latency                                      ║\n")
	b.WriteString("╠══════════════════════════════════════════════════════╣\n")
	writeLatencySection(&b, r.ReadLatency)
	b.WriteString("╠══════════════════════════════════════════════════════╣\n")
	b.WriteString("║  Write Latency                                     ║\n")
	b.WriteString("╠══════════════════════════════════════════════════════╣\n")
	writeLatencySection(&b, r.WriteLatency)
	b.WriteString("╠══════════════════════════════════════════════════════╣\n")
	b.WriteString("║  Throughput                                        ║\n")
	b.WriteString("╠══════════════════════════════════════════════════════╣\n")
	b.WriteString(fmt.Sprintf("║  Avg Ops/sec:   %-37.2f ║\n", r.Throughput.AvgOpsPerSec))
	b.WriteString(fmt.Sprintf("║  Peak Ops/sec:  %-37.2f ║\n", r.Throughput.PeakOpsPerSec))
	b.WriteString("╚══════════════════════════════════════════════════════╝\n")

	return b.String(), nil
}

func writeLatencySection(b *strings.Builder, ls LatencyStats) {
	b.WriteString(fmt.Sprintf("║  Min:           %-37s ║\n", ls.Min))
	b.WriteString(fmt.Sprintf("║  Mean:          %-37s ║\n", ls.Mean))
	b.WriteString(fmt.Sprintf("║  P50:           %-37s ║\n", ls.P50))
	b.WriteString(fmt.Sprintf("║  P95:           %-37s ║\n", ls.P95))
	b.WriteString(fmt.Sprintf("║  P99:           %-37s ║\n", ls.P99))
	b.WriteString(fmt.Sprintf("║  P99.9:         %-37s ║\n", ls.P999))
	b.WriteString(fmt.Sprintf("║  Max:           %-37s ║\n", ls.Max))
	b.WriteString(fmt.Sprintf("║  StdDev:        %-37s ║\n", ls.StdDev))
}

// Stats returns aggregate statistics across all benchmark runs.
func (s *Suite) Stats() SuiteStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := SuiteStats{
		TotalRuns:  len(s.benchmarks),
		ByWorkload: make(map[string]int),
		ByStatus:   make(map[string]int),
	}

	var totalOps float64
	var completedCount int
	var bestP99 time.Duration
	var bestP99Name string

	for _, run := range s.benchmarks {
		stats.ByWorkload[string(run.Workload)]++
		stats.ByStatus[string(run.Status)]++

		if run.Status == StatusCompleted && run.Results != nil {
			completedCount++
			totalOps += run.Results.OpsPerSecond

			p99 := run.Results.Latency.P99
			if bestP99Name == "" || p99 < bestP99 {
				bestP99 = p99
				bestP99Name = run.Name
			}
		}
	}

	stats.CompletedRuns = completedCount
	if completedCount > 0 {
		stats.AvgOpsPerSec = totalOps / float64(completedCount)
	}
	stats.BestP99 = bestP99Name

	return stats
}

// simulateWorkload generates deterministic benchmark results based on workload type.
func (s *Suite) simulateWorkload(workload WorkloadType, cfg SuiteConfig) (*BenchmarkResults, error) {
	rng := &deterministicRNG{seed: workloadSeed(workload)}

	opsPerSecondTarget := float64(cfg.Concurrency) * 1000
	totalOps := int64(cfg.Duration.Seconds() * opsPerSecondTarget)

	var readRatio float64
	switch workload {
	case WorkloadReadHeavy:
		readRatio = 0.9
	case WorkloadWriteHeavy:
		readRatio = 0.1
	case WorkloadMixed:
		readRatio = 0.5
	case WorkloadBurst:
		readRatio = 0.6
	case WorkloadLatency:
		readRatio = 0.7
	case WorkloadThroughput:
		readRatio = 0.5
		totalOps = int64(float64(totalOps) * 1.5) // higher throughput
	default:
		return nil, fmt.Errorf("unsupported workload type: %s", workload)
	}

	readOps := int64(float64(totalOps) * readRatio)
	writeOps := totalOps - readOps

	readSamples := generateLatencySamples(rng, workload, true, int(readOps))
	writeSamples := generateLatencySamples(rng, workload, false, int(writeOps))

	allSamples := make([]time.Duration, 0, len(readSamples)+len(writeSamples))
	allSamples = append(allSamples, readSamples...)
	allSamples = append(allSamples, writeSamples...)

	errorCount := int64(float64(totalOps) * 0.001) // 0.1% error rate
	errorRate := float64(errorCount) / float64(totalOps) * 100

	// Compute throughput samples (1-second buckets)
	buckets := int(cfg.Duration.Seconds())
	if buckets < 1 {
		buckets = 1
	}
	opsPerBucket := float64(totalOps) / float64(buckets)
	throughputSamples := make([]float64, buckets)
	var peakOps float64
	for i := 0; i < buckets; i++ {
		variation := 0.8 + rng.Float64()*0.4 // 0.8x to 1.2x
		throughputSamples[i] = opsPerBucket * variation
		if throughputSamples[i] > peakOps {
			peakOps = throughputSamples[i]
		}
	}

	return &BenchmarkResults{
		TotalOps:     totalOps,
		Duration:     cfg.Duration,
		OpsPerSecond: float64(totalOps) / cfg.Duration.Seconds(),
		ReadOps:      readOps,
		WriteOps:     writeOps,
		Errors:       errorCount,
		ErrorRate:    errorRate,
		Latency:      computeLatencyStats(allSamples),
		ReadLatency:  computeLatencyStats(readSamples),
		WriteLatency: computeLatencyStats(writeSamples),
		Throughput: ThroughputStats{
			AvgOpsPerSec:  float64(totalOps) / cfg.Duration.Seconds(),
			PeakOpsPerSec: peakOps,
			Samples:       throughputSamples,
		},
		MemoryUsed: int64(cfg.NumEntities) * int64(cfg.NumFeatures) * 256,
	}, nil
}

// generateLatencySamples produces deterministic latency samples for the given workload.
func generateLatencySamples(rng *deterministicRNG, workload WorkloadType, isRead bool, count int) []time.Duration {
	if count <= 0 {
		return nil
	}
	samples := make([]time.Duration, count)

	var baseMin, baseMax float64 // in microseconds
	switch workload {
	case WorkloadReadHeavy:
		if isRead {
			baseMin, baseMax = 100, 500
		} else {
			baseMin, baseMax = 500, 2000
		}
	case WorkloadWriteHeavy:
		if isRead {
			baseMin, baseMax = 500, 2000
		} else {
			baseMin, baseMax = 100, 500
		}
	case WorkloadMixed:
		if isRead {
			baseMin, baseMax = 200, 800
		} else {
			baseMin, baseMax = 300, 1000
		}
	case WorkloadBurst:
		// Bimodal: half normal, half elevated
		if isRead {
			baseMin, baseMax = 100, 400
		} else {
			baseMin, baseMax = 200, 800
		}
		for i := 0; i < count; i++ {
			if i%2 == 0 {
				us := baseMin + rng.Float64()*(baseMax-baseMin)
				samples[i] = time.Duration(us * float64(time.Microsecond))
			} else {
				// Burst: 3x-5x elevated
				us := baseMax*3 + rng.Float64()*(baseMax*2)
				samples[i] = time.Duration(us * float64(time.Microsecond))
			}
		}
		return samples
	case WorkloadLatency:
		// Very tight distribution
		if isRead {
			baseMin, baseMax = 50, 150
		} else {
			baseMin, baseMax = 80, 200
		}
	case WorkloadThroughput:
		// Wider range, higher ops
		if isRead {
			baseMin, baseMax = 200, 2000
		} else {
			baseMin, baseMax = 300, 3000
		}
	}

	for i := 0; i < count; i++ {
		us := baseMin + rng.Float64()*(baseMax-baseMin)
		samples[i] = time.Duration(us * float64(time.Microsecond))
	}
	return samples
}

// computeLatencyStats calculates percentile-based latency statistics from sorted samples.
func computeLatencyStats(samples []time.Duration) LatencyStats {
	if len(samples) == 0 {
		return LatencyStats{}
	}

	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	n := len(sorted)

	var sum time.Duration
	for _, s := range sorted {
		sum += s
	}
	mean := sum / time.Duration(n)

	var varianceSum float64
	for _, s := range sorted {
		diff := float64(s - mean)
		varianceSum += diff * diff
	}
	stdDev := time.Duration(math.Sqrt(varianceSum / float64(n)))

	return LatencyStats{
		Min:    sorted[0],
		Max:    sorted[n-1],
		Mean:   mean,
		P50:    sorted[n*50/100],
		P95:    sorted[n*95/100],
		P99:    sorted[n*99/100],
		P999:   sorted[min(n-1, n*999/1000)],
		StdDev: stdDev,
	}
}

// deterministicRNG is a simple LCG-based pseudo-random number generator.
type deterministicRNG struct {
	seed uint64
}

func (r *deterministicRNG) nextUint64() uint64 {
	const (
		multiplier = 6364136223846793005
		increment  = 1442695040888963407
	)
	r.seed = r.seed*multiplier + increment
	return r.seed
}

func (r *deterministicRNG) Float64() float64 {
	const denom = 1 << 53
	return float64(r.nextUint64()>>11) / denom
}

// workloadSeed returns a deterministic seed for a given workload type.
func workloadSeed(w WorkloadType) uint64 {
	switch w {
	case WorkloadReadHeavy:
		return 1001
	case WorkloadWriteHeavy:
		return 2002
	case WorkloadMixed:
		return 3003
	case WorkloadBurst:
		return 4004
	case WorkloadLatency:
		return 5005
	case WorkloadThroughput:
		return 6006
	default:
		return 9999
	}
}
