// Package warehouse provides connectors for cloud data warehouses.
//
// This package enables bidirectional synchronization between Feather's
// feature store and cloud data warehouses like Snowflake and BigQuery.
// Features can be exported to warehouses for offline analysis and training,
// while warehouse data can be imported to serve real-time inference.
//
// # Supported Warehouses
//
//   - Snowflake: Full read/write support with bulk load/unload
//   - BigQuery: Full read/write support with streaming inserts
//
// # Usage
//
//	connector, err := warehouse.NewSnowflakeConnector(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer connector.Close()
//
//	// Export features to warehouse
//	result, err := connector.Export(ctx, &warehouse.ExportRequest{
//	    Table:    "features_snapshot",
//	    Entities: []string{"user:123", "user:456"},
//	    Features: []string{"click_count", "purchase_total"},
//	})
package warehouse

import (
	"context"
	"errors"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

// Errors returned by warehouse connectors.
var (
	ErrConnectorNotConnected = errors.New("connector not connected")
	ErrConnectionFailed      = errors.New("connection failed")
	ErrAuthenticationFailed  = errors.New("authentication failed")
	ErrTableNotFound         = errors.New("table not found")
	ErrTableExists           = errors.New("table already exists")
	ErrSchemaNotFound        = errors.New("schema not found")
	ErrSchemaMismatch        = errors.New("schema mismatch")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrQueryFailed           = errors.New("query execution failed")
	ErrExportFailed          = errors.New("export failed")
	ErrImportFailed          = errors.New("import failed")
	ErrSyncFailed            = errors.New("sync failed")
	ErrQuotaExceeded         = errors.New("warehouse quota exceeded")
	ErrRateLimited           = errors.New("rate limited by warehouse")
	ErrTimeout               = errors.New("operation timed out")
	ErrInvalidConfig         = errors.New("invalid configuration")
	ErrIteratorDone          = errors.New("iterator done")
)

// ConnectorType identifies the warehouse type.
type ConnectorType string

const (
	// ConnectorTypeSnowflake identifies the Snowflake connector.
	ConnectorTypeSnowflake ConnectorType = "snowflake"
	// ConnectorTypeBigQuery identifies the BigQuery connector.
	ConnectorTypeBigQuery ConnectorType = "bigquery"
	// ConnectorTypeRedshift identifies the Amazon Redshift connector.
	ConnectorTypeRedshift ConnectorType = "redshift"
)

// ConnectionState represents the current state of a warehouse connection.
type ConnectionState string

const (
	// ConnectionStateDisconnected indicates no active connection.
	ConnectionStateDisconnected ConnectionState = "disconnected"
	// ConnectionStateConnecting indicates an in-progress connection attempt.
	ConnectionStateConnecting ConnectionState = "connecting"
	// ConnectionStateConnected indicates an active connection.
	ConnectionStateConnected ConnectionState = "connected"
	// ConnectionStateFailed indicates a failed connection attempt.
	ConnectionStateFailed ConnectionState = "failed"
)

// SyncDirection indicates the direction of data synchronization.
type SyncDirection string

const (
	// SyncDirectionExport sends data from Feather to the warehouse.
	SyncDirectionExport SyncDirection = "export"
	// SyncDirectionImport pulls data from the warehouse into Feather.
	SyncDirectionImport SyncDirection = "import"
	// SyncDirectionBidir allows bidirectional sync.
	SyncDirectionBidir SyncDirection = "bidirectional"
)

// SyncMode determines how synchronization handles existing data.
type SyncMode string

const (
	// SyncModeFull replaces all data.
	SyncModeFull SyncMode = "full"
	// SyncModeIncremental syncs only changes since last sync.
	SyncModeIncremental SyncMode = "incremental"
	// SyncModeMerge merges with conflict resolution.
	SyncModeMerge SyncMode = "merge"
)

// ConflictResolution determines how to resolve conflicts during merge sync.
type ConflictResolution string

const (
	// ConflictResolutionLatest keeps the newest timestamp.
	ConflictResolutionLatest ConflictResolution = "latest"
	// ConflictResolutionSource prefers the Feather source.
	ConflictResolutionSource ConflictResolution = "source"
	// ConflictResolutionTarget prefers the warehouse target.
	ConflictResolutionTarget ConflictResolution = "target"
	// ConflictResolutionHigherVer prefers the higher version.
	ConflictResolutionHigherVer ConflictResolution = "higher_ver"
)

// Connector defines the interface for warehouse connectors.
type Connector interface {
	// Connect establishes a connection to the warehouse.
	Connect(ctx context.Context) error

	// Close closes the warehouse connection.
	Close() error

	// State returns the current connection state.
	State() ConnectionState

	// Type returns the connector type.
	Type() ConnectorType

	// Ping verifies the connection is alive.
	Ping(ctx context.Context) error

	// Export exports features from Feather to the warehouse.
	Export(ctx context.Context, req *ExportRequest) (*ExportResult, error)

	// Import imports data from the warehouse to Feather.
	Import(ctx context.Context, req *ImportRequest) (*ImportResult, error)

	// ListTables returns available tables in the warehouse.
	ListTables(ctx context.Context, schema string) ([]TableInfo, error)

	// GetTableSchema returns the schema for a specific table.
	GetTableSchema(ctx context.Context, table string) (*TableSchema, error)

	// CreateTable creates a new table for feature storage.
	CreateTable(ctx context.Context, req *CreateTableRequest) error

	// ExecuteQuery executes a read-only query and returns results.
	ExecuteQuery(ctx context.Context, query string) (*QueryResult, error)
}

// BaseConfig contains common configuration for all warehouse connectors.
type BaseConfig struct {
	// ConnectionTimeout is the maximum time to establish a connection.
	ConnectionTimeout time.Duration `json:"connection_timeout" yaml:"connection_timeout"`

	// QueryTimeout is the maximum time for query execution.
	QueryTimeout time.Duration `json:"query_timeout" yaml:"query_timeout"`

	// MaxRetries is the number of retry attempts for transient failures.
	MaxRetries int `json:"max_retries" yaml:"max_retries"`

	// RetryBackoff is the initial backoff duration between retries.
	RetryBackoff time.Duration `json:"retry_backoff" yaml:"retry_backoff"`

	// BatchSize is the number of rows per batch operation.
	BatchSize int `json:"batch_size" yaml:"batch_size"`

	// EnableCompression enables data compression during transfer.
	EnableCompression bool `json:"enable_compression" yaml:"enable_compression"`
}

// DefaultBaseConfig returns the default base configuration.
func DefaultBaseConfig() BaseConfig {
	return BaseConfig{
		ConnectionTimeout: 30 * time.Second,
		QueryTimeout:      5 * time.Minute,
		MaxRetries:        3,
		RetryBackoff:      time.Second,
		BatchSize:         10000,
		EnableCompression: true,
	}
}

// ExportRequest defines the parameters for exporting features to a warehouse.
type ExportRequest struct {
	// Table is the destination table name.
	Table string `json:"table"`

	// Schema is the destination schema/dataset name.
	Schema string `json:"schema,omitempty"`

	// Entities to export (empty means all).
	Entities []string `json:"entities,omitempty"`

	// Features to export.
	Features []string `json:"features"`

	// StartTime filters features updated after this time.
	StartTime *time.Time `json:"start_time,omitempty"`

	// EndTime filters features updated before this time.
	EndTime *time.Time `json:"end_time,omitempty"`

	// Mode determines how to handle existing data.
	Mode SyncMode `json:"mode"`

	// CreateTable creates the table if it doesn't exist.
	CreateTable bool `json:"create_table"`

	// PartitionBy specifies partition columns.
	PartitionBy []string `json:"partition_by,omitempty"`
}

// ExportResult contains the result of an export operation.
type ExportResult struct {
	// RowsExported is the number of rows written.
	RowsExported int64 `json:"rows_exported"`

	// BytesExported is the total bytes transferred.
	BytesExported int64 `json:"bytes_exported"`

	// EntitiesExported is the number of unique entities.
	EntitiesExported int64 `json:"entities_exported"`

	// FeaturesExported is the number of features exported.
	FeaturesExported int `json:"features_exported"`

	// Duration is the total operation time.
	Duration time.Duration `json:"duration"`

	// Table is the destination table.
	Table string `json:"table"`

	// Checksum is the data checksum for validation.
	Checksum string `json:"checksum,omitempty"`

	// Errors contains any non-fatal errors encountered.
	Errors []string `json:"errors,omitempty"`
}

// ImportRequest defines the parameters for importing data from a warehouse.
type ImportRequest struct {
	// Table is the source table name.
	Table string `json:"table"`

	// Schema is the source schema/dataset name.
	Schema string `json:"schema,omitempty"`

	// Query is a custom SQL query (alternative to Table).
	Query string `json:"query,omitempty"`

	// EntityColumn maps to the entity key.
	EntityColumn string `json:"entity_column"`

	// FeatureColumns maps column names to feature names.
	FeatureColumns map[string]string `json:"feature_columns"`

	// TimestampColumn contains the timestamp for each row.
	TimestampColumn string `json:"timestamp_column,omitempty"`

	// Mode determines how to handle existing data in Feather.
	Mode SyncMode `json:"mode"`

	// Limit restricts the number of rows to import.
	Limit int64 `json:"limit,omitempty"`

	// Filter is an optional WHERE clause condition.
	Filter string `json:"filter,omitempty"`
}

// ImportResult contains the result of an import operation.
type ImportResult struct {
	// RowsImported is the number of rows read.
	RowsImported int64 `json:"rows_imported"`

	// FeaturesUpdated is the number of feature updates made.
	FeaturesUpdated int64 `json:"features_updated"`

	// EntitiesUpdated is the number of unique entities updated.
	EntitiesUpdated int64 `json:"entities_updated"`

	// Duration is the total operation time.
	Duration time.Duration `json:"duration"`

	// SkippedRows is the number of rows skipped due to errors.
	SkippedRows int64 `json:"skipped_rows,omitempty"`

	// Errors contains any non-fatal errors encountered.
	Errors []string `json:"errors,omitempty"`
}

// CreateTableRequest defines parameters for creating a feature table.
type CreateTableRequest struct {
	// Table is the table name to create.
	Table string `json:"table"`

	// Schema is the schema/dataset name.
	Schema string `json:"schema,omitempty"`

	// Features defines the feature columns.
	Features []domain.FeatureSpec `json:"features"`

	// PartitionBy specifies partition columns.
	PartitionBy []string `json:"partition_by,omitempty"`

	// ClusterBy specifies clustering columns (BigQuery).
	ClusterBy []string `json:"cluster_by,omitempty"`

	// TTLDays is the data retention period in days.
	TTLDays int `json:"ttl_days,omitempty"`

	// IfNotExists skips creation if table exists.
	IfNotExists bool `json:"if_not_exists"`
}

// TableInfo contains metadata about a warehouse table.
type TableInfo struct {
	// Name is the table name.
	Name string `json:"name"`

	// Schema is the schema/dataset name.
	Schema string `json:"schema"`

	// RowCount is the approximate row count.
	RowCount int64 `json:"row_count"`

	// SizeBytes is the approximate size in bytes.
	SizeBytes int64 `json:"size_bytes"`

	// CreatedAt is when the table was created.
	CreatedAt time.Time `json:"created_at"`

	// LastModified is when the table was last modified.
	LastModified time.Time `json:"last_modified"`

	// PartitionedBy lists partition columns.
	PartitionedBy []string `json:"partitioned_by,omitempty"`
}

// TableSchema describes the schema of a warehouse table.
type TableSchema struct {
	// Table is the table name.
	Table string `json:"table"`

	// Schema is the schema/dataset name.
	Schema string `json:"schema"`

	// Columns lists all columns in order.
	Columns []ColumnInfo `json:"columns"`

	// PrimaryKey lists primary key columns.
	PrimaryKey []string `json:"primary_key,omitempty"`

	// PartitionedBy lists partition columns.
	PartitionedBy []string `json:"partitioned_by,omitempty"`
}

// ColumnInfo describes a single column in a table.
type ColumnInfo struct {
	// Name is the column name.
	Name string `json:"name"`

	// Type is the warehouse-specific data type.
	Type string `json:"type"`

	// FeatureType is the mapped Feather data type.
	FeatureType domain.DataType `json:"feature_type"`

	// Nullable indicates if the column allows NULL.
	Nullable bool `json:"nullable"`

	// Description is the column description.
	Description string `json:"description,omitempty"`
}

// QueryResult contains the result of a query execution.
type QueryResult struct {
	// Columns lists the column names.
	Columns []string `json:"columns"`

	// Rows contains the result rows.
	Rows [][]interface{} `json:"rows"`

	// RowCount is the total number of rows.
	RowCount int64 `json:"row_count"`

	// BytesScanned is the bytes scanned by the query.
	BytesScanned int64 `json:"bytes_scanned"`

	// Duration is the query execution time.
	Duration time.Duration `json:"duration"`
}

// ConnectorMetrics tracks connector performance.
type ConnectorMetrics struct {
	// ConnectionAttempts is the total connection attempts.
	ConnectionAttempts int64 `json:"connection_attempts"`

	// ConnectionFailures is the number of failed connections.
	ConnectionFailures int64 `json:"connection_failures"`

	// QueriesExecuted is the total queries executed.
	QueriesExecuted int64 `json:"queries_executed"`

	// QueryErrors is the number of failed queries.
	QueryErrors int64 `json:"query_errors"`

	// RowsExported is the total rows exported.
	RowsExported int64 `json:"rows_exported"`

	// RowsImported is the total rows imported.
	RowsImported int64 `json:"rows_imported"`

	// BytesTransferred is the total bytes transferred.
	BytesTransferred int64 `json:"bytes_transferred"`

	// LastConnectedAt is the last successful connection time.
	LastConnectedAt time.Time `json:"last_connected_at,omitempty"`

	// LastExportAt is the last successful export time.
	LastExportAt time.Time `json:"last_export_at,omitempty"`

	// LastImportAt is the last successful import time.
	LastImportAt time.Time `json:"last_import_at,omitempty"`
}

// mapFeatureTypeToWarehouse converts Feather types to warehouse types.
func mapFeatureTypeToWarehouse(dt domain.DataType, warehouseType ConnectorType) string {
	switch warehouseType {
	case ConnectorTypeSnowflake:
		return mapFeatureTypeToSnowflake(dt)
	case ConnectorTypeBigQuery:
		return mapFeatureTypeToBigQuery(dt)
	case ConnectorTypeRedshift:
		return mapFeatureTypeToRedshift(dt)
	default:
		return "STRING"
	}
}

func mapFeatureTypeToSnowflake(dt domain.DataType) string {
	switch dt {
	case domain.DataTypeInt64:
		return "BIGINT"
	case domain.DataTypeFloat64:
		return "DOUBLE"
	case domain.DataTypeString:
		return "VARCHAR"
	case domain.DataTypeBool:
		return "BOOLEAN"
	case domain.DataTypeBytes:
		return "BINARY"
	case domain.DataTypeVector:
		return "ARRAY"
	case domain.DataTypeTimestamp:
		return "TIMESTAMP_NTZ"
	default:
		return "VARCHAR"
	}
}

func mapFeatureTypeToBigQuery(dt domain.DataType) string {
	switch dt {
	case domain.DataTypeInt64:
		return "INT64"
	case domain.DataTypeFloat64:
		return "FLOAT64"
	case domain.DataTypeString:
		return "STRING"
	case domain.DataTypeBool:
		return "BOOL"
	case domain.DataTypeBytes:
		return "BYTES"
	case domain.DataTypeVector:
		return "ARRAY<FLOAT64>"
	case domain.DataTypeTimestamp:
		return "TIMESTAMP"
	default:
		return "STRING"
	}
}

// mapWarehouseTypeToFeature converts warehouse types to Feather types.
func mapWarehouseTypeToFeature(warehouseType string, connectorType ConnectorType) domain.DataType {
	switch connectorType {
	case ConnectorTypeSnowflake:
		return mapSnowflakeTypeToFeature(warehouseType)
	case ConnectorTypeBigQuery:
		return mapBigQueryTypeToFeature(warehouseType)
	case ConnectorTypeRedshift:
		return mapRedshiftTypeToFeature(warehouseType)
	default:
		return domain.DataTypeString
	}
}

func mapSnowflakeTypeToFeature(t string) domain.DataType {
	switch t {
	case "BIGINT", "INTEGER", "INT", "SMALLINT", "TINYINT":
		return domain.DataTypeInt64
	case "DOUBLE", "FLOAT", "FLOAT4", "FLOAT8", "REAL", "DECIMAL", "NUMERIC", "NUMBER":
		return domain.DataTypeFloat64
	case "VARCHAR", "CHAR", "STRING", "TEXT":
		return domain.DataTypeString
	case "BOOLEAN":
		return domain.DataTypeBool
	case "BINARY", "VARBINARY":
		return domain.DataTypeBytes
	case "ARRAY":
		return domain.DataTypeVector
	case "TIMESTAMP", "TIMESTAMP_NTZ", "TIMESTAMP_LTZ", "TIMESTAMP_TZ":
		return domain.DataTypeTimestamp
	default:
		return domain.DataTypeString
	}
}

func mapBigQueryTypeToFeature(t string) domain.DataType {
	switch t {
	case "INT64", "INTEGER":
		return domain.DataTypeInt64
	case "FLOAT64", "FLOAT", "NUMERIC", "BIGNUMERIC":
		return domain.DataTypeFloat64
	case "STRING":
		return domain.DataTypeString
	case "BOOL", "BOOLEAN":
		return domain.DataTypeBool
	case "BYTES":
		return domain.DataTypeBytes
	case "ARRAY<FLOAT64>", "ARRAY<FLOAT>":
		return domain.DataTypeVector
	case "TIMESTAMP", "DATETIME", "DATE", "TIME":
		return domain.DataTypeTimestamp
	default:
		return domain.DataTypeString
	}
}

func mapFeatureTypeToRedshift(dt domain.DataType) string {
	switch dt {
	case domain.DataTypeInt64:
		return "BIGINT"
	case domain.DataTypeFloat64:
		return "DOUBLE PRECISION"
	case domain.DataTypeString:
		return "VARCHAR(65535)"
	case domain.DataTypeBool:
		return "BOOLEAN"
	case domain.DataTypeBytes:
		return "VARBYTE"
	case domain.DataTypeVector:
		return "SUPER"
	case domain.DataTypeTimestamp:
		return "TIMESTAMP"
	default:
		return "VARCHAR(65535)"
	}
}

func mapRedshiftTypeToFeature(t string) domain.DataType {
	switch t {
	case "BIGINT", "INTEGER", "INT", "INT4", "INT8", "SMALLINT", "INT2":
		return domain.DataTypeInt64
	case "DOUBLE PRECISION", "FLOAT", "FLOAT4", "FLOAT8", "REAL", "DECIMAL", "NUMERIC":
		return domain.DataTypeFloat64
	case "VARCHAR", "CHAR", "CHARACTER VARYING", "NCHAR", "NVARCHAR", "BPCHAR", "TEXT":
		return domain.DataTypeString
	case "BOOLEAN", "BOOL":
		return domain.DataTypeBool
	case "VARBYTE", "BINARY VARYING":
		return domain.DataTypeBytes
	case "SUPER":
		return domain.DataTypeVector
	case "TIMESTAMP", "TIMESTAMPTZ", "TIMESTAMP WITHOUT TIME ZONE", "TIMESTAMP WITH TIME ZONE", "DATE":
		return domain.DataTypeTimestamp
	default:
		return domain.DataTypeString
	}
}
