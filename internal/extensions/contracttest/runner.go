package contracttest

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// ContractType identifies the type of contract validation.
type ContractType string

// Contract types.
const (
	SchemaContract       ContractType = "schema"
	DistributionContract ContractType = "distribution"
	FreshnessContract    ContractType = "freshness"
	RangeContract        ContractType = "range"
)

// Contract defines validation rules for a feature group.
type Contract struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	FeatureGroup string                 `json:"feature_group"`
	Type         ContractType           `json:"type"`
	Rules        map[string]interface{} `json:"rules"`
	Severity     string                 `json:"severity"` // "warn" or "block"
	CreatedAt    time.Time              `json:"created_at"`
}

// TestResult represents the outcome of a contract validation.
type TestResult struct {
	ContractID string
	Passed     bool
	Violations []Violation
	TestedAt   time.Time
	DurationMs float64
}

// Violation describes a specific contract rule violation.
type Violation struct {
	Rule     string
	Expected string
	Actual   string
	Message  string
}

// RunnerConfig configures the contract test runner.
type RunnerConfig struct {
	// MaxContracts is the maximum number of contracts to store.
	MaxContracts int

	// DefaultSeverity is the default severity for new contracts.
	DefaultSeverity string
}

// DefaultRunnerConfig returns sensible defaults.
func DefaultRunnerConfig() RunnerConfig {
	return RunnerConfig{
		MaxContracts:    10000,
		DefaultSeverity: "warn",
	}
}

// RunnerStats contains runner statistics.
type RunnerStats struct {
	TotalContracts     int
	TotalRuns          int
	PassRate           float64
	BlockingViolations int
}

// Runner manages contracts and executes validation tests.
type Runner struct {
	mu        sync.RWMutex
	config    RunnerConfig
	contracts map[string]*Contract
	results   []TestResult
}

// NewRunner creates a new contract test runner.
func NewRunner(config RunnerConfig) *Runner {
	if config.MaxContracts == 0 {
		config = DefaultRunnerConfig()
	}

	return &Runner{
		config:    config,
		contracts: make(map[string]*Contract),
		results:   make([]TestResult, 0),
	}
}

// RegisterContract validates and stores a contract.
func (r *Runner) RegisterContract(c Contract) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if c.ID == "" || c.Name == "" {
		return fmt.Errorf("%w: ID and Name are required", ErrInvalidContract)
	}

	if c.Type == "" {
		return fmt.Errorf("%w: Type is required", ErrInvalidContract)
	}

	if len(r.contracts) >= r.config.MaxContracts {
		return fmt.Errorf("%w: maximum contracts reached", ErrInvalidContract)
	}

	if c.Severity == "" {
		c.Severity = r.config.DefaultSeverity
	}

	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}

	stored := c
	r.contracts[c.ID] = &stored
	return nil
}

// ListContracts returns all registered contracts.
func (r *Runner) ListContracts() []Contract {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Contract, 0, len(r.contracts))
	for _, c := range r.contracts {
		result = append(result, *c)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result
}

// GetContract returns a contract by ID.
func (r *Runner) GetContract(id string) (*Contract, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, exists := r.contracts[id]
	if !exists {
		return nil, ErrContractNotFound
	}

	result := *c
	return &result, nil
}

// DeleteContract removes a contract by ID.
func (r *Runner) DeleteContract(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.contracts[id]; !exists {
		return ErrContractNotFound
	}

	delete(r.contracts, id)
	return nil
}

// ValidateSchema validates field existence and types against a schema contract.
func (r *Runner) ValidateSchema(contractID string, fields map[string]string) TestResult {
	start := time.Now()

	r.mu.RLock()
	c, exists := r.contracts[contractID]
	r.mu.RUnlock()

	result := TestResult{
		ContractID: contractID,
		Passed:     true,
		TestedAt:   start,
	}

	if !exists {
		result.Passed = false
		result.Violations = []Violation{{
			Rule:    "contract_exists",
			Message: "contract not found",
		}}
		result.DurationMs = float64(time.Since(start).Microseconds()) / 1000.0
		r.mu.Lock()
		r.results = append(r.results, result)
		r.mu.Unlock()
		return result
	}

	// Check required fields from rules
	for ruleName, ruleVal := range c.Rules {
		expectedType, ok := ruleVal.(string)
		if !ok {
			continue
		}

		actualType, fieldExists := fields[ruleName]
		if !fieldExists {
			result.Passed = false
			result.Violations = append(result.Violations, Violation{
				Rule:     ruleName,
				Expected: expectedType,
				Actual:   "<missing>",
				Message:  fmt.Sprintf("required field %q is missing", ruleName),
			})
			continue
		}

		if actualType != expectedType {
			result.Passed = false
			result.Violations = append(result.Violations, Violation{
				Rule:     ruleName,
				Expected: expectedType,
				Actual:   actualType,
				Message:  fmt.Sprintf("field %q has type %q, expected %q", ruleName, actualType, expectedType),
			})
		}
	}

	result.DurationMs = float64(time.Since(start).Microseconds()) / 1000.0

	r.mu.Lock()
	r.results = append(r.results, result)
	r.mu.Unlock()

	return result
}

// ValidateRange validates that values fall within configured min/max ranges.
func (r *Runner) ValidateRange(contractID string, values map[string]float64) TestResult {
	start := time.Now()

	r.mu.RLock()
	c, exists := r.contracts[contractID]
	r.mu.RUnlock()

	result := TestResult{
		ContractID: contractID,
		Passed:     true,
		TestedAt:   start,
	}

	if !exists {
		result.Passed = false
		result.Violations = []Violation{{
			Rule:    "contract_exists",
			Message: "contract not found",
		}}
		result.DurationMs = float64(time.Since(start).Microseconds()) / 1000.0
		r.mu.Lock()
		r.results = append(r.results, result)
		r.mu.Unlock()
		return result
	}

	for fieldName, value := range values {
		ruleVal, ruleExists := c.Rules[fieldName]
		if !ruleExists {
			continue
		}

		rangeMap, ok := ruleVal.(map[string]interface{})
		if !ok {
			continue
		}

		if minVal, ok := rangeMap["min"]; ok {
			if minFloat, ok := toFloat64(minVal); ok && value < minFloat {
				result.Passed = false
				result.Violations = append(result.Violations, Violation{
					Rule:     fieldName,
					Expected: fmt.Sprintf(">= %v", minFloat),
					Actual:   fmt.Sprintf("%v", value),
					Message:  fmt.Sprintf("field %q value %v is below minimum %v", fieldName, value, minFloat),
				})
			}
		}

		if maxVal, ok := rangeMap["max"]; ok {
			if maxFloat, ok := toFloat64(maxVal); ok && value > maxFloat {
				result.Passed = false
				result.Violations = append(result.Violations, Violation{
					Rule:     fieldName,
					Expected: fmt.Sprintf("<= %v", maxFloat),
					Actual:   fmt.Sprintf("%v", value),
					Message:  fmt.Sprintf("field %q value %v is above maximum %v", fieldName, value, maxFloat),
				})
			}
		}
	}

	result.DurationMs = float64(time.Since(start).Microseconds()) / 1000.0

	r.mu.Lock()
	r.results = append(r.results, result)
	r.mu.Unlock()

	return result
}

// RunAll runs all registered contracts against the provided data.
func (r *Runner) RunAll(schemaData map[string]string, rangeData map[string]float64) []TestResult {
	r.mu.RLock()
	ids := make([]string, 0, len(r.contracts))
	for id, c := range r.contracts {
		_ = c
		ids = append(ids, id)
	}
	r.mu.RUnlock()

	results := make([]TestResult, 0, len(ids))
	for _, id := range ids {
		r.mu.RLock()
		c := r.contracts[id]
		r.mu.RUnlock()

		switch c.Type {
		case SchemaContract:
			results = append(results, r.ValidateSchema(id, schemaData))
		case RangeContract:
			results = append(results, r.ValidateRange(id, rangeData))
		}
	}

	return results
}

// GetResults returns test results for a specific contract, limited to the most recent.
func (r *Runner) GetResults(contractID string, limit int) []TestResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []TestResult
	for _, res := range r.results {
		if res.ContractID == contractID {
			filtered = append(filtered, res)
		}
	}

	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	return filtered
}

// Stats returns runner statistics.
func (r *Runner) Stats() RunnerStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := RunnerStats{
		TotalContracts: len(r.contracts),
		TotalRuns:      len(r.results),
	}

	if stats.TotalRuns > 0 {
		passed := 0
		for _, res := range r.results {
			if res.Passed {
				passed++
			} else {
				// Check if blocking
				if c, exists := r.contracts[res.ContractID]; exists && c.Severity == "block" {
					stats.BlockingViolations++
				}
			}
		}
		stats.PassRate = float64(passed) / float64(stats.TotalRuns)
	}

	return stats
}

// toFloat64 converts an interface{} to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}
