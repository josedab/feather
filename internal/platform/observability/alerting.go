package observability

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/core/storage"
)

// AlertSeverity represents alert severity levels.
type AlertSeverity string

// AlertSeverity constants for alert severity.
const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityError    AlertSeverity = "error"
	SeverityCritical AlertSeverity = "critical"
)

// AlertType represents types of alerts.
type AlertType string

// AlertType constants for alert categories.
const (
	AlertTypeFreshness    AlertType = "freshness"
	AlertTypeQuality      AlertType = "quality"
	AlertTypeDrift        AlertType = "drift"
	AlertTypeAnomaly      AlertType = "anomaly"
	AlertTypePerformance  AlertType = "performance"
	AlertTypeAvailability AlertType = "availability"
)

// Alert represents an observability alert.
type Alert struct {
	ID          string                 `json:"id"`
	Type        AlertType              `json:"type"`
	Severity    AlertSeverity          `json:"severity"`
	Feature     string                 `json:"feature"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Value       interface{}            `json:"value,omitempty"`
	Threshold   interface{}            `json:"threshold,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
	AckAt       *time.Time             `json:"ack_at,omitempty"`
	AckBy       string                 `json:"ack_by,omitempty"`
}

// AlertRule defines when to trigger alerts.
type AlertRule struct {
	Name      string            `json:"name"`
	Type      AlertType         `json:"type"`
	Feature   string            `json:"feature"`
	Condition string            `json:"condition"` // lt, gt, eq, ne
	Threshold float64           `json:"threshold"`
	Duration  time.Duration     `json:"duration"`
	Severity  AlertSeverity     `json:"severity"`
	Enabled   bool              `json:"enabled"`
	Cooldown  time.Duration     `json:"cooldown"`
	LastFired time.Time         `json:"last_fired"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// AlertHandler is called when an alert is triggered.
type AlertHandler interface {
	HandleAlert(ctx context.Context, alert *Alert) error
}

// AlertHandlerFunc is a function that implements AlertHandler.
type AlertHandlerFunc func(ctx context.Context, alert *Alert) error

// HandleAlert calls the wrapped handler function.
func (f AlertHandlerFunc) HandleAlert(ctx context.Context, alert *Alert) error {
	return f(ctx, alert)
}

// AlertManager manages alerts and notifications.
type AlertManager struct {
	rules    map[string]*AlertRule
	alerts   []*Alert
	handlers []AlertHandler
	nextID   int64
	mu       sync.RWMutex
}

// NewAlertManager creates a new alert manager.
func NewAlertManager() *AlertManager {
	return &AlertManager{
		rules:    make(map[string]*AlertRule),
		alerts:   make([]*Alert, 0),
		handlers: make([]AlertHandler, 0),
	}
}

// AddRule adds an alert rule.
func (m *AlertManager) AddRule(rule *AlertRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules[rule.Name] = rule
}

// RemoveRule removes an alert rule.
func (m *AlertManager) RemoveRule(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rules, name)
}

// GetRules returns all alert rules.
func (m *AlertManager) GetRules() []*AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AlertRule, 0, len(m.rules))
	for _, r := range m.rules {
		result = append(result, r)
	}
	return result
}

// AddHandler adds an alert handler.
func (m *AlertManager) AddHandler(handler AlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
}

// Trigger creates and fires an alert.
func (m *AlertManager) Trigger(ctx context.Context, alertType AlertType, severity AlertSeverity, feature, title, description string, value, threshold interface{}) *Alert {
	m.mu.Lock()
	m.nextID++
	id := m.nextID
	m.mu.Unlock()

	alert := &Alert{
		ID:          fmt.Sprintf("%d", id),
		Type:        alertType,
		Severity:    severity,
		Feature:     feature,
		Title:       title,
		Description: description,
		Value:       value,
		Threshold:   threshold,
		CreatedAt:   time.Now(),
	}

	m.mu.Lock()
	m.alerts = append(m.alerts, alert)
	// Keep last 1000 alerts
	if len(m.alerts) > 1000 {
		m.alerts = m.alerts[len(m.alerts)-1000:]
	}
	handlers := make([]AlertHandler, len(m.handlers))
	copy(handlers, m.handlers)
	m.mu.Unlock()

	// Notify handlers
	for _, handler := range handlers {
		go func(h AlertHandler) {
			_ = h.HandleAlert(ctx, alert)
		}(handler)
	}

	return alert
}

// CheckRules evaluates all rules against current metrics.
func (m *AlertManager) CheckRules(ctx context.Context, metrics *MetricsCollector) {
	m.mu.RLock()
	rules := make([]*AlertRule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	m.mu.RUnlock()

	now := time.Now()

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// Check cooldown
		if now.Sub(rule.LastFired) < rule.Cooldown {
			continue
		}

		fm := metrics.GetMetrics(rule.Feature)
		if fm == nil {
			continue
		}

		var value float64
		var shouldAlert bool

		switch rule.Type {
		case AlertTypePerformance:
			value = fm.P99LatencyUs
			shouldAlert = m.evaluateCondition(rule.Condition, value, rule.Threshold)

		case AlertTypeAvailability:
			if fm.ReadCount+fm.WriteCount > 0 {
				value = float64(fm.ErrorCount) / float64(fm.ReadCount+fm.WriteCount) * 100
				shouldAlert = m.evaluateCondition(rule.Condition, value, rule.Threshold)
			}

		case AlertTypeFreshness:
			if !fm.LastWrite.IsZero() {
				value = time.Since(fm.LastWrite).Seconds()
				shouldAlert = m.evaluateCondition(rule.Condition, value, rule.Threshold)
			}
		case AlertTypeQuality, AlertTypeDrift, AlertTypeAnomaly:
		}

		if shouldAlert {
			m.mu.Lock()
			rule.LastFired = now
			m.mu.Unlock()

			m.Trigger(ctx, rule.Type, rule.Severity, rule.Feature,
				"Alert: "+rule.Name,
				"Threshold exceeded",
				value, rule.Threshold)
		}
	}
}

func (m *AlertManager) evaluateCondition(condition string, value, threshold float64) bool {
	switch condition {
	case "lt":
		return value < threshold
	case "gt":
		return value > threshold
	case "eq":
		return value == threshold
	case "ne":
		return value != threshold
	case "lte":
		return value <= threshold
	case "gte":
		return value >= threshold
	default:
		return false
	}
}

// GetAlerts returns alerts matching the filter.
func (m *AlertManager) GetAlerts(alertType AlertType, feature string, since time.Time) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Alert, 0)
	for _, a := range m.alerts {
		if alertType != "" && a.Type != alertType {
			continue
		}
		if feature != "" && a.Feature != feature {
			continue
		}
		if a.CreatedAt.Before(since) {
			continue
		}
		result = append(result, a)
	}

	return result
}

// Acknowledge acknowledges an alert.
func (m *AlertManager) Acknowledge(alertID string, by string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, a := range m.alerts {
		if a.ID == alertID {
			now := time.Now()
			a.AckAt = &now
			a.AckBy = by
			return true
		}
	}
	return false
}

// Resolve marks an alert as resolved.
func (m *AlertManager) Resolve(alertID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, a := range m.alerts {
		if a.ID == alertID {
			now := time.Now()
			a.ResolvedAt = &now
			return true
		}
	}
	return false
}

// ObservabilityStack combines all observability components.
type ObservabilityStack struct { //nolint:revive
	Metrics   *MetricsCollector
	Freshness *FreshnessChecker
	Usage     *UsageTracker
	Quality   *QualityMonitor
	Alerts    *AlertManager
	stopCh    chan struct{}
}

// NewObservabilityStack creates a complete observability stack.
func NewObservabilityStack(store interface{}) *ObservabilityStack {
	metrics := NewMetricsCollector()

	stack := &ObservabilityStack{
		Metrics:   metrics,
		Freshness: NewFreshnessChecker(metrics, time.Hour),
		Usage:     NewUsageTracker(metrics),
		Alerts:    NewAlertManager(),
		stopCh:    make(chan struct{}),
	}

	// Only create quality monitor if we have a compatible store
	if s, ok := store.(*storage.Store); ok {
		stack.Quality = NewQualityMonitor(s)
	}

	return stack
}

// Start starts background monitoring tasks.
func (s *ObservabilityStack) Start(ctx context.Context) {
	// Record usage patterns every hour
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.Usage.Record(ctx)
			}
		}
	}()

	// Check alert rules every minute
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.Alerts.CheckRules(ctx, s.Metrics)
			}
		}
	}()
}

// Stop stops background monitoring tasks.
func (s *ObservabilityStack) Stop() {
	close(s.stopCh)
}
