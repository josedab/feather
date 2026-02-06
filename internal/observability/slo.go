package observability

import (
	"context"
	"math"
	"sync"
	"time"
)

// SLOType defines the type of SLO.
type SLOType string

// SLOType constants for SLO definitions.
const (
	SLOTypeLatency      SLOType = "latency"
	SLOTypeAvailability SLOType = "availability"
	SLOTypeFreshness    SLOType = "freshness"
	SLOTypeThroughput   SLOType = "throughput"
	SLOTypeErrorRate    SLOType = "error_rate"
)

// SLODefinition defines a Service Level Objective.
type SLODefinition struct {
	Name        string        `json:"name"`
	Type        SLOType       `json:"type"`
	Target      float64       `json:"target"`      // Target value (e.g., 99.9 for 99.9% availability)
	Window      time.Duration `json:"window"`      // Measurement window (e.g., 30 days)
	Threshold   float64       `json:"threshold"`   // Threshold for latency/freshness in ms
	Component   string        `json:"component"`   // Component this SLO applies to
	Description string        `json:"description"` // Human-readable description
}

// SLOStatus represents the current status of an SLO.
type SLOStatus struct {
	Definition      *SLODefinition `json:"definition"`
	CurrentValue    float64        `json:"current_value"`
	ErrorBudget     float64        `json:"error_budget"`      // Remaining error budget (0-100%)
	ErrorBudgetUsed float64        `json:"error_budget_used"` // Used error budget (0-100%)
	InCompliance    bool           `json:"in_compliance"`
	BurnRate        float64        `json:"burn_rate"` // Current burn rate (1.0 = normal)
	TimeToExhaust   time.Duration  `json:"time_to_exhaust,omitempty"`
	LastUpdated     time.Time      `json:"last_updated"`
}

// SLOBreach represents an SLO violation event.
type SLOBreach struct {
	SLOName     string    `json:"slo_name"`
	Type        SLOType   `json:"type"`
	ExpectedMin float64   `json:"expected_min"`
	ActualValue float64   `json:"actual_value"`
	Severity    string    `json:"severity"` // warning, critical
	DetectedAt  time.Time `json:"detected_at"`
	Component   string    `json:"component"`
	ErrorBudget float64   `json:"error_budget_remaining"`
	BurnRate    float64   `json:"burn_rate"`
}

// SLOTracker tracks SLOs and calculates compliance.
type SLOTracker struct {
	definitions map[string]*SLODefinition
	windows     map[string]*sloWindow
	breaches    []SLOBreach
	mu          sync.RWMutex
}

type sloWindow struct {
	successCount int64
	failureCount int64
	latencies    []float64
	timestamps   []time.Time
	windowStart  time.Time
}

// NewSLOTracker creates a new SLO tracker.
func NewSLOTracker() *SLOTracker {
	return &SLOTracker{
		definitions: make(map[string]*SLODefinition),
		windows:     make(map[string]*sloWindow),
		breaches:    make([]SLOBreach, 0),
	}
}

// RegisterSLO registers a new SLO definition.
func (t *SLOTracker) RegisterSLO(def *SLODefinition) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.definitions[def.Name] = def
	t.windows[def.Name] = &sloWindow{
		latencies:   make([]float64, 0, 10000),
		timestamps:  make([]time.Time, 0, 10000),
		windowStart: time.Now(),
	}
}

// RecordSuccess records a successful operation for an SLO.
func (t *SLOTracker) RecordSuccess(sloName string, latencyMs float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	w, ok := t.windows[sloName]
	if !ok {
		return
	}

	now := time.Now()
	w.successCount++
	w.latencies = append(w.latencies, latencyMs)
	w.timestamps = append(w.timestamps, now)

	// Trim old data outside the window
	t.trimWindow(sloName)
}

// RecordFailure records a failed operation for an SLO.
func (t *SLOTracker) RecordFailure(sloName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	w, ok := t.windows[sloName]
	if !ok {
		return
	}

	w.failureCount++

	// Trim old data outside the window
	t.trimWindow(sloName)
}

// GetStatus returns the current status of an SLO.
func (t *SLOTracker) GetStatus(sloName string) *SLOStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.getStatusLocked(sloName)
}

// getStatusLocked returns the current status of an SLO.
// Caller must hold at least a read lock on t.mu.
func (t *SLOTracker) getStatusLocked(sloName string) *SLOStatus {
	def, ok := t.definitions[sloName]
	if !ok {
		return nil
	}

	w, ok := t.windows[sloName]
	if !ok {
		return nil
	}

	status := &SLOStatus{
		Definition:  def,
		LastUpdated: time.Now(),
	}

	totalOps := w.successCount + w.failureCount

	switch def.Type {
	case SLOTypeAvailability:
		if totalOps > 0 {
			status.CurrentValue = (float64(w.successCount) / float64(totalOps)) * 100
		} else {
			status.CurrentValue = 100 // No operations = 100% availability
		}
		status.InCompliance = status.CurrentValue >= def.Target
		status.ErrorBudget = calculateErrorBudget(status.CurrentValue, def.Target)
		status.ErrorBudgetUsed = 100 - status.ErrorBudget

	case SLOTypeLatency:
		// Calculate P99 latency
		if len(w.latencies) > 0 {
			p99 := calculatePercentile(w.latencies, 0.99)
			// Percentage of requests meeting threshold
			withinThreshold := 0
			for _, lat := range w.latencies {
				if lat <= def.Threshold {
					withinThreshold++
				}
			}
			status.CurrentValue = (float64(withinThreshold) / float64(len(w.latencies))) * 100

			// Store P99 for reference
			_ = p99
		} else {
			status.CurrentValue = 100
		}
		status.InCompliance = status.CurrentValue >= def.Target
		status.ErrorBudget = calculateErrorBudget(status.CurrentValue, def.Target)
		status.ErrorBudgetUsed = 100 - status.ErrorBudget

	case SLOTypeErrorRate:
		if totalOps > 0 {
			errorRate := (float64(w.failureCount) / float64(totalOps)) * 100
			status.CurrentValue = 100 - errorRate // Invert to show success rate
		} else {
			status.CurrentValue = 100
		}
		status.InCompliance = status.CurrentValue >= def.Target
		status.ErrorBudget = calculateErrorBudget(status.CurrentValue, def.Target)
		status.ErrorBudgetUsed = 100 - status.ErrorBudget
	case SLOTypeFreshness, SLOTypeThroughput:
	}

	// Calculate burn rate
	if status.ErrorBudgetUsed > 0 {
		elapsed := time.Since(w.windowStart)
		expectedUsage := (elapsed.Seconds() / def.Window.Seconds()) * 100
		if expectedUsage > 0 {
			status.BurnRate = status.ErrorBudgetUsed / expectedUsage
		}

		// Calculate time to exhaust error budget at current burn rate
		if status.BurnRate > 0 && status.ErrorBudget > 0 {
			remainingBudget := status.ErrorBudget
			exhaustTime := time.Duration(remainingBudget/status.BurnRate) * def.Window / 100
			status.TimeToExhaust = exhaustTime
		}
	}

	return status
}

// GetAllStatus returns status for all registered SLOs.
func (t *SLOTracker) GetAllStatus() []*SLOStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	statuses := make([]*SLOStatus, 0, len(t.definitions))
	for name := range t.definitions {
		status := t.getStatusLocked(name)
		if status != nil {
			statuses = append(statuses, status)
		}
	}
	return statuses
}

// CheckBreaches checks all SLOs for breaches and returns any new breaches.
func (t *SLOTracker) CheckBreaches(ctx context.Context) []SLOBreach {
	t.mu.Lock()
	defer t.mu.Unlock()

	var newBreaches []SLOBreach

	for name, def := range t.definitions {
		status := t.getStatusLocked(name)
		if status == nil {
			continue
		}

		if !status.InCompliance {
			severity := "warning"
			if status.ErrorBudgetUsed > 80 {
				severity = "critical"
			}

			breach := SLOBreach{
				SLOName:     name,
				Type:        def.Type,
				ExpectedMin: def.Target,
				ActualValue: status.CurrentValue,
				Severity:    severity,
				DetectedAt:  time.Now(),
				Component:   def.Component,
				ErrorBudget: status.ErrorBudget,
				BurnRate:    status.BurnRate,
			}

			newBreaches = append(newBreaches, breach)
			t.breaches = append(t.breaches, breach)
		}
	}

	// Keep only last 1000 breaches
	if len(t.breaches) > 1000 {
		t.breaches = t.breaches[len(t.breaches)-1000:]
	}

	return newBreaches
}

// GetBreaches returns recent breaches, optionally filtered by time.
func (t *SLOTracker) GetBreaches(since time.Time) []SLOBreach {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var filtered []SLOBreach
	for _, b := range t.breaches {
		if b.DetectedAt.After(since) {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

// trimWindow removes data outside the SLO window.
func (t *SLOTracker) trimWindow(sloName string) {
	def := t.definitions[sloName]
	w := t.windows[sloName]
	if def == nil || w == nil {
		return
	}

	cutoff := time.Now().Add(-def.Window)

	// Trim latencies and timestamps
	newStart := 0
	for i, ts := range w.timestamps {
		if ts.After(cutoff) {
			newStart = i
			break
		}
	}

	if newStart > 0 {
		trimmed := len(w.latencies) - len(w.latencies[newStart:])
		w.latencies = w.latencies[newStart:]
		w.timestamps = w.timestamps[newStart:]

		// Adjust counts proportionally (simplified approach)
		if trimmed > 0 {
			ratio := float64(len(w.latencies)) / float64(len(w.latencies)+trimmed)
			w.successCount = int64(float64(w.successCount) * ratio)
			w.failureCount = int64(float64(w.failureCount) * ratio)
		}
	}
}

// calculateErrorBudget calculates remaining error budget.
func calculateErrorBudget(current, target float64) float64 {
	if target >= 100 {
		return 0
	}

	// Error budget = (current - (100 - target)) / (100 - target) * 100
	allowedErrors := 100 - target
	actualErrors := 100 - current

	if allowedErrors <= 0 {
		return 0
	}

	remaining := ((allowedErrors - actualErrors) / allowedErrors) * 100
	if remaining < 0 {
		return 0
	}
	if remaining > 100 {
		return 100
	}
	return remaining
}

// calculatePercentile calculates a percentile from a slice of values.
func calculatePercentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)

	// Simple sort
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// DefaultSLOs returns a set of default SLO definitions for a feature store.
func DefaultSLOs() []*SLODefinition {
	return []*SLODefinition{
		{
			Name:        "api_availability",
			Type:        SLOTypeAvailability,
			Target:      99.9,
			Window:      30 * 24 * time.Hour, // 30 days
			Component:   "api",
			Description: "API availability target of 99.9%",
		},
		{
			Name:        "feature_read_latency_p99",
			Type:        SLOTypeLatency,
			Target:      99.0,
			Threshold:   10, // 10ms
			Window:      24 * time.Hour,
			Component:   "storage",
			Description: "99% of feature reads complete in under 10ms",
		},
		{
			Name:        "feature_write_latency_p99",
			Type:        SLOTypeLatency,
			Target:      99.0,
			Threshold:   50, // 50ms
			Window:      24 * time.Hour,
			Component:   "storage",
			Description: "99% of feature writes complete in under 50ms",
		},
		{
			Name:        "ingestion_error_rate",
			Type:        SLOTypeErrorRate,
			Target:      99.5,
			Window:      24 * time.Hour,
			Component:   "ingestion",
			Description: "Ingestion error rate below 0.5%",
		},
	}
}
