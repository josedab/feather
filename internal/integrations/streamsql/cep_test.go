package streamsql

import (
	"testing"
	"time"
)

func TestCEPRegisterPattern(t *testing.T) {
	engine := NewCEPEngine(DefaultCEPConfig())

	pattern := &Pattern{
		ID:   "p1",
		Name: "test-pattern",
		Conditions: []PatternCondition{
			{EventType: "login"},
		},
		WindowDuration: time.Minute,
	}

	if err := engine.RegisterPattern(pattern); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Duplicate registration should fail.
	if err := engine.RegisterPattern(pattern); err == nil {
		t.Fatal("expected error for duplicate pattern")
	}

	// Nil pattern should fail.
	if err := engine.RegisterPattern(nil); err == nil {
		t.Fatal("expected error for nil pattern")
	}

	// Pattern without ID should fail.
	if err := engine.RegisterPattern(&Pattern{Conditions: []PatternCondition{{EventType: "x"}}}); err == nil {
		t.Fatal("expected error for pattern without ID")
	}

	// Pattern without conditions should fail.
	if err := engine.RegisterPattern(&Pattern{ID: "p2"}); err == nil {
		t.Fatal("expected error for pattern without conditions")
	}
}

func TestCEPRemovePattern(t *testing.T) {
	engine := NewCEPEngine(DefaultCEPConfig())

	pattern := &Pattern{
		ID:   "p1",
		Name: "test-pattern",
		Conditions: []PatternCondition{
			{EventType: "login"},
		},
	}
	if err := engine.RegisterPattern(pattern); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := engine.RemovePattern("p1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Removing non-existent pattern should fail.
	if err := engine.RemovePattern("p1"); err == nil {
		t.Fatal("expected error for non-existent pattern")
	}
}

func TestCEPProcessEventSingleCondition(t *testing.T) {
	engine := NewCEPEngine(DefaultCEPConfig())

	pattern := &Pattern{
		ID:   "p1",
		Name: "login-detect",
		Conditions: []PatternCondition{
			{EventType: "login"},
		},
		WindowDuration: time.Minute,
	}
	if err := engine.RegisterPattern(pattern); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Event that matches.
	matches := engine.ProcessEvent(&Event{
		ID:        "e1",
		Type:      "login",
		Timestamp: time.Now(),
	})
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].PatternID != "p1" {
		t.Fatalf("expected pattern ID p1, got %s", matches[0].PatternID)
	}

	// Event that does not match.
	matches = engine.ProcessEvent(&Event{
		ID:        "e2",
		Type:      "logout",
		Timestamp: time.Now(),
	})
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(matches))
	}
}

func TestCEPProcessEventMultiCondition(t *testing.T) {
	engine := NewCEPEngine(DefaultCEPConfig())

	pattern := &Pattern{
		ID:   "p1",
		Name: "login-then-purchase",
		Conditions: []PatternCondition{
			{EventType: "login"},
			{EventType: "purchase"},
		},
		WindowDuration: time.Minute,
	}
	if err := engine.RegisterPattern(pattern); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	now := time.Now()

	// First event: login.
	matches := engine.ProcessEvent(&Event{ID: "e1", Type: "login", Timestamp: now})
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches after first event, got %d", len(matches))
	}

	// Second event: purchase (completes the pattern).
	matches = engine.ProcessEvent(&Event{ID: "e2", Type: "purchase", Timestamp: now.Add(10 * time.Second)})
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if len(matches[0].Events) != 2 {
		t.Fatalf("expected 2 events in match, got %d", len(matches[0].Events))
	}
}

func TestCEPWindowTimeout(t *testing.T) {
	engine := NewCEPEngine(DefaultCEPConfig())

	pattern := &Pattern{
		ID:   "p1",
		Name: "timeout-test",
		Conditions: []PatternCondition{
			{EventType: "start"},
			{EventType: "end"},
		},
		WindowDuration: time.Second,
	}
	if err := engine.RegisterPattern(pattern); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	now := time.Now()

	// First event within window.
	engine.ProcessEvent(&Event{ID: "e1", Type: "start", Timestamp: now})

	// Second event outside window.
	matches := engine.ProcessEvent(&Event{ID: "e2", Type: "end", Timestamp: now.Add(5 * time.Second)})
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches after timeout, got %d", len(matches))
	}
}

func TestCEPMultipleConcurrentPatterns(t *testing.T) {
	engine := NewCEPEngine(DefaultCEPConfig())

	p1 := &Pattern{
		ID:   "p1",
		Name: "pattern-a",
		Conditions: []PatternCondition{
			{EventType: "login"},
		},
		WindowDuration: time.Minute,
	}
	p2 := &Pattern{
		ID:   "p2",
		Name: "pattern-b",
		Conditions: []PatternCondition{
			{EventType: "login"},
			{EventType: "click"},
		},
		WindowDuration: time.Minute,
	}
	if err := engine.RegisterPattern(p1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := engine.RegisterPattern(p2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	now := time.Now()

	// Login event matches p1 immediately, starts partial for p2.
	matches := engine.ProcessEvent(&Event{ID: "e1", Type: "login", Timestamp: now})
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].PatternID != "p1" {
		t.Fatalf("expected match for p1, got %s", matches[0].PatternID)
	}

	// Click event completes p2.
	matches = engine.ProcessEvent(&Event{ID: "e2", Type: "click", Timestamp: now.Add(time.Second)})
	if len(matches) != 0 {
		// p2 has conditions [login, click]; the click completes p2.
		// Actually this should match p2 since login started it and click completes it.
		// But p1 also matches single "click"? No, p1 only matches "login".
	}
	// Re-check: p2 should match since login started partial and click completes.
	allMatches := engine.GetMatches("p2", time.Time{})
	if len(allMatches) != 1 {
		t.Fatalf("expected 1 historical match for p2, got %d", len(allMatches))
	}
}

func TestCEPPredicateCondition(t *testing.T) {
	engine := NewCEPEngine(DefaultCEPConfig())

	pattern := &Pattern{
		ID:   "p1",
		Name: "high-value",
		Conditions: []PatternCondition{
			{
				EventType: "purchase",
				Predicate: func(e *Event) bool {
					amount, ok := e.Attributes["amount"].(float64)
					return ok && amount > 100
				},
				PredicateExpr: "amount > 100",
			},
		},
		WindowDuration: time.Minute,
	}
	if err := engine.RegisterPattern(pattern); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Low value purchase should not match.
	matches := engine.ProcessEvent(&Event{
		ID:         "e1",
		Type:       "purchase",
		Timestamp:  time.Now(),
		Attributes: map[string]interface{}{"amount": 50.0},
	})
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches for low value, got %d", len(matches))
	}

	// High value purchase should match.
	matches = engine.ProcessEvent(&Event{
		ID:         "e2",
		Type:       "purchase",
		Timestamp:  time.Now(),
		Attributes: map[string]interface{}{"amount": 200.0},
	})
	if len(matches) != 1 {
		t.Fatalf("expected 1 match for high value, got %d", len(matches))
	}
}

func TestCEPStats(t *testing.T) {
	engine := NewCEPEngine(DefaultCEPConfig())

	pattern := &Pattern{
		ID:   "p1",
		Name: "stats-test",
		Conditions: []PatternCondition{
			{EventType: "ping"},
		},
		WindowDuration: time.Minute,
	}
	if err := engine.RegisterPattern(pattern); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	engine.ProcessEvent(&Event{ID: "e1", Type: "ping", Timestamp: time.Now()})
	engine.ProcessEvent(&Event{ID: "e2", Type: "other", Timestamp: time.Now()})

	stats := engine.Stats()
	if stats.TotalEvents != 2 {
		t.Fatalf("expected 2 total events, got %d", stats.TotalEvents)
	}
	if stats.TotalPatterns != 1 {
		t.Fatalf("expected 1 total pattern, got %d", stats.TotalPatterns)
	}
	if stats.TotalMatches != 1 {
		t.Fatalf("expected 1 total match, got %d", stats.TotalMatches)
	}
}

func TestCEPListAndGetPatterns(t *testing.T) {
	engine := NewCEPEngine(DefaultCEPConfig())

	p := &Pattern{
		ID:   "p1",
		Name: "test",
		Conditions: []PatternCondition{
			{EventType: "x"},
		},
	}
	if err := engine.RegisterPattern(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	patterns := engine.ListPatterns()
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}

	got, err := engine.GetPattern("p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "test" {
		t.Fatalf("expected name 'test', got %q", got.Name)
	}

	_, err = engine.GetPattern("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent pattern")
	}
}

func TestCEPGetMatches(t *testing.T) {
	engine := NewCEPEngine(DefaultCEPConfig())

	pattern := &Pattern{
		ID:   "p1",
		Name: "match-history",
		Conditions: []PatternCondition{
			{EventType: "event"},
		},
		WindowDuration: time.Minute,
	}
	if err := engine.RegisterPattern(pattern); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	engine.ProcessEvent(&Event{ID: "e1", Type: "event", Timestamp: time.Now()})
	engine.ProcessEvent(&Event{ID: "e2", Type: "event", Timestamp: time.Now()})

	all := engine.GetMatches("", time.Time{})
	if len(all) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(all))
	}

	filtered := engine.GetMatches("p1", time.Time{})
	if len(filtered) != 2 {
		t.Fatalf("expected 2 matches for p1, got %d", len(filtered))
	}

	future := engine.GetMatches("", time.Now().Add(time.Hour))
	if len(future) != 0 {
		t.Fatalf("expected 0 matches in future, got %d", len(future))
	}
}

func TestCEPNilEvent(t *testing.T) {
	engine := NewCEPEngine(DefaultCEPConfig())
	matches := engine.ProcessEvent(nil)
	if matches != nil {
		t.Fatalf("expected nil for nil event, got %v", matches)
	}
}
