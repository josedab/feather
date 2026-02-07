package warehouse

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/feather-store/feather/internal/domain"
)

func TestConnectorType_String(t *testing.T) {
	assert.Equal(t, ConnectorType("snowflake"), ConnectorTypeSnowflake)
	assert.Equal(t, ConnectorType("bigquery"), ConnectorTypeBigQuery)
}

func TestConnectionState_Values(t *testing.T) {
	assert.Equal(t, ConnectionState("disconnected"), ConnectionStateDisconnected)
	assert.Equal(t, ConnectionState("connecting"), ConnectionStateConnecting)
	assert.Equal(t, ConnectionState("connected"), ConnectionStateConnected)
	assert.Equal(t, ConnectionState("failed"), ConnectionStateFailed)
}

func TestSyncDirection_Values(t *testing.T) {
	assert.Equal(t, SyncDirection("export"), SyncDirectionExport)
	assert.Equal(t, SyncDirection("import"), SyncDirectionImport)
	assert.Equal(t, SyncDirection("bidirectional"), SyncDirectionBidir)
}

func TestSyncMode_Values(t *testing.T) {
	assert.Equal(t, SyncMode("full"), SyncModeFull)
	assert.Equal(t, SyncMode("incremental"), SyncModeIncremental)
	assert.Equal(t, SyncMode("merge"), SyncModeMerge)
}

func TestConflictResolution_Values(t *testing.T) {
	assert.Equal(t, ConflictResolution("latest"), ConflictResolutionLatest)
	assert.Equal(t, ConflictResolution("source"), ConflictResolutionSource)
	assert.Equal(t, ConflictResolution("target"), ConflictResolutionTarget)
	assert.Equal(t, ConflictResolution("higher_ver"), ConflictResolutionHigherVer)
}

func TestDefaultBaseConfig(t *testing.T) {
	config := DefaultBaseConfig()

	assert.NotZero(t, config.ConnectionTimeout)
	assert.NotZero(t, config.QueryTimeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.NotZero(t, config.RetryBackoff)
	assert.Equal(t, 10000, config.BatchSize)
	assert.True(t, config.EnableCompression)
}

func TestMapFeatureTypeToSnowflake(t *testing.T) {
	tests := []struct {
		input    domain.DataType
		expected string
	}{
		{domain.DataTypeInt64, "BIGINT"},
		{domain.DataTypeFloat64, "DOUBLE"},
		{domain.DataTypeString, "VARCHAR"},
		{domain.DataTypeBool, "BOOLEAN"},
		{domain.DataTypeBytes, "BINARY"},
		{domain.DataTypeVector, "ARRAY"},
		{domain.DataTypeTimestamp, "TIMESTAMP_NTZ"},
	}

	for _, tt := range tests {
		t.Run(tt.input.String(), func(t *testing.T) {
			result := mapFeatureTypeToSnowflake(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapFeatureTypeToBigQuery(t *testing.T) {
	tests := []struct {
		input    domain.DataType
		expected string
	}{
		{domain.DataTypeInt64, "INT64"},
		{domain.DataTypeFloat64, "FLOAT64"},
		{domain.DataTypeString, "STRING"},
		{domain.DataTypeBool, "BOOL"},
		{domain.DataTypeBytes, "BYTES"},
		{domain.DataTypeVector, "ARRAY<FLOAT64>"},
		{domain.DataTypeTimestamp, "TIMESTAMP"},
	}

	for _, tt := range tests {
		t.Run(tt.input.String(), func(t *testing.T) {
			result := mapFeatureTypeToBigQuery(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapSnowflakeTypeToFeature(t *testing.T) {
	tests := []struct {
		input    string
		expected domain.DataType
	}{
		{"BIGINT", domain.DataTypeInt64},
		{"INTEGER", domain.DataTypeInt64},
		{"INT", domain.DataTypeInt64},
		{"DOUBLE", domain.DataTypeFloat64},
		{"FLOAT", domain.DataTypeFloat64},
		{"DECIMAL", domain.DataTypeFloat64},
		{"VARCHAR", domain.DataTypeString},
		{"STRING", domain.DataTypeString},
		{"TEXT", domain.DataTypeString},
		{"BOOLEAN", domain.DataTypeBool},
		{"BINARY", domain.DataTypeBytes},
		{"ARRAY", domain.DataTypeVector},
		{"TIMESTAMP", domain.DataTypeTimestamp},
		{"TIMESTAMP_NTZ", domain.DataTypeTimestamp},
		{"UNKNOWN", domain.DataTypeString},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapSnowflakeTypeToFeature(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapBigQueryTypeToFeature(t *testing.T) {
	tests := []struct {
		input    string
		expected domain.DataType
	}{
		{"INT64", domain.DataTypeInt64},
		{"INTEGER", domain.DataTypeInt64},
		{"FLOAT64", domain.DataTypeFloat64},
		{"FLOAT", domain.DataTypeFloat64},
		{"NUMERIC", domain.DataTypeFloat64},
		{"STRING", domain.DataTypeString},
		{"BOOL", domain.DataTypeBool},
		{"BOOLEAN", domain.DataTypeBool},
		{"BYTES", domain.DataTypeBytes},
		{"ARRAY<FLOAT64>", domain.DataTypeVector},
		{"TIMESTAMP", domain.DataTypeTimestamp},
		{"DATETIME", domain.DataTypeTimestamp},
		{"DATE", domain.DataTypeTimestamp},
		{"UNKNOWN", domain.DataTypeString},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapBigQueryTypeToFeature(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapFeatureTypeToWarehouse(t *testing.T) {
	// Test Snowflake mapping
	result := mapFeatureTypeToWarehouse(domain.DataTypeInt64, ConnectorTypeSnowflake)
	assert.Equal(t, "BIGINT", result)

	// Test BigQuery mapping
	result = mapFeatureTypeToWarehouse(domain.DataTypeInt64, ConnectorTypeBigQuery)
	assert.Equal(t, "INT64", result)

	// Test unknown warehouse type
	result = mapFeatureTypeToWarehouse(domain.DataTypeInt64, "unknown")
	assert.Equal(t, "STRING", result)
}

func TestMapWarehouseTypeToFeature(t *testing.T) {
	// Test Snowflake mapping
	result := mapWarehouseTypeToFeature("BIGINT", ConnectorTypeSnowflake)
	assert.Equal(t, domain.DataTypeInt64, result)

	// Test BigQuery mapping
	result = mapWarehouseTypeToFeature("INT64", ConnectorTypeBigQuery)
	assert.Equal(t, domain.DataTypeInt64, result)

	// Test unknown warehouse type
	result = mapWarehouseTypeToFeature("BIGINT", "unknown")
	assert.Equal(t, domain.DataTypeString, result)
}

func TestExportRequest_Validation(t *testing.T) {
	req := &ExportRequest{
		Table:    "features",
		Features: []string{"click_count", "purchase_total"},
		Mode:     SyncModeFull,
	}

	assert.NotEmpty(t, req.Table)
	assert.Len(t, req.Features, 2)
	assert.Equal(t, SyncModeFull, req.Mode)
}

func TestImportRequest_Validation(t *testing.T) {
	req := &ImportRequest{
		Table:        "features",
		EntityColumn: "entity_key",
		FeatureColumns: map[string]string{
			"clicks":    "click_count",
			"purchases": "purchase_total",
		},
		Mode: SyncModeIncremental,
	}

	assert.NotEmpty(t, req.Table)
	assert.NotEmpty(t, req.EntityColumn)
	assert.Len(t, req.FeatureColumns, 2)
	assert.Equal(t, SyncModeIncremental, req.Mode)
}

func TestTableInfo_Fields(t *testing.T) {
	info := TableInfo{
		Name:      "features",
		Schema:    "public",
		RowCount:  1000,
		SizeBytes: 1024 * 1024,
	}

	assert.Equal(t, "features", info.Name)
	assert.Equal(t, "public", info.Schema)
	assert.Equal(t, int64(1000), info.RowCount)
	assert.Equal(t, int64(1024*1024), info.SizeBytes)
}

func TestTableSchema_Fields(t *testing.T) {
	schema := TableSchema{
		Table:  "features",
		Schema: "public",
		Columns: []ColumnInfo{
			{Name: "entity_key", Type: "VARCHAR", Nullable: false},
			{Name: "click_count", Type: "BIGINT", Nullable: true},
		},
		PrimaryKey:    []string{"entity_key"},
		PartitionedBy: []string{"timestamp"},
	}

	_ = schema.Schema
	_ = schema.PrimaryKey
	_ = schema.PartitionedBy
	assert.Equal(t, "features", schema.Table)
	assert.Len(t, schema.Columns, 2)
	assert.Equal(t, "entity_key", schema.Columns[0].Name)
	assert.False(t, schema.Columns[0].Nullable)
	assert.True(t, schema.Columns[1].Nullable)
}

func TestColumnInfo_FeatureType(t *testing.T) {
	col := ColumnInfo{
		Name:        "click_count",
		Type:        "BIGINT",
		FeatureType: domain.DataTypeInt64,
		Nullable:    true,
	}

	_ = col.Name
	_ = col.Type
	_ = col.Nullable
	assert.Equal(t, domain.DataTypeInt64, col.FeatureType)
}

func TestQueryResult_Fields(t *testing.T) {
	result := QueryResult{
		Columns:      []string{"entity_key", "click_count"},
		Rows:         [][]interface{}{{"user:1", int64(42)}},
		RowCount:     1,
		BytesScanned: 1024,
	}

	assert.Len(t, result.Columns, 2)
	assert.Len(t, result.Rows, 1)
	assert.Equal(t, int64(1), result.RowCount)
	assert.Equal(t, int64(1024), result.BytesScanned)
}

func TestConnectorMetrics_Fields(t *testing.T) {
	metrics := ConnectorMetrics{
		ConnectionAttempts: 5,
		ConnectionFailures: 1,
		QueriesExecuted:    100,
		QueryErrors:        2,
		RowsExported:       10000,
		RowsImported:       5000,
		BytesTransferred:   1024 * 1024,
	}

	_ = metrics.BytesTransferred
	assert.Equal(t, int64(5), metrics.ConnectionAttempts)
	assert.Equal(t, int64(1), metrics.ConnectionFailures)
	assert.Equal(t, int64(100), metrics.QueriesExecuted)
	assert.Equal(t, int64(2), metrics.QueryErrors)
	assert.Equal(t, int64(10000), metrics.RowsExported)
	assert.Equal(t, int64(5000), metrics.RowsImported)
}

func TestCreateTableRequest_Fields(t *testing.T) {
	req := &CreateTableRequest{
		Table: "features",
		Features: []domain.FeatureSpec{
			{Name: "click_count", DataType: domain.DataTypeInt64},
		},
		PartitionBy: []string{"timestamp"},
		IfNotExists: true,
	}

	_ = req.PartitionBy
	assert.Equal(t, "features", req.Table)
	assert.Len(t, req.Features, 1)
	assert.True(t, req.IfNotExists)
}

func TestExportResult_Fields(t *testing.T) {
	result := &ExportResult{
		RowsExported:     1000,
		BytesExported:    1024 * 100,
		EntitiesExported: 500,
		FeaturesExported: 5,
		Table:            "features",
		Checksum:         "abc123",
	}

	_ = result.Checksum
	assert.Equal(t, int64(1000), result.RowsExported)
	assert.Equal(t, int64(1024*100), result.BytesExported)
	assert.Equal(t, int64(500), result.EntitiesExported)
	assert.Equal(t, 5, result.FeaturesExported)
	assert.Equal(t, "features", result.Table)
}

func TestImportResult_Fields(t *testing.T) {
	result := &ImportResult{
		RowsImported:    1000,
		FeaturesUpdated: 5000,
		EntitiesUpdated: 500,
		SkippedRows:     10,
		Errors:          []string{"row 5: invalid type"},
	}

	assert.Equal(t, int64(1000), result.RowsImported)
	assert.Equal(t, int64(5000), result.FeaturesUpdated)
	assert.Equal(t, int64(500), result.EntitiesUpdated)
	assert.Equal(t, int64(10), result.SkippedRows)
	assert.Len(t, result.Errors, 1)
}

func TestErrors(t *testing.T) {
	errors := []error{
		ErrConnectorNotConnected,
		ErrConnectionFailed,
		ErrAuthenticationFailed,
		ErrTableNotFound,
		ErrTableExists,
		ErrSchemaNotFound,
		ErrSchemaMismatch,
		ErrInvalidCredentials,
		ErrQueryFailed,
		ErrExportFailed,
		ErrImportFailed,
		ErrSyncFailed,
		ErrQuotaExceeded,
		ErrRateLimited,
		ErrTimeout,
		ErrInvalidConfig,
	}

	for _, err := range errors {
		assert.NotNil(t, err)
		assert.NotEmpty(t, err.Error())
	}
}
