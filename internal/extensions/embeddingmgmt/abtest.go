package embeddingmgmt

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"math"
	mrand "math/rand"
	"sync"
	"time"
)

// ABTestConfig configures an A/B test between two embedding models.
type ABTestConfig struct {
	Name         string    `json:"name"`
	ModelA       string    `json:"model_a"`
	ModelB       string    `json:"model_b"`
	TrafficSplit float64   `json:"traffic_split"` // 0.0-1.0, fraction routed to Model B
	Collection   string    `json:"collection"`
	StartedAt    time.Time `json:"started_at"`
	Metrics      []string  `json:"metrics"`
}

// ABTestResult contains aggregated results for an A/B test.
type ABTestResult struct {
	Config      ABTestConfig `json:"config"`
	ModelAStats ModelMetrics `json:"model_a_stats"`
	ModelBStats ModelMetrics `json:"model_b_stats"`
	Winner      string       `json:"winner,omitempty"`
	Confidence  float64      `json:"confidence"`
	SampleSize  [2]int       `json:"sample_size"`
}

// ModelMetrics tracks performance metrics for one model in an A/B test.
type ModelMetrics struct {
	AvgLatency    float64 `json:"avg_latency_ms"`
	AvgSimilarity float64 `json:"avg_similarity"`
	QueryCount    int64   `json:"query_count"`
	ErrorRate     float64 `json:"error_rate"`

	totalLatency    float64
	totalSimilarity float64
	sumSqSimilarity float64 // for variance calculation
	errorCount      int64
}

// A/B tester errors.
var (
	ErrTestNotFound = errors.New("a/b test not found")
	ErrTestExists   = errors.New("a/b test already exists")
	ErrTestStopped  = errors.New("a/b test is stopped")
)

type abTest struct {
	config  ABTestConfig
	modelA  ModelMetrics
	modelB  ModelMetrics
	stopped bool
}

// ABTester manages A/B tests between embedding models.
type ABTester struct {
	mu    sync.RWMutex
	mgr   *Manager
	tests map[string]*abTest
	rng   *mrand.Rand
}

// NewABTester creates a new A/B tester.
func NewABTester(mgr *Manager) *ABTester {
	var seed int64
	if err := binary.Read(rand.Reader, binary.LittleEndian, &seed); err != nil {
		seed = time.Now().UnixNano()
	}
	return &ABTester{
		mgr:   mgr,
		tests: make(map[string]*abTest),
		rng:   mrand.New(mrand.NewSource(seed)), //nolint:gosec // Traffic splitting doesn't need crypto-grade randomness.
	}
}

// CreateTest creates a new A/B test between two models.
func (t *ABTester) CreateTest(cfg ABTestConfig) (*ABTestConfig, error) {
	if cfg.Name == "" {
		return nil, errors.New("test name is required")
	}
	if cfg.TrafficSplit < 0 || cfg.TrafficSplit > 1 {
		return nil, errors.New("traffic split must be between 0.0 and 1.0")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.tests[cfg.Name]; exists {
		return nil, ErrTestExists
	}

	if cfg.StartedAt.IsZero() {
		cfg.StartedAt = time.Now()
	}

	t.tests[cfg.Name] = &abTest{
		config: cfg,
	}

	return &cfg, nil
}

// RouteQuery returns the model ID to use for a query based on traffic split.
func (t *ABTester) RouteQuery(testName string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	test, exists := t.tests[testName]
	if !exists || test.stopped {
		return ""
	}

	if t.rng.Float64() < test.config.TrafficSplit {
		return test.config.ModelB
	}
	return test.config.ModelA
}

// RecordResult records a query result for an A/B test.
func (t *ABTester) RecordResult(testName, modelID string, latency time.Duration, similarity float64, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	test, exists := t.tests[testName]
	if !exists || test.stopped {
		return
	}

	latencyMs := float64(latency.Microseconds()) / 1000.0

	var metrics *ModelMetrics
	switch modelID {
	case test.config.ModelA:
		metrics = &test.modelA
	case test.config.ModelB:
		metrics = &test.modelB
	default:
		return
	}

	metrics.QueryCount++
	metrics.totalLatency += latencyMs
	metrics.totalSimilarity += similarity
	metrics.sumSqSimilarity += similarity * similarity
	if err != nil {
		metrics.errorCount++
	}

	// Update running averages
	metrics.AvgLatency = metrics.totalLatency / float64(metrics.QueryCount)
	metrics.AvgSimilarity = metrics.totalSimilarity / float64(metrics.QueryCount)
	metrics.ErrorRate = float64(metrics.errorCount) / float64(metrics.QueryCount)
}

// GetResults returns the current A/B test results with statistical analysis.
func (t *ABTester) GetResults(testName string) (*ABTestResult, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	test, exists := t.tests[testName]
	if !exists {
		return nil, ErrTestNotFound
	}

	result := &ABTestResult{
		Config:      test.config,
		ModelAStats: test.modelA,
		ModelBStats: test.modelB,
		SampleSize:  [2]int{int(test.modelA.QueryCount), int(test.modelB.QueryCount)},
	}

	// Determine winner using similarity score comparison with z-test
	if test.modelA.QueryCount > 0 && test.modelB.QueryCount > 0 {
		varA := computeVariance(test.modelA.sumSqSimilarity, test.modelA.totalSimilarity, int(test.modelA.QueryCount))
		varB := computeVariance(test.modelB.sumSqSimilarity, test.modelB.totalSimilarity, int(test.modelB.QueryCount))
		result.Confidence = zTestConfidence(
			test.modelA.AvgSimilarity, test.modelB.AvgSimilarity,
			varA, varB,
			int(test.modelA.QueryCount), int(test.modelB.QueryCount),
		)

		if result.Confidence >= 0.95 {
			if test.modelA.AvgSimilarity > test.modelB.AvgSimilarity {
				result.Winner = test.config.ModelA
			} else {
				result.Winner = test.config.ModelB
			}
		}
	}

	return result, nil
}

// ListTests returns all A/B test configurations.
func (t *ABTester) ListTests() []ABTestConfig {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]ABTestConfig, 0, len(t.tests))
	for _, test := range t.tests {
		result = append(result, test.config)
	}
	return result
}

// StopTest stops an active A/B test.
func (t *ABTester) StopTest(testName string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	test, exists := t.tests[testName]
	if !exists {
		return ErrTestNotFound
	}

	test.stopped = true
	return nil
}

// computeVariance returns the sample variance given sum of squares, sum, and count.
func computeVariance(sumSq, sum float64, n int) float64 {
	if n < 2 {
		return 0
	}
	mean := sum / float64(n)
	v := (sumSq / float64(n)) - mean*mean
	if v < 0 {
		v = 0
	}
	return v
}

// zTestConfidence computes an approximate confidence level for the difference
// between two means using Welch's z-test.
func zTestConfidence(meanA, meanB, varA, varB float64, nA, nB int) float64 {
	if nA < 2 || nB < 2 {
		return 0
	}

	seA := varA / float64(nA)
	seB := varB / float64(nB)
	se := math.Sqrt(seA + seB)

	// When variance is zero (all identical values), use a small floor
	// to avoid division by zero while still reflecting high confidence.
	if se < 1e-12 {
		if math.Abs(meanA-meanB) < 1e-12 {
			return 0
		}
		return 1.0
	}

	z := math.Abs(meanA-meanB) / se

	// Approximate p-value using the complementary error function
	p := math.Erfc(z / math.Sqrt2)

	return 1 - p
}
