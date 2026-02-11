package graphql

import (
	"testing"
)

func TestSubscribeUnsubscribe(t *testing.T) {
	m := NewSubscriptionManager(DefaultSubscriptionConfig())

	sub, err := m.Subscribe("client1", "{ features }", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.ID == "" {
		t.Fatal("expected subscription ID")
	}
	if sub.ClientID != "client1" {
		t.Fatalf("expected client1, got %s", sub.ClientID)
	}

	subs := m.ListSubscriptions("client1")
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}

	if err := m.Unsubscribe(sub.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	subs = m.ListSubscriptions("client1")
	if len(subs) != 0 {
		t.Fatalf("expected 0 subscriptions, got %d", len(subs))
	}
}

func TestUnsubscribeNotFound(t *testing.T) {
	m := NewSubscriptionManager(DefaultSubscriptionConfig())
	if err := m.Unsubscribe("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent subscription")
	}
}

func TestPublishAndReceive(t *testing.T) {
	m := NewSubscriptionManager(DefaultSubscriptionConfig())

	filter := &SubscriptionFilter{
		EntityKeys: []string{"user:123"},
		EventTypes: []string{"updated"},
	}
	sub, err := m.Subscribe("client1", "{ feature }", nil, filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m.Publish("updated", "user:123", "user_features", map[string]interface{}{"value": 42})

	events := m.GetEvents(sub.ID, 10)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "updated" {
		t.Fatalf("expected event type updated, got %s", events[0].Type)
	}
}

func TestFilterMatchingLogic(t *testing.T) {
	m := NewSubscriptionManager(DefaultSubscriptionConfig())

	// Subscription with entity key filter only
	filter := &SubscriptionFilter{EntityKeys: []string{"user:1"}}
	sub, _ := m.Subscribe("c1", "q", nil, filter)

	// Matching entity key
	m.Publish("created", "user:1", "group1", "data1")
	// Non-matching entity key
	m.Publish("created", "user:2", "group1", "data2")

	events := m.GetEvents(sub.ID, 10)
	if len(events) != 1 {
		t.Fatalf("expected 1 event (filter by entity), got %d", len(events))
	}

	// Subscription with feature group filter
	filter2 := &SubscriptionFilter{FeatureGroups: []string{"payments"}}
	sub2, _ := m.Subscribe("c1", "q", nil, filter2)

	m.Publish("updated", "user:1", "payments", "pay1")
	m.Publish("updated", "user:1", "orders", "ord1")

	events2 := m.GetEvents(sub2.ID, 10)
	if len(events2) != 1 {
		t.Fatalf("expected 1 event (filter by group), got %d", len(events2))
	}

	// Subscription with event type filter
	filter3 := &SubscriptionFilter{EventTypes: []string{"deleted"}}
	sub3, _ := m.Subscribe("c1", "q", nil, filter3)

	m.Publish("deleted", "user:1", "group1", "del1")
	m.Publish("created", "user:1", "group1", "cre1")

	events3 := m.GetEvents(sub3.ID, 10)
	if len(events3) != 1 {
		t.Fatalf("expected 1 event (filter by type), got %d", len(events3))
	}

	// Nil filter matches everything
	sub4, _ := m.Subscribe("c2", "q", nil, nil)
	m.Publish("updated", "any", "any", "any")
	events4 := m.GetEvents(sub4.ID, 10)
	if len(events4) != 1 {
		t.Fatalf("expected 1 event (nil filter), got %d", len(events4))
	}
}

func TestMultipleSubscribersReceiveSameEvent(t *testing.T) {
	m := NewSubscriptionManager(DefaultSubscriptionConfig())

	sub1, _ := m.Subscribe("c1", "q", nil, nil)
	sub2, _ := m.Subscribe("c2", "q", nil, nil)

	m.Publish("updated", "user:1", "group1", "shared_data")

	e1 := m.GetEvents(sub1.ID, 10)
	e2 := m.GetEvents(sub2.ID, 10)

	if len(e1) != 1 || len(e2) != 1 {
		t.Fatalf("expected both subscribers to receive 1 event, got %d and %d", len(e1), len(e2))
	}
}

func TestBufferOverflow(t *testing.T) {
	cfg := DefaultSubscriptionConfig()
	cfg.BufferSize = 3
	m := NewSubscriptionManager(cfg)

	sub, _ := m.Subscribe("c1", "q", nil, nil)

	for i := 0; i < 5; i++ {
		m.Publish("updated", "user:1", "group1", i)
	}

	events := m.GetEvents(sub.ID, 10)
	if len(events) != 3 {
		t.Fatalf("expected 3 buffered events after overflow, got %d", len(events))
	}
	// Should have the most recent events (2, 3, 4)
	if events[0].Data != 2 || events[1].Data != 3 || events[2].Data != 4 {
		t.Fatalf("expected most recent events [2,3,4], got [%v,%v,%v]", events[0].Data, events[1].Data, events[2].Data)
	}
}

func TestStatsTracking(t *testing.T) {
	m := NewSubscriptionManager(DefaultSubscriptionConfig())

	m.Subscribe("c1", "q", nil, nil)
	m.Subscribe("c2", "q", nil, nil)
	m.Subscribe("c1", "q2", nil, nil)

	m.Publish("updated", "u1", "g1", "data")

	stats := m.Stats()
	if stats.TotalSubscriptions != 3 {
		t.Fatalf("expected 3 subscriptions, got %d", stats.TotalSubscriptions)
	}
	if stats.ActiveClients != 2 {
		t.Fatalf("expected 2 active clients, got %d", stats.ActiveClients)
	}
	if stats.TotalEventsPublished != 1 {
		t.Fatalf("expected 1 event published, got %d", stats.TotalEventsPublished)
	}
	if stats.TotalEventsDelivered != 3 {
		t.Fatalf("expected 3 events delivered, got %d", stats.TotalEventsDelivered)
	}
}

func TestMaxSubscriptionsLimit(t *testing.T) {
	cfg := DefaultSubscriptionConfig()
	cfg.MaxSubscriptions = 2
	m := NewSubscriptionManager(cfg)

	m.Subscribe("c1", "q", nil, nil)
	m.Subscribe("c2", "q", nil, nil)

	_, err := m.Subscribe("c3", "q", nil, nil)
	if err == nil {
		t.Fatal("expected error when exceeding max subscriptions")
	}
}

func TestMaxConnectionsPerClientLimit(t *testing.T) {
	cfg := DefaultSubscriptionConfig()
	cfg.MaxConnectionsPerClient = 2
	m := NewSubscriptionManager(cfg)

	m.Subscribe("c1", "q1", nil, nil)
	m.Subscribe("c1", "q2", nil, nil)

	_, err := m.Subscribe("c1", "q3", nil, nil)
	if err == nil {
		t.Fatal("expected error when exceeding max connections per client")
	}
}

func TestGetSubscription(t *testing.T) {
	m := NewSubscriptionManager(DefaultSubscriptionConfig())

	sub, _ := m.Subscribe("c1", "{ features }", nil, nil)

	got, err := m.GetSubscription(sub.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != sub.ID {
		t.Fatalf("expected %s, got %s", sub.ID, got.ID)
	}

	_, err = m.GetSubscription("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent subscription")
	}
}

func TestPubSubBasic(t *testing.T) {
	ps := NewPubSub(10)
	defer ps.Close()

	ch := ps.Subscribe("topic1")

	event := &SubscriptionEvent{ID: "e1", Type: "test"}
	ps.Publish("topic1", event)

	select {
	case got := <-ch:
		if got.ID != "e1" {
			t.Fatalf("expected e1, got %s", got.ID)
		}
	default:
		t.Fatal("expected to receive event")
	}
}

func TestPubSubUnsubscribe(t *testing.T) {
	ps := NewPubSub(10)
	defer ps.Close()

	ch := ps.Subscribe("topic1")
	ps.Unsubscribe("topic1", ch)

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed")
	}
}

func TestGetEventsLimit(t *testing.T) {
	m := NewSubscriptionManager(DefaultSubscriptionConfig())
	sub, _ := m.Subscribe("c1", "q", nil, nil)

	for i := 0; i < 10; i++ {
		m.Publish("updated", "u1", "g1", i)
	}

	events := m.GetEvents(sub.ID, 3)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	// Should return the most recent 3
	if events[0].Data != 7 || events[1].Data != 8 || events[2].Data != 9 {
		t.Fatalf("expected [7,8,9], got [%v,%v,%v]", events[0].Data, events[1].Data, events[2].Data)
	}
}
