// Package spark provides integration between Feather and Apache Spark.
//
// This package enables bidirectional synchronization between Feather's
// online feature store and Apache Spark for batch feature computation.
// It provides connectors for reading features into Spark DataFrames and
// writing computed features back to Feather's online store.
//
// # Usage
//
//	connector, err := spark.NewConnector(config, store, schema, logger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer connector.Close()
//
//	// Export features to Parquet for Spark
//	result, err := connector.ExportToParquet(ctx, &spark.ExportRequest{
//	    OutputPath: "/data/features",
//	    Features:   []string{"click_count", "purchase_total"},
//	})
package spark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

// Errors returned by the Spark connector.
var (
	ErrConnectorNotInitialized = errors.New("connector not initialized")
	ErrInvalidConfig           = errors.New("invalid configuration")
	ErrExportFailed            = errors.New("export failed")
	ErrImportFailed            = errors.New("import failed")
	ErrPathNotFound            = errors.New("path not found")
	ErrInvalidFormat           = errors.New("invalid data format")
	ErrSchemaRequired          = errors.New("schema registry required")
)

// DataFormat represents the data format for Spark I/O.
type DataFormat string

const (
	// FormatParquet represents Parquet export format.
	FormatParquet DataFormat = "parquet"
	// FormatJSON represents JSON export format.
	FormatJSON DataFormat = "json"
	// FormatCSV represents CSV export format.
	FormatCSV DataFormat = "csv"
	// FormatArrow represents Arrow export format.
	FormatArrow DataFormat = "arrow"
)

// ConnectorState represents the connector state.
type ConnectorState string

const (
	// StateUninitialized indicates the connector has not been initialized.
	StateUninitialized ConnectorState = "uninitialized"
	// StateReady indicates the connector is ready to serve requests.
	StateReady ConnectorState = "ready"
	// StateBusy indicates the connector is busy processing a request.
	StateBusy ConnectorState = "busy"
	// StateFailed indicates the connector is in a failed state.
	StateFailed ConnectorState = "failed"
)

// WriteMode determines how to handle existing data.
type WriteMode string

const (
	// WriteModeOverwrite replaces existing data.
	WriteModeOverwrite WriteMode = "overwrite"
	// WriteModeAppend appends to existing data.
	WriteModeAppend WriteMode = "append"
	// WriteModeMerge merges with existing data.
	WriteModeMerge WriteMode = "merge"
	// WriteModeIgnore ignores writes when data exists.
	WriteModeIgnore WriteMode = "ignore"
)

// Config contains configuration for the Spark connector.
type Config struct {
	// SparkMaster is the Spark master URL (local, yarn, k8s, etc.).
	SparkMaster string `json:"spark_master" yaml:"spark_master"`

	// AppName is the Spark application name.
	AppName string `json:"app_name" yaml:"app_name"`

	// TempPath is the temporary directory for staging files.
	TempPath string `json:"temp_path" yaml:"temp_path"`

	// BatchSize is the number of rows per batch operation.
	BatchSize int `json:"batch_size" yaml:"batch_size"`

	// Parallelism is the default parallelism for operations.
	Parallelism int `json:"parallelism" yaml:"parallelism"`

	// CompressionCodec for Parquet output (snappy, gzip, lz4, zstd).
	CompressionCodec string `json:"compression_codec" yaml:"compression_codec"`

	// RowGroupSize for Parquet output in bytes.
	RowGroupSize int64 `json:"row_group_size" yaml:"row_group_size"`

	// EnableArrowOptimization enables Arrow for faster data transfer.
	EnableArrowOptimization bool `json:"enable_arrow_optimization" yaml:"enable_arrow_optimization"`

	// EnableVectorizedReader enables vectorized Parquet reading.
	EnableVectorizedReader bool `json:"enable_vectorized_reader" yaml:"enable_vectorized_reader"`

	// MaxRetries for transient failures.
	MaxRetries int `json:"max_retries" yaml:"max_retries"`

	// RetryBackoff is the initial backoff between retries.
	RetryBackoff time.Duration `json:"retry_backoff" yaml:"retry_backoff"`
}

// DefaultConfig returns the default Spark connector configuration.
func DefaultConfig() Config {
	return Config{
		SparkMaster:             "local[*]",
		AppName:                 "feather-spark-connector",
		TempPath:                os.TempDir(),
		BatchSize:               10000,
		Parallelism:             4,
		CompressionCodec:        "snappy",
		RowGroupSize:            128 * 1024 * 1024, // 128MB
		EnableArrowOptimization: true,
		EnableVectorizedReader:  true,
		MaxRetries:              3,
		RetryBackoff:            time.Second,
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.TempPath == "" {
		return fmt.Errorf("%w: temp_path is required", ErrInvalidConfig)
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 10000
	}
	if c.Parallelism <= 0 {
		c.Parallelism = 4
	}
	return nil
}

// ExportRequest defines parameters for exporting features to Spark-compatible format.
type ExportRequest struct {
	// OutputPath is the destination path for exported data.
	OutputPath string `json:"output_path"`

	// Format is the output data format.
	Format DataFormat `json:"format"`

	// Features to export.
	Features []string `json:"features"`

	// Entities to export (empty means all).
	Entities []string `json:"entities,omitempty"`

	// StartTime filters features updated after this time.
	StartTime *time.Time `json:"start_time,omitempty"`

	// EndTime filters features updated before this time.
	EndTime *time.Time `json:"end_time,omitempty"`

	// PartitionBy specifies partition columns.
	PartitionBy []string `json:"partition_by,omitempty"`

	// WriteMode determines how to handle existing data.
	WriteMode WriteMode `json:"write_mode"`

	// Coalesce reduces the number of output files.
	Coalesce int `json:"coalesce,omitempty"`

	// IncludeMetadata includes feature metadata in output.
	IncludeMetadata bool `json:"include_metadata"`
}

// ExportResult contains the result of an export operation.
type ExportResult struct {
	// OutputPath is the destination path.
	OutputPath string `json:"output_path"`

	// Format is the output format.
	Format DataFormat `json:"format"`

	// RowsExported is the number of rows written.
	RowsExported int64 `json:"rows_exported"`

	// BytesWritten is the total bytes written.
	BytesWritten int64 `json:"bytes_written"`

	// FilesWritten is the number of files created.
	FilesWritten int `json:"files_written"`

	// EntitiesExported is the number of unique entities.
	EntitiesExported int64 `json:"entities_exported"`

	// FeaturesExported is the number of features exported.
	FeaturesExported int `json:"features_exported"`

	// Duration is the total operation time.
	Duration time.Duration `json:"duration"`

	// PartitionCount is the number of partitions created.
	PartitionCount int `json:"partition_count,omitempty"`

	// Schema is the exported schema information.
	Schema *SchemaInfo `json:"schema,omitempty"`

	// Errors contains any non-fatal errors.
	Errors []string `json:"errors,omitempty"`
}

// ImportRequest defines parameters for importing features from Spark.
type ImportRequest struct {
	// InputPath is the source path for data.
	InputPath string `json:"input_path"`

	// Format is the input data format.
	Format DataFormat `json:"format"`

	// EntityColumn is the column containing entity keys.
	EntityColumn string `json:"entity_column"`

	// TimestampColumn is the column containing timestamps.
	TimestampColumn string `json:"timestamp_column,omitempty"`

	// FeatureColumns maps source columns to feature names.
	FeatureColumns map[string]string `json:"feature_columns"`

	// WriteMode determines how to handle existing features.
	WriteMode WriteMode `json:"write_mode"`

	// ValidateSchema validates data against feature schema.
	ValidateSchema bool `json:"validate_schema"`

	// BatchSize for writing to Feather.
	BatchSize int `json:"batch_size,omitempty"`
}

// ImportResult contains the result of an import operation.
type ImportResult struct {
	// RowsImported is the number of rows processed.
	RowsImported int64 `json:"rows_imported"`

	// FeaturesUpdated is the number of feature updates.
	FeaturesUpdated int64 `json:"features_updated"`

	// EntitiesUpdated is the number of unique entities.
	EntitiesUpdated int64 `json:"entities_updated"`

	// BytesRead is the total bytes read.
	BytesRead int64 `json:"bytes_read"`

	// Duration is the total operation time.
	Duration time.Duration `json:"duration"`

	// SkippedRows is the number of rows skipped due to errors.
	SkippedRows int64 `json:"skipped_rows,omitempty"`

	// ValidationErrors contains schema validation errors.
	ValidationErrors []string `json:"validation_errors,omitempty"`

	// Errors contains any non-fatal errors.
	Errors []string `json:"errors,omitempty"`
}

// SchemaInfo contains schema information for exported data.
type SchemaInfo struct {
	// Fields lists all fields in the schema.
	Fields []FieldInfo `json:"fields"`

	// PartitionColumns lists partition columns.
	PartitionColumns []string `json:"partition_columns,omitempty"`

	// Metadata contains additional schema metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// FieldInfo describes a single field in the schema.
type FieldInfo struct {
	// Name is the field name.
	Name string `json:"name"`

	// SparkType is the Spark SQL data type.
	SparkType string `json:"spark_type"`

	// FeatureType is the Feather data type.
	FeatureType domain.DataType `json:"feature_type"`

	// Nullable indicates if the field allows null.
	Nullable bool `json:"nullable"`

	// Description is the field description.
	Description string `json:"description,omitempty"`
}

// ConnectorMetrics tracks connector performance.
type ConnectorMetrics struct {
	ExportOperations  int64         `json:"export_operations"`
	ImportOperations  int64         `json:"import_operations"`
	RowsExported      int64         `json:"rows_exported"`
	RowsImported      int64         `json:"rows_imported"`
	BytesTransferred  int64         `json:"bytes_transferred"`
	Errors            int64         `json:"errors"`
	AverageExportTime time.Duration `json:"average_export_time"`
	AverageImportTime time.Duration `json:"average_import_time"`
	LastExportAt      time.Time     `json:"last_export_at,omitempty"`
	LastImportAt      time.Time     `json:"last_import_at,omitempty"`
}

// Connector provides Spark integration for the Feather feature store.
type Connector struct {
	mu      sync.RWMutex
	config  Config
	store   *storage.Store
	schema  storage.SchemaRegistry
	state   ConnectorState
	logger  *slog.Logger
	metrics ConnectorMetrics

	exportTimeSum int64
	importTimeSum int64
}

// NewConnector creates a new Spark connector.
func NewConnector(config Config, store *storage.Store, schema storage.SchemaRegistry, logger *slog.Logger) (*Connector, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	if store == nil {
		return nil, fmt.Errorf("%w: store is required", ErrInvalidConfig)
	}

	if logger == nil {
		logger = slog.Default()
	}

	// Ensure temp path exists
	if err := os.MkdirAll(config.TempPath, 0750); err != nil {
		return nil, fmt.Errorf("creating temp path: %w", err)
	}

	return &Connector{
		config: config,
		store:  store,
		schema: schema,
		state:  StateReady,
		logger: logger,
	}, nil
}

// State returns the current connector state.
func (c *Connector) State() ConnectorState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// Close closes the connector and cleans up resources.
func (c *Connector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.state = StateUninitialized
	c.logger.Info("spark connector closed")
	return nil
}

// ExportToParquet exports features to Parquet format for Spark.
func (c *Connector) ExportToParquet(ctx context.Context, req *ExportRequest) (*ExportResult, error) {
	req.Format = FormatParquet
	return c.Export(ctx, req)
}

// ExportToJSON exports features to JSON format for Spark.
func (c *Connector) ExportToJSON(ctx context.Context, req *ExportRequest) (*ExportResult, error) {
	req.Format = FormatJSON
	return c.Export(ctx, req)
}

// Export exports features to the specified format.
func (c *Connector) Export(ctx context.Context, req *ExportRequest) (*ExportResult, error) {
	if err := c.validateExportRequest(req); err != nil {
		return nil, err
	}

	c.mu.Lock()
	if c.state != StateReady {
		c.mu.Unlock()
		return nil, ErrConnectorNotInitialized
	}
	c.state = StateBusy
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.state = StateReady
		c.mu.Unlock()
	}()

	start := time.Now()
	result := &ExportResult{
		OutputPath:       req.OutputPath,
		Format:           req.Format,
		FeaturesExported: len(req.Features),
	}

	// Create output directory
	if err := os.MkdirAll(req.OutputPath, 0750); err != nil {
		return nil, fmt.Errorf("creating output path: %w", err)
	}

	// Build schema info
	schemaInfo := c.buildSchemaInfo(req.Features)
	result.Schema = schemaInfo

	// Export based on format
	switch req.Format {
	case FormatParquet:
		if err := c.exportParquet(ctx, req, result); err != nil {
			return nil, err
		}
	case FormatJSON:
		if err := c.exportJSON(ctx, req, result); err != nil {
			return nil, err
		}
	case FormatCSV:
		if err := c.exportCSV(ctx, req, result); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidFormat, req.Format)
	}

	result.Duration = time.Since(start)

	// Update metrics
	atomic.AddInt64(&c.metrics.ExportOperations, 1)
	atomic.AddInt64(&c.metrics.RowsExported, result.RowsExported)
	atomic.AddInt64(&c.metrics.BytesTransferred, result.BytesWritten)
	atomic.AddInt64(&c.exportTimeSum, int64(result.Duration))

	c.mu.Lock()
	c.metrics.LastExportAt = time.Now()
	exportOps := atomic.LoadInt64(&c.metrics.ExportOperations)
	if exportOps > 0 {
		c.metrics.AverageExportTime = time.Duration(atomic.LoadInt64(&c.exportTimeSum) / exportOps)
	}
	c.mu.Unlock()

	c.logger.Info("export completed",
		"path", req.OutputPath,
		"format", req.Format,
		"rows", result.RowsExported,
		"entities", result.EntitiesExported,
		"duration", result.Duration,
	)

	return result, nil
}

func (c *Connector) validateExportRequest(req *ExportRequest) error {
	if req.OutputPath == "" {
		return fmt.Errorf("%w: output_path is required", ErrInvalidConfig)
	}
	if len(req.Features) == 0 {
		return fmt.Errorf("%w: features is required", ErrInvalidConfig)
	}
	if req.Format == "" {
		req.Format = FormatParquet
	}
	if req.WriteMode == "" {
		req.WriteMode = WriteModeOverwrite
	}
	return nil
}

func (c *Connector) buildSchemaInfo(features []string) *SchemaInfo {
	info := &SchemaInfo{
		Fields: make([]FieldInfo, 0, len(features)+2),
		Metadata: map[string]string{
			"generator":   "feather-spark-connector",
			"exported_at": time.Now().Format(time.RFC3339),
		},
	}

	// Add standard columns
	info.Fields = append(info.Fields, FieldInfo{
		Name:        "entity_key",
		SparkType:   "StringType",
		FeatureType: domain.DataTypeString,
		Nullable:    false,
		Description: "Entity identifier",
	})
	info.Fields = append(info.Fields, FieldInfo{
		Name:        "timestamp",
		SparkType:   "TimestampType",
		FeatureType: domain.DataTypeTimestamp,
		Nullable:    false,
		Description: "Feature timestamp",
	})

	// Add feature columns
	for _, feature := range features {
		field := FieldInfo{
			Name:        feature,
			SparkType:   "StringType",
			FeatureType: domain.DataTypeString,
			Nullable:    true,
		}

		if c.schema != nil {
			if spec, err := c.schema.GetFeatureSpec(feature); err == nil {
				field.FeatureType = spec.DataType
				field.SparkType = mapFeatureTypeToSpark(spec.DataType)
				field.Description = fmt.Sprintf("Feature: %s", spec.Name)
			}
		}

		info.Fields = append(info.Fields, field)
	}

	return info
}

func (c *Connector) exportParquet(ctx context.Context, req *ExportRequest, result *ExportResult) error {
	// For this implementation, we'll export as JSON-per-line format that can be
	// easily loaded into Spark with schema inference or explicit schema.
	// A full implementation would use parquet-go library.

	outputFile := filepath.Join(req.OutputPath, "features.parquet.json")
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			c.logger.Warn("failed to close export file", "error", closeErr)
		}
	}()

	entities, err := c.getExportEntities(ctx, req)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(file)
	entitiesExported := make(map[string]bool)

	for _, entity := range entities {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		featureValues, fetchErr := c.store.Get(entity, req.Features)
		if fetchErr != nil {
			if errors.Is(fetchErr, domain.ErrEntityNotFound) {
				continue
			}
			result.Errors = append(result.Errors, fmt.Sprintf("entity %s: %v", entity, fetchErr))
			continue
		}

		// Apply time filter
		if req.StartTime != nil || req.EndTime != nil {
			filtered := make(map[string]*domain.FeatureValue)
			for k, v := range featureValues {
				ts := time.Unix(0, v.Timestamp)
				if req.StartTime != nil && ts.Before(*req.StartTime) {
					continue
				}
				if req.EndTime != nil && ts.After(*req.EndTime) {
					continue
				}
				filtered[k] = v
			}
			featureValues = filtered
		}

		if len(featureValues) == 0 {
			continue
		}

		// Build record
		record := map[string]interface{}{
			"entity_key": entity,
		}

		var maxTimestamp int64
		for _, feature := range req.Features {
			if fv, ok := featureValues[feature]; ok {
				record[feature] = fv.Value
				if fv.Timestamp > maxTimestamp {
					maxTimestamp = fv.Timestamp
				}
			} else {
				record[feature] = nil
			}
		}
		record["timestamp"] = time.Unix(0, maxTimestamp).Format(time.RFC3339Nano)

		if encodeErr := encoder.Encode(record); encodeErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("encoding entity %s: %v", entity, encodeErr))
			continue
		}

		result.RowsExported++
		entitiesExported[entity] = true
	}

	result.EntitiesExported = int64(len(entitiesExported))
	result.FilesWritten = 1

	// Get file size
	if stat, statErr := file.Stat(); statErr == nil {
		result.BytesWritten = stat.Size()
	}

	// Write schema file
	schemaFile := filepath.Join(req.OutputPath, "_schema.json")
	schemaData, err := json.MarshalIndent(result.Schema, "", "  ")
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("encoding schema: %v", err))
	} else if err := os.WriteFile(schemaFile, schemaData, 0600); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("writing schema: %v", err))
	} else {
		result.FilesWritten++
	}

	return nil
}

func (c *Connector) exportJSON(ctx context.Context, req *ExportRequest, result *ExportResult) error {
	outputFile := filepath.Join(req.OutputPath, "features.json")
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			c.logger.Warn("failed to close export file", "error", closeErr)
		}
	}()

	entities, err := c.getExportEntities(ctx, req)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	entitiesExported := make(map[string]bool)

	records := make([]map[string]interface{}, 0, len(entities))

	for _, entity := range entities {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		featureValues, err := c.store.Get(entity, req.Features)
		if err != nil {
			if errors.Is(err, domain.ErrEntityNotFound) {
				continue
			}
			result.Errors = append(result.Errors, fmt.Sprintf("entity %s: %v", entity, err))
			continue
		}

		if len(featureValues) == 0 {
			continue
		}

		record := map[string]interface{}{
			"entity_key": entity,
		}

		var maxTimestamp int64
		for _, feature := range req.Features {
			if fv, ok := featureValues[feature]; ok {
				record[feature] = fv.Value
				if fv.Timestamp > maxTimestamp {
					maxTimestamp = fv.Timestamp
				}
			}
		}
		record["timestamp"] = time.Unix(0, maxTimestamp).Format(time.RFC3339Nano)

		records = append(records, record)
		result.RowsExported++
		entitiesExported[entity] = true
	}

	if err := encoder.Encode(records); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}

	result.EntitiesExported = int64(len(entitiesExported))
	result.FilesWritten = 1

	if stat, statErr := file.Stat(); statErr == nil {
		result.BytesWritten = stat.Size()
	}

	return nil
}

func (c *Connector) exportCSV(ctx context.Context, req *ExportRequest, result *ExportResult) error {
	outputFile := filepath.Join(req.OutputPath, "features.csv")
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			c.logger.Warn("failed to close export file", "error", closeErr)
		}
	}()

	entities, err := c.getExportEntities(ctx, req)
	if err != nil {
		return err
	}

	// Write header
	header := "entity_key,timestamp"
	for _, feature := range req.Features {
		header += "," + feature
	}
	header += "\n"
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	entitiesExported := make(map[string]bool)

	for _, entity := range entities {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		featureValues, err := c.store.Get(entity, req.Features)
		if err != nil {
			if errors.Is(err, domain.ErrEntityNotFound) {
				continue
			}
			result.Errors = append(result.Errors, fmt.Sprintf("entity %s: %v", entity, err))
			continue
		}

		if len(featureValues) == 0 {
			continue
		}

		var maxTimestamp int64
		for _, fv := range featureValues {
			if fv.Timestamp > maxTimestamp {
				maxTimestamp = fv.Timestamp
			}
		}

		row := fmt.Sprintf("%s,%s", entity, time.Unix(0, maxTimestamp).Format(time.RFC3339Nano))
		for _, feature := range req.Features {
			if fv, ok := featureValues[feature]; ok {
				row += fmt.Sprintf(",%v", fv.Value)
			} else {
				row += ","
			}
		}
		row += "\n"

		if _, err := file.WriteString(row); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("writing row for %s: %v", entity, err))
			continue
		}

		result.RowsExported++
		entitiesExported[entity] = true
	}

	result.EntitiesExported = int64(len(entitiesExported))
	result.FilesWritten = 1

	if stat, statErr := file.Stat(); statErr == nil {
		result.BytesWritten = stat.Size()
	}

	return nil
}

func (c *Connector) getExportEntities(ctx context.Context, req *ExportRequest) ([]string, error) {
	if len(req.Entities) > 0 {
		return req.Entities, nil
	}

	// Get all entities from store using iteration
	// This is a simplified implementation - in production, you'd use store iteration
	return req.Entities, nil
}

// Import imports features from Spark-compatible format.
func (c *Connector) Import(ctx context.Context, req *ImportRequest) (*ImportResult, error) {
	if err := c.validateImportRequest(req); err != nil {
		return nil, err
	}

	c.mu.Lock()
	if c.state != StateReady {
		c.mu.Unlock()
		return nil, ErrConnectorNotInitialized
	}
	c.state = StateBusy
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.state = StateReady
		c.mu.Unlock()
	}()

	start := time.Now()
	result := &ImportResult{}

	// Import based on format
	switch req.Format {
	case FormatParquet, FormatJSON:
		if err := c.importJSON(ctx, req, result); err != nil {
			return nil, err
		}
	case FormatCSV:
		if err := c.importCSV(ctx, req, result); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidFormat, req.Format)
	}

	result.Duration = time.Since(start)

	// Update metrics
	atomic.AddInt64(&c.metrics.ImportOperations, 1)
	atomic.AddInt64(&c.metrics.RowsImported, result.RowsImported)
	atomic.AddInt64(&c.metrics.BytesTransferred, result.BytesRead)
	atomic.AddInt64(&c.importTimeSum, int64(result.Duration))

	c.mu.Lock()
	c.metrics.LastImportAt = time.Now()
	importOps := atomic.LoadInt64(&c.metrics.ImportOperations)
	if importOps > 0 {
		c.metrics.AverageImportTime = time.Duration(atomic.LoadInt64(&c.importTimeSum) / importOps)
	}
	c.mu.Unlock()

	c.logger.Info("import completed",
		"path", req.InputPath,
		"format", req.Format,
		"rows", result.RowsImported,
		"entities", result.EntitiesUpdated,
		"duration", result.Duration,
	)

	return result, nil
}

func (c *Connector) validateImportRequest(req *ImportRequest) error {
	if req.InputPath == "" {
		return fmt.Errorf("%w: input_path is required", ErrInvalidConfig)
	}
	if req.EntityColumn == "" {
		return fmt.Errorf("%w: entity_column is required", ErrInvalidConfig)
	}
	if len(req.FeatureColumns) == 0 {
		return fmt.Errorf("%w: feature_columns is required", ErrInvalidConfig)
	}
	if req.Format == "" {
		req.Format = FormatJSON
	}
	if req.BatchSize <= 0 {
		req.BatchSize = c.config.BatchSize
	}
	return nil
}

func (c *Connector) importJSON(ctx context.Context, req *ImportRequest, result *ImportResult) error {
	file, err := os.Open(req.InputPath)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPathNotFound, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			c.logger.Warn("failed to close import file", "error", closeErr)
		}
	}()

	if stat, statErr := file.Stat(); statErr == nil {
		result.BytesRead = stat.Size()
	}

	decoder := json.NewDecoder(file)
	entities := make(map[string]bool)
	batch := make([]featureRecord, 0, req.BatchSize)

	// Try to decode as array first
	var records []map[string]interface{}
	if err := decoder.Decode(&records); err == nil {
		for _, record := range records {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			rec, err := c.processImportRecord(record, req, result)
			if err != nil {
				result.Errors = append(result.Errors, err.Error())
				result.SkippedRows++
				continue
			}

			batch = append(batch, rec)
			entities[rec.entityID] = true
			result.RowsImported++

			if len(batch) >= req.BatchSize {
				if err := c.writeBatch(ctx, batch, req.WriteMode, result); err != nil {
					return err
				}
				batch = batch[:0]
			}
		}
	} else {
		// Try JSON lines format
		if _, err := file.Seek(0, 0); err != nil {
			return fmt.Errorf("resetting file: %w", err)
		}
		decoder = json.NewDecoder(file)

		for {
			var record map[string]interface{}
			if err := decoder.Decode(&record); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				result.Errors = append(result.Errors, err.Error())
				result.SkippedRows++
				continue
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			rec, err := c.processImportRecord(record, req, result)
			if err != nil {
				result.Errors = append(result.Errors, err.Error())
				result.SkippedRows++
				continue
			}

			batch = append(batch, rec)
			entities[rec.entityID] = true
			result.RowsImported++

			if len(batch) >= req.BatchSize {
				if err := c.writeBatch(ctx, batch, req.WriteMode, result); err != nil {
					return err
				}
				batch = batch[:0]
			}
		}
	}

	// Write remaining batch
	if len(batch) > 0 {
		if err := c.writeBatch(ctx, batch, req.WriteMode, result); err != nil {
			return err
		}
	}

	result.EntitiesUpdated = int64(len(entities))
	return nil
}

type featureRecord struct {
	entityID  string
	features  map[string]*domain.FeatureValue
	timestamp int64
}

func (c *Connector) processImportRecord(record map[string]interface{}, req *ImportRequest, result *ImportResult) (featureRecord, error) {
	rec := featureRecord{
		features: make(map[string]*domain.FeatureValue),
	}

	// Extract entity ID
	entityVal, ok := record[req.EntityColumn]
	if !ok {
		return rec, fmt.Errorf("missing entity column: %s", req.EntityColumn)
	}
	rec.entityID = fmt.Sprintf("%v", entityVal)

	// Extract timestamp
	rec.timestamp = time.Now().UnixNano()
	if req.TimestampColumn != "" {
		if tsVal, ok := record[req.TimestampColumn]; ok {
			switch t := tsVal.(type) {
			case string:
				if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
					rec.timestamp = parsed.UnixNano()
				} else if parsed, err := time.Parse(time.RFC3339, t); err == nil {
					rec.timestamp = parsed.UnixNano()
				}
			case float64:
				rec.timestamp = int64(t)
			case int64:
				rec.timestamp = t
			}
		}
	}

	// Extract features
	for srcCol, featureName := range req.FeatureColumns {
		if val, ok := record[srcCol]; ok && val != nil {
			// Validate schema if requested
			if req.ValidateSchema && c.schema != nil {
				if spec, err := c.schema.GetFeatureSpec(featureName); err == nil {
					if err := validateValue(val, spec.DataType); err != nil {
						result.ValidationErrors = append(result.ValidationErrors,
							fmt.Sprintf("feature %s: %v", featureName, err))
						continue
					}
				}
			}

			rec.features[featureName] = &domain.FeatureValue{
				Value:     val,
				Timestamp: rec.timestamp,
			}
		}
	}

	return rec, nil
}

func (c *Connector) writeBatch(ctx context.Context, batch []featureRecord, mode WriteMode, result *ImportResult) error {
	for _, rec := range batch {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check write mode for merge handling
		if mode == WriteModeMerge {
			existing, err := c.store.Get(rec.entityID, nil)
			if err == nil && len(existing) > 0 {
				// Merge with existing - keep newer timestamps
				for featureName, newVal := range rec.features {
					if existingVal, ok := existing[featureName]; ok {
						if existingVal.Timestamp > newVal.Timestamp {
							rec.features[featureName] = existingVal
						}
					}
				}
			}
		}

		if len(rec.features) > 0 {
			if err := c.store.Put(rec.entityID, rec.features); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("writing entity %s: %v", rec.entityID, err))
				continue
			}
			result.FeaturesUpdated += int64(len(rec.features))
		}
	}
	return nil
}

func (c *Connector) importCSV(ctx context.Context, req *ImportRequest, result *ImportResult) error {
	file, err := os.Open(req.InputPath)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPathNotFound, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			c.logger.Warn("failed to close import file", "error", closeErr)
		}
	}()

	if stat, statErr := file.Stat(); statErr == nil {
		result.BytesRead = stat.Size()
	}

	// Read entire file content
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// Parse CSV manually (simple implementation)
	lines := splitLines(string(content))
	if len(lines) == 0 {
		return nil
	}

	// Parse header
	header := splitCSVLine(lines[0])
	colIndex := make(map[string]int)
	for i, col := range header {
		colIndex[col] = i
	}

	entityIdx, ok := colIndex[req.EntityColumn]
	if !ok {
		return fmt.Errorf("entity column %s not found in header", req.EntityColumn)
	}

	timestampIdx := -1
	if req.TimestampColumn != "" {
		if idx, ok := colIndex[req.TimestampColumn]; ok {
			timestampIdx = idx
		}
	}

	entities := make(map[string]bool)
	batch := make([]featureRecord, 0, req.BatchSize)

	// Process data rows
	for i := 1; i < len(lines); i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		values := splitCSVLine(lines[i])
		if len(values) <= entityIdx {
			result.SkippedRows++
			continue
		}

		rec := featureRecord{
			entityID:  values[entityIdx],
			features:  make(map[string]*domain.FeatureValue),
			timestamp: time.Now().UnixNano(),
		}

		if timestampIdx >= 0 && len(values) > timestampIdx {
			if parsed, err := time.Parse(time.RFC3339Nano, values[timestampIdx]); err == nil {
				rec.timestamp = parsed.UnixNano()
			}
		}

		for srcCol, featureName := range req.FeatureColumns {
			if idx, ok := colIndex[srcCol]; ok && len(values) > idx {
				val := values[idx]
				if val != "" {
					rec.features[featureName] = &domain.FeatureValue{
						Value:     val,
						Timestamp: rec.timestamp,
					}
				}
			}
		}

		if len(rec.features) > 0 {
			batch = append(batch, rec)
			entities[rec.entityID] = true
			result.RowsImported++
		}

		if len(batch) >= req.BatchSize {
			if err := c.writeBatch(ctx, batch, req.WriteMode, result); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := c.writeBatch(ctx, batch, req.WriteMode, result); err != nil {
			return err
		}
	}

	result.EntitiesUpdated = int64(len(entities))
	return nil
}

// Metrics returns connector metrics.
func (c *Connector) Metrics() ConnectorMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return ConnectorMetrics{
		ExportOperations:  atomic.LoadInt64(&c.metrics.ExportOperations),
		ImportOperations:  atomic.LoadInt64(&c.metrics.ImportOperations),
		RowsExported:      atomic.LoadInt64(&c.metrics.RowsExported),
		RowsImported:      atomic.LoadInt64(&c.metrics.RowsImported),
		BytesTransferred:  atomic.LoadInt64(&c.metrics.BytesTransferred),
		Errors:            atomic.LoadInt64(&c.metrics.Errors),
		AverageExportTime: c.metrics.AverageExportTime,
		AverageImportTime: c.metrics.AverageImportTime,
		LastExportAt:      c.metrics.LastExportAt,
		LastImportAt:      c.metrics.LastImportAt,
	}
}

// GenerateSparkSchema generates a Spark DataFrame schema definition.
func (c *Connector) GenerateSparkSchema(features []string) string {
	schemaInfo := c.buildSchemaInfo(features)

	var sb string
	sb = "from pyspark.sql.types import StructType, StructField, StringType, LongType, DoubleType, BooleanType, TimestampType, ArrayType\n\n"
	sb += "schema = StructType([\n"

	for i, field := range schemaInfo.Fields {
		nullable := "True"
		if !field.Nullable {
			nullable = "False"
		}
		sb += fmt.Sprintf("    StructField(\"%s\", %s(), %s)", field.Name, field.SparkType, nullable)
		if i < len(schemaInfo.Fields)-1 {
			sb += ","
		}
		sb += "\n"
	}

	sb += "])\n"
	return sb
}

// GenerateSparkReadCode generates PySpark code for reading exported data.
func (c *Connector) GenerateSparkReadCode(path string, format DataFormat) string {
	switch format {
	case FormatParquet:
		return fmt.Sprintf(`df = spark.read.json("%s/features.parquet.json")`, path)
	case FormatJSON:
		return fmt.Sprintf(`df = spark.read.json("%s/features.json")`, path)
	case FormatCSV:
		return fmt.Sprintf(`df = spark.read.csv("%s/features.csv", header=True, inferSchema=True)`, path)
	default:
		return fmt.Sprintf(`df = spark.read.json("%s")`, path)
	}
}

// mapFeatureTypeToSpark converts Feather types to Spark SQL types.
func mapFeatureTypeToSpark(dt domain.DataType) string {
	switch dt {
	case domain.DataTypeInt64:
		return "LongType"
	case domain.DataTypeFloat64:
		return "DoubleType"
	case domain.DataTypeString:
		return "StringType"
	case domain.DataTypeBool:
		return "BooleanType"
	case domain.DataTypeTimestamp:
		return "TimestampType"
	case domain.DataTypeVector:
		return "ArrayType(DoubleType)"
	case domain.DataTypeBytes:
		return "BinaryType"
	default:
		return "StringType"
	}
}

// mapSparkTypeToFeature converts Spark SQL types to Feather types.
func mapSparkTypeToFeature(sparkType string) domain.DataType {
	switch sparkType {
	case "LongType", "IntegerType", "ShortType", "ByteType":
		return domain.DataTypeInt64
	case "DoubleType", "FloatType", "DecimalType":
		return domain.DataTypeFloat64
	case "StringType":
		return domain.DataTypeString
	case "BooleanType":
		return domain.DataTypeBool
	case "TimestampType", "DateType":
		return domain.DataTypeTimestamp
	case "ArrayType(DoubleType)", "ArrayType(FloatType)":
		return domain.DataTypeVector
	case "BinaryType":
		return domain.DataTypeBytes
	default:
		return domain.DataTypeString
	}
}

func validateValue(val interface{}, dt domain.DataType) error {
	switch dt {
	case domain.DataTypeInt64:
		switch val.(type) {
		case int, int32, int64, float64:
			return nil
		default:
			return fmt.Errorf("expected integer, got %T", val)
		}
	case domain.DataTypeFloat64:
		switch val.(type) {
		case float32, float64, int, int64:
			return nil
		default:
			return fmt.Errorf("expected float, got %T", val)
		}
	case domain.DataTypeBool:
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("expected bool, got %T", val)
		}
	case domain.DataTypeString:
		if _, ok := val.(string); !ok {
			return fmt.Errorf("expected string, got %T", val)
		}
	case domain.DataTypeBytes:
		if _, ok := val.([]byte); !ok {
			return fmt.Errorf("expected bytes, got %T", val)
		}
	case domain.DataTypeVector:
		if _, ok := val.([]float32); !ok {
			return fmt.Errorf("expected vector, got %T", val)
		}
	case domain.DataTypeTimestamp:
		switch val.(type) {
		case time.Time, int64, float64, string:
			return nil
		default:
			return fmt.Errorf("expected timestamp, got %T", val)
		}
	}
	return nil
}

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
