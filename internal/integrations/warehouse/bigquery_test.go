package warehouse

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

func TestDefaultBigQueryConfig(t *testing.T) {
	config := DefaultBigQueryConfig()

	assert.Equal(t, "US", config.Location)
	assert.True(t, config.UseDefaultCredentials)
	assert.True(t, config.EnableStreaming)
	assert.Equal(t, 10000, config.MaxStreamingRows)
	assert.Equal(t, 30, config.JobTimeoutMinutes)
	assert.NotZero(t, config.ConnectionTimeout)
}

func TestBigQueryConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  BigQueryConfig
		wantErr bool
	}{
		{
			name: "valid config with default credentials",
			config: BigQueryConfig{
				ProjectID:             "test-project",
				Dataset:               "test-dataset",
				UseDefaultCredentials: true,
			},
			wantErr: false,
		},
		{
			name: "valid config with credentials file",
			config: BigQueryConfig{
				ProjectID:       "test-project",
				Dataset:         "test-dataset",
				CredentialsFile: "/path/to/credentials.json",
			},
			wantErr: false,
		},
		{
			name: "valid config with credentials json",
			config: BigQueryConfig{
				ProjectID:       "test-project",
				Dataset:         "test-dataset",
				CredentialsJSON: `{"type": "service_account"}`,
			},
			wantErr: false,
		},
		{
			name: "missing project_id",
			config: BigQueryConfig{
				Dataset:               "test-dataset",
				UseDefaultCredentials: true,
			},
			wantErr: true,
		},
		{
			name: "missing dataset",
			config: BigQueryConfig{
				ProjectID:             "test-project",
				UseDefaultCredentials: true,
			},
			wantErr: true,
		},
		{
			name: "missing credentials",
			config: BigQueryConfig{
				ProjectID: "test-project",
				Dataset:   "test-dataset",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewBigQueryConnector(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, nil, nil, logger)
	require.NoError(t, err)
	require.NotNil(t, connector)

	assert.Equal(t, ConnectorTypeBigQuery, connector.Type())
	assert.Equal(t, ConnectionStateDisconnected, connector.State())
}

func TestNewBigQueryConnector_InvalidConfig(t *testing.T) {
	config := BigQueryConfig{} // Empty config

	connector, err := NewBigQueryConnector(config, nil, nil, nil)
	assert.Error(t, err)
	assert.Nil(t, connector)
}

func TestBigQueryConnector_State(t *testing.T) {
	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, nil, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, ConnectionStateDisconnected, connector.State())
}

func TestBigQueryConnector_Type(t *testing.T) {
	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, nil, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, ConnectorTypeBigQuery, connector.Type())
}

func TestBigQueryConnector_Connect(t *testing.T) {
	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, nil, nil, nil)
	require.NoError(t, err)

	// Connect (simulated mode without real client)
	err = connector.Connect(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, ConnectionStateConnected, connector.State())

	// Connect again should be idempotent
	err = connector.Connect(context.Background())
	assert.NoError(t, err)
}

func TestBigQueryConnector_Close(t *testing.T) {
	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, nil, nil, nil)
	require.NoError(t, err)

	// Connect first
	err = connector.Connect(context.Background())
	require.NoError(t, err)

	// Close
	err = connector.Close()
	assert.NoError(t, err)
	assert.Equal(t, ConnectionStateDisconnected, connector.State())
}

func TestBigQueryConnector_Ping(t *testing.T) {
	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, nil, nil, nil)
	require.NoError(t, err)

	// Ping when not connected
	err = connector.Ping(context.Background())
	assert.ErrorIs(t, err, ErrConnectorNotConnected)

	// Connect and ping (simulated)
	err = connector.Connect(context.Background())
	require.NoError(t, err)

	err = connector.Ping(context.Background())
	assert.NoError(t, err)
}

func TestBigQueryConnector_Export_NotConnected(t *testing.T) {
	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, nil, nil, nil)
	require.NoError(t, err)

	req := &ExportRequest{
		Table:    "features",
		Features: []string{"click_count"},
	}

	result, err := connector.Export(context.Background(), req)
	assert.ErrorIs(t, err, ErrConnectorNotConnected)
	assert.Nil(t, result)
}

func TestBigQueryConnector_Import_NotConnected(t *testing.T) {
	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, nil, nil, nil)
	require.NoError(t, err)

	req := &ImportRequest{
		Table:        "features",
		EntityColumn: "entity_key",
		FeatureColumns: map[string]string{
			"clicks": "click_count",
		},
	}

	result, err := connector.Import(context.Background(), req)
	assert.ErrorIs(t, err, ErrConnectorNotConnected)
	assert.Nil(t, result)
}

func TestBigQueryConnector_ListTables_NotConnected(t *testing.T) {
	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, nil, nil, nil)
	require.NoError(t, err)

	tables, err := connector.ListTables(context.Background(), "")
	assert.ErrorIs(t, err, ErrConnectorNotConnected)
	assert.Nil(t, tables)
}

func TestBigQueryConnector_GetTableSchema_NotConnected(t *testing.T) {
	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, nil, nil, nil)
	require.NoError(t, err)

	schema, err := connector.GetTableSchema(context.Background(), "features")
	assert.ErrorIs(t, err, ErrConnectorNotConnected)
	assert.Nil(t, schema)
}

func TestBigQueryConnector_CreateTable_NotConnected(t *testing.T) {
	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, nil, nil, nil)
	require.NoError(t, err)

	req := &CreateTableRequest{
		Table: "features",
		Features: []domain.FeatureSpec{
			{Name: "click_count", DataType: domain.DataTypeInt64},
		},
	}

	err = connector.CreateTable(context.Background(), req)
	assert.ErrorIs(t, err, ErrConnectorNotConnected)
}

func TestBigQueryConnector_ExecuteQuery_NotConnected(t *testing.T) {
	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, nil, nil, nil)
	require.NoError(t, err)

	result, err := connector.ExecuteQuery(context.Background(), "SELECT 1")
	assert.ErrorIs(t, err, ErrConnectorNotConnected)
	assert.Nil(t, result)
}

func TestBigQueryConnector_Metrics(t *testing.T) {
	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, nil, nil, nil)
	require.NoError(t, err)

	metrics := connector.Metrics()
	assert.Equal(t, int64(0), metrics.ConnectionAttempts)
	assert.Equal(t, int64(0), metrics.RowsExported)
}

func TestBigQueryConnector_buildImportQuery(t *testing.T) {
	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, nil, nil, nil)
	require.NoError(t, err)

	req := &ImportRequest{
		Table:           "features",
		EntityColumn:    "entity_key",
		TimestampColumn: "updated_at",
		FeatureColumns: map[string]string{
			"clicks":    "click_count",
			"purchases": "purchase_total",
		},
		Limit: 1000,
	}

	query, err := connector.buildImportQuery(req)
	require.NoError(t, err)
	assert.Contains(t, query, "SELECT entity_key")
	assert.Contains(t, query, "updated_at")
	assert.Contains(t, query, "FROM `test-project.test-dataset.features`")
	assert.Contains(t, query, "LIMIT 1000")
}

func TestBigQueryConnector_buildImportQuery_RejectsFilter(t *testing.T) {
	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, nil, nil, nil)
	require.NoError(t, err)

	req := &ImportRequest{
		Table:        "features",
		EntityColumn: "entity_key",
		FeatureColumns: map[string]string{
			"clicks": "click_count",
		},
		Filter: "updated_at > '2024-01-01'",
	}

	_, err = connector.buildImportQuery(req)
	assert.ErrorIs(t, err, ErrUnsafeFilter)
}

func TestBigQueryConnector_buildImportQuery_WithDataset(t *testing.T) {
	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, nil, nil, nil)
	require.NoError(t, err)

	req := &ImportRequest{
		Table:        "features",
		Schema:       "custom_dataset",
		EntityColumn: "entity_key",
		FeatureColumns: map[string]string{
			"clicks": "click_count",
		},
	}

	query, err := connector.buildImportQuery(req)
	require.NoError(t, err)
	assert.Contains(t, query, "FROM `test-project.custom_dataset.features`")
}

func TestSerializeValueForBigQuery(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		wantErr  bool
		checkVal func(t *testing.T, result interface{})
	}{
		{
			name:    "float32 slice converts to float64",
			input:   []float32{1.0, 2.0, 3.0},
			wantErr: false,
			checkVal: func(t *testing.T, result interface{}) {
				f64, ok := result.([]float64)
				assert.True(t, ok)
				assert.Len(t, f64, 3)
			},
		},
		{
			name:    "float64 slice unchanged",
			input:   []float64{1.0, 2.0, 3.0},
			wantErr: false,
			checkVal: func(t *testing.T, result interface{}) {
				f64, ok := result.([]float64)
				assert.True(t, ok)
				assert.Len(t, f64, 3)
			},
		},
		{
			name:    "bytes unchanged",
			input:   []byte("hello"),
			wantErr: false,
		},
		{
			name:    "time unchanged",
			input:   time.Now(),
			wantErr: false,
		},
		{
			name:    "string unchanged",
			input:   "test",
			wantErr: false,
		},
		{
			name:    "int64 unchanged",
			input:   int64(42),
			wantErr: false,
		},
		{
			name:    "map converts to json string",
			input:   map[string]interface{}{"key": "value"},
			wantErr: false,
			checkVal: func(t *testing.T, result interface{}) {
				str, ok := result.(string)
				assert.True(t, ok)
				assert.Contains(t, str, "key")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := serializeValueForBigQuery(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.checkVal != nil {
					tt.checkVal(t, result)
				}
			}
		})
	}
}

func setupBigQueryTestConnector(t *testing.T) (*BigQueryConnector, *storage.Store) {
	t.Helper()

	// Create mock schema registry
	schemaReg := &mockSchemaRegistry{
		features: map[string]*domain.FeatureSpec{
			"click_count":    {Name: "click_count", DataType: domain.DataTypeInt64},
			"purchase_total": {Name: "purchase_total", DataType: domain.DataTypeFloat64},
		},
	}

	// Create store with in-memory warm tier
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, schemaReg)
	require.NoError(t, err)

	config := BigQueryConfig{
		ProjectID:             "test-project",
		Dataset:               "test-dataset",
		UseDefaultCredentials: true,
	}

	connector, err := NewBigQueryConnector(config, store, schemaReg, nil)
	require.NoError(t, err)

	return connector, store
}

func TestBigQueryConnector_Export_Connected(t *testing.T) {
	connector, store := setupBigQueryTestConnector(t)

	// Connect
	err := connector.Connect(context.Background())
	require.NoError(t, err)

	// Add test data
	err = store.Put("user:1", map[string]*domain.FeatureValue{
		"click_count":    {Value: int64(42), Timestamp: time.Now().UnixNano()},
		"purchase_total": {Value: 99.99, Timestamp: time.Now().UnixNano()},
	})
	require.NoError(t, err)

	// Export
	req := &ExportRequest{
		Table:       "features",
		Features:    []string{"click_count", "purchase_total"},
		Entities:    []string{"user:1"},
		Mode:        SyncModeFull,
		CreateTable: true,
	}

	result, err := connector.Export(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int64(1), result.RowsExported)
	assert.Equal(t, int64(1), result.EntitiesExported)
	assert.Equal(t, 2, result.FeaturesExported)
}

func TestBigQueryConnector_Export_ValidationErrors(t *testing.T) {
	connector, _ := setupBigQueryTestConnector(t)

	// Connect first
	err := connector.Connect(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name    string
		req     *ExportRequest
		wantErr string
	}{
		{
			name:    "empty table",
			req:     &ExportRequest{Features: []string{"click_count"}},
			wantErr: "table is required",
		},
		{
			name:    "empty features",
			req:     &ExportRequest{Table: "features"},
			wantErr: "features is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := connector.Export(context.Background(), tt.req)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestBigQueryConnector_Import_ValidationErrors(t *testing.T) {
	connector, _ := setupBigQueryTestConnector(t)

	// Connect first
	err := connector.Connect(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name    string
		req     *ImportRequest
		wantErr string
	}{
		{
			name:    "empty table and query",
			req:     &ImportRequest{EntityColumn: "entity_key", FeatureColumns: map[string]string{"a": "b"}},
			wantErr: "table or query is required",
		},
		{
			name:    "empty entity column",
			req:     &ImportRequest{Table: "features", FeatureColumns: map[string]string{"a": "b"}},
			wantErr: "entity_column is required",
		},
		{
			name:    "empty feature columns",
			req:     &ImportRequest{Table: "features", EntityColumn: "entity_key"},
			wantErr: "feature_columns is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := connector.Import(context.Background(), tt.req)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestBigQueryConnector_CreateTable_ValidationErrors(t *testing.T) {
	connector, _ := setupBigQueryTestConnector(t)

	// Connect first
	err := connector.Connect(context.Background())
	require.NoError(t, err)

	req := &CreateTableRequest{
		Table: "", // Empty table name
		Features: []domain.FeatureSpec{
			{Name: "click_count", DataType: domain.DataTypeInt64},
		},
	}

	err = connector.CreateTable(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table is required")
}

func TestBigQueryConnector_ListTables_Connected(t *testing.T) {
	connector, _ := setupBigQueryTestConnector(t)

	// Connect
	err := connector.Connect(context.Background())
	require.NoError(t, err)

	// List tables (simulated, returns empty)
	tables, err := connector.ListTables(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, tables)
}

func TestBigQueryConnector_GetTableSchema_Connected(t *testing.T) {
	connector, _ := setupBigQueryTestConnector(t)

	// Connect
	err := connector.Connect(context.Background())
	require.NoError(t, err)

	// Get schema (simulated)
	schema, err := connector.GetTableSchema(context.Background(), "features")
	require.NoError(t, err)
	require.NotNil(t, schema)

	assert.Equal(t, "features", schema.Table)
}

func TestBigQueryConnector_ExecuteQuery_Connected(t *testing.T) {
	connector, _ := setupBigQueryTestConnector(t)

	// Connect
	err := connector.Connect(context.Background())
	require.NoError(t, err)

	// Execute query (simulated)
	result, err := connector.ExecuteQuery(context.Background(), "SELECT 1")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotZero(t, result.Duration)
}

// Mock BigQuery client for testing
type mockBigQueryClient struct {
	tables   map[string]*BigQueryTableMetadata
	queryErr error
}

func (m *mockBigQueryClient) Close() error {
	return nil
}

func (m *mockBigQueryClient) Query(ctx context.Context, query string) (BigQueryIterator, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	return &mockIterator{}, nil
}

func (m *mockBigQueryClient) InsertRows(ctx context.Context, dataset, table string, rows []map[string]interface{}) error {
	return nil
}

func (m *mockBigQueryClient) CreateTable(ctx context.Context, dataset, table string, schema BigQuerySchema) error {
	return nil
}

func (m *mockBigQueryClient) TableExists(ctx context.Context, dataset, table string) (bool, error) {
	_, exists := m.tables[table]
	return exists, nil
}

func (m *mockBigQueryClient) GetTableMetadata(ctx context.Context, dataset, table string) (*BigQueryTableMetadata, error) {
	if meta, ok := m.tables[table]; ok {
		return meta, nil
	}
	return nil, ErrTableNotFound
}

func (m *mockBigQueryClient) ListTables(ctx context.Context, dataset string) ([]string, error) {
	names := make([]string, 0, len(m.tables))
	for name := range m.tables {
		names = append(names, name)
	}
	return names, nil
}

type mockIterator struct {
	called bool
}

func (m *mockIterator) Next(dst interface{}) error {
	if m.called {
		return ErrIteratorDone
	}
	m.called = true
	return nil
}

func TestBigQueryConnector_WithMockClient(t *testing.T) {
	connector, _ := setupBigQueryTestConnector(t)

	// Set mock client
	mockClient := &mockBigQueryClient{
		tables: map[string]*BigQueryTableMetadata{
			"features": {
				Name:         "features",
				NumRows:      1000,
				NumBytes:     1024 * 1024,
				CreationTime: time.Now(),
				LastModified: time.Now(),
				Schema: BigQuerySchema{
					Fields: []BigQueryFieldSchema{
						{Name: "entity_key", Type: "STRING", Mode: "REQUIRED"},
						{Name: "click_count", Type: "INT64", Mode: "NULLABLE"},
					},
				},
			},
		},
	}
	connector.SetClient(mockClient)

	// Connect
	err := connector.Connect(context.Background())
	require.NoError(t, err)

	// Test ListTables
	tables, err := connector.ListTables(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, tables, 1)
	assert.Equal(t, "features", tables[0].Name)

	// Test GetTableSchema
	schema, err := connector.GetTableSchema(context.Background(), "features")
	require.NoError(t, err)
	assert.Equal(t, "features", schema.Table)
	assert.Len(t, schema.Columns, 2)
}
