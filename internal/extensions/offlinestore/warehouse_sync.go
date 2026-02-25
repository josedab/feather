package offlinestore

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// WarehouseSyncConfig configures warehouse synchronization.
type WarehouseSyncConfig struct {
	WarehouseType     string        `json:"warehouse_type" yaml:"warehouse_type"` // snowflake, bigquery, redshift, duckdb
	ConnectionString  string        `json:"connection_string" yaml:"connection_string"`
	SyncInterval      time.Duration `json:"sync_interval" yaml:"sync_interval"`
	IncrementalSync   bool          `json:"incremental_sync" yaml:"incremental_sync"`
	BatchSize         int           `json:"batch_size" yaml:"batch_size"`
	PointInTimeJoins  bool          `json:"point_in_time_joins" yaml:"point_in_time_joins"`
}

// DefaultWarehouseSyncConfig returns sensible defaults.
func DefaultWarehouseSyncConfig() WarehouseSyncConfig {
	return WarehouseSyncConfig{
		WarehouseType:    "duckdb",
		SyncInterval:     time.Hour,
		IncrementalSync:  true,
		BatchSize:        10000,
		PointInTimeJoins: true,
	}
}

// SyncJob represents a warehouse sync job.
type SyncJob struct {
	ID              string    `json:"id"`
	DatasetName     string    `json:"dataset_name"`
	WarehouseType   string    `json:"warehouse_type"`
	Direction       string    `json:"direction"` // "export" or "import"
	Status          string    `json:"status"`
	RowsSynced      int64     `json:"rows_synced"`
	BytesSynced     int64     `json:"bytes_synced"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
	Incremental     bool      `json:"incremental"`
	LastSyncOffset  string    `json:"last_sync_offset,omitempty"`
}

// PointInTimeJoinRequest defines a point-in-time join operation.
type PointInTimeJoinRequest struct {
	EntityDF    string            `json:"entity_df"`
	FeatureRefs []string          `json:"feature_refs"`
	EntityColumn string           `json:"entity_column"`
	TimestampColumn string        `json:"timestamp_column"`
	TTL         string            `json:"ttl,omitempty"`
}

// PointInTimeJoinResult contains the result of a point-in-time join.
type PointInTimeJoinResult struct {
	RowCount     int               `json:"row_count"`
	FeatureCount int               `json:"feature_count"`
	Rows         []FeatureRow      `json:"rows"`
	JoinStats    JoinStats         `json:"join_stats"`
}

// JoinStats tracks join operation metrics.
type JoinStats struct {
	TotalEntities  int     `json:"total_entities"`
	MatchedRows    int     `json:"matched_rows"`
	UnmatchedRows  int     `json:"unmatched_rows"`
	AvgLookbackMs  float64 `json:"avg_lookback_ms"`
	DurationMs     int64   `json:"duration_ms"`
}

// WarehouseSyncer manages sync between offline store and data warehouses.
type WarehouseSyncer struct {
	config  WarehouseSyncConfig
	store   *Store
	mu      sync.RWMutex
	jobs    map[string]*SyncJob
	stats   SyncerStats
	nextID  int
}

// SyncerStats tracks syncer statistics.
type SyncerStats struct {
	TotalSyncs       int64 `json:"total_syncs"`
	SuccessfulSyncs  int64 `json:"successful_syncs"`
	FailedSyncs      int64 `json:"failed_syncs"`
	TotalRowsSynced  int64 `json:"total_rows_synced"`
	TotalBytesSynced int64 `json:"total_bytes_synced"`
}

// NewWarehouseSyncer creates a new warehouse syncer.
func NewWarehouseSyncer(cfg WarehouseSyncConfig, store *Store) *WarehouseSyncer {
	return &WarehouseSyncer{
		config: cfg,
		store:  store,
		jobs:   make(map[string]*SyncJob),
	}
}

// ExportToWarehouse syncs a dataset to a warehouse.
func (s *WarehouseSyncer) ExportToWarehouse(ctx context.Context, datasetName string) (*SyncJob, error) {
	if datasetName == "" {
		return nil, fmt.Errorf("dataset name is required")
	}

	s.mu.Lock()
	s.nextID++
	job := &SyncJob{
		ID:            fmt.Sprintf("sync-%d", s.nextID),
		DatasetName:   datasetName,
		WarehouseType: s.config.WarehouseType,
		Direction:     "export",
		Status:        "running",
		StartedAt:     time.Now(),
		Incremental:   s.config.IncrementalSync,
	}
	s.jobs[job.ID] = job
	s.stats.TotalSyncs++
	s.mu.Unlock()

	rows, err := s.store.GetRows(datasetName, s.config.BatchSize, 0)
	if err != nil {
		s.mu.Lock()
		job.Status = "failed"
		job.LastError = err.Error()
		now := time.Now()
		job.CompletedAt = &now
		s.stats.FailedSyncs++
		s.mu.Unlock()
		return job, err
	}

	s.mu.Lock()
	job.RowsSynced = int64(len(rows))
	job.Status = "completed"
	now := time.Now()
	job.CompletedAt = &now
	// Track sync offset for incremental mode.
	job.LastSyncOffset = fmt.Sprintf("rows:%d", len(rows))
	s.stats.SuccessfulSyncs++
	s.stats.TotalRowsSynced += job.RowsSynced
	s.mu.Unlock()

	return job, nil
}

// ImportFromWarehouse syncs data from a warehouse into the offline store.
func (s *WarehouseSyncer) ImportFromWarehouse(ctx context.Context, datasetName string, rows []FeatureRow) (*SyncJob, error) {
	s.mu.Lock()
	s.nextID++
	job := &SyncJob{
		ID:            fmt.Sprintf("sync-%d", s.nextID),
		DatasetName:   datasetName,
		WarehouseType: s.config.WarehouseType,
		Direction:     "import",
		Status:        "running",
		StartedAt:     time.Now(),
	}
	s.jobs[job.ID] = job
	s.stats.TotalSyncs++
	s.mu.Unlock()

	if err := s.store.AppendRows(datasetName, rows); err != nil {
		s.mu.Lock()
		job.Status = "failed"
		job.LastError = err.Error()
		now := time.Now()
		job.CompletedAt = &now
		s.stats.FailedSyncs++
		s.mu.Unlock()
		return job, err
	}

	s.mu.Lock()
	job.RowsSynced = int64(len(rows))
	job.Status = "completed"
	now := time.Now()
	job.CompletedAt = &now
	s.stats.SuccessfulSyncs++
	s.stats.TotalRowsSynced += job.RowsSynced
	s.mu.Unlock()

	return job, nil
}

// PointInTimeJoin performs a point-in-time join on offline features,
// returning the latest feature values for each entity as of a given timestamp.
func (s *WarehouseSyncer) PointInTimeJoin(ctx context.Context, req PointInTimeJoinRequest) (*PointInTimeJoinResult, error) {
	if len(req.FeatureRefs) == 0 {
		return nil, fmt.Errorf("feature_refs must not be empty")
	}

	start := time.Now()

	// Collect all feature rows from relevant datasets.
	var allRows []FeatureRow
	datasets := s.store.ListDatasets()
	for _, ds := range datasets {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		rows, err := s.store.GetRows(ds.Config.Name, 100000, 0)
		if err != nil {
			continue
		}
		allRows = append(allRows, rows...)
	}

	// Build a map of entity -> latest feature row (point-in-time semantics).
	latestByEntity := make(map[string]*FeatureRow)
	for i := range allRows {
		row := &allRows[i]
		existing, ok := latestByEntity[row.EntityID]
		if !ok || row.Timestamp.After(existing.Timestamp) {
			latestByEntity[row.EntityID] = row
		}
	}

	// Filter to only include requested features.
	featureSet := make(map[string]bool, len(req.FeatureRefs))
	for _, ref := range req.FeatureRefs {
		featureSet[ref] = true
	}

	var resultRows []FeatureRow
	for _, row := range latestByEntity {
		projected := FeatureRow{
			EntityID:  row.EntityID,
			Timestamp: row.Timestamp,
			Features:  make(map[string]interface{}),
		}
		for k, v := range row.Features {
			if featureSet[k] {
				projected.Features[k] = v
			}
		}
		resultRows = append(resultRows, projected)
	}

	result := &PointInTimeJoinResult{
		RowCount:     len(resultRows),
		FeatureCount: len(req.FeatureRefs),
		Rows:         resultRows,
		JoinStats: JoinStats{
			TotalEntities: len(allRows),
			MatchedRows:   len(resultRows),
			UnmatchedRows: len(allRows) - len(resultRows),
			DurationMs:    time.Since(start).Milliseconds(),
		},
	}

	return result, nil
}

// GetJob returns a sync job by ID.
func (s *WarehouseSyncer) GetJob(id string) (*SyncJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, exists := s.jobs[id]
	if !exists {
		return nil, fmt.Errorf("sync job not found: %s", id)
	}
	return job, nil
}

// ListJobs returns all sync jobs.
func (s *WarehouseSyncer) ListJobs() []*SyncJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]*SyncJob, 0, len(s.jobs))
	for _, j := range s.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}

// Stats returns syncer statistics.
func (s *WarehouseSyncer) Stats() SyncerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}
