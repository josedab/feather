package streamcompute

import (
	"sync"
	"time"
)

// Watermark tracks event-time progress for a stream, enabling
// late-event detection and window firing decisions.
type Watermark struct {
	mu         sync.RWMutex
	current    time.Time
	maxLate    time.Duration
	idleTimeout time.Duration
	lastAdvance time.Time
}

// NewWatermark creates a watermark with the given allowed lateness.
func NewWatermark(maxLate time.Duration) *Watermark {
	return &Watermark{
		maxLate:     maxLate,
		idleTimeout: 5 * time.Minute,
		lastAdvance: time.Now(),
	}
}

// Advance updates the watermark based on an observed event timestamp.
// The watermark is set to max(current, eventTime - maxLate).
func (w *Watermark) Advance(eventTime time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()

	candidate := eventTime.Add(-w.maxLate)
	if candidate.After(w.current) {
		w.current = candidate
		w.lastAdvance = time.Now()
	}
}

// Current returns the current watermark value.
func (w *Watermark) Current() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.current
}

// IsLate returns true if the event timestamp is before the current watermark.
func (w *Watermark) IsLate(eventTime time.Time) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return eventTime.Before(w.current)
}

// Checkpoint represents a snapshot of pipeline state for recovery.
type Checkpoint struct {
	PipelineID string                      `json:"pipeline_id"`
	Watermark  time.Time                   `json:"watermark"`
	Windows    map[string]WindowCheckpoint `json:"windows"`
	Sequence   int64                       `json:"sequence"`
	CreatedAt  time.Time                   `json:"created_at"`
}

// WindowCheckpoint captures per-key window state for recovery.
type WindowCheckpoint struct {
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	Count   int64     `json:"count"`
	Sum     float64   `json:"sum"`
	Min     float64   `json:"min"`
	Max     float64   `json:"max"`
	First   float64   `json:"first"`
	Last    float64   `json:"last"`
	HasData bool      `json:"has_data"`
}

// CheckpointStore manages pipeline checkpoints for fault tolerance.
type CheckpointStore struct {
	mu          sync.RWMutex
	checkpoints map[string][]Checkpoint // pipelineID -> checkpoints (most recent last)
	maxPerPipeline int
}

// NewCheckpointStore creates a checkpoint store.
func NewCheckpointStore(maxPerPipeline int) *CheckpointStore {
	if maxPerPipeline <= 0 {
		maxPerPipeline = 10
	}
	return &CheckpointStore{
		checkpoints:    make(map[string][]Checkpoint),
		maxPerPipeline: maxPerPipeline,
	}
}

// Save stores a checkpoint for a pipeline.
func (cs *CheckpointStore) Save(cp Checkpoint) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cp.CreatedAt = time.Now()
	cps := cs.checkpoints[cp.PipelineID]
	cps = append(cps, cp)
	if len(cps) > cs.maxPerPipeline {
		cps = cps[len(cps)-cs.maxPerPipeline:]
	}
	cs.checkpoints[cp.PipelineID] = cps
}

// Latest returns the most recent checkpoint for a pipeline.
func (cs *CheckpointStore) Latest(pipelineID string) (*Checkpoint, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	cps, exists := cs.checkpoints[pipelineID]
	if !exists || len(cps) == 0 {
		return nil, false
	}
	cp := cps[len(cps)-1]
	return &cp, true
}

// List returns all checkpoints for a pipeline.
func (cs *CheckpointStore) List(pipelineID string) []Checkpoint {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	cps := cs.checkpoints[pipelineID]
	out := make([]Checkpoint, len(cps))
	copy(out, cps)
	return out
}

// CreateCheckpoint snapshots the current state of a pipeline.
func (e *Engine) CreateCheckpoint(pipelineID string) (*Checkpoint, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	p, exists := e.pipelines[pipelineID]
	if !exists {
		return nil, ErrPipelineNotFound
	}

	cp := &Checkpoint{
		PipelineID: pipelineID,
		Sequence:   p.eventsIn,
		CreatedAt:  time.Now(),
		Windows:    make(map[string]WindowCheckpoint),
	}

	if p.watermark != nil {
		cp.Watermark = p.watermark.Current()
	}

	for key, ws := range p.windows {
		cp.Windows[key] = WindowCheckpoint{
			Start:   ws.start,
			End:     ws.end,
			Count:   ws.count,
			Sum:     ws.sum,
			Min:     ws.min,
			Max:     ws.max,
			First:   ws.first,
			Last:    ws.last,
			HasData: ws.hasData,
		}
	}

	if e.checkpoints != nil {
		e.checkpoints.Save(*cp)
	}

	return cp, nil
}

// RestoreFromCheckpoint restores pipeline state from a checkpoint.
func (e *Engine) RestoreFromCheckpoint(pipelineID string) error {
	if e.checkpoints == nil {
		return ErrCheckpointNotFound
	}

	cp, exists := e.checkpoints.Latest(pipelineID)
	if !exists {
		return ErrCheckpointNotFound
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	p, pExists := e.pipelines[pipelineID]
	if !pExists {
		return ErrPipelineNotFound
	}

	// Restore watermark
	if p.watermark != nil {
		p.watermark.mu.Lock()
		p.watermark.current = cp.Watermark
		p.watermark.mu.Unlock()
	}

	// Restore window state
	p.windows = make(map[string]*windowState)
	for key, wcp := range cp.Windows {
		p.windows[key] = &windowState{
			start:   wcp.Start,
			end:     wcp.End,
			count:   wcp.Count,
			sum:     wcp.Sum,
			min:     wcp.Min,
			max:     wcp.Max,
			first:   wcp.First,
			last:    wcp.Last,
			hasData: wcp.HasData,
		}
	}

	p.eventsIn = cp.Sequence
	return nil
}

// GetCheckpoints returns all checkpoints for a pipeline.
func (e *Engine) GetCheckpoints(pipelineID string) []Checkpoint {
	if e.checkpoints == nil {
		return nil
	}
	return e.checkpoints.List(pipelineID)
}
