package graphql

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// SubscriptionConfig holds configuration for the subscription manager.
type SubscriptionConfig struct {
	MaxSubscriptions        int
	MaxConnectionsPerClient int
	HeartbeatInterval       time.Duration
	BufferSize              int
	EnableCompression       bool
}

// DefaultSubscriptionConfig returns sensible default subscription settings.
func DefaultSubscriptionConfig() SubscriptionConfig {
	return SubscriptionConfig{
		MaxSubscriptions:        10000,
		MaxConnectionsPerClient: 10,
		HeartbeatInterval:       30 * time.Second,
		BufferSize:              100,
		EnableCompression:       false,
	}
}

// Subscription represents an active subscription.
type Subscription struct {
	ID          string                 `json:"id"`
	ClientID    string                 `json:"client_id"`
	Query       string                 `json:"query"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	Filter      *SubscriptionFilter    `json:"filter,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	LastEventAt time.Time              `json:"last_event_at"`
	EventCount  int64                  `json:"event_count"`
}

// SubscriptionFilter specifies which events a subscription receives.
type SubscriptionFilter struct {
	EntityKeys    []string `json:"entity_keys,omitempty"`
	FeatureGroups []string `json:"feature_groups,omitempty"`
	EventTypes    []string `json:"event_types,omitempty"`
}

// SubscriptionEvent represents an event delivered to subscribers.
type SubscriptionEvent struct {
	ID             string      `json:"id"`
	Type           string      `json:"type"`
	Timestamp      time.Time   `json:"timestamp"`
	Data           interface{} `json:"data"`
	SubscriptionID string      `json:"subscription_id"`
}

// SubscriptionStats provides runtime statistics about the subscription system.
type SubscriptionStats struct {
	TotalSubscriptions   int     `json:"total_subscriptions"`
	ActiveClients        int     `json:"active_clients"`
	TotalEventsPublished int64   `json:"total_events_published"`
	TotalEventsDelivered int64   `json:"total_events_delivered"`
	BufferUtilization    float64 `json:"buffer_utilization"`
}

// SubscriptionManager manages subscriptions and event delivery.
type SubscriptionManager struct {
	config        SubscriptionConfig
	mu            sync.RWMutex
	subscriptions map[string]*Subscription
	buffers       map[string][]*SubscriptionEvent
	pubsub        *PubSub
	nextID        atomic.Int64
	published     atomic.Int64
	delivered     atomic.Int64
}

// NewSubscriptionManager creates a new subscription manager.
func NewSubscriptionManager(config SubscriptionConfig) *SubscriptionManager {
	return &SubscriptionManager{
		config:        config,
		subscriptions: make(map[string]*Subscription),
		buffers:       make(map[string][]*SubscriptionEvent),
		pubsub:        NewPubSub(config.BufferSize),
	}
}

// Subscribe creates a new subscription.
func (m *SubscriptionManager) Subscribe(clientID, query string, variables map[string]interface{}, filter *SubscriptionFilter) (*Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.subscriptions) >= m.config.MaxSubscriptions {
		return nil, fmt.Errorf("maximum subscriptions reached (%d)", m.config.MaxSubscriptions)
	}

	clientCount := 0
	for _, sub := range m.subscriptions {
		if sub.ClientID == clientID {
			clientCount++
		}
	}
	if clientCount >= m.config.MaxConnectionsPerClient {
		return nil, fmt.Errorf("maximum subscriptions per client reached (%d)", m.config.MaxConnectionsPerClient)
	}

	id := fmt.Sprintf("sub_%d", m.nextID.Add(1))
	sub := &Subscription{
		ID:        id,
		ClientID:  clientID,
		Query:     query,
		Variables: variables,
		Filter:    filter,
		CreatedAt: time.Now(),
	}
	m.subscriptions[id] = sub
	m.buffers[id] = make([]*SubscriptionEvent, 0)
	return sub, nil
}

// Unsubscribe removes a subscription.
func (m *SubscriptionManager) Unsubscribe(subscriptionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.subscriptions[subscriptionID]; !ok {
		return fmt.Errorf("subscription %s not found", subscriptionID)
	}
	delete(m.subscriptions, subscriptionID)
	delete(m.buffers, subscriptionID)
	return nil
}

// Publish sends an event to all matching subscriptions.
func (m *SubscriptionManager) Publish(eventType string, entityKey string, featureGroup string, data interface{}) {
	m.published.Add(1)

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, sub := range m.subscriptions {
		if !m.matchesFilter(sub.Filter, eventType, entityKey, featureGroup) {
			continue
		}

		event := &SubscriptionEvent{
			ID:             fmt.Sprintf("evt_%d_%s", time.Now().UnixNano(), sub.ID),
			Type:           eventType,
			Timestamp:      time.Now(),
			Data:           data,
			SubscriptionID: sub.ID,
		}

		buf := m.buffers[sub.ID]
		if len(buf) >= m.config.BufferSize {
			// Drop oldest event on overflow
			buf = buf[1:]
		}
		m.buffers[sub.ID] = append(buf, event)
		sub.LastEventAt = event.Timestamp
		sub.EventCount++
		m.delivered.Add(1)
	}
}

// matchesFilter checks if an event matches a subscription's filter.
func (m *SubscriptionManager) matchesFilter(filter *SubscriptionFilter, eventType, entityKey, featureGroup string) bool {
	if filter == nil {
		return true
	}

	if len(filter.EventTypes) > 0 && !contains(filter.EventTypes, eventType) {
		return false
	}
	if len(filter.EntityKeys) > 0 && !contains(filter.EntityKeys, entityKey) {
		return false
	}
	if len(filter.FeatureGroups) > 0 && !contains(filter.FeatureGroups, featureGroup) {
		return false
	}
	return true
}

// GetEvents returns buffered events for a subscription.
func (m *SubscriptionManager) GetEvents(subscriptionID string, limit int) []*SubscriptionEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	buf, ok := m.buffers[subscriptionID]
	if !ok {
		return nil
	}

	if limit <= 0 || limit > len(buf) {
		limit = len(buf)
	}
	// Return most recent events
	start := len(buf) - limit
	result := make([]*SubscriptionEvent, limit)
	copy(result, buf[start:])
	return result
}

// ListSubscriptions returns subscriptions, optionally filtered by client ID.
func (m *SubscriptionManager) ListSubscriptions(clientID string) []*Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Subscription, 0)
	for _, sub := range m.subscriptions {
		if clientID == "" || sub.ClientID == clientID {
			result = append(result, sub)
		}
	}
	return result
}

// GetSubscription returns a single subscription by ID.
func (m *SubscriptionManager) GetSubscription(id string) (*Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sub, ok := m.subscriptions[id]
	if !ok {
		return nil, fmt.Errorf("subscription %s not found", id)
	}
	return sub, nil
}

// Stats returns current subscription statistics.
func (m *SubscriptionManager) Stats() *SubscriptionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clients := make(map[string]struct{})
	totalBuf := 0
	totalCap := len(m.subscriptions) * m.config.BufferSize

	for _, sub := range m.subscriptions {
		clients[sub.ClientID] = struct{}{}
		totalBuf += len(m.buffers[sub.ID])
	}

	var utilization float64
	if totalCap > 0 {
		utilization = float64(totalBuf) / float64(totalCap)
	}

	return &SubscriptionStats{
		TotalSubscriptions:   len(m.subscriptions),
		ActiveClients:        len(clients),
		TotalEventsPublished: m.published.Load(),
		TotalEventsDelivered: m.delivered.Load(),
		BufferUtilization:    utilization,
	}
}

// PubSub is an inner event bus for routing events to topic channels.
type PubSub struct {
	mu         sync.RWMutex
	topics     map[string][]chan *SubscriptionEvent
	bufferSize int
	closed     bool
}

// NewPubSub creates a new PubSub event bus.
func NewPubSub(bufferSize int) *PubSub {
	return &PubSub{
		topics:     make(map[string][]chan *SubscriptionEvent),
		bufferSize: bufferSize,
	}
}

// Subscribe returns a channel that receives events for the given topic.
func (ps *PubSub) Subscribe(topic string) <-chan *SubscriptionEvent {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ch := make(chan *SubscriptionEvent, ps.bufferSize)
	ps.topics[topic] = append(ps.topics[topic], ch)
	return ch
}

// Unsubscribe removes a channel from a topic.
func (ps *PubSub) Unsubscribe(topic string, ch <-chan *SubscriptionEvent) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	subs := ps.topics[topic]
	for i, s := range subs {
		if s == ch {
			ps.topics[topic] = append(subs[:i], subs[i+1:]...)
			close(s)
			break
		}
	}
}

// Publish sends an event to all subscribers of a topic.
func (ps *PubSub) Publish(topic string, event *SubscriptionEvent) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if ps.closed {
		return
	}

	for _, ch := range ps.topics[topic] {
		select {
		case ch <- event:
		default:
			// Drop event if buffer is full
		}
	}
}

// Close shuts down the PubSub and closes all channels.
func (ps *PubSub) Close() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.closed = true
	for topic, subs := range ps.topics {
		for _, ch := range subs {
			close(ch)
		}
		delete(ps.topics, topic)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
