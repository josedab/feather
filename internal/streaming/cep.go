package streaming

import (
	"sync"
	"time"
)

// CEPEngine provides Complex Event Processing capabilities.
// It detects patterns across event streams.
type CEPEngine struct {
	mu       sync.RWMutex
	patterns map[string][]CEPPattern // keyed by pipeline ID
	states   map[string]*PatternState
}

// CEPPattern defines a pattern to match.
type CEPPattern struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Sequence    []PatternStep `json:"sequence"`
	Within      time.Duration `json:"within"`
	Contiguous  bool          `json:"contiguous"`
}

// PatternStep defines a single step in a pattern.
type PatternStep struct {
	Name       string             `json:"name"`
	EventType  string             `json:"event_type"`
	Conditions []PatternCondition `json:"conditions"`
	Quantifier Quantifier         `json:"quantifier"`
	Optional   bool               `json:"optional"`
}

// PatternCondition defines a condition for matching.
type PatternCondition struct {
	Field    string      `json:"field"`
	Operator Operator    `json:"operator"`
	Value    interface{} `json:"value"`
}

// Operator for pattern conditions.
type Operator string

const (
	// OpEquals checks for equality.
	OpEquals Operator = "eq"
	// OpNotEquals checks for inequality.
	OpNotEquals Operator = "neq"
	// OpGreaterThan checks for greater-than comparison.
	OpGreaterThan Operator = "gt"
	// OpLessThan checks for less-than comparison.
	OpLessThan Operator = "lt"
	// OpGreaterOrEqual checks for greater-than-or-equal comparison.
	OpGreaterOrEqual Operator = "gte"
	// OpLessOrEqual checks for less-than-or-equal comparison.
	OpLessOrEqual Operator = "lte"
	// OpContains checks if a value contains a substring.
	OpContains Operator = "contains"
	// OpStartsWith checks prefix matching.
	OpStartsWith Operator = "starts_with"
	// OpEndsWith checks suffix matching.
	OpEndsWith Operator = "ends_with"
	// OpIn checks membership in a set.
	OpIn Operator = "in"
	// OpNotIn checks non-membership in a set.
	OpNotIn Operator = "not_in"
	// OpRegex checks a regex match.
	OpRegex Operator = "regex"
)

// Quantifier for pattern steps.
type Quantifier struct {
	Type string `json:"type"` // one, one_or_more, zero_or_more, times
	Min  int    `json:"min,omitempty"`
	Max  int    `json:"max,omitempty"`
}

// PatternState tracks the current state of pattern matching for an entity.
type PatternState struct {
	PatternName   string
	EntityID      string
	CurrentStep   int
	MatchedEvents []*Event
	StartTime     time.Time
	LastEventTime time.Time
}

// PatternMatch represents a successful pattern match.
type PatternMatch struct {
	PatternName string
	EntityID    string
	Events      []*Event
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
}

// NewCEPEngine creates a new CEP engine.
func NewCEPEngine() *CEPEngine {
	return &CEPEngine{
		patterns: make(map[string][]CEPPattern),
		states:   make(map[string]*PatternState),
	}
}

// RegisterPattern registers a pattern for a pipeline.
func (c *CEPEngine) RegisterPattern(pipelineID string, pattern CEPPattern) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.patterns[pipelineID] = append(c.patterns[pipelineID], pattern)
}

// UnregisterPatterns removes all patterns for a pipeline.
func (c *CEPEngine) UnregisterPatterns(pipelineID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.patterns, pipelineID)

	// Clean up states
	for key := range c.states {
		if len(key) > len(pipelineID) && key[:len(pipelineID)+1] == pipelineID+":" {
			delete(c.states, key)
		}
	}
}

// MatchEvent attempts to match an event against registered patterns.
func (c *CEPEngine) MatchEvent(pipelineID string, event *Event) []PatternMatch {
	c.mu.Lock()
	defer c.mu.Unlock()

	patterns, ok := c.patterns[pipelineID]
	if !ok {
		return nil
	}

	var matches []PatternMatch

	for _, pattern := range patterns {
		match := c.tryMatch(pipelineID, pattern, event)
		if match != nil {
			matches = append(matches, *match)
		}
	}

	return matches
}

func (c *CEPEngine) tryMatch(pipelineID string, pattern CEPPattern, event *Event) *PatternMatch {
	stateKey := pipelineID + ":" + pattern.Name + ":" + event.EntityID

	state, exists := c.states[stateKey]

	// Check if pattern window has expired
	if exists && pattern.Within > 0 && time.Since(state.StartTime) > pattern.Within {
		delete(c.states, stateKey)
		exists = false
	}

	// Try to start a new pattern match
	if !exists {
		if c.matchesStep(event, pattern.Sequence[0]) {
			// Start new state
			c.states[stateKey] = &PatternState{
				PatternName:   pattern.Name,
				EntityID:      event.EntityID,
				CurrentStep:   0,
				MatchedEvents: []*Event{event},
				StartTime:     event.Timestamp,
				LastEventTime: event.Timestamp,
			}

			// Check if pattern is complete (single step)
			if len(pattern.Sequence) == 1 {
				match := &PatternMatch{
					PatternName: pattern.Name,
					EntityID:    event.EntityID,
					Events:      []*Event{event},
					StartTime:   event.Timestamp,
					EndTime:     event.Timestamp,
					Duration:    0,
				}
				delete(c.states, stateKey)
				return match
			}
		}
		return nil
	}

	// Continue existing pattern match
	state = c.states[stateKey]
	nextStepIdx := state.CurrentStep + 1

	// Check contiguity if required
	if pattern.Contiguous {
		// For contiguous patterns, event must match the next step
		if c.matchesStep(event, pattern.Sequence[nextStepIdx]) {
			state.MatchedEvents = append(state.MatchedEvents, event)
			state.CurrentStep = nextStepIdx
			state.LastEventTime = event.Timestamp

			// Check if pattern is complete
			if nextStepIdx == len(pattern.Sequence)-1 {
				match := &PatternMatch{
					PatternName: pattern.Name,
					EntityID:    event.EntityID,
					Events:      state.MatchedEvents,
					StartTime:   state.StartTime,
					EndTime:     event.Timestamp,
					Duration:    event.Timestamp.Sub(state.StartTime),
				}
				delete(c.states, stateKey)
				return match
			}
		} else {
			// Pattern broken, reset
			delete(c.states, stateKey)
		}
	} else {
		// Non-contiguous: look for matching step
		if c.matchesStep(event, pattern.Sequence[nextStepIdx]) {
			state.MatchedEvents = append(state.MatchedEvents, event)
			state.CurrentStep = nextStepIdx
			state.LastEventTime = event.Timestamp

			// Check if pattern is complete
			if nextStepIdx == len(pattern.Sequence)-1 {
				match := &PatternMatch{
					PatternName: pattern.Name,
					EntityID:    event.EntityID,
					Events:      state.MatchedEvents,
					StartTime:   state.StartTime,
					EndTime:     event.Timestamp,
					Duration:    event.Timestamp.Sub(state.StartTime),
				}
				delete(c.states, stateKey)
				return match
			}
		}
		// Otherwise, keep waiting
	}

	return nil
}

func (c *CEPEngine) matchesStep(event *Event, step PatternStep) bool {
	// Check event type
	if step.EventType != "" && step.EventType != event.Type {
		return false
	}

	// Check conditions
	for _, cond := range step.Conditions {
		if !c.evaluateCondition(event, cond) {
			return false
		}
	}

	return true
}

func (c *CEPEngine) evaluateCondition(event *Event, cond PatternCondition) bool {
	value, ok := event.Data[cond.Field]
	if !ok {
		return false
	}

	switch cond.Operator {
	case OpEquals:
		return compareEqual(value, cond.Value)
	case OpNotEquals:
		return !compareEqual(value, cond.Value)
	case OpGreaterThan:
		return compareNumeric(value, cond.Value) > 0
	case OpLessThan:
		return compareNumeric(value, cond.Value) < 0
	case OpGreaterOrEqual:
		return compareNumeric(value, cond.Value) >= 0
	case OpLessOrEqual:
		return compareNumeric(value, cond.Value) <= 0
	case OpContains:
		str, ok1 := value.(string)
		substr, ok2 := cond.Value.(string)
		if !ok1 || !ok2 {
			return false
		}
		return containsString(str, substr)
	case OpStartsWith:
		str, ok1 := value.(string)
		prefix, ok2 := cond.Value.(string)
		if !ok1 || !ok2 {
			return false
		}
		return len(str) >= len(prefix) && str[:len(prefix)] == prefix
	case OpEndsWith:
		str, ok1 := value.(string)
		suffix, ok2 := cond.Value.(string)
		if !ok1 || !ok2 {
			return false
		}
		return len(str) >= len(suffix) && str[len(str)-len(suffix):] == suffix
	case OpIn:
		list, ok := cond.Value.([]interface{})
		if !ok {
			return false
		}
		for _, v := range list {
			if compareEqual(value, v) {
				return true
			}
		}
		return false
	case OpNotIn:
		list, ok := cond.Value.([]interface{})
		if !ok {
			return true
		}
		for _, v := range list {
			if compareEqual(value, v) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func compareEqual(a, b interface{}) bool {
	// Handle numeric comparisons
	aNum, aOk := toFloat64(a)
	bNum, bOk := toFloat64(b)
	if aOk && bOk {
		return aNum == bNum
	}

	// String comparison
	aStr, aOk := a.(string)
	bStr, bOk := b.(string)
	if aOk && bOk {
		return aStr == bStr
	}

	// Bool comparison
	aBool, aOk := a.(bool)
	bBool, bOk := b.(bool)
	if aOk && bOk {
		return aBool == bBool
	}

	return false
}

func compareNumeric(a, b interface{}) int {
	aNum, aOk := toFloat64(a)
	bNum, bOk := toFloat64(b)
	if !aOk || !bOk {
		return 0
	}

	if aNum < bNum {
		return -1
	} else if aNum > bNum {
		return 1
	}
	return 0
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	default:
		return 0, false
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetActivePatterns returns patterns for a pipeline.
func (c *CEPEngine) GetActivePatterns(pipelineID string) []CEPPattern {
	c.mu.RLock()
	defer c.mu.RUnlock()

	patterns, ok := c.patterns[pipelineID]
	if !ok {
		return nil
	}

	result := make([]CEPPattern, len(patterns))
	copy(result, patterns)
	return result
}

// GetPendingMatches returns currently pending pattern states.
func (c *CEPEngine) GetPendingMatches(pipelineID string) []*PatternState {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*PatternState
	prefix := pipelineID + ":"

	for key, state := range c.states {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			result = append(result, state)
		}
	}

	return result
}
