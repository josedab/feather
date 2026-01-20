package warehouse

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultSnowflakeConfig(t *testing.T) {
	config := DefaultSnowflakeConfig()

	assert.Equal(t, "PUBLIC", config.Schema)
	assert.True(t, config.UseStagingArea)
	assert.Equal(t, "FEATHER_STAGING", config.StagingSchema)
	assert.NotZero(t, config.ConnectionTimeout)
	assert.NotZero(t, config.QueryTimeout)
}

func TestSnowflakeConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  SnowflakeConfig
		wantErr bool
	}{
		{
			name: "valid config with password",
			config: SnowflakeConfig{
				Account:   "test-account",
				User:      "test-user",
				Password:  "test-password",
				Database:  "test-db",
				Warehouse: "test-warehouse",
			},
			wantErr: false,
		},
		{
			name: "valid config with private key",
			config: SnowflakeConfig{
				Account:    "test-account",
				User:       "test-user",
				PrivateKey: "-----BEGIN PRIVATE KEY-----\n...",
				Database:   "test-db",
				Warehouse:  "test-warehouse",
			},
			wantErr: false,
		},
		{
			name: "missing account",
			config: SnowflakeConfig{
				User:      "test-user",
				Password:  "test-password",
				Database:  "test-db",
				Warehouse: "test-warehouse",
			},
			wantErr: true,
		},
		{
			name: "missing user",
			config: SnowflakeConfig{
				Account:   "test-account",
				Password:  "test-password",
				Database:  "test-db",
				Warehouse: "test-warehouse",
			},
			wantErr: true,
		},
		{
			name: "missing credentials",
			config: SnowflakeConfig{
				Account:   "test-account",
				User:      "test-user",
				Database:  "test-db",
				Warehouse: "test-warehouse",
			},
			wantErr: true,
		},
		{
			name: "missing database",
			config: SnowflakeConfig{
				Account:   "test-account",
				User:      "test-user",
				Password:  "test-password",
				Warehouse: "test-warehouse",
			},
			wantErr: true,
		},
		{
			name: "missing warehouse",
			config: SnowflakeConfig{
				Account:  "test-account",
				User:     "test-user",
				Password: "test-password",
				Database: "test-db",
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

func TestNewSnowflakeConnector(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Warehouse: "test-warehouse",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, logger)
	require.NoError(t, err)
	require.NotNil(t, connector)

	assert.Equal(t, ConnectorTypeSnowflake, connector.Type())
	assert.Equal(t, ConnectionStateDisconnected, connector.State())
}

func TestNewSnowflakeConnector_InvalidConfig(t *testing.T) {
	config := SnowflakeConfig{} // Empty config

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
	assert.Error(t, err)
	assert.Nil(t, connector)
}

func TestSnowflakeConnector_State(t *testing.T) {
	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Warehouse: "test-warehouse",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, ConnectionStateDisconnected, connector.State())
}

func TestSnowflakeConnector_Type(t *testing.T) {
	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Warehouse: "test-warehouse",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, ConnectorTypeSnowflake, connector.Type())
}

func TestSnowflakeConnector_Close_NotConnected(t *testing.T) {
	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Warehouse: "test-warehouse",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
	require.NoError(t, err)

	// Should not error when not connected
	err = connector.Close()
	assert.NoError(t, err)
	assert.Equal(t, ConnectionStateDisconnected, connector.State())
}

func TestSnowflakeConnector_Ping_NotConnected(t *testing.T) {
	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Warehouse: "test-warehouse",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
	require.NoError(t, err)

	err = connector.Ping(context.Background())
	assert.ErrorIs(t, err, ErrConnectorNotConnected)
}

func TestSnowflakeConnector_Export_NotConnected(t *testing.T) {
	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Warehouse: "test-warehouse",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
	require.NoError(t, err)

	req := &ExportRequest{
		Table:    "features",
		Features: []string{"click_count"},
	}

	result, err := connector.Export(context.Background(), req)
	assert.ErrorIs(t, err, ErrConnectorNotConnected)
	assert.Nil(t, result)
}

func TestSnowflakeConnector_Import_NotConnected(t *testing.T) {
	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Warehouse: "test-warehouse",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
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

func TestSnowflakeConnector_ListTables_NotConnected(t *testing.T) {
	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Warehouse: "test-warehouse",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
	require.NoError(t, err)

	tables, err := connector.ListTables(context.Background(), "public")
	assert.ErrorIs(t, err, ErrConnectorNotConnected)
	assert.Nil(t, tables)
}

func TestSnowflakeConnector_GetTableSchema_NotConnected(t *testing.T) {
	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Warehouse: "test-warehouse",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
	require.NoError(t, err)

	schema, err := connector.GetTableSchema(context.Background(), "features")
	assert.ErrorIs(t, err, ErrConnectorNotConnected)
	assert.Nil(t, schema)
}

func TestSnowflakeConnector_CreateTable_NotConnected(t *testing.T) {
	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Warehouse: "test-warehouse",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
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

func TestSnowflakeConnector_ExecuteQuery_NotConnected(t *testing.T) {
	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Warehouse: "test-warehouse",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
	require.NoError(t, err)

	result, err := connector.ExecuteQuery(context.Background(), "SELECT 1")
	assert.ErrorIs(t, err, ErrConnectorNotConnected)
	assert.Nil(t, result)
}

func TestSnowflakeConnector_Metrics(t *testing.T) {
	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Warehouse: "test-warehouse",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
	require.NoError(t, err)

	metrics := connector.Metrics()
	assert.Equal(t, int64(0), metrics.ConnectionAttempts)
	assert.Equal(t, int64(0), metrics.RowsExported)
}

func TestSnowflakeConnector_buildConnectionString(t *testing.T) {
	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Schema:    "PUBLIC",
		Warehouse: "test-warehouse",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
	require.NoError(t, err)

	connStr := connector.buildConnectionString()
	assert.Contains(t, connStr, "test-user")
	assert.Contains(t, connStr, "test-account")
	assert.Contains(t, connStr, "test-db")
	assert.Contains(t, connStr, "PUBLIC")
	assert.Contains(t, connStr, "warehouse=test-warehouse")
}

func TestSnowflakeConnector_buildConnectionString_WithRegion(t *testing.T) {
	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Schema:    "PUBLIC",
		Warehouse: "test-warehouse",
		Region:    "us-east-1",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
	require.NoError(t, err)

	connStr := connector.buildConnectionString()
	assert.Contains(t, connStr, "us-east-1")
}

func TestSnowflakeConnector_buildConnectionString_WithRole(t *testing.T) {
	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Schema:    "PUBLIC",
		Warehouse: "test-warehouse",
		Role:      "SYSADMIN",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
	require.NoError(t, err)

	connStr := connector.buildConnectionString()
	assert.Contains(t, connStr, "role=SYSADMIN")
}

func TestSnowflakeConnector_buildImportQuery(t *testing.T) {
	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Schema:    "PUBLIC",
		Warehouse: "test-warehouse",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
	require.NoError(t, err)

	req := &ImportRequest{
		Table:           "features",
		EntityColumn:    "entity_key",
		TimestampColumn: "updated_at",
		FeatureColumns: map[string]string{
			"clicks":    "click_count",
			"purchases": "purchase_total",
		},
		Filter: "updated_at > '2024-01-01'",
		Limit:  1000,
	}

	query := connector.buildImportQuery(req)
	assert.Contains(t, query, "SELECT entity_key")
	assert.Contains(t, query, "updated_at")
	assert.Contains(t, query, "FROM PUBLIC.features")
	assert.Contains(t, query, "WHERE updated_at > '2024-01-01'")
	assert.Contains(t, query, "LIMIT 1000")
}

func TestSnowflakeConnector_buildImportQuery_WithSchema(t *testing.T) {
	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Schema:    "PUBLIC",
		Warehouse: "test-warehouse",
	}

	connector, err := NewSnowflakeConnector(config, nil, nil, nil)
	require.NoError(t, err)

	req := &ImportRequest{
		Table:        "features",
		Schema:       "CUSTOM_SCHEMA",
		EntityColumn: "entity_key",
		FeatureColumns: map[string]string{
			"clicks": "click_count",
		},
	}

	query := connector.buildImportQuery(req)
	assert.Contains(t, query, "FROM CUSTOM_SCHEMA.features")
}

func TestSerializeValueForSnowflake(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{
			name:    "float32 slice",
			input:   []float32{1.0, 2.0, 3.0},
			wantErr: false,
		},
		{
			name:    "float64 slice",
			input:   []float64{1.0, 2.0, 3.0},
			wantErr: false,
		},
		{
			name:    "bytes",
			input:   []byte("hello"),
			wantErr: false,
		},
		{
			name:    "time",
			input:   time.Now(),
			wantErr: false,
		},
		{
			name:    "string",
			input:   "test",
			wantErr: false,
		},
		{
			name:    "int64",
			input:   int64(42),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := serializeValueForSnowflake(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// Mock schema registry for testing
type mockSchemaRegistry struct {
	features map[string]*domain.FeatureSpec
}

func (m *mockSchemaRegistry) GetGroup(name string) (*domain.FeatureGroup, error) {
	return nil, nil
}

func (m *mockSchemaRegistry) GetFeatureSpec(name string) (*domain.FeatureSpec, error) {
	if spec, ok := m.features[name]; ok {
		return spec, nil
	}
	return nil, domain.ErrFeatureNotFound
}

func (m *mockSchemaRegistry) ListGroups() []*domain.FeatureGroup {
	return nil
}

func setupTestConnectorWithStore(t *testing.T) (*SnowflakeConnector, *storage.Store) {
	t.Helper()

	// Create mock schema registry
	schemaReg := &mockSchemaRegistry{
		features: map[string]*domain.FeatureSpec{
			"click_count":    {Name: "click_count", DataType: domain.DataTypeInt64},
			"purchase_total": {Name: "purchase_total", DataType: domain.DataTypeFloat64},
		},
	}

	// Create store with in-memory warm tier
	store, err := storage.NewStore(storage.StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, schemaReg)
	require.NoError(t, err)

	config := SnowflakeConfig{
		Account:   "test-account",
		User:      "test-user",
		Password:  "test-password",
		Database:  "test-db",
		Schema:    "PUBLIC",
		Warehouse: "test-warehouse",
	}

	connector, err := NewSnowflakeConnector(config, store, schemaReg, nil)
	require.NoError(t, err)

	return connector, store
}

func TestSnowflakeConnector_Export_ValidationErrors(t *testing.T) {
	connector, _ := setupTestConnectorWithStore(t)

	// Manually set state to connected for validation testing
	connector.mu.Lock()
	connector.state = ConnectionStateConnected
	connector.mu.Unlock()

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

func TestSnowflakeConnector_Import_ValidationErrors(t *testing.T) {
	connector, _ := setupTestConnectorWithStore(t)

	// Manually set state to connected for validation testing
	connector.mu.Lock()
	connector.state = ConnectionStateConnected
	connector.mu.Unlock()

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

func TestSnowflakeConnector_CreateTable_ValidationErrors(t *testing.T) {
	connector, _ := setupTestConnectorWithStore(t)

	// Manually set state to connected for validation testing
	connector.mu.Lock()
	connector.state = ConnectionStateConnected
	connector.mu.Unlock()

	req := &CreateTableRequest{
		Table: "", // Empty table name
		Features: []domain.FeatureSpec{
			{Name: "click_count", DataType: domain.DataTypeInt64},
		},
	}

	err := connector.CreateTable(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table is required")
}
