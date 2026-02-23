package streamcompute

import (
	"testing"
	"time"
)

func TestPatternMatcher_SimpleSequence(t *testing.T) {
	spec := PatternSpec{
		Name: "login-then-purchase",
		Steps: []PatternStep{
			{
				Name:       "login",
				Expression: "key == login",
				Condition:  func(e Event) bool { return e.Key == "login" },
			},
			{
				Name:       "purchase",
				Expression: "key == purchase",
				Condition:  func(e Event) bool { return e.Key == "purchase" },
			},
		},
		WithinTime: 10 * time.Second,
	}

	pm := NewPatternMatcher(spec)
	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// First step: login
	matches := pm.ProcessEvent(Event{Key: "login", Value: 1, Timestamp: base})
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches after first step, got %d", len(matches))
	}

	// Second step: purchase
	matches = pm.ProcessEvent(Event{Key: "purchase", Value: 50, Timestamp: base.Add(3 * time.Second)})
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].PatternName != "login-then-purchase" {
		t.Errorf("expected pattern name 'login-then-purchase', got %q", matches[0].PatternName)
	}
	if len(matches[0].Events) != 2 {
		t.Errorf("expected 2 events in match, got %d", len(matches[0].Events))
	}
}

func TestPatternMatcher_TimeoutExpiry(t *testing.T) {
	spec := PatternSpec{
		Name: "fast-sequence",
		Steps: []PatternStep{
			{
				Name:      "start",
				Condition: func(e Event) bool { return e.Key == "start" },
			},
			{
				Name:      "end",
				Condition: func(e Event) bool { return e.Key == "end" },
			},
		},
		WithinTime: 5 * time.Second,
	}

	pm := NewPatternMatcher(spec)
	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	pm.ProcessEvent(Event{Key: "start", Value: 1, Timestamp: base})

	// Too late - beyond WithinTime
	matches := pm.ProcessEvent(Event{Key: "end", Value: 2, Timestamp: base.Add(10 * time.Second)})
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches (expired), got %d", len(matches))
	}

	stats := pm.Stats()
	if stats.ExpiredPartials != 1 {
		t.Errorf("expected 1 expired partial, got %d", stats.ExpiredPartials)
	}
}

func TestPatternMatcher_SingleStep(t *testing.T) {
	spec := PatternSpec{
		Name: "error-detect",
		Steps: []PatternStep{
			{
				Name:      "error",
				Condition: func(e Event) bool { return e.Key == "error" },
			},
		},
		WithinTime: time.Minute,
	}

	pm := NewPatternMatcher(spec)
	matches := pm.ProcessEvent(Event{Key: "error", Value: 1, Timestamp: time.Now()})
	if len(matches) != 1 {
		t.Fatalf("expected 1 match for single-step pattern, got %d", len(matches))
	}
}

func TestPatternMatcher_NoMatch(t *testing.T) {
	spec := PatternSpec{
		Name: "sequence",
		Steps: []PatternStep{
			{
				Name:      "a",
				Condition: func(e Event) bool { return e.Key == "a" },
			},
			{
				Name:      "b",
				Condition: func(e Event) bool { return e.Key == "b" },
			},
		},
		WithinTime: 10 * time.Second,
	}

	pm := NewPatternMatcher(spec)
	base := time.Now()

	// Wrong order: b then a
	pm.ProcessEvent(Event{Key: "b", Value: 1, Timestamp: base})
	matches := pm.ProcessEvent(Event{Key: "a", Value: 2, Timestamp: base.Add(time.Second)})

	// Should not match b->a, but "a" starts a new partial
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches for wrong order, got %d", len(matches))
	}
}

func TestPatternMatcher_ThreeStepPattern(t *testing.T) {
	spec := PatternSpec{
		Name: "browse-cart-buy",
		Steps: []PatternStep{
			{Name: "browse", Condition: func(e Event) bool { return e.Key == "browse" }},
			{Name: "add_cart", Condition: func(e Event) bool { return e.Key == "add_cart" }},
			{Name: "purchase", Condition: func(e Event) bool { return e.Key == "purchase" }},
		},
		WithinTime: 30 * time.Second,
	}

	pm := NewPatternMatcher(spec)
	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	pm.ProcessEvent(Event{Key: "browse", Value: 1, Timestamp: base})
	pm.ProcessEvent(Event{Key: "add_cart", Value: 2, Timestamp: base.Add(5 * time.Second)})
	matches := pm.ProcessEvent(Event{Key: "purchase", Value: 3, Timestamp: base.Add(10 * time.Second)})

	if len(matches) != 1 {
		t.Fatalf("expected 1 match for 3-step pattern, got %d", len(matches))
	}
	if len(matches[0].Events) != 3 {
		t.Errorf("expected 3 events, got %d", len(matches[0].Events))
	}
}

func TestPatternMatcher_EmptySteps(t *testing.T) {
	spec := PatternSpec{
		Name:  "empty",
		Steps: []PatternStep{},
	}

	pm := NewPatternMatcher(spec)
	matches := pm.ProcessEvent(Event{Key: "any", Value: 1, Timestamp: time.Now()})
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches for empty pattern, got %d", len(matches))
	}
}

func TestPatternMatcher_Stats(t *testing.T) {
	spec := PatternSpec{
		Name: "test",
		Steps: []PatternStep{
			{Name: "a", Condition: func(e Event) bool { return e.Key == "a" }},
		},
		WithinTime: time.Minute,
	}

	pm := NewPatternMatcher(spec)
	pm.ProcessEvent(Event{Key: "a", Value: 1, Timestamp: time.Now()})
	pm.ProcessEvent(Event{Key: "x", Value: 2, Timestamp: time.Now()})

	stats := pm.Stats()
	if stats.TotalEvents != 2 {
		t.Errorf("expected 2 total events, got %d", stats.TotalEvents)
	}
	if stats.TotalMatches != 1 {
		t.Errorf("expected 1 match, got %d", stats.TotalMatches)
	}
}
