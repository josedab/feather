// Package offlinesync provides synchronization between offline batch processing
// and Feather's online feature store.
//
// This package coordinates the materialization of features computed by batch
// processing frameworks (Spark, Flink) into Feather's online store with
// versioning, scheduling, and automatic dependency resolution.
//
// # Usage
//
//	engine, err := offlinesync.NewEngine(config, store, schema, logger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer engine.Close()
//
//	// Create a materialization job
//	job, err := engine.CreateJob(&offlinesync.JobSpec{
//	    Name:     "daily_user_features",
//	    Source:   "/data/features/daily",
//	    Schedule: "0 2 * * *",
//	})
package offlinesync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/storage"
)

// Errors returned by the sync engine.
var (
	ErrEngineNotRunning   = errors.New("engine not running")
	ErrJobNotFound        = errors.New("job not found")
	ErrJobAlreadyExists   = errors.New("job already exists")
	ErrInvalidJobSpec     = errors.New("invalid job specification")
	ErrSyncFailed         = errors.New("sync failed")
	ErrVersionConflict    = errors.New("version conflict")
	ErrSourceNotFound     = errors.New("source not found")
	ErrDependencyNotMet   = errors.New("dependency not met")
	ErrScheduleInvalid    = errors.New("invalid schedule")
)

// JobStatus represents the status of a sync job.
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusScheduled  JobStatus = "scheduled"
	JobStatusRunning    JobStatus = "running"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCancelled  JobStatus = "cancelled"
	JobStatusRetrying   JobStatus = "retrying"
)

// SourceType identifies the source data format.
type SourceType string

const (
	SourceTypeParquet SourceType = "parquet"
	SourceTypeJSON    SourceType = "json"
	SourceTypeCSV     SourceType = "csv"
	SourceTypeDelta   SourceType = "delta"
	SourceTypeIceberg SourceType = "iceberg"
	SourceTypeHudi    SourceType = "hudi"
)

// SyncStrategy determines how updates are applied.
type SyncStrategy string

const (
	SyncStrategyReplace    SyncStrategy = "replace"     // Replace all features
	SyncStrategyMerge      SyncStrategy = "merge"       // Merge with existing
	SyncStrategyAppend     SyncStrategy = "append"      // Append only (no updates)
	SyncStrategyIncremental SyncStrategy = "incremental" // Only sync changes
)

// Config contains configuration for the sync engine.
type Config struct {
	// WorkDir is the working directory for temporary files.
	WorkDir string `json:"work_dir" yaml:"work_dir"`

	// MaxConcurrentJobs is the maximum number of concurrent sync jobs.
	MaxConcurrentJobs int `json:"max_concurrent_jobs" yaml:"max_concurrent_jobs"`

	// DefaultBatchSize is the default batch size for sync operations.
	DefaultBatchSize int `json:"default_batch_size" yaml:"default_batch_size"`

	// DefaultTimeout is the default timeout for sync operations.
	DefaultTimeout time.Duration `json:"default_timeout" yaml:"default_timeout"`

	// RetryAttempts is the number of retry attempts for failed syncs.
	RetryAttempts int `json:"retry_attempts" yaml:"retry_attempts"`

	// RetryBackoff is the initial backoff between retries.
	RetryBackoff time.Duration `json:"retry_backoff" yaml:"retry_backoff"`

	// EnableVersioning enables feature versioning.
	EnableVersioning bool `json:"enable_versioning" yaml:"enable_versioning"`

	// VersionRetention is how long to retain old versions.
	VersionRetention time.Duration `json:"version_retention" yaml:"version_retention"`

	// EnableNotifications enables job status notifications.
	EnableNotifications bool `json:"enable_notifications" yaml:"enable_notifications"`

	// NotificationWebhook is the webhook URL for notifications.
	NotificationWebhook string `json:"notification_webhook" yaml:"notification_webhook"`
}

// DefaultConfig returns the default sync engine configuration.
func DefaultConfig() Config {
	return Config{
		WorkDir:           os.TempDir(),
		MaxConcurrentJobs: 4,
		DefaultBatchSize:  10000,
		DefaultTimeout:    30 * time.Minute,
		RetryAttempts:     3,
		RetryBackoff:      time.Second,
		EnableVersioning:  true,
		VersionRetention:  7 * 24 * time.Hour,
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.WorkDir == "" {
		c.WorkDir = os.TempDir()
	}
	if c.MaxConcurrentJobs <= 0 {
		c.MaxConcurrentJobs = 4
	}
	if c.DefaultBatchSize <= 0 {
		c.DefaultBatchSize = 10000
	}
	if c.DefaultTimeout <= 0 {
		c.DefaultTimeout = 30 * time.Minute
	}
	return nil
}

// JobSpec defines a sync job specification.
type JobSpec struct {
	// ID is the unique job identifier.
	ID string `json:"id"`

	// Name is the human-readable job name.
	Name string `json:"name"`

	// Description is the job description.
	Description string `json:"description,omitempty"`

	// Source is the source data location.
	Source string `json:"source"`

	// SourceType is the source data format.
	SourceType SourceType `json:"source_type"`

	// EntityColumn is the column containing entity keys.
	EntityColumn string `json:"entity_column"`

	// TimestampColumn is the column containing timestamps.
	TimestampColumn string `json:"timestamp_column,omitempty"`

	// FeatureColumns maps source columns to feature names.
	FeatureColumns map[string]string `json:"feature_columns"`

	// Features to sync (derived from FeatureColumns if empty).
	Features []string `json:"features,omitempty"`

	// Schedule is the cron expression for scheduled execution.
	Schedule string `json:"schedule,omitempty"`

	// Strategy determines how updates are applied.
	Strategy SyncStrategy `json:"strategy"`

	// Dependencies lists jobs that must complete before this one.
	Dependencies []string `json:"dependencies,omitempty"`

	// Priority is the job priority (higher = more important).
	Priority int `json:"priority"`

	// Timeout is the maximum execution time.
	Timeout time.Duration `json:"timeout,omitempty"`

	// BatchSize is the batch size for this job.
	BatchSize int `json:"batch_size,omitempty"`

	// ValidateSchema validates data against schema.
	ValidateSchema bool `json:"validate_schema"`

	// Tags for job organization.
	Tags []string `json:"tags,omitempty"`

	// Owner is the job owner.
	Owner string `json:"owner,omitempty"`
}

// Job represents a sync job instance.
type Job struct {
	mu sync.RWMutex

	// Spec is the job specification.
	Spec JobSpec `json:"spec"`

	// Status is the current job status.
	Status JobStatus `json:"status"`

	// Progress tracks sync progress.
	Progress JobProgress `json:"progress"`

	// Version is the current sync version.
	Version int64 `json:"version"`

	// CreatedAt is when the job was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the job was last updated.
	UpdatedAt time.Time `json:"updated_at"`

	// StartedAt is when the job started.
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt is when the job completed.
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// NextRunAt is the next scheduled run time.
	NextRunAt *time.Time `json:"next_run_at,omitempty"`

	// LastRunAt is the last execution time.
	LastRunAt *time.Time `json:"last_run_at,omitempty"`

	// Error contains the error message if failed.
	Error string `json:"error,omitempty"`

	// RetryCount is the number of retries attempted.
	RetryCount int `json:"retry_count"`

	// History contains execution history.
	History []JobExecution `json:"history,omitempty"`
}

// JobProgress tracks sync progress.
type JobProgress struct {
	TotalRecords     int64     `json:"total_records"`
	ProcessedRecords int64     `json:"processed_records"`
	FailedRecords    int64     `json:"failed_records"`
	SkippedRecords   int64     `json:"skipped_records"`
	Percentage       float64   `json:"percentage"`
	RecordsPerSecond float64   `json:"records_per_second"`
	EstimatedETA     time.Time `json:"estimated_eta,omitempty"`
	CurrentBatch     int       `json:"current_batch"`
	TotalBatches     int       `json:"total_batches"`
}

// JobExecution represents a single job execution.
type JobExecution struct {
	ID            string        `json:"id"`
	StartedAt     time.Time     `json:"started_at"`
	CompletedAt   time.Time     `json:"completed_at"`
	Duration      time.Duration `json:"duration"`
	Status        JobStatus     `json:"status"`
	RecordsSync   int64         `json:"records_synced"`
	Error         string        `json:"error,omitempty"`
	Version       int64         `json:"version"`
}

// EngineMetrics tracks engine performance.
type EngineMetrics struct {
	JobsCreated     int64         `json:"jobs_created"`
	JobsCompleted   int64         `json:"jobs_completed"`
	JobsFailed      int64         `json:"jobs_failed"`
	RecordsSynced   int64         `json:"records_synced"`
	BytesProcessed  int64         `json:"bytes_processed"`
	ActiveJobs      int           `json:"active_jobs"`
	AverageDuration time.Duration `json:"average_duration"`
}

// Engine manages offline-to-online sync operations.
type Engine struct {
	mu     sync.RWMutex
	config Config
	store  *storage.Store
	schema storage.SchemaRegistry
	logger *slog.Logger

	// Job management
	jobs    map[string]*Job
	running map[string]context.CancelFunc

	// Metrics
	metrics EngineMetrics

	// State
	started   bool
	stopCh    chan struct{}
	stoppedCh chan struct{}

	// Concurrency control
	semaphore chan struct{}
}

// NewEngine creates a new sync engine.
func NewEngine(config Config, store *storage.Store, schema storage.SchemaRegistry, logger *slog.Logger) (*Engine, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	if store == nil {
		return nil, fmt.Errorf("%w: store is required", ErrInvalidJobSpec)
	}

	if logger == nil {
		logger = slog.Default()
	}

	// Ensure work directory exists
	if err := os.MkdirAll(config.WorkDir, 0755); err != nil {
		return nil, fmt.Errorf("creating work directory: %w", err)
	}

	return &Engine{
		config:    config,
		store:     store,
		schema:    schema,
		logger:    logger,
		jobs:      make(map[string]*Job),
		running:   make(map[string]context.CancelFunc),
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
		semaphore: make(chan struct{}, config.MaxConcurrentJobs),
	}, nil
}

// Start starts the sync engine.
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.started {
		e.mu.Unlock()
		return nil
	}
	e.started = true
	e.mu.Unlock()

	// Start scheduler loop
	go e.schedulerLoop(ctx)

	e.logger.Info("offline sync engine started",
		"max_concurrent_jobs", e.config.MaxConcurrentJobs,
	)

	return nil
}

// Stop stops the sync engine.
func (e *Engine) Stop() error {
	e.mu.Lock()
	if !e.started {
		e.mu.Unlock()
		return nil
	}
	e.started = false

	// Cancel all running jobs
	for id, cancel := range e.running {
		cancel()
		e.logger.Info("cancelled running job", "job_id", id)
	}
	e.mu.Unlock()

	close(e.stopCh)

	// Wait for graceful shutdown
	select {
	case <-e.stoppedCh:
	case <-time.After(30 * time.Second):
		e.logger.Warn("force stopping engine after timeout")
	}

	e.logger.Info("offline sync engine stopped",
		"jobs_completed", e.metrics.JobsCompleted,
		"jobs_failed", e.metrics.JobsFailed,
	)

	return nil
}

// Close closes the engine.
func (e *Engine) Close() error {
	return e.Stop()
}

func (e *Engine) schedulerLoop(ctx context.Context) {
	defer close(e.stoppedCh)

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.checkScheduledJobs(ctx)
		}
	}
}

func (e *Engine) checkScheduledJobs(ctx context.Context) {
	e.mu.RLock()
	jobs := make([]*Job, 0)
	for _, job := range e.jobs {
		if job.NextRunAt != nil && time.Now().After(*job.NextRunAt) {
			jobs = append(jobs, job)
		}
	}
	e.mu.RUnlock()

	for _, job := range jobs {
		go func(j *Job) {
			if _, err := e.RunJob(ctx, j.Spec.ID); err != nil {
				e.logger.Error("scheduled job failed",
					"job_id", j.Spec.ID,
					"error", err,
				)
			}
		}(job)
	}
}

// CreateJob creates a new sync job.
func (e *Engine) CreateJob(spec *JobSpec) (*Job, error) {
	if err := e.validateJobSpec(spec); err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.jobs[spec.ID]; exists {
		return nil, ErrJobAlreadyExists
	}

	now := time.Now()
	job := &Job{
		Spec:      *spec,
		Status:    JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Calculate next run time if scheduled
	if spec.Schedule != "" {
		nextRun := e.calculateNextRun(spec.Schedule)
		job.NextRunAt = &nextRun
		job.Status = JobStatusScheduled
	}

	e.jobs[spec.ID] = job
	atomic.AddInt64(&e.metrics.JobsCreated, 1)

	e.logger.Info("created sync job",
		"job_id", spec.ID,
		"name", spec.Name,
		"source", spec.Source,
	)

	return job, nil
}

func (e *Engine) validateJobSpec(spec *JobSpec) error {
	if spec.ID == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidJobSpec)
	}
	if spec.Source == "" {
		return fmt.Errorf("%w: source is required", ErrInvalidJobSpec)
	}
	if spec.EntityColumn == "" {
		return fmt.Errorf("%w: entity_column is required", ErrInvalidJobSpec)
	}
	if len(spec.FeatureColumns) == 0 && len(spec.Features) == 0 {
		return fmt.Errorf("%w: feature_columns or features is required", ErrInvalidJobSpec)
	}
	if spec.SourceType == "" {
		spec.SourceType = SourceTypeJSON
	}
	if spec.Strategy == "" {
		spec.Strategy = SyncStrategyMerge
	}
	if spec.BatchSize <= 0 {
		spec.BatchSize = e.config.DefaultBatchSize
	}
	if spec.Timeout <= 0 {
		spec.Timeout = e.config.DefaultTimeout
	}
	return nil
}

func (e *Engine) calculateNextRun(schedule string) time.Time {
	// Simple implementation - in production use a proper cron parser
	// For now, just schedule 24 hours from now
	return time.Now().Add(24 * time.Hour)
}

// GetJob returns a job by ID.
func (e *Engine) GetJob(id string) (*Job, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	job, exists := e.jobs[id]
	if !exists {
		return nil, ErrJobNotFound
	}
	return job, nil
}

// ListJobs returns all jobs matching the filter.
func (e *Engine) ListJobs(status *JobStatus) []*Job {
	e.mu.RLock()
	defer e.mu.RUnlock()

	jobs := make([]*Job, 0)
	for _, job := range e.jobs {
		if status == nil || job.Status == *status {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// DeleteJob deletes a job.
func (e *Engine) DeleteJob(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	job, exists := e.jobs[id]
	if !exists {
		return ErrJobNotFound
	}

	if job.Status == JobStatusRunning {
		if cancel, ok := e.running[id]; ok {
			cancel()
		}
	}

	delete(e.jobs, id)
	delete(e.running, id)

	e.logger.Info("deleted sync job", "job_id", id)
	return nil
}

// RunJob executes a sync job.
func (e *Engine) RunJob(ctx context.Context, id string) (*JobExecution, error) {
	e.mu.Lock()
	job, exists := e.jobs[id]
	if !exists {
		e.mu.Unlock()
		return nil, ErrJobNotFound
	}

	if job.Status == JobStatusRunning {
		e.mu.Unlock()
		return nil, fmt.Errorf("job %s is already running", id)
	}

	// Check dependencies
	for _, depID := range job.Spec.Dependencies {
		if depJob, ok := e.jobs[depID]; ok {
			if depJob.Status != JobStatusCompleted {
				e.mu.Unlock()
				return nil, fmt.Errorf("%w: %s", ErrDependencyNotMet, depID)
			}
		}
	}

	// Acquire semaphore
	select {
	case e.semaphore <- struct{}{}:
	default:
		e.mu.Unlock()
		return nil, fmt.Errorf("max concurrent jobs reached")
	}

	// Create cancellable context
	jobCtx, cancel := context.WithTimeout(ctx, job.Spec.Timeout)
	e.running[id] = cancel

	now := time.Now()
	job.Status = JobStatusRunning
	job.StartedAt = &now
	job.UpdatedAt = now
	job.Progress = JobProgress{}
	e.mu.Unlock()

	// Run job
	execution := e.executeJob(jobCtx, job)

	// Release semaphore
	<-e.semaphore

	// Update job state
	e.mu.Lock()
	delete(e.running, id)
	completedAt := time.Now()
	job.CompletedAt = &completedAt
	job.UpdatedAt = completedAt
	job.LastRunAt = &completedAt

	if execution.Status == JobStatusCompleted {
		job.Status = JobStatusCompleted
		job.Error = ""
		job.RetryCount = 0
		atomic.AddInt64(&e.metrics.JobsCompleted, 1)
	} else {
		job.Error = execution.Error
		job.RetryCount++

		if job.RetryCount < e.config.RetryAttempts {
			job.Status = JobStatusRetrying
		} else {
			job.Status = JobStatusFailed
			atomic.AddInt64(&e.metrics.JobsFailed, 1)
		}
	}

	// Update version
	if execution.Status == JobStatusCompleted {
		job.Version++
		execution.Version = job.Version
	}

	// Add to history
	job.History = append(job.History, execution)
	if len(job.History) > 10 {
		job.History = job.History[len(job.History)-10:]
	}

	// Calculate next run
	if job.Spec.Schedule != "" {
		nextRun := e.calculateNextRun(job.Spec.Schedule)
		job.NextRunAt = &nextRun
	}
	e.mu.Unlock()

	e.logger.Info("sync job completed",
		"job_id", id,
		"status", execution.Status,
		"records", execution.RecordsSync,
		"duration", execution.Duration,
	)

	return &execution, nil
}

func (e *Engine) executeJob(ctx context.Context, job *Job) JobExecution {
	start := time.Now()
	execution := JobExecution{
		ID:        fmt.Sprintf("%s-%d", job.Spec.ID, time.Now().UnixNano()),
		StartedAt: start,
		Status:    JobStatusRunning,
	}

	// Verify source exists
	if _, err := os.Stat(job.Spec.Source); err != nil {
		execution.Status = JobStatusFailed
		execution.Error = fmt.Sprintf("source not found: %v", err)
		execution.CompletedAt = time.Now()
		execution.Duration = time.Since(start)
		return execution
	}

	// Process based on source type
	var err error
	var recordsSynced int64

	switch job.Spec.SourceType {
	case SourceTypeJSON:
		recordsSynced, err = e.syncFromJSON(ctx, job)
	case SourceTypeCSV:
		recordsSynced, err = e.syncFromCSV(ctx, job)
	case SourceTypeParquet:
		recordsSynced, err = e.syncFromParquet(ctx, job)
	default:
		recordsSynced, err = e.syncFromJSON(ctx, job)
	}

	execution.RecordsSync = recordsSynced
	execution.CompletedAt = time.Now()
	execution.Duration = time.Since(start)

	if err != nil {
		execution.Status = JobStatusFailed
		execution.Error = err.Error()
	} else {
		execution.Status = JobStatusCompleted
	}

	atomic.AddInt64(&e.metrics.RecordsSynced, recordsSynced)

	return execution
}

func (e *Engine) syncFromJSON(ctx context.Context, job *Job) (int64, error) {
	file, err := os.Open(job.Spec.Source)
	if err != nil {
		return 0, fmt.Errorf("opening source: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	var records []map[string]interface{}

	// Try to decode as array first
	if err := decoder.Decode(&records); err != nil {
		// Try as newline-delimited JSON
		file.Seek(0, 0)
		decoder = json.NewDecoder(file)
		records = make([]map[string]interface{}, 0)

		for decoder.More() {
			var record map[string]interface{}
			if err := decoder.Decode(&record); err != nil {
				break
			}
			records = append(records, record)
		}
	}

	return e.processRecords(ctx, job, records)
}

func (e *Engine) syncFromCSV(ctx context.Context, job *Job) (int64, error) {
	content, err := os.ReadFile(job.Spec.Source)
	if err != nil {
		return 0, fmt.Errorf("reading source: %w", err)
	}

	lines := splitLines(string(content))
	if len(lines) == 0 {
		return 0, nil
	}

	// Parse header
	header := splitCSVLine(lines[0])
	colIndex := make(map[string]int)
	for i, col := range header {
		colIndex[col] = i
	}

	records := make([]map[string]interface{}, 0, len(lines)-1)
	for i := 1; i < len(lines); i++ {
		values := splitCSVLine(lines[i])
		record := make(map[string]interface{})
		for col, idx := range colIndex {
			if idx < len(values) {
				record[col] = values[idx]
			}
		}
		records = append(records, record)
	}

	return e.processRecords(ctx, job, records)
}

func (e *Engine) syncFromParquet(ctx context.Context, job *Job) (int64, error) {
	// For parquet, we read it as JSON format (simplified)
	// In production, use parquet-go library
	return e.syncFromJSON(ctx, job)
}

func (e *Engine) processRecords(ctx context.Context, job *Job, records []map[string]interface{}) (int64, error) {
	if len(records) == 0 {
		return 0, nil
	}

	job.mu.Lock()
	job.Progress.TotalRecords = int64(len(records))
	job.Progress.TotalBatches = (len(records) + job.Spec.BatchSize - 1) / job.Spec.BatchSize
	job.mu.Unlock()

	var recordsSynced int64
	batchStart := time.Now()

	for i := 0; i < len(records); i += job.Spec.BatchSize {
		select {
		case <-ctx.Done():
			return recordsSynced, ctx.Err()
		default:
		}

		end := i + job.Spec.BatchSize
		if end > len(records) {
			end = len(records)
		}

		batch := records[i:end]
		synced, err := e.processBatch(ctx, job, batch)
		if err != nil {
			return recordsSynced, err
		}

		recordsSynced += synced

		// Update progress
		job.mu.Lock()
		job.Progress.ProcessedRecords = int64(end)
		job.Progress.Percentage = float64(end) / float64(len(records)) * 100
		job.Progress.CurrentBatch = i/job.Spec.BatchSize + 1

		elapsed := time.Since(batchStart).Seconds()
		if elapsed > 0 {
			job.Progress.RecordsPerSecond = float64(end) / elapsed
			remaining := float64(len(records) - end)
			if job.Progress.RecordsPerSecond > 0 {
				etaSeconds := remaining / job.Progress.RecordsPerSecond
				job.Progress.EstimatedETA = time.Now().Add(time.Duration(etaSeconds) * time.Second)
			}
		}
		job.mu.Unlock()
	}

	return recordsSynced, nil
}

func (e *Engine) processBatch(ctx context.Context, job *Job, records []map[string]interface{}) (int64, error) {
	var synced int64

	for _, record := range records {
		// Extract entity key
		entityKey, ok := record[job.Spec.EntityColumn].(string)
		if !ok {
			job.mu.Lock()
			job.Progress.SkippedRecords++
			job.mu.Unlock()
			continue
		}

		// Extract timestamp
		var timestamp int64 = time.Now().UnixNano()
		if job.Spec.TimestampColumn != "" {
			if ts, ok := record[job.Spec.TimestampColumn]; ok {
				switch t := ts.(type) {
				case string:
					if parsed, err := time.Parse(time.RFC3339, t); err == nil {
						timestamp = parsed.UnixNano()
					}
				case float64:
					timestamp = int64(t)
				case int64:
					timestamp = t
				}
			}
		}

		// Build feature map
		features := make(map[string]*domain.FeatureValue)

		if len(job.Spec.FeatureColumns) > 0 {
			for srcCol, featureName := range job.Spec.FeatureColumns {
				if val, ok := record[srcCol]; ok && val != nil {
					features[featureName] = &domain.FeatureValue{
						Value:     val,
						Timestamp: timestamp,
					}
				}
			}
		} else {
			for _, featureName := range job.Spec.Features {
				if val, ok := record[featureName]; ok && val != nil {
					features[featureName] = &domain.FeatureValue{
						Value:     val,
						Timestamp: timestamp,
					}
				}
			}
		}

		if len(features) == 0 {
			job.mu.Lock()
			job.Progress.SkippedRecords++
			job.mu.Unlock()
			continue
		}

		// Apply sync strategy
		if job.Spec.Strategy == SyncStrategyMerge {
			existing, err := e.store.Get(entityKey, nil)
			if err == nil && len(existing) > 0 {
				for name, existingVal := range existing {
					if _, hasNew := features[name]; !hasNew {
						features[name] = existingVal
					}
				}
			}
		}

		// Write to store
		if err := e.store.Put(entityKey, features); err != nil {
			job.mu.Lock()
			job.Progress.FailedRecords++
			job.mu.Unlock()
			continue
		}

		synced++
	}

	return synced, nil
}

// CancelJob cancels a running job.
func (e *Engine) CancelJob(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	job, exists := e.jobs[id]
	if !exists {
		return ErrJobNotFound
	}

	if job.Status != JobStatusRunning {
		return fmt.Errorf("job %s is not running", id)
	}

	if cancel, ok := e.running[id]; ok {
		cancel()
		delete(e.running, id)
	}

	job.Status = JobStatusCancelled
	job.UpdatedAt = time.Now()

	e.logger.Info("cancelled sync job", "job_id", id)
	return nil
}

// Metrics returns engine metrics.
func (e *Engine) Metrics() EngineMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return EngineMetrics{
		JobsCreated:   atomic.LoadInt64(&e.metrics.JobsCreated),
		JobsCompleted: atomic.LoadInt64(&e.metrics.JobsCompleted),
		JobsFailed:    atomic.LoadInt64(&e.metrics.JobsFailed),
		RecordsSynced: atomic.LoadInt64(&e.metrics.RecordsSynced),
		BytesProcessed: atomic.LoadInt64(&e.metrics.BytesProcessed),
		ActiveJobs:    len(e.running),
	}
}

// Helper functions

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			if line != "" {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitCSVLine(line string) []string {
	var fields []string
	var field string
	inQuotes := false

	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '"' {
			inQuotes = !inQuotes
		} else if c == ',' && !inQuotes {
			fields = append(fields, field)
			field = ""
		} else {
			field += string(c)
		}
	}
	fields = append(fields, field)
	return fields
}

// ManifestFile represents a sync manifest for tracking versions.
type ManifestFile struct {
	Version     int64                  `json:"version"`
	CreatedAt   time.Time              `json:"created_at"`
	JobID       string                 `json:"job_id"`
	Features    []string               `json:"features"`
	RecordCount int64                  `json:"record_count"`
	Checksum    string                 `json:"checksum,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// WriteManifest writes a sync manifest file.
func (e *Engine) WriteManifest(path string, manifest *ManifestFile) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	manifestPath := filepath.Join(path, "_manifest.json")
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}

// ReadManifest reads a sync manifest file.
func (e *Engine) ReadManifest(path string) (*ManifestFile, error) {
	manifestPath := filepath.Join(path, "_manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var manifest ManifestFile
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshaling manifest: %w", err)
	}

	return &manifest, nil
}
