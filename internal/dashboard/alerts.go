package dashboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// AlertManager manages dashboard alerts.
type AlertManager struct {
	mu         sync.RWMutex
	alerts     map[string]*Alert
	webhookURL string
	client     *http.Client
}

// Alert represents a dashboard alert.
type Alert struct {
	ID           string                 `json:"id"`
	Title        string                 `json:"title"`
	Message      string                 `json:"message"`
	Severity     AlertSeverity          `json:"severity"`
	Source       string                 `json:"source"`
	FeatureName  string                 `json:"feature_name,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	AcknowledgedAt *time.Time           `json:"acknowledged_at,omitempty"`
	ResolvedAt   *time.Time             `json:"resolved_at,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// AlertSeverity indicates alert severity.
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// NewAlertManager creates a new alert manager.
func NewAlertManager(webhookURL string) *AlertManager {
	return &AlertManager{
		alerts:     make(map[string]*Alert),
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Create creates a new alert.
func (m *AlertManager) Create(alert *Alert) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if alert.ID == "" {
		alert.ID = fmt.Sprintf("alert-%d", time.Now().UnixNano())
	}
	alert.CreatedAt = time.Now()

	m.alerts[alert.ID] = alert

	// Send webhook notification
	if m.webhookURL != "" {
		go m.sendWebhook(alert)
	}

	return nil
}

// Get returns an alert by ID.
func (m *AlertManager) Get(id string) (*Alert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alert, ok := m.alerts[id]
	if !ok {
		return nil, fmt.Errorf("alert not found: %s", id)
	}
	return alert, nil
}

// GetAll returns all alerts.
func (m *AlertManager) GetAll() []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*Alert, 0, len(m.alerts))
	for _, a := range m.alerts {
		alerts = append(alerts, a)
	}
	return alerts
}

// GetActive returns unacknowledged alerts.
func (m *AlertManager) GetActive() []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var alerts []*Alert
	for _, a := range m.alerts {
		if a.AcknowledgedAt == nil && a.ResolvedAt == nil {
			alerts = append(alerts, a)
		}
	}
	return alerts
}

// Acknowledge acknowledges an alert.
func (m *AlertManager) Acknowledge(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[id]
	if !ok {
		return fmt.Errorf("alert not found: %s", id)
	}

	now := time.Now()
	alert.AcknowledgedAt = &now
	return nil
}

// Resolve resolves an alert.
func (m *AlertManager) Resolve(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[id]
	if !ok {
		return fmt.Errorf("alert not found: %s", id)
	}

	now := time.Now()
	alert.ResolvedAt = &now
	return nil
}

// Delete deletes an alert.
func (m *AlertManager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.alerts[id]; !ok {
		return fmt.Errorf("alert not found: %s", id)
	}

	delete(m.alerts, id)
	return nil
}

// Cleanup removes old resolved alerts.
func (m *AlertManager) Cleanup(maxAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, alert := range m.alerts {
		if alert.ResolvedAt != nil && alert.ResolvedAt.Before(cutoff) {
			delete(m.alerts, id)
		}
	}
}

func (m *AlertManager) sendWebhook(alert *Alert) {
	payload := map[string]interface{}{
		"type":     "alert",
		"alert":    alert,
		"sent_at":  time.Now(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	resp, err := m.client.Post(m.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return
	}
	resp.Body.Close()
}

// AlertForDrift creates an alert for drift detection.
func AlertForDrift(featureName string, score float64, threshold float64) *Alert {
	return &Alert{
		Title:       fmt.Sprintf("Drift detected: %s", featureName),
		Message:     fmt.Sprintf("Feature %s drift score (%.4f) exceeds threshold (%.4f)", featureName, score, threshold),
		Severity:    SeverityWarning,
		Source:      "drift_detector",
		FeatureName: featureName,
		Metadata: map[string]interface{}{
			"score":     score,
			"threshold": threshold,
		},
	}
}

// AlertForStaleness creates an alert for stale features.
func AlertForStaleness(featureName string, lastUpdated time.Time, expectedTTL time.Duration) *Alert {
	staleness := time.Since(lastUpdated)
	severity := SeverityWarning
	if staleness > 2*expectedTTL {
		severity = SeverityCritical
	}

	return &Alert{
		Title:       fmt.Sprintf("Stale feature: %s", featureName),
		Message:     fmt.Sprintf("Feature %s hasn't been updated for %s (expected TTL: %s)", featureName, staleness.Round(time.Minute), expectedTTL),
		Severity:    severity,
		Source:      "freshness_monitor",
		FeatureName: featureName,
		Metadata: map[string]interface{}{
			"last_updated": lastUpdated,
			"staleness":    staleness.String(),
			"expected_ttl": expectedTTL.String(),
		},
	}
}

// AlertForError creates an alert for system errors.
func AlertForError(source, message string, err error) *Alert {
	return &Alert{
		Title:    fmt.Sprintf("Error in %s", source),
		Message:  message,
		Severity: SeverityCritical,
		Source:   source,
		Metadata: map[string]interface{}{
			"error": err.Error(),
		},
	}
}
