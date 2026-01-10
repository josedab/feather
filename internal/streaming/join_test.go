package streaming

import (
	"context"
	"testing"
	"time"
)

func TestJoinOperatorInner(t *testing.T) {
	config := JoinConfig{
		Name:           "user_order_join",
		Type:           JoinTypeInner,
		LeftStream:     "users",
		RightStream:    "orders",
		JoinKey:        "user_id",
		WindowDuration: 1 * time.Minute,
		GracePeriod:    5 * time.Second,
	}

	join := NewJoinOperator(config)
	join.Start()
	defer join.Stop()

	ctx := context.Background()

	// Send left event (user)
	userEvent := &Event{
		ID:        "u1",
		Type:      "users",
		EntityID:  "user:123",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"user_id": "123",
			"name":    "John",
		},
	}

	if err := join.ProcessEvent(ctx, userEvent); err != nil {
		t.Fatalf("process user event: %v", err)
	}

	// Send right event (order)
	orderEvent := &Event{
		ID:        "o1",
		Type:      "orders",
		EntityID:  "order:456",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"user_id": "123",
			"amount":  99.99,
		},
	}

	if err := join.ProcessEvent(ctx, orderEvent); err != nil {
		t.Fatalf("process order event: %v", err)
	}

	// Check for join result
	select {
	case result := <-join.GetOutputChannel():
		if result.JoinKey != "123" {
			t.Errorf("expected join key 123, got %s", result.JoinKey)
		}
		if result.LeftEvent == nil || result.RightEvent == nil {
			t.Error("expected both events in join result")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected join result within timeout")
	}

	// Verify metrics
	metrics := join.GetMetrics()
	if metrics.LeftEvents != 1 {
		t.Errorf("expected 1 left event, got %d", metrics.LeftEvents)
	}
	if metrics.RightEvents != 1 {
		t.Errorf("expected 1 right event, got %d", metrics.RightEvents)
	}
	if metrics.JoinMatches != 1 {
		t.Errorf("expected 1 join match, got %d", metrics.JoinMatches)
	}
}

func TestJoinOperatorLeftJoin(t *testing.T) {
	config := JoinConfig{
		Name:           "left_join",
		Type:           JoinTypeLeft,
		LeftStream:     "users",
		RightStream:    "orders",
		JoinKey:        "user_id",
		WindowDuration: 100 * time.Millisecond,
		GracePeriod:    10 * time.Millisecond,
	}

	join := NewJoinOperator(config)
	join.Start()
	defer join.Stop()

	ctx := context.Background()

	// Send left event without matching right
	userEvent := &Event{
		ID:        "u1",
		Type:      "users",
		EntityID:  "user:999",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"user_id": "999",
			"name":    "NoOrders",
		},
	}

	if err := join.ProcessEvent(ctx, userEvent); err != nil {
		t.Fatalf("process event: %v", err)
	}

	// Initially no result (no match)
	select {
	case <-join.GetOutputChannel():
		t.Error("unexpected immediate result for left join")
	case <-time.After(50 * time.Millisecond):
		// Expected - no immediate result
	}
}

func TestJoinOperatorEventTime(t *testing.T) {
	config := JoinConfig{
		Name:           "event_time_join",
		Type:           JoinTypeInner,
		LeftStream:     "clicks",
		RightStream:    "purchases",
		JoinKey:        "session_id",
		WindowDuration: 30 * time.Second,
		GracePeriod:    5 * time.Second,
		TimestampField: "event_time",
	}

	join := NewJoinOperator(config)
	join.Start()
	defer join.Stop()

	ctx := context.Background()
	baseTime := time.Now()

	// Click event at t=0
	clickEvent := &Event{
		ID:        "c1",
		Type:      "clicks",
		EntityID:  "session:abc",
		Timestamp: baseTime,
		Data: map[string]interface{}{
			"session_id": "abc",
			"product_id": "prod1",
			"event_time": baseTime.Format(time.RFC3339),
		},
	}

	if err := join.ProcessEvent(ctx, clickEvent); err != nil {
		t.Fatalf("process click: %v", err)
	}

	// Purchase event at t=10s (within window)
	purchaseEvent := &Event{
		ID:        "p1",
		Type:      "purchases",
		EntityID:  "session:abc",
		Timestamp: baseTime.Add(10 * time.Second),
		Data: map[string]interface{}{
			"session_id": "abc",
			"amount":     49.99,
			"event_time": baseTime.Add(10 * time.Second).Format(time.RFC3339),
		},
	}

	if err := join.ProcessEvent(ctx, purchaseEvent); err != nil {
		t.Fatalf("process purchase: %v", err)
	}

	// Should get a join result
	select {
	case result := <-join.GetOutputChannel():
		if result.JoinKey != "abc" {
			t.Errorf("expected join key abc, got %s", result.JoinKey)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected join result")
	}
}

func TestJoinOperatorOutputFields(t *testing.T) {
	config := JoinConfig{
		Name:           "output_fields_join",
		Type:           JoinTypeInner,
		LeftStream:     "users",
		RightStream:    "orders",
		JoinKey:        "user_id",
		WindowDuration: 1 * time.Minute,
		OutputFields: []JoinOutputField{
			{Source: "left", Field: "name", Alias: "user_name"},
			{Source: "right", Field: "amount", Alias: "order_amount"},
		},
	}

	join := NewJoinOperator(config)
	join.Start()
	defer join.Stop()

	ctx := context.Background()

	userEvent := &Event{
		ID:   "u1",
		Type: "users",
		Data: map[string]interface{}{
			"user_id": "123",
			"name":    "Alice",
			"email":   "alice@example.com",
		},
	}
	join.ProcessEvent(ctx, userEvent)

	orderEvent := &Event{
		ID:   "o1",
		Type: "orders",
		Data: map[string]interface{}{
			"user_id": "123",
			"amount":  150.0,
			"status":  "pending",
		},
	}
	join.ProcessEvent(ctx, orderEvent)

	select {
	case result := <-join.GetOutputChannel():
		// Should only have configured fields
		if _, ok := result.OutputData["user_name"]; !ok {
			t.Error("expected user_name in output")
		}
		if _, ok := result.OutputData["order_amount"]; !ok {
			t.Error("expected order_amount in output")
		}
		if _, ok := result.OutputData["email"]; ok {
			t.Error("email should not be in output")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected join result")
	}
}

func TestMultiJoinPipeline(t *testing.T) {
	pipeline := NewMultiJoinPipeline()
	defer pipeline.Stop()

	// Add two joins
	err := pipeline.AddJoin(JoinConfig{
		Name:           "user_order",
		Type:           JoinTypeInner,
		LeftStream:     "users",
		RightStream:    "orders",
		JoinKey:        "user_id",
		WindowDuration: 1 * time.Minute,
	})
	if err != nil {
		t.Fatalf("add first join: %v", err)
	}

	err = pipeline.AddJoin(JoinConfig{
		Name:           "order_payment",
		Type:           JoinTypeInner,
		LeftStream:     "orders",
		RightStream:    "payments",
		JoinKey:        "order_id",
		WindowDuration: 1 * time.Minute,
	})
	if err != nil {
		t.Fatalf("add second join: %v", err)
	}

	ctx := context.Background()

	// Process events
	pipeline.ProcessEvent(ctx, &Event{
		Type: "users",
		Data: map[string]interface{}{"user_id": "u1", "name": "Test"},
	})
	pipeline.ProcessEvent(ctx, &Event{
		Type: "orders",
		Data: map[string]interface{}{"user_id": "u1", "order_id": "o1", "amount": 100},
	})

	// Should get user-order join
	select {
	case result := <-pipeline.GetOutputChannel():
		if result.JoinName != "user_order" {
			t.Errorf("expected user_order join, got %s", result.JoinName)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected join result")
	}

	// Check metrics
	allMetrics := pipeline.GetAllMetrics()
	if len(allMetrics) != 2 {
		t.Errorf("expected 2 joins in metrics, got %d", len(allMetrics))
	}
}

func BenchmarkJoinOperator(b *testing.B) {
	config := JoinConfig{
		Name:           "bench_join",
		Type:           JoinTypeInner,
		LeftStream:     "left",
		RightStream:    "right",
		JoinKey:        "id",
		WindowDuration: 1 * time.Minute,
	}

	join := NewJoinOperator(config)
	join.Start()
	defer join.Stop()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		join.ProcessEvent(ctx, &Event{
			Type: "left",
			Data: map[string]interface{}{"id": "key1"},
		})
		join.ProcessEvent(ctx, &Event{
			Type: "right",
			Data: map[string]interface{}{"id": "key1"},
		})
	}
}
