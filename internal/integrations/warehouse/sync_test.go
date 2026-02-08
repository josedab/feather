package warehouse

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

func TestDefaultSyncConfig(t *testing.T) {
	config := DefaultSyncConfig()

	assert.Equal(t, 15*time.Minute, config.SyncInterval)
	assert.Equal(t, 10000, config.BatchSize)
	assert.Equal(t, 4, config.MaxConcurrency)
	assert.Equal(t, 3, config.RetryAttempts)
	assert.Equal(t, 5*time.Second, config.RetryBackoff)
	assert.Equal(t, ConflictResolutionLatest, config.ConflictResolution)
	assert.True(t, config.EnableChangeTracking)
}

func TestSyncStatus_Values(t *testing.T) {
	assert.Equal(t, SyncStatus("pending"), SyncStatusPending)
	assert.Equal(t, SyncStatus("running"), SyncStatusRunning)
	assert.Equal(t, SyncStatus("completed"), SyncStatusCompleted)
	assert.Equal(t, SyncStatus("failed"), SyncStatusFailed)
	assert.Equal(t, SyncStatus("canceled"), SyncStatusCanceled)
}

func TestNewSyncEngine(t *testing.T) {
	config := DefaultSyncConfig()
	engine := NewSyncEngine(config, nil, nil, nil)

	require.NotNil(t, engine)
	assert.NotNil(t, engine.connectors)
	assert.NotNil(t, engine.jobs)
	assert.NotNil(t, engine.executions)
}

func TestSyncEngine_RegisterConnector(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	// Create mock connector
	mockConn := &mockConnector{
		connType: ConnectorTypeSnowflake,
		state:    ConnectionStateDisconnected,
	}

	// Register connector
	err := engine.RegisterConnector("snowflake", mockConn)
	require.NoError(t, err)

	// Verify registration
	connectors := engine.ListConnectors()
	assert.Len(t, connectors, 1)
	assert.Equal(t, ConnectorTypeSnowflake, connectors["snowflake"])
}

func TestSyncEngine_RegisterConnector_Duplicate(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	mockConn := &mockConnector{connType: ConnectorTypeSnowflake}

	// Register first time
	err := engine.RegisterConnector("snowflake", mockConn)
	require.NoError(t, err)

	// Register again should fail
	err = engine.RegisterConnector("snowflake", mockConn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestSyncEngine_UnregisterConnector(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	mockConn := &mockConnector{connType: ConnectorTypeSnowflake}
	err := engine.RegisterConnector("snowflake", mockConn)
	require.NoError(t, err)

	// Unregister
	err = engine.UnregisterConnector("snowflake")
	require.NoError(t, err)

	// Verify removal
	connectors := engine.ListConnectors()
	assert.Empty(t, connectors)
}

func TestSyncEngine_UnregisterConnector_NotFound(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	err := engine.UnregisterConnector("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSyncEngine_GetConnector(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	mockConn := &mockConnector{connType: ConnectorTypeSnowflake}
	err := engine.RegisterConnector("snowflake", mockConn)
	require.NoError(t, err)

	// Get existing connector
	conn, err := engine.GetConnector("snowflake")
	require.NoError(t, err)
	assert.Equal(t, mockConn, conn)

	// Get non-existent connector
	_, err = engine.GetConnector("nonexistent")
	assert.Error(t, err)
}

func TestSyncEngine_CreateJob(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	job := &SyncJob{
		ID:        "job-1",
		Name:      "Test Job",
		Direction: SyncDirectionExport,
		Mode:      SyncModeFull,
		Source:    "",
		Target:    "features",
		Features:  []string{"click_count", "purchase_total"},
		Enabled:   true,
	}

	err := engine.CreateJob(job)
	require.NoError(t, err)

	// Verify job was created
	retrieved, err := engine.GetJob("job-1")
	require.NoError(t, err)
	assert.Equal(t, "Test Job", retrieved.Name)
	assert.NotZero(t, retrieved.CreatedAt)
	assert.NotZero(t, retrieved.UpdatedAt)
}

func TestSyncEngine_CreateJob_EmptyID(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	job := &SyncJob{
		Name: "Test Job",
	}

	err := engine.CreateJob(job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ID is required")
}

func TestSyncEngine_CreateJob_Duplicate(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	job := &SyncJob{
		ID:   "job-1",
		Name: "Test Job",
	}

	err := engine.CreateJob(job)
	require.NoError(t, err)

	err = engine.CreateJob(job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestSyncEngine_UpdateJob(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	job := &SyncJob{
		ID:   "job-1",
		Name: "Test Job",
	}
	err := engine.CreateJob(job)
	require.NoError(t, err)

	// Update job
	job.Name = "Updated Job"
	err = engine.UpdateJob(job)
	require.NoError(t, err)

	// Verify update
	retrieved, err := engine.GetJob("job-1")
	require.NoError(t, err)
	assert.Equal(t, "Updated Job", retrieved.Name)
}

func TestSyncEngine_UpdateJob_NotFound(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	job := &SyncJob{
		ID:   "nonexistent",
		Name: "Test Job",
	}

	err := engine.UpdateJob(job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSyncEngine_DeleteJob(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	job := &SyncJob{
		ID:   "job-1",
		Name: "Test Job",
	}
	err := engine.CreateJob(job)
	require.NoError(t, err)

	// Delete job
	err = engine.DeleteJob("job-1")
	require.NoError(t, err)

	// Verify deletion
	_, err = engine.GetJob("job-1")
	assert.Error(t, err)
}

func TestSyncEngine_DeleteJob_NotFound(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	err := engine.DeleteJob("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSyncEngine_ListJobs(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	// Create multiple jobs
	for i := 0; i < 3; i++ {
		job := &SyncJob{
			ID:   "job-" + string(rune('a'+i)),
			Name: "Test Job",
		}
		err := engine.CreateJob(job)
		require.NoError(t, err)
	}

	jobs := engine.ListJobs()
	assert.Len(t, jobs, 3)
}

func TestSyncEngine_ExecuteJob_Export(t *testing.T) {
	// Setup store
	schemaReg := &mockSchemaRegistry{
		features: map[string]*domain.FeatureSpec{
			"click_count": {Name: "click_count", DataType: domain.DataTypeInt64},
		},
	}

	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, schemaReg)
	require.NoError(t, err)
	defer store.Close()

	// Add test data
	err = store.Put("user:1", map[string]*domain.FeatureValue{
		"click_count": {Value: int64(42), Timestamp: time.Now().UnixNano()},
	})
	require.NoError(t, err)

	engine := NewSyncEngine(DefaultSyncConfig(), store, schemaReg, nil)

	// Register mock connector
	mockConn := &mockConnector{
		connType: ConnectorTypeSnowflake,
		state:    ConnectionStateConnected,
		store:    store,
	}
	err = engine.RegisterConnector("snowflake", mockConn)
	require.NoError(t, err)

	// Create export job
	job := &SyncJob{
		ID:        "export-job",
		Name:      "Export Job",
		Direction: SyncDirectionExport,
		Mode:      SyncModeFull,
		Target:    "features",
		Features:  []string{"click_count"},
		Enabled:   true,
	}
	err = engine.CreateJob(job)
	require.NoError(t, err)

	// Execute job
	execution, err := engine.ExecuteJob(context.Background(), "export-job", "snowflake")
	require.NoError(t, err)
	require.NotNil(t, execution)

	assert.Equal(t, "export-job", execution.JobID)
	assert.Equal(t, SyncStatusCompleted, execution.Status)
	assert.NotNil(t, execution.CompletedAt)
}

func TestSyncEngine_ExecuteJob_Import(t *testing.T) {
	// Setup store
	schemaReg := &mockSchemaRegistry{
		features: map[string]*domain.FeatureSpec{
			"click_count": {Name: "click_count", DataType: domain.DataTypeInt64},
		},
	}

	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024,
		WarmInMemory: true,
	}, schemaReg)
	require.NoError(t, err)
	defer store.Close()

	engine := NewSyncEngine(DefaultSyncConfig(), store, schemaReg, nil)

	// Register mock connector
	mockConn := &mockConnector{
		connType: ConnectorTypeBigQuery,
		state:    ConnectionStateConnected,
		store:    store,
	}
	err = engine.RegisterConnector("bigquery", mockConn)
	require.NoError(t, err)

	// Create import job
	job := &SyncJob{
		ID:           "import-job",
		Name:         "Import Job",
		Direction:    SyncDirectionImport,
		Mode:         SyncModeIncremental,
		Source:       "features",
		EntityColumn: "entity_key",
		FeatureMapping: map[string]string{
			"clicks": "click_count",
		},
		Enabled: true,
	}
	err = engine.CreateJob(job)
	require.NoError(t, err)

	// Execute job
	execution, err := engine.ExecuteJob(context.Background(), "import-job", "bigquery")
	require.NoError(t, err)
	require.NotNil(t, execution)

	assert.Equal(t, "import-job", execution.JobID)
	assert.Equal(t, SyncStatusCompleted, execution.Status)
}

func TestSyncEngine_ExecuteJob_NotFound(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	mockConn := &mockConnector{connType: ConnectorTypeSnowflake}
	err := engine.RegisterConnector("snowflake", mockConn)
	require.NoError(t, err)

	// Try to execute non-existent job
	_, err = engine.ExecuteJob(context.Background(), "nonexistent", "snowflake")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSyncEngine_ExecuteJob_ConnectorNotFound(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	job := &SyncJob{
		ID:   "job-1",
		Name: "Test Job",
	}
	err := engine.CreateJob(job)
	require.NoError(t, err)

	// Try to execute with non-existent connector
	_, err = engine.ExecuteJob(context.Background(), "job-1", "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSyncEngine_GetExecution(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	// Add mock connector and job
	mockConn := &mockConnector{connType: ConnectorTypeSnowflake, state: ConnectionStateConnected}
	err := engine.RegisterConnector("snowflake", mockConn)
	require.NoError(t, err)

	job := &SyncJob{
		ID:        "job-1",
		Direction: SyncDirectionExport,
		Target:    "features",
		Features:  []string{"click_count"},
	}
	err = engine.CreateJob(job)
	require.NoError(t, err)

	// Execute to create an execution
	execution, err := engine.ExecuteJob(context.Background(), "job-1", "snowflake")
	require.NoError(t, err)

	// Get execution
	retrieved, err := engine.GetExecution(execution.ID)
	require.NoError(t, err)
	assert.Equal(t, execution.ID, retrieved.ID)
}

func TestSyncEngine_GetExecution_NotFound(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	_, err := engine.GetExecution("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSyncEngine_ListExecutions(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	// Add mock connector
	mockConn := &mockConnector{connType: ConnectorTypeSnowflake, state: ConnectionStateConnected}
	err := engine.RegisterConnector("snowflake", mockConn)
	require.NoError(t, err)

	// Create job
	job := &SyncJob{
		ID:        "job-1",
		Direction: SyncDirectionExport,
		Target:    "features",
		Features:  []string{"click_count"},
	}
	err = engine.CreateJob(job)
	require.NoError(t, err)

	// Execute multiple times
	for i := 0; i < 3; i++ {
		_, err = engine.ExecuteJob(context.Background(), "job-1", "snowflake")
		require.NoError(t, err)
	}

	// List executions
	executions := engine.ListExecutions("job-1")
	assert.Len(t, executions, 3)
}

func TestSyncEngine_StartStop(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	// Start engine
	engine.Start()

	// Start again should be idempotent
	engine.Start()

	// Stop engine
	engine.Stop()
}

func TestSyncEngine_Stats(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	// Register connector and job
	mockConn := &mockConnector{connType: ConnectorTypeSnowflake, state: ConnectionStateConnected}
	err := engine.RegisterConnector("snowflake", mockConn)
	require.NoError(t, err)

	job := &SyncJob{
		ID:        "job-1",
		Direction: SyncDirectionExport,
		Target:    "features",
		Features:  []string{"click_count"},
	}
	err = engine.CreateJob(job)
	require.NoError(t, err)

	// Execute job
	_, err = engine.ExecuteJob(context.Background(), "job-1", "snowflake")
	require.NoError(t, err)

	// Get stats
	stats := engine.Stats()
	assert.Equal(t, 1, stats["connectors"])
	assert.Equal(t, 1, stats["jobs"])
	assert.Equal(t, 1, stats["executions"])
	assert.Equal(t, int64(1), stats["sync_count"])
}

func TestSyncEngine_MarshalJSON(t *testing.T) {
	engine := NewSyncEngine(DefaultSyncConfig(), nil, nil, nil)

	mockConn := &mockConnector{connType: ConnectorTypeSnowflake}
	err := engine.RegisterConnector("snowflake", mockConn)
	require.NoError(t, err)

	data, err := engine.MarshalJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "snowflake")
}

func TestSyncJob_Fields(t *testing.T) {
	now := time.Now()
	job := &SyncJob{
		ID:              "job-1",
		Name:            "Test Job",
		Direction:       SyncDirectionExport,
		Mode:            SyncModeFull,
		Source:          "warehouse.features",
		Target:          "features",
		Features:        []string{"click_count", "purchase_total"},
		EntityColumn:    "entity_key",
		TimestampColumn: "updated_at",
		FeatureMapping:  map[string]string{"clicks": "click_count"},
		Filter:          "updated_at > '2024-01-01'",
		Schedule:        "0 * * * *",
		Enabled:         true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	_ = job.Name
	_ = job.Mode
	_ = job.Source
	_ = job.Target
	_ = job.EntityColumn
	_ = job.TimestampColumn
	_ = job.FeatureMapping
	_ = job.Filter
	_ = job.Schedule
	_ = job.CreatedAt
	_ = job.UpdatedAt
	assert.Equal(t, "job-1", job.ID)
	assert.Equal(t, SyncDirectionExport, job.Direction)
	assert.Len(t, job.Features, 2)
	assert.True(t, job.Enabled)
}

func TestSyncExecution_Fields(t *testing.T) {
	now := time.Now()
	execution := &SyncExecution{
		ID:               "exec-1",
		JobID:            "job-1",
		Status:           SyncStatusCompleted,
		StartedAt:        now,
		CompletedAt:      &now,
		RowsSynced:       1000,
		RowsFailed:       10,
		BytesTransferred: 1024 * 100,
		Checkpoint:       "offset=1000",
	}

	_ = execution.JobID
	_ = execution.StartedAt
	_ = execution.BytesTransferred
	_ = execution.Checkpoint
	assert.Equal(t, "exec-1", execution.ID)
	assert.Equal(t, SyncStatusCompleted, execution.Status)
	assert.Equal(t, int64(1000), execution.RowsSynced)
	assert.Equal(t, int64(10), execution.RowsFailed)
	assert.NotNil(t, execution.CompletedAt)
}

// Mock connector for testing
type mockConnector struct {
	connType ConnectorType
	state    ConnectionState
	store    *storage.Store
}

func (m *mockConnector) Connect(ctx context.Context) error {
	m.state = ConnectionStateConnected
	return nil
}

func (m *mockConnector) Close() error {
	m.state = ConnectionStateDisconnected
	return nil
}

func (m *mockConnector) State() ConnectionState {
	return m.state
}

func (m *mockConnector) Type() ConnectorType {
	return m.connType
}

func (m *mockConnector) Ping(ctx context.Context) error {
	if m.state != ConnectionStateConnected {
		return ErrConnectorNotConnected
	}
	return nil
}

func (m *mockConnector) Export(ctx context.Context, req *ExportRequest) (*ExportResult, error) {
	return &ExportResult{
		RowsExported:     100,
		BytesExported:    1024,
		EntitiesExported: 50,
		FeaturesExported: len(req.Features),
		Duration:         time.Millisecond * 100,
		Table:            req.Table,
	}, nil
}

func (m *mockConnector) Import(ctx context.Context, req *ImportRequest) (*ImportResult, error) {
	return &ImportResult{
		RowsImported:    100,
		FeaturesUpdated: 500,
		EntitiesUpdated: 100,
		Duration:        time.Millisecond * 100,
	}, nil
}

func (m *mockConnector) ListTables(ctx context.Context, schema string) ([]TableInfo, error) {
	return []TableInfo{
		{Name: "features", Schema: schema, RowCount: 1000},
	}, nil
}

func (m *mockConnector) GetTableSchema(ctx context.Context, table string) (*TableSchema, error) {
	return &TableSchema{
		Table: table,
		Columns: []ColumnInfo{
			{Name: "entity_key", Type: "VARCHAR"},
			{Name: "click_count", Type: "BIGINT"},
		},
	}, nil
}

func (m *mockConnector) CreateTable(ctx context.Context, req *CreateTableRequest) error {
	return nil
}

func (m *mockConnector) ExecuteQuery(ctx context.Context, query string) (*QueryResult, error) {
	return &QueryResult{
		Columns:  []string{"result"},
		Rows:     [][]interface{}{{int64(1)}},
		RowCount: 1,
		Duration: time.Millisecond * 10,
	}, nil
}
