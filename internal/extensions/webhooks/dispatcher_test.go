package webhooks

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDispatcher(t *testing.T) {
	cfg := DefaultDispatcherConfig()
	d := NewDispatcher(cfg)
	require.NotNil(t, d)
	assert.Equal(t, 1000, d.config.MaxWebhooks)
	assert.Empty(t, d.webhooks)
}

func TestRegisterWebhook(t *testing.T) {
	d := NewDispatcher(DefaultDispatcherConfig())

	wh := WebhookConfig{
		ID:     "wh-1",
		Name:   "Test Webhook",
		URL:    "webhook://test/hook",
		Events: []EventType{EventDriftDetected},
		Active: true,
	}
	require.NoError(t, d.RegisterWebhook(wh))

	got, err := d.GetWebhook("wh-1")
	require.NoError(t, err)
	assert.Equal(t, "Test Webhook", got.Name)

	// duplicate registration
	err = d.RegisterWebhook(wh)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrWebhookExists))
}

func TestDispatch(t *testing.T) {
	d := NewDispatcher(DefaultDispatcherConfig())

	wh := WebhookConfig{
		ID:     "wh-drift",
		Name:   "Drift Hook",
		URL:    "webhook://test/drift",
		Events: []EventType{EventDriftDetected},
		Active: true,
	}
	require.NoError(t, d.RegisterWebhook(wh))

	event := Event{
		ID:        "evt-1",
		Type:      EventDriftDetected,
		Source:    "test",
		Data:      map[string]interface{}{"feature": "user_age"},
		Timestamp: time.Now(),
	}

	results := d.Dispatch(event)
	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, "wh-drift", results[0].WebhookID)
	assert.Equal(t, "evt-1", results[0].EventID)
	assert.Equal(t, 200, results[0].StatusCode)
}

func TestEventFiltering(t *testing.T) {
	d := NewDispatcher(DefaultDispatcherConfig())

	wh := WebhookConfig{
		ID:     "wh-sla",
		Name:   "SLA Hook",
		URL:    "webhook://test/sla",
		Events: []EventType{EventSLABreached},
		Active: true,
	}
	require.NoError(t, d.RegisterWebhook(wh))

	// dispatch a different event type
	event := Event{
		ID:        "evt-2",
		Type:      EventFeatureCreated,
		Source:    "test",
		Timestamp: time.Now(),
	}

	results := d.Dispatch(event)
	assert.Empty(t, results)
}

func TestDeleteWebhook(t *testing.T) {
	d := NewDispatcher(DefaultDispatcherConfig())

	wh := WebhookConfig{ID: "wh-del", Name: "Delete Me", Active: true}
	require.NoError(t, d.RegisterWebhook(wh))
	require.NoError(t, d.DeleteWebhook("wh-del"))

	_, err := d.GetWebhook("wh-del")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrWebhookNotFound))

	// delete non-existent
	err = d.DeleteWebhook("wh-none")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrWebhookNotFound))
}

func TestDeadLetter(t *testing.T) {
	d := NewDispatcher(DefaultDispatcherConfig())

	dead := d.GetDeadLetter(10)
	assert.Empty(t, dead)

	results := d.RetryDeadLetter()
	assert.Empty(t, results)
}

func TestStats(t *testing.T) {
	d := NewDispatcher(DefaultDispatcherConfig())

	wh1 := WebhookConfig{ID: "wh-a", Active: true, Events: []EventType{EventFeatureCreated}}
	wh2 := WebhookConfig{ID: "wh-b", Active: false}
	require.NoError(t, d.RegisterWebhook(wh1))
	require.NoError(t, d.RegisterWebhook(wh2))

	d.Dispatch(Event{ID: "e1", Type: EventFeatureCreated, Timestamp: time.Now()})

	stats := d.Stats()
	assert.Equal(t, 2, stats.TotalWebhooks)
	assert.Equal(t, 1, stats.ActiveWebhooks)
	assert.Equal(t, int64(1), stats.TotalDispatched)
	assert.Equal(t, int64(0), stats.TotalFailed)
	assert.Equal(t, 0, stats.DeadLetterSize)
}

func TestDispatchHTTPDelivery(t *testing.T) {
	var received Event
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, http.MethodPost, r.Method)
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := DefaultDispatcherConfig()
	cfg.AllowedCIDRs = []string{"127.0.0.0/8"} // Allow localhost for testing
	d := NewDispatcher(cfg)
	require.NoError(t, d.RegisterWebhook(WebhookConfig{
		ID:     "wh-http",
		Name:   "HTTP Hook",
		URL:    ts.URL,
		Events: []EventType{EventFeatureUpdated},
		Active: true,
	}))

	event := Event{
		ID:        "evt-http",
		Type:      EventFeatureUpdated,
		Source:    "test",
		Data:      map[string]interface{}{"key": "value"},
		Timestamp: time.Now(),
	}
	results := d.Dispatch(event)
	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, 200, results[0].StatusCode)
	assert.Equal(t, "evt-http", received.ID)
}

func TestDispatchHTTPFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cfg := DefaultDispatcherConfig()
	cfg.AllowedCIDRs = []string{"127.0.0.0/8"} // Allow localhost for testing
	d := NewDispatcher(cfg)
	require.NoError(t, d.RegisterWebhook(WebhookConfig{
		ID:     "wh-fail",
		Name:   "Failing Hook",
		URL:    ts.URL,
		Events: []EventType{EventDriftDetected},
		Active: true,
	}))

	results := d.Dispatch(Event{ID: "evt-fail", Type: EventDriftDetected, Timestamp: time.Now()})
	require.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.Equal(t, 500, results[0].StatusCode)
	assert.NotEmpty(t, results[0].Error)

	dead := d.GetDeadLetter(10)
	assert.Len(t, dead, 1)
}
