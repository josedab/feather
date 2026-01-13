package warehouse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/storage"
)

// SyncConfig configures the synchronization engine.
type SyncConfig struct {
	// SyncInterval is the interval between automatic syncs.
	SyncInterval time.Duration `json:"sync_interval" yaml:"sync_interval"`

	// BatchSize is the number of rows per sync batch.
	BatchSize int `json:"batch_size" yaml:"batch_size"`

	// MaxConcurrency is the maximum concurrent sync operations.
	MaxConcurrency int `json:"max_concurrency" yaml:"max_concurrency"`

	// RetryAttempts is the number of retry attempts for failed operations.
	RetryAttempts int `json:"retry_attempts" yaml:"retry_attempts"`

	// RetryBackoff is the backoff duration between retries.
	RetryBackoff time.Duration `json:"retry_backoff" yaml:"retry_backoff"`

	// ConflictResolution determines how to resolve conflicts.
	ConflictResolution ConflictResolution `json:"conflict_resolution" yaml:"conflict_resolution"`

	// EnableChangeTracking tracks changes since last sync.
	EnableChangeTracking bool `json:"enable_change_tracking" yaml:"enable_change_tracking"`
}

// DefaultSyncConfig returns the default sync configuration.
func DefaultSyncConfig() SyncConfig {
	return SyncConfig{
		SyncInterval:         15 * time.Minute,
		BatchSize:            10000,
		MaxConcurrency:       4,
		RetryAttempts:        3,
		RetryBackoff:         5 * time.Second,
		ConflictResolution:   ConflictResolutionLatest,
		EnableChangeTracking: true,
	}
}

// SyncJob defines a synchronization job.
type SyncJob struct {
	// ID is the unique job identifier.
	ID string `json:"id"`

	// Name is the human-readable job name.
	Name string `json:"name"`

	// Direction is the sync direction.
	Direction SyncDirection `json:"direction"`

	// Mode is the sync mode.
	Mode SyncMode `json:"mode"`

	// Source is the source table (for import) or feature set (for export).
	Source string `json:"source"`

	// Target is the target table (for export) or feature store (for import).
	Target string `json:"target"`

	// Features is the list of features to sync.
	Features []string `json:"features"`

	// EntityColumn is the entity key column (for import).
	EntityColumn string `json:"entity_column,omitempty"`

	// TimestampColumn is the timestamp column.
	TimestampColumn string `json:"timestamp_column,omitempty"`

	// FeatureMapping maps warehouse columns to feature names.
	FeatureMapping map[string]string `json:"feature_mapping,omitempty"`

	// Filter is an optional WHERE clause.
	Filter string `json:"filter,omitempty"`

	// Schedule is a cron expression for scheduled syncs.
	Schedule string `json:"schedule,omitempty"`

	// Enabled indicates if the job is active.
	Enabled bool `json:"enabled"`

	// CreatedAt is when the job was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the job was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// SyncStatus represents the status of a sync operation.
type SyncStatus string

const (
	// SyncStatusPending indicates a sync is queued but not running.
	SyncStatusPending SyncStatus = "pending"
	// SyncStatusRunning indicates a sync is in progress.
	SyncStatusRunning SyncStatus = "running"
	// SyncStatusCompleted indicates a sync finished successfully.
	SyncStatusCompleted SyncStatus = "completed"
	// SyncStatusFailed indicates a sync finished with errors.
	SyncStatusFailed SyncStatus = "failed"
	// SyncStatusCanceled indicates a sync was canceled.
	SyncStatusCanceled SyncStatus = "canceled"
)

// SyncExecution represents a single sync execution.
type SyncExecution struct {
	// ID is the unique execution identifier.
	ID string `json:"id"`

	// JobID is the associated job ID.
	JobID string `json:"job_id"`

	// Status is the current execution status.
	Status SyncStatus `json:"status"`

	// StartedAt is when the execution started.
	StartedAt time.Time `json:"started_at"`

	// CompletedAt is when the execution completed.
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// RowsSynced is the number of rows processed.
	RowsSynced int64 `json:"rows_synced"`

	// RowsFailed is the number of failed rows.
	RowsFailed int64 `json:"rows_failed"`

	// BytesTransferred is the total bytes transferred.
	BytesTransferred int64 `json:"bytes_transferred"`

	// Error is the error message if failed.
	Error string `json:"error,omitempty"`

	// Checkpoint is the last processed position for resume.
	Checkpoint string `json:"checkpoint,omitempty"`
}

// SyncEngine manages synchronization between Feather and warehouses.
type SyncEngine struct {
	mu         sync.RWMutex
	config     SyncConfig
	connectors map[string]Connector
	jobs       map[string]*SyncJob
	executions map[string]*SyncExecution
	store      *storage.Store
	schema     storage.SchemaRegistry
	logger     *slog.Logger

	// Metrics
	syncCount     int64
	syncErrors    int64
	rowsSynced    int64
	bytesTransfer int64

	// Control
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSyncEngine creates a new synchronization engine.
func NewSyncEngine(config SyncConfig, store *storage.Store, schema storage.SchemaRegistry, logger *slog.Logger) *SyncEngine {
	if logger == nil {
		logger = slog.Default()
	}

	return &SyncEngine{
		config:     config,
		connectors: make(map[string]Connector),
		jobs:       make(map[string]*SyncJob),
		executions: make(map[string]*SyncExecution),
		store:      store,
		schema:     schema,
		logger:     logger,
	}
}

// RegisterConnector registers a warehouse connector.
func (e *SyncEngine) RegisterConnector(name string, connector Connector) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.connectors[name]; exists {
		return fmt.Errorf("connector %s already registered", name)
	}

	e.connectors[name] = connector
	e.logger.Info("registered connector", "name", name, "type", connector.Type())

	return nil
}

// UnregisterConnector removes a warehouse connector.
func (e *SyncEngine) UnregisterConnector(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	connector, exists := e.connectors[name]
	if !exists {
		return fmt.Errorf("connector %s not found", name)
	}

	if err := connector.Close(); err != nil {
		e.logger.Warn("error closing connector", "name", name, "error", err)
	}

	delete(e.connectors, name)
	return nil
}

// GetConnector returns a registered connector.
func (e *SyncEngine) GetConnector(name string) (Connector, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	connector, exists := e.connectors[name]
	if !exists {
		return nil, fmt.Errorf("connector %s not found", name)
	}

	return connector, nil
}

// ListConnectors returns all registered connectors.
func (e *SyncEngine) ListConnectors() map[string]ConnectorType {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]ConnectorType)
	for name, connector := range e.connectors {
		result[name] = connector.Type()
	}

	return result
}

// CreateJob creates a new sync job.
func (e *SyncEngine) CreateJob(job *SyncJob) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if job.ID == "" {
		return errors.New("job ID is required")
	}
	if _, exists := e.jobs[job.ID]; exists {
		return fmt.Errorf("job %s already exists", job.ID)
	}

	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now

	e.jobs[job.ID] = job
	e.logger.Info("created sync job",
		"id", job.ID,
		"name", job.Name,
		"direction", job.Direction,
	)

	return nil
}

// UpdateJob updates an existing sync job.
func (e *SyncEngine) UpdateJob(job *SyncJob) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.jobs[job.ID]; !exists {
		return fmt.Errorf("job %s not found", job.ID)
	}

	job.UpdatedAt = time.Now()
	e.jobs[job.ID] = job

	return nil
}

// DeleteJob removes a sync job.
func (e *SyncEngine) DeleteJob(jobID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.jobs[jobID]; !exists {
		return fmt.Errorf("job %s not found", jobID)
	}

	delete(e.jobs, jobID)
	return nil
}

// GetJob returns a sync job by ID.
func (e *SyncEngine) GetJob(jobID string) (*SyncJob, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	job, exists := e.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	return job, nil
}

// ListJobs returns all sync jobs.
func (e *SyncEngine) ListJobs() []*SyncJob {
	e.mu.RLock()
	defer e.mu.RUnlock()

	jobs := make([]*SyncJob, 0, len(e.jobs))
	for _, job := range e.jobs {
		jobs = append(jobs, job)
	}

	return jobs
}

// ExecuteJob runs a sync job immediately.
func (e *SyncEngine) ExecuteJob(ctx context.Context, jobID string, connectorName string) (*SyncExecution, error) {
	e.mu.RLock()
	job, jobExists := e.jobs[jobID]
	connector, connectorExists := e.connectors[connectorName]
	e.mu.RUnlock()

	if !jobExists {
		return nil, fmt.Errorf("job %s not found", jobID)
	}
	if !connectorExists {
		return nil, fmt.Errorf("connector %s not found", connectorName)
	}

	// Create execution record
	execution := &SyncExecution{
		ID:        fmt.Sprintf("%s-%d", jobID, time.Now().UnixNano()),
		JobID:     jobID,
		Status:    SyncStatusRunning,
		StartedAt: time.Now(),
	}

	e.mu.Lock()
	e.executions[execution.ID] = execution
	e.mu.Unlock()

	atomic.AddInt64(&e.syncCount, 1)

	// Run sync
	var syncErr error
	switch job.Direction {
	case SyncDirectionExport:
		syncErr = e.runExport(ctx, job, connector, execution)
	case SyncDirectionImport:
		syncErr = e.runImport(ctx, job, connector, execution)
	case SyncDirectionBidir:
		syncErr = e.runBidirectional(ctx, job, connector, execution)
	default:
		syncErr = fmt.Errorf("unknown sync direction: %s", job.Direction)
	}

	// Update execution status
	now := time.Now()
	execution.CompletedAt = &now

	if syncErr != nil {
		execution.Status = SyncStatusFailed
		execution.Error = syncErr.Error()
		atomic.AddInt64(&e.syncErrors, 1)
		e.logger.Error("sync failed",
			"job", jobID,
			"execution", execution.ID,
			"error", syncErr,
		)
	} else {
		execution.Status = SyncStatusCompleted
		e.logger.Info("sync completed",
			"job", jobID,
			"execution", execution.ID,
			"rows", execution.RowsSynced,
			"duration", now.Sub(execution.StartedAt),
		)
	}

	atomic.AddInt64(&e.rowsSynced, execution.RowsSynced)
	atomic.AddInt64(&e.bytesTransfer, execution.BytesTransferred)

	return execution, syncErr
}

// runExport executes an export sync.
func (e *SyncEngine) runExport(ctx context.Context, job *SyncJob, connector Connector, execution *SyncExecution) error {
	req := &ExportRequest{
		Table:       job.Target,
		Features:    job.Features,
		Mode:        job.Mode,
		CreateTable: true,
	}

	result, err := connector.Export(ctx, req)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	execution.RowsSynced = result.RowsExported
	execution.BytesTransferred = result.BytesExported

	return nil
}

// runImport executes an import sync.
func (e *SyncEngine) runImport(ctx context.Context, job *SyncJob, connector Connector, execution *SyncExecution) error {
	req := &ImportRequest{
		Table:           job.Source,
		EntityColumn:    job.EntityColumn,
		FeatureColumns:  job.FeatureMapping,
		TimestampColumn: job.TimestampColumn,
		Mode:            job.Mode,
		Filter:          job.Filter,
	}

	result, err := connector.Import(ctx, req)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	execution.RowsSynced = result.RowsImported
	execution.RowsFailed = result.SkippedRows

	return nil
}

// runBidirectional executes a bidirectional sync.
func (e *SyncEngine) runBidirectional(ctx context.Context, job *SyncJob, connector Connector, execution *SyncExecution) error {
	// Export first
	if err := e.runExport(ctx, job, connector, execution); err != nil {
		return fmt.Errorf("export phase: %w", err)
	}

	// Then import
	if err := e.runImport(ctx, job, connector, execution); err != nil {
		return fmt.Errorf("import phase: %w", err)
	}

	return nil
}

// GetExecution returns an execution by ID.
func (e *SyncEngine) GetExecution(executionID string) (*SyncExecution, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	execution, exists := e.executions[executionID]
	if !exists {
		return nil, fmt.Errorf("execution %s not found", executionID)
	}

	return execution, nil
}

// ListExecutions returns executions for a job.
func (e *SyncEngine) ListExecutions(jobID string) []*SyncExecution {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var executions []*SyncExecution
	for _, exec := range e.executions {
		if exec.JobID == jobID {
			executions = append(executions, exec)
		}
	}

	return executions
}

// Start starts the sync engine background workers.
func (e *SyncEngine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cancel != nil {
		return // Already running
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel

	e.wg.Add(1)
	go e.schedulerLoop(ctx)

	e.logger.Info("sync engine started", "interval", e.config.SyncInterval)
}

// Stop stops the sync engine.
func (e *SyncEngine) Stop() {
	e.mu.Lock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	e.mu.Unlock()

	e.wg.Wait()
	e.logger.Info("sync engine stopped")
}

// schedulerLoop runs scheduled sync jobs.
func (e *SyncEngine) schedulerLoop(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(e.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.runScheduledJobs(ctx)
		}
	}
}

// runScheduledJobs executes all enabled scheduled jobs.
func (e *SyncEngine) runScheduledJobs(ctx context.Context) {
	e.mu.RLock()
	jobs := make([]*SyncJob, 0)
	for _, job := range e.jobs {
		if job.Enabled && job.Schedule != "" {
			jobs = append(jobs, job)
		}
	}
	connectors := make(map[string]Connector)
	for name, conn := range e.connectors {
		connectors[name] = conn
	}
	e.mu.RUnlock()

	// Execute jobs concurrently with max concurrency
	sem := make(chan struct{}, e.config.MaxConcurrency)

	for _, job := range jobs {
		select {
		case <-ctx.Done():
			return
		case sem <- struct{}{}:
		}

		go func(j *SyncJob) {
			defer func() { <-sem }()

			// Find appropriate connector (simplified - uses first matching type)
			for name, conn := range connectors {
				if conn.State() == ConnectionStateConnected {
					_, _ = e.ExecuteJob(ctx, j.ID, name)
					break
				}
			}
		}(job)
	}
}

// Stats returns engine statistics.
func (e *SyncEngine) Stats() map[string]interface{} {
	e.mu.RLock()
	connectorCount := len(e.connectors)
	jobCount := len(e.jobs)
	executionCount := len(e.executions)
	e.mu.RUnlock()

	return map[string]interface{}{
		"connectors":        connectorCount,
		"jobs":              jobCount,
		"executions":        executionCount,
		"sync_count":        atomic.LoadInt64(&e.syncCount),
		"sync_errors":       atomic.LoadInt64(&e.syncErrors),
		"rows_synced":       atomic.LoadInt64(&e.rowsSynced),
		"bytes_transferred": atomic.LoadInt64(&e.bytesTransfer),
	}
}

// MarshalJSON serializes the engine state for API responses.
func (e *SyncEngine) MarshalJSON() ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return json.Marshal(map[string]interface{}{
		"connectors": e.ListConnectors(),
		"jobs":       e.jobs,
		"stats":      e.Stats(),
	})
}
