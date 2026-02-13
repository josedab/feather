package streaming

import (
	"testing"
	"time"
)

func makeWindowConfig(wType WindowType, size time.Duration, aggs []AggregationConfig) WindowConfig {
	return WindowConfig{
		Name:         "test",
		Type:         wType,
		Size:         size,
		Aggregations: aggs,
	}
}

func makeSlidingWindowConfig(size, slide time.Duration, aggs []AggregationConfig) WindowConfig {
	return WindowConfig{
		Name:          "test",
		Type:          WindowTypeSliding,
		Size:          size,
		SlideInterval: slide,
		Aggregations:  aggs,
	}
}

func sumAgg() []AggregationConfig {
	return []AggregationConfig{
		{Name: "sum_amount", Field: "amount", Function: AggSum, OutputName: "sum_amount"},
	}
}

func multiAgg() []AggregationConfig {
	return []AggregationConfig{
		{Name: "sum_amount", Field: "amount", Function: AggSum, OutputName: "sum_amount"},
		{Name: "count_amount", Field: "amount", Function: AggCount, OutputName: "count_amount"},
		{Name: "avg_amount", Field: "amount", Function: AggAvg, OutputName: "avg_amount"},
		{Name: "min_amount", Field: "amount", Function: AggMin, OutputName: "min_amount"},
		{Name: "max_amount", Field: "amount", Function: AggMax, OutputName: "max_amount"},
	}
}

func makeEvent(entityID string, ts time.Time, amount float64) *Event {
	return &Event{
		ID:        "e",
		Type:      "test",
		EntityID:  entityID,
		Timestamp: ts,
		Data:      map[string]interface{}{"amount": amount},
	}
}

// --- Tumbling window ---

func TestTumblingWindow_Basic(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, sumAgg()), 0)

	now := time.Now().Truncate(time.Minute)
	wm.AddEvent(makeEvent("u1", now.Add(10*time.Second), 10.0))
	wm.AddEvent(makeEvent("u1", now.Add(20*time.Second), 20.0))

	if wm.GetWindowCount() != 1 {
		t.Fatalf("expected 1 window, got %d", wm.GetWindowCount())
	}

	// Evict after window ends
	results := wm.ComputeAndEvict(now.Add(2 * time.Minute))
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Value != 30.0 {
		t.Fatalf("expected sum 30.0, got %v", results[0].Value)
	}
}

func TestTumblingWindow_TwoWindows(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, sumAgg()), 0)

	now := time.Now().Truncate(time.Minute)
	wm.AddEvent(makeEvent("u1", now.Add(10*time.Second), 10.0))
	wm.AddEvent(makeEvent("u1", now.Add(70*time.Second), 20.0)) // next window

	if wm.GetWindowCount() != 2 {
		t.Fatalf("expected 2 windows, got %d", wm.GetWindowCount())
	}
}

func TestTumblingWindow_EmptyWindow(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, sumAgg()), 0)
	results := wm.ComputeAndEvict(time.Now().Add(time.Hour))
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty windows, got %d", len(results))
	}
}

// --- Sliding window ---

func TestSlidingWindow_MultipleWindows(t *testing.T) {
	wm := NewWindowManager(makeSlidingWindowConfig(time.Minute, 30*time.Second, sumAgg()), 0)

	now := time.Now().Truncate(30 * time.Second)
	wm.AddEvent(makeEvent("u1", now.Add(15*time.Second), 100.0))

	// Event should be in multiple overlapping windows
	if wm.GetWindowCount() < 1 {
		t.Fatalf("expected at least 1 window for sliding, got %d", wm.GetWindowCount())
	}
}

// --- Session window ---

func TestSessionWindow_GapCreatesNew(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeSession, 5*time.Minute, sumAgg()), 0)

	now := time.Now()
	wm.AddEvent(makeEvent("u1", now, 10.0))
	wm.AddEvent(makeEvent("u1", now.Add(2*time.Minute), 20.0)) // within gap

	if wm.GetWindowCount() != 1 {
		t.Fatalf("expected 1 session window, got %d", wm.GetWindowCount())
	}
}

func TestSessionWindow_Eviction(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeSession, time.Second, sumAgg()), 0)

	now := time.Now()
	wm.AddEvent(makeEvent("u1", now, 10.0))

	// Evict after session gap
	results := wm.ComputeAndEvict(now.Add(5 * time.Second))
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

// --- Global window ---

func TestGlobalWindow_NeverEvicts(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeGlobal, 0, sumAgg()), 0)

	wm.AddEvent(makeEvent("u1", time.Now(), 10.0))
	wm.AddEvent(makeEvent("u1", time.Now(), 20.0))

	// Global windows never complete automatically
	results := wm.ComputeAndEvict(time.Now().Add(time.Hour))
	if len(results) != 0 {
		t.Fatalf("expected 0 results for global window, got %d", len(results))
	}

	if wm.GetWindowCount() != 1 {
		t.Fatalf("expected 1 global window, got %d", wm.GetWindowCount())
	}
}

// --- Aggregate functions ---

func TestAggregates_AllFunctions(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, multiAgg()), 0)

	now := time.Now().Truncate(time.Minute)
	wm.AddEvent(makeEvent("u1", now.Add(10*time.Second), 10.0))
	wm.AddEvent(makeEvent("u1", now.Add(20*time.Second), 30.0))
	wm.AddEvent(makeEvent("u1", now.Add(30*time.Second), 20.0))

	results := wm.ComputeAndEvict(now.Add(2 * time.Minute))

	resultMap := make(map[string]interface{})
	for _, r := range results {
		resultMap[r.Name] = r.Value
	}

	if resultMap["sum_amount"] != 60.0 {
		t.Fatalf("expected sum 60.0, got %v", resultMap["sum_amount"])
	}
	if resultMap["count_amount"] != int64(3) {
		t.Fatalf("expected count 3, got %v", resultMap["count_amount"])
	}
	if resultMap["avg_amount"] != 20.0 {
		t.Fatalf("expected avg 20.0, got %v", resultMap["avg_amount"])
	}
	if resultMap["min_amount"] != 10.0 {
		t.Fatalf("expected min 10.0, got %v", resultMap["min_amount"])
	}
	if resultMap["max_amount"] != 30.0 {
		t.Fatalf("expected max 30.0, got %v", resultMap["max_amount"])
	}
}

// --- computeAggregateResult edge cases ---

func TestComputeAggregateResult_EmptyCount(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, []AggregationConfig{
		{Name: "avg", Field: "x", Function: AggAvg, OutputName: "avg"},
		{Name: "min", Field: "x", Function: AggMin, OutputName: "min"},
		{Name: "max", Field: "x", Function: AggMax, OutputName: "max"},
	}), 0)

	state := &AggregateState{Config: AggregationConfig{Function: AggAvg}, Count: 0}
	result := wm.computeAggregateResult(state)
	if result != 0.0 {
		t.Fatalf("expected 0.0 for empty avg, got %v", result)
	}

	state.Config.Function = AggMin
	result = wm.computeAggregateResult(state)
	if result != 0.0 {
		t.Fatalf("expected 0.0 for empty min, got %v", result)
	}

	state.Config.Function = AggMax
	result = wm.computeAggregateResult(state)
	if result != 0.0 {
		t.Fatalf("expected 0.0 for empty max, got %v", result)
	}
}

func TestComputeAggregateResult_First(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, nil), 0)

	state := &AggregateState{
		Config:   AggregationConfig{Function: AggFirst},
		First:    "first_val",
		HasFirst: true,
	}
	result := wm.computeAggregateResult(state)
	if result != "first_val" {
		t.Fatalf("expected first_val, got %v", result)
	}
}

func TestComputeAggregateResult_Last(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, nil), 0)

	state := &AggregateState{
		Config: AggregationConfig{Function: AggLast},
		Last:   "last_val",
	}
	result := wm.computeAggregateResult(state)
	if result != "last_val" {
		t.Fatalf("expected last_val, got %v", result)
	}
}

func TestComputeAggregateResult_Distinct(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, nil), 0)

	state := &AggregateState{
		Config:   AggregationConfig{Function: AggDistinct},
		Distinct: map[interface{}]bool{"a": true, "b": true, "c": true},
	}
	result := wm.computeAggregateResult(state)
	if result != int64(3) {
		t.Fatalf("expected 3 distinct, got %v", result)
	}
}

func TestComputeAggregateResult_Percentile(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, nil), 0)

	state := &AggregateState{
		Config: AggregationConfig{Function: AggPercentile},
		Values: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
	}
	result := wm.computeAggregateResult(state)
	if result.(float64) < 9 {
		t.Fatalf("expected P95 >= 9, got %v", result)
	}
}

func TestComputeAggregateResult_PercentileEmpty(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, nil), 0)

	state := &AggregateState{
		Config: AggregationConfig{Function: AggPercentile},
		Values: []float64{},
	}
	result := wm.computeAggregateResult(state)
	if result != 0.0 {
		t.Fatalf("expected 0.0 for empty percentile, got %v", result)
	}
}

func TestComputeAggregateResult_StdDev(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, nil), 0)

	state := &AggregateState{
		Config: AggregationConfig{Function: AggStdDev},
		Count:  3,
		Sum:    30,
		Values: []float64{10, 10, 10},
	}
	result := wm.computeAggregateResult(state)
	if result != 0.0 {
		t.Fatalf("expected stddev 0 for uniform values, got %v", result)
	}
}

func TestComputeAggregateResult_StdDevLessThan2(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, nil), 0)

	state := &AggregateState{
		Config: AggregationConfig{Function: AggStdDev},
		Count:  1,
	}
	result := wm.computeAggregateResult(state)
	if result != 0.0 {
		t.Fatalf("expected 0.0 for count < 2, got %v", result)
	}
}

func TestComputeAggregateResult_Variance(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, nil), 0)

	state := &AggregateState{
		Config: AggregationConfig{Function: AggVariance},
		Count:  2,
		Sum:    20,
		Values: []float64{5, 15},
	}
	result := wm.computeAggregateResult(state)
	if result.(float64) <= 0 {
		t.Fatalf("expected positive variance, got %v", result)
	}
}

func TestComputeAggregateResult_UnknownFunction(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, nil), 0)

	state := &AggregateState{
		Config: AggregationConfig{Function: "unknown"},
	}
	result := wm.computeAggregateResult(state)
	if result != nil {
		t.Fatalf("expected nil for unknown function, got %v", result)
	}
}

// --- updateAggregate edge cases ---

func TestUpdateAggregate_MissingField(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, sumAgg()), 0)

	now := time.Now().Truncate(time.Minute)
	event := &Event{
		ID:        "e1",
		Type:      "test",
		EntityID:  "u1",
		Timestamp: now.Add(10 * time.Second),
		Data:      map[string]interface{}{"other_field": 42},
	}
	wm.AddEvent(event)

	// Should not crash
	results := wm.ComputeAndEvict(now.Add(2 * time.Minute))
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Value != 0.0 {
		t.Fatalf("expected sum 0 for missing field, got %v", results[0].Value)
	}
}

func TestUpdateAggregate_IntValues(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, sumAgg()), 0)

	now := time.Now().Truncate(time.Minute)
	event := &Event{
		ID: "e1", Type: "test", EntityID: "u1",
		Timestamp: now.Add(10 * time.Second),
		Data:      map[string]interface{}{"amount": 42},
	}
	wm.AddEvent(event)

	results := wm.ComputeAndEvict(now.Add(2 * time.Minute))
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Value != 42.0 {
		t.Fatalf("expected sum 42.0, got %v", results[0].Value)
	}
}

// --- Empty entity ID ---

func TestAddEvent_EmptyEntityID(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, sumAgg()), 0)

	now := time.Now().Truncate(time.Minute)
	event := &Event{
		ID: "e1", Type: "test", EntityID: "",
		Timestamp: now.Add(10 * time.Second),
		Data:      map[string]interface{}{"amount": 5.0},
	}
	wm.AddEvent(event)

	// Should use "_global" as entity ID
	if wm.GetWindowCount() != 1 {
		t.Fatalf("expected 1 window, got %d", wm.GetWindowCount())
	}
}

// --- GetWindowStats ---

func TestGetWindowStats(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, sumAgg()), 0)

	now := time.Now().Truncate(time.Minute)
	wm.AddEvent(makeEvent("u1", now.Add(10*time.Second), 10.0))

	stats := wm.GetWindowStats()
	if stats["window_count"] != 1 {
		t.Fatalf("expected window_count 1, got %v", stats["window_count"])
	}
	if stats["total_events"] != 1 {
		t.Fatalf("expected total_events 1, got %v", stats["total_events"])
	}
	if stats["window_type"] != WindowTypeTumbling {
		t.Fatalf("expected window_type tumbling, got %v", stats["window_type"])
	}
}

// --- Late tolerance ---

func TestTumblingWindow_LateTolerance(t *testing.T) {
	wm := NewWindowManager(makeWindowConfig(WindowTypeTumbling, time.Minute, sumAgg()), 5*time.Minute)

	now := time.Now().Truncate(time.Minute)
	wm.AddEvent(makeEvent("u1", now.Add(10*time.Second), 10.0))

	// Without tolerance exceeded, should not evict
	results := wm.ComputeAndEvict(now.Add(90 * time.Second))
	if len(results) != 0 {
		t.Fatalf("expected 0 results within late tolerance, got %d", len(results))
	}

	// After tolerance, should evict
	results = wm.ComputeAndEvict(now.Add(7 * time.Minute))
	if len(results) != 1 {
		t.Fatalf("expected 1 result after late tolerance, got %d", len(results))
	}
}
