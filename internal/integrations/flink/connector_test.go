package flink

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

// newTestStore creates a store for testing with in-memory warm tier
func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	schema := storage.NewRegistry()
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024 * 10, // 10MB
		WarmInMemory: true,
	}, schema)
	require.NoError(t, err)
	return store
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "localhost:8081", cfg.JobManagerAddress)
	assert.Equal(t, 4, cfg.TaskSlots)
	assert.Equal(t, 4, cfg.Parallelism)
	assert.Equal(t, 10000, cfg.BufferSize)
	assert.Equal(t, GuaranteeAtLeastOnce, cfg.DeliveryGuarantee)
	assert.Equal(t, CheckpointModeAligned, cfg.CheckpointMode)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "valid default config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "zero buffer size gets default",
			config: Config{
				BufferSize: 0,
			},
			wantErr: false,
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

func TestNewConnector(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	t.Run("valid config", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.EnableMetrics = false // Disable for testing
		connector, err := NewConnector(cfg, store, nil, nil)
		require.NoError(t, err)
		assert.NotNil(t, connector)
		assert.Equal(t, StateUninitialized, connector.State())
	})

	t.Run("nil store", func(t *testing.T) {
		cfg := DefaultConfig()
		_, err := NewConnector(cfg, nil, nil, nil)
		assert.Error(t, err)
	})
}

func TestConnectorLifecycle(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	cfg.EnableMetrics = false
	connector, err := NewConnector(cfg, store, nil, nil)
	require.NoError(t, err)

	// Start
	ctx := context.Background()
	err = connector.Start(ctx)
	require.NoError(t, err)
	assert.Equal(t, StateRunning, connector.State())

	// Start again should be idempotent
	err = connector.Start(ctx)
	require.NoError(t, err)

	// Stop
	err = connector.Stop()
	require.NoError(t, err)
	assert.Equal(t, StateStopped, connector.State())

	// Stop again should be idempotent
	err = connector.Stop()
	require.NoError(t, err)
}

func TestProcessRecord(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	cfg.EnableMetrics = false
	cfg.FlushInterval = 10 * time.Millisecond
	connector, err := NewConnector(cfg, store, nil, nil)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, connector.Start(ctx))
	defer connector.Stop()

	t.Run("valid record", func(t *testing.T) {
		record := StreamRecord{
			EntityKey: "user:1",
			Features: map[string]interface{}{
				"click_count":    int64(10),
				"purchase_total": 99.99,
			},
			Timestamp: time.Now(),
		}

		err := connector.ProcessRecord(ctx, record)
		require.NoError(t, err)

		// Wait for flush
		time.Sleep(50 * time.Millisecond)

		// Verify data was written
		features, err := store.Get("user:1", []string{"click_count", "purchase_total"})
		require.NoError(t, err)
		assert.NotNil(t, features["click_count"])
		assert.NotNil(t, features["purchase_total"])
	})

	t.Run("missing entity key", func(t *testing.T) {
		record := StreamRecord{
			Features: map[string]interface{}{
				"click_count": int64(10),
			},
		}

		err := connector.ProcessRecord(ctx, record)
		assert.Error(t, err)
	})

	t.Run("with watermark", func(t *testing.T) {
		wm := time.Now()
		record := StreamRecord{
			EntityKey: "user:2",
			Features: map[string]interface{}{
				"clicks": int64(5),
			},
			Timestamp: time.Now(),
			Watermark: &wm,
		}

		err := connector.ProcessRecord(ctx, record)
		require.NoError(t, err)

		// Check watermark was updated
		currentWM := connector.CurrentWatermark()
		assert.False(t, currentWM.IsZero())
	})
}

func TestProcessBatch(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	cfg.EnableMetrics = false
	cfg.FlushInterval = 10 * time.Millisecond
	connector, err := NewConnector(cfg, store, nil, nil)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, connector.Start(ctx))
	defer connector.Stop()

	records := []StreamRecord{
		{EntityKey: "batch:1", Features: map[string]interface{}{"a": 1}, Timestamp: time.Now()},
		{EntityKey: "batch:2", Features: map[string]interface{}{"a": 2}, Timestamp: time.Now()},
		{EntityKey: "batch:3", Features: map[string]interface{}{"a": 3}, Timestamp: time.Now()},
	}

	err = connector.ProcessBatch(ctx, records)
	require.NoError(t, err)

	// Wait for flush
	time.Sleep(50 * time.Millisecond)

	// Verify data was written
	for i := 1; i <= 3; i++ {
		features, err := store.Get("batch:"+string(rune('0'+i)), []string{"a"})
		require.NoError(t, err)
		assert.NotNil(t, features["a"])
	}
}

func TestCheckpoint(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	cfg.EnableMetrics = false
	cfg.FlushInterval = 10 * time.Millisecond
	connector, err := NewConnector(cfg, store, nil, nil)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, connector.Start(ctx))
	defer connector.Stop()

	// Process some records
	for i := 0; i < 5; i++ {
		connector.ProcessRecord(ctx, StreamRecord{
			EntityKey: "cp:" + string(rune('0'+i)),
			Features:  map[string]interface{}{"a": i},
			Timestamp: time.Now(),
		})
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Trigger checkpoint
	checkpoint, err := connector.TriggerCheckpoint(ctx)
	require.NoError(t, err)
	assert.NotNil(t, checkpoint)
	assert.True(t, checkpoint.Completed)
	assert.Greater(t, checkpoint.ID, int64(0))
	assert.GreaterOrEqual(t, checkpoint.ProcessedRecords, int64(5))

	// Verify last checkpoint
	lastCP := connector.GetLastCheckpoint()
	assert.Equal(t, checkpoint.ID, lastCP.ID)
}

func TestRestoreFromCheckpoint(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	cfg.EnableMetrics = false
	connector, err := NewConnector(cfg, store, nil, nil)
	require.NoError(t, err)

	checkpoint := &Checkpoint{
		ID:               42,
		Timestamp:        time.Now(),
		ProcessedRecords: 1000,
		LastWatermark:    time.Now().Add(-time.Minute),
		Completed:        true,
	}

	err = connector.RestoreFromCheckpoint(checkpoint)
	require.NoError(t, err)

	// Verify state was restored
	assert.Equal(t, checkpoint.LastWatermark, connector.CurrentWatermark())
	assert.Equal(t, checkpoint.ID, connector.GetLastCheckpoint().ID)
}

func TestMetrics(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	cfg.EnableMetrics = false
	cfg.FlushInterval = 10 * time.Millisecond
	connector, err := NewConnector(cfg, store, nil, nil)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, connector.Start(ctx))
	defer connector.Stop()

	// Process records
	for i := 0; i < 10; i++ {
		connector.ProcessRecord(ctx, StreamRecord{
			EntityKey: "metrics:" + string(rune('0'+i)),
			Features:  map[string]interface{}{"a": i},
			Timestamp: time.Now(),
		})
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	metrics := connector.Metrics()
	assert.GreaterOrEqual(t, metrics.RecordsProcessed, int64(10))
	assert.GreaterOrEqual(t, metrics.BufferUtilization, float64(0))
}

func TestSink(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	cfg.EnableMetrics = false
	cfg.FlushInterval = 10 * time.Millisecond
	connector, err := NewConnector(cfg, store, nil, nil)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, connector.Start(ctx))
	defer connector.Stop()

	t.Run("valid sink", func(t *testing.T) {
		sinkConfig := SinkConfig{
			EntityColumn: "user_id",
			FeatureColumns: map[string]string{
				"clicks": "click_count",
				"spend":  "purchase_total",
			},
			BatchSize: 100,
		}

		sink, err := NewSink(connector, sinkConfig)
		require.NoError(t, err)
		assert.NotNil(t, sink)

		// Write data
		err = sink.Write(ctx, map[string]interface{}{
			"user_id": "sink:1",
			"clicks":  int64(50),
			"spend":   199.99,
		})
		require.NoError(t, err)

		// Wait for processing
		time.Sleep(50 * time.Millisecond)

		// Verify data
		features, err := store.Get("sink:1", []string{"click_count", "purchase_total"})
		require.NoError(t, err)
		assert.NotNil(t, features["click_count"])
		assert.NotNil(t, features["purchase_total"])
	})

	t.Run("missing entity column config", func(t *testing.T) {
		_, err := NewSink(connector, SinkConfig{
			FeatureColumns: map[string]string{"a": "b"},
		})
		assert.Error(t, err)
	})

	t.Run("missing feature columns config", func(t *testing.T) {
		_, err := NewSink(connector, SinkConfig{
			EntityColumn: "user_id",
		})
		assert.Error(t, err)
	})

	t.Run("missing entity in data", func(t *testing.T) {
		sink, _ := NewSink(connector, SinkConfig{
			EntityColumn:   "user_id",
			FeatureColumns: map[string]string{"a": "b"},
		})

		err := sink.Write(ctx, map[string]interface{}{
			"other_field": "value",
		})
		assert.Error(t, err)
	})

	t.Run("with timestamp column", func(t *testing.T) {
		sinkConfig := SinkConfig{
			EntityColumn:    "user_id",
			TimestampColumn: "event_time",
			FeatureColumns: map[string]string{
				"clicks": "click_count",
			},
		}

		sink, err := NewSink(connector, sinkConfig)
		require.NoError(t, err)

		ts := time.Now().Add(-time.Hour)
		err = sink.Write(ctx, map[string]interface{}{
			"user_id":    "sink:ts",
			"clicks":     int64(10),
			"event_time": ts.Format(time.RFC3339),
		})
		require.NoError(t, err)
	})
}

func TestSource(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Seed data
	for i := 0; i < 3; i++ {
		store.Put("source:"+string(rune('0'+i)), map[string]*domain.FeatureValue{
			"clicks": {Value: int64(i * 10), Timestamp: time.Now().UnixNano()},
		})
	}

	cfg := DefaultConfig()
	cfg.EnableMetrics = false
	connector, err := NewConnector(cfg, store, nil, nil)
	require.NoError(t, err)

	t.Run("valid source", func(t *testing.T) {
		sourceConfig := SourceConfig{
			Features:     []string{"clicks"},
			Entities:     []string{"source:0", "source:1", "source:2"},
			PollInterval: 10 * time.Millisecond,
		}

		source, err := NewSource(connector, sourceConfig)
		require.NoError(t, err)
		assert.NotNil(t, source)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		output := make(chan StreamRecord, 10)

		go func() {
			source.Read(ctx, output)
		}()

		// Wait for some records
		time.Sleep(50 * time.Millisecond)
		source.Stop()

		// Should have received some records
		count := len(output)
		assert.Greater(t, count, 0)
	})

	t.Run("missing features config", func(t *testing.T) {
		_, err := NewSource(connector, SourceConfig{})
		assert.Error(t, err)
	})
}

func TestGenerateFlinkJobCode(t *testing.T) {
	config := SinkConfig{
		EntityColumn: "user_id",
		FeatureColumns: map[string]string{
			"clicks": "click_count",
		},
	}

	code := GenerateFlinkJobCode(config)
	assert.Contains(t, code, "StreamExecutionEnvironment")
	assert.Contains(t, code, "enableCheckpointing")
	assert.Contains(t, code, "FeatherSink")
}

func TestGeneratePyFlinkCode(t *testing.T) {
	config := SinkConfig{
		EntityColumn: "user_id",
		FeatureColumns: map[string]string{
			"clicks": "click_count",
		},
	}

	code := GeneratePyFlinkCode(config)
	assert.Contains(t, code, "StreamExecutionEnvironment")
	assert.Contains(t, code, "enable_checkpointing")
	assert.Contains(t, code, "FeatherSink")
}

func TestContextCancellation(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	cfg.EnableMetrics = false
	connector, err := NewConnector(cfg, store, nil, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, connector.Start(ctx))
	defer connector.Stop()

	// Cancel context
	cancel()

	// Processing should fail gracefully
	err = connector.ProcessRecord(ctx, StreamRecord{
		EntityKey: "test",
		Features:  map[string]interface{}{"a": 1},
	})
	// May return context.Canceled or succeed depending on timing
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled)
	}
}
