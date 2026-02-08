package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// DataMigrator handles data migration from Feast to Feather.
type DataMigrator struct {
	config  DataMigratorConfig
	stats   *MigrationStats
	statsMu sync.Mutex
}

// DataMigratorConfig configures the data migrator.
type DataMigratorConfig struct {
	// BatchSize is the number of records to process at once
	BatchSize int
	// Workers is the number of parallel workers
	Workers int
	// SourceType is the type of data source (parquet, csv, bigquery, etc.)
	SourceType string
	// TargetStore is the feature store interface
	TargetStore FeatureStore
	// TransformFunc allows custom data transformation
	TransformFunc func(record map[string]interface{}) (map[string]interface{}, error)
	// OnError defines error handling behavior
	OnError ErrorHandling
	// ValidateTypes enables type validation during migration
	ValidateTypes bool
}

// ErrorHandling defines how to handle migration errors.
type ErrorHandling string

// ErrorHandling constants.
const (
	ErrorSkip ErrorHandling = "skip" // Skip bad records
	ErrorFail ErrorHandling = "fail" // Fail on first error
	ErrorLog  ErrorHandling = "log"  // Log and continue
)

// FeatureStore interface for storing migrated features.
type FeatureStore interface {
	Put(entityKey string, features map[string]interface{}, timestamp time.Time) error
	PutBatch(records []FeatureRecord) error
}

// FeatureRecord represents a single feature record for migration.
type FeatureRecord struct {
	EntityKey string                 `json:"entity_key"`
	Features  map[string]interface{} `json:"features"`
	Timestamp time.Time              `json:"timestamp"`
	EventTime time.Time              `json:"event_time"`
}

// MigrationStats tracks migration progress and statistics.
type MigrationStats struct { //nolint:revive
	StartTime        time.Time     `json:"start_time"`
	EndTime          time.Time     `json:"end_time,omitempty"`
	Duration         time.Duration `json:"duration,omitempty"`
	TotalRecords     int64         `json:"total_records"`
	ProcessedRecords int64         `json:"processed_records"`
	FailedRecords    int64         `json:"failed_records"`
	SkippedRecords   int64         `json:"skipped_records"`
	BytesProcessed   int64         `json:"bytes_processed"`
	Errors           []string      `json:"errors,omitempty"`
	Warnings         []string      `json:"warnings,omitempty"`
	Status           string        `json:"status"`
}

// StatusCanceled indicates a migration was canceled.
const StatusCanceled = "canceled"

// DefaultDataMigratorConfig returns sensible defaults.
func DefaultDataMigratorConfig() DataMigratorConfig {
	return DataMigratorConfig{
		BatchSize:     1000,
		Workers:       4,
		SourceType:    "parquet",
		OnError:       ErrorLog,
		ValidateTypes: true,
	}
}

// NewDataMigrator creates a new data migrator.
func NewDataMigrator(config DataMigratorConfig) *DataMigrator {
	return &DataMigrator{
		config: config,
		stats: &MigrationStats{
			Status: "initialized",
		},
	}
}

// DataSource interface for reading feature data.
type DataSource interface {
	// Read returns the next batch of records
	Read(ctx context.Context, batchSize int) ([]map[string]interface{}, error)
	// Close releases resources
	Close() error
	// EstimateTotal returns estimated total records (0 if unknown)
	EstimateTotal() int64
}

// MigrateFromSource migrates data from a DataSource.
func (m *DataMigrator) MigrateFromSource(ctx context.Context, source DataSource, mapping *FieldMapping) (*MigrationStats, error) {
	m.statsMu.Lock()
	m.stats.StartTime = time.Now()
	m.stats.Status = "running"
	m.stats.TotalRecords = source.EstimateTotal()
	m.statsMu.Unlock()

	defer func() {
		m.statsMu.Lock()
		m.stats.EndTime = time.Now()
		m.stats.Duration = m.stats.EndTime.Sub(m.stats.StartTime)
		if m.stats.Status == "running" {
			m.stats.Status = "completed"
		}
		m.statsMu.Unlock()
	}()

	// Create work queue
	recordCh := make(chan []map[string]interface{}, m.config.Workers*2)
	errCh := make(chan error, m.config.Workers)
	doneCh := make(chan struct{})

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < m.config.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.worker(ctx, recordCh, mapping, errCh)
		}()
	}

	// Close workers when done
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	// Read and distribute work
	go func() {
		defer close(recordCh)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			batch, err := source.Read(ctx, m.config.BatchSize)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				m.recordError(fmt.Sprintf("read error: %v", err))
				if m.config.OnError == ErrorFail {
					errCh <- err
					return
				}
				continue
			}

			if len(batch) == 0 {
				return
			}

			select {
			case recordCh <- batch:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for completion or error
	select {
	case <-doneCh:
		return m.GetStats(), nil
	case err := <-errCh:
		m.statsMu.Lock()
		m.stats.Status = "failed"
		m.statsMu.Unlock()
		return m.GetStats(), err
	case <-ctx.Done():
		m.statsMu.Lock()
		m.stats.Status = StatusCanceled
		m.statsMu.Unlock()
		return m.GetStats(), ctx.Err()
	}
}

func (m *DataMigrator) worker(ctx context.Context, recordCh <-chan []map[string]interface{}, mapping *FieldMapping, errCh chan<- error) {
	for batch := range recordCh {
		select {
		case <-ctx.Done():
			return
		default:
		}

		records := make([]FeatureRecord, 0, len(batch))
		for _, raw := range batch {
			record, err := m.transformRecord(raw, mapping)
			if err != nil {
				m.handleRecordError(err, raw)
				if m.config.OnError == ErrorFail {
					errCh <- err
					return
				}
				continue
			}
			records = append(records, *record)
		}

		if len(records) > 0 && m.config.TargetStore != nil {
			if err := m.config.TargetStore.PutBatch(records); err != nil {
				m.recordError(fmt.Sprintf("store error: %v", err))
				if m.config.OnError == ErrorFail {
					errCh <- err
					return
				}
			}
		}

		m.statsMu.Lock()
		m.stats.ProcessedRecords += int64(len(records))
		m.statsMu.Unlock()
	}
}

func (m *DataMigrator) transformRecord(raw map[string]interface{}, mapping *FieldMapping) (*FeatureRecord, error) {
	// Apply custom transform if provided
	if m.config.TransformFunc != nil {
		var err error
		raw, err = m.config.TransformFunc(raw)
		if err != nil {
			return nil, fmt.Errorf("transform error: %w", err)
		}
	}

	// Extract entity key
	entityKey, err := mapping.GetEntityKey(raw)
	if err != nil {
		return nil, fmt.Errorf("entity key error: %w", err)
	}

	// Extract timestamp
	timestamp, err := mapping.GetTimestamp(raw)
	if err != nil {
		timestamp = time.Now()
	}

	// Extract event time
	eventTime, err := mapping.GetEventTime(raw)
	if err != nil {
		eventTime = timestamp
	}

	// Map features
	features, err := mapping.MapFeatures(raw)
	if err != nil {
		return nil, fmt.Errorf("feature mapping error: %w", err)
	}

	return &FeatureRecord{
		EntityKey: entityKey,
		Features:  features,
		Timestamp: timestamp,
		EventTime: eventTime,
	}, nil
}

func (m *DataMigrator) handleRecordError(err error, record map[string]interface{}) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()

	switch m.config.OnError {
	case ErrorSkip:
		m.stats.SkippedRecords++
	case ErrorLog:
		m.stats.FailedRecords++
		if len(m.stats.Errors) < 100 { // Limit stored errors
			recordJSON, _ := json.Marshal(record)
			m.stats.Errors = append(m.stats.Errors, fmt.Sprintf("%v: %s", err, string(recordJSON)[:minInt(200, len(recordJSON))]))
		}
	case ErrorFail:
		m.stats.FailedRecords++
	}
}

func (m *DataMigrator) recordError(msg string) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()

	if len(m.stats.Errors) < 100 {
		m.stats.Errors = append(m.stats.Errors, msg)
	}
}

// GetStats returns current migration statistics.
func (m *DataMigrator) GetStats() *MigrationStats {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()

	// Return a copy
	stats := *m.stats
	return &stats
}

// FieldMapping defines how to map source fields to target fields.
type FieldMapping struct {
	EntityKeyField   string            `json:"entity_key_field"`
	TimestampField   string            `json:"timestamp_field"`
	EventTimeField   string            `json:"event_time_field,omitempty"`
	FeatureFields    map[string]string `json:"feature_fields"` // source -> target
	IgnoreFields     []string          `json:"ignore_fields,omitempty"`
	DefaultTimestamp time.Time         `json:"default_timestamp,omitempty"`
	TimestampFormat  string            `json:"timestamp_format,omitempty"`
}

// NewFieldMapping creates a new field mapping with defaults.
func NewFieldMapping() *FieldMapping {
	return &FieldMapping{
		EntityKeyField:  "entity_id",
		TimestampField:  "event_timestamp",
		FeatureFields:   make(map[string]string),
		IgnoreFields:    make([]string, 0),
		TimestampFormat: time.RFC3339,
	}
}

// GetEntityKey extracts the entity key from a record.
func (fm *FieldMapping) GetEntityKey(record map[string]interface{}) (string, error) {
	val, ok := record[fm.EntityKeyField]
	if !ok {
		return "", fmt.Errorf("entity key field '%s' not found", fm.EntityKeyField)
	}
	return fmt.Sprintf("%v", val), nil
}

// GetTimestamp extracts the timestamp from a record.
func (fm *FieldMapping) GetTimestamp(record map[string]interface{}) (time.Time, error) {
	val, ok := record[fm.TimestampField]
	if !ok {
		if !fm.DefaultTimestamp.IsZero() {
			return fm.DefaultTimestamp, nil
		}
		return time.Time{}, fmt.Errorf("timestamp field '%s' not found", fm.TimestampField)
	}
	return fm.parseTimestamp(val)
}

// GetEventTime extracts the event time from a record.
func (fm *FieldMapping) GetEventTime(record map[string]interface{}) (time.Time, error) {
	if fm.EventTimeField == "" {
		return fm.GetTimestamp(record)
	}
	val, ok := record[fm.EventTimeField]
	if !ok {
		return fm.GetTimestamp(record)
	}
	return fm.parseTimestamp(val)
}

func (fm *FieldMapping) parseTimestamp(val interface{}) (time.Time, error) {
	switch v := val.(type) {
	case time.Time:
		return v, nil
	case string:
		return time.Parse(fm.TimestampFormat, v)
	case int64:
		return time.Unix(v, 0), nil
	case float64:
		return time.Unix(int64(v), 0), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported timestamp type: %T", val)
	}
}

// MapFeatures maps source fields to target feature names.
func (fm *FieldMapping) MapFeatures(record map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// Build ignore set
	ignoreSet := make(map[string]bool)
	for _, f := range fm.IgnoreFields {
		ignoreSet[f] = true
	}
	ignoreSet[fm.EntityKeyField] = true
	ignoreSet[fm.TimestampField] = true
	if fm.EventTimeField != "" {
		ignoreSet[fm.EventTimeField] = true
	}

	// If explicit mapping provided, use it
	if len(fm.FeatureFields) > 0 {
		for source, target := range fm.FeatureFields {
			if val, ok := record[source]; ok {
				result[target] = val
			}
		}
		return result, nil
	}

	// Otherwise, map all non-ignored fields
	for key, val := range record {
		if ignoreSet[key] {
			continue
		}
		result[key] = val
	}

	return result, nil
}

// minInt returns the minimum of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MigrationPlan represents a planned migration.
type MigrationPlan struct { //nolint:revive
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description,omitempty"`
	SourceType    string                 `json:"source_type"`
	SourceConfig  map[string]interface{} `json:"source_config"`
	FieldMapping  *FieldMapping          `json:"field_mapping"`
	TargetGroups  []string               `json:"target_groups"`
	EstimatedRows int64                  `json:"estimated_rows"`
	CreatedAt     time.Time              `json:"created_at"`
	Status        string                 `json:"status"`
}

// MigrationJob represents an active or completed migration job.
type MigrationJob struct { //nolint:revive
	ID          string          `json:"id"`
	PlanID      string          `json:"plan_id"`
	Stats       *MigrationStats `json:"stats"`
	StartedAt   time.Time       `json:"started_at"`
	CompletedAt time.Time       `json:"completed_at,omitempty"`
	Status      string          `json:"status"`
}
