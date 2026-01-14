package federation

import (
	"errors"
	"math"
	"math/rand/v2"
	"sync"
	"time"
)

// PrivacyEngine provides differential privacy for federated queries.
type PrivacyEngine struct {
	mu       sync.RWMutex
	config   PrivacyConfig
	budgets  map[string]*PrivacyBudget // per-org privacy budgets
	auditLog []PrivacyAuditEntry
}

// PrivacyConfig configures the privacy engine.
type PrivacyConfig struct {
	DefaultEpsilon  float64       `json:"default_epsilon"`  // Privacy loss parameter (lower = more private)
	DefaultDelta    float64       `json:"default_delta"`    // Probability of privacy breach
	MaxEpsilon      float64       `json:"max_epsilon"`      // Maximum allowed epsilon
	BudgetWindow    time.Duration `json:"budget_window"`    // Rolling window for budget tracking
	MinKAnonymity   int           `json:"min_k_anonymity"`  // Minimum k for k-anonymity
	NoiseMultiplier float64       `json:"noise_multiplier"` // Gaussian noise multiplier
}

// DefaultPrivacyConfig returns sensible privacy defaults.
func DefaultPrivacyConfig() PrivacyConfig {
	return PrivacyConfig{
		DefaultEpsilon:  1.0,
		DefaultDelta:    1e-5,
		MaxEpsilon:      10.0,
		BudgetWindow:    24 * time.Hour,
		MinKAnonymity:   5,
		NoiseMultiplier: 1.1,
	}
}

// PrivacyBudget tracks the privacy budget consumption for an organization.
type PrivacyBudget struct {
	OrgID        string    `json:"org_id"`
	TotalEpsilon float64   `json:"total_epsilon"`
	UsedEpsilon  float64   `json:"used_epsilon"`
	QueryCount   int64     `json:"query_count"`
	WindowStart  time.Time `json:"window_start"`
	LastQuery    time.Time `json:"last_query"`
}

// PrivacyAuditEntry records a privacy-related action.
type PrivacyAuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	OrgID     string    `json:"org_id"`
	Action    string    `json:"action"`
	Epsilon   float64   `json:"epsilon_used"`
	QueryType string    `json:"query_type"`
	Approved  bool      `json:"approved"`
}

// FederatedQueryRequest represents a privacy-preserving query.
type FederatedQueryRequest struct {
	QueryID     string   `json:"query_id"`
	OrgID       string   `json:"org_id"`
	Features    []string `json:"features"`
	Aggregation string   `json:"aggregation"` // "sum", "avg", "count", "min", "max"
	Epsilon     float64  `json:"epsilon"`
	Delta       float64  `json:"delta"`
}

// FederatedQueryResult holds the differentially-private result.
type FederatedQueryResult struct {
	QueryID    string             `json:"query_id"`
	Results    map[string]float64 `json:"results"`
	Epsilon    float64            `json:"epsilon_used"`
	NoiseAdded bool               `json:"noise_added"`
	KAnonymity int                `json:"k_anonymity"`
	Timestamp  time.Time          `json:"timestamp"`
}

var (
	// ErrOrgAlreadyRegistered is returned when an org is already registered.
	ErrOrgAlreadyRegistered = errors.New("org already registered")

	// ErrOrgNotFound is returned when an org is not found.
	ErrOrgNotFound = errors.New("org not found")

	// ErrBudgetExceeded is returned when the privacy budget is exceeded.
	ErrBudgetExceeded = errors.New("privacy budget exceeded")

	// ErrKAnonymityViolation is returned when k-anonymity threshold is not met.
	ErrKAnonymityViolation = errors.New("k-anonymity violation")

	// ErrInvalidAggregation is returned for unsupported aggregation types.
	ErrInvalidAggregation = errors.New("invalid aggregation type")
)

// NewPrivacyEngine creates a new PrivacyEngine with the given config.
func NewPrivacyEngine(config PrivacyConfig) *PrivacyEngine {
	return &PrivacyEngine{
		config:   config,
		budgets:  make(map[string]*PrivacyBudget),
		auditLog: make([]PrivacyAuditEntry, 0),
	}
}

// RegisterOrg sets up a privacy budget for an organization.
func (pe *PrivacyEngine) RegisterOrg(orgID string, totalEpsilon float64) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if _, exists := pe.budgets[orgID]; exists {
		return ErrOrgAlreadyRegistered
	}

	pe.budgets[orgID] = &PrivacyBudget{
		OrgID:        orgID,
		TotalEpsilon: totalEpsilon,
		UsedEpsilon:  0,
		QueryCount:   0,
		WindowStart:  time.Now(),
	}

	return nil
}

// CheckBudget verifies that the org has sufficient privacy budget remaining.
func (pe *PrivacyEngine) CheckBudget(orgID string, requestedEpsilon float64) error {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	budget, exists := pe.budgets[orgID]
	if !exists {
		return ErrOrgNotFound
	}

	if budget.UsedEpsilon+requestedEpsilon > budget.TotalEpsilon {
		return ErrBudgetExceeded
	}

	return nil
}

// ConsumeBudget deducts epsilon from an org's privacy budget.
func (pe *PrivacyEngine) ConsumeBudget(orgID string, epsilon float64) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	budget, exists := pe.budgets[orgID]
	if !exists {
		return
	}

	budget.UsedEpsilon += epsilon
	budget.QueryCount++
	budget.LastQuery = time.Now()
}

// AddLaplaceNoise adds Laplace noise to a value for differential privacy.
func (pe *PrivacyEngine) AddLaplaceNoise(value, sensitivity, epsilon float64) float64 {
	scale := sensitivity / epsilon
	u := rand.Float64() - 0.5
	sign := 1.0
	if u < 0 {
		sign = -1.0
	}
	noise := -sign * scale * math.Log(1-2*math.Abs(u))
	return value + noise
}

// AddGaussianNoise adds Gaussian noise to a value for differential privacy.
func (pe *PrivacyEngine) AddGaussianNoise(value, sensitivity, epsilon, delta float64) float64 {
	sigma := pe.config.NoiseMultiplier * sensitivity * math.Sqrt(2*math.Log(1.25/delta)) / epsilon
	noise := rand.NormFloat64() * sigma
	return value + noise
}

// CheckKAnonymity verifies that the group size meets the k-anonymity threshold.
func (pe *PrivacyEngine) CheckKAnonymity(groupSize int) error {
	if groupSize < pe.config.MinKAnonymity {
		return ErrKAnonymityViolation
	}
	return nil
}

// ExecutePrivateQuery executes a federated query with differential privacy guarantees.
func (pe *PrivacyEngine) ExecutePrivateQuery(req *FederatedQueryRequest, rawValues map[string][]float64) (*FederatedQueryResult, error) {
	epsilon := req.Epsilon
	if epsilon == 0 {
		epsilon = pe.config.DefaultEpsilon
	}
	delta := req.Delta
	if delta == 0 {
		delta = pe.config.DefaultDelta
	}

	if err := pe.CheckBudget(req.OrgID, epsilon); err != nil {
		pe.logAudit(req.OrgID, "query_denied", epsilon, req.Aggregation, false)
		return nil, err
	}

	results := make(map[string]float64)
	minGroupSize := math.MaxInt

	for feature, values := range rawValues {
		if len(values) < minGroupSize {
			minGroupSize = len(values)
		}

		aggregated, err := aggregate(values, req.Aggregation)
		if err != nil {
			return nil, err
		}

		sensitivity := computeSensitivity(values, req.Aggregation)

		var noisy float64
		switch req.Aggregation {
		case "avg":
			noisy = pe.AddGaussianNoise(aggregated, sensitivity, epsilon, delta)
		default:
			noisy = pe.AddLaplaceNoise(aggregated, sensitivity, epsilon)
		}

		results[feature] = noisy
	}

	if err := pe.CheckKAnonymity(minGroupSize); err != nil {
		pe.logAudit(req.OrgID, "k_anonymity_violation", epsilon, req.Aggregation, false)
		return nil, err
	}

	pe.ConsumeBudget(req.OrgID, epsilon)
	pe.logAudit(req.OrgID, "query_executed", epsilon, req.Aggregation, true)

	return &FederatedQueryResult{
		QueryID:    req.QueryID,
		Results:    results,
		Epsilon:    epsilon,
		NoiseAdded: true,
		KAnonymity: minGroupSize,
		Timestamp:  time.Now(),
	}, nil
}

// GetBudget returns the privacy budget for an organization.
func (pe *PrivacyEngine) GetBudget(orgID string) (*PrivacyBudget, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	budget, exists := pe.budgets[orgID]
	if !exists {
		return nil, ErrOrgNotFound
	}

	return budget, nil
}

// GetAuditLog returns the privacy audit log.
func (pe *PrivacyEngine) GetAuditLog() []PrivacyAuditEntry {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	log := make([]PrivacyAuditEntry, len(pe.auditLog))
	copy(log, pe.auditLog)
	return log
}

// ResetBudget resets the privacy budget for an org for a new window.
func (pe *PrivacyEngine) ResetBudget(orgID string) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	budget, exists := pe.budgets[orgID]
	if !exists {
		return
	}

	budget.UsedEpsilon = 0
	budget.QueryCount = 0
	budget.WindowStart = time.Now()
}

func (pe *PrivacyEngine) logAudit(orgID, action string, epsilon float64, queryType string, approved bool) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	pe.auditLog = append(pe.auditLog, PrivacyAuditEntry{
		Timestamp: time.Now(),
		OrgID:     orgID,
		Action:    action,
		Epsilon:   epsilon,
		QueryType: queryType,
		Approved:  approved,
	})
}

func aggregate(values []float64, aggType string) (float64, error) {
	if len(values) == 0 {
		return 0, nil
	}

	switch aggType {
	case "sum":
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum, nil
	case "avg":
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values)), nil
	case "count":
		return float64(len(values)), nil
	case "min":
		minVal := values[0]
		for _, v := range values[1:] {
			if v < minVal {
				minVal = v
			}
		}
		return minVal, nil
	case "max":
		maxVal := values[0]
		for _, v := range values[1:] {
			if v > maxVal {
				maxVal = v
			}
		}
		return maxVal, nil
	default:
		return 0, ErrInvalidAggregation
	}
}

func computeSensitivity(values []float64, aggType string) float64 {
	if len(values) == 0 {
		return 1.0
	}

	switch aggType {
	case "count":
		return 1.0
	case "sum":
		maxAbs := 0.0
		for _, v := range values {
			if abs := math.Abs(v); abs > maxAbs {
				maxAbs = abs
			}
		}
		return maxAbs
	case "avg":
		maxAbs := 0.0
		for _, v := range values {
			if abs := math.Abs(v); abs > maxAbs {
				maxAbs = abs
			}
		}
		return maxAbs / float64(len(values))
	case "min", "max":
		minVal, maxVal := values[0], values[0]
		for _, v := range values[1:] {
			if v < minVal {
				minVal = v
			}
			if v > maxVal {
				maxVal = v
			}
		}
		return maxVal - minVal
	default:
		return 1.0
	}
}
