package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/core/logging"
)

// StreamEvent represents a streaming event for pub/sub.
type StreamEvent struct {
	Type      StreamEventType        `json:"type"`
	EntityID  string                 `json:"entity_id,omitempty"`
	Feature   string                 `json:"feature,omitempty"`
	Value     interface{}            `json:"value,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// StreamEventType represents the type of streaming event.
type StreamEventType string

const (
	// StreamEventFeatureUpdate indicates a feature update event.
	StreamEventFeatureUpdate StreamEventType = "feature_update"
	// StreamEventFeatureDelete indicates a feature deletion event.
	StreamEventFeatureDelete StreamEventType = "feature_delete"
	// StreamEventSchemaChange indicates a schema change event.
	StreamEventSchemaChange StreamEventType = "schema_change"
	// StreamEventAlert indicates an alert event.
	StreamEventAlert StreamEventType = "alert"
	// StreamEventDriftDetected indicates drift detection.
	StreamEventDriftDetected StreamEventType = "drift_detected"
	// StreamEventHeartbeat indicates a heartbeat event.
	StreamEventHeartbeat StreamEventType = "heartbeat"
)

// Subscription represents a client subscription.
type Subscription struct {
	ID         string
	ClientID   string
	Features   []string          // Empty means all features
	EntityIDs  []string          // Empty means all entities
	EventTypes []StreamEventType // Empty means all event types
	Filters    map[string]string // Additional filters
	CreatedAt  time.Time
}

// Hub manages streaming subscriptions and event distribution.
type Hub struct {
	subscriptions map[string]*subscriberInfo
	byFeature     map[string]map[string]bool // feature -> subscription IDs
	byEntity      map[string]map[string]bool // entity -> subscription IDs
	byEventType   map[StreamEventType]map[string]bool
	eventCh       chan StreamEvent
	mu            sync.RWMutex
	stopCh        chan struct{}
	wg            sync.WaitGroup
	ctx           context.Context

	// Metrics for monitoring
	droppedHubEvents        int64 // Events dropped because hub channel full
	droppedSubscriberEvents int64 // Events dropped because subscriber channel full
}

type subscriberInfo struct {
	subscription *Subscription
	eventCh      chan StreamEvent
	done         chan struct{}
}

// NewHub creates a new streaming hub.
func NewHub(ctx context.Context) *Hub { //nolint:contextcheck
	if ctx == nil {
		ctx = context.Background()
	}
	h := &Hub{
		subscriptions: make(map[string]*subscriberInfo),
		byFeature:     make(map[string]map[string]bool),
		byEntity:      make(map[string]map[string]bool),
		byEventType:   make(map[StreamEventType]map[string]bool),
		eventCh:       make(chan StreamEvent, 10000),
		stopCh:        make(chan struct{}),
		ctx:           ctx,
	}

	h.wg.Add(1)
	go h.distribute()

	return h
}

// Subscribe creates a new subscription.
func (h *Hub) Subscribe(sub *Subscription) (<-chan StreamEvent, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	info := &subscriberInfo{
		subscription: sub,
		eventCh:      make(chan StreamEvent, 1000),
		done:         make(chan struct{}),
	}

	h.subscriptions[sub.ID] = info

	// Index by features
	if len(sub.Features) == 0 {
		if h.byFeature["*"] == nil {
			h.byFeature["*"] = make(map[string]bool)
		}
		h.byFeature["*"][sub.ID] = true
	} else {
		for _, f := range sub.Features {
			if h.byFeature[f] == nil {
				h.byFeature[f] = make(map[string]bool)
			}
			h.byFeature[f][sub.ID] = true
		}
	}

	// Index by entities
	if len(sub.EntityIDs) == 0 {
		if h.byEntity["*"] == nil {
			h.byEntity["*"] = make(map[string]bool)
		}
		h.byEntity["*"][sub.ID] = true
	} else {
		for _, e := range sub.EntityIDs {
			if h.byEntity[e] == nil {
				h.byEntity[e] = make(map[string]bool)
			}
			h.byEntity[e][sub.ID] = true
		}
	}

	// Index by event types
	if len(sub.EventTypes) == 0 {
		for _, et := range []StreamEventType{StreamEventFeatureUpdate, StreamEventFeatureDelete, StreamEventSchemaChange, StreamEventAlert, StreamEventDriftDetected} {
			if h.byEventType[et] == nil {
				h.byEventType[et] = make(map[string]bool)
			}
			h.byEventType[et][sub.ID] = true
		}
	} else {
		for _, et := range sub.EventTypes {
			if h.byEventType[et] == nil {
				h.byEventType[et] = make(map[string]bool)
			}
			h.byEventType[et][sub.ID] = true
		}
	}

	// Unsubscribe function
	unsubscribe := func() {
		h.Unsubscribe(sub.ID)
	}

	return info.eventCh, unsubscribe
}

// Unsubscribe removes a subscription.
func (h *Hub) Unsubscribe(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	info, ok := h.subscriptions[id]
	if !ok {
		return
	}

	close(info.done)
	delete(h.subscriptions, id)

	// Clean up indexes
	sub := info.subscription

	if len(sub.Features) == 0 {
		delete(h.byFeature["*"], id)
	} else {
		for _, f := range sub.Features {
			delete(h.byFeature[f], id)
		}
	}

	if len(sub.EntityIDs) == 0 {
		delete(h.byEntity["*"], id)
	} else {
		for _, e := range sub.EntityIDs {
			delete(h.byEntity[e], id)
		}
	}

	for et := range h.byEventType {
		delete(h.byEventType[et], id)
	}
}

// Publish publishes an event to matching subscribers.
func (h *Hub) Publish(ctx context.Context, event StreamEvent) error {
	event.Timestamp = time.Now()

	select {
	case h.eventCh <- event:
		return nil
	default:
		// Channel full, drop event and track metric
		dropped := atomic.AddInt64(&h.droppedHubEvents, 1)
		if dropped%1000 == 1 { // Log every 1000th drop to avoid log spam
			logging.FromContext(ctx, nil).Warn("streaming hub channel full, dropping events",
				"total_dropped", dropped,
				"event_type", event.Type,
			)
		}
		return fmt.Errorf("streaming hub channel full")
	}
}

// PublishFeatureUpdate publishes a feature update event.
func (h *Hub) PublishFeatureUpdate(entityID, feature string, value interface{}) {
	_ = h.Publish(context.Background(), StreamEvent{
		Type:     StreamEventFeatureUpdate,
		EntityID: entityID,
		Feature:  feature,
		Value:    value,
	})
}

func (h *Hub) distribute() {
	defer h.wg.Done()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-h.stopCh:
			return
		case event := <-h.eventCh:
			h.distributeEvent(event)
		}
	}
}

func (h *Hub) distributeEvent(event StreamEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Find matching subscriptions
	matching := make(map[string]bool)

	// Match by event type
	for id := range h.byEventType[event.Type] {
		matching[id] = true
	}

	// Filter by feature
	if event.Feature != "" {
		featureMatches := make(map[string]bool)
		for id := range h.byFeature[event.Feature] {
			featureMatches[id] = true
		}
		for id := range h.byFeature["*"] {
			featureMatches[id] = true
		}

		// Intersect
		for id := range matching {
			if !featureMatches[id] {
				delete(matching, id)
			}
		}
	}

	// Filter by entity
	if event.EntityID != "" {
		entityMatches := make(map[string]bool)
		for id := range h.byEntity[event.EntityID] {
			entityMatches[id] = true
		}
		for id := range h.byEntity["*"] {
			entityMatches[id] = true
		}

		// Intersect
		for id := range matching {
			if !entityMatches[id] {
				delete(matching, id)
			}
		}
	}

	// Send to matching subscribers
	for id := range matching {
		if info, ok := h.subscriptions[id]; ok {
			select {
			case info.eventCh <- event:
			default:
				// Subscriber channel full, track metric
				dropped := atomic.AddInt64(&h.droppedSubscriberEvents, 1)
				if dropped%1000 == 1 { // Log every 1000th drop to avoid log spam
					logging.FromContext(h.ctx, nil).Warn("subscriber channel full, dropping event",
						"total_dropped", dropped,
						"subscription_id", id,
						"event_type", event.Type,
					)
				}
			}
		}
	}
}

// GetSubscriptions returns all active subscriptions.
func (h *Hub) GetSubscriptions() []*Subscription {
	h.mu.RLock()
	defer h.mu.RUnlock()

	subs := make([]*Subscription, 0, len(h.subscriptions))
	for _, info := range h.subscriptions {
		subs = append(subs, info.subscription)
	}
	return subs
}

// Stats returns hub statistics.
func (h *Hub) Stats() HubStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return HubStats{
		ActiveSubscriptions:     len(h.subscriptions),
		QueuedEvents:            len(h.eventCh),
		DroppedHubEvents:        atomic.LoadInt64(&h.droppedHubEvents),
		DroppedSubscriberEvents: atomic.LoadInt64(&h.droppedSubscriberEvents),
	}
}

// HubStats contains hub statistics.
type HubStats struct {
	ActiveSubscriptions     int   `json:"active_subscriptions"`
	QueuedEvents            int   `json:"queued_events"`
	DroppedHubEvents        int64 `json:"dropped_hub_events"`
	DroppedSubscriberEvents int64 `json:"dropped_subscriber_events"`
}

// Stop stops the hub.
func (h *Hub) Stop() {
	close(h.stopCh)
	h.wg.Wait()
}

// SSEWriter provides Server-Sent Events formatting.
type SSEWriter struct {
	events <-chan StreamEvent
	done   chan struct{}
}

// NewSSEWriter creates a new SSE writer.
func NewSSEWriter(events <-chan StreamEvent) *SSEWriter {
	return &SSEWriter{
		events: events,
		done:   make(chan struct{}),
	}
}

// WriteTo writes events as SSE format.
func (w *SSEWriter) WriteTo(ctx context.Context, writer func(data []byte) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.done:
			return nil
		case event, ok := <-w.events:
			if !ok {
				return nil
			}

			data, err := json.Marshal(event)
			if err != nil {
				continue
			}

			sseData := []byte("event: " + string(event.Type) + "\ndata: " + string(data) + "\n\n")
			if err := writer(sseData); err != nil {
				return err
			}
		}
	}
}

// Stop stops the SSE writer.
func (w *SSEWriter) Stop() {
	close(w.done)
}

// WebSocketMessage represents a WebSocket message.
type WebSocketMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// SubscribeMessage represents a subscribe message.
type SubscribeMessage struct {
	Features   []string          `json:"features,omitempty"`
	EntityIDs  []string          `json:"entity_ids,omitempty"`
	EventTypes []StreamEventType `json:"event_types,omitempty"`
}
