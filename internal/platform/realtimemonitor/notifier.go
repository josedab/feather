package realtimemonitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// NotifierType identifies the alerting integration type.
type NotifierType string

const (
	NotifierSlack    NotifierType = "slack"
	NotifierPagerDuty NotifierType = "pagerduty"
	NotifierWebhook  NotifierType = "webhook"
)

// NotifierConfig defines an alerting integration.
type NotifierConfig struct {
	ID         string            `json:"id"`
	Type       NotifierType      `json:"type"`
	Name       string            `json:"name"`
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers,omitempty"`
	Enabled    bool              `json:"enabled"`
	MinSeverity AlertSeverity    `json:"min_severity"`
	CreatedAt  time.Time         `json:"created_at"`
}

// NotificationResult records the outcome of sending a notification.
type NotificationResult struct {
	NotifierID string    `json:"notifier_id"`
	AlertID    string    `json:"alert_id"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	SentAt     time.Time `json:"sent_at"`
}

// AlertNotifier manages alerting integrations and dispatches notifications.
type AlertNotifier struct {
	mu        sync.RWMutex
	notifiers map[string]*NotifierConfig
	history   []NotificationResult
	client    *http.Client
	maxHistory int
}

// NewAlertNotifier creates a new alert notifier.
func NewAlertNotifier() *AlertNotifier {
	return &AlertNotifier{
		notifiers:  make(map[string]*NotifierConfig),
		client:     &http.Client{Timeout: 10 * time.Second},
		maxHistory: 1000,
	}
}

// AddNotifier registers an alerting integration.
func (an *AlertNotifier) AddNotifier(config NotifierConfig) error {
	if config.ID == "" || config.URL == "" {
		return fmt.Errorf("id and url are required")
	}

	an.mu.Lock()
	defer an.mu.Unlock()

	config.CreatedAt = time.Now()
	if config.MinSeverity == "" {
		config.MinSeverity = SeverityWarning
	}
	an.notifiers[config.ID] = &config
	return nil
}

// RemoveNotifier removes an alerting integration.
func (an *AlertNotifier) RemoveNotifier(id string) error {
	an.mu.Lock()
	defer an.mu.Unlock()

	if _, exists := an.notifiers[id]; !exists {
		return fmt.Errorf("notifier %s not found", id)
	}
	delete(an.notifiers, id)
	return nil
}

// ListNotifiers returns all registered notifiers.
func (an *AlertNotifier) ListNotifiers() []NotifierConfig {
	an.mu.RLock()
	defer an.mu.RUnlock()

	result := make([]NotifierConfig, 0, len(an.notifiers))
	for _, n := range an.notifiers {
		result = append(result, *n)
	}
	return result
}

// Notify sends an alert to all matching notifiers.
func (an *AlertNotifier) Notify(alert Alert) []NotificationResult {
	an.mu.RLock()
	notifiers := make([]*NotifierConfig, 0)
	for _, n := range an.notifiers {
		if n.Enabled && severityGTE(alert.Severity, n.MinSeverity) {
			cp := *n
			notifiers = append(notifiers, &cp)
		}
	}
	an.mu.RUnlock()

	var results []NotificationResult
	for _, n := range notifiers {
		result := an.dispatch(n, alert)
		results = append(results, result)
	}

	an.mu.Lock()
	an.history = append(an.history, results...)
	if len(an.history) > an.maxHistory {
		an.history = an.history[len(an.history)-an.maxHistory:]
	}
	an.mu.Unlock()

	return results
}

// GetHistory returns recent notification results.
func (an *AlertNotifier) GetHistory(limit int) []NotificationResult {
	an.mu.RLock()
	defer an.mu.RUnlock()

	if limit <= 0 || limit > len(an.history) {
		limit = len(an.history)
	}
	start := len(an.history) - limit
	out := make([]NotificationResult, limit)
	copy(out, an.history[start:])
	return out
}

func (an *AlertNotifier) dispatch(n *NotifierConfig, alert Alert) NotificationResult {
	result := NotificationResult{
		NotifierID: n.ID,
		AlertID:    alert.ID,
		SentAt:     time.Now(),
	}

	var payload []byte
	var err error

	switch n.Type {
	case NotifierSlack:
		payload, err = buildSlackPayload(alert)
	case NotifierPagerDuty:
		payload, err = buildPagerDutyPayload(alert)
	case NotifierWebhook:
		payload, err = json.Marshal(alert)
	default:
		result.Error = fmt.Sprintf("unknown notifier type: %s", n.Type)
		return result
	}

	if err != nil {
		result.Error = fmt.Sprintf("building payload: %v", err)
		return result
	}

	req, err := http.NewRequest(http.MethodPost, n.URL, bytes.NewReader(payload))
	if err != nil {
		result.Error = fmt.Sprintf("creating request: %v", err)
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range n.Headers {
		req.Header.Set(k, v)
	}

	resp, err := an.client.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("sending notification: %v", err)
		return result
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Success = true
	} else {
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return result
}

func buildSlackPayload(alert Alert) ([]byte, error) {
	emoji := ":information_source:"
	switch alert.Severity {
	case SeverityWarning:
		emoji = ":warning:"
	case SeverityCritical:
		emoji = ":rotating_light:"
	}

	payload := map[string]interface{}{
		"text": fmt.Sprintf("%s *[%s] %s*\n%s\nSource: %s",
			emoji, alert.Severity, alert.Name, alert.Message, alert.Source),
		"blocks": []map[string]interface{}{
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": fmt.Sprintf("%s *[%s] %s*", emoji, alert.Severity, alert.Name),
				},
			},
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": alert.Message,
				},
			},
		},
	}
	return json.Marshal(payload)
}

func buildPagerDutyPayload(alert Alert) ([]byte, error) {
	severity := "info"
	switch alert.Severity {
	case SeverityWarning:
		severity = "warning"
	case SeverityCritical:
		severity = "critical"
	}

	payload := map[string]interface{}{
		"routing_key":  "", // Set via headers
		"event_action": "trigger",
		"payload": map[string]interface{}{
			"summary":   fmt.Sprintf("[%s] %s: %s", alert.Severity, alert.Name, alert.Message),
			"source":    alert.Source,
			"severity":  severity,
			"timestamp": alert.FiredAt.Format(time.RFC3339),
			"custom_details": alert.Labels,
		},
		"dedup_key": alert.ID,
	}
	return json.Marshal(payload)
}

func severityGTE(a, b AlertSeverity) bool {
	order := map[AlertSeverity]int{
		SeverityInfo:     0,
		SeverityWarning:  1,
		SeverityCritical: 2,
	}
	return order[a] >= order[b]
}

// AddNotifier integrates an alerting notifier with the dashboard.
func (d *Dashboard) AddNotifier(config NotifierConfig) error {
	d.mu.Lock()
	if d.notifier == nil {
		d.notifier = NewAlertNotifier()
	}
	d.mu.Unlock()
	return d.notifier.AddNotifier(config)
}

// GetNotifier returns the dashboard's alert notifier.
func (d *Dashboard) GetNotifier() *AlertNotifier {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.notifier
}
