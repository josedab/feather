package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/storage"
)

// Suite represents a benchmark suite for the feature store.
type Suite struct {
	store     *storage.Store
	config    Config
	results   map[string]*Result
	resultsMu sync.RWMutex
}

// Config configures the benchmark suite.
type Config struct {
	// NumEntities is the number of unique entities to use
	NumEntities int
	// NumFeatures is the number of features per entity
	NumFeatures int
	// NumOperations is the total operations per benchmark
	NumOperations int
	// Concurrency is the number of concurrent workers
	Concurrency int
	// WarmupDuration is the duration of the warmup phase
	WarmupDuration time.Duration
	// BenchmarkDuration is the duration of the benchmark phase (if > 0, overrides NumOperations)
	BenchmarkDuration time.Duration
	// DataSize is the approximate size of each feature value in bytes
	DataSize int
}

// DefaultConfig returns sensible defaults for benchmarking.
func DefaultConfig() Config {
	return Config{
		NumEntities:       10000,
		NumFeatures:       10,
		NumOperations:     100000,
		Concurrency:       10,
		WarmupDuration:    5 * time.Second,
		BenchmarkDuration: 0,
		DataSize:          100,
	}
}

// Result holds benchmark results for a single operation type.
type Result struct {
	Name           string        `json:"name"`
	TotalOps       int64         `json:"total_ops"`
	Duration       time.Duration `json:"duration_ns"`
	OpsPerSecond   float64       `json:"ops_per_second"`
	LatencyP50     time.Duration `json:"latency_p50_ns"`
	LatencyP95     time.Duration `json:"latency_p95_ns"`
	LatencyP99     time.Duration `json:"latency_p99_ns"`
	LatencyP999    time.Duration `json:"latency_p999_ns"`
	LatencyMin     time.Duration `json:"latency_min_ns"`
	LatencyMax     time.Duration `json:"latency_max_ns"`
	LatencyMean    time.Duration `json:"latency_mean_ns"`
	LatencyStdDev  float64       `json:"latency_stddev_ns"`
	Errors         int64         `json:"errors"`
	BytesProcessed int64         `json:"bytes_processed"`
	ThroughputMBps float64       `json:"throughput_mbps"`
}

// NewSuite creates a new benchmark suite.
func NewSuite(store *storage.Store, config Config) *Suite {
	return &Suite{
		store:   store,
		config:  config,
		results: make(map[string]*Result),
	}
}

// Run executes all benchmarks.
func (s *Suite) Run(ctx context.Context) error {
	fmt.Printf("=== Feather Feature Store Benchmark Suite ===\n\n")
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Entities:    %d\n", s.config.NumEntities)
	fmt.Printf("  Features:    %d\n", s.config.NumFeatures)
	fmt.Printf("  Operations:  %d\n", s.config.NumOperations)
	fmt.Printf("  Concurrency: %d\n", s.config.Concurrency)
	fmt.Printf("  Data Size:   %d bytes\n", s.config.DataSize)
	fmt.Printf("\n")

	// Seed data
	fmt.Print("Seeding test data... ")
	if err := s.seedData(ctx); err != nil {
		return fmt.Errorf("seeding data: %w", err)
	}
	fmt.Println("done")

	// Warmup phase
	if s.config.WarmupDuration > 0 {
		fmt.Printf("Warming up for %v... ", s.config.WarmupDuration)
		s.warmup(ctx)
		fmt.Println("done")
	}
	fmt.Println()

	// Run benchmarks
	benchmarks := []struct {
		name string
		fn   func(context.Context) (*Result, error)
	}{
		{"Write", s.benchmarkWrite},
		{"Read", s.benchmarkRead},
		{"BatchRead", s.benchmarkBatchRead},
		{"MixedWorkload", s.benchmarkMixedWorkload},
		{"ConcurrentWrite", s.benchmarkConcurrentWrite},
		{"PointInTimeRead", s.benchmarkPointInTimeRead},
	}

	for _, b := range benchmarks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fmt.Printf("Running %s benchmark...\n", b.name)
		result, err := b.fn(ctx)
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
			continue
		}

		s.resultsMu.Lock()
		s.results[b.name] = result
		s.resultsMu.Unlock()

		s.printResult(result)
		fmt.Println()
	}

	return nil
}

// GetResults returns all benchmark results.
func (s *Suite) GetResults() map[string]*Result {
	s.resultsMu.RLock()
	defer s.resultsMu.RUnlock()

	results := make(map[string]*Result, len(s.results))
	for k, v := range s.results {
		results[k] = v
	}
	return results
}

// GetResultsJSON returns benchmark results as JSON.
func (s *Suite) GetResultsJSON() ([]byte, error) {
	return json.MarshalIndent(s.GetResults(), "", "  ")
}

func (s *Suite) seedData(ctx context.Context) error {
	for i := 0; i < s.config.NumEntities; i++ {
		if i%1000 == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		entityKey := fmt.Sprintf("entity:%d", i)
		features := make(map[string]*domain.FeatureValue, s.config.NumFeatures)

		for j := 0; j < s.config.NumFeatures; j++ {
			features[fmt.Sprintf("feature_%d", j)] = &domain.FeatureValue{
				Value:     s.generateValue(),
				Timestamp: time.Now().UnixNano(),
				Version:   1,
			}
		}

		if err := s.store.Put(entityKey, features); err != nil {
			return err
		}
	}
	return nil
}

func (s *Suite) warmup(ctx context.Context) {
	deadline := time.Now().Add(s.config.WarmupDuration)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		entityKey := fmt.Sprintf("entity:%d", rand.Intn(s.config.NumEntities))
		features := []string{fmt.Sprintf("feature_%d", rand.Intn(s.config.NumFeatures))}
		s.store.Get(entityKey, features)
	}
}

func (s *Suite) benchmarkWrite(ctx context.Context) (*Result, error) {
	latencies := make([]time.Duration, 0, s.config.NumOperations)
	var totalBytes int64
	var errors int64
	var mu sync.Mutex

	start := time.Now()
	var wg sync.WaitGroup
	opsPerWorker := s.config.NumOperations / s.config.Concurrency

	for w := 0; w < s.config.Concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			localLatencies := make([]time.Duration, 0, opsPerWorker)

			for i := 0; i < opsPerWorker; i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				entityKey := fmt.Sprintf("entity:%d", rand.Intn(s.config.NumEntities))
				featureName := fmt.Sprintf("feature_%d", rand.Intn(s.config.NumFeatures))
				value := s.generateValue()

				opStart := time.Now()
				err := s.store.Put(entityKey, map[string]*domain.FeatureValue{
					featureName: {
						Value:     value,
						Timestamp: time.Now().UnixNano(),
						Version:   1,
					},
				})
				latency := time.Since(opStart)

				if err != nil {
					atomic.AddInt64(&errors, 1)
				} else {
					localLatencies = append(localLatencies, latency)
					atomic.AddInt64(&totalBytes, int64(s.config.DataSize))
				}
			}

			mu.Lock()
			latencies = append(latencies, localLatencies...)
			mu.Unlock()
		}(w)
	}

	wg.Wait()
	duration := time.Since(start)

	return s.calculateResult("Write", latencies, duration, totalBytes, errors), nil
}

func (s *Suite) benchmarkRead(ctx context.Context) (*Result, error) {
	latencies := make([]time.Duration, 0, s.config.NumOperations)
	var totalBytes int64
	var errors int64
	var mu sync.Mutex

	start := time.Now()
	var wg sync.WaitGroup
	opsPerWorker := s.config.NumOperations / s.config.Concurrency

	for w := 0; w < s.config.Concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			localLatencies := make([]time.Duration, 0, opsPerWorker)

			for i := 0; i < opsPerWorker; i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				entityKey := fmt.Sprintf("entity:%d", rand.Intn(s.config.NumEntities))
				featureName := fmt.Sprintf("feature_%d", rand.Intn(s.config.NumFeatures))

				opStart := time.Now()
				values, err := s.store.Get(entityKey, []string{featureName})
				latency := time.Since(opStart)

				if err != nil {
					atomic.AddInt64(&errors, 1)
				} else {
					localLatencies = append(localLatencies, latency)
					if len(values) > 0 {
						atomic.AddInt64(&totalBytes, int64(s.config.DataSize))
					}
				}
			}

			mu.Lock()
			latencies = append(latencies, localLatencies...)
			mu.Unlock()
		}(w)
	}

	wg.Wait()
	duration := time.Since(start)

	return s.calculateResult("Read", latencies, duration, totalBytes, errors), nil
}

func (s *Suite) benchmarkBatchRead(ctx context.Context) (*Result, error) {
	latencies := make([]time.Duration, 0, s.config.NumOperations)
	var totalBytes int64
	var errors int64
	var mu sync.Mutex

	batchSize := min(s.config.NumFeatures, 5)
	start := time.Now()
	var wg sync.WaitGroup
	opsPerWorker := s.config.NumOperations / s.config.Concurrency

	for w := 0; w < s.config.Concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			localLatencies := make([]time.Duration, 0, opsPerWorker)

			for i := 0; i < opsPerWorker; i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				entityKey := fmt.Sprintf("entity:%d", rand.Intn(s.config.NumEntities))
				features := make([]string, batchSize)
				for j := 0; j < batchSize; j++ {
					features[j] = fmt.Sprintf("feature_%d", j)
				}

				opStart := time.Now()
				values, err := s.store.Get(entityKey, features)
				latency := time.Since(opStart)

				if err != nil {
					atomic.AddInt64(&errors, 1)
				} else {
					localLatencies = append(localLatencies, latency)
					atomic.AddInt64(&totalBytes, int64(len(values)*s.config.DataSize))
				}
			}

			mu.Lock()
			latencies = append(latencies, localLatencies...)
			mu.Unlock()
		}(w)
	}

	wg.Wait()
	duration := time.Since(start)

	return s.calculateResult("BatchRead", latencies, duration, totalBytes, errors), nil
}

func (s *Suite) benchmarkMixedWorkload(ctx context.Context) (*Result, error) {
	// 80% reads, 20% writes
	latencies := make([]time.Duration, 0, s.config.NumOperations)
	var totalBytes int64
	var errors int64
	var mu sync.Mutex

	start := time.Now()
	var wg sync.WaitGroup
	opsPerWorker := s.config.NumOperations / s.config.Concurrency

	for w := 0; w < s.config.Concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			localLatencies := make([]time.Duration, 0, opsPerWorker)

			for i := 0; i < opsPerWorker; i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				entityKey := fmt.Sprintf("entity:%d", rand.Intn(s.config.NumEntities))
				featureName := fmt.Sprintf("feature_%d", rand.Intn(s.config.NumFeatures))

				var latency time.Duration
				var err error

				if rand.Float64() < 0.8 {
					// Read operation
					opStart := time.Now()
					_, err = s.store.Get(entityKey, []string{featureName})
					latency = time.Since(opStart)
				} else {
					// Write operation
					opStart := time.Now()
					err = s.store.Put(entityKey, map[string]*domain.FeatureValue{
						featureName: {
							Value:     s.generateValue(),
							Timestamp: time.Now().UnixNano(),
							Version:   1,
						},
					})
					latency = time.Since(opStart)
				}

				if err != nil {
					atomic.AddInt64(&errors, 1)
				} else {
					localLatencies = append(localLatencies, latency)
					atomic.AddInt64(&totalBytes, int64(s.config.DataSize))
				}
			}

			mu.Lock()
			latencies = append(latencies, localLatencies...)
			mu.Unlock()
		}(w)
	}

	wg.Wait()
	duration := time.Since(start)

	return s.calculateResult("MixedWorkload", latencies, duration, totalBytes, errors), nil
}

func (s *Suite) benchmarkConcurrentWrite(ctx context.Context) (*Result, error) {
	// High contention write test - all workers write to same set of entities
	latencies := make([]time.Duration, 0, s.config.NumOperations)
	var totalBytes int64
	var errors int64
	var mu sync.Mutex

	hotEntities := 100 // Small number of entities for contention
	start := time.Now()
	var wg sync.WaitGroup
	opsPerWorker := s.config.NumOperations / s.config.Concurrency

	for w := 0; w < s.config.Concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			localLatencies := make([]time.Duration, 0, opsPerWorker)

			for i := 0; i < opsPerWorker; i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				entityKey := fmt.Sprintf("hot_entity:%d", rand.Intn(hotEntities))
				featureName := fmt.Sprintf("feature_%d", rand.Intn(s.config.NumFeatures))

				opStart := time.Now()
				err := s.store.Put(entityKey, map[string]*domain.FeatureValue{
					featureName: {
						Value:     s.generateValue(),
						Timestamp: time.Now().UnixNano(),
						Version:   1,
					},
				})
				latency := time.Since(opStart)

				if err != nil {
					atomic.AddInt64(&errors, 1)
				} else {
					localLatencies = append(localLatencies, latency)
					atomic.AddInt64(&totalBytes, int64(s.config.DataSize))
				}
			}

			mu.Lock()
			latencies = append(latencies, localLatencies...)
			mu.Unlock()
		}(w)
	}

	wg.Wait()
	duration := time.Since(start)

	return s.calculateResult("ConcurrentWrite", latencies, duration, totalBytes, errors), nil
}

func (s *Suite) benchmarkPointInTimeRead(ctx context.Context) (*Result, error) {
	latencies := make([]time.Duration, 0, s.config.NumOperations)
	var totalBytes int64
	var errors int64
	var mu sync.Mutex

	// Create some historical data first
	for i := 0; i < 100; i++ {
		entityKey := fmt.Sprintf("pit_entity:%d", i)
		for j := 0; j < 10; j++ {
			features := make(map[string]*domain.FeatureValue)
			for k := 0; k < s.config.NumFeatures; k++ {
				features[fmt.Sprintf("feature_%d", k)] = &domain.FeatureValue{
					Value:     s.generateValue(),
					Timestamp: time.Now().Add(-time.Duration(10-j) * time.Hour).UnixNano(),
					Version:   int64(j + 1),
				}
			}
			s.store.Put(entityKey, features)
			time.Sleep(time.Millisecond) // Ensure different timestamps
		}
	}

	start := time.Now()
	var wg sync.WaitGroup
	opsPerWorker := s.config.NumOperations / s.config.Concurrency

	for w := 0; w < s.config.Concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			localLatencies := make([]time.Duration, 0, opsPerWorker)

			for i := 0; i < opsPerWorker; i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				entityKey := fmt.Sprintf("pit_entity:%d", rand.Intn(100))
				featureName := fmt.Sprintf("feature_%d", rand.Intn(s.config.NumFeatures))
				asOf := time.Now().Add(-time.Duration(rand.Intn(10)) * time.Hour)

				opStart := time.Now()
				values, err := s.store.GetAsOf(entityKey, []string{featureName}, asOf)
				latency := time.Since(opStart)

				if err != nil {
					atomic.AddInt64(&errors, 1)
				} else {
					localLatencies = append(localLatencies, latency)
					if len(values) > 0 {
						atomic.AddInt64(&totalBytes, int64(s.config.DataSize))
					}
				}
			}

			mu.Lock()
			latencies = append(latencies, localLatencies...)
			mu.Unlock()
		}(w)
	}

	wg.Wait()
	duration := time.Since(start)

	return s.calculateResult("PointInTimeRead", latencies, duration, totalBytes, errors), nil
}

func (s *Suite) generateValue() interface{} {
	// Generate a value of approximately the configured size
	data := make([]byte, s.config.DataSize)
	for i := range data {
		data[i] = byte('a' + rand.Intn(26))
	}
	return string(data)
}

func (s *Suite) calculateResult(name string, latencies []time.Duration, duration time.Duration, totalBytes, errors int64) *Result {
	if len(latencies) == 0 {
		return &Result{Name: name, Errors: errors}
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	totalOps := int64(len(latencies))
	opsPerSecond := float64(totalOps) / duration.Seconds()
	throughputMBps := float64(totalBytes) / (1024 * 1024) / duration.Seconds()

	// Calculate statistics
	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}
	mean := sum / time.Duration(len(latencies))

	var varianceSum float64
	for _, l := range latencies {
		diff := float64(l - mean)
		varianceSum += diff * diff
	}
	stdDev := math.Sqrt(varianceSum / float64(len(latencies)))

	return &Result{
		Name:           name,
		TotalOps:       totalOps,
		Duration:       duration,
		OpsPerSecond:   opsPerSecond,
		LatencyP50:     latencies[len(latencies)*50/100],
		LatencyP95:     latencies[len(latencies)*95/100],
		LatencyP99:     latencies[len(latencies)*99/100],
		LatencyP999:    latencies[min(len(latencies)-1, len(latencies)*999/1000)],
		LatencyMin:     latencies[0],
		LatencyMax:     latencies[len(latencies)-1],
		LatencyMean:    mean,
		LatencyStdDev:  stdDev,
		Errors:         errors,
		BytesProcessed: totalBytes,
		ThroughputMBps: throughputMBps,
	}
}

func (s *Suite) printResult(r *Result) {
	fmt.Printf("  %-18s %s\n", "Total Operations:", formatNumber(r.TotalOps))
	fmt.Printf("  %-18s %.2f ops/sec\n", "Throughput:", r.OpsPerSecond)
	fmt.Printf("  %-18s %.2f MB/s\n", "Data Throughput:", r.ThroughputMBps)
	fmt.Printf("  %-18s %v\n", "Duration:", r.Duration.Round(time.Millisecond))
	fmt.Printf("  %-18s %v\n", "Latency P50:", r.LatencyP50)
	fmt.Printf("  %-18s %v\n", "Latency P95:", r.LatencyP95)
	fmt.Printf("  %-18s %v\n", "Latency P99:", r.LatencyP99)
	fmt.Printf("  %-18s %v\n", "Latency P99.9:", r.LatencyP999)
	fmt.Printf("  %-18s %v\n", "Latency Min:", r.LatencyMin)
	fmt.Printf("  %-18s %v\n", "Latency Max:", r.LatencyMax)
	fmt.Printf("  %-18s %v\n", "Latency Mean:", r.LatencyMean)
	fmt.Printf("  %-18s %.2f µs\n", "Latency StdDev:", r.LatencyStdDev/1000)
	if r.Errors > 0 {
		fmt.Printf("  %-18s %d\n", "Errors:", r.Errors)
	}
}

func formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.2fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.2fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
