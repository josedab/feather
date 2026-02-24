package backfillengine

import (
	"context"
	"errors"
	"time"
)

// Errors returned by the backfill engine.
var (
	ErrSourceNotFound     = errors.New("backfillengine: source not found")
	ErrSourceExists       = errors.New("backfillengine: source already registered")
	ErrJobNotFound        = errors.New("backfillengine: job not found")
	ErrJobAlreadyRunning  = errors.New("backfillengine: job already running")
	ErrJobNotRunning      = errors.New("backfillengine: job not running")
	ErrInvalidTimeRange   = errors.New("backfillengine: invalid time range")
	ErrCheckpointFailed   = errors.New("backfillengine: checkpoint failed")
	ErrDuplicateEvent     = errors.New("backfillengine: duplicate event detected")
	ErrMaterializeFailed  = errors.New("backfillengine: materialize failed")
)

// SourceType identifies the kind of streaming source.
type SourceType string

const (
	SourceTypeKafka SourceType = "kafka"
	SourceTypeFlink SourceType = "flink"
	SourceTypeFile  SourceType = "file"
)

// JobStatus tracks the state of a backfill job.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusPaused    JobStatus = "paused"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// Event represents a single feature event from a streaming source.
type Event struct {
	ID        string                 `json:"id"`
	EntityKey string                 `json:"entity_key"`
	Features  map[string]interface{} `json:"features"`
	Timestamp time.Time              `json:"timestamp"`
	Offset    int64                  `json:"offset"`
	Partition int                    `json:"partition"`
	Source    string                 `json:"source"`
}

// Source is the abstraction layer for streaming data sources.
type Source interface {
	// Type returns the source type identifier.
	Type() SourceType
	// Connect establishes a connection to the source.
	Connect(ctx context.Context) error
	// ReadBatch reads a batch of events starting from the given offset.
	ReadBatch(ctx context.Context, fromOffset int64, batchSize int) ([]Event, error)
	// SeekToTimestamp moves the read position to events at or after the given time.
	SeekToTimestamp(ctx context.Context, ts time.Time) (int64, error)
	// LatestOffset returns the most recent offset in the source.
	LatestOffset(ctx context.Context) (int64, error)
	// Close releases source resources.
	Close() error
}

// FeatureWriter writes backfilled features to the storage tier.
type FeatureWriter interface {
	WriteFeature(ctx context.Context, entityKey string, featureName string, value interface{}, timestamp time.Time) error
	Flush(ctx context.Context) error
}

// Checkpoint stores the progress of a backfill job for resumability.
type Checkpoint struct {
	JobID           string    `json:"job_id"`
	LastOffset      int64     `json:"last_offset"`
	LastTimestamp   time.Time `json:"last_timestamp"`
	EventsProcessed int64    `json:"events_processed"`
	EventsSkipped   int64    `json:"events_skipped"`
	CreatedAt       time.Time `json:"created_at"`
}

// JobRequest describes a new backfill job to create.
type JobRequest struct {
	SourceName  string    `json:"source_name"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	BatchSize   int       `json:"batch_size,omitempty"`
	Parallelism int       `json:"parallelism,omitempty"`
	Features    []string  `json:"features,omitempty"`
}

// Job represents a backfill job with its current state and progress.
type Job struct {
	ID              string     `json:"id"`
	SourceName      string     `json:"source_name"`
	Status          JobStatus  `json:"status"`
	StartTime       time.Time  `json:"start_time"`
	EndTime         time.Time  `json:"end_time"`
	BatchSize       int        `json:"batch_size"`
	Parallelism     int        `json:"parallelism"`
	Features        []string   `json:"features,omitempty"`
	EventsProcessed int64      `json:"events_processed"`
	EventsSkipped   int64      `json:"events_skipped"`
	ErrorCount      int64      `json:"error_count"`
	RetryCount      int64      `json:"retry_count"`
	LastError       string     `json:"last_error,omitempty"`
	Watermark       *time.Time `json:"watermark,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

// Progress returns the estimated progress percentage (0-100).
func (j *Job) Progress() float64 {
	if j.Status == JobStatusCompleted {
		return 100
	}
	if j.Watermark == nil || j.StartTime.Equal(j.EndTime) {
		return 0
	}
	total := j.EndTime.Sub(j.StartTime).Seconds()
	if total <= 0 {
		return 0
	}
	elapsed := j.Watermark.Sub(j.StartTime).Seconds()
	pct := (elapsed / total) * 100
	if pct > 100 {
		pct = 100
	}
	return pct
}

// CoordinatorStats provides aggregate statistics for the backfill coordinator.
type CoordinatorStats struct {
	TotalJobs       int   `json:"total_jobs"`
	RunningJobs     int   `json:"running_jobs"`
	CompletedJobs   int   `json:"completed_jobs"`
	FailedJobs      int   `json:"failed_jobs"`
	TotalEvents     int64 `json:"total_events"`
	DuplicatesFound int64 `json:"duplicates_found"`
}
