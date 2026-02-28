package monitoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/feather-store/feather/internal/platform/urlvalidation"
)

// WebhookNotifier sends alerts to a webhook URL.
type WebhookNotifier struct {
	name    string
	url     string
	client  *http.Client
	headers map[string]string
}

// NewWebhookNotifier creates a notifier that POSTs alerts as JSON to the given URL.
func NewWebhookNotifier(name, url string) *WebhookNotifier {
	return &WebhookNotifier{
		name:    name,
		url:     url,
		client:  &http.Client{},
		headers: make(map[string]string),
	}
}

func (w *WebhookNotifier) Name() string { return w.name }

// Notify sends the alert as a JSON POST to the configured webhook URL.
func (w *WebhookNotifier) Notify(alert Alert) error {
	if err := urlvalidation.ValidateWebhookURL(w.url); err != nil {
		return fmt.Errorf("webhook URL blocked by SSRF protection: %w", err)
	}

	body, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("marshaling alert: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// LogNotifier logs alerts using slog.
type LogNotifier struct {
	name string
}

// NewLogNotifier creates a notifier that logs alerts via slog.
func NewLogNotifier(name string) *LogNotifier {
	return &LogNotifier{name: name}
}

func (l *LogNotifier) Name() string { return l.name }

// Notify logs the alert details at warning level.
func (l *LogNotifier) Notify(alert Alert) error {
	slog.Warn("alert fired",
		"alert_id", alert.ID,
		"feature", alert.FeatureName,
		"severity", alert.Severity,
		"message", alert.Message,
		"value", alert.Value,
		"threshold", alert.Threshold,
	)
	return nil
}
