package streamcompute

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// WindowType defines the windowing strategy.
type WindowType int

const (
	// WindowTumbling creates fixed-size, non-overlapping windows.
	WindowTumbling WindowType = iota
	// WindowSliding creates fixed-size windows that advance by a slide interval.
	WindowSliding
	// WindowSession creates windows based on activity gaps.
	WindowSession
)

func (w WindowType) String() string {
	switch w {
	case WindowTumbling:
		return "tumbling"
	case WindowSliding:
		return "sliding"
	case WindowSession:
		return "session"
	default:
		return "unknown"
	}
}

// WindowConfig configures a processing window.
type WindowConfig struct {
	Type    WindowType    `json:"type"`
	Size    time.Duration `json:"size"`
	Slide   time.Duration `json:"slide,omitempty"`    // Only for sliding windows
	Gap     time.Duration `json:"gap,omitempty"`      // Only for session windows
	MaxLate time.Duration `json:"max_late,omitempty"` // Allowed lateness for events
}

// AggregationType defines the aggregation function applied to windowed data.
type AggregationType int

const (
	AggCount AggregationType = iota
	AggSum
	AggAvg
	AggMin
	AggMax
	AggFirst
	AggLast
)

func (a AggregationType) String() string {
	switch a {
	case AggCount:
		return "count"
	case AggSum:
		return "sum"
	case AggAvg:
		return "avg"
	case AggMin:
		return "min"
	case AggMax:
		return "max"
	case AggFirst:
		return "first"
	case AggLast:
		return "last"
	default:
		return "unknown"
	}
}

// Event represents an incoming data event.
type Event struct {
	Key       string                 `json:"key"`
	Value     float64                `json:"value"`
	Timestamp time.Time              `json:"timestamp"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// WindowResult represents the output of a window computation.
type WindowResult struct {
	PipelineID  string    `json:"pipeline_id"`
	Key         string    `json:"key"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	Value       float64   `json:"value"`
	Count       int64     `json:"count"`
	EmittedAt   time.Time `json:"emitted_at"`
}

// PipelineConfig defines a stream processing pipeline.
type PipelineConfig struct {
	ID            string          `json:"id"`
	Description   string          `json:"description,omitempty"`
	Window        WindowConfig    `json:"window"`
	GroupByKey    bool            `json:"group_by_key"`
	Aggregation   AggregationType `json:"aggregation"`
	OutputEntity  string          `json:"output_entity,omitempty"`
	OutputFeature string          `json:"output_feature,omitempty"`
}

// PipelineStatus represents the runtime status of a pipeline.
type PipelineStatus int

const (
	StatusCreated PipelineStatus = iota
	StatusRunning
	StatusStopped
	StatusFailed
)

func (s PipelineStatus) String() string {
	switch s {
	case StatusCreated:
		return "created"
	case StatusRunning:
		return "running"
	case StatusStopped:
		return "stopped"
	case StatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// PipelineInfo provides runtime information about a pipeline.
type PipelineInfo struct {
	Config        PipelineConfig `json:"config"`
	Status        string         `json:"status"`
	EventsIn      int64          `json:"events_in"`
	EventsOut     int64          `json:"events_out"`
	WindowsFired  int64          `json:"windows_fired"`
	LateEvents    int64          `json:"late_events"`
	LastEventAt   time.Time      `json:"last_event_at,omitempty"`
	StartedAt     time.Time      `json:"started_at,omitempty"`
	ActiveWindows int            `json:"active_windows"`
}

// windowState tracks per-key window state for aggregations.
type windowState struct {
	start   time.Time
	end     time.Time
	count   int64
	sum     float64
	min     float64
	max     float64
	first   float64
	last    float64
	hasData bool
}

// pipeline is the runtime representation of a processing pipeline.
type pipeline struct {
	config       PipelineConfig
	status       PipelineStatus
	eventsIn     int64
	eventsOut    int64
	windowsFired int64
	lateEvents   int64
	lastEvent    time.Time
	startedAt    time.Time

	// Per-key window state
	windows map[string]*windowState
	cancel  context.CancelFunc
}

// EngineConfig configures the stream compute engine.
type EngineConfig struct {
	MaxPipelines       int           `json:"max_pipelines"`
	CheckpointInterval time.Duration `json:"checkpoint_interval"`
	MaxLateAllowed     time.Duration `json:"max_late_allowed"`
	ResultBufferSize   int           `json:"result_buffer_size"`
}

// DefaultEngineConfig returns sensible defaults.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		MaxPipelines:       100,
		CheckpointInterval: 30 * time.Second,
		MaxLateAllowed:     1 * time.Minute,
		ResultBufferSize:   10000,
	}
}

// Engine orchestrates stream processing pipelines.
type Engine struct {
	mu        sync.RWMutex
	pipelines map[string]*pipeline
	config    EngineConfig
	results   []WindowResult
}

// NewEngine creates a new stream processing engine.
func NewEngine(config EngineConfig) *Engine {
	if config.MaxPipelines == 0 {
		config = DefaultEngineConfig()
	}
	return &Engine{
		pipelines: make(map[string]*pipeline),
		config:    config,
		results:   make([]WindowResult, 0, config.ResultBufferSize),
	}
}

// CreatePipeline creates a new processing pipeline.
func (e *Engine) CreatePipeline(cfg PipelineConfig) error {
	if err := validateWindowConfig(cfg.Window); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.pipelines[cfg.ID]; exists {
		return ErrPipelineExists
	}

	if len(e.pipelines) >= e.config.MaxPipelines {
		return fmt.Errorf("max pipelines reached (%d)", e.config.MaxPipelines)
	}

	e.pipelines[cfg.ID] = &pipeline{
		config:  cfg,
		status:  StatusCreated,
		windows: make(map[string]*windowState),
	}
	return nil
}

// StartPipeline starts a pipeline for processing.
func (e *Engine) StartPipeline(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	p, exists := e.pipelines[id]
	if !exists {
		return ErrPipelineNotFound
	}
	if p.status == StatusRunning {
		return ErrPipelineRunning
	}

	_, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.status = StatusRunning
	p.startedAt = time.Now()
	return nil
}

// StopPipeline stops a running pipeline.
func (e *Engine) StopPipeline(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	p, exists := e.pipelines[id]
	if !exists {
		return ErrPipelineNotFound
	}
	if p.status != StatusRunning {
		return ErrPipelineStopped
	}

	if p.cancel != nil {
		p.cancel()
	}
	p.status = StatusStopped
	return nil
}

// DeletePipeline removes a pipeline.
func (e *Engine) DeletePipeline(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	p, exists := e.pipelines[id]
	if !exists {
		return ErrPipelineNotFound
	}
	if p.status == StatusRunning {
		if p.cancel != nil {
			p.cancel()
		}
	}

	delete(e.pipelines, id)
	return nil
}

// Ingest processes an event through all running pipelines.
func (e *Engine) Ingest(event Event) []WindowResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	var results []WindowResult

	for _, p := range e.pipelines {
		if p.status != StatusRunning {
			continue
		}

		p.eventsIn++
		p.lastEvent = event.Timestamp

		key := event.Key
		if !p.config.GroupByKey {
			key = "__global__"
		}

		ws, exists := p.windows[key]
		if !exists {
			ws = e.initWindow(p, event.Timestamp)
			p.windows[key] = ws
		}

		// Check if event falls within current window
		if event.Timestamp.Before(ws.start) {
			// Late event
			maxLate := p.config.Window.MaxLate
			if maxLate == 0 {
				maxLate = e.config.MaxLateAllowed
			}
			if ws.start.Sub(event.Timestamp) > maxLate {
				p.lateEvents++
				continue
			}
		}

		// Check if window should fire
		if !event.Timestamp.Before(ws.end) {
			result := e.fireWindow(p, key, ws)
			results = append(results, result)
			p.windowsFired++
			p.eventsOut++

			// Start new window
			ws = e.initWindow(p, event.Timestamp)
			p.windows[key] = ws
		}

		// Update window state
		e.updateWindow(ws, event.Value, p.config.Aggregation)
	}

	e.results = append(e.results, results...)
	if len(e.results) > e.config.ResultBufferSize {
		e.results = e.results[len(e.results)-e.config.ResultBufferSize:]
	}

	return results
}

// GetPipeline returns info about a specific pipeline.
func (e *Engine) GetPipeline(id string) (PipelineInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	p, exists := e.pipelines[id]
	if !exists {
		return PipelineInfo{}, ErrPipelineNotFound
	}

	return pipelineToInfo(p), nil
}

// ListPipelines returns info about all pipelines.
func (e *Engine) ListPipelines() []PipelineInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	infos := make([]PipelineInfo, 0, len(e.pipelines))
	for _, p := range e.pipelines {
		infos = append(infos, pipelineToInfo(p))
	}
	return infos
}

// GetResults returns recent window results.
func (e *Engine) GetResults(pipelineID string, limit int) []WindowResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	var filtered []WindowResult
	for _, r := range e.results {
		if pipelineID != "" && r.PipelineID != pipelineID {
			continue
		}
		filtered = append(filtered, r)
	}

	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered
}

// Stats returns aggregate engine statistics.
func (e *Engine) Stats() EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var stats EngineStats
	stats.TotalPipelines = len(e.pipelines)
	stats.TotalResults = len(e.results)

	for _, p := range e.pipelines {
		switch p.status {
		case StatusRunning:
			stats.RunningPipelines++
		case StatusStopped:
			stats.StoppedPipelines++
		case StatusFailed:
			stats.FailedPipelines++
		}
		stats.TotalEventsIn += p.eventsIn
		stats.TotalEventsOut += p.eventsOut
		stats.TotalWindowsFired += p.windowsFired
		stats.TotalLateEvents += p.lateEvents
	}
	return stats
}

// EngineStats provides aggregate engine statistics.
type EngineStats struct {
	TotalPipelines    int   `json:"total_pipelines"`
	RunningPipelines  int   `json:"running_pipelines"`
	StoppedPipelines  int   `json:"stopped_pipelines"`
	FailedPipelines   int   `json:"failed_pipelines"`
	TotalEventsIn     int64 `json:"total_events_in"`
	TotalEventsOut    int64 `json:"total_events_out"`
	TotalWindowsFired int64 `json:"total_windows_fired"`
	TotalLateEvents   int64 `json:"total_late_events"`
	TotalResults      int   `json:"total_results"`
}

func (e *Engine) initWindow(p *pipeline, ts time.Time) *windowState {
	ws := &windowState{start: ts}
	switch p.config.Window.Type {
	case WindowTumbling:
		ws.end = ts.Add(p.config.Window.Size)
	case WindowSliding:
		ws.end = ts.Add(p.config.Window.Size)
	case WindowSession:
		ws.end = ts.Add(p.config.Window.Gap)
	}
	return ws
}

func (e *Engine) fireWindow(p *pipeline, key string, ws *windowState) WindowResult {
	value := computeAggregate(ws, p.config.Aggregation)
	return WindowResult{
		PipelineID:  p.config.ID,
		Key:         key,
		WindowStart: ws.start,
		WindowEnd:   ws.end,
		Value:       value,
		Count:       ws.count,
		EmittedAt:   time.Now(),
	}
}

func (e *Engine) updateWindow(ws *windowState, value float64, agg AggregationType) {
	ws.count++
	ws.sum += value
	ws.last = value

	if !ws.hasData {
		ws.min = value
		ws.max = value
		ws.first = value
		ws.hasData = true
	} else {
		if value < ws.min {
			ws.min = value
		}
		if value > ws.max {
			ws.max = value
		}
	}
}

func computeAggregate(ws *windowState, agg AggregationType) float64 {
	switch agg {
	case AggCount:
		return float64(ws.count)
	case AggSum:
		return ws.sum
	case AggAvg:
		if ws.count == 0 {
			return 0
		}
		return ws.sum / float64(ws.count)
	case AggMin:
		return ws.min
	case AggMax:
		return ws.max
	case AggFirst:
		return ws.first
	case AggLast:
		return ws.last
	default:
		return ws.sum
	}
}

func validateWindowConfig(w WindowConfig) error {
	if w.Size <= 0 && w.Type != WindowSession {
		return fmt.Errorf("%w: window size must be positive", ErrInvalidWindow)
	}
	if w.Type == WindowSliding && w.Slide <= 0 {
		return fmt.Errorf("%w: slide interval must be positive for sliding windows", ErrInvalidWindow)
	}
	if w.Type == WindowSession && w.Gap <= 0 {
		return fmt.Errorf("%w: gap duration must be positive for session windows", ErrInvalidWindow)
	}
	return nil
}

func pipelineToInfo(p *pipeline) PipelineInfo {
	return PipelineInfo{
		Config:        p.config,
		Status:        p.status.String(),
		EventsIn:      p.eventsIn,
		EventsOut:     p.eventsOut,
		WindowsFired:  p.windowsFired,
		LateEvents:    p.lateEvents,
		LastEventAt:   p.lastEvent,
		StartedAt:     p.startedAt,
		ActiveWindows: len(p.windows),
	}
}
