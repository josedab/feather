package streamsql

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// PatternState represents the state of a pattern match attempt.
type PatternState string

const (
	PatternStateInitial  PatternState = "initial"
	PatternStatePartial  PatternState = "partial"
	PatternStateMatched  PatternState = "matched"
	PatternStateTimedOut PatternState = "timed_out"
	PatternStateFailed   PatternState = "failed"
)

// CEPConfig configures the Complex Event Processing engine.
type CEPConfig struct {
	MaxPatterns       int           `json:"max_patterns"`
	WindowTimeout     time.Duration `json:"window_timeout"`
	BufferSize        int           `json:"buffer_size"`
	EnablePersistence bool          `json:"enable_persistence"`
}

// DefaultCEPConfig returns a CEPConfig with sensible defaults.
func DefaultCEPConfig() CEPConfig {
	return CEPConfig{
		MaxPatterns:       1000,
		WindowTimeout:     5 * time.Minute,
		BufferSize:        10000,
		EnablePersistence: false,
	}
}

// Event represents an event to be processed by the CEP engine.
type Event struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Timestamp  time.Time              `json:"timestamp"`
	Attributes map[string]interface{} `json:"attributes"`
}

// PatternCondition defines a single condition within a pattern.
type PatternCondition struct {
	EventType     string             `json:"event_type"`
	Predicate     func(*Event) bool  `json:"-"`
	PredicateExpr string             `json:"predicate_expr"`
}

// Pattern defines a sequence of conditions to detect in the event stream.
type Pattern struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Conditions     []PatternCondition `json:"conditions"`
	WindowDuration time.Duration      `json:"window_duration"`
	State          PatternState       `json:"state"`
	CreatedAt      time.Time          `json:"created_at"`
}

// PatternMatch represents a successful pattern match.
type PatternMatch struct {
	PatternID  string    `json:"pattern_id"`
	Events     []*Event  `json:"events"`
	MatchedAt  time.Time `json:"matched_at"`
	Duration   time.Duration `json:"duration"`
	Confidence float64   `json:"confidence"`
}

// CEPStats provides runtime statistics about the CEP engine.
type CEPStats struct {
	TotalEvents     int64   `json:"total_events"`
	TotalPatterns   int     `json:"total_patterns"`
	TotalMatches    int64   `json:"total_matches"`
	ActivePatterns  int     `json:"active_patterns"`
	EventsPerSecond float64 `json:"events_per_second"`
	AvgMatchLatency float64 `json:"avg_match_latency_ms"`
}

// partialMatch tracks an in-progress pattern match attempt.
type partialMatch struct {
	patternID    string
	matchedIdx   int
	events       []*Event
	startedAt    time.Time
}

// CEPEngine is a thread-safe Complex Event Processing engine.
type CEPEngine struct {
	config         CEPConfig
	patterns       map[string]*Pattern
	matches        []*PatternMatch
	partials       []*partialMatch
	mu             sync.RWMutex
	totalEvents    int64
	totalMatches   int64
	startTime      time.Time
	totalLatencyNs int64
}

// NewCEPEngine creates a new CEP engine with the given configuration.
func NewCEPEngine(config CEPConfig) *CEPEngine {
	return &CEPEngine{
		config:    config,
		patterns:  make(map[string]*Pattern),
		matches:   make([]*PatternMatch, 0),
		partials:  make([]*partialMatch, 0),
		startTime: time.Now(),
	}
}

// RegisterPattern registers a new detection pattern.
func (e *CEPEngine) RegisterPattern(pattern *Pattern) error {
	if pattern == nil {
		return fmt.Errorf("registering pattern: pattern is nil")
	}
	if pattern.ID == "" {
		return fmt.Errorf("registering pattern: pattern ID is required")
	}
	if len(pattern.Conditions) == 0 {
		return fmt.Errorf("registering pattern: at least one condition is required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.patterns[pattern.ID]; exists {
		return fmt.Errorf("registering pattern: pattern %q already exists", pattern.ID)
	}
	if len(e.patterns) >= e.config.MaxPatterns {
		return fmt.Errorf("registering pattern: max patterns limit (%d) reached", e.config.MaxPatterns)
	}

	if pattern.WindowDuration == 0 {
		pattern.WindowDuration = e.config.WindowTimeout
	}
	if pattern.State == "" {
		pattern.State = PatternStateInitial
	}
	if pattern.CreatedAt.IsZero() {
		pattern.CreatedAt = time.Now()
	}

	e.patterns[pattern.ID] = pattern
	return nil
}

// RemovePattern removes a registered pattern by ID.
func (e *CEPEngine) RemovePattern(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.patterns[id]; !exists {
		return fmt.Errorf("removing pattern: pattern %q not found", id)
	}
	delete(e.patterns, id)

	// Remove partial matches for this pattern.
	filtered := e.partials[:0]
	for _, p := range e.partials {
		if p.patternID != id {
			filtered = append(filtered, p)
		}
	}
	e.partials = filtered

	return nil
}

// ProcessEvent processes an event against all registered patterns and returns any matches.
func (e *CEPEngine) ProcessEvent(event *Event) []*PatternMatch {
	if event == nil {
		return nil
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	atomic.AddInt64(&e.totalEvents, 1)
	startTime := time.Now()

	e.mu.Lock()
	defer e.mu.Unlock()

	var newMatches []*PatternMatch

	// Try to advance existing partial matches.
	remaining := e.partials[:0]
	for _, pm := range e.partials {
		pattern, exists := e.patterns[pm.patternID]
		if !exists {
			continue
		}

		// Check window timeout.
		if event.Timestamp.Sub(pm.startedAt) > pattern.WindowDuration {
			continue // timed out
		}

		nextCond := pattern.Conditions[pm.matchedIdx]
		if e.eventMatchesCondition(event, &nextCond) {
			pm.events = append(pm.events, event)
			pm.matchedIdx++

			if pm.matchedIdx >= len(pattern.Conditions) {
				// Full match.
				match := &PatternMatch{
					PatternID:  pm.patternID,
					Events:     pm.events,
					MatchedAt:  time.Now(),
					Duration:   event.Timestamp.Sub(pm.startedAt),
					Confidence: 1.0,
				}
				e.matches = append(e.matches, match)
				newMatches = append(newMatches, match)
				atomic.AddInt64(&e.totalMatches, 1)
			} else {
				remaining = append(remaining, pm)
			}
		} else {
			remaining = append(remaining, pm)
		}
	}
	e.partials = remaining

	// Try to start new partial matches from first condition of each pattern.
	for id, pattern := range e.patterns {
		if len(pattern.Conditions) == 0 {
			continue
		}
		firstCond := pattern.Conditions[0]
		if e.eventMatchesCondition(event, &firstCond) {
			if len(pattern.Conditions) == 1 {
				// Single-condition pattern: immediate match.
				match := &PatternMatch{
					PatternID:  id,
					Events:     []*Event{event},
					MatchedAt:  time.Now(),
					Duration:   0,
					Confidence: 1.0,
				}
				e.matches = append(e.matches, match)
				newMatches = append(newMatches, match)
				atomic.AddInt64(&e.totalMatches, 1)
			} else {
				e.partials = append(e.partials, &partialMatch{
					patternID:  id,
					matchedIdx: 1,
					events:     []*Event{event},
					startedAt:  event.Timestamp,
				})
			}
		}
	}

	// Trim match buffer if too large.
	if len(e.matches) > e.config.BufferSize {
		e.matches = e.matches[len(e.matches)-e.config.BufferSize:]
	}

	latency := time.Since(startTime)
	atomic.AddInt64(&e.totalLatencyNs, int64(latency))

	return newMatches
}

// eventMatchesCondition checks if an event matches a pattern condition.
func (e *CEPEngine) eventMatchesCondition(event *Event, cond *PatternCondition) bool {
	if cond.EventType != "" && event.Type != cond.EventType {
		return false
	}
	if cond.Predicate != nil && !cond.Predicate(event) {
		return false
	}
	return true
}

// ListPatterns returns all registered patterns.
func (e *CEPEngine) ListPatterns() []*Pattern {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Pattern, 0, len(e.patterns))
	for _, p := range e.patterns {
		result = append(result, p)
	}
	return result
}

// GetPattern returns a pattern by ID.
func (e *CEPEngine) GetPattern(id string) (*Pattern, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	p, exists := e.patterns[id]
	if !exists {
		return nil, fmt.Errorf("getting pattern: pattern %q not found", id)
	}
	return p, nil
}

// GetMatches returns historical matches for a pattern since the given time.
func (e *CEPEngine) GetMatches(patternID string, since time.Time) []*PatternMatch {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*PatternMatch
	for _, m := range e.matches {
		if patternID != "" && m.PatternID != patternID {
			continue
		}
		if !since.IsZero() && m.MatchedAt.Before(since) {
			continue
		}
		result = append(result, m)
	}
	return result
}

// Stats returns runtime statistics about the CEP engine.
func (e *CEPEngine) Stats() *CEPStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	totalEvents := atomic.LoadInt64(&e.totalEvents)
	totalMatches := atomic.LoadInt64(&e.totalMatches)
	totalLatencyNs := atomic.LoadInt64(&e.totalLatencyNs)

	elapsed := time.Since(e.startTime).Seconds()
	var eps float64
	if elapsed > 0 {
		eps = float64(totalEvents) / elapsed
	}

	var avgLatency float64
	if totalEvents > 0 {
		avgLatency = float64(totalLatencyNs) / float64(totalEvents) / 1e6 // ns to ms
	}

	activeCount := 0
	for _, p := range e.patterns {
		if p.State == PatternStateInitial || p.State == PatternStatePartial {
			activeCount++
		}
	}

	return &CEPStats{
		TotalEvents:     totalEvents,
		TotalPatterns:   len(e.patterns),
		TotalMatches:    totalMatches,
		ActivePatterns:  activeCount,
		EventsPerSecond: eps,
		AvgMatchLatency: avgLatency,
	}
}
