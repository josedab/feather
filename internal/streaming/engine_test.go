package streaming

import (
	"context"
	"testing"
	"time"
)

func TestEngine_CreatePipeline(t *testing.T) {
	engine := NewEngine(DefaultConfig(), nil)

	pipeline := &Pipeline{
		Name:        "test-pipeline",
		Description: "Test pipeline for unit tests",
		InputType:   "purchase",
	}

	err := engine.CreatePipeline(pipeline)
	if err != nil {
		t.Fatalf("CreatePipeline failed: %v", err)
	}

	if pipeline.ID == "" {
		t.Error("expected pipeline ID to be set")
	}

	// Verify we can retrieve it
	retrieved, err := engine.GetPipeline(pipeline.ID)
	if err != nil {
		t.Fatalf("GetPipeline failed: %v", err)
	}

	if retrieved.Name != "test-pipeline" {
		t.Errorf("expected name 'test-pipeline', got %s", retrieved.Name)
	}
}

func TestEngine_StartStopPipeline(t *testing.T) {
	engine := NewEngine(DefaultConfig(), nil)

	pipeline := &Pipeline{Name: "test"}
	engine.CreatePipeline(pipeline)

	// Start
	err := engine.StartPipeline(pipeline.ID)
	if err != nil {
		t.Fatalf("StartPipeline failed: %v", err)
	}

	retrieved, _ := engine.GetPipeline(pipeline.ID)
	if retrieved.State != StateRunning {
		t.Errorf("expected state Running, got %v", retrieved.State)
	}

	// Stop
	err = engine.StopPipeline(pipeline.ID)
	if err != nil {
		t.Fatalf("StopPipeline failed: %v", err)
	}

	retrieved, _ = engine.GetPipeline(pipeline.ID)
	if retrieved.State != StateStopped {
		t.Errorf("expected state Stopped, got %v", retrieved.State)
	}
}

func TestEngine_ProcessEvent(t *testing.T) {
	engine := NewEngine(DefaultConfig(), nil)
	engine.Start()
	defer engine.Stop()

	pipeline := &Pipeline{
		Name:      "event-processor",
		InputType: "click",
		Windows: []WindowConfig{
			{
				Name: "click_count",
				Type: WindowTypeTumbling,
				Size: 1 * time.Minute,
				Aggregations: []AggregationConfig{
					{
						Name:       "count",
						Field:      "value",
						Function:   AggCount,
						OutputName: "click_count",
					},
				},
			},
		},
	}
	engine.CreatePipeline(pipeline)
	engine.StartPipeline(pipeline.ID)

	// Process events
	for i := 0; i < 10; i++ {
		event := &Event{
			ID:        "event-" + string(rune('0'+i)),
			Type:      "click",
			EntityID:  "user-123",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"value": 1.0,
			},
		}
		err := engine.ProcessEvent(context.Background(), event)
		if err != nil {
			t.Fatalf("ProcessEvent failed: %v", err)
		}
	}

	// Check metrics
	p, _ := engine.GetPipeline(pipeline.ID)
	if p.Metrics.EventsProcessed != 10 {
		t.Errorf("expected 10 events processed, got %d", p.Metrics.EventsProcessed)
	}
}

func TestWindowManager_TumblingWindow(t *testing.T) {
	config := WindowConfig{
		Name: "test_window",
		Type: WindowTypeTumbling,
		Size: 10 * time.Second,
		Aggregations: []AggregationConfig{
			{
				Name:       "sum",
				Field:      "amount",
				Function:   AggSum,
				OutputName: "total_amount",
			},
		},
	}

	wm := NewWindowManager(config, 1*time.Second)

	// Add events
	now := time.Now().Truncate(10 * time.Second)
	for i := 0; i < 5; i++ {
		event := &Event{
			EntityID:  "entity-1",
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Data: map[string]interface{}{
				"amount": 10.0,
			},
		}
		wm.AddEvent(event)
	}

	stats := wm.GetWindowStats()
	if stats["window_count"].(int) != 1 {
		t.Errorf("expected 1 window, got %d", stats["window_count"])
	}
}

func TestWindowManager_SlidingWindow(t *testing.T) {
	config := WindowConfig{
		Name:          "sliding_test",
		Type:          WindowTypeSliding,
		Size:          30 * time.Second,
		SlideInterval: 10 * time.Second,
		Aggregations: []AggregationConfig{
			{
				Name:       "avg",
				Field:      "value",
				Function:   AggAvg,
				OutputName: "avg_value",
			},
		},
	}

	wm := NewWindowManager(config, 1*time.Second)

	now := time.Now()
	for i := 0; i < 3; i++ {
		event := &Event{
			EntityID:  "entity-1",
			Timestamp: now.Add(time.Duration(i*5) * time.Second),
			Data: map[string]interface{}{
				"value": float64(i * 10),
			},
		}
		wm.AddEvent(event)
	}

	// Sliding windows overlap, so we might have multiple
	stats := wm.GetWindowStats()
	if stats["window_count"].(int) < 1 {
		t.Error("expected at least 1 sliding window")
	}
}

func TestCEPEngine_PatternMatch(t *testing.T) {
	cep := NewCEPEngine()

	// Pattern: login -> browse -> purchase (within 5 minutes)
	pattern := CEPPattern{
		Name:        "conversion_funnel",
		Description: "User conversion pattern",
		Sequence: []PatternStep{
			{Name: "login", EventType: "login"},
			{Name: "browse", EventType: "browse"},
			{Name: "purchase", EventType: "purchase"},
		},
		Within:     5 * time.Minute,
		Contiguous: false,
	}

	cep.RegisterPattern("pipeline-1", pattern)

	// Send events
	now := time.Now()

	event1 := &Event{
		ID:        "e1",
		Type:      "login",
		EntityID:  "user-1",
		Timestamp: now,
		Data:      map[string]interface{}{},
	}
	matches1 := cep.MatchEvent("pipeline-1", event1)
	if len(matches1) != 0 {
		t.Error("should not match after first event")
	}

	event2 := &Event{
		ID:        "e2",
		Type:      "browse",
		EntityID:  "user-1",
		Timestamp: now.Add(1 * time.Minute),
		Data:      map[string]interface{}{},
	}
	matches2 := cep.MatchEvent("pipeline-1", event2)
	if len(matches2) != 0 {
		t.Error("should not match after second event")
	}

	event3 := &Event{
		ID:        "e3",
		Type:      "purchase",
		EntityID:  "user-1",
		Timestamp: now.Add(2 * time.Minute),
		Data:      map[string]interface{}{},
	}
	matches3 := cep.MatchEvent("pipeline-1", event3)
	if len(matches3) != 1 {
		t.Errorf("expected 1 match after third event, got %d", len(matches3))
	}

	if len(matches3) > 0 && matches3[0].PatternName != "conversion_funnel" {
		t.Errorf("expected pattern name 'conversion_funnel', got %s", matches3[0].PatternName)
	}
}

func TestCEPEngine_ConditionMatch(t *testing.T) {
	cep := NewCEPEngine()

	pattern := CEPPattern{
		Name: "high_value_order",
		Sequence: []PatternStep{
			{
				Name:      "order",
				EventType: "order",
				Conditions: []PatternCondition{
					{Field: "amount", Operator: OpGreaterThan, Value: 100.0},
				},
			},
		},
	}

	cep.RegisterPattern("pipeline-1", pattern)

	// Low value order - should not match
	lowOrder := &Event{
		ID:        "e1",
		Type:      "order",
		EntityID:  "user-1",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"amount": 50.0},
	}
	if len(cep.MatchEvent("pipeline-1", lowOrder)) != 0 {
		t.Error("low value order should not match")
	}

	// High value order - should match
	highOrder := &Event{
		ID:        "e2",
		Type:      "order",
		EntityID:  "user-1",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"amount": 150.0},
	}
	if len(cep.MatchEvent("pipeline-1", highOrder)) != 1 {
		t.Error("high value order should match")
	}
}

func TestFilterProcessor(t *testing.T) {
	processor := NewFilterProcessor("filter", []FilterCondition{
		{Field: "status", Operator: OpEquals, Value: "active"},
	})

	// Should pass
	activeEvent := &Event{
		Data: map[string]interface{}{"status": "active"},
	}
	result, err := processor.Process(context.Background(), activeEvent)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if result == nil {
		t.Error("active event should pass filter")
	}

	// Should be filtered
	inactiveEvent := &Event{
		Data: map[string]interface{}{"status": "inactive"},
	}
	result, err = processor.Process(context.Background(), inactiveEvent)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if result != nil {
		t.Error("inactive event should be filtered")
	}
}

func TestTransformProcessor(t *testing.T) {
	processor := NewTransformProcessor("transform", []Transformation{
		{Type: TransformRename, SourceField: "old_name", TargetField: "new_name"},
		{Type: TransformCopy, SourceField: "value", TargetField: "value_copy"},
		{Type: TransformDelete, SourceField: "temp"},
	})

	event := &Event{
		Data: map[string]interface{}{
			"old_name": "test",
			"value":    100,
			"temp":     "delete_me",
		},
	}

	result, err := processor.Process(context.Background(), event)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Check rename
	if _, ok := result.Data["old_name"]; ok {
		t.Error("old_name should be renamed")
	}
	if result.Data["new_name"] != "test" {
		t.Error("new_name should have the renamed value")
	}

	// Check copy
	if result.Data["value_copy"] != 100 {
		t.Error("value_copy should be copied")
	}

	// Check delete
	if _, ok := result.Data["temp"]; ok {
		t.Error("temp should be deleted")
	}
}

func TestDeduplicateProcessor(t *testing.T) {
	processor := NewDeduplicateProcessor("dedup", 100)

	event1 := &Event{ID: "event-1"}
	event2 := &Event{ID: "event-2"}
	event1Dup := &Event{ID: "event-1"}

	// First occurrence should pass
	result, _ := processor.Process(context.Background(), event1)
	if result == nil {
		t.Error("first occurrence should pass")
	}

	// Second event should pass
	result, _ = processor.Process(context.Background(), event2)
	if result == nil {
		t.Error("second event should pass")
	}

	// Duplicate should be filtered
	result, _ = processor.Process(context.Background(), event1Dup)
	if result != nil {
		t.Error("duplicate should be filtered")
	}
}
