package streaming

import (
	"math"
	"sort"
	"sync"
	"time"
)

// WindowManager manages windowed aggregations for a pipeline.
type WindowManager struct {
	mu             sync.RWMutex
	config         WindowConfig
	windows        map[string]*Window // keyed by entity ID
	lateTolerance  time.Duration
}

// Window represents a single window instance for an entity.
type Window struct {
	EntityID    string
	Start       time.Time
	End         time.Time
	Events      []*Event
	Aggregates  map[string]*AggregateState
}

// AggregateState holds the state for an aggregation.
type AggregateState struct {
	Config    AggregationConfig
	Count     int64
	Sum       float64
	Min       float64
	Max       float64
	First     interface{}
	Last      interface{}
	Values    []float64
	Distinct  map[interface{}]bool
	HasFirst  bool
}

// WindowResult represents the result of a window computation.
type WindowResult struct {
	Name        string
	EntityID    string
	WindowStart time.Time
	WindowEnd   time.Time
	Value       interface{}
}

// NewWindowManager creates a new window manager.
func NewWindowManager(config WindowConfig, lateTolerance time.Duration) *WindowManager {
	return &WindowManager{
		config:        config,
		windows:       make(map[string]*Window),
		lateTolerance: lateTolerance,
	}
}

// AddEvent adds an event to the appropriate window(s).
func (wm *WindowManager) AddEvent(event *Event) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	entityID := event.EntityID
	if entityID == "" {
		entityID = "_global"
	}

	// Get or create windows for this entity
	windows := wm.getWindowsForEvent(entityID, event.Timestamp)

	for _, w := range windows {
		w.Events = append(w.Events, event)

		// Update aggregates
		for name, agg := range w.Aggregates {
			wm.updateAggregate(agg, event, wm.config.Aggregations[wm.findAggIndex(name)])
		}
	}
}

func (wm *WindowManager) getWindowsForEvent(entityID string, eventTime time.Time) []*Window {
	var result []*Window

	switch wm.config.Type {
	case WindowTypeTumbling:
		// Single window at a time
		windowStart := eventTime.Truncate(wm.config.Size)
		windowEnd := windowStart.Add(wm.config.Size)
		key := entityID + ":" + windowStart.Format(time.RFC3339)

		w, ok := wm.windows[key]
		if !ok {
			w = wm.createWindow(entityID, windowStart, windowEnd)
			wm.windows[key] = w
		}
		result = append(result, w)

	case WindowTypeSliding:
		// Multiple overlapping windows
		slideInterval := wm.config.SlideInterval
		if slideInterval == 0 {
			slideInterval = wm.config.Size / 10
		}

		// Find all windows this event belongs to
		windowEnd := eventTime.Truncate(slideInterval).Add(slideInterval)
		for i := 0; i < int(wm.config.Size/slideInterval); i++ {
			windowStart := windowEnd.Add(-wm.config.Size)
			if eventTime.Before(windowStart) || !eventTime.Before(windowEnd) {
				windowEnd = windowEnd.Add(-slideInterval)
				continue
			}

			key := entityID + ":" + windowStart.Format(time.RFC3339)
			w, ok := wm.windows[key]
			if !ok {
				w = wm.createWindow(entityID, windowStart, windowEnd)
				wm.windows[key] = w
			}
			result = append(result, w)
			windowEnd = windowEnd.Add(-slideInterval)
		}

	case WindowTypeSession:
		// Session windows with gap-based boundaries
		// Find existing session or create new one
		var matchingWindow *Window
		for _, w := range wm.windows {
			if w.EntityID != entityID {
				continue
			}
			// Check if event is within session gap
			if eventTime.After(w.Start.Add(-wm.config.Size)) && eventTime.Before(w.End.Add(wm.config.Size)) {
				matchingWindow = w
				// Extend window end
				if eventTime.After(w.End) {
					w.End = eventTime.Add(wm.config.Size)
				}
				break
			}
		}

		if matchingWindow == nil {
			// Create new session
			w := wm.createWindow(entityID, eventTime, eventTime.Add(wm.config.Size))
			wm.windows[entityID+":"+eventTime.Format(time.RFC3339Nano)] = w
			matchingWindow = w
		}
		result = append(result, matchingWindow)

	case WindowTypeGlobal:
		// Single global window per entity
		key := entityID + ":global"
		w, ok := wm.windows[key]
		if !ok {
			w = wm.createWindow(entityID, time.Time{}, time.Time{})
			wm.windows[key] = w
		}
		result = append(result, w)
	}

	return result
}

func (wm *WindowManager) createWindow(entityID string, start, end time.Time) *Window {
	w := &Window{
		EntityID:   entityID,
		Start:      start,
		End:        end,
		Events:     make([]*Event, 0),
		Aggregates: make(map[string]*AggregateState),
	}

	// Initialize aggregates
	for _, agg := range wm.config.Aggregations {
		w.Aggregates[agg.OutputName] = &AggregateState{
			Config:   agg,
			Min:      math.MaxFloat64,
			Max:      -math.MaxFloat64,
			Values:   make([]float64, 0),
			Distinct: make(map[interface{}]bool),
		}
	}

	return w
}

func (wm *WindowManager) findAggIndex(name string) int {
	for i, agg := range wm.config.Aggregations {
		if agg.OutputName == name {
			return i
		}
	}
	return -1
}

func (wm *WindowManager) updateAggregate(state *AggregateState, event *Event, config AggregationConfig) {
	value, ok := event.Data[config.Field]
	if !ok {
		return
	}

	// Convert to float64 if possible
	var floatVal float64
	switch v := value.(type) {
	case float64:
		floatVal = v
	case float32:
		floatVal = float64(v)
	case int:
		floatVal = float64(v)
	case int64:
		floatVal = float64(v)
	case int32:
		floatVal = float64(v)
	}

	state.Count++
	state.Sum += floatVal
	state.Last = value
	state.Values = append(state.Values, floatVal)

	if !state.HasFirst {
		state.First = value
		state.HasFirst = true
	}

	if floatVal < state.Min {
		state.Min = floatVal
	}
	if floatVal > state.Max {
		state.Max = floatVal
	}

	state.Distinct[value] = true
}

// ComputeAndEvict computes results for completed windows and removes them.
func (wm *WindowManager) ComputeAndEvict(now time.Time) []WindowResult {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	var results []WindowResult
	var toDelete []string

	for key, w := range wm.windows {
		// Check if window is complete
		var isComplete bool

		switch wm.config.Type {
		case WindowTypeTumbling, WindowTypeSliding:
			isComplete = now.After(w.End.Add(wm.lateTolerance))
		case WindowTypeSession:
			// Session is complete if no events in gap period
			isComplete = now.After(w.End)
		case WindowTypeGlobal:
			isComplete = false // Global windows never complete automatically
		}

		if !isComplete {
			continue
		}

		// Compute aggregation results
		for name, state := range w.Aggregates {
			result := wm.computeAggregateResult(state)
			results = append(results, WindowResult{
				Name:        name,
				EntityID:    w.EntityID,
				WindowStart: w.Start,
				WindowEnd:   w.End,
				Value:       result,
			})
		}

		toDelete = append(toDelete, key)
	}

	// Clean up completed windows
	for _, key := range toDelete {
		delete(wm.windows, key)
	}

	return results
}

func (wm *WindowManager) computeAggregateResult(state *AggregateState) interface{} {
	switch state.Config.Function {
	case AggCount:
		return state.Count
	case AggSum:
		return state.Sum
	case AggAvg:
		if state.Count == 0 {
			return 0.0
		}
		return state.Sum / float64(state.Count)
	case AggMin:
		if state.Count == 0 {
			return 0.0
		}
		return state.Min
	case AggMax:
		if state.Count == 0 {
			return 0.0
		}
		return state.Max
	case AggFirst:
		return state.First
	case AggLast:
		return state.Last
	case AggDistinct:
		return int64(len(state.Distinct))
	case AggPercentile:
		if len(state.Values) == 0 {
			return 0.0
		}
		sorted := make([]float64, len(state.Values))
		copy(sorted, state.Values)
		sort.Float64s(sorted)
		// P95 by default
		idx := int(float64(len(sorted)) * 0.95)
		if idx >= len(sorted) {
			idx = len(sorted) - 1
		}
		return sorted[idx]
	case AggStdDev:
		if state.Count < 2 {
			return 0.0
		}
		mean := state.Sum / float64(state.Count)
		var variance float64
		for _, v := range state.Values {
			diff := v - mean
			variance += diff * diff
		}
		variance /= float64(state.Count)
		return math.Sqrt(variance)
	case AggVariance:
		if state.Count < 2 {
			return 0.0
		}
		mean := state.Sum / float64(state.Count)
		var variance float64
		for _, v := range state.Values {
			diff := v - mean
			variance += diff * diff
		}
		return variance / float64(state.Count)
	default:
		return nil
	}
}

// GetWindowCount returns the number of active windows.
func (wm *WindowManager) GetWindowCount() int {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return len(wm.windows)
}

// GetWindowStats returns statistics about windows.
func (wm *WindowManager) GetWindowStats() map[string]interface{} {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	totalEvents := 0
	for _, w := range wm.windows {
		totalEvents += len(w.Events)
	}

	return map[string]interface{}{
		"window_count":  len(wm.windows),
		"total_events":  totalEvents,
		"window_type":   wm.config.Type,
		"window_size":   wm.config.Size.String(),
	}
}
