package backfill

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// Job represents a backfill job.
type Job struct {
	mu          sync.RWMutex           `json:"-"`
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Source      DataSource             `json:"source"`
	Features    []string               `json:"features"`
	EntityType  string                 `json:"entity_type"`
	StartTime   time.Time              `json:"start_time"` // Historical start
	EndTime     time.Time              `json:"end_time"`   // Historical end
	Status      JobStatus              `json:"status"`
	Progress    JobProgress            `json:"progress"`
	Config      JobConfig              `json:"config"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	CreatedBy   string                 `json:"created_by"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// DataSource represents a source for backfill data.
type DataSource struct {
	Type    string                 `json:"type"`   // file, database, http, s3
	URI     string                 `json:"uri"`    // Connection string or path
	Format  string                 `json:"format"` // csv, json, parquet
	Options map[string]interface{} `json:"options"`
	Mapping FieldMapping           `json:"mapping"`
}

// FieldMapping maps source fields to feature fields.
type FieldMapping struct {
	EntityIDField  string            `json:"entity_id_field"`
	TimestampField string            `json:"timestamp_field"`
	FeatureFields  map[string]string `json:"feature_fields"` // feature name -> source field
}

// JobStatus represents the status of a backfill job.
type JobStatus string

const (
	// StatusPending indicates a queued job.
	StatusPending JobStatus = "pending"
	// StatusRunning indicates a currently running job.
	StatusRunning JobStatus = "running"
	// StatusPaused indicates a paused job.
	StatusPaused JobStatus = "paused"
	// StatusCompleted indicates a completed job.
	StatusCompleted JobStatus = "completed"
	// StatusFailed indicates a failed job.
	StatusFailed JobStatus = "failed"
	// StatusCancelled indicates a canceled job.
	StatusCancelled JobStatus = "cancelled" //nolint:misspell
)

// JobProgress tracks job progress.
type JobProgress struct {
	TotalRecords     int64     `json:"total_records"`
	ProcessedRecords int64     `json:"processed_records"`
	FailedRecords    int64     `json:"failed_records"`
	SkippedRecords   int64     `json:"skipped_records"`
	Percentage       float64   `json:"percentage"`
	CurrentTime      time.Time `json:"current_time"` // Current position in time range
	RecordsPerSec    float64   `json:"records_per_sec"`
	EstimatedETA     time.Time `json:"estimated_eta"`
	LastCheckpoint   time.Time `json:"last_checkpoint"`
}

// JobConfig configures job execution.
type JobConfig struct {
	BatchSize       int           `json:"batch_size"`
	Parallelism     int           `json:"parallelism"`
	RetryAttempts   int           `json:"retry_attempts"`
	RetryDelay      time.Duration `json:"retry_delay"`
	CheckpointEvery int           `json:"checkpoint_every"` // Records between checkpoints
	DryRun          bool          `json:"dry_run"`
	ValidateSchema  bool          `json:"validate_schema"`
	OnConflict      string        `json:"on_conflict"` // skip, overwrite, error
}

// DefaultJobConfig returns default job configuration.
func DefaultJobConfig() JobConfig {
	return JobConfig{
		BatchSize:       1000,
		Parallelism:     4,
		RetryAttempts:   3,
		RetryDelay:      time.Second,
		CheckpointEvery: 10000,
		DryRun:          false,
		ValidateSchema:  true,
		OnConflict:      "overwrite",
	}
}

// Manager manages backfill jobs.
type Manager struct {
	jobs        map[string]*Job
	running     map[string]context.CancelFunc
	checkpoints map[string]*Checkpoint
	writer      FeatureWriter
	mu          sync.RWMutex
}

// FeatureWriter interface for writing features during backfill.
type FeatureWriter interface {
	WriteFeature(ctx context.Context, entityID string, feature string, value interface{}, timestamp time.Time) error
	WriteBatch(ctx context.Context, records []FeatureRecord) error
}

// FeatureRecord represents a single feature record.
type FeatureRecord struct {
	EntityID  string      `json:"entity_id"`
	Feature   string      `json:"feature"`
	Value     interface{} `json:"value"`
	Timestamp time.Time   `json:"timestamp"`
}

// Checkpoint stores job checkpoint for resumability.
type Checkpoint struct {
	JobID            string    `json:"job_id"`
	ProcessedRecords int64     `json:"processed_records"`
	LastTimestamp    time.Time `json:"last_timestamp"`
	LastEntityID     string    `json:"last_entity_id"`
	CreatedAt        time.Time `json:"created_at"`
}

// NewManager creates a new backfill manager.
func NewManager(writer FeatureWriter) *Manager {
	return &Manager{
		jobs:        make(map[string]*Job),
		running:     make(map[string]context.CancelFunc),
		checkpoints: make(map[string]*Checkpoint),
		writer:      writer,
	}
}

// CreateJob creates a new backfill job.
func (m *Manager) CreateJob(job *Job) error {
	if job.ID == "" {
		return ErrJobIDRequired
	}
	if len(job.Features) == 0 {
		return ErrFeaturesRequired
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.jobs[job.ID]; ok {
		return ErrJobExists
	}

	job.Status = StatusPending
	job.CreatedAt = time.Now()
	job.Progress = JobProgress{}

	if job.Config.BatchSize == 0 {
		job.Config = DefaultJobConfig()
	}

	m.jobs[job.ID] = job
	return nil
}

// GetJob retrieves a job by ID.
func (m *Manager) GetJob(id string) *Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.jobs[id]
}

// ListJobs lists all jobs with optional status filter.
func (m *Manager) ListJobs(status JobStatus) []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var jobs []*Job
	for _, job := range m.jobs {
		job.mu.RLock()
		jobStatus := job.Status
		job.mu.RUnlock()
		if status == "" || jobStatus == status {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// StartJob starts a backfill job.
func (m *Manager) StartJob(ctx context.Context, id string) error {
	m.mu.Lock()
	job, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return ErrJobNotFound
	}

	job.mu.Lock()
	if job.Status == StatusRunning {
		job.mu.Unlock()
		m.mu.Unlock()
		return ErrJobAlreadyRunning
	}

	// Create cancellable context
	jobCtx, cancel := context.WithCancel(ctx)
	m.running[id] = cancel

	job.Status = StatusRunning
	now := time.Now()
	job.StartedAt = &now
	job.mu.Unlock()
	m.mu.Unlock()

	// Run job in background
	go m.runJob(jobCtx, job)

	return nil
}

// PauseJob pauses a running job.
func (m *Manager) PauseJob(id string) error {
	m.mu.Lock()
	job, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return ErrJobNotFound
	}

	job.mu.Lock()
	if job.Status != StatusRunning {
		job.mu.Unlock()
		m.mu.Unlock()
		return ErrJobNotRunning
	}

	// Cancel the job context
	if cancel, ok := m.running[id]; ok {
		cancel()
		delete(m.running, id)
	}

	job.Status = StatusPaused
	job.mu.Unlock()
	m.mu.Unlock()
	return nil
}

// ResumeJob resumes a paused job.
func (m *Manager) ResumeJob(ctx context.Context, id string) error {
	m.mu.Lock()
	job, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return ErrJobNotFound
	}

	job.mu.Lock()
	if job.Status != StatusPaused {
		job.mu.Unlock()
		m.mu.Unlock()
		return ErrJobNotPaused
	}

	// Create new cancellable context
	jobCtx, cancel := context.WithCancel(ctx)
	m.running[id] = cancel

	job.Status = StatusRunning
	job.mu.Unlock()
	m.mu.Unlock()

	// Resume from checkpoint
	go m.runJob(jobCtx, job)

	return nil
}

// CancelJob cancels a job.
func (m *Manager) CancelJob(id string) error {
	m.mu.Lock()
	job, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return ErrJobNotFound
	}

	// Cancel if running
	if cancel, ok := m.running[id]; ok {
		cancel()
		delete(m.running, id)
	}
	m.mu.Unlock()

	job.mu.Lock()
	job.Status = StatusCancelled
	job.mu.Unlock()
	return nil
}

// DeleteJob deletes a job.
func (m *Manager) DeleteJob(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[id]
	if !ok {
		return ErrJobNotFound
	}

	job.mu.RLock()
	status := job.Status
	job.mu.RUnlock()

	if status == StatusRunning {
		return ErrCannotDeleteRunning
	}

	// Cancel if still running
	if cancel, ok := m.running[id]; ok {
		cancel()
		delete(m.running, id)
	}

	delete(m.jobs, id)
	delete(m.checkpoints, id)
	return nil
}

func (m *Manager) runJob(ctx context.Context, job *Job) {
	defer func() {
		m.mu.Lock()
		delete(m.running, job.ID)
		m.mu.Unlock()
	}()

	// Get checkpoint if resuming
	m.mu.RLock()
	checkpoint := m.checkpoints[job.ID]
	m.mu.RUnlock()

	startTime := job.StartTime
	if checkpoint != nil {
		startTime = checkpoint.LastTimestamp
		job.mu.Lock()
		job.Progress.ProcessedRecords = checkpoint.ProcessedRecords
		job.mu.Unlock()
	}

	// Create data reader based on source type
	reader, err := m.createReader(job.Source)
	if err != nil {
		m.failJob(job, err)
		return
	}

	// Process in batches
	batch := make([]FeatureRecord, 0, job.Config.BatchSize)
	processedInCheckpoint := 0
	startProcessTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			// Save checkpoint
			m.saveCheckpoint(job)
			return
		default:
		}

		// Read next record
		record, err := reader.Read(ctx, startTime, job.EndTime)
		if err != nil {
			if errors.Is(err, ErrEndOfData) {
				// Process remaining batch
				if len(batch) > 0 {
					if processErr := m.processBatch(ctx, job, batch); processErr != nil {
						m.failJob(job, processErr)
						return
					}
				}
				m.completeJob(job)
				return
			}
			m.failJob(job, err)
			return
		}

		// Map record to features
		for _, feature := range job.Features {
			sourceField := job.Source.Mapping.FeatureFields[feature]
			if sourceField == "" {
				sourceField = feature
			}

			value := record[sourceField]
			if value == nil {
				continue
			}

			entityID, ok := record[job.Source.Mapping.EntityIDField].(string)
			if !ok {
				continue
			}
			timestamp, ok := record[job.Source.Mapping.TimestampField].(time.Time)
			if !ok {
				continue
			}
			batch = append(batch, FeatureRecord{
				EntityID:  entityID,
				Feature:   feature,
				Value:     value,
				Timestamp: timestamp,
			})
		}

		// Process batch if full
		if len(batch) >= job.Config.BatchSize {
			if !job.Config.DryRun {
				if err := m.processBatch(ctx, job, batch); err != nil {
					m.failJob(job, err)
					return
				}
			}

			job.mu.Lock()
			job.Progress.ProcessedRecords += int64(len(batch))
			processedInCheckpoint += len(batch)

			// Update progress stats
			elapsed := time.Since(startProcessTime).Seconds()
			if elapsed > 0 {
				job.Progress.RecordsPerSec = float64(job.Progress.ProcessedRecords) / elapsed
			}
			job.mu.Unlock()
			batch = batch[:0]

			// Checkpoint if needed
			if processedInCheckpoint >= job.Config.CheckpointEvery {
				m.saveCheckpoint(job)
				processedInCheckpoint = 0
			}
		}
	}
}

func (m *Manager) processBatch(ctx context.Context, job *Job, batch []FeatureRecord) error {
	return m.writer.WriteBatch(ctx, batch)
}

func (m *Manager) saveCheckpoint(job *Job) {
	job.mu.RLock()
	processedRecords := job.Progress.ProcessedRecords
	currentTime := job.Progress.CurrentTime
	job.mu.RUnlock()

	m.mu.Lock()
	m.checkpoints[job.ID] = &Checkpoint{
		JobID:            job.ID,
		ProcessedRecords: processedRecords,
		LastTimestamp:    currentTime,
		CreatedAt:        time.Now(),
	}
	m.mu.Unlock()

	job.mu.Lock()
	job.Progress.LastCheckpoint = time.Now()
	job.mu.Unlock()
}

func (m *Manager) failJob(job *Job, err error) {
	job.mu.Lock()
	defer job.mu.Unlock()

	job.Status = StatusFailed
	job.Error = err.Error()
	now := time.Now()
	job.CompletedAt = &now
}

func (m *Manager) completeJob(job *Job) {
	job.mu.Lock()
	defer job.mu.Unlock()

	job.Status = StatusCompleted
	job.Progress.Percentage = 100
	now := time.Now()
	job.CompletedAt = &now
}

// DataReader interface for reading backfill data.
type DataReader interface {
	Read(ctx context.Context, from, to time.Time) (map[string]interface{}, error)
	Close() error
}

func (m *Manager) createReader(source DataSource) (DataReader, error) {
	switch source.Type {
	case "mock":
		return &mockReader{}, nil
	default:
		return nil, ErrUnsupportedSource
	}
}

// mockReader for testing
type mockReader struct {
	count int
}

func (r *mockReader) Read(ctx context.Context, from, to time.Time) (map[string]interface{}, error) {
	if r.count >= 100 {
		return nil, ErrEndOfData
	}
	r.count++
	return map[string]interface{}{
		"entity_id": "entity_" + string(rune('0'+r.count)),
		"timestamp": from.Add(time.Duration(r.count) * time.Hour),
		"value":     float64(r.count),
	}, nil
}

func (r *mockReader) Close() error {
	return nil
}

// GetCheckpoint retrieves the checkpoint for a job.
func (m *Manager) GetCheckpoint(jobID string) *Checkpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.checkpoints[jobID]
}

// GetStats returns backfill statistics.
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := Stats{
		TotalJobs: len(m.jobs),
		ByStatus:  make(map[JobStatus]int),
	}

	for _, job := range m.jobs {
		job.mu.RLock()
		stats.ByStatus[job.Status]++
		stats.TotalRecordsProcessed += job.Progress.ProcessedRecords
		job.mu.RUnlock()
	}

	return stats
}

// Stats contains backfill statistics.
type Stats struct {
	TotalJobs             int               `json:"total_jobs"`
	ByStatus              map[JobStatus]int `json:"by_status"`
	TotalRecordsProcessed int64             `json:"total_records_processed"`
}

// ExportJob exports job configuration to JSON.
func (m *Manager) ExportJob(id string) ([]byte, error) {
	m.mu.RLock()
	job, ok := m.jobs[id]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrJobNotFound
	}

	job.mu.RLock()
	defer job.mu.RUnlock()
	return json.Marshal(job)
}

// ImportJob imports a job from JSON.
func (m *Manager) ImportJob(data []byte) error {
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return err
	}

	return m.CreateJob(&job)
}

// Errors
var (
	ErrJobIDRequired       = errors.New("job ID is required")
	ErrFeaturesRequired    = errors.New("features are required")
	ErrJobExists           = errors.New("job already exists")
	ErrJobNotFound         = errors.New("job not found")
	ErrJobAlreadyRunning   = errors.New("job is already running")
	ErrJobNotRunning       = errors.New("job is not running")
	ErrJobNotPaused        = errors.New("job is not paused")
	ErrCannotDeleteRunning = errors.New("cannot delete running job")
	ErrUnsupportedSource   = errors.New("unsupported data source type")
	ErrEndOfData           = errors.New("end of data")
)
