// Package sla provides Service Level Agreement tracking and enforcement for features.
package sla

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// SLAType represents the type of SLA being defined.
type SLAType string

const (
	// SLATypeLatency enforces maximum latency for feature retrieval.
	SLATypeLatency SLAType = "latency"
	// SLATypeFreshness enforces maximum age of feature data.
	SLATypeFreshness SLAType = "freshness"
	// SLATypeAvailability enforces minimum availability percentage.
	SLATypeAvailability SLAType = "availability"
	// SLATypeThroughput enforces minimum throughput.
	SLATypeThroughput SLAType = "throughput"
)

// Priority represents the priority level of an SLA.
type Priority int

const (
	PriorityLow Priority = iota
	PriorityMedium
	PriorityHigh
	PriorityCritical
)

// String returns the string representation of a priority.
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityMedium:
		return "medium"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// SLASpec defines a Service Level Agreement specification.
type SLASpec struct {
	// Name is the unique identifier for this SLA.
	Name string `json:"name"`
	// Description describes the SLA purpose.
	Description string `json:"description,omitempty"`
	// Type is the type of SLA (latency, freshness, availability).
	Type SLAType `json:"type"`
	// Target is the target value (e.g., 100ms for latency, 99.9 for availability).
	Target float64 `json:"target"`
	// Priority indicates the importance of this SLA.
	Priority Priority `json:"priority"`
	// Features lists the features this SLA applies to (empty means all).
	Features []string `json:"features,omitempty"`
	// Groups lists the feature groups this SLA applies to (empty means all).
	Groups []string `json:"groups,omitempty"`
	// Window is the evaluation window for the SLA.
	Window time.Duration `json:"window"`
	// AlertThreshold is the threshold percentage before breaching (e.g., 0.9 = alert at 90% of target).
	AlertThreshold float64 `json:"alertThreshold,omitempty"`
	// Owner is the team/person responsible for this SLA.
	Owner string `json:"owner,omitempty"`
	// Enabled indicates if the SLA is actively enforced.
	Enabled bool `json:"enabled"`
}

// Validate validates the SLA specification.
func (s *SLASpec) Validate() error {
	if s.Name == "" {
		return errors.New("SLA name is required")
	}
	if s.Type == "" {
		return errors.New("SLA type is required")
	}
	if s.Target <= 0 {
		return errors.New("SLA target must be positive")
	}
	if s.Window <= 0 {
		return errors.New("SLA window must be positive")
	}
	switch s.Type {
	case SLATypeLatency, SLATypeFreshness, SLATypeAvailability, SLATypeThroughput:
		// Valid types
	default:
		return fmt.Errorf("unknown SLA type: %s", s.Type)
	}
	if s.Type == SLATypeAvailability && (s.Target < 0 || s.Target > 100) {
		return errors.New("availability SLA target must be between 0 and 100")
	}
	return nil
}

// SLAStatus represents the current status of an SLA.
type SLAStatus struct {
	// Spec is the SLA specification.
	Spec *SLASpec `json:"spec"`
	// CurrentValue is the current measured value.
	CurrentValue float64 `json:"currentValue"`
	// IsBreached indicates if the SLA is currently breached.
	IsBreached bool `json:"isBreached"`
	// IsWarning indicates if the SLA is approaching breach threshold.
	IsWarning bool `json:"isWarning"`
	// BreachCount is the number of breaches in the current window.
	BreachCount int `json:"breachCount"`
	// LastBreachTime is when the last breach occurred.
	LastBreachTime *time.Time `json:"lastBreachTime,omitempty"`
	// CompliancePercentage is the percentage of time the SLA was met.
	CompliancePercentage float64 `json:"compliancePercentage"`
	// LastUpdated is when the status was last updated.
	LastUpdated time.Time `json:"lastUpdated"`
}

// Breach represents an SLA breach event.
type Breach struct {
	// SLAName is the name of the breached SLA.
	SLAName string `json:"slaName"`
	// Type is the SLA type.
	Type SLAType `json:"type"`
	// Feature is the specific feature that breached (if applicable).
	Feature string `json:"feature,omitempty"`
	// TargetValue is the SLA target.
	TargetValue float64 `json:"targetValue"`
	// ActualValue is the measured value that breached.
	ActualValue float64 `json:"actualValue"`
	// Timestamp is when the breach occurred.
	Timestamp time.Time `json:"timestamp"`
	// Duration is how long the breach lasted.
	Duration time.Duration `json:"duration,omitempty"`
	// Priority is the SLA priority.
	Priority Priority `json:"priority"`
	// Severity is the breach severity (how far from target).
	Severity float64 `json:"severity"`
}

// MetricsProvider provides metrics for SLA evaluation.
type MetricsProvider interface {
	// GetLatencyP99 returns the P99 latency for a feature/group.
	GetLatencyP99(ctx context.Context, feature string, window time.Duration) (time.Duration, error)
	// GetFreshness returns the age of the most recent data for a feature.
	GetFreshness(ctx context.Context, feature string) (time.Duration, error)
	// GetAvailability returns the availability percentage for a feature.
	GetAvailability(ctx context.Context, feature string, window time.Duration) (float64, error)
	// GetThroughput returns the requests per second for a feature.
	GetThroughput(ctx context.Context, feature string, window time.Duration) (float64, error)
}

// AlertHandler handles SLA breach alerts.
type AlertHandler interface {
	// OnWarning is called when an SLA is approaching breach threshold.
	OnWarning(ctx context.Context, status *SLAStatus) error
	// OnBreach is called when an SLA is breached.
	OnBreach(ctx context.Context, breach *Breach) error
	// OnRecovery is called when an SLA recovers from breach.
	OnRecovery(ctx context.Context, slaName string) error
}

// Manager manages SLA specifications and enforcement.
type Manager struct {
	mu             sync.RWMutex
	specs          map[string]*SLASpec
	statuses       map[string]*SLAStatus
	breaches       []*Breach
	maxBreaches    int
	metrics        MetricsProvider
	alertHandlers  []AlertHandler
	checkInterval  time.Duration
	stopCh         chan struct{}
	lastEvaluation time.Time
}

// ManagerConfig configures the SLA manager.
type ManagerConfig struct {
	// CheckInterval is how often to check SLAs.
	CheckInterval time.Duration
	// MaxBreachHistory is the maximum number of breaches to retain.
	MaxBreachHistory int
}

// DefaultManagerConfig returns default configuration.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		CheckInterval:    30 * time.Second,
		MaxBreachHistory: 1000,
	}
}

// NewManager creates a new SLA manager.
func NewManager(metrics MetricsProvider, config ManagerConfig) *Manager {
	if config.CheckInterval <= 0 {
		config.CheckInterval = 30 * time.Second
	}
	if config.MaxBreachHistory <= 0 {
		config.MaxBreachHistory = 1000
	}

	return &Manager{
		specs:         make(map[string]*SLASpec),
		statuses:      make(map[string]*SLAStatus),
		breaches:      make([]*Breach, 0),
		maxBreaches:   config.MaxBreachHistory,
		metrics:       metrics,
		alertHandlers: make([]AlertHandler, 0),
		checkInterval: config.CheckInterval,
		stopCh:        make(chan struct{}),
	}
}

// RegisterSLA registers a new SLA specification.
func (m *Manager) RegisterSLA(spec *SLASpec) error {
	if err := spec.Validate(); err != nil {
		return fmt.Errorf("invalid SLA specification: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.specs[spec.Name] = spec
	m.statuses[spec.Name] = &SLAStatus{
		Spec:                 spec,
		CompliancePercentage: 100.0,
		LastUpdated:          time.Now(),
	}

	return nil
}

// UnregisterSLA removes an SLA specification.
func (m *Manager) UnregisterSLA(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.specs[name]; !exists {
		return fmt.Errorf("SLA not found: %s", name)
	}

	delete(m.specs, name)
	delete(m.statuses, name)
	return nil
}

// GetSLA returns an SLA specification by name.
func (m *Manager) GetSLA(name string) (*SLASpec, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	spec, exists := m.specs[name]
	if !exists {
		return nil, fmt.Errorf("SLA not found: %s", name)
	}
	return spec, nil
}

// ListSLAs returns all registered SLA specifications.
func (m *Manager) ListSLAs() []*SLASpec {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SLASpec, 0, len(m.specs))
	for _, spec := range m.specs {
		result = append(result, spec)
	}
	return result
}

// GetStatus returns the current status of an SLA.
func (m *Manager) GetStatus(name string) (*SLAStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, exists := m.statuses[name]
	if !exists {
		return nil, fmt.Errorf("SLA status not found: %s", name)
	}
	return status, nil
}

// GetAllStatuses returns all SLA statuses.
func (m *Manager) GetAllStatuses() []*SLAStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SLAStatus, 0, len(m.statuses))
	for _, status := range m.statuses {
		result = append(result, status)
	}
	return result
}

// GetBreaches returns breaches within the specified time window.
func (m *Manager) GetBreaches(since time.Time) []*Breach {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Breach, 0)
	for _, breach := range m.breaches {
		if breach.Timestamp.After(since) {
			result = append(result, breach)
		}
	}
	return result
}

// AddAlertHandler adds an alert handler.
func (m *Manager) AddAlertHandler(handler AlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertHandlers = append(m.alertHandlers, handler)
}

// Start begins periodic SLA evaluation.
func (m *Manager) Start(ctx context.Context) {
	go m.evaluationLoop(ctx)
}

// Stop stops periodic SLA evaluation.
func (m *Manager) Stop() {
	close(m.stopCh)
}

func (m *Manager) evaluationLoop(ctx context.Context) {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.evaluate(ctx)
		}
	}
}

func (m *Manager) evaluate(ctx context.Context) {
	m.mu.Lock()
	specs := make([]*SLASpec, 0, len(m.specs))
	for _, spec := range m.specs {
		if spec.Enabled {
			specs = append(specs, spec)
		}
	}
	m.lastEvaluation = time.Now()
	m.mu.Unlock()

	for _, spec := range specs {
		m.evaluateSLA(ctx, spec)
	}
}

func (m *Manager) evaluateSLA(ctx context.Context, spec *SLASpec) {
	if m.metrics == nil {
		return
	}

	var currentValue float64
	var err error

	// Get appropriate metric based on SLA type
	switch spec.Type {
	case SLATypeLatency:
		var latency time.Duration
		if len(spec.Features) > 0 {
			latency, err = m.metrics.GetLatencyP99(ctx, spec.Features[0], spec.Window)
		} else {
			latency, err = m.metrics.GetLatencyP99(ctx, "", spec.Window)
		}
		if err == nil {
			currentValue = float64(latency.Milliseconds())
		}

	case SLATypeFreshness:
		var age time.Duration
		if len(spec.Features) > 0 {
			age, err = m.metrics.GetFreshness(ctx, spec.Features[0])
		} else {
			age, err = m.metrics.GetFreshness(ctx, "")
		}
		if err == nil {
			currentValue = age.Seconds()
		}

	case SLATypeAvailability:
		feature := ""
		if len(spec.Features) > 0 {
			feature = spec.Features[0]
		}
		currentValue, err = m.metrics.GetAvailability(ctx, feature, spec.Window)

	case SLATypeThroughput:
		feature := ""
		if len(spec.Features) > 0 {
			feature = spec.Features[0]
		}
		currentValue, err = m.metrics.GetThroughput(ctx, feature, spec.Window)
	}

	if err != nil {
		return
	}

	// Determine if breached
	var isBreached, isWarning bool
	switch spec.Type {
	case SLATypeLatency, SLATypeFreshness:
		// Higher is worse for latency/freshness
		isBreached = currentValue > spec.Target
		if spec.AlertThreshold > 0 {
			isWarning = currentValue > spec.Target*spec.AlertThreshold
		}
	case SLATypeAvailability, SLATypeThroughput:
		// Lower is worse for availability/throughput
		isBreached = currentValue < spec.Target
		if spec.AlertThreshold > 0 {
			isWarning = currentValue < spec.Target*(1+(1-spec.AlertThreshold))
		}
	}

	// Update status
	m.mu.Lock()
	status := m.statuses[spec.Name]
	if status == nil {
		status = &SLAStatus{Spec: spec}
		m.statuses[spec.Name] = status
	}

	wasBreached := status.IsBreached
	status.CurrentValue = currentValue
	status.IsBreached = isBreached
	status.IsWarning = isWarning
	status.LastUpdated = time.Now()

	if isBreached {
		status.BreachCount++
		now := time.Now()
		status.LastBreachTime = &now

		breach := &Breach{
			SLAName:     spec.Name,
			Type:        spec.Type,
			TargetValue: spec.Target,
			ActualValue: currentValue,
			Timestamp:   now,
			Priority:    spec.Priority,
		}

		// Calculate severity
		switch spec.Type {
		case SLATypeLatency, SLATypeFreshness:
			breach.Severity = (currentValue - spec.Target) / spec.Target
		case SLATypeAvailability, SLATypeThroughput:
			breach.Severity = (spec.Target - currentValue) / spec.Target
		}

		m.breaches = append(m.breaches, breach)
		if len(m.breaches) > m.maxBreaches {
			m.breaches = m.breaches[len(m.breaches)-m.maxBreaches:]
		}
	}

	// Copy alert handlers
	handlers := make([]AlertHandler, len(m.alertHandlers))
	copy(handlers, m.alertHandlers)
	m.mu.Unlock()

	// Send alerts outside lock
	for _, handler := range handlers {
		if isBreached && !wasBreached {
			breach := &Breach{
				SLAName:     spec.Name,
				Type:        spec.Type,
				TargetValue: spec.Target,
				ActualValue: currentValue,
				Timestamp:   time.Now(),
				Priority:    spec.Priority,
			}
			_ = handler.OnBreach(ctx, breach)
		} else if !isBreached && wasBreached {
			_ = handler.OnRecovery(ctx, spec.Name)
		} else if isWarning && !isBreached {
			_ = handler.OnWarning(ctx, status)
		}
	}
}

// EvaluateNow triggers immediate SLA evaluation.
func (m *Manager) EvaluateNow(ctx context.Context) {
	m.evaluate(ctx)
}

// GetComplianceSummary returns overall compliance statistics.
func (m *Manager) GetComplianceSummary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := len(m.statuses)
	breached := 0
	warning := 0
	healthy := 0

	for _, status := range m.statuses {
		if status.IsBreached {
			breached++
		} else if status.IsWarning {
			warning++
		} else {
			healthy++
		}
	}

	recentBreaches := 0
	oneDayAgo := time.Now().Add(-24 * time.Hour)
	for _, breach := range m.breaches {
		if breach.Timestamp.After(oneDayAgo) {
			recentBreaches++
		}
	}

	return map[string]interface{}{
		"totalSLAs":         total,
		"breached":          breached,
		"warning":           warning,
		"healthy":           healthy,
		"compliancePercent": float64(healthy) / float64(total) * 100,
		"breaches24h":       recentBreaches,
		"lastEvaluation":    m.lastEvaluation,
	}
}
