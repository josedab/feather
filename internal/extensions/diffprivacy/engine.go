package diffprivacy

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"
)

// Mechanism defines the type of noise mechanism to apply.
type Mechanism string

const (
	MechanismLaplace  Mechanism = "laplace"
	MechanismGaussian Mechanism = "gaussian"
	MechanismLocalDP  Mechanism = "local_dp"
)

// Config holds engine-level configuration.
type Config struct {
	DefaultEpsilon     float64
	DefaultDelta       float64
	DefaultMechanism   Mechanism
	DefaultSensitivity float64
	DefaultMaxBudget   float64
	BudgetRefreshRate  time.Duration
}

// DefaultConfig returns production-ready defaults.
func DefaultConfig() Config {
	return Config{
		DefaultEpsilon:     1.0,
		DefaultDelta:       1e-5,
		DefaultMechanism:   MechanismLaplace,
		DefaultSensitivity: 1.0,
		DefaultMaxBudget:   10.0,
		BudgetRefreshRate:  24 * time.Hour,
	}
}

// FeaturePrivacyConfig defines per-feature privacy parameters.
type FeaturePrivacyConfig struct {
	Epsilon     float64
	Delta       float64
	Mechanism   Mechanism
	Sensitivity float64
	MaxBudget   float64
}

// BudgetInfo reports the remaining privacy budget for a feature.
type BudgetInfo struct {
	TotalEpsilon     float64
	ConsumedEpsilon  float64
	TotalDelta       float64
	ConsumedDelta    float64
	QueriesRemaining int
}

// NoisyAggregation holds the result of a privacy-preserving aggregation.
type NoisyAggregation struct {
	OriginalValue float64
	NoisyValue    float64
	Mechanism     Mechanism
	EpsilonUsed   float64
}

// Stats holds engine-level statistics.
type Stats struct {
	RegisteredFeatures int
	TotalQueries       int64
	BudgetExhaustions  int64
	MechanismCounts    map[Mechanism]int64
}

type featureState struct {
	config          FeaturePrivacyConfig
	consumedEpsilon float64
	consumedDelta   float64
	queryCount      int64
}

// Engine provides differential privacy guarantees for feature retrieval.
type Engine struct {
	mu       sync.RWMutex
	cfg      Config
	features map[string]*featureState
	stats    Stats
}

// NewEngine creates a new differential privacy engine.
func NewEngine(cfg Config) *Engine {
	return &Engine{
		cfg:      cfg,
		features: make(map[string]*featureState),
		stats: Stats{
			MechanismCounts: make(map[Mechanism]int64),
		},
	}
}

// RegisterFeature registers a feature for privacy-aware access.
func (e *Engine) RegisterFeature(name string, cfg FeaturePrivacyConfig) error {
	if name == "" {
		return fmt.Errorf("registering feature: name cannot be empty")
	}
	if cfg.Epsilon <= 0 {
		return fmt.Errorf("registering feature %q: epsilon must be positive", name)
	}
	if cfg.Sensitivity <= 0 {
		return fmt.Errorf("registering feature %q: sensitivity must be positive", name)
	}
	if cfg.Mechanism == MechanismGaussian && cfg.Delta <= 0 {
		return fmt.Errorf("registering feature %q: delta must be positive for gaussian mechanism", name)
	}
	if cfg.MaxBudget <= 0 {
		cfg.MaxBudget = e.cfg.DefaultMaxBudget
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.features[name]; exists {
		return fmt.Errorf("registering feature %q: already registered", name)
	}

	e.features[name] = &featureState{config: cfg}
	e.stats.RegisteredFeatures++
	return nil
}

// AddNoise applies the configured noise mechanism to a value.
func (e *Engine) AddNoise(featureName string, value float64) (float64, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	fs, err := e.getFeatureStateLocked(featureName)
	if err != nil {
		return 0, fmt.Errorf("adding noise to %q: %w", featureName, err)
	}

	if err := e.checkBudgetLocked(fs); err != nil {
		e.stats.BudgetExhaustions++
		return 0, fmt.Errorf("adding noise to %q: %w", featureName, err)
	}

	noisy, err := e.applyMechanism(fs.config.Mechanism, value, fs.config.Sensitivity, fs.config.Epsilon, fs.config.Delta)
	if err != nil {
		return 0, fmt.Errorf("adding noise to %q: %w", featureName, err)
	}

	e.consumeBudgetLocked(fs)
	return noisy, nil
}

// NoisyCount returns a differentially private count.
func (e *Engine) NoisyCount(featureName string, count int64) (NoisyAggregation, error) {
	// Count queries have sensitivity 1.
	return e.noisyAggregation(featureName, float64(count), 1.0)
}

// NoisySum returns a differentially private sum.
func (e *Engine) NoisySum(featureName string, sum float64) (NoisyAggregation, error) {
	e.mu.RLock()
	fs, err := e.getFeatureStateLocked(featureName)
	sensitivity := fs.config.Sensitivity
	e.mu.RUnlock()
	if err != nil {
		return NoisyAggregation{}, fmt.Errorf("noisy sum for %q: %w", featureName, err)
	}
	return e.noisyAggregation(featureName, sum, sensitivity)
}

// NoisyAvg returns a differentially private average.
func (e *Engine) NoisyAvg(featureName string, sum float64, count int64) (NoisyAggregation, error) {
	if count == 0 {
		return NoisyAggregation{}, fmt.Errorf("noisy avg for %q: count cannot be zero", featureName)
	}

	e.mu.RLock()
	fs, err := e.getFeatureStateLocked(featureName)
	sensitivity := fs.config.Sensitivity
	e.mu.RUnlock()
	if err != nil {
		return NoisyAggregation{}, fmt.Errorf("noisy avg for %q: %w", featureName, err)
	}

	// Sensitivity of average = sensitivity / count.
	avgSensitivity := sensitivity / float64(count)
	avg := sum / float64(count)
	return e.noisyAggregation(featureName, avg, avgSensitivity)
}

// NoisyMin returns a differentially private minimum.
func (e *Engine) NoisyMin(featureName string, min float64) (NoisyAggregation, error) {
	e.mu.RLock()
	fs, err := e.getFeatureStateLocked(featureName)
	sensitivity := fs.config.Sensitivity
	e.mu.RUnlock()
	if err != nil {
		return NoisyAggregation{}, fmt.Errorf("noisy min for %q: %w", featureName, err)
	}
	return e.noisyAggregation(featureName, min, sensitivity)
}

// NoisyMax returns a differentially private maximum.
func (e *Engine) NoisyMax(featureName string, max float64) (NoisyAggregation, error) {
	e.mu.RLock()
	fs, err := e.getFeatureStateLocked(featureName)
	sensitivity := fs.config.Sensitivity
	e.mu.RUnlock()
	if err != nil {
		return NoisyAggregation{}, fmt.Errorf("noisy max for %q: %w", featureName, err)
	}
	return e.noisyAggregation(featureName, max, sensitivity)
}

// BudgetStatus returns the remaining privacy budget for a feature.
func (e *Engine) BudgetStatus(featureName string) (BudgetInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	fs, err := e.getFeatureStateLocked(featureName)
	if err != nil {
		return BudgetInfo{}, fmt.Errorf("budget status for %q: %w", featureName, err)
	}

	remaining := int((fs.config.MaxBudget - fs.consumedEpsilon) / fs.config.Epsilon)
	if remaining < 0 {
		remaining = 0
	}

	return BudgetInfo{
		TotalEpsilon:     fs.config.MaxBudget,
		ConsumedEpsilon:  fs.consumedEpsilon,
		TotalDelta:       fs.config.Delta * float64(int(fs.config.MaxBudget/fs.config.Epsilon)),
		ConsumedDelta:    fs.consumedDelta,
		QueriesRemaining: remaining,
	}, nil
}

// Stats returns a snapshot of engine statistics.
func (e *Engine) Stats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	counts := make(map[Mechanism]int64, len(e.stats.MechanismCounts))
	for k, v := range e.stats.MechanismCounts {
		counts[k] = v
	}

	return Stats{
		RegisteredFeatures: e.stats.RegisteredFeatures,
		TotalQueries:       e.stats.TotalQueries,
		BudgetExhaustions:  e.stats.BudgetExhaustions,
		MechanismCounts:    counts,
	}
}

// SequentialComposition returns the total (ε, δ) for k sequential queries
// on the same data, each with per-query (epsilon, delta).
func SequentialComposition(epsilon, delta float64, k int) (totalEpsilon, totalDelta float64) {
	return epsilon * float64(k), delta * float64(k)
}

// ParallelComposition returns the total (ε, δ) for queries on disjoint
// partitions, which equals the maximum single-query budget.
func ParallelComposition(epsilon, delta float64) (totalEpsilon, totalDelta float64) {
	return epsilon, delta
}

// RenyiComposition computes the composed privacy guarantee under Rényi
// differential privacy for k queries with the given alpha order.
// Returns the (ε, δ) guarantee after converting from RDP.
func RenyiComposition(epsilon, delta float64, alpha float64, k int) (totalEpsilon, totalDelta float64) {
	// RDP epsilon for a single Gaussian query at order alpha.
	rdpEpsilon := alpha * epsilon * epsilon / 2.0
	composedRDP := rdpEpsilon * float64(k)

	// Convert back from RDP to (ε, δ)-DP.
	totalEpsilon = composedRDP + math.Log(1.0/delta)/(alpha-1.0)
	totalDelta = delta
	return totalEpsilon, totalDelta
}

// noisyAggregation applies noise with a caller-provided sensitivity and
// consumes privacy budget under the write lock.
func (e *Engine) noisyAggregation(featureName string, value, sensitivity float64) (NoisyAggregation, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	fs, err := e.getFeatureStateLocked(featureName)
	if err != nil {
		return NoisyAggregation{}, err
	}

	if err := e.checkBudgetLocked(fs); err != nil {
		e.stats.BudgetExhaustions++
		return NoisyAggregation{}, fmt.Errorf("budget exhausted for %q: %w", featureName, err)
	}

	noisy, err := e.applyMechanism(fs.config.Mechanism, value, sensitivity, fs.config.Epsilon, fs.config.Delta)
	if err != nil {
		return NoisyAggregation{}, err
	}

	e.consumeBudgetLocked(fs)

	return NoisyAggregation{
		OriginalValue: value,
		NoisyValue:    noisy,
		Mechanism:     fs.config.Mechanism,
		EpsilonUsed:   fs.config.Epsilon,
	}, nil
}

func (e *Engine) getFeatureStateLocked(name string) (*featureState, error) {
	fs, ok := e.features[name]
	if !ok {
		return nil, fmt.Errorf("feature %q not registered", name)
	}
	return fs, nil
}

func (e *Engine) checkBudgetLocked(fs *featureState) error {
	if fs.consumedEpsilon+fs.config.Epsilon > fs.config.MaxBudget {
		return fmt.Errorf("privacy budget exhausted: consumed %.4f of %.4f epsilon",
			fs.consumedEpsilon, fs.config.MaxBudget)
	}
	return nil
}

func (e *Engine) consumeBudgetLocked(fs *featureState) {
	fs.consumedEpsilon += fs.config.Epsilon
	fs.consumedDelta += fs.config.Delta
	fs.queryCount++
	e.stats.TotalQueries++
	e.stats.MechanismCounts[fs.config.Mechanism]++
}

// applyMechanism dispatches to the correct noise generator.
func (e *Engine) applyMechanism(m Mechanism, value, sensitivity, epsilon, delta float64) (float64, error) {
	switch m {
	case MechanismLaplace:
		return e.laplace(value, sensitivity, epsilon)
	case MechanismGaussian:
		return e.gaussian(value, sensitivity, epsilon, delta)
	case MechanismLocalDP:
		return e.randomizedResponse(value, epsilon)
	default:
		return 0, fmt.Errorf("unknown mechanism %q", m)
	}
}

// cryptoFloat64 returns a cryptographically secure random float64 in [0, 1).
// Returns an error if the CSPRNG fails.
func cryptoFloat64() (float64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, fmt.Errorf("crypto/rand failed: %w", err)
	}
	// Use 53 bits of entropy for a uniform float64 in [0, 1).
	return float64(binary.LittleEndian.Uint64(b[:])>>11) / (1 << 53), nil
}

// cryptoNormFloat64 returns a cryptographically secure normally distributed
// float64 using the Box-Muller transform.
func cryptoNormFloat64() (float64, error) {
	u1, err := cryptoFloat64()
	if err != nil {
		return 0, err
	}
	u2, err := cryptoFloat64()
	if err != nil {
		return 0, err
	}
	// Avoid log(0) by clamping u1 away from zero.
	for u1 == 0 {
		u1, err = cryptoFloat64()
		if err != nil {
			return 0, err
		}
	}
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2), nil
}

// laplace adds Laplace noise scaled to sensitivity/epsilon.
func (e *Engine) laplace(value, sensitivity, epsilon float64) (float64, error) {
	scale := sensitivity / epsilon
	// Laplace noise via inverse CDF: L(0, scale) = -scale * sign(u) * ln(1 - 2|u|)
	u, err := cryptoFloat64()
	if err != nil {
		return 0, err
	}
	u -= 0.5
	noise := -scale * math.Copysign(1, u) * math.Log(1-2*math.Abs(u))
	return value + noise, nil
}

// gaussian adds Gaussian noise calibrated so the mechanism satisfies (ε, δ)-DP.
func (e *Engine) gaussian(value, sensitivity, epsilon, delta float64) (float64, error) {
	sigma := sensitivity * math.Sqrt(2*math.Log(1.25/delta)) / epsilon
	norm, err := cryptoNormFloat64()
	if err != nil {
		return 0, err
	}
	noise := norm * sigma
	return value + noise, nil
}

// randomizedResponse implements local DP via randomized response.
// Treats value > 0.5 as "true" and flips the answer with probability
// determined by epsilon. Returns 1.0 (true) or 0.0 (false).
func (e *Engine) randomizedResponse(value, epsilon float64) (float64, error) {
	truthful := value > 0.5
	p := math.Exp(epsilon) / (math.Exp(epsilon) + 1) // probability of telling truth
	r, err := cryptoFloat64()
	if err != nil {
		return 0, err
	}
	if r < p {
		if truthful {
			return 1.0, nil
		}
		return 0.0, nil
	}
	if truthful {
		return 0.0, nil
	}
	return 1.0, nil
}
