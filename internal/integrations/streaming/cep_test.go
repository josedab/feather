package streaming

import (
	"testing"
	"time"
)

func newTestEvent(id, typ, entityID string, data map[string]interface{}) *Event {
	return &Event{
		ID:        id,
		Type:      typ,
		EntityID:  entityID,
		Timestamp: time.Now(),
		Data:      data,
	}
}

// --- compareEqual ---

func TestCompareEqual_Numeric(t *testing.T) {
	if !compareEqual(float64(5), float64(5)) {
		t.Fatal("expected equal floats")
	}
	if !compareEqual(5, float64(5)) {
		t.Fatal("expected int == float64")
	}
	if compareEqual(float64(5), float64(6)) {
		t.Fatal("expected not equal")
	}
}

func TestCompareEqual_String(t *testing.T) {
	if !compareEqual("abc", "abc") {
		t.Fatal("expected equal strings")
	}
	if compareEqual("abc", "xyz") {
		t.Fatal("expected not equal")
	}
}

func TestCompareEqual_Bool(t *testing.T) {
	if !compareEqual(true, true) {
		t.Fatal("expected equal bools")
	}
	if compareEqual(true, false) {
		t.Fatal("expected not equal")
	}
}

func TestCompareEqual_MixedTypes(t *testing.T) {
	if compareEqual("5", 5) {
		t.Fatal("expected string != int")
	}
}

// --- compareNumeric ---

func TestCompareNumeric(t *testing.T) {
	if compareNumeric(10.0, 5.0) != 1 {
		t.Fatal("expected 10 > 5")
	}
	if compareNumeric(5.0, 10.0) != -1 {
		t.Fatal("expected 5 < 10")
	}
	if compareNumeric(5.0, 5.0) != 0 {
		t.Fatal("expected 5 == 5")
	}
}

func TestCompareNumeric_NonNumeric(t *testing.T) {
	if compareNumeric("a", "b") != 0 {
		t.Fatal("expected 0 for non-numeric")
	}
}

// --- toFloat64 ---

func TestToFloat64(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
		ok       bool
	}{
		{float64(5.5), 5.5, true},
		{float32(3.0), 3.0, true},
		{int(42), 42.0, true},
		{int64(100), 100.0, true},
		{int32(7), 7.0, true},
		{"string", 0, false},
		{true, 0, false},
	}

	for _, tt := range tests {
		val, ok := toFloat64(tt.input)
		if ok != tt.ok {
			t.Errorf("toFloat64(%v): ok = %v, want %v", tt.input, ok, tt.ok)
		}
		if ok && val != tt.expected {
			t.Errorf("toFloat64(%v) = %f, want %f", tt.input, val, tt.expected)
		}
	}
}

// --- evaluateCondition ---

func TestEvaluateCondition_Eq(t *testing.T) {
	c := NewCEPEngine()
	event := newTestEvent("e1", "click", "u1", map[string]interface{}{"amount": float64(100)})

	cond := PatternCondition{Field: "amount", Operator: OpEquals, Value: float64(100)}
	if !c.evaluateCondition(event, cond) {
		t.Fatal("expected eq match")
	}
}

func TestEvaluateCondition_Neq(t *testing.T) {
	c := NewCEPEngine()
	event := newTestEvent("e1", "click", "u1", map[string]interface{}{"amount": float64(100)})

	cond := PatternCondition{Field: "amount", Operator: OpNotEquals, Value: float64(50)}
	if !c.evaluateCondition(event, cond) {
		t.Fatal("expected neq match")
	}
}

func TestEvaluateCondition_Gt(t *testing.T) {
	c := NewCEPEngine()
	event := newTestEvent("e1", "click", "u1", map[string]interface{}{"amount": float64(100)})

	cond := PatternCondition{Field: "amount", Operator: OpGreaterThan, Value: float64(50)}
	if !c.evaluateCondition(event, cond) {
		t.Fatal("expected gt match")
	}
}

func TestEvaluateCondition_Lt(t *testing.T) {
	c := NewCEPEngine()
	event := newTestEvent("e1", "click", "u1", map[string]interface{}{"amount": float64(10)})

	cond := PatternCondition{Field: "amount", Operator: OpLessThan, Value: float64(50)}
	if !c.evaluateCondition(event, cond) {
		t.Fatal("expected lt match")
	}
}

func TestEvaluateCondition_Contains(t *testing.T) {
	c := NewCEPEngine()
	event := newTestEvent("e1", "click", "u1", map[string]interface{}{"name": "hello world"})

	cond := PatternCondition{Field: "name", Operator: OpContains, Value: "world"}
	if !c.evaluateCondition(event, cond) {
		t.Fatal("expected contains match")
	}

	cond2 := PatternCondition{Field: "name", Operator: OpContains, Value: "xyz"}
	if c.evaluateCondition(event, cond2) {
		t.Fatal("expected no contains match")
	}
}

func TestEvaluateCondition_StartsWith(t *testing.T) {
	c := NewCEPEngine()
	event := newTestEvent("e1", "click", "u1", map[string]interface{}{"name": "hello world"})

	cond := PatternCondition{Field: "name", Operator: OpStartsWith, Value: "hello"}
	if !c.evaluateCondition(event, cond) {
		t.Fatal("expected starts_with match")
	}
}

func TestEvaluateCondition_EndsWith(t *testing.T) {
	c := NewCEPEngine()
	event := newTestEvent("e1", "click", "u1", map[string]interface{}{"name": "hello world"})

	cond := PatternCondition{Field: "name", Operator: OpEndsWith, Value: "world"}
	if !c.evaluateCondition(event, cond) {
		t.Fatal("expected ends_with match")
	}
}

func TestEvaluateCondition_In(t *testing.T) {
	c := NewCEPEngine()
	event := newTestEvent("e1", "click", "u1", map[string]interface{}{"status": "active"})

	cond := PatternCondition{Field: "status", Operator: OpIn, Value: []interface{}{"active", "pending"}}
	if !c.evaluateCondition(event, cond) {
		t.Fatal("expected in match")
	}

	cond2 := PatternCondition{Field: "status", Operator: OpIn, Value: []interface{}{"deleted"}}
	if c.evaluateCondition(event, cond2) {
		t.Fatal("expected no in match")
	}
}

func TestEvaluateCondition_NotIn(t *testing.T) {
	c := NewCEPEngine()
	event := newTestEvent("e1", "click", "u1", map[string]interface{}{"status": "active"})

	cond := PatternCondition{Field: "status", Operator: OpNotIn, Value: []interface{}{"deleted", "banned"}}
	if !c.evaluateCondition(event, cond) {
		t.Fatal("expected not_in match")
	}
}

func TestEvaluateCondition_MissingField(t *testing.T) {
	c := NewCEPEngine()
	event := newTestEvent("e1", "click", "u1", map[string]interface{}{})

	cond := PatternCondition{Field: "missing", Operator: OpEquals, Value: "x"}
	if c.evaluateCondition(event, cond) {
		t.Fatal("expected no match for missing field")
	}
}

func TestEvaluateCondition_UnknownOperator(t *testing.T) {
	c := NewCEPEngine()
	event := newTestEvent("e1", "click", "u1", map[string]interface{}{"x": 1})

	cond := PatternCondition{Field: "x", Operator: "unknown", Value: 1}
	if c.evaluateCondition(event, cond) {
		t.Fatal("expected no match for unknown operator")
	}
}

// --- matchesStep ---

func TestMatchesStep_EventType(t *testing.T) {
	c := NewCEPEngine()
	event := newTestEvent("e1", "click", "u1", map[string]interface{}{})

	step := PatternStep{EventType: "click"}
	if !c.matchesStep(event, step) {
		t.Fatal("expected match")
	}

	step2 := PatternStep{EventType: "purchase"}
	if c.matchesStep(event, step2) {
		t.Fatal("expected no match for wrong event type")
	}
}

func TestMatchesStep_EmptyEventType(t *testing.T) {
	c := NewCEPEngine()
	event := newTestEvent("e1", "click", "u1", map[string]interface{}{})

	step := PatternStep{EventType: ""} // matches any type
	if !c.matchesStep(event, step) {
		t.Fatal("expected match with empty event type")
	}
}

func TestMatchesStep_WithConditions(t *testing.T) {
	c := NewCEPEngine()
	event := newTestEvent("e1", "click", "u1", map[string]interface{}{"amount": float64(100)})

	step := PatternStep{
		EventType: "click",
		Conditions: []PatternCondition{
			{Field: "amount", Operator: OpGreaterThan, Value: float64(50)},
		},
	}
	if !c.matchesStep(event, step) {
		t.Fatal("expected match")
	}
}

// --- tryMatch / MatchEvent ---

func TestMatchEvent_NoPatterns(t *testing.T) {
	c := NewCEPEngine()
	event := newTestEvent("e1", "click", "u1", map[string]interface{}{})
	matches := c.MatchEvent("pipeline1", event)
	if matches != nil {
		t.Fatal("expected no matches with no patterns")
	}
}

func TestMatchEvent_SingleStepPattern(t *testing.T) {
	c := NewCEPEngine()
	c.RegisterPattern("p1", CEPPattern{
		Name: "single",
		Sequence: []PatternStep{
			{EventType: "click"},
		},
	})

	event := newTestEvent("e1", "click", "u1", map[string]interface{}{})
	matches := c.MatchEvent("p1", event)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].PatternName != "single" {
		t.Fatalf("expected pattern name 'single', got %s", matches[0].PatternName)
	}
}

func TestMatchEvent_TwoStepSequential(t *testing.T) {
	c := NewCEPEngine()
	c.RegisterPattern("p1", CEPPattern{
		Name: "two_step",
		Sequence: []PatternStep{
			{EventType: "view"},
			{EventType: "purchase"},
		},
	})

	// First event starts pattern
	e1 := newTestEvent("e1", "view", "u1", map[string]interface{}{})
	matches := c.MatchEvent("p1", e1)
	if len(matches) != 0 {
		t.Fatal("expected no complete match after first event")
	}

	// Second event completes pattern
	e2 := newTestEvent("e2", "purchase", "u1", map[string]interface{}{})
	matches = c.MatchEvent("p1", e2)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
}

func TestMatchEvent_WindowExpiry(t *testing.T) {
	c := NewCEPEngine()
	c.RegisterPattern("p1", CEPPattern{
		Name:   "timed",
		Within: time.Millisecond,
		Sequence: []PatternStep{
			{EventType: "view"},
			{EventType: "purchase"},
		},
	})

	// First event
	e1 := &Event{ID: "e1", Type: "view", EntityID: "u1", Timestamp: time.Now().Add(-time.Second), Data: map[string]interface{}{}}
	c.MatchEvent("p1", e1)

	// Wait for window to expire
	time.Sleep(5 * time.Millisecond)

	// Second event — window expired, should restart
	e2 := newTestEvent("e2", "purchase", "u1", map[string]interface{}{})
	matches := c.MatchEvent("p1", e2)
	if len(matches) != 0 {
		t.Fatal("expected no match after window expiry")
	}
}

func TestMatchEvent_Contiguous_Broken(t *testing.T) {
	c := NewCEPEngine()
	c.RegisterPattern("p1", CEPPattern{
		Name:       "contiguous",
		Contiguous: true,
		Sequence: []PatternStep{
			{EventType: "a"},
			{EventType: "b"},
		},
	})

	e1 := newTestEvent("e1", "a", "u1", map[string]interface{}{})
	c.MatchEvent("p1", e1)

	// Wrong event type breaks contiguous pattern
	e2 := newTestEvent("e2", "c", "u1", map[string]interface{}{})
	matches := c.MatchEvent("p1", e2)
	if len(matches) != 0 {
		t.Fatal("expected no match for broken contiguous pattern")
	}

	// Now "b" won't match because state was reset
	e3 := newTestEvent("e3", "b", "u1", map[string]interface{}{})
	matches = c.MatchEvent("p1", e3)
	if len(matches) != 0 {
		t.Fatal("expected no match after broken contiguous")
	}
}

func TestMatchEvent_NonContiguous_SkipsIrrelevant(t *testing.T) {
	c := NewCEPEngine()
	c.RegisterPattern("p1", CEPPattern{
		Name: "non_contiguous",
		Sequence: []PatternStep{
			{EventType: "a"},
			{EventType: "b"},
		},
	})

	e1 := newTestEvent("e1", "a", "u1", map[string]interface{}{})
	c.MatchEvent("p1", e1)

	// Irrelevant event — pattern should wait
	e2 := newTestEvent("e2", "c", "u1", map[string]interface{}{})
	c.MatchEvent("p1", e2)

	// Matching event completes pattern
	e3 := newTestEvent("e3", "b", "u1", map[string]interface{}{})
	matches := c.MatchEvent("p1", e3)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
}

func TestMatchEvent_DifferentEntities(t *testing.T) {
	c := NewCEPEngine()
	c.RegisterPattern("p1", CEPPattern{
		Name: "multi_entity",
		Sequence: []PatternStep{
			{EventType: "a"},
			{EventType: "b"},
		},
	})

	// u1: start pattern
	e1 := newTestEvent("e1", "a", "u1", map[string]interface{}{})
	c.MatchEvent("p1", e1)

	// u2: start pattern
	e2 := newTestEvent("e2", "a", "u2", map[string]interface{}{})
	c.MatchEvent("p1", e2)

	// u1: complete pattern
	e3 := newTestEvent("e3", "b", "u1", map[string]interface{}{})
	matches := c.MatchEvent("p1", e3)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match for u1, got %d", len(matches))
	}

	// u2 state should still be pending
	pending := c.GetPendingMatches("p1")
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending match for u2, got %d", len(pending))
	}
}

func TestMatchEvent_EmptyEventStream(t *testing.T) {
	c := NewCEPEngine()
	c.RegisterPattern("p1", CEPPattern{
		Name:     "test",
		Sequence: []PatternStep{{EventType: "a"}},
	})
	// No events sent — just check no panics
	pending := c.GetPendingMatches("p1")
	if len(pending) != 0 {
		t.Fatal("expected no pending matches")
	}
}

// --- RegisterPattern / UnregisterPatterns ---

func TestRegisterAndUnregisterPatterns(t *testing.T) {
	c := NewCEPEngine()
	c.RegisterPattern("p1", CEPPattern{Name: "pat1", Sequence: []PatternStep{{EventType: "a"}}})
	c.RegisterPattern("p1", CEPPattern{Name: "pat2", Sequence: []PatternStep{{EventType: "b"}}})

	patterns := c.GetActivePatterns("p1")
	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(patterns))
	}

	c.UnregisterPatterns("p1")
	patterns = c.GetActivePatterns("p1")
	if patterns != nil {
		t.Fatal("expected nil patterns after unregister")
	}
}

func TestGetActivePatterns_NonExistent(t *testing.T) {
	c := NewCEPEngine()
	patterns := c.GetActivePatterns("nonexistent")
	if patterns != nil {
		t.Fatal("expected nil patterns")
	}
}

// --- containsString ---

func TestContainsString(t *testing.T) {
	if !containsString("hello world", "world") {
		t.Fatal("expected contains")
	}
	if containsString("hello", "world") {
		t.Fatal("expected not contains")
	}
	if !containsString("abc", "abc") {
		t.Fatal("expected exact match contains")
	}
	if containsString("ab", "abc") {
		t.Fatal("expected not contains for longer substr")
	}
}
