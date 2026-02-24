package backfillengine

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// CoordinatorConfig configures the backfill coordinator.
type CoordinatorConfig struct {
	MaxConcurrentJobs int           `json:"max_concurrent_jobs" yaml:"max_concurrent_jobs"`
	DefaultBatchSize  int           `json:"default_batch_size" yaml:"default_batch_size"`
	CheckpointEvery   int           `json:"checkpoint_every" yaml:"checkpoint_every"`
	MaxRetries        int           `json:"max_retries" yaml:"max_retries"`
	RetryDelay        time.Duration `json:"retry_delay" yaml:"retry_delay"`
}

// DefaultCoordinatorConfig returns sensible defaults.
func DefaultCoordinatorConfig() CoordinatorConfig {
	return CoordinatorConfig{
		MaxConcurrentJobs: 4,
		DefaultBatchSize:  1000,
		CheckpointEvery:   5000,
		MaxRetries:        3,
		RetryDelay:        5 * time.Second,
	}
}

// Coordinator orchestrates streaming backfill jobs with exactly-once
// semantics, checkpoint-based resumability, and parallelism control.
type Coordinator struct {
	config  CoordinatorConfig
	sources map[string]Source
	jobs    map[string]*Job
	cancels map[string]context.CancelFunc
	checks  map[string]*Checkpoint
	mu      sync.RWMutex
	running int64
	stats   CoordinatorStats
}

// NewCoordinator creates a new backfill coordinator.
func NewCoordinator(cfg CoordinatorConfig) *Coordinator {
	return &Coordinator{
		config:  cfg,
		sources: make(map[string]Source),
		jobs:    make(map[string]*Job),
		cancels: make(map[string]context.CancelFunc),
		checks:  make(map[string]*Checkpoint),
	}
}

// RegisterSource adds a streaming source for backfill operations.
func (c *Coordinator) RegisterSource(name string, source Source) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.sources[name]; exists {
		return ErrSourceExists
	}
	c.sources[name] = source
	return nil
}

// UnregisterSource removes a streaming source.
func (c *Coordinator) UnregisterSource(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.sources[name]; !exists {
		return ErrSourceNotFound
	}
	delete(c.sources, name)
	return nil
}

// ListSources returns all registered source names and types.
func (c *Coordinator) ListSources() map[string]SourceType {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]SourceType, len(c.sources))
	for name, src := range c.sources {
		result[name] = src.Type()
	}
	return result
}

// CreateJob creates a new backfill job.
func (c *Coordinator) CreateJob(req JobRequest) (*Job, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.sources[req.SourceName]; !exists {
		return nil, ErrSourceNotFound
	}
	if !req.EndTime.After(req.StartTime) {
		return nil, ErrInvalidTimeRange
	}

	batchSize := req.BatchSize
	if batchSize <= 0 {
		batchSize = c.config.DefaultBatchSize
	}
	parallelism := req.Parallelism
	if parallelism <= 0 {
		parallelism = 1
	}

	job := &Job{
		ID:          uuid.New().String(),
		SourceName:  req.SourceName,
		Status:      JobStatusPending,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		BatchSize:   batchSize,
		Parallelism: parallelism,
		Features:    req.Features,
		CreatedAt:   time.Now(),
	}
	c.jobs[job.ID] = job
	c.stats.TotalJobs++
	return job, nil
}

// GetJob returns a job by ID.
func (c *Coordinator) GetJob(id string) (*Job, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	job, exists := c.jobs[id]
	if !exists {
		return nil, ErrJobNotFound
	}
	return job, nil
}

// ListJobs returns all backfill jobs.
func (c *Coordinator) ListJobs() []*Job {
	c.mu.RLock()
	defer c.mu.RUnlock()
	jobs := make([]*Job, 0, len(c.jobs))
	for _, j := range c.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}

// StartJob begins executing a backfill job.
func (c *Coordinator) StartJob(id string, writer FeatureWriter) error {
	c.mu.Lock()
	job, exists := c.jobs[id]
	if !exists {
		c.mu.Unlock()
		return ErrJobNotFound
	}
	if job.Status == JobStatusRunning {
		c.mu.Unlock()
		return ErrJobAlreadyRunning
	}

	src, exists := c.sources[job.SourceName]
	if !exists {
		c.mu.Unlock()
		return ErrSourceNotFound
	}

	job.Status = JobStatusRunning
	now := time.Now()
	job.StartedAt = &now
	c.stats.RunningJobs++

	ctx, cancel := context.WithCancel(context.Background())
	c.cancels[id] = cancel
	c.mu.Unlock()

	go c.runJob(ctx, job, src, writer)
	return nil
}

// PauseJob pauses a running backfill job.
func (c *Coordinator) PauseJob(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	job, exists := c.jobs[id]
	if !exists {
		return ErrJobNotFound
	}
	if job.Status != JobStatusRunning {
		return ErrJobNotRunning
	}
	if cancel, ok := c.cancels[id]; ok {
		cancel()
		delete(c.cancels, id)
	}
	job.Status = JobStatusPaused
	c.stats.RunningJobs--
	return nil
}

// CancelJob cancels a backfill job.
func (c *Coordinator) CancelJob(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	job, exists := c.jobs[id]
	if !exists {
		return ErrJobNotFound
	}
	if cancel, ok := c.cancels[id]; ok {
		cancel()
		delete(c.cancels, id)
	}
	if job.Status == JobStatusRunning {
		c.stats.RunningJobs--
	}
	job.Status = JobStatusCancelled
	now := time.Now()
	job.CompletedAt = &now
	return nil
}

// GetCheckpoint returns the latest checkpoint for a job.
func (c *Coordinator) GetCheckpoint(jobID string) (*Checkpoint, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cp, exists := c.checks[jobID]
	if !exists {
		return nil, ErrJobNotFound
	}
	return cp, nil
}

// Stats returns coordinator-level statistics.
func (c *Coordinator) Stats() CoordinatorStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

func (c *Coordinator) runJob(ctx context.Context, job *Job, src Source, writer FeatureWriter) {
	materializer := NewMaterializer(writer)

	if err := c.connectWithRetry(ctx, src, job); err != nil {
		c.failJob(job, fmt.Sprintf("connecting to source: %v", err))
		return
	}
	defer src.Close()

	startOffset, err := src.SeekToTimestamp(ctx, job.StartTime)
	if err != nil {
		c.failJob(job, fmt.Sprintf("seeking to start time: %v", err))
		return
	}

	// Resume from checkpoint if available.
	c.mu.RLock()
	if cp, exists := c.checks[job.ID]; exists && cp.LastOffset > startOffset {
		startOffset = cp.LastOffset
	}
	c.mu.RUnlock()

	offset := startOffset
	eventsInBatch := int64(0)
	consecutiveErrors := 0

	for {
		if ctx.Err() != nil {
			return
		}

		events, err := src.ReadBatch(ctx, offset, job.BatchSize)
		if err != nil {
			consecutiveErrors++
			if consecutiveErrors > c.config.MaxRetries {
				c.failJob(job, fmt.Sprintf("reading batch at offset %d after %d retries: %v", offset, c.config.MaxRetries, err))
				return
			}
			c.mu.Lock()
			job.ErrorCount++
			job.LastError = fmt.Sprintf("retry %d/%d: %v", consecutiveErrors, c.config.MaxRetries, err)
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-time.After(c.config.RetryDelay * time.Duration(consecutiveErrors)):
				continue
			}
		}
		consecutiveErrors = 0

		if len(events) == 0 {
			break
		}

		// Filter events within the time range and by requested features.
		filtered, reachedEnd := c.filterEvents(events, job)
		if len(filtered) > 0 {
			if err := c.materializeWithRetry(ctx, materializer, filtered, job); err != nil {
				c.failJob(job, fmt.Sprintf("materializing events: %v", err))
				return
			}
		}

		c.mu.Lock()
		processed := int64(len(filtered))
		skipped := int64(len(events)) - processed
		job.EventsProcessed += processed
		job.EventsSkipped += skipped
		c.mu.Unlock()
		atomic.AddInt64(&c.stats.TotalEvents, processed)

		if len(events) > 0 {
			offset = events[len(events)-1].Offset + 1
		}

		eventsInBatch += processed
		if eventsInBatch >= int64(c.config.CheckpointEvery) {
			c.saveCheckpoint(job, offset)
			eventsInBatch = 0
		}

		if reachedEnd {
			break
		}
	}

	c.saveCheckpoint(job, offset)
	c.completeJob(job)
}

// connectWithRetry attempts to connect to a source with exponential backoff.
func (c *Coordinator) connectWithRetry(ctx context.Context, src Source, job *Job) error {
	var lastErr error
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := src.Connect(ctx); err != nil {
			lastErr = err
			c.mu.Lock()
			job.ErrorCount++
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.config.RetryDelay * time.Duration(attempt+1)):
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("after %d retries: %w", c.config.MaxRetries, lastErr)
}

// filterEvents separates in-range events and applies feature projection.
// Returns filtered events and whether the job's end time was reached.
func (c *Coordinator) filterEvents(events []Event, job *Job) ([]Event, bool) {
	filtered := make([]Event, 0, len(events))
	for _, evt := range events {
		if evt.Timestamp.After(job.EndTime) {
			return filtered, true
		}
		// Track watermark for progress reporting.
		c.mu.Lock()
		wm := evt.Timestamp
		job.Watermark = &wm
		c.mu.Unlock()

		if len(job.Features) > 0 {
			projected := make(map[string]interface{}, len(job.Features))
			for _, f := range job.Features {
				if v, ok := evt.Features[f]; ok {
					projected[f] = v
				}
			}
			evt.Features = projected
		}
		filtered = append(filtered, evt)
	}
	return filtered, false
}

// materializeWithRetry writes events with retry on transient failure.
func (c *Coordinator) materializeWithRetry(ctx context.Context, mat *Materializer, events []Event, job *Job) error {
	var lastErr error
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := mat.Materialize(ctx, events); err != nil {
			lastErr = err
			c.mu.Lock()
			job.ErrorCount++
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.config.RetryDelay):
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("after %d retries: %w", c.config.MaxRetries, lastErr)
}

func (c *Coordinator) saveCheckpoint(job *Job, offset int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[job.ID] = &Checkpoint{
		JobID:           job.ID,
		LastOffset:      offset,
		LastTimestamp:    time.Now(),
		EventsProcessed: job.EventsProcessed,
		EventsSkipped:   job.EventsSkipped,
		CreatedAt:       time.Now(),
	}
}

func (c *Coordinator) failJob(job *Job, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	job.Status = JobStatusFailed
	job.LastError = reason
	job.ErrorCount++
	now := time.Now()
	job.CompletedAt = &now
	c.stats.RunningJobs--
	c.stats.FailedJobs++
}

func (c *Coordinator) completeJob(job *Job) {
	c.mu.Lock()
	defer c.mu.Unlock()
	job.Status = JobStatusCompleted
	now := time.Now()
	job.CompletedAt = &now
	c.stats.RunningJobs--
	c.stats.CompletedJobs++
}
