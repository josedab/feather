package offlinestore

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// DatasetConfig defines the parameters for creating a dataset.
type DatasetConfig struct {
	Name         string
	FeatureGroup string
	EntityType   string
	StartTime    time.Time
	EndTime      time.Time
	CreatedAt    time.Time
}

// FeatureRow represents a single feature observation.
type FeatureRow struct {
	EntityID  string
	Features  map[string]interface{}
	Timestamp time.Time
}

// DatasetInfo contains metadata about a dataset.
type DatasetInfo struct {
	Config      DatasetConfig
	RowCount    int64
	SizeBytes   int64
	Format      string
	Status      string // "pending", "ready", "failed"
	CreatedAt   time.Time
	CompletedAt time.Time
}

// ExportConfig configures dataset export.
type ExportConfig struct {
	// Format is the export format ("parquet", "csv", "jsonl").
	Format string

	// Compression is the compression algorithm.
	Compression string

	// MaxRows is the maximum number of rows to export.
	MaxRows int
}

// DefaultExportConfig returns sensible defaults.
func DefaultExportConfig() ExportConfig {
	return ExportConfig{
		Format:      "parquet",
		Compression: "snappy",
		MaxRows:     10000000,
	}
}

// ExportResult contains metadata about an export operation.
type ExportResult struct {
	Dataset      string
	Format       string
	RowCount     int64
	SizeEstimate int64
}

// StoreConfig configures the offline store.
type StoreConfig struct {
	// MaxDatasets is the maximum number of datasets.
	MaxDatasets int

	// MaxRowsPerDataset is the maximum rows per dataset.
	MaxRowsPerDataset int64

	// RetentionDays is how long to keep datasets.
	RetentionDays int
}

// DefaultStoreConfig returns sensible defaults.
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		MaxDatasets:       1000,
		MaxRowsPerDataset: 100000000,
		RetentionDays:     90,
	}
}

// StoreStats contains store statistics.
type StoreStats struct {
	TotalDatasets  int64
	TotalRows      int64
	TotalSizeBytes int64
}

// Store manages datasets and feature rows for offline access.
type Store struct {
	mu            sync.RWMutex
	config        StoreConfig
	datasets      map[string]*DatasetInfo
	data          map[string][]FeatureRow
	parquetWriter *ParquetWriter
}

// NewStore creates a new offline store.
func NewStore(config StoreConfig) *Store {
	if config.MaxDatasets == 0 {
		config = DefaultStoreConfig()
	}

	return &Store{
		config:   config,
		datasets: make(map[string]*DatasetInfo),
		data:     make(map[string][]FeatureRow),
	}
}

// CreateDataset creates a new dataset.
func (s *Store) CreateDataset(cfg DatasetConfig) (*DatasetInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cfg.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrDatasetNotFound)
	}

	if _, exists := s.datasets[cfg.Name]; exists {
		return nil, fmt.Errorf("%w: %s", ErrDatasetExists, cfg.Name)
	}

	if !cfg.StartTime.IsZero() && !cfg.EndTime.IsZero() && cfg.EndTime.Before(cfg.StartTime) {
		return nil, fmt.Errorf("%w: end time before start time", ErrInvalidTimeRange)
	}

	if len(s.datasets) >= s.config.MaxDatasets {
		return nil, fmt.Errorf("maximum datasets (%d) reached", s.config.MaxDatasets)
	}

	now := time.Now()
	if cfg.CreatedAt.IsZero() {
		cfg.CreatedAt = now
	}

	info := &DatasetInfo{
		Config:    cfg,
		Format:    "parquet",
		Status:    "pending",
		CreatedAt: now,
	}

	s.datasets[cfg.Name] = info
	s.data[cfg.Name] = make([]FeatureRow, 0)

	result := *info
	return &result, nil
}

// GetDataset returns dataset info by name.
func (s *Store) GetDataset(name string) (*DatasetInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, exists := s.datasets[name]
	if !exists {
		return nil, ErrDatasetNotFound
	}

	result := *info
	return &result, nil
}

// ListDatasets returns all datasets.
func (s *Store) ListDatasets() []DatasetInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]DatasetInfo, 0, len(s.datasets))
	for _, info := range s.datasets {
		result = append(result, *info)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Config.Name < result[j].Config.Name
	})

	return result
}

// DeleteDataset removes a dataset and its data.
func (s *Store) DeleteDataset(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.datasets[name]; !exists {
		return ErrDatasetNotFound
	}

	delete(s.datasets, name)
	delete(s.data, name)
	return nil
}

// AppendRows adds feature rows to a dataset.
func (s *Store) AppendRows(dataset string, rows []FeatureRow) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, exists := s.datasets[dataset]
	if !exists {
		return ErrDatasetNotFound
	}

	currentRows := s.data[dataset]
	if int64(len(currentRows))+int64(len(rows)) > s.config.MaxRowsPerDataset {
		return fmt.Errorf("would exceed max rows per dataset (%d)", s.config.MaxRowsPerDataset)
	}

	// Write to Parquet first so a failure doesn't leave inconsistent state.
	if s.parquetWriter != nil {
		if _, _, err := s.parquetWriter.WriteRows(dataset, info.Config.EntityType, rows); err != nil {
			return fmt.Errorf("writing parquet: %w", err)
		}
	}

	s.data[dataset] = append(currentRows, rows...)
	info.RowCount = int64(len(s.data[dataset]))

	// Estimate size: ~100 bytes per row
	info.SizeBytes = info.RowCount * 100
	info.Status = "ready"
	info.CompletedAt = time.Now()

	return nil
}

// SetParquetWriter configures Parquet persistence for the store.
func (s *Store) SetParquetWriter(pw *ParquetWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.parquetWriter = pw
}

// GetRows returns rows from a dataset with pagination.
func (s *Store) GetRows(dataset string, limit, offset int) ([]FeatureRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.datasets[dataset]; !exists {
		return nil, ErrDatasetNotFound
	}

	rows := s.data[dataset]
	if offset >= len(rows) {
		return []FeatureRow{}, nil
	}

	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}

	result := make([]FeatureRow, end-offset)
	copy(result, rows[offset:end])

	return result, nil
}

// GetPointInTime returns features valid as of the given time for an entity.
func (s *Store) GetPointInTime(dataset string, entityID string, asOf time.Time) ([]FeatureRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.datasets[dataset]; !exists {
		return nil, ErrDatasetNotFound
	}

	rows := s.data[dataset]
	var result []FeatureRow

	for _, row := range rows {
		if row.EntityID == entityID && !row.Timestamp.After(asOf) {
			result = append(result, row)
		}
	}

	// Sort by timestamp descending (most recent first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	return result, nil
}

// ExportDataset returns metadata about what would be exported.
func (s *Store) ExportDataset(name string, config ExportConfig) (*ExportResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, exists := s.datasets[name]
	if !exists {
		return nil, ErrDatasetNotFound
	}

	if config.Format == "" {
		config = DefaultExportConfig()
	}

	rowCount := info.RowCount
	if config.MaxRows > 0 && rowCount > int64(config.MaxRows) {
		rowCount = int64(config.MaxRows)
	}

	// Estimate size based on format
	var sizeEstimate int64
	switch config.Format {
	case "parquet":
		sizeEstimate = rowCount * 50 // Parquet is compact
	case "csv":
		sizeEstimate = rowCount * 200
	case "jsonl":
		sizeEstimate = rowCount * 300
	default:
		sizeEstimate = rowCount * 100
	}

	return &ExportResult{
		Dataset:      name,
		Format:       config.Format,
		RowCount:     rowCount,
		SizeEstimate: sizeEstimate,
	}, nil
}

// Stats returns store statistics.
func (s *Store) Stats() StoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalRows, totalSize int64
	for _, info := range s.datasets {
		totalRows += info.RowCount
		totalSize += info.SizeBytes
	}

	return StoreStats{
		TotalDatasets:  int64(len(s.datasets)),
		TotalRows:      totalRows,
		TotalSizeBytes: totalSize,
	}
}
