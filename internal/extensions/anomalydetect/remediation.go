package anomalydetect

import (
	"fmt"
	"sync"
	"time"
)

// RemediationAction identifies a remediation type.
type RemediationAction string

const (
	ActionCacheFlush    RemediationAction = "cache_flush"
	ActionFallback      RemediationAction = "fallback_value"
	ActionCircuitBreak  RemediationAction = "circuit_breaker"
	ActionAlertEscalate RemediationAction = "alert_escalation"
	ActionQuarantine    RemediationAction = "quarantine"
	ActionNoOp          RemediationAction = "no_op"
)

// RemediationPolicy defines when and how to remediate.
type RemediationPolicy struct {
	Name           string              `json:"name"`
	Feature        string              `json:"feature"`
	Condition      PolicyCondition     `json:"condition"`
	Actions        []RemediationAction `json:"actions"`
	FallbackValue  interface{}         `json:"fallback_value,omitempty"`
	CooldownPeriod time.Duration       `json:"cooldown_period"`
	Enabled        bool                `json:"enabled"`
}

// PolicyCondition defines what triggers remediation.
type PolicyCondition struct {
	AnomalyType      AnomalyType `json:"anomaly_type"`
	MinScore         float64     `json:"min_score"`
	ConsecutiveCount int         `json:"consecutive_count"`
}

// RemediationEvent records an executed remediation.
type RemediationEvent struct {
	ID         string              `json:"id"`
	PolicyName string              `json:"policy_name"`
	Feature    string              `json:"feature"`
	Actions    []RemediationAction `json:"actions"`
	Anomaly    AnomalyResult       `json:"anomaly"`
	Timestamp  time.Time           `json:"timestamp"`
	Success    bool                `json:"success"`
	Error      string              `json:"error,omitempty"`
}

// RemediationStats tracks remediation activity.
type RemediationStats struct {
	TotalPolicies    int                            `json:"total_policies"`
	TotalEvents      int64                          `json:"total_events"`
	SuccessfulEvents int64                          `json:"successful_events"`
	FailedEvents     int64                          `json:"failed_events"`
	ActionCounts     map[RemediationAction]int64    `json:"action_counts"`
}

// RemediationEngine manages auto-remediation policies and execution.
type RemediationEngine struct {
	mu               sync.RWMutex
	policies         map[string]*RemediationPolicy
	events           []RemediationEvent
	consecutiveCount map[string]int
	lastRemediation  map[string]time.Time
	nextID           int
	stats            RemediationStats
}

// NewRemediationEngine creates a new remediation engine.
func NewRemediationEngine() *RemediationEngine {
	return &RemediationEngine{
		policies:         make(map[string]*RemediationPolicy),
		consecutiveCount: make(map[string]int),
		lastRemediation:  make(map[string]time.Time),
		stats: RemediationStats{
			ActionCounts: make(map[RemediationAction]int64),
		},
	}
}

// AddPolicy registers a remediation policy.
func (e *RemediationEngine) AddPolicy(policy RemediationPolicy) error {
	if policy.Name == "" {
		return fmt.Errorf("policy name is required")
	}
	if policy.Feature == "" {
		return fmt.Errorf("feature is required")
	}
	if len(policy.Actions) == 0 {
		return fmt.Errorf("at least one action is required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.policies[policy.Name] = &policy
	e.stats.TotalPolicies = len(e.policies)
	return nil
}

// RemovePolicy removes a remediation policy.
func (e *RemediationEngine) RemovePolicy(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.policies[name]; !exists {
		return fmt.Errorf("policy %s not found", name)
	}
	delete(e.policies, name)
	e.stats.TotalPolicies = len(e.policies)
	return nil
}

// ListPolicies returns all policies.
func (e *RemediationEngine) ListPolicies() []RemediationPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]RemediationPolicy, 0, len(e.policies))
	for _, p := range e.policies {
		result = append(result, *p)
	}
	return result
}

// Evaluate processes an anomaly result and executes matching policies.
func (e *RemediationEngine) Evaluate(anomaly AnomalyResult) []RemediationEvent {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Update consecutive count
	if anomaly.IsAnomaly {
		e.consecutiveCount[anomaly.Feature]++
	} else {
		e.consecutiveCount[anomaly.Feature] = 0
		return nil
	}

	var events []RemediationEvent

	for _, policy := range e.policies {
		if !policy.Enabled {
			continue
		}
		if policy.Feature != anomaly.Feature && policy.Feature != "*" {
			continue
		}

		// Check condition
		if policy.Condition.AnomalyType != "" && policy.Condition.AnomalyType != anomaly.Type {
			continue
		}
		if anomaly.Score < policy.Condition.MinScore {
			continue
		}
		if policy.Condition.ConsecutiveCount > 0 && e.consecutiveCount[anomaly.Feature] < policy.Condition.ConsecutiveCount {
			continue
		}

		// Check cooldown
		if last, ok := e.lastRemediation[policy.Name]; ok {
			if policy.CooldownPeriod > 0 && time.Since(last) < policy.CooldownPeriod {
				continue
			}
		}

		// Execute remediation
		e.nextID++
		event := RemediationEvent{
			ID:         fmt.Sprintf("rem-%d", e.nextID),
			PolicyName: policy.Name,
			Feature:    anomaly.Feature,
			Actions:    policy.Actions,
			Anomaly:    anomaly,
			Timestamp:  time.Now(),
			Success:    true,
		}

		// Track action counts
		for _, action := range policy.Actions {
			e.stats.ActionCounts[action]++
		}

		e.lastRemediation[policy.Name] = time.Now()
		e.events = append(e.events, event)
		e.stats.TotalEvents++
		e.stats.SuccessfulEvents++
		events = append(events, event)
	}

	// Trim event history
	if len(e.events) > 10000 {
		e.events = e.events[len(e.events)-5000:]
	}

	return events
}

// GetEvents returns recent remediation events.
func (e *RemediationEngine) GetEvents(feature string, limit int) []RemediationEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []RemediationEvent
	for i := len(e.events) - 1; i >= 0; i-- {
		if feature == "" || e.events[i].Feature == feature {
			result = append(result, e.events[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

// Stats returns remediation statistics.
func (e *RemediationEngine) Stats() RemediationStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	stats := e.stats
	stats.ActionCounts = make(map[RemediationAction]int64)
	for k, v := range e.stats.ActionCounts {
		stats.ActionCounts[k] = v
	}
	return stats
}
