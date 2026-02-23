package streamcompute

import (
	"sync"
	"time"
)

// PatternStep defines a single step in a pattern sequence.
type PatternStep struct {
	Name       string           `json:"name"`
	Condition  func(Event) bool `json:"-"`
	Expression string           `json:"expression,omitempty"`
}

// PatternSpec defines a pattern to detect in an event stream.
type PatternSpec struct {
	Name       string        `json:"name"`
	Steps      []PatternStep `json:"steps"`
	WithinTime time.Duration `json:"within_time"`
}

// PatternMatch represents a detected pattern occurrence.
type PatternMatch struct {
	PatternName string    `json:"pattern_name"`
	Events      []Event   `json:"events"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// PatternStats provides statistics about pattern matching.
type PatternStats struct {
	TotalEvents     int64 `json:"total_events"`
	TotalMatches    int64 `json:"total_matches"`
	PendingPartials int   `json:"pending_partials"`
	ExpiredPartials int64 `json:"expired_partials"`
}

// partialMatch tracks an in-progress pattern match.
type partialMatch struct {
	stepIndex int
	events    []Event
	startedAt time.Time
}

// PatternMatcher detects sequences of events matching a pattern specification.
type PatternMatcher struct {
	mu       sync.Mutex
	spec     PatternSpec
	partials []*partialMatch

	totalEvents     int64
	totalMatches    int64
	expiredPartials int64
}

// NewPatternMatcher creates a new pattern matcher for the given spec.
func NewPatternMatcher(spec PatternSpec) *PatternMatcher {
	return &PatternMatcher{
		spec:     spec,
		partials: make([]*partialMatch, 0),
	}
}

// ProcessEvent processes an event and returns any completed pattern matches.
func (pm *PatternMatcher) ProcessEvent(event Event) []PatternMatch {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.totalEvents++
	if len(pm.spec.Steps) == 0 {
		return nil
	}

	var matches []PatternMatch
	now := event.Timestamp

	// Expire old partial matches
	pm.expirePartials(now)

	// Try to advance existing partial matches
	active := make([]*partialMatch, 0, len(pm.partials))
	for _, p := range pm.partials {
		nextStep := pm.spec.Steps[p.stepIndex]
		if nextStep.Condition != nil && nextStep.Condition(event) {
			p.events = append(p.events, event)
			p.stepIndex++

			// Check if pattern is complete
			if p.stepIndex >= len(pm.spec.Steps) {
				matches = append(matches, PatternMatch{
					PatternName: pm.spec.Name,
					Events:      p.events,
					StartedAt:   p.startedAt,
					CompletedAt: now,
				})
				pm.totalMatches++
			} else {
				active = append(active, p)
			}
		} else {
			// Keep partial alive for future events
			active = append(active, p)
		}
	}
	pm.partials = active

	// Try to start a new partial match with the first step
	firstStep := pm.spec.Steps[0]
	if firstStep.Condition != nil && firstStep.Condition(event) {
		p := &partialMatch{
			stepIndex: 1,
			events:    []Event{event},
			startedAt: now,
		}
		if len(pm.spec.Steps) == 1 {
			// Single-step pattern
			matches = append(matches, PatternMatch{
				PatternName: pm.spec.Name,
				Events:      p.events,
				StartedAt:   p.startedAt,
				CompletedAt: now,
			})
			pm.totalMatches++
		} else {
			pm.partials = append(pm.partials, p)
		}
	}

	return matches
}

// Stats returns pattern matching statistics.
func (pm *PatternMatcher) Stats() PatternStats {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return PatternStats{
		TotalEvents:     pm.totalEvents,
		TotalMatches:    pm.totalMatches,
		PendingPartials: len(pm.partials),
		ExpiredPartials: pm.expiredPartials,
	}
}

// expirePartials removes partial matches that have exceeded the time window.
// Must be called with pm.mu held.
func (pm *PatternMatcher) expirePartials(now time.Time) {
	if pm.spec.WithinTime <= 0 {
		return
	}

	active := make([]*partialMatch, 0, len(pm.partials))
	for _, p := range pm.partials {
		if now.Sub(p.startedAt) > pm.spec.WithinTime {
			pm.expiredPartials++
		} else {
			active = append(active, p)
		}
	}
	pm.partials = active
}
