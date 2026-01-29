package monitoring

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrMonitorNotFound = errors.New("monitor not found")
	ErrAlreadyExists   = errors.New("monitor already exists")
	ErrRuleNotFound    = errors.New("alerting rule not found")
)

// MonitorType indicates what aspect of a feature is being monitored.
type MonitorType string

const (
	MonitorTypeDrift     MonitorType = "drift"
	MonitorTypeFreshness MonitorType = "freshness"
	MonitorTypeVolume    MonitorType = "volume"
	MonitorTypeSchema    MonitorType = "schema"
	MonitorTypeValue     MonitorType = "value_range"
)

// Severity indicates the severity of an alert.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// MonitorStatus indicates whether a monitor is healthy.
type MonitorStatus string

const (
	StatusHealthy  MonitorStatus = "healthy"
	StatusWarning  MonitorStatus = "warning"
	StatusCritical MonitorStatus = "critical"
	StatusUnknown  MonitorStatus = "unknown"
)

// ManagerConfig configures the monitoring manager.
type ManagerConfig struct {
	MaxMonitors     int           `json:"max_monitors"`
	MaxAlerts       int           `json:"max_alerts"`
	AlertCooldown   time.Duration `json:"alert_cooldown"`
	DefaultInterval time.Duration `json:"default_interval"`
}

// DefaultManagerConfig returns sensible defaults for the monitoring manager.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		MaxMonitors:     1000,
		MaxAlerts:       10000,
		AlertCooldown:   5 * time.Minute,
		DefaultInterval: 1 * time.Minute,
	}
}

// Manager coordinates all feature monitors and alerting.
type Manager struct {
	mu        sync.RWMutex
	config    ManagerConfig
	monitors  map[string]*FeatureMonitor
	rules     map[string]*AlertRule
	alerts    []Alert
	notifiers []Notifier
}

// FeatureMonitor tracks the health of a specific feature.
type FeatureMonitor struct {
	ID           string        `json:"id"`
	FeatureName  string        `json:"feature_name"`
	Type         MonitorType   `json:"type"`
	Status       MonitorStatus `json:"status"`
	Threshold    float64       `json:"threshold"`
	CurrentValue float64       `json:"current_value"`
	LastCheck    time.Time     `json:"last_check"`
	CreatedAt    time.Time     `json:"created_at"`
	Interval     time.Duration `json:"interval"`
	Enabled      bool          `json:"enabled"`
	CheckCount   int64         `json:"check_count"`
	AlertCount   int64         `json:"alert_count"`
}

// AlertRule defines when to fire an alert.
type AlertRule struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	MonitorID string        `json:"monitor_id"`
	Severity  Severity      `json:"severity"`
	Condition string        `json:"condition"` // "above", "below", "equals"
	Threshold float64       `json:"threshold"`
	Cooldown  time.Duration `json:"cooldown"`
	LastFired time.Time     `json:"last_fired"`
	Enabled   bool          `json:"enabled"`
}

// Alert represents a fired alert.
type Alert struct {
	ID           string    `json:"id"`
	RuleID       string    `json:"rule_id"`
	MonitorID    string    `json:"monitor_id"`
	FeatureName  string    `json:"feature_name"`
	Severity     Severity  `json:"severity"`
	Message      string    `json:"message"`
	Value        float64   `json:"value"`
	Threshold    float64   `json:"threshold"`
	Timestamp    time.Time `json:"timestamp"`
	Acknowledged bool      `json:"acknowledged"`
}

// Notifier is an interface for alert delivery.
type Notifier interface {
	Name() string
	Notify(alert Alert) error
}

// MonitoringSummary provides an overview of the monitoring state.
type MonitoringSummary struct {
	TotalMonitors int `json:"total_monitors"`
	HealthyCount  int `json:"healthy_count"`
	WarningCount  int `json:"warning_count"`
	CriticalCount int `json:"critical_count"`
	TotalAlerts   int `json:"total_alerts"`
	UnackedAlerts int `json:"unacked_alerts"`
	RuleCount     int `json:"rule_count"`
	NotifierCount int `json:"notifier_count"`
}

// NewManager creates a new monitoring manager with the given configuration.
func NewManager(config ManagerConfig) *Manager {
	if config.MaxMonitors == 0 {
		config = DefaultManagerConfig()
	}
	return &Manager{
		config:    config,
		monitors:  make(map[string]*FeatureMonitor),
		rules:     make(map[string]*AlertRule),
		alerts:    make([]Alert, 0),
		notifiers: make([]Notifier, 0),
	}
}

// RegisterMonitor adds a monitor. Returns error on duplicate ID or capacity limit.
func (m *Manager) RegisterMonitor(monitor *FeatureMonitor) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.monitors[monitor.ID]; exists {
		return fmt.Errorf("%w: %s", ErrAlreadyExists, monitor.ID)
	}
	if len(m.monitors) >= m.config.MaxMonitors {
		return fmt.Errorf("monitor limit reached (%d)", m.config.MaxMonitors)
	}
	if monitor.Status == "" {
		monitor.Status = StatusUnknown
	}
	if monitor.CreatedAt.IsZero() {
		monitor.CreatedAt = time.Now()
	}
	m.monitors[monitor.ID] = monitor
	return nil
}

// RemoveMonitor removes a monitor by ID.
func (m *Manager) RemoveMonitor(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.monitors[id]; !exists {
		return fmt.Errorf("%w: %s", ErrMonitorNotFound, id)
	}
	delete(m.monitors, id)
	return nil
}

// GetMonitor retrieves a monitor by ID.
func (m *Manager) GetMonitor(id string) (*FeatureMonitor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mon, exists := m.monitors[id]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrMonitorNotFound, id)
	}
	return mon, nil
}

// ListMonitors returns all registered monitors.
func (m *Manager) ListMonitors() []*FeatureMonitor {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*FeatureMonitor, 0, len(m.monitors))
	for _, mon := range m.monitors {
		result = append(result, mon)
	}
	return result
}

// RecordValue updates a monitor's current value and evaluates alerting rules.
func (m *Manager) RecordValue(monitorID string, value float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mon, exists := m.monitors[monitorID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrMonitorNotFound, monitorID)
	}

	mon.CurrentValue = value
	mon.LastCheck = time.Now()
	mon.CheckCount++

	m.checkRules(mon)
	return nil
}

// AddRule adds an alerting rule.
func (m *Manager) AddRule(rule *AlertRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[rule.ID]; exists {
		return fmt.Errorf("%w: %s", ErrAlreadyExists, rule.ID)
	}
	if rule.Cooldown == 0 {
		rule.Cooldown = m.config.AlertCooldown
	}
	m.rules[rule.ID] = rule
	return nil
}

// RemoveRule removes an alerting rule by ID.
func (m *Manager) RemoveRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[id]; !exists {
		return fmt.Errorf("%w: %s", ErrRuleNotFound, id)
	}
	delete(m.rules, id)
	return nil
}

// ListRules returns all alerting rules.
func (m *Manager) ListRules() []*AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AlertRule, 0, len(m.rules))
	for _, rule := range m.rules {
		result = append(result, rule)
	}
	return result
}

// GetAlerts returns alerts fired since the given time.
func (m *Manager) GetAlerts(since time.Time) []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Alert
	for _, a := range m.alerts {
		if !a.Timestamp.Before(since) {
			result = append(result, a)
		}
	}
	return result
}

// AcknowledgeAlert marks an alert as acknowledged.
func (m *Manager) AcknowledgeAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.alerts {
		if m.alerts[i].ID == alertID {
			m.alerts[i].Acknowledged = true
			return nil
		}
	}
	return fmt.Errorf("alert not found: %s", alertID)
}

// AddNotifier registers a notifier for alert delivery.
func (m *Manager) AddNotifier(n Notifier) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.notifiers = append(m.notifiers, n)
}

// checkRules evaluates all rules for the given monitor and fires alerts as needed.
// Must be called with m.mu held.
func (m *Manager) checkRules(monitor *FeatureMonitor) {
	hasWarning := false
	hasCritical := false

	for _, rule := range m.rules {
		if rule.MonitorID != monitor.ID || !rule.Enabled {
			continue
		}

		triggered := false
		switch rule.Condition {
		case "above":
			triggered = monitor.CurrentValue > rule.Threshold
		case "below":
			triggered = monitor.CurrentValue < rule.Threshold
		case "equals":
			triggered = monitor.CurrentValue == rule.Threshold
		}

		if triggered {
			if rule.Severity == SeverityWarning {
				hasWarning = true
			}
			if rule.Severity == SeverityCritical {
				hasCritical = true
			}

			if time.Since(rule.LastFired) >= rule.Cooldown {
				m.fireAlert(rule, monitor)
			}
		}
	}

	if hasCritical {
		monitor.Status = StatusCritical
	} else if hasWarning {
		monitor.Status = StatusWarning
	} else {
		monitor.Status = StatusHealthy
	}
}

// fireAlert creates an alert and sends it to all notifiers.
// Must be called with m.mu held.
func (m *Manager) fireAlert(rule *AlertRule, monitor *FeatureMonitor) {
	alert := Alert{
		ID:          fmt.Sprintf("mon-%d", time.Now().UnixNano()),
		RuleID:      rule.ID,
		MonitorID:   monitor.ID,
		FeatureName: monitor.FeatureName,
		Severity:    rule.Severity,
		Message:     fmt.Sprintf("Feature %s %s threshold %.2f (current: %.2f)", monitor.FeatureName, rule.Condition, rule.Threshold, monitor.CurrentValue),
		Value:       monitor.CurrentValue,
		Threshold:   rule.Threshold,
		Timestamp:   time.Now(),
	}

	if len(m.alerts) < m.config.MaxAlerts {
		m.alerts = append(m.alerts, alert)
	}

	rule.LastFired = time.Now()
	monitor.AlertCount++

	for _, n := range m.notifiers {
		_ = n.Notify(alert)
	}
}

// Summary returns an overview of the monitoring state.
func (m *Manager) Summary() *MonitoringSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s := &MonitoringSummary{
		TotalMonitors: len(m.monitors),
		TotalAlerts:   len(m.alerts),
		RuleCount:     len(m.rules),
		NotifierCount: len(m.notifiers),
	}

	for _, mon := range m.monitors {
		switch mon.Status {
		case StatusHealthy:
			s.HealthyCount++
		case StatusWarning:
			s.WarningCount++
		case StatusCritical:
			s.CriticalCount++
		}
	}

	for _, a := range m.alerts {
		if !a.Acknowledged {
			s.UnackedAlerts++
		}
	}

	return s
}
