package contract

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// RuleType identifies the type of contract rule.
type RuleType string

const (
	// RuleFreshness enforces maximum staleness for feature data.
	RuleFreshness RuleType = "freshness"
	// RuleCompleteness enforces minimum non-null ratio.
	RuleCompleteness RuleType = "completeness"
	// RuleDistribution enforces value bounds for numeric features.
	RuleDistribution RuleType = "distribution"
	// RuleSchema enforces data type constraints.
	RuleSchema RuleType = "schema"
	// RuleCustom allows user-defined validation logic.
	RuleCustom RuleType = "custom"
)

// Severity indicates the importance of a contract violation.
type Severity string

const (
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// Status represents the overall contract health.
type Status string

const (
	StatusPassing  Status = "passing"
	StatusWarning  Status = "warning"
	StatusBreached Status = "breached"
	StatusUnknown  Status = "unknown"
)

var (
	ErrContractNotFound = errors.New("contract not found")
	ErrContractExists   = errors.New("contract already exists")
	ErrInvalidContract  = errors.New("invalid contract specification")
	ErrNoRules          = errors.New("contract must have at least one rule")
	ErrInvalidRuleType  = errors.New("unknown rule type")
)

// Rule defines a single constraint within a contract.
type Rule struct {
	// Type is the kind of rule.
	Type RuleType `json:"type"`
	// Severity determines alert priority when violated.
	Severity Severity `json:"severity"`
	// Freshness rule fields
	MaxStaleness time.Duration `json:"max_staleness,omitempty"`
	// Completeness rule fields
	MinCompleteness float64 `json:"min_completeness,omitempty"`
	// Distribution rule fields
	MinValue *float64 `json:"min_value,omitempty"`
	MaxValue *float64 `json:"max_value,omitempty"`
	MeanMin  *float64 `json:"mean_min,omitempty"`
	MeanMax  *float64 `json:"mean_max,omitempty"`
	// Schema rule fields
	ExpectedType string `json:"expected_type,omitempty"`
	// Custom rule name (evaluated by external hook)
	CustomRule string `json:"custom_rule,omitempty"`
}

// Spec defines a complete feature contract.
type Spec struct {
	// Name uniquely identifies this contract.
	Name string `json:"name"`
	// Description explains the contract's purpose.
	Description string `json:"description,omitempty"`
	// FeatureGroup is the group this contract applies to.
	FeatureGroup string `json:"feature_group"`
	// FeatureName is the specific feature (empty means all in group).
	FeatureName string `json:"feature_name,omitempty"`
	// Rules are the constraints that must be satisfied.
	Rules []Rule `json:"rules"`
	// Owner identifies the team responsible.
	Owner string `json:"owner,omitempty"`
	// CreatedAt is when the contract was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the contract was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// Violation records a single contract breach.
type Violation struct {
	// ContractName is the name of the breached contract.
	ContractName string `json:"contract_name"`
	// RuleType is the type of rule that was violated.
	RuleType RuleType `json:"rule_type"`
	// Severity of this violation.
	Severity Severity `json:"severity"`
	// Message describes the violation.
	Message string `json:"message"`
	// ActualValue is the observed value.
	ActualValue float64 `json:"actual_value"`
	// ExpectedValue is the threshold that was breached.
	ExpectedValue float64 `json:"expected_value"`
	// Timestamp of the violation.
	Timestamp time.Time `json:"timestamp"`
}

// ContractStatus reports the current health of a contract.
type ContractStatus struct {
	// Name of the contract.
	Name string `json:"name"`
	// Status is the overall health.
	Status Status `json:"status"`
	// LastEvaluated is when the contract was last checked.
	LastEvaluated time.Time `json:"last_evaluated"`
	// Violations are active breaches.
	Violations []Violation `json:"violations,omitempty"`
	// RulesPassing is the count of passing rules.
	RulesPassing int `json:"rules_passing"`
	// RulesTotal is the total number of rules.
	RulesTotal int `json:"rules_total"`
}

// FeatureMetrics provides measured values for contract evaluation.
type FeatureMetrics struct {
	// LastUpdated is the most recent data timestamp.
	LastUpdated time.Time
	// Completeness is the ratio of non-null values (0.0-1.0).
	Completeness float64
	// Mean is the average value for numeric features.
	Mean float64
	// Min is the minimum observed value.
	Min float64
	// Max is the maximum observed value.
	Max float64
	// DataType is the observed data type.
	DataType string
	// SampleCount is the number of samples evaluated.
	SampleCount int64
}

// MetricsProvider supplies feature metrics for contract evaluation.
type MetricsProvider interface {
	GetFeatureMetrics(ctx context.Context, group, feature string) (*FeatureMetrics, error)
}

// AlertHandler receives contract violation notifications.
type AlertHandler interface {
	HandleViolation(ctx context.Context, violation Violation) error
}

// ManagerConfig configures the contract manager.
type ManagerConfig struct {
	// EvalInterval is how often contracts are evaluated.
	EvalInterval time.Duration
	// ViolationRetention is how long violations are kept.
	ViolationRetention time.Duration
	// MaxViolations caps stored violations.
	MaxViolations int
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		EvalInterval:       time.Minute,
		ViolationRetention: 24 * time.Hour,
		MaxViolations:      10000,
	}
}

// Manager manages feature contracts and evaluates them periodically.
type Manager struct {
	mu         sync.RWMutex
	contracts  map[string]*Spec
	statuses   map[string]*ContractStatus
	violations []Violation
	config     ManagerConfig
	provider   MetricsProvider
	handlers   []AlertHandler
	stopCh     chan struct{}
}

// NewManager creates a new contract manager.
func NewManager(config ManagerConfig, provider MetricsProvider) *Manager {
	if config.EvalInterval == 0 {
		config = DefaultManagerConfig()
	}
	return &Manager{
		contracts:  make(map[string]*Spec),
		statuses:   make(map[string]*ContractStatus),
		violations: make([]Violation, 0),
		config:     config,
		provider:   provider,
		handlers:   make([]AlertHandler, 0),
		stopCh:     make(chan struct{}),
	}
}

// RegisterAlert adds an alert handler for violations.
func (m *Manager) RegisterAlert(handler AlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
}

// CreateContract registers a new contract.
func (m *Manager) CreateContract(spec *Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidContract)
	}
	if spec.FeatureGroup == "" {
		return fmt.Errorf("%w: feature_group is required", ErrInvalidContract)
	}
	if len(spec.Rules) == 0 {
		return ErrNoRules
	}
	for _, r := range spec.Rules {
		if err := validateRule(r); err != nil {
			return err
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.contracts[spec.Name]; exists {
		return ErrContractExists
	}

	now := time.Now()
	spec.CreatedAt = now
	spec.UpdatedAt = now
	m.contracts[spec.Name] = spec
	m.statuses[spec.Name] = &ContractStatus{
		Name:       spec.Name,
		Status:     StatusUnknown,
		RulesTotal: len(spec.Rules),
	}
	return nil
}

// UpdateContract updates an existing contract.
func (m *Manager) UpdateContract(spec *Spec) error {
	if len(spec.Rules) == 0 {
		return ErrNoRules
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.contracts[spec.Name]; !exists {
		return ErrContractNotFound
	}

	spec.UpdatedAt = time.Now()
	m.contracts[spec.Name] = spec
	m.statuses[spec.Name] = &ContractStatus{
		Name:       spec.Name,
		Status:     StatusUnknown,
		RulesTotal: len(spec.Rules),
	}
	return nil
}

// DeleteContract removes a contract.
func (m *Manager) DeleteContract(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.contracts[name]; !exists {
		return ErrContractNotFound
	}
	delete(m.contracts, name)
	delete(m.statuses, name)
	return nil
}

// GetContract retrieves a contract by name.
func (m *Manager) GetContract(name string) (*Spec, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	spec, exists := m.contracts[name]
	if !exists {
		return nil, ErrContractNotFound
	}
	return spec, nil
}

// ListContracts returns all registered contracts.
func (m *Manager) ListContracts() []*Spec {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Spec, 0, len(m.contracts))
	for _, spec := range m.contracts {
		result = append(result, spec)
	}
	return result
}

// GetStatus returns the current health of a contract.
func (m *Manager) GetStatus(name string) (*ContractStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, exists := m.statuses[name]
	if !exists {
		return nil, ErrContractNotFound
	}
	return status, nil
}

// ListStatuses returns all contract statuses.
func (m *Manager) ListStatuses() []*ContractStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ContractStatus, 0, len(m.statuses))
	for _, s := range m.statuses {
		result = append(result, s)
	}
	return result
}

// GetViolations returns violations since the given time.
func (m *Manager) GetViolations(since time.Time) []Violation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Violation, 0)
	for _, v := range m.violations {
		if v.Timestamp.After(since) {
			result = append(result, v)
		}
	}
	return result
}

// Start begins periodic contract evaluation.
func (m *Manager) Start(ctx context.Context) {
	go m.evaluationLoop(ctx)
}

// Stop halts the evaluation loop.
func (m *Manager) Stop() {
	close(m.stopCh)
}

func (m *Manager) evaluationLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.EvalInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.evaluateAll(ctx)
		}
	}
}

// EvaluateAll evaluates all contracts immediately.
func (m *Manager) EvaluateAll(ctx context.Context) {
	m.evaluateAll(ctx)
}

func (m *Manager) evaluateAll(ctx context.Context) {
	m.mu.RLock()
	specs := make([]*Spec, 0, len(m.contracts))
	for _, s := range m.contracts {
		specs = append(specs, s)
	}
	m.mu.RUnlock()

	for _, spec := range specs {
		m.evaluateContract(ctx, spec)
	}
}

func (m *Manager) evaluateContract(ctx context.Context, spec *Spec) {
	now := time.Now()
	var violations []Violation
	passing := 0

	for _, rule := range spec.Rules {
		v := m.evaluateRule(ctx, spec, rule, now)
		if v != nil {
			violations = append(violations, *v)
		} else {
			passing++
		}
	}

	status := StatusPassing
	if len(violations) > 0 {
		hasCritical := false
		for _, v := range violations {
			if v.Severity == SeverityCritical || v.Severity == SeverityError {
				hasCritical = true
				break
			}
		}
		if hasCritical {
			status = StatusBreached
		} else {
			status = StatusWarning
		}
	}

	m.mu.Lock()
	m.statuses[spec.Name] = &ContractStatus{
		Name:          spec.Name,
		Status:        status,
		LastEvaluated: now,
		Violations:    violations,
		RulesPassing:  passing,
		RulesTotal:    len(spec.Rules),
	}
	for _, v := range violations {
		m.violations = append(m.violations, v)
	}
	// Trim violations to max
	if len(m.violations) > m.config.MaxViolations {
		m.violations = m.violations[len(m.violations)-m.config.MaxViolations:]
	}
	handlers := make([]AlertHandler, len(m.handlers))
	copy(handlers, m.handlers)
	m.mu.Unlock()

	// Notify handlers outside the lock
	for _, v := range violations {
		for _, h := range handlers {
			_ = h.HandleViolation(ctx, v)
		}
	}
}

func (m *Manager) evaluateRule(ctx context.Context, spec *Spec, rule Rule, now time.Time) *Violation {
	if m.provider == nil {
		return nil
	}

	metrics, err := m.provider.GetFeatureMetrics(ctx, spec.FeatureGroup, spec.FeatureName)
	if err != nil {
		return nil
	}

	severity := rule.Severity
	if severity == "" {
		severity = SeverityError
	}

	switch rule.Type {
	case RuleFreshness:
		if rule.MaxStaleness > 0 {
			staleness := now.Sub(metrics.LastUpdated)
			if staleness > rule.MaxStaleness {
				return &Violation{
					ContractName:  spec.Name,
					RuleType:      RuleFreshness,
					Severity:      severity,
					Message:       fmt.Sprintf("feature staleness %s exceeds max %s", staleness.Round(time.Second), rule.MaxStaleness),
					ActualValue:   staleness.Seconds(),
					ExpectedValue: rule.MaxStaleness.Seconds(),
					Timestamp:     now,
				}
			}
		}
	case RuleCompleteness:
		if metrics.Completeness < rule.MinCompleteness {
			return &Violation{
				ContractName:  spec.Name,
				RuleType:      RuleCompleteness,
				Severity:      severity,
				Message:       fmt.Sprintf("completeness %.2f%% below minimum %.2f%%", metrics.Completeness*100, rule.MinCompleteness*100),
				ActualValue:   metrics.Completeness,
				ExpectedValue: rule.MinCompleteness,
				Timestamp:     now,
			}
		}
	case RuleDistribution:
		if rule.MinValue != nil && metrics.Min < *rule.MinValue {
			return &Violation{
				ContractName:  spec.Name,
				RuleType:      RuleDistribution,
				Severity:      severity,
				Message:       fmt.Sprintf("minimum value %.4f below bound %.4f", metrics.Min, *rule.MinValue),
				ActualValue:   metrics.Min,
				ExpectedValue: *rule.MinValue,
				Timestamp:     now,
			}
		}
		if rule.MaxValue != nil && metrics.Max > *rule.MaxValue {
			return &Violation{
				ContractName:  spec.Name,
				RuleType:      RuleDistribution,
				Severity:      severity,
				Message:       fmt.Sprintf("maximum value %.4f above bound %.4f", metrics.Max, *rule.MaxValue),
				ActualValue:   metrics.Max,
				ExpectedValue: *rule.MaxValue,
				Timestamp:     now,
			}
		}
		if rule.MeanMin != nil && metrics.Mean < *rule.MeanMin {
			return &Violation{
				ContractName:  spec.Name,
				RuleType:      RuleDistribution,
				Severity:      severity,
				Message:       fmt.Sprintf("mean %.4f below minimum %.4f", metrics.Mean, *rule.MeanMin),
				ActualValue:   metrics.Mean,
				ExpectedValue: *rule.MeanMin,
				Timestamp:     now,
			}
		}
		if rule.MeanMax != nil && metrics.Mean > *rule.MeanMax {
			return &Violation{
				ContractName:  spec.Name,
				RuleType:      RuleDistribution,
				Severity:      severity,
				Message:       fmt.Sprintf("mean %.4f above maximum %.4f", metrics.Mean, *rule.MeanMax),
				ActualValue:   metrics.Mean,
				ExpectedValue: *rule.MeanMax,
				Timestamp:     now,
			}
		}
	case RuleSchema:
		if rule.ExpectedType != "" && metrics.DataType != rule.ExpectedType {
			return &Violation{
				ContractName:  spec.Name,
				RuleType:      RuleSchema,
				Severity:      severity,
				Message:       fmt.Sprintf("expected type %s, got %s", rule.ExpectedType, metrics.DataType),
				ActualValue:   0,
				ExpectedValue: 0,
				Timestamp:     now,
			}
		}
	case RuleCustom:
		// Custom rules evaluated externally; no-op here
	}

	return nil
}

func validateRule(r Rule) error {
	switch r.Type {
	case RuleFreshness, RuleCompleteness, RuleDistribution, RuleSchema, RuleCustom:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidRuleType, r.Type)
	}
}
