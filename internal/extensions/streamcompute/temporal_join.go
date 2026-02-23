package streamcompute

import (
	"sync"
	"time"
)

// JoinType defines the type of temporal join.
type JoinType string

// JoinType values.
const (
	JoinInner JoinType = "inner"
	JoinLeft  JoinType = "left"
	JoinRight JoinType = "right"
)

// TemporalJoinConfig configures a temporal join between two streams.
type TemporalJoinConfig struct {
	LeftStream    string        `json:"left_stream"`
	RightStream   string        `json:"right_stream"`
	JoinType      JoinType      `json:"join_type"`
	JoinKey       string        `json:"join_key"`
	TimeTolerance time.Duration `json:"time_tolerance"`
	MaxBufferSize int           `json:"max_buffer_size"`
}

// JoinedEvent represents the result of joining two events.
type JoinedEvent struct {
	Left      *Event    `json:"left,omitempty"`
	Right     *Event    `json:"right,omitempty"`
	JoinKey   string    `json:"join_key"`
	MatchedAt time.Time `json:"matched_at"`
}

// JoinStats provides statistics about temporal join processing.
type JoinStats struct {
	LeftEvents     int64 `json:"left_events"`
	RightEvents    int64 `json:"right_events"`
	MatchedPairs   int64 `json:"matched_pairs"`
	UnmatchedLeft  int64 `json:"unmatched_left"`
	UnmatchedRight int64 `json:"unmatched_right"`
	BufferSize     int   `json:"buffer_size"`
}

// TemporalJoin joins two event streams based on time windows and keys.
type TemporalJoin struct {
	mu     sync.Mutex
	config TemporalJoinConfig

	leftBuffer  []Event
	rightBuffer []Event

	leftEvents     int64
	rightEvents    int64
	matchedPairs   int64
	unmatchedLeft  int64
	unmatchedRight int64
}

// NewTemporalJoin creates a new temporal join processor.
func NewTemporalJoin(cfg TemporalJoinConfig) *TemporalJoin {
	if cfg.MaxBufferSize <= 0 {
		cfg.MaxBufferSize = 10000
	}
	if cfg.TimeTolerance <= 0 {
		cfg.TimeTolerance = 5 * time.Second
	}
	return &TemporalJoin{
		config:      cfg,
		leftBuffer:  make([]Event, 0),
		rightBuffer: make([]Event, 0),
	}
}

// AddLeft adds an event to the left stream buffer.
func (tj *TemporalJoin) AddLeft(event Event) {
	tj.mu.Lock()
	defer tj.mu.Unlock()

	tj.leftEvents++
	tj.leftBuffer = append(tj.leftBuffer, event)
	if len(tj.leftBuffer) > tj.config.MaxBufferSize {
		tj.leftBuffer = tj.leftBuffer[1:]
	}
}

// AddRight adds an event to the right stream buffer.
func (tj *TemporalJoin) AddRight(event Event) {
	tj.mu.Lock()
	defer tj.mu.Unlock()

	tj.rightEvents++
	tj.rightBuffer = append(tj.rightBuffer, event)
	if len(tj.rightBuffer) > tj.config.MaxBufferSize {
		tj.rightBuffer = tj.rightBuffer[1:]
	}
}

// Match performs the temporal join and returns matched event pairs.
func (tj *TemporalJoin) Match() []JoinedEvent {
	tj.mu.Lock()
	defer tj.mu.Unlock()

	var results []JoinedEvent
	now := time.Now()

	leftMatched := make(map[int]bool)
	rightMatched := make(map[int]bool)

	// Find matching pairs based on key and time tolerance
	for i, left := range tj.leftBuffer {
		leftKey := tj.extractKey(left)
		for j, right := range tj.rightBuffer {
			rightKey := tj.extractKey(right)
			if leftKey != rightKey {
				continue
			}

			diff := left.Timestamp.Sub(right.Timestamp)
			if diff < 0 {
				diff = -diff
			}
			if diff <= tj.config.TimeTolerance {
				results = append(results, JoinedEvent{
					Left:      &Event{Key: left.Key, Value: left.Value, Timestamp: left.Timestamp, Fields: left.Fields},
					Right:     &Event{Key: right.Key, Value: right.Value, Timestamp: right.Timestamp, Fields: right.Fields},
					JoinKey:   leftKey,
					MatchedAt: now,
				})
				leftMatched[i] = true
				rightMatched[j] = true
				tj.matchedPairs++
			}
		}
	}

	// Handle unmatched events for left/right joins
	switch tj.config.JoinType {
	case JoinInner:
		// Inner join: unmatched events are simply discarded.
	case JoinLeft:
		for i, left := range tj.leftBuffer {
			if !leftMatched[i] {
				results = append(results, JoinedEvent{
					Left:      &Event{Key: left.Key, Value: left.Value, Timestamp: left.Timestamp, Fields: left.Fields},
					JoinKey:   tj.extractKey(left),
					MatchedAt: now,
				})
				tj.unmatchedLeft++
			}
		}
	case JoinRight:
		for j, right := range tj.rightBuffer {
			if !rightMatched[j] {
				results = append(results, JoinedEvent{
					Right:     &Event{Key: right.Key, Value: right.Value, Timestamp: right.Timestamp, Fields: right.Fields},
					JoinKey:   tj.extractKey(right),
					MatchedAt: now,
				})
				tj.unmatchedRight++
			}
		}
	}

	// Clear buffers after matching
	tj.leftBuffer = tj.leftBuffer[:0]
	tj.rightBuffer = tj.rightBuffer[:0]

	return results
}

// Stats returns temporal join statistics.
func (tj *TemporalJoin) Stats() JoinStats {
	tj.mu.Lock()
	defer tj.mu.Unlock()

	return JoinStats{
		LeftEvents:     tj.leftEvents,
		RightEvents:    tj.rightEvents,
		MatchedPairs:   tj.matchedPairs,
		UnmatchedLeft:  tj.unmatchedLeft,
		UnmatchedRight: tj.unmatchedRight,
		BufferSize:     len(tj.leftBuffer) + len(tj.rightBuffer),
	}
}

// extractKey returns the join key from an event. If a JoinKey field is
// configured, it looks in the event's Fields map; otherwise uses the event Key.
func (tj *TemporalJoin) extractKey(event Event) string {
	if tj.config.JoinKey != "" && event.Fields != nil {
		if v, ok := event.Fields[tj.config.JoinKey]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return event.Key
}
