package streamsql

import (
	"testing"
	"time"
)

func TestWindowAggregator_TumblingWindow(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	records := []*Record{
		{Fields: map[string]interface{}{"value": 10.0}, Timestamp: base},
		{Fields: map[string]interface{}{"value": 20.0}, Timestamp: base.Add(30 * time.Second)},
		{Fields: map[string]interface{}{"value": 30.0}, Timestamp: base.Add(65 * time.Second)},
		{Fields: map[string]interface{}{"value": 40.0}, Timestamp: base.Add(90 * time.Second)},
	}

	agg := NewWindowAggregator([]WindowFunction{
		{Name: "total", Function: "sum", Field: "value", Window: WindowSpec{Type: "tumbling", Size: time.Minute}},
		{Name: "cnt", Function: "count", Field: "value", Window: WindowSpec{Type: "tumbling", Size: time.Minute}},
	}, nil)

	results, err := agg.Aggregate(records)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(results))
	}

	// First window [00:00, 01:00) should have records at 0s and 30s
	first := results[0]
	if first.RecordCount != 2 {
		t.Fatalf("expected 2 records in first window, got %d", first.RecordCount)
	}
	if first.Values["total"] != 30.0 {
		t.Fatalf("expected sum 30, got %v", first.Values["total"])
	}
}

func TestWindowAggregator_SlidingWindow(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	records := []*Record{
		{Fields: map[string]interface{}{"value": 10.0}, Timestamp: base},
		{Fields: map[string]interface{}{"value": 20.0}, Timestamp: base.Add(30 * time.Second)},
		{Fields: map[string]interface{}{"value": 30.0}, Timestamp: base.Add(60 * time.Second)},
	}

	agg := NewWindowAggregator([]WindowFunction{
		{Name: "avg_val", Function: "avg", Field: "value", Window: WindowSpec{
			Type: "sliding", Size: time.Minute, Slide: 30 * time.Second,
		}},
	}, nil)

	results, err := agg.Aggregate(records)
	if err != nil {
		t.Fatal(err)
	}

	// Sliding windows should overlap
	if len(results) < 2 {
		t.Fatalf("expected at least 2 windows, got %d", len(results))
	}
}

func TestWindowAggregator_SessionWindow(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	records := []*Record{
		{Fields: map[string]interface{}{"value": 1.0}, Timestamp: base},
		{Fields: map[string]interface{}{"value": 2.0}, Timestamp: base.Add(10 * time.Second)},
		// Gap > 30s
		{Fields: map[string]interface{}{"value": 3.0}, Timestamp: base.Add(60 * time.Second)},
		{Fields: map[string]interface{}{"value": 4.0}, Timestamp: base.Add(70 * time.Second)},
	}

	agg := NewWindowAggregator([]WindowFunction{
		{Name: "session_sum", Function: "sum", Field: "value", Window: WindowSpec{
			Type: "session", GapSize: 30 * time.Second,
		}},
	}, nil)

	results, err := agg.Aggregate(records)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(results))
	}

	if results[0].Values["session_sum"] != 3.0 {
		t.Fatalf("expected session 1 sum 3, got %v", results[0].Values["session_sum"])
	}
	if results[1].Values["session_sum"] != 7.0 {
		t.Fatalf("expected session 2 sum 7, got %v", results[1].Values["session_sum"])
	}
}

func TestWindowAggregator_GroupBy(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	records := []*Record{
		{Fields: map[string]interface{}{"user": "alice", "value": 10.0}, Timestamp: base},
		{Fields: map[string]interface{}{"user": "bob", "value": 20.0}, Timestamp: base.Add(time.Second)},
		{Fields: map[string]interface{}{"user": "alice", "value": 30.0}, Timestamp: base.Add(2 * time.Second)},
	}

	agg := NewWindowAggregator([]WindowFunction{
		{Name: "total", Function: "sum", Field: "value", Window: WindowSpec{
			Type: "tumbling", Size: time.Minute,
		}},
	}, []string{"user"})

	results, err := agg.Aggregate(records)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(results))
	}
}

func TestWindowAggregator_StdDev(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	records := []*Record{
		{Fields: map[string]interface{}{"value": 2.0}, Timestamp: base},
		{Fields: map[string]interface{}{"value": 4.0}, Timestamp: base.Add(time.Second)},
		{Fields: map[string]interface{}{"value": 4.0}, Timestamp: base.Add(2 * time.Second)},
		{Fields: map[string]interface{}{"value": 4.0}, Timestamp: base.Add(3 * time.Second)},
		{Fields: map[string]interface{}{"value": 5.0}, Timestamp: base.Add(4 * time.Second)},
		{Fields: map[string]interface{}{"value": 5.0}, Timestamp: base.Add(5 * time.Second)},
		{Fields: map[string]interface{}{"value": 7.0}, Timestamp: base.Add(6 * time.Second)},
		{Fields: map[string]interface{}{"value": 9.0}, Timestamp: base.Add(7 * time.Second)},
	}

	agg := NewWindowAggregator([]WindowFunction{
		{Name: "sd", Function: "stddev", Field: "value", Window: WindowSpec{
			Type: "tumbling", Size: time.Minute,
		}},
	}, nil)

	results, err := agg.Aggregate(records)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 window, got %d", len(results))
	}

	sd, ok := results[0].Values["sd"].(float64)
	if !ok || sd <= 0 {
		t.Fatalf("expected positive stddev, got %v", results[0].Values["sd"])
	}
}

func TestWindowAggregator_Empty(t *testing.T) {
	agg := NewWindowAggregator([]WindowFunction{
		{Name: "cnt", Function: "count", Field: "value"},
	}, nil)

	results, err := agg.Aggregate(nil)
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Fatal("expected nil for empty input")
	}
}
