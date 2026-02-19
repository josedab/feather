package warehouse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

// BigQueryConfig contains configuration for the BigQuery connector.
type BigQueryConfig struct {
	BaseConfig

	// ProjectID is the GCP project ID.
	ProjectID string `json:"project_id" yaml:"project_id"`

	// Dataset is the default dataset.
	Dataset string `json:"dataset" yaml:"dataset"`

	// Location is the dataset location (e.g., "US", "EU").
	Location string `json:"location" yaml:"location"`

	// CredentialsFile is the path to the service account JSON file.
	CredentialsFile string `json:"credentials_file,omitempty" yaml:"credentials_file,omitempty"`

	// CredentialsJSON is the service account credentials as JSON string.
	CredentialsJSON string `json:"-" yaml:"credentials_json,omitempty"`

	// UseDefaultCredentials uses Application Default Credentials.
	UseDefaultCredentials bool `json:"use_default_credentials" yaml:"use_default_credentials"`

	// EnableStreaming uses streaming inserts instead of batch loads.
	EnableStreaming bool `json:"enable_streaming" yaml:"enable_streaming"`

	// MaxStreamingRows is the maximum rows per streaming insert request.
	MaxStreamingRows int `json:"max_streaming_rows" yaml:"max_streaming_rows"`

	// JobTimeoutMinutes is the maximum time for batch jobs.
	JobTimeoutMinutes int `json:"job_timeout_minutes" yaml:"job_timeout_minutes"`
}

// DefaultBigQueryConfig returns the default BigQuery configuration.
func DefaultBigQueryConfig() BigQueryConfig {
	return BigQueryConfig{
		BaseConfig:            DefaultBaseConfig(),
		Location:              "US",
		UseDefaultCredentials: true,
		EnableStreaming:       true,
		MaxStreamingRows:      10000,
		JobTimeoutMinutes:     30,
	}
}

// Validate validates the BigQuery configuration.
func (c *BigQueryConfig) Validate() error {
	if c.ProjectID == "" {
		return fmt.Errorf("%w: project_id is required", ErrInvalidConfig)
	}
	if c.Dataset == "" {
		return fmt.Errorf("%w: dataset is required", ErrInvalidConfig)
	}
	if !c.UseDefaultCredentials && c.CredentialsFile == "" && c.CredentialsJSON == "" {
		return fmt.Errorf("%w: credentials_file, credentials_json, or use_default_credentials is required", ErrInvalidConfig)
	}
	return nil
}

// BigQueryConnector implements the Connector interface for BigQuery.
type BigQueryConnector struct {
	mu      sync.RWMutex
	config  BigQueryConfig
	client  BigQueryClient
	state   ConnectionState
	store   *storage.Store
	schema  storage.SchemaRegistry
	logger  *slog.Logger
	metrics ConnectorMetrics
}

// BigQueryClient abstracts the BigQuery client for testing.
type BigQueryClient interface {
	Close() error
	Query(ctx context.Context, query string) (BigQueryIterator, error)
	InsertRows(ctx context.Context, dataset, table string, rows []map[string]interface{}) error
	CreateTable(ctx context.Context, dataset, table string, schema BigQuerySchema) error
	TableExists(ctx context.Context, dataset, table string) (bool, error)
	GetTableMetadata(ctx context.Context, dataset, table string) (*BigQueryTableMetadata, error)
	ListTables(ctx context.Context, dataset string) ([]string, error)
}

// BigQueryIterator abstracts result iteration.
type BigQueryIterator interface {
	Next(dst interface{}) error
}

// BigQuerySchema represents a BigQuery table schema.
type BigQuerySchema struct {
	Fields []BigQueryFieldSchema `json:"fields"`
}

// BigQueryFieldSchema represents a single field in the schema.
type BigQueryFieldSchema struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Mode        string `json:"mode,omitempty"`
	Description string `json:"description,omitempty"`
}

// BigQueryTableMetadata contains table information.
type BigQueryTableMetadata struct {
	Name         string    `json:"name"`
	NumRows      uint64    `json:"num_rows"`
	NumBytes     int64     `json:"num_bytes"`
	CreationTime time.Time `json:"creation_time"`
	LastModified time.Time `json:"last_modified"`
	Schema       BigQuerySchema
}

// NewBigQueryConnector creates a new BigQuery connector.
func NewBigQueryConnector(config BigQueryConfig, store *storage.Store, schema storage.SchemaRegistry, logger *slog.Logger) (*BigQueryConnector, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &BigQueryConnector{
		config: config,
		store:  store,
		schema: schema,
		state:  ConnectionStateDisconnected,
		logger: logger,
	}, nil
}

// SetClient sets the BigQuery client (for testing).
func (c *BigQueryConnector) SetClient(client BigQueryClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.client = client
}

// Connect establishes a connection to BigQuery.
func (c *BigQueryConnector) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state == ConnectionStateConnected {
		return nil
	}

	c.state = ConnectionStateConnecting
	atomic.AddInt64(&c.metrics.ConnectionAttempts, 1)

	// In a real implementation, we would create the BigQuery client here
	// using cloud.google.com/go/bigquery
	// For now, we set state to connected if client is already set (for testing)
	if c.client != nil {
		c.state = ConnectionStateConnected
		c.metrics.LastConnectedAt = time.Now()
		c.logger.Info("connected to BigQuery",
			"project", c.config.ProjectID,
			"dataset", c.config.Dataset,
		)
		return nil
	}

	// Simulated connection (real impl would create actual client)
	c.state = ConnectionStateConnected
	c.metrics.LastConnectedAt = time.Now()

	c.logger.Info("connected to BigQuery",
		"project", c.config.ProjectID,
		"dataset", c.config.Dataset,
		"location", c.config.Location,
	)

	return nil
}

// Close closes the BigQuery connection.
func (c *BigQueryConnector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil {
		if err := c.client.Close(); err != nil {
			return fmt.Errorf("closing client: %w", err)
		}
		c.client = nil
	}

	c.state = ConnectionStateDisconnected
	c.logger.Info("disconnected from BigQuery")

	return nil
}

// State returns the current connection state.
func (c *BigQueryConnector) State() ConnectionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// Type returns the connector type.
func (c *BigQueryConnector) Type() ConnectorType {
	return ConnectorTypeBigQuery
}

// Ping verifies the connection is alive.
func (c *BigQueryConnector) Ping(ctx context.Context) error {
	c.mu.RLock()
	client := c.client
	state := c.state
	c.mu.RUnlock()

	if state != ConnectionStateConnected {
		return ErrConnectorNotConnected
	}

	if client == nil {
		return nil // Simulated mode
	}

	// Run a simple query to verify connection
	_, err := client.Query(ctx, "SELECT 1")
	return err
}

// Export exports features from Feather to BigQuery.
func (c *BigQueryConnector) Export(ctx context.Context, req *ExportRequest) (*ExportResult, error) {
	c.mu.RLock()
	client := c.client
	state := c.state
	c.mu.RUnlock()

	if state != ConnectionStateConnected {
		return nil, ErrConnectorNotConnected
	}

	start := time.Now()
	result := &ExportResult{
		Table:            req.Table,
		FeaturesExported: len(req.Features),
	}

	// Validate request
	if req.Table == "" {
		return nil, fmt.Errorf("%w: table is required", ErrInvalidConfig)
	}
	if len(req.Features) == 0 {
		return nil, fmt.Errorf("%w: features is required", ErrInvalidConfig)
	}

	dataset := c.config.Dataset
	if req.Schema != "" {
		dataset = req.Schema
	}

	// Create table if requested
	if req.CreateTable {
		if err := c.ensureTableExists(ctx, client, dataset, req); err != nil {
			return nil, fmt.Errorf("creating table: %w", err)
		}
	}

	// Get entities to export
	entities := req.Entities
	if len(entities) == 0 {
		// In a real implementation, we would iterate all entities
		result.Duration = time.Since(start)
		return result, nil
	}

	// Export in batches
	batchSize := c.config.BatchSize
	if batchSize == 0 {
		batchSize = 10000
	}

	if c.config.EnableStreaming && client != nil {
		// Use streaming inserts
		for i := 0; i < len(entities); i += batchSize {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			end := i + batchSize
			if end > len(entities) {
				end = len(entities)
			}

			batch := entities[i:end]
			rowsExported, bytesExported, err := c.exportBatchStreaming(ctx, client, dataset, req.Table, batch, req.Features)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("batch %d: %v", i/batchSize, err))
				continue
			}

			result.RowsExported += rowsExported
			result.BytesExported += bytesExported
			result.EntitiesExported += int64(len(batch))
		}
	} else {
		// Simulated export for testing
		for _, entity := range entities {
			featureValues, err := c.store.Get(ctx, entity, req.Features)
			if err != nil {
				if errors.Is(err, domain.ErrEntityNotFound) {
					continue
				}
				result.Errors = append(result.Errors, fmt.Sprintf("get %s: %v", entity, err))
				continue
			}

			if len(featureValues) > 0 {
				result.RowsExported++
				result.EntitiesExported++
				result.BytesExported += int64(len(entity) * len(req.Features) * 8)
			}
		}
	}

	result.Duration = time.Since(start)

	atomic.AddInt64(&c.metrics.RowsExported, result.RowsExported)
	atomic.AddInt64(&c.metrics.BytesTransferred, result.BytesExported)
	c.mu.Lock()
	c.metrics.LastExportAt = time.Now()
	c.mu.Unlock()

	c.logger.Info("export completed",
		"table", req.Table,
		"rows", result.RowsExported,
		"entities", result.EntitiesExported,
		"duration", result.Duration,
	)

	return result, nil
}

// ensureTableExists creates the table if it doesn't exist.
func (c *BigQueryConnector) ensureTableExists(ctx context.Context, client BigQueryClient, dataset string, req *ExportRequest) error {
	if client == nil {
		return nil // Simulated mode
	}

	exists, err := client.TableExists(ctx, dataset, req.Table)
	if err != nil {
		return fmt.Errorf("checking table existence: %w", err)
	}

	if exists {
		return nil
	}

	// Build schema
	schema := BigQuerySchema{
		Fields: []BigQueryFieldSchema{
			{Name: "entity_key", Type: "STRING", Mode: "REQUIRED"},
			{Name: "timestamp", Type: "TIMESTAMP", Mode: "REQUIRED"},
			{Name: "version", Type: "INT64", Mode: "NULLABLE"},
		},
	}

	for _, feature := range req.Features {
		spec, err := c.schema.GetFeatureSpec(feature)
		var bqType string
		if err != nil {
			bqType = "STRING"
		} else {
			bqType = mapFeatureTypeToBigQuery(spec.DataType)
		}

		schema.Fields = append(schema.Fields, BigQueryFieldSchema{
			Name: feature,
			Type: bqType,
			Mode: "NULLABLE",
		})
	}

	return client.CreateTable(ctx, dataset, req.Table, schema)
}

// exportBatchStreaming exports a batch using streaming inserts.
func (c *BigQueryConnector) exportBatchStreaming(ctx context.Context, client BigQueryClient, dataset, table string, entities []string, features []string) (int64, int64, error) {
	if client == nil {
		return 0, 0, nil
	}

	rows := make([]map[string]interface{}, 0, len(entities))
	var bytesExported int64

	for _, entity := range entities {
		featureValues, err := c.store.Get(ctx, entity, features)
		if err != nil {
			if errors.Is(err, domain.ErrEntityNotFound) {
				continue
			}
			return 0, 0, fmt.Errorf("getting entity %s: %w", entity, err)
		}

		row := map[string]interface{}{
			"entity_key": entity,
			"timestamp":  time.Now(),
		}

		var maxTimestamp int64
		for _, f := range features {
			if fv, ok := featureValues[f]; ok {
				serialized, err := serializeValueForBigQuery(fv.Value)
				if err != nil {
					return 0, 0, fmt.Errorf("serializing %s: %w", f, err)
				}
				row[f] = serialized
				bytesExported += int64(len(fmt.Sprintf("%v", serialized)))
				if fv.Timestamp > maxTimestamp {
					maxTimestamp = fv.Timestamp
				}
			}
		}

		if maxTimestamp > 0 {
			row["timestamp"] = time.Unix(0, maxTimestamp)
		}

		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return 0, 0, nil
	}

	if err := client.InsertRows(ctx, dataset, table, rows); err != nil {
		return 0, 0, fmt.Errorf("inserting rows: %w", err)
	}

	return int64(len(rows)), bytesExported, nil
}

// Import imports data from BigQuery to Feather.
func (c *BigQueryConnector) Import(ctx context.Context, req *ImportRequest) (*ImportResult, error) {
	c.mu.RLock()
	client := c.client
	state := c.state
	c.mu.RUnlock()

	if state != ConnectionStateConnected {
		return nil, ErrConnectorNotConnected
	}

	start := time.Now()
	result := &ImportResult{}

	// Validate request
	if req.Table == "" && req.Query == "" {
		return nil, fmt.Errorf("%w: table or query is required", ErrInvalidConfig)
	}
	if req.EntityColumn == "" {
		return nil, fmt.Errorf("%w: entity_column is required", ErrInvalidConfig)
	}
	if len(req.FeatureColumns) == 0 {
		return nil, fmt.Errorf("%w: feature_columns is required", ErrInvalidConfig)
	}

	// Build query
	query := req.Query
	if query == "" {
		var buildErr error
		query, buildErr = c.buildImportQuery(req)
		if buildErr != nil {
			return nil, fmt.Errorf("building import query: %w", buildErr)
		}
	}

	// If no client, return simulated result
	if client == nil {
		result.Duration = time.Since(start)
		return result, nil
	}

	// Execute query
	iter, err := client.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrQueryFailed, err)
	}

	// Process rows
	entities := make(map[string]bool)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var row map[string]interface{}
		if err := iter.Next(&row); err != nil {
			if errors.Is(err, ErrIteratorDone) {
				break
			}
			return nil, fmt.Errorf("iterating rows: %w", err)
		}

		// Extract entity key
		entityKey, ok := row[req.EntityColumn].(string)
		if !ok {
			result.SkippedRows++
			continue
		}

		// Extract timestamp
		var timestamp int64
		if req.TimestampColumn != "" {
			if ts, ok := row[req.TimestampColumn].(time.Time); ok {
				timestamp = ts.UnixNano()
			}
		}
		if timestamp == 0 {
			timestamp = time.Now().UnixNano()
		}

		// Build feature map
		features := make(map[string]*domain.FeatureValue)
		for col, featureName := range req.FeatureColumns {
			if value, ok := row[col]; ok && value != nil {
				features[featureName] = &domain.FeatureValue{
					Value:     value,
					Timestamp: timestamp,
				}
			}
		}

		// Store in Feather
		if len(features) > 0 {
			if err := c.store.Put(ctx, entityKey, features); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("put %s: %v", entityKey, err))
				continue
			}
			result.FeaturesUpdated += int64(len(features))
			entities[entityKey] = true
		}

		result.RowsImported++
	}

	result.EntitiesUpdated = int64(len(entities))
	result.Duration = time.Since(start)

	atomic.AddInt64(&c.metrics.RowsImported, result.RowsImported)
	c.mu.Lock()
	c.metrics.LastImportAt = time.Now()
	c.mu.Unlock()

	c.logger.Info("import completed",
		"table", req.Table,
		"rows", result.RowsImported,
		"entities", result.EntitiesUpdated,
		"duration", result.Duration,
	)

	return result, nil
}

// buildImportQuery constructs the SELECT query for import.
func (c *BigQueryConnector) buildImportQuery(req *ImportRequest) (string, error) {
	if err := validateIdentifiers(req.EntityColumn, req.Table); err != nil {
		return "", fmt.Errorf("buildImportQuery: %w", err)
	}
	if req.TimestampColumn != "" {
		if err := validateIdentifier(req.TimestampColumn); err != nil {
			return "", fmt.Errorf("buildImportQuery timestamp: %w", err)
		}
	}
	for col := range req.FeatureColumns {
		if err := validateIdentifier(col); err != nil {
			return "", fmt.Errorf("buildImportQuery column: %w", err)
		}
	}

	dataset := c.config.Dataset
	if req.Schema != "" {
		dataset = req.Schema
	}
	if err := validateProjectID(dataset); err != nil {
		return "", fmt.Errorf("buildImportQuery dataset: %w", err)
	}
	if err := validateProjectID(c.config.ProjectID); err != nil {
		return "", fmt.Errorf("buildImportQuery project: %w", err)
	}

	if req.Filter != "" {
		return "", fmt.Errorf("%w: raw SQL filters are not allowed", ErrUnsafeFilter)
	}

	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(req.EntityColumn)

	if req.TimestampColumn != "" {
		sb.WriteString(", ")
		sb.WriteString(req.TimestampColumn)
	}

	for col := range req.FeatureColumns {
		sb.WriteString(", ")
		sb.WriteString(col)
	}

	sb.WriteString(" FROM `")
	sb.WriteString(fmt.Sprintf("%s.%s.%s", c.config.ProjectID, dataset, req.Table))
	sb.WriteString("`")

	if req.Limit > 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", req.Limit))
	}

	return sb.String(), nil
}

// ListTables returns available tables in BigQuery.
func (c *BigQueryConnector) ListTables(ctx context.Context, dataset string) ([]TableInfo, error) {
	c.mu.RLock()
	client := c.client
	state := c.state
	c.mu.RUnlock()

	if state != ConnectionStateConnected {
		return nil, ErrConnectorNotConnected
	}

	if dataset == "" {
		dataset = c.config.Dataset
	}

	if client == nil {
		return []TableInfo{}, nil // Simulated mode
	}

	tableNames, err := client.ListTables(ctx, dataset)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrQueryFailed, err)
	}

	tables := make([]TableInfo, 0, len(tableNames))
	for _, name := range tableNames {
		metadata, err := client.GetTableMetadata(ctx, dataset, name)
		if err != nil {
			continue
		}

		tables = append(tables, TableInfo{
			Name:         name,
			Schema:       dataset,
			RowCount:     clampUint64ToInt64(metadata.NumRows),
			SizeBytes:    metadata.NumBytes,
			CreatedAt:    metadata.CreationTime,
			LastModified: metadata.LastModified,
		})
	}

	return tables, nil
}

// GetTableSchema returns the schema for a specific table.
func (c *BigQueryConnector) GetTableSchema(ctx context.Context, table string) (*TableSchema, error) {
	c.mu.RLock()
	client := c.client
	state := c.state
	c.mu.RUnlock()

	if state != ConnectionStateConnected {
		return nil, ErrConnectorNotConnected
	}

	// Parse table name
	parts := strings.Split(table, ".")
	dataset := c.config.Dataset
	tableName := table
	if len(parts) == 2 {
		dataset = parts[0]
		tableName = parts[1]
	}

	if client == nil {
		return &TableSchema{
			Table:  tableName,
			Schema: dataset,
		}, nil
	}

	metadata, err := client.GetTableMetadata(ctx, dataset, tableName)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTableNotFound, err)
	}

	schema := &TableSchema{
		Table:  tableName,
		Schema: dataset,
	}

	for _, field := range metadata.Schema.Fields {
		schema.Columns = append(schema.Columns, ColumnInfo{
			Name:        field.Name,
			Type:        field.Type,
			FeatureType: mapBigQueryTypeToFeature(field.Type),
			Nullable:    field.Mode != "REQUIRED",
			Description: field.Description,
		})
	}

	return schema, nil
}

// CreateTable creates a new table for feature storage.
func (c *BigQueryConnector) CreateTable(ctx context.Context, req *CreateTableRequest) error {
	c.mu.RLock()
	client := c.client
	state := c.state
	c.mu.RUnlock()

	if state != ConnectionStateConnected {
		return ErrConnectorNotConnected
	}

	if req.Table == "" {
		return fmt.Errorf("%w: table is required", ErrInvalidConfig)
	}

	dataset := c.config.Dataset
	if req.Schema != "" {
		dataset = req.Schema
	}

	if client == nil {
		return nil // Simulated mode
	}

	// Check if table exists
	if req.IfNotExists {
		exists, err := client.TableExists(ctx, dataset, req.Table)
		if err != nil {
			return fmt.Errorf("checking table existence: %w", err)
		}
		if exists {
			return nil
		}
	}

	// Build schema
	schema := BigQuerySchema{
		Fields: []BigQueryFieldSchema{
			{Name: "entity_key", Type: "STRING", Mode: "REQUIRED"},
			{Name: "timestamp", Type: "TIMESTAMP", Mode: "REQUIRED"},
			{Name: "version", Type: "INT64", Mode: "NULLABLE"},
		},
	}

	for _, feature := range req.Features {
		bqType := mapFeatureTypeToBigQuery(feature.DataType)
		mode := "NULLABLE"
		if feature.Validation != nil && feature.Validation.NotNull {
			mode = "REQUIRED"
		}

		schema.Fields = append(schema.Fields, BigQueryFieldSchema{
			Name:        feature.Name,
			Type:        bqType,
			Mode:        mode,
			Description: feature.Name,
		})
	}

	if err := client.CreateTable(ctx, dataset, req.Table, schema); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return ErrTableExists
		}
		return fmt.Errorf("creating table: %w", err)
	}

	c.logger.Info("created table",
		"dataset", dataset,
		"table", req.Table,
		"features", len(req.Features),
	)

	return nil
}

// ExecuteQuery executes a read-only query and returns results.
func (c *BigQueryConnector) ExecuteQuery(ctx context.Context, query string) (*QueryResult, error) {
	c.mu.RLock()
	client := c.client
	state := c.state
	c.mu.RUnlock()

	if state != ConnectionStateConnected {
		return nil, ErrConnectorNotConnected
	}

	start := time.Now()
	atomic.AddInt64(&c.metrics.QueriesExecuted, 1)

	if client == nil {
		return &QueryResult{
			Duration: time.Since(start),
		}, nil
	}

	iter, err := client.Query(ctx, query)
	if err != nil {
		atomic.AddInt64(&c.metrics.QueryErrors, 1)
		return nil, fmt.Errorf("%w: %w", ErrQueryFailed, err)
	}

	result := &QueryResult{}

	for {
		var row map[string]interface{}
		if err := iter.Next(&row); err != nil {
			if errors.Is(err, ErrIteratorDone) {
				break
			}
			return nil, fmt.Errorf("iterating rows: %w", err)
		}

		// Build columns on first row
		if len(result.Columns) == 0 {
			for col := range row {
				result.Columns = append(result.Columns, col)
			}
		}

		// Add row values in column order
		values := make([]interface{}, len(result.Columns))
		for i, col := range result.Columns {
			values[i] = row[col]
		}
		result.Rows = append(result.Rows, values)
		result.RowCount++
	}

	result.Duration = time.Since(start)

	return result, nil
}

func clampUint64ToInt64(value uint64) int64 {
	if value > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(value)
}

// Metrics returns connector performance metrics.
func (c *BigQueryConnector) Metrics() ConnectorMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return ConnectorMetrics{
		ConnectionAttempts: atomic.LoadInt64(&c.metrics.ConnectionAttempts),
		ConnectionFailures: atomic.LoadInt64(&c.metrics.ConnectionFailures),
		QueriesExecuted:    atomic.LoadInt64(&c.metrics.QueriesExecuted),
		QueryErrors:        atomic.LoadInt64(&c.metrics.QueryErrors),
		RowsExported:       atomic.LoadInt64(&c.metrics.RowsExported),
		RowsImported:       atomic.LoadInt64(&c.metrics.RowsImported),
		BytesTransferred:   atomic.LoadInt64(&c.metrics.BytesTransferred),
		LastConnectedAt:    c.metrics.LastConnectedAt,
		LastExportAt:       c.metrics.LastExportAt,
		LastImportAt:       c.metrics.LastImportAt,
	}
}

// serializeValueForBigQuery converts values for BigQuery storage.
func serializeValueForBigQuery(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case []float32:
		// Convert to []float64 for BigQuery
		f64 := make([]float64, len(v))
		for i, f := range v {
			f64[i] = float64(f)
		}
		return f64, nil
	case []float64:
		return v, nil
	case []byte:
		return v, nil
	case time.Time:
		return v, nil
	case map[string]interface{}:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return string(data), nil
	default:
		return v, nil
	}
}
