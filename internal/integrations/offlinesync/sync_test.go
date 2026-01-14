package offlinesync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

	assert.Equal(t, 4, cfg.MaxConcurrentJobs)
	assert.Equal(t, 10000, cfg.DefaultBatchSize)
	assert.Equal(t, 30*time.Minute, cfg.DefaultTimeout)
	assert.Equal(t, 3, cfg.RetryAttempts)
	assert.True(t, cfg.EnableVersioning)
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
			name: "empty work dir gets default",
			config: Config{
				WorkDir: "",
			},
			wantErr: false,
		},
		{
			name: "zero values get defaults",
			config: Config{
				MaxConcurrentJobs: 0,
				DefaultBatchSize:  0,
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

func TestNewEngine(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	t.Run("valid config", func(t *testing.T) {
		cfg := DefaultConfig()
		engine, err := NewEngine(cfg, store, nil, nil)
		require.NoError(t, err)
		assert.NotNil(t, engine)
	})

	t.Run("nil store", func(t *testing.T) {
		cfg := DefaultConfig()
		_, err := NewEngine(cfg, nil, nil, nil)
		assert.Error(t, err)
	})
}

func TestEngineLifecycle(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, store, nil, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Start
	err = engine.Start(ctx)
	require.NoError(t, err)

	// Start again should be idempotent
	err = engine.Start(ctx)
	require.NoError(t, err)

	// Stop
	err = engine.Stop()
	require.NoError(t, err)

	// Stop again should be idempotent
	err = engine.Stop()
	require.NoError(t, err)
}

func TestCreateJob(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, store, nil, nil)
	require.NoError(t, err)
	defer engine.Close()

	t.Run("valid job", func(t *testing.T) {
		spec := &JobSpec{
			ID:           "test-job-1",
			Name:         "Test Job",
			Source:       "/data/features",
			SourceType:   SourceTypeJSON,
			EntityColumn: "user_id",
			FeatureColumns: map[string]string{
				"clicks": "click_count",
			},
		}

		job, err := engine.CreateJob(spec)
		require.NoError(t, err)
		assert.NotNil(t, job)
		assert.Equal(t, "test-job-1", job.Spec.ID)
		assert.Equal(t, JobStatusPending, job.Status)
	})

	t.Run("duplicate job", func(t *testing.T) {
		spec := &JobSpec{
			ID:           "test-job-1", // Same ID
			Name:         "Test Job 2",
			Source:       "/data/features",
			EntityColumn: "user_id",
			FeatureColumns: map[string]string{
				"clicks": "click_count",
			},
		}

		_, err := engine.CreateJob(spec)
		assert.ErrorIs(t, err, ErrJobAlreadyExists)
	})

	t.Run("scheduled job", func(t *testing.T) {
		spec := &JobSpec{
			ID:           "test-job-scheduled",
			Name:         "Scheduled Job",
			Source:       "/data/features",
			EntityColumn: "user_id",
			FeatureColumns: map[string]string{
				"clicks": "click_count",
			},
			Schedule: "0 2 * * *",
		}

		job, err := engine.CreateJob(spec)
		require.NoError(t, err)
		assert.Equal(t, JobStatusScheduled, job.Status)
		assert.NotNil(t, job.NextRunAt)
	})

	t.Run("missing required fields", func(t *testing.T) {
		tests := []struct {
			name string
			spec *JobSpec
		}{
			{
				name: "missing id",
				spec: &JobSpec{Source: "/data", EntityColumn: "id", FeatureColumns: map[string]string{"a": "b"}},
			},
			{
				name: "missing source",
				spec: &JobSpec{ID: "test", EntityColumn: "id", FeatureColumns: map[string]string{"a": "b"}},
			},
			{
				name: "missing entity column",
				spec: &JobSpec{ID: "test", Source: "/data", FeatureColumns: map[string]string{"a": "b"}},
			},
			{
				name: "missing features",
				spec: &JobSpec{ID: "test", Source: "/data", EntityColumn: "id"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := engine.CreateJob(tt.spec)
				assert.Error(t, err)
			})
		}
	})
}

func TestGetJob(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, store, nil, nil)
	require.NoError(t, err)
	defer engine.Close()

	// Create job
	spec := &JobSpec{
		ID:           "get-job-test",
		Name:         "Test Job",
		Source:       "/data",
		EntityColumn: "user_id",
		FeatureColumns: map[string]string{
			"clicks": "click_count",
		},
	}
	_, err = engine.CreateJob(spec)
	require.NoError(t, err)

	t.Run("existing job", func(t *testing.T) {
		job, err := engine.GetJob("get-job-test")
		require.NoError(t, err)
		assert.NotNil(t, job)
		assert.Equal(t, "get-job-test", job.Spec.ID)
	})

	t.Run("non-existing job", func(t *testing.T) {
		_, err := engine.GetJob("non-existing")
		assert.ErrorIs(t, err, ErrJobNotFound)
	})
}

func TestListJobs(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, store, nil, nil)
	require.NoError(t, err)
	defer engine.Close()

	// Create jobs
	for i := 0; i < 3; i++ {
		spec := &JobSpec{
			ID:           "list-job-" + string(rune('0'+i)),
			Name:         "Test Job",
			Source:       "/data",
			EntityColumn: "user_id",
			FeatureColumns: map[string]string{
				"clicks": "click_count",
			},
		}
		engine.CreateJob(spec)
	}

	t.Run("all jobs", func(t *testing.T) {
		jobs := engine.ListJobs(nil)
		assert.Len(t, jobs, 3)
	})

	t.Run("by status", func(t *testing.T) {
		status := JobStatusPending
		jobs := engine.ListJobs(&status)
		assert.Len(t, jobs, 3)
	})
}

func TestDeleteJob(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, store, nil, nil)
	require.NoError(t, err)
	defer engine.Close()

	// Create job
	spec := &JobSpec{
		ID:           "delete-job-test",
		Name:         "Test Job",
		Source:       "/data",
		EntityColumn: "user_id",
		FeatureColumns: map[string]string{
			"clicks": "click_count",
		},
	}
	engine.CreateJob(spec)

	t.Run("existing job", func(t *testing.T) {
		err := engine.DeleteJob("delete-job-test")
		require.NoError(t, err)

		_, err = engine.GetJob("delete-job-test")
		assert.ErrorIs(t, err, ErrJobNotFound)
	})

	t.Run("non-existing job", func(t *testing.T) {
		err := engine.DeleteJob("non-existing")
		assert.ErrorIs(t, err, ErrJobNotFound)
	})
}

func TestRunJob(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	cfg.DefaultTimeout = 5 * time.Second
	engine, err := NewEngine(cfg, store, nil, nil)
	require.NoError(t, err)
	defer engine.Close()

	ctx := context.Background()

	t.Run("run JSON job", func(t *testing.T) {
		// Create test data file
		tempDir := t.TempDir()
		testData := []map[string]interface{}{
			{"user_id": "user:1", "clicks": 10, "spend": 99.99},
			{"user_id": "user:2", "clicks": 20, "spend": 199.99},
			{"user_id": "user:3", "clicks": 30, "spend": 299.99},
		}
		data, _ := json.Marshal(testData)
		inputPath := filepath.Join(tempDir, "features.json")
		require.NoError(t, os.WriteFile(inputPath, data, 0644))

		// Create job
		spec := &JobSpec{
			ID:           "run-json-job",
			Name:         "JSON Sync Job",
			Source:       inputPath,
			SourceType:   SourceTypeJSON,
			EntityColumn: "user_id",
			FeatureColumns: map[string]string{
				"clicks": "click_count",
				"spend":  "purchase_total",
			},
			Strategy: SyncStrategyMerge,
		}
		_, err := engine.CreateJob(spec)
		require.NoError(t, err)

		// Run job
		execution, err := engine.RunJob(ctx, "run-json-job")
		require.NoError(t, err)
		assert.Equal(t, JobStatusCompleted, execution.Status)
		assert.Equal(t, int64(3), execution.RecordsSync)

		// Verify data was written
		features, err := store.Get("user:1", []string{"click_count", "purchase_total"})
		require.NoError(t, err)
		assert.NotNil(t, features["click_count"])
		assert.NotNil(t, features["purchase_total"])
	})

	t.Run("run CSV job", func(t *testing.T) {
		tempDir := t.TempDir()
		csvContent := `user_id,clicks,spend
user:10,100,999.99
user:11,110,1099.99`
		inputPath := filepath.Join(tempDir, "features.csv")
		require.NoError(t, os.WriteFile(inputPath, []byte(csvContent), 0644))

		spec := &JobSpec{
			ID:           "run-csv-job",
			Name:         "CSV Sync Job",
			Source:       inputPath,
			SourceType:   SourceTypeCSV,
			EntityColumn: "user_id",
			FeatureColumns: map[string]string{
				"clicks": "click_count",
			},
		}
		engine.CreateJob(spec)

		execution, err := engine.RunJob(ctx, "run-csv-job")
		require.NoError(t, err)
		assert.Equal(t, JobStatusCompleted, execution.Status)
		assert.Equal(t, int64(2), execution.RecordsSync)
	})

	t.Run("source not found", func(t *testing.T) {
		spec := &JobSpec{
			ID:           "missing-source-job",
			Name:         "Missing Source",
			Source:       "/nonexistent/path",
			EntityColumn: "user_id",
			FeatureColumns: map[string]string{
				"a": "b",
			},
		}
		engine.CreateJob(spec)

		execution, err := engine.RunJob(ctx, "missing-source-job")
		require.NoError(t, err)
		assert.Equal(t, JobStatusFailed, execution.Status)
		assert.Contains(t, execution.Error, "source not found")
	})

	t.Run("non-existing job", func(t *testing.T) {
		_, err := engine.RunJob(ctx, "non-existing-job")
		assert.ErrorIs(t, err, ErrJobNotFound)
	})

	t.Run("dependency not met", func(t *testing.T) {
		// Create dependent job first
		spec1 := &JobSpec{
			ID:           "dep-parent",
			Name:         "Parent Job",
			Source:       "/data",
			EntityColumn: "user_id",
			FeatureColumns: map[string]string{
				"a": "b",
			},
		}
		engine.CreateJob(spec1)

		// Create job with dependency
		spec2 := &JobSpec{
			ID:           "dep-child",
			Name:         "Child Job",
			Source:       "/data",
			EntityColumn: "user_id",
			FeatureColumns: map[string]string{
				"a": "b",
			},
			Dependencies: []string{"dep-parent"},
		}
		engine.CreateJob(spec2)

		// Try to run child before parent completes
		_, err := engine.RunJob(ctx, "dep-child")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dependency")
	})
}

func TestCancelJob(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, store, nil, nil)
	require.NoError(t, err)
	defer engine.Close()

	// Create job
	spec := &JobSpec{
		ID:           "cancel-job-test",
		Name:         "Test Job",
		Source:       "/data",
		EntityColumn: "user_id",
		FeatureColumns: map[string]string{
			"clicks": "click_count",
		},
	}
	engine.CreateJob(spec)

	t.Run("cancel non-running job", func(t *testing.T) {
		err := engine.CancelJob("cancel-job-test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	})

	t.Run("non-existing job", func(t *testing.T) {
		err := engine.CancelJob("non-existing")
		assert.ErrorIs(t, err, ErrJobNotFound)
	})
}

func TestMetrics(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, store, nil, nil)
	require.NoError(t, err)
	defer engine.Close()

	// Create and run a job
	tempDir := t.TempDir()
	testData := []map[string]interface{}{
		{"user_id": "user:1", "clicks": 10},
	}
	data, _ := json.Marshal(testData)
	inputPath := filepath.Join(tempDir, "features.json")
	os.WriteFile(inputPath, data, 0644)

	spec := &JobSpec{
		ID:           "metrics-job",
		Name:         "Metrics Test",
		Source:       inputPath,
		SourceType:   SourceTypeJSON,
		EntityColumn: "user_id",
		FeatureColumns: map[string]string{
			"clicks": "click_count",
		},
	}
	engine.CreateJob(spec)
	engine.RunJob(context.Background(), "metrics-job")

	metrics := engine.Metrics()
	assert.Equal(t, int64(1), metrics.JobsCreated)
	assert.Equal(t, int64(1), metrics.JobsCompleted)
	assert.Equal(t, int64(1), metrics.RecordsSynced)
}

func TestManifest(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, store, nil, nil)
	require.NoError(t, err)
	defer engine.Close()

	tempDir := t.TempDir()

	manifest := &ManifestFile{
		Version:     1,
		CreatedAt:   time.Now(),
		JobID:       "test-job",
		Features:    []string{"click_count", "purchase_total"},
		RecordCount: 1000,
		Metadata: map[string]interface{}{
			"source": "spark-job",
		},
	}

	t.Run("write manifest", func(t *testing.T) {
		err := engine.WriteManifest(tempDir, manifest)
		require.NoError(t, err)

		// Verify file exists
		_, err = os.Stat(filepath.Join(tempDir, "_manifest.json"))
		assert.NoError(t, err)
	})

	t.Run("read manifest", func(t *testing.T) {
		read, err := engine.ReadManifest(tempDir)
		require.NoError(t, err)
		assert.Equal(t, manifest.Version, read.Version)
		assert.Equal(t, manifest.JobID, read.JobID)
		assert.Equal(t, manifest.Features, read.Features)
		assert.Equal(t, manifest.RecordCount, read.RecordCount)
	})

	t.Run("read non-existing manifest", func(t *testing.T) {
		_, err := engine.ReadManifest("/nonexistent")
		assert.Error(t, err)
	})
}

func TestJobProgress(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	cfg.DefaultBatchSize = 2 // Small batch for testing
	engine, err := NewEngine(cfg, store, nil, nil)
	require.NoError(t, err)
	defer engine.Close()

	// Create test data
	tempDir := t.TempDir()
	testData := make([]map[string]interface{}, 10)
	for i := 0; i < 10; i++ {
		testData[i] = map[string]interface{}{
			"user_id": "user:" + string(rune('0'+i)),
			"clicks":  i * 10,
		}
	}
	data, _ := json.Marshal(testData)
	inputPath := filepath.Join(tempDir, "features.json")
	os.WriteFile(inputPath, data, 0644)

	spec := &JobSpec{
		ID:           "progress-job",
		Name:         "Progress Test",
		Source:       inputPath,
		SourceType:   SourceTypeJSON,
		EntityColumn: "user_id",
		FeatureColumns: map[string]string{
			"clicks": "click_count",
		},
		BatchSize: 2,
	}
	engine.CreateJob(spec)

	execution, err := engine.RunJob(context.Background(), "progress-job")
	require.NoError(t, err)
	assert.Equal(t, JobStatusCompleted, execution.Status)

	// Check final progress
	job, _ := engine.GetJob("progress-job")
	assert.Equal(t, int64(10), job.Progress.TotalRecords)
	assert.Equal(t, int64(10), job.Progress.ProcessedRecords)
	assert.Equal(t, float64(100), job.Progress.Percentage)
}

func TestSyncStrategy(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, store, nil, nil)
	require.NoError(t, err)
	defer engine.Close()

	ctx := context.Background()

	t.Run("merge strategy", func(t *testing.T) {
		// Pre-seed data
		store.Put("merge:1", map[string]*domain.FeatureValue{
			"existing_feature": {Value: "existing", Timestamp: time.Now().UnixNano()},
		})

		// Create job with merge strategy
		tempDir := t.TempDir()
		testData := []map[string]interface{}{
			{"user_id": "merge:1", "new_feature": "new"},
		}
		data, _ := json.Marshal(testData)
		inputPath := filepath.Join(tempDir, "features.json")
		os.WriteFile(inputPath, data, 0644)

		spec := &JobSpec{
			ID:           "merge-strategy-job",
			Name:         "Merge Strategy Test",
			Source:       inputPath,
			SourceType:   SourceTypeJSON,
			EntityColumn: "user_id",
			FeatureColumns: map[string]string{
				"new_feature": "new_feature",
			},
			Strategy: SyncStrategyMerge,
		}
		engine.CreateJob(spec)
		engine.RunJob(ctx, "merge-strategy-job")

		// Verify the new feature was synced
		features, err := store.Get("merge:1", []string{"new_feature", "existing_feature"})
		require.NoError(t, err)
		// The new feature should be there
		assert.NotNil(t, features["new_feature"])
		// With merge strategy, existing features should also be preserved
		assert.NotNil(t, features["existing_feature"])
	})
}
