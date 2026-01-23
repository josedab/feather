package webhooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// EventType identifies the kind of feature lifecycle event.
type EventType string

const (
	EventFeatureCreated  EventType = "feature.created"
	EventFeatureUpdated  EventType = "feature.updated"
	EventDriftDetected   EventType = "drift.detected"
	EventSLABreached     EventType = "sla.breached"
	EventAnomalyDetected EventType = "anomaly.detected"
	EventSchemaChanged   EventType = "schema.changed"
)

// Event represents a feature lifecycle event.
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Source    string                 `json:"source"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// WebhookConfig defines a webhook endpoint and its subscription.
type WebhookConfig struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	URL       string            `json:"url"`
	Events    []EventType       `json:"events"`
	Headers   map[string]string `json:"headers"`
	Secret    string            `json:"secret"`
	Active    bool              `json:"active"`
	CreatedAt time.Time         `json:"created_at"`
}

// DeliveryResult records the outcome of a webhook delivery attempt.
type DeliveryResult struct {
	WebhookID  string    `json:"webhook_id"`
	EventID    string    `json:"event_id"`
	StatusCode int       `json:"status_code"`
	Success    bool      `json:"success"`
	Attempt    int       `json:"attempt"`
	Error      string    `json:"error,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// DispatcherConfig configures the webhook dispatcher.
type DispatcherConfig struct {
	MaxWebhooks    int `json:"max_webhooks"`
	MaxRetries     int `json:"max_retries"`
	RetryDelayMs   int `json:"retry_delay_ms"`
	DeadLetterSize int `json:"dead_letter_size"`
}

// DefaultDispatcherConfig returns sensible defaults.
func DefaultDispatcherConfig() DispatcherConfig {
	return DispatcherConfig{
		MaxWebhooks:    1000,
		MaxRetries:     3,
		RetryDelayMs:   1000,
		DeadLetterSize: 10000,
	}
}

// DispatcherStats holds dispatcher statistics.
type DispatcherStats struct {
	TotalWebhooks   int   `json:"total_webhooks"`
	ActiveWebhooks  int   `json:"active_webhooks"`
	TotalDispatched int64 `json:"total_dispatched"`
	TotalFailed     int64 `json:"total_failed"`
	DeadLetterSize  int   `json:"dead_letter_size"`
}

// Dispatcher manages webhook registrations and event delivery.
type Dispatcher struct {
	mu              sync.RWMutex
	config          DispatcherConfig
	webhooks        map[string]*WebhookConfig
	deliveries      []DeliveryResult
	deadLetter      []Event
	totalDispatched atomic.Int64
	totalFailed     atomic.Int64
	httpClient      *http.Client
}

// NewDispatcher creates a new webhook dispatcher.
func NewDispatcher(config DispatcherConfig) *Dispatcher {
	timeout := time.Duration(config.RetryDelayMs) * time.Millisecond * 3
	return &Dispatcher{
		config:     config,
		webhooks:   make(map[string]*WebhookConfig),
		deliveries: make([]DeliveryResult, 0),
		deadLetter: make([]Event, 0),
		httpClient: &http.Client{Timeout: timeout},
	}
}

// RegisterWebhook adds a new webhook configuration.
func (d *Dispatcher) RegisterWebhook(wh WebhookConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.webhooks[wh.ID]; exists {
		return fmt.Errorf("webhook %s: %w", wh.ID, ErrWebhookExists)
	}
	if len(d.webhooks) >= d.config.MaxWebhooks {
		return fmt.Errorf("max webhooks (%d) reached", d.config.MaxWebhooks)
	}

	copy := wh
	d.webhooks[wh.ID] = &copy
	return nil
}

// UpdateWebhook updates an existing webhook configuration.
func (d *Dispatcher) UpdateWebhook(id string, wh WebhookConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.webhooks[id]; !exists {
		return fmt.Errorf("webhook %s: %w", id, ErrWebhookNotFound)
	}

	copy := wh
	copy.ID = id
	d.webhooks[id] = &copy
	return nil
}

// DeleteWebhook removes a webhook by ID.
func (d *Dispatcher) DeleteWebhook(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.webhooks[id]; !exists {
		return fmt.Errorf("webhook %s: %w", id, ErrWebhookNotFound)
	}

	delete(d.webhooks, id)
	return nil
}

// GetWebhook retrieves a webhook by ID.
func (d *Dispatcher) GetWebhook(id string) (*WebhookConfig, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	wh, exists := d.webhooks[id]
	if !exists {
		return nil, fmt.Errorf("webhook %s: %w", id, ErrWebhookNotFound)
	}

	copy := *wh
	return &copy, nil
}

// ListWebhooks returns all registered webhooks.
func (d *Dispatcher) ListWebhooks() []WebhookConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]WebhookConfig, 0, len(d.webhooks))
	for _, wh := range d.webhooks {
		result = append(result, *wh)
	}
	return result
}

// Dispatch sends an event to all matching webhooks.
func (d *Dispatcher) Dispatch(event Event) []DeliveryResult {
	d.mu.Lock()
	defer d.mu.Unlock()

	var results []DeliveryResult

	for _, wh := range d.webhooks {
		if !wh.Active {
			continue
		}
		if !d.matchesEvent(wh, event.Type) {
			continue
		}

		result := DeliveryResult{
			WebhookID: wh.ID,
			EventID:   event.ID,
			Attempt:   1,
			Timestamp: time.Now(),
		}

		if strings.HasPrefix(wh.URL, "http://") || strings.HasPrefix(wh.URL, "https://") {
			body, _ := json.Marshal(event)
			resp, err := d.httpClient.Post(wh.URL, "application/json", bytes.NewReader(body))
			if err != nil {
				result.Success = false
				result.StatusCode = 0
				result.Error = err.Error()
				d.totalFailed.Add(1)
				d.deadLetter = append(d.deadLetter, event)
			} else {
				resp.Body.Close()
				result.StatusCode = resp.StatusCode
				if resp.StatusCode >= 400 {
					result.Success = false
					result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
					d.totalFailed.Add(1)
					d.deadLetter = append(d.deadLetter, event)
				} else {
					result.Success = true
				}
			}
		} else {
			result.StatusCode = 200
			result.Success = true
		}

		results = append(results, result)
		d.deliveries = append(d.deliveries, result)
		d.totalDispatched.Add(1)
	}

	return results
}

// GetDeliveries returns delivery results for a webhook, limited to the
// most recent entries up to limit.
func (d *Dispatcher) GetDeliveries(webhookID string, limit int) []DeliveryResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var results []DeliveryResult
	for i := len(d.deliveries) - 1; i >= 0 && len(results) < limit; i-- {
		if d.deliveries[i].WebhookID == webhookID {
			results = append(results, d.deliveries[i])
		}
	}
	return results
}

// GetDeadLetter returns events from the dead-letter queue.
func (d *Dispatcher) GetDeadLetter(limit int) []Event {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if limit > len(d.deadLetter) {
		limit = len(d.deadLetter)
	}
	result := make([]Event, limit)
	copy(result, d.deadLetter[:limit])
	return result
}

// RetryDeadLetter attempts to re-dispatch all dead-letter events.
func (d *Dispatcher) RetryDeadLetter() []DeliveryResult {
	d.mu.Lock()
	events := make([]Event, len(d.deadLetter))
	copy(events, d.deadLetter)
	d.deadLetter = d.deadLetter[:0]
	d.mu.Unlock()

	var results []DeliveryResult
	for _, event := range events {
		r := d.Dispatch(event)
		results = append(results, r...)
	}
	return results
}

// Stats returns dispatcher statistics.
func (d *Dispatcher) Stats() DispatcherStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	active := 0
	for _, wh := range d.webhooks {
		if wh.Active {
			active++
		}
	}

	return DispatcherStats{
		TotalWebhooks:   len(d.webhooks),
		ActiveWebhooks:  active,
		TotalDispatched: d.totalDispatched.Load(),
		TotalFailed:     d.totalFailed.Load(),
		DeadLetterSize:  len(d.deadLetter),
	}
}

func (d *Dispatcher) matchesEvent(wh *WebhookConfig, eventType EventType) bool {
	if len(wh.Events) == 0 {
		return true
	}
	for _, e := range wh.Events {
		if e == eventType {
			return true
		}
	}
	return false
}
