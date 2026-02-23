package streamcompute

import (
	"fmt"
	"sync"
	"time"
)

// LateDataPolicy defines how late-arriving events are handled.
type LateDataPolicy string

// LateDataPolicy values.
const (
	LateDataDrop       LateDataPolicy = "drop"
	LateDataUpdate     LateDataPolicy = "update"
	LateDataSideOutput LateDataPolicy = "side_output"
)

// LateDataAction describes the action taken for a late event.
type LateDataAction struct {
	Policy  LateDataPolicy `json:"policy"`
	IsLate  bool           `json:"is_late"`
	Dropped bool           `json:"dropped"`
}

// LateDataStats provides statistics about late data handling.
type LateDataStats struct {
	TotalEvents      int64 `json:"total_events"`
	LateEvents       int64 `json:"late_events"`
	DroppedEvents    int64 `json:"dropped_events"`
	UpdatedEvents    int64 `json:"updated_events"`
	SideOutputEvents int64 `json:"side_output_events"`
	OnTimeEvents     int64 `json:"on_time_events"`
}

// LateDataHandler handles late-arriving events according to a configurable policy.
type LateDataHandler struct {
	mu              sync.Mutex
	policy          LateDataPolicy
	allowedLateness time.Duration
	sideOutput      []Event

	totalEvents      int64
	lateEvents       int64
	droppedEvents    int64
	updatedEvents    int64
	sideOutputEvents int64
	onTimeEvents     int64
}

// NewLateDataHandler creates a new late data handler.
func NewLateDataHandler(policy LateDataPolicy, allowedLateness time.Duration) *LateDataHandler {
	if allowedLateness < 0 {
		allowedLateness = 0
	}
	return &LateDataHandler{
		policy:          policy,
		allowedLateness: allowedLateness,
		sideOutput:      make([]Event, 0),
	}
}

// HandleEvent processes an event and determines the appropriate action
// based on the event timestamp relative to the watermark.
func (h *LateDataHandler) HandleEvent(event Event, watermark time.Time) (LateDataAction, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.totalEvents++

	action := LateDataAction{
		Policy: h.policy,
	}

	// Check if event is late relative to watermark
	if !event.Timestamp.Before(watermark) {
		h.onTimeEvents++
		action.IsLate = false
		return action, nil
	}

	action.IsLate = true
	h.lateEvents++

	// Check if within allowed lateness
	lateness := watermark.Sub(event.Timestamp)
	if h.allowedLateness > 0 && lateness <= h.allowedLateness {
		// Within allowed lateness, treat as update
		h.updatedEvents++
		action.Policy = LateDataUpdate
		return action, nil
	}

	// Beyond allowed lateness, apply policy
	switch h.policy {
	case LateDataDrop:
		h.droppedEvents++
		action.Dropped = true
		return action, nil

	case LateDataUpdate:
		h.updatedEvents++
		return action, nil

	case LateDataSideOutput:
		h.sideOutputEvents++
		h.sideOutput = append(h.sideOutput, event)
		return action, nil

	default:
		return action, fmt.Errorf("unknown late data policy: %s", h.policy)
	}
}

// GetSideOutput returns and drains the side output buffer.
func (h *LateDataHandler) GetSideOutput() []Event {
	h.mu.Lock()
	defer h.mu.Unlock()

	out := make([]Event, len(h.sideOutput))
	copy(out, h.sideOutput)
	h.sideOutput = h.sideOutput[:0]
	return out
}

// Stats returns late data handling statistics.
func (h *LateDataHandler) Stats() LateDataStats {
	h.mu.Lock()
	defer h.mu.Unlock()

	return LateDataStats{
		TotalEvents:      h.totalEvents,
		LateEvents:       h.lateEvents,
		DroppedEvents:    h.droppedEvents,
		UpdatedEvents:    h.updatedEvents,
		SideOutputEvents: h.sideOutputEvents,
		OnTimeEvents:     h.onTimeEvents,
	}
}
