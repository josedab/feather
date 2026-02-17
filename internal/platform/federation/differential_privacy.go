package federation

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"sync"
	"time"
)

// DPConfig configures differential privacy mechanisms.
type DPConfig struct {
	Epsilon         float64    `json:"epsilon"`
	Delta           float64    `json:"delta"`
	MaxGradientNorm float64    `json:"max_gradient_norm"`
	NoiseType       string     `json:"noise_type"` // "laplace" or "gaussian"
	ClipBounds      [2]float64 `json:"clip_bounds"`
}

// DefaultDPConfig returns sensible defaults for differential privacy.
func DefaultDPConfig() DPConfig {
	return DPConfig{
		Epsilon:         1.0,
		Delta:           1e-5,
		MaxGradientNorm: 1.0,
		NoiseType:       "gaussian",
		ClipBounds:      [2]float64{-1.0, 1.0},
	}
}

// DPMechanism provides differential privacy noise addition and clipping.
type DPMechanism struct {
	config DPConfig
}

// NewDPMechanism creates a new differential privacy mechanism.
func NewDPMechanism(config DPConfig) *DPMechanism {
	return &DPMechanism{
		config: config,
	}
}

// AddNoise adds calibrated noise to a value based on the configured mechanism.
func (dp *DPMechanism) AddNoise(value float64) float64 {
	sensitivity := dp.config.ClipBounds[1] - dp.config.ClipBounds[0]
	if sensitivity <= 0 {
		sensitivity = 1.0
	}

	switch dp.config.NoiseType {
	case "laplace":
		return value + dp.laplaceNoise(sensitivity)
	default: // gaussian
		return value + dp.gaussianNoise(sensitivity)
	}
}

// AddNoiseVector adds calibrated noise to each element of a vector.
func (dp *DPMechanism) AddNoiseVector(values []float64) []float64 {
	result := make([]float64, len(values))
	for i, v := range values {
		result[i] = dp.AddNoise(v)
	}
	return result
}

// ClipValue clips a value to the configured bounds.
func (dp *DPMechanism) ClipValue(value float64) float64 {
	if value < dp.config.ClipBounds[0] {
		return dp.config.ClipBounds[0]
	}
	if value > dp.config.ClipBounds[1] {
		return dp.config.ClipBounds[1]
	}
	return value
}

// ClipGradient clips a gradient vector by L2 norm.
func (dp *DPMechanism) ClipGradient(gradient []float64) []float64 {
	norm := 0.0
	for _, v := range gradient {
		norm += v * v
	}
	norm = math.Sqrt(norm)

	if norm <= dp.config.MaxGradientNorm {
		result := make([]float64, len(gradient))
		copy(result, gradient)
		return result
	}

	scale := dp.config.MaxGradientNorm / norm
	result := make([]float64, len(gradient))
	for i, v := range gradient {
		result[i] = v * scale
	}
	return result
}

// Config returns the current DP configuration.
func (dp *DPMechanism) Config() DPConfig {
	return dp.config
}

// laplaceNoise generates Laplace noise using crypto/rand.
func (dp *DPMechanism) laplaceNoise(sensitivity float64) float64 {
	scale := sensitivity / dp.config.Epsilon
	u := dp.cryptoRandFloat64() - 0.5
	sign := 1.0
	if u < 0 {
		sign = -1.0
	}
	return -sign * scale * math.Log(1-2*math.Abs(u))
}

// gaussianNoise generates Gaussian noise using crypto/rand via Box-Muller.
func (dp *DPMechanism) gaussianNoise(sensitivity float64) float64 {
	sigma := sensitivity * math.Sqrt(2*math.Log(1.25/dp.config.Delta)) / dp.config.Epsilon

	u1 := dp.cryptoRandFloat64()
	u2 := dp.cryptoRandFloat64()
	// Avoid log(0)
	for u1 == 0 {
		u1 = dp.cryptoRandFloat64()
	}
	normal := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
	return normal * sigma
}

// cryptoRandFloat64 returns a cryptographically random float64 in [0, 1).
func (dp *DPMechanism) cryptoRandFloat64() float64 {
	max := big.NewInt(1 << 53)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		// Fallback: read 8 bytes from crypto/rand
		var buf [8]byte
		if _, readErr := rand.Read(buf[:]); readErr != nil {
			return 0 // safe fallback for privacy noise
		}
		bits := binary.LittleEndian.Uint64(buf[:])
		return float64(bits>>11) / float64(1<<53)
	}
	return float64(n.Int64()) / float64(1<<53)
}

// DPBudget tracks cumulative privacy spend.
type DPBudget struct {
	mu           sync.Mutex
	totalEpsilon float64
	totalDelta   float64
	maxEpsilon   float64
	maxDelta     float64
}

// NewDPBudget creates a new privacy budget with the given limits.
func NewDPBudget(maxEpsilon, maxDelta float64) *DPBudget {
	return &DPBudget{
		maxEpsilon: maxEpsilon,
		maxDelta:   maxDelta,
	}
}

// Consume deducts from the privacy budget.
func (b *DPBudget) Consume(epsilon, delta float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.totalEpsilon+epsilon > b.maxEpsilon {
		return fmt.Errorf("privacy budget exceeded: would use %.4f of %.4f epsilon", b.totalEpsilon+epsilon, b.maxEpsilon)
	}
	if b.totalDelta+delta > b.maxDelta {
		return fmt.Errorf("privacy budget exceeded: would use %e of %e delta", b.totalDelta+delta, b.maxDelta)
	}

	b.totalEpsilon += epsilon
	b.totalDelta += delta
	return nil
}

// Remaining returns the remaining epsilon and delta budget.
func (b *DPBudget) Remaining() (float64, float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.maxEpsilon - b.totalEpsilon, b.maxDelta - b.totalDelta
}

// IsExhausted returns true if the budget is fully consumed.
func (b *DPBudget) IsExhausted() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.totalEpsilon >= b.maxEpsilon || b.totalDelta >= b.maxDelta
}

// Reset resets the privacy budget to zero.
func (b *DPBudget) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.totalEpsilon = 0
	b.totalDelta = 0
}

// PrivacyQuery records a single privacy-consuming query.
type PrivacyQuery struct {
	QueryID   string    `json:"query_id"`
	Epsilon   float64   `json:"epsilon"`
	Delta     float64   `json:"delta"`
	Timestamp time.Time `json:"timestamp"`
}

// PrivacyReport summarizes the privacy accounting state.
type PrivacyReport struct {
	TotalQueries  int            `json:"total_queries"`
	TotalEpsilon  float64        `json:"total_epsilon"`
	TotalDelta    float64        `json:"total_delta"`
	BudgetUsedPct float64        `json:"budget_used_pct"`
	Queries       []PrivacyQuery `json:"queries"`
}

// PrivacyAccountant tracks privacy queries and generates reports.
type PrivacyAccountant struct {
	mu      sync.RWMutex
	budget  *DPBudget
	queries []PrivacyQuery
}

// NewPrivacyAccountant creates a new privacy accountant with the given budget.
func NewPrivacyAccountant(budget *DPBudget) *PrivacyAccountant {
	return &PrivacyAccountant{
		budget:  budget,
		queries: make([]PrivacyQuery, 0),
	}
}

// Track records a privacy-consuming query.
func (pa *PrivacyAccountant) Track(queryID string, epsilon, delta float64) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	pa.queries = append(pa.queries, PrivacyQuery{
		QueryID:   queryID,
		Epsilon:   epsilon,
		Delta:     delta,
		Timestamp: time.Now(),
	})
}

// GetReport returns a summary of all tracked privacy queries.
func (pa *PrivacyAccountant) GetReport() *PrivacyReport {
	pa.mu.RLock()
	defer pa.mu.RUnlock()

	totalEpsilon := 0.0
	totalDelta := 0.0
	for _, q := range pa.queries {
		totalEpsilon += q.Epsilon
		totalDelta += q.Delta
	}

	budgetUsedPct := 0.0
	if pa.budget != nil {
		remEps, _ := pa.budget.Remaining()
		maxEps := remEps + totalEpsilon
		if maxEps > 0 {
			budgetUsedPct = (totalEpsilon / maxEps) * 100.0
		}
	}

	queries := make([]PrivacyQuery, len(pa.queries))
	copy(queries, pa.queries)

	return &PrivacyReport{
		TotalQueries:  len(pa.queries),
		TotalEpsilon:  totalEpsilon,
		TotalDelta:    totalDelta,
		BudgetUsedPct: budgetUsedPct,
		Queries:       queries,
	}
}
