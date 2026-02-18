package warehouse

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

// SnowflakeConfig contains configuration for the Snowflake connector.
type SnowflakeConfig struct {
	BaseConfig

	// Account is the Snowflake account identifier.
	Account string `json:"account" yaml:"account"`

	// User is the authentication username.
	User string `json:"user" yaml:"user"`

	// Password is the authentication password (use PrivateKey for key-pair auth).
	Password string `json:"-" yaml:"password,omitempty"`

	// PrivateKey is the private key for key-pair authentication (PEM format).
	PrivateKey string `json:"-" yaml:"private_key,omitempty"`

	// PrivateKeyPassphrase is the passphrase for encrypted private keys.
	PrivateKeyPassphrase string `json:"-" yaml:"private_key_passphrase,omitempty"`

	// Database is the default database.
	Database string `json:"database" yaml:"database"`

	// Schema is the default schema.
	Schema string `json:"schema" yaml:"schema"`

	// Warehouse is the compute warehouse to use.
	Warehouse string `json:"warehouse" yaml:"warehouse"`

	// Role is the role to assume after connecting.
	Role string `json:"role,omitempty" yaml:"role,omitempty"`

	// Region is the Snowflake region (for non-standard regions).
	Region string `json:"region,omitempty" yaml:"region,omitempty"`

	// UseStagingArea uses internal staging for bulk operations.
	UseStagingArea bool `json:"use_staging_area" yaml:"use_staging_area"`

	// StagingSchema is the schema for staging tables.
	StagingSchema string `json:"staging_schema,omitempty" yaml:"staging_schema,omitempty"`
}

// DefaultSnowflakeConfig returns the default Snowflake configuration.
func DefaultSnowflakeConfig() SnowflakeConfig {
	return SnowflakeConfig{
		BaseConfig:     DefaultBaseConfig(),
		Schema:         "PUBLIC",
		UseStagingArea: true,
		StagingSchema:  "FEATHER_STAGING",
	}
}

// Validate validates the Snowflake configuration.
func (c *SnowflakeConfig) Validate() error {
	if c.Account == "" {
		return fmt.Errorf("%w: account is required", ErrInvalidConfig)
	}
	if c.User == "" {
		return fmt.Errorf("%w: user is required", ErrInvalidConfig)
	}
	if c.Password == "" && c.PrivateKey == "" {
		return fmt.Errorf("%w: password or private_key is required", ErrInvalidConfig)
	}
	if c.Database == "" {
		return fmt.Errorf("%w: database is required", ErrInvalidConfig)
	}
	if c.Warehouse == "" {
		return fmt.Errorf("%w: warehouse is required", ErrInvalidConfig)
	}
	return nil
}

// SnowflakeConnector implements the Connector interface for Snowflake.
type SnowflakeConnector struct {
	mu      sync.RWMutex
	config  SnowflakeConfig
	db      *sql.DB
	state   ConnectionState
	store   *storage.Store
	schema  storage.SchemaRegistry
	logger  *slog.Logger
	metrics ConnectorMetrics
}

// NewSnowflakeConnector creates a new Snowflake connector.
func NewSnowflakeConnector(config SnowflakeConfig, store *storage.Store, schema storage.SchemaRegistry, logger *slog.Logger) (*SnowflakeConnector, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &SnowflakeConnector{
		config: config,
		store:  store,
		schema: schema,
		state:  ConnectionStateDisconnected,
		logger: logger,
	}, nil
}

// Connect establishes a connection to Snowflake.
func (c *SnowflakeConnector) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state == ConnectionStateConnected {
		return nil
	}

	c.state = ConnectionStateConnecting
	atomic.AddInt64(&c.metrics.ConnectionAttempts, 1)

	// Build connection string
	connStr := c.buildConnectionString()

	// Open connection
	db, err := sql.Open("snowflake", connStr)
	if err != nil {
		c.state = ConnectionStateFailed
		atomic.AddInt64(&c.metrics.ConnectionFailures, 1)
		return fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// Test connection
	pingCtx, cancel := context.WithTimeout(ctx, c.config.ConnectionTimeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			c.logger.Warn("failed to close Snowflake connection after ping failure", "error", closeErr)
		}
		c.state = ConnectionStateFailed
		atomic.AddInt64(&c.metrics.ConnectionFailures, 1)
		return fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}

	c.db = db
	c.state = ConnectionStateConnected
	c.metrics.LastConnectedAt = time.Now()

	c.logger.Info("connected to Snowflake",
		"account", c.config.Account,
		"database", c.config.Database,
		"warehouse", c.config.Warehouse,
	)

	return nil
}

// buildConnectionString constructs the Snowflake DSN.
func (c *SnowflakeConnector) buildConnectionString() string {
	var sb strings.Builder

	sb.WriteString(c.config.User)
	if c.config.Password != "" {
		sb.WriteString(":")
		sb.WriteString(c.config.Password)
	}
	sb.WriteString("@")
	sb.WriteString(c.config.Account)

	if c.config.Region != "" {
		sb.WriteString(".")
		sb.WriteString(c.config.Region)
	}

	sb.WriteString("/")
	sb.WriteString(c.config.Database)
	sb.WriteString("/")
	sb.WriteString(c.config.Schema)

	sb.WriteString("?warehouse=")
	sb.WriteString(c.config.Warehouse)

	if c.config.Role != "" {
		sb.WriteString("&role=")
		sb.WriteString(c.config.Role)
	}

	return sb.String()
}

// Close closes the Snowflake connection.
func (c *SnowflakeConnector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db != nil {
		if err := c.db.Close(); err != nil {
			return fmt.Errorf("closing connection: %w", err)
		}
		c.db = nil
	}

	c.state = ConnectionStateDisconnected
	c.logger.Info("disconnected from Snowflake")

	return nil
}

// State returns the current connection state.
func (c *SnowflakeConnector) State() ConnectionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// Type returns the connector type.
func (c *SnowflakeConnector) Type() ConnectorType {
	return ConnectorTypeSnowflake
}

// Ping verifies the connection is alive.
func (c *SnowflakeConnector) Ping(ctx context.Context) error {
	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	if db == nil {
		return ErrConnectorNotConnected
	}

	return db.PingContext(ctx)
}

// Export exports features from Feather to Snowflake.
func (c *SnowflakeConnector) Export(ctx context.Context, req *ExportRequest) (*ExportResult, error) {
	// Validate request first
	if req.Table == "" {
		return nil, fmt.Errorf("%w: table is required", ErrInvalidConfig)
	}
	if len(req.Features) == 0 {
		return nil, fmt.Errorf("%w: features is required", ErrInvalidConfig)
	}

	c.mu.RLock()
	db := c.db
	state := c.state
	c.mu.RUnlock()

	if state != ConnectionStateConnected || db == nil {
		return nil, ErrConnectorNotConnected
	}

	start := time.Now()
	result := &ExportResult{
		Table:            req.Table,
		FeaturesExported: len(req.Features),
	}

	// Create table if requested
	if req.CreateTable {
		if err := c.ensureTable(ctx, req); err != nil {
			return nil, fmt.Errorf("creating table: %w", err)
		}
	}

	// Get entities to export
	entities, err := c.getExportEntities(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("getting entities: %w", err)
	}

	if len(entities) == 0 {
		result.Duration = time.Since(start)
		return result, nil
	}

	// Export in batches
	batchSize := c.config.BatchSize
	if batchSize == 0 {
		batchSize = 10000
	}

	schema := c.config.Schema
	if req.Schema != "" {
		schema = req.Schema
	}

	var exportedEntities int64
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
		rowsExported, bytesExported, err := c.exportBatch(ctx, db, schema, req.Table, batch, req.Features)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("batch %d: %v", i/batchSize, err))
			continue
		}

		result.RowsExported += rowsExported
		result.BytesExported += bytesExported
		exportedEntities += int64(len(batch))
	}

	result.EntitiesExported = exportedEntities
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

// ensureTable creates the table if it doesn't exist.
func (c *SnowflakeConnector) ensureTable(ctx context.Context, req *ExportRequest) error {
	schema := c.config.Schema
	if req.Schema != "" {
		schema = req.Schema
	}

	if err := validateIdentifiers(schema, req.Table); err != nil {
		return fmt.Errorf("ensureTable: %w", err)
	}
	for _, f := range req.Features {
		if err := validateIdentifier(f); err != nil {
			return fmt.Errorf("ensureTable feature: %w", err)
		}
	}
	for _, p := range req.PartitionBy {
		if err := validateIdentifier(p); err != nil {
			return fmt.Errorf("ensureTable partition: %w", err)
		}
	}

	// Build CREATE TABLE statement
	var sb strings.Builder
	sb.WriteString("CREATE TABLE IF NOT EXISTS ")
	sb.WriteString(fmt.Sprintf("%s.%s", schema, req.Table))
	sb.WriteString(" (\n")
	sb.WriteString("  entity_key VARCHAR NOT NULL,\n")
	sb.WriteString("  timestamp TIMESTAMP_NTZ NOT NULL,\n")

	for i, feature := range req.Features {
		spec, err := c.schema.GetFeatureSpec(feature)
		if err != nil {
			sb.WriteString(fmt.Sprintf("  %s VARCHAR", feature))
		} else {
			snowflakeType := mapFeatureTypeToSnowflake(spec.DataType)
			sb.WriteString(fmt.Sprintf("  %s %s", feature, snowflakeType))
		}
		if i < len(req.Features)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(")")

	// Add clustering if specified
	if len(req.PartitionBy) > 0 {
		sb.WriteString(fmt.Sprintf(" CLUSTER BY (%s)", strings.Join(req.PartitionBy, ", ")))
	}

	_, err := c.db.ExecContext(ctx, sb.String())
	return err
}

// getExportEntities returns the entities to export.
func (c *SnowflakeConnector) getExportEntities(ctx context.Context, req *ExportRequest) ([]string, error) {
	if len(req.Entities) > 0 {
		return req.Entities, nil
	}

	// Get all entities from store (this would need iteration support in real impl)
	// For now, return empty to indicate all entities should be scanned
	return []string{}, nil
}

// exportBatch exports a batch of entities to Snowflake.
func (c *SnowflakeConnector) exportBatch(ctx context.Context, db *sql.DB, schema, table string, entities []string, features []string) (int64, int64, error) {
	if len(entities) == 0 {
		return 0, 0, nil
	}

	if err := validateIdentifiers(schema, table); err != nil {
		return 0, 0, fmt.Errorf("exportBatch: %w", err)
	}
	for _, f := range features {
		if err := validateIdentifier(f); err != nil {
			return 0, 0, fmt.Errorf("exportBatch feature: %w", err)
		}
	}

	// Build INSERT statement
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("INSERT INTO %s.%s (entity_key, timestamp", schema, table))
	for _, f := range features {
		sb.WriteString(", ")
		sb.WriteString(f)
	}
	sb.WriteString(") VALUES ")

	values := make([]interface{}, 0, len(entities)*(len(features)+2))
	var rowsExported int64
	var bytesExported int64

	for i, entity := range entities {
		// Get features from store
		featureValues, err := c.store.Get(entity, features)
		if err != nil {
			if errors.Is(err, domain.ErrEntityNotFound) {
				continue
			}
			return rowsExported, bytesExported, fmt.Errorf("getting entity %s: %w", entity, err)
		}

		if i > 0 {
			sb.WriteString(", ")
		}

		// Build value placeholders
		placeholders := make([]string, len(features)+2)
		placeholders[0] = "?"
		placeholders[1] = "?"
		for j := range features {
			placeholders[j+2] = "?"
		}
		sb.WriteString("(")
		sb.WriteString(strings.Join(placeholders, ", "))
		sb.WriteString(")")

		// Add values
		var timestamp int64
		values = append(values, entity)
		for _, f := range features {
			if fv, ok := featureValues[f]; ok {
				if fv.Timestamp > timestamp {
					timestamp = fv.Timestamp
				}
			}
		}
		values = append(values, time.Unix(0, timestamp))

		for _, f := range features {
			if fv, ok := featureValues[f]; ok {
				serialized, err := serializeValueForSnowflake(fv.Value)
				if err != nil {
					return rowsExported, bytesExported, fmt.Errorf("serializing %s: %w", f, err)
				}
				values = append(values, serialized)
				bytesExported += int64(len(fmt.Sprintf("%v", serialized)))
			} else {
				values = append(values, nil)
			}
		}

		rowsExported++
	}

	if rowsExported == 0 {
		return 0, 0, nil
	}

	_, err := db.ExecContext(ctx, sb.String(), values...)
	if err != nil {
		return 0, 0, fmt.Errorf("executing insert: %w", err)
	}

	return rowsExported, bytesExported, nil
}

// Import imports data from Snowflake to Feather.
func (c *SnowflakeConnector) Import(ctx context.Context, req *ImportRequest) (*ImportResult, error) {
	// Validate request first
	if req.Table == "" && req.Query == "" {
		return nil, fmt.Errorf("%w: table or query is required", ErrInvalidConfig)
	}
	if req.EntityColumn == "" {
		return nil, fmt.Errorf("%w: entity_column is required", ErrInvalidConfig)
	}
	if len(req.FeatureColumns) == 0 {
		return nil, fmt.Errorf("%w: feature_columns is required", ErrInvalidConfig)
	}

	c.mu.RLock()
	db := c.db
	state := c.state
	c.mu.RUnlock()

	if state != ConnectionStateConnected || db == nil {
		return nil, ErrConnectorNotConnected
	}

	start := time.Now()
	result := &ImportResult{}

	// Build query
	query := req.Query
	if query == "" {
		var buildErr error
		query, buildErr = c.buildImportQuery(req)
		if buildErr != nil {
			return nil, fmt.Errorf("building import query: %w", buildErr)
		}
	}

	// Execute query
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrQueryFailed, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			c.logger.Warn("failed to close Snowflake rows", "error", closeErr)
		}
	}()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("getting columns: %w", err)
	}

	// Find column indices
	entityIdx := -1
	timestampIdx := -1
	featureIndices := make(map[int]string)

	for i, col := range columns {
		if col == req.EntityColumn {
			entityIdx = i
		}
		if col == req.TimestampColumn {
			timestampIdx = i
		}
		if featureName, ok := req.FeatureColumns[col]; ok {
			featureIndices[i] = featureName
		}
	}

	if entityIdx == -1 {
		return nil, fmt.Errorf("%w: entity column %s not found", ErrSchemaMismatch, req.EntityColumn)
	}

	// Process rows
	entities := make(map[string]bool)
	for rows.Next() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Scan row
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			result.SkippedRows++
			result.Errors = append(result.Errors, fmt.Sprintf("scan error: %v", err))
			continue
		}

		// Extract entity key
		entityKey, ok := values[entityIdx].(string)
		if !ok {
			result.SkippedRows++
			continue
		}

		// Extract timestamp
		var timestamp int64
		if timestampIdx >= 0 && values[timestampIdx] != nil {
			switch t := values[timestampIdx].(type) {
			case time.Time:
				timestamp = t.UnixNano()
			case int64:
				timestamp = t
			}
		}
		if timestamp == 0 {
			timestamp = time.Now().UnixNano()
		}

		// Build feature map
		features := make(map[string]*domain.FeatureValue)
		for idx, featureName := range featureIndices {
			if values[idx] != nil {
				features[featureName] = &domain.FeatureValue{
					Value:     values[idx],
					Timestamp: timestamp,
				}
			}
		}

		// Store in Feather
		if len(features) > 0 {
			if err := c.store.Put(entityKey, features); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("put %s: %v", entityKey, err))
				continue
			}
			result.FeaturesUpdated += int64(len(features))
			entities[entityKey] = true
		}

		result.RowsImported++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
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
func (c *SnowflakeConnector) buildImportQuery(req *ImportRequest) (string, error) {
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

	schema := c.config.Schema
	if req.Schema != "" {
		schema = req.Schema
	}
	if err := validateIdentifier(schema); err != nil {
		return "", fmt.Errorf("buildImportQuery schema: %w", err)
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

	sb.WriteString(" FROM ")
	sb.WriteString(fmt.Sprintf("%s.%s", schema, req.Table))

	if req.Limit > 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", req.Limit))
	}

	return sb.String(), nil
}

// ListTables returns available tables in Snowflake.
func (c *SnowflakeConnector) ListTables(ctx context.Context, schema string) ([]TableInfo, error) {
	c.mu.RLock()
	db := c.db
	state := c.state
	c.mu.RUnlock()

	if state != ConnectionStateConnected || db == nil {
		return nil, ErrConnectorNotConnected
	}

	if schema == "" {
		schema = c.config.Schema
	}

	query := `
		SELECT
			TABLE_NAME,
			TABLE_SCHEMA,
			ROW_COUNT,
			BYTES,
			CREATED,
			LAST_ALTERED
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME
	`

	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrQueryFailed, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			c.logger.Warn("failed to close Snowflake rows", "error", closeErr)
		}
	}()

	var tables []TableInfo
	for rows.Next() {
		var t TableInfo
		var created, altered sql.NullTime
		var rowCount, bytes sql.NullInt64

		if err := rows.Scan(&t.Name, &t.Schema, &rowCount, &bytes, &created, &altered); err != nil {
			continue
		}

		if rowCount.Valid {
			t.RowCount = rowCount.Int64
		}
		if bytes.Valid {
			t.SizeBytes = bytes.Int64
		}
		if created.Valid {
			t.CreatedAt = created.Time
		}
		if altered.Valid {
			t.LastModified = altered.Time
		}

		tables = append(tables, t)
	}

	return tables, rows.Err()
}

// GetTableSchema returns the schema for a specific table.
func (c *SnowflakeConnector) GetTableSchema(ctx context.Context, table string) (*TableSchema, error) {
	c.mu.RLock()
	db := c.db
	state := c.state
	c.mu.RUnlock()

	if state != ConnectionStateConnected || db == nil {
		return nil, ErrConnectorNotConnected
	}

	// Parse table name for schema
	parts := strings.Split(table, ".")
	schemaName := c.config.Schema
	tableName := table
	if len(parts) == 2 {
		schemaName = parts[0]
		tableName = parts[1]
	}

	query := `
		SELECT
			COLUMN_NAME,
			DATA_TYPE,
			IS_NULLABLE
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`

	rows, err := db.QueryContext(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrQueryFailed, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			c.logger.Warn("failed to close Snowflake rows", "error", closeErr)
		}
	}()

	schema := &TableSchema{
		Table:  tableName,
		Schema: schemaName,
	}

	for rows.Next() {
		var col ColumnInfo
		var nullable string

		if err := rows.Scan(&col.Name, &col.Type, &nullable); err != nil {
			continue
		}

		col.Nullable = nullable == "YES"
		col.FeatureType = mapSnowflakeTypeToFeature(col.Type)
		schema.Columns = append(schema.Columns, col)
	}

	if len(schema.Columns) == 0 {
		return nil, fmt.Errorf("%w: %s.%s", ErrTableNotFound, schemaName, tableName)
	}

	return schema, rows.Err()
}

// CreateTable creates a new table for feature storage.
func (c *SnowflakeConnector) CreateTable(ctx context.Context, req *CreateTableRequest) error {
	// Validate request first
	if req.Table == "" {
		return fmt.Errorf("%w: table is required", ErrInvalidConfig)
	}

	c.mu.RLock()
	db := c.db
	state := c.state
	c.mu.RUnlock()

	if state != ConnectionStateConnected || db == nil {
		return ErrConnectorNotConnected
	}

	schema := c.config.Schema
	if req.Schema != "" {
		schema = req.Schema
	}

	// Build CREATE TABLE statement
	var sb strings.Builder
	sb.WriteString("CREATE TABLE ")
	if req.IfNotExists {
		sb.WriteString("IF NOT EXISTS ")
	}
	sb.WriteString(fmt.Sprintf("%s.%s", schema, req.Table))
	sb.WriteString(" (\n")
	sb.WriteString("  entity_key VARCHAR NOT NULL,\n")
	sb.WriteString("  timestamp TIMESTAMP_NTZ NOT NULL,\n")
	sb.WriteString("  version BIGINT DEFAULT 1,\n")

	for i, feature := range req.Features {
		snowflakeType := mapFeatureTypeToSnowflake(feature.DataType)
		nullable := ""
		if feature.Validation != nil && feature.Validation.NotNull {
			nullable = " NOT NULL"
		}
		sb.WriteString(fmt.Sprintf("  %s %s%s", feature.Name, snowflakeType, nullable))
		if i < len(req.Features)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(")")

	// Add clustering
	if len(req.ClusterBy) > 0 {
		sb.WriteString(fmt.Sprintf(" CLUSTER BY (%s)", strings.Join(req.ClusterBy, ", ")))
	} else if len(req.PartitionBy) > 0 {
		sb.WriteString(fmt.Sprintf(" CLUSTER BY (%s)", strings.Join(req.PartitionBy, ", ")))
	}

	_, err := db.ExecContext(ctx, sb.String())
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return ErrTableExists
		}
		return fmt.Errorf("creating table: %w", err)
	}

	c.logger.Info("created table",
		"schema", schema,
		"table", req.Table,
		"features", len(req.Features),
	)

	return nil
}

// ExecuteQuery executes a read-only query and returns results.
func (c *SnowflakeConnector) ExecuteQuery(ctx context.Context, query string) (*QueryResult, error) {
	c.mu.RLock()
	db := c.db
	state := c.state
	c.mu.RUnlock()

	if state != ConnectionStateConnected || db == nil {
		return nil, ErrConnectorNotConnected
	}

	start := time.Now()
	atomic.AddInt64(&c.metrics.QueriesExecuted, 1)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		atomic.AddInt64(&c.metrics.QueryErrors, 1)
		return nil, fmt.Errorf("%w: %w", ErrQueryFailed, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			c.logger.Warn("failed to close Snowflake rows", "error", closeErr)
		}
	}()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("getting columns: %w", err)
	}

	result := &QueryResult{
		Columns: columns,
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		result.Rows = append(result.Rows, values)
		result.RowCount++
	}

	result.Duration = time.Since(start)

	return result, rows.Err()
}

// Metrics returns connector performance metrics.
func (c *SnowflakeConnector) Metrics() ConnectorMetrics {
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

// serializeValueForSnowflake converts values for Snowflake storage.
func serializeValueForSnowflake(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case []float32:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return string(data), nil
	case []float64:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return string(data), nil
	case []byte:
		return v, nil
	case time.Time:
		return v, nil
	default:
		return v, nil
	}
}
