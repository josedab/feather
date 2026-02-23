package streamcompute

import (
	"testing"
	"time"
)

func TestTemporalJoin_InnerJoin(t *testing.T) {
	tj := NewTemporalJoin(TemporalJoinConfig{
		LeftStream:    "orders",
		RightStream:   "payments",
		JoinType:      JoinInner,
		TimeTolerance: 5 * time.Second,
	})

	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tj.AddLeft(Event{Key: "user1", Value: 100, Timestamp: base})
	tj.AddRight(Event{Key: "user1", Value: 100, Timestamp: base.Add(2 * time.Second)})

	results := tj.Match()
	if len(results) != 1 {
		t.Fatalf("expected 1 match, got %d", len(results))
	}
	if results[0].Left == nil || results[0].Right == nil {
		t.Fatal("expected both left and right events")
	}
}

func TestTemporalJoin_NoMatchOutsideTolerance(t *testing.T) {
	tj := NewTemporalJoin(TemporalJoinConfig{
		LeftStream:    "orders",
		RightStream:   "payments",
		JoinType:      JoinInner,
		TimeTolerance: 2 * time.Second,
	})

	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tj.AddLeft(Event{Key: "user1", Value: 100, Timestamp: base})
	tj.AddRight(Event{Key: "user1", Value: 100, Timestamp: base.Add(10 * time.Second)})

	results := tj.Match()
	if len(results) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(results))
	}
}

func TestTemporalJoin_DifferentKeys(t *testing.T) {
	tj := NewTemporalJoin(TemporalJoinConfig{
		LeftStream:    "orders",
		RightStream:   "payments",
		JoinType:      JoinInner,
		TimeTolerance: 5 * time.Second,
	})

	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tj.AddLeft(Event{Key: "user1", Value: 100, Timestamp: base})
	tj.AddRight(Event{Key: "user2", Value: 100, Timestamp: base.Add(1 * time.Second)})

	results := tj.Match()
	if len(results) != 0 {
		t.Fatalf("expected 0 matches for different keys, got %d", len(results))
	}
}

func TestTemporalJoin_LeftJoin(t *testing.T) {
	tj := NewTemporalJoin(TemporalJoinConfig{
		LeftStream:    "orders",
		RightStream:   "payments",
		JoinType:      JoinLeft,
		TimeTolerance: 5 * time.Second,
	})

	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tj.AddLeft(Event{Key: "user1", Value: 100, Timestamp: base})
	tj.AddLeft(Event{Key: "user2", Value: 200, Timestamp: base})
	tj.AddRight(Event{Key: "user1", Value: 100, Timestamp: base.Add(1 * time.Second)})

	results := tj.Match()
	if len(results) != 2 {
		t.Fatalf("expected 2 results (1 match + 1 unmatched left), got %d", len(results))
	}

	// One should have both, one should have only left
	var hasMatch, hasLeftOnly bool
	for _, r := range results {
		if r.Left != nil && r.Right != nil {
			hasMatch = true
		}
		if r.Left != nil && r.Right == nil {
			hasLeftOnly = true
		}
	}
	if !hasMatch {
		t.Error("expected at least one matched pair")
	}
	if !hasLeftOnly {
		t.Error("expected at least one unmatched left event")
	}
}

func TestTemporalJoin_RightJoin(t *testing.T) {
	tj := NewTemporalJoin(TemporalJoinConfig{
		LeftStream:    "orders",
		RightStream:   "payments",
		JoinType:      JoinRight,
		TimeTolerance: 5 * time.Second,
	})

	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tj.AddLeft(Event{Key: "user1", Value: 100, Timestamp: base})
	tj.AddRight(Event{Key: "user1", Value: 100, Timestamp: base.Add(1 * time.Second)})
	tj.AddRight(Event{Key: "user3", Value: 300, Timestamp: base})

	results := tj.Match()
	if len(results) != 2 {
		t.Fatalf("expected 2 results (1 match + 1 unmatched right), got %d", len(results))
	}
}

func TestTemporalJoin_JoinKeyFromFields(t *testing.T) {
	tj := NewTemporalJoin(TemporalJoinConfig{
		LeftStream:    "orders",
		RightStream:   "payments",
		JoinType:      JoinInner,
		JoinKey:       "order_id",
		TimeTolerance: 5 * time.Second,
	})

	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tj.AddLeft(Event{
		Key: "user1", Value: 100, Timestamp: base,
		Fields: map[string]interface{}{"order_id": "ord-123"},
	})
	tj.AddRight(Event{
		Key: "user2", Value: 100, Timestamp: base.Add(1 * time.Second),
		Fields: map[string]interface{}{"order_id": "ord-123"},
	})

	results := tj.Match()
	if len(results) != 1 {
		t.Fatalf("expected 1 match on join key, got %d", len(results))
	}
	if results[0].JoinKey != "ord-123" {
		t.Errorf("expected join key 'ord-123', got %q", results[0].JoinKey)
	}
}

func TestTemporalJoin_Stats(t *testing.T) {
	tj := NewTemporalJoin(TemporalJoinConfig{
		LeftStream:    "a",
		RightStream:   "b",
		JoinType:      JoinInner,
		TimeTolerance: 5 * time.Second,
	})

	base := time.Now()
	tj.AddLeft(Event{Key: "k", Value: 1, Timestamp: base})
	tj.AddRight(Event{Key: "k", Value: 2, Timestamp: base})
	tj.Match()

	stats := tj.Stats()
	if stats.LeftEvents != 1 {
		t.Errorf("expected 1 left event, got %d", stats.LeftEvents)
	}
	if stats.RightEvents != 1 {
		t.Errorf("expected 1 right event, got %d", stats.RightEvents)
	}
	if stats.MatchedPairs != 1 {
		t.Errorf("expected 1 matched pair, got %d", stats.MatchedPairs)
	}
}

func TestTemporalJoin_BufferLimit(t *testing.T) {
	tj := NewTemporalJoin(TemporalJoinConfig{
		LeftStream:    "a",
		RightStream:   "b",
		JoinType:      JoinInner,
		TimeTolerance: 5 * time.Second,
		MaxBufferSize: 3,
	})

	base := time.Now()
	for i := 0; i < 5; i++ {
		tj.AddLeft(Event{Key: "k", Value: float64(i), Timestamp: base})
	}

	tj.mu.Lock()
	bufLen := len(tj.leftBuffer)
	tj.mu.Unlock()

	if bufLen > 3 {
		t.Errorf("expected buffer size <= 3, got %d", bufLen)
	}
}
