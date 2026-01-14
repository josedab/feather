package monitoring

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ActionType defines the type of remediation action.
type ActionType string

const (
	ActionNotify    ActionType = "notify"
	ActionFallback  ActionType = "fallback"
	ActionRecompute ActionType = "recompute"
	ActionDisable   ActionType = "disable"
)

// RemediationAction defines an automatic response to an alert condition.
type RemediationAction struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Trigger        Severity          `json:"trigger"`
	MonitorType    MonitorType       `json:"monitor_type"`
	ActionType     ActionType        `json:"action_type"`
	Config         map[string]string `json:"config"`
	Enabled        bool              `json:"enabled"`
	ExecutionCount int64             `json:"execution_count"`
	LastExecuted   time.Time         `json:"last_executed"`
}

// RemediationEngine manages and executes remediation playbooks.
type RemediationEngine struct {
	mu      sync.RWMutex
	actions map[string]*RemediationAction
}

// NewRemediationEngine creates a new remediation engine.
func NewRemediationEngine() *RemediationEngine {
	return &RemediationEngine{
		actions: make(map[string]*RemediationAction),
	}
}

// RegisterAction adds a remediation action.
func (re *RemediationEngine) RegisterAction(action *RemediationAction) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	if _, exists := re.actions[action.ID]; exists {
		return fmt.Errorf("%w: %s", ErrAlreadyExists, action.ID)
	}
	re.actions[action.ID] = action
	return nil
}

// RemoveAction removes a remediation action by ID.
func (re *RemediationEngine) RemoveAction(id string) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	if _, exists := re.actions[id]; !exists {
		return fmt.Errorf("remediation action not found: %s", id)
	}
	delete(re.actions, id)
	return nil
}

// ListActions returns all registered remediation actions.
func (re *RemediationEngine) ListActions() []*RemediationAction {
	re.mu.RLock()
	defer re.mu.RUnlock()

	result := make([]*RemediationAction, 0, len(re.actions))
	for _, a := range re.actions {
		result = append(result, a)
	}
	return result
}

// Evaluate returns all enabled actions matching the alert's severity.
func (re *RemediationEngine) Evaluate(alert Alert) []*RemediationAction {
	re.mu.RLock()
	defer re.mu.RUnlock()

	var matched []*RemediationAction
	for _, action := range re.actions {
		if !action.Enabled {
			continue
		}
		if action.Trigger == alert.Severity {
			matched = append(matched, action)
		}
	}
	return matched
}

// Execute runs a remediation action, logging execution and incrementing the count.
func (re *RemediationEngine) Execute(action *RemediationAction, alert Alert) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	slog.Info("executing remediation action",
		"action_id", action.ID,
		"action_type", action.ActionType,
		"alert_id", alert.ID,
		"feature", alert.FeatureName,
	)

	action.ExecutionCount++
	action.LastExecuted = time.Now()
	return nil
}
