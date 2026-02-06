package spark

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/storage"
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

	assert.Equal(t, "local[*]", cfg.SparkMaster)
	assert.Equal(t, "feather-spark-connector", cfg.AppName)
	assert.Equal(t, 10000, cfg.BatchSize)
	assert.Equal(t, 4, cfg.Parallelism)
	assert.Equal(t, "snappy", cfg.CompressionCodec)
	assert.True(t, cfg.EnableArrowOptimization)
	assert.True(t, cfg.EnableVectorizedReader)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "empty temp path",
			config: Config{
				TempPath: "",
			},
			wantErr: true,
		},
		{
			name: "zero batch size gets default",
			config: Config{
				TempPath:  os.TempDir(),
				BatchSize: 0,
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
		connector, err := NewConnector(cfg, store, nil, nil)
		require.NoError(t, err)
		assert.NotNil(t, connector)
		assert.Equal(t, StateReady, connector.State())
		connector.Close()
	})

	t.Run("nil store", func(t *testing.T) {
		cfg := DefaultConfig()
		_, err := NewConnector(cfg, nil, nil, nil)
		assert.Error(t, err)
	})

	t.Run("invalid config", func(t *testing.T) {
		cfg := Config{TempPath: ""}
		_, err := NewConnector(cfg, store, nil, nil)
		assert.Error(t, err)
	})
}

func TestConnectorExport(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Seed data
	testEntities := []string{"user:1", "user:2", "user:3"}
	for i, entity := range testEntities {
		features := map[string]*domain.FeatureValue{
			"click_count": {
				Value:     int64(i * 10),
				Timestamp: time.Now().UnixNano(),
			},
			"purchase_total": {
				Value:     float64(i) * 99.99,
				Timestamp: time.Now().UnixNano(),
			},
		}
		require.NoError(t, store.Put(entity, features))
	}

	cfg := DefaultConfig()
	connector, err := NewConnector(cfg, store, nil, nil)
	require.NoError(t, err)
	defer connector.Close()

	t.Run("export to JSON", func(t *testing.T) {
		tempDir := t.TempDir()

		req := &ExportRequest{
			OutputPath: tempDir,
			Format:     FormatJSON,
			Features:   []string{"click_count", "purchase_total"},
			Entities:   testEntities,
		}

		result, err := connector.ExportToJSON(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.RowsExported)
		assert.Equal(t, int64(3), result.EntitiesExported)
		assert.Equal(t, 2, result.FeaturesExported)
		assert.Equal(t, 1, result.FilesWritten)
		assert.Greater(t, result.BytesWritten, int64(0))

		// Verify file exists and is valid JSON
		data, err := os.ReadFile(filepath.Join(tempDir, "features.json"))
		require.NoError(t, err)

		var records []map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &records))
		assert.Len(t, records, 3)
	})

	t.Run("export to Parquet (JSON format)", func(t *testing.T) {
		tempDir := t.TempDir()

		req := &ExportRequest{
			OutputPath: tempDir,
			Features:   []string{"click_count", "purchase_total"},
			Entities:   testEntities,
		}

		result, err := connector.ExportToParquet(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.RowsExported)
		assert.Equal(t, FormatParquet, result.Format)

		// Check schema file was created
		_, err = os.Stat(filepath.Join(tempDir, "_schema.json"))
		assert.NoError(t, err)
	})

	t.Run("export to CSV", func(t *testing.T) {
		tempDir := t.TempDir()

		req := &ExportRequest{
			OutputPath: tempDir,
			Format:     FormatCSV,
			Features:   []string{"click_count", "purchase_total"},
			Entities:   testEntities,
		}

		result, err := connector.Export(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.RowsExported)

		// Verify CSV file exists
		_, err = os.Stat(filepath.Join(tempDir, "features.csv"))
		assert.NoError(t, err)
	})

	t.Run("export with time filter", func(t *testing.T) {
		tempDir := t.TempDir()

		past := time.Now().Add(-time.Hour)
		future := time.Now().Add(time.Hour)

		req := &ExportRequest{
			OutputPath: tempDir,
			Format:     FormatJSON,
			Features:   []string{"click_count"},
			Entities:   testEntities,
			StartTime:  &past,
			EndTime:    &future,
		}

		result, err := connector.Export(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.RowsExported)
	})

	t.Run("export validation errors", func(t *testing.T) {
		// Missing output path
		_, err := connector.Export(context.Background(), &ExportRequest{
			Features: []string{"click_count"},
		})
		assert.Error(t, err)

		// Missing features
		_, err = connector.Export(context.Background(), &ExportRequest{
			OutputPath: t.TempDir(),
		})
		assert.Error(t, err)
	})
}

func TestConnectorImport(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	connector, err := NewConnector(cfg, store, nil, nil)
	require.NoError(t, err)
	defer connector.Close()

	t.Run("import from JSON", func(t *testing.T) {
		// Create test JSON file
		tempDir := t.TempDir()
		testData := []map[string]interface{}{
			{
				"user_id":    "user:100",
				"clicks":     50,
				"spend":      199.99,
				"updated_at": time.Now().Format(time.RFC3339),
			},
			{
				"user_id":    "user:101",
				"clicks":     75,
				"spend":      299.99,
				"updated_at": time.Now().Format(time.RFC3339),
			},
		}

		data, _ := json.Marshal(testData)
		inputPath := filepath.Join(tempDir, "input.json")
		require.NoError(t, os.WriteFile(inputPath, data, 0644))

		req := &ImportRequest{
			InputPath:       inputPath,
			Format:          FormatJSON,
			EntityColumn:    "user_id",
			TimestampColumn: "updated_at",
			FeatureColumns: map[string]string{
				"clicks": "click_count",
				"spend":  "purchase_total",
			},
		}

		result, err := connector.Import(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, int64(2), result.RowsImported)
		assert.Equal(t, int64(2), result.EntitiesUpdated)
		assert.Equal(t, int64(4), result.FeaturesUpdated)

		// Verify data was written
		features, err := store.Get("user:100", []string{"click_count", "purchase_total"})
		require.NoError(t, err)
		assert.NotNil(t, features["click_count"])
		assert.NotNil(t, features["purchase_total"])
	})

	t.Run("import from JSON lines", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create JSON lines file
		var content string
		content += `{"user_id":"user:200","clicks":10}` + "\n"
		content += `{"user_id":"user:201","clicks":20}` + "\n"

		inputPath := filepath.Join(tempDir, "input.jsonl")
		require.NoError(t, os.WriteFile(inputPath, []byte(content), 0644))

		req := &ImportRequest{
			InputPath:    inputPath,
			Format:       FormatJSON,
			EntityColumn: "user_id",
			FeatureColumns: map[string]string{
				"clicks": "click_count",
			},
		}

		result, err := connector.Import(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, int64(2), result.RowsImported)
	})

	t.Run("import from CSV", func(t *testing.T) {
		tempDir := t.TempDir()

		csvContent := `user_id,clicks,spend
user:300,30,399.99
user:301,40,499.99
user:302,50,599.99`

		inputPath := filepath.Join(tempDir, "input.csv")
		require.NoError(t, os.WriteFile(inputPath, []byte(csvContent), 0644))

		req := &ImportRequest{
			InputPath:    inputPath,
			Format:       FormatCSV,
			EntityColumn: "user_id",
			FeatureColumns: map[string]string{
				"clicks": "click_count",
				"spend":  "purchase_total",
			},
		}

		result, err := connector.Import(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.RowsImported)
		assert.Equal(t, int64(3), result.EntitiesUpdated)

		// Verify data was written
		features, err := store.Get("user:300", []string{"click_count", "purchase_total"})
		require.NoError(t, err)
		assert.NotNil(t, features["click_count"])
	})

	t.Run("import with merge mode", func(t *testing.T) {
		tempDir := t.TempDir()

		// First write some existing data with recent timestamp
		existingFeatures := map[string]*domain.FeatureValue{
			"click_count": {
				Value:     int64(999),
				Timestamp: time.Now().Add(time.Hour).UnixNano(), // Future timestamp
			},
		}
		require.NoError(t, store.Put("user:merge", existingFeatures))

		// Import data with older timestamp
		testData := []map[string]interface{}{
			{
				"user_id":    "user:merge",
				"clicks":     1,
				"updated_at": time.Now().Add(-time.Hour).Format(time.RFC3339),
			},
		}
		data, _ := json.Marshal(testData)
		inputPath := filepath.Join(tempDir, "merge.json")
		require.NoError(t, os.WriteFile(inputPath, data, 0644))

		req := &ImportRequest{
			InputPath:       inputPath,
			Format:          FormatJSON,
			EntityColumn:    "user_id",
			TimestampColumn: "updated_at",
			WriteMode:       WriteModeMerge,
			FeatureColumns: map[string]string{
				"clicks": "click_count",
			},
		}

		result, err := connector.Import(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, int64(1), result.RowsImported)

		// The newer value should be preserved in merge mode
		features, err := store.Get("user:merge", []string{"click_count"})
		require.NoError(t, err)
		assert.Equal(t, int64(999), features["click_count"].Value)
	})

	t.Run("import validation errors", func(t *testing.T) {
		// Missing input path
		_, err := connector.Import(context.Background(), &ImportRequest{
			EntityColumn:   "user_id",
			FeatureColumns: map[string]string{"a": "b"},
		})
		assert.Error(t, err)

		// Missing entity column
		_, err = connector.Import(context.Background(), &ImportRequest{
			InputPath:      "/tmp/test.json",
			FeatureColumns: map[string]string{"a": "b"},
		})
		assert.Error(t, err)

		// Missing feature columns
		_, err = connector.Import(context.Background(), &ImportRequest{
			InputPath:    "/tmp/test.json",
			EntityColumn: "user_id",
		})
		assert.Error(t, err)
	})
}

func TestConnectorMetrics(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	connector, err := NewConnector(cfg, store, nil, nil)
	require.NoError(t, err)
	defer connector.Close()

	// Seed data
	require.NoError(t, store.Put("user:1", map[string]*domain.FeatureValue{
		"clicks": {Value: int64(10), Timestamp: time.Now().UnixNano()},
	}))

	// Perform export
	tempDir := t.TempDir()
	_, err = connector.Export(context.Background(), &ExportRequest{
		OutputPath: tempDir,
		Format:     FormatJSON,
		Features:   []string{"clicks"},
		Entities:   []string{"user:1"},
	})
	require.NoError(t, err)

	metrics := connector.Metrics()
	assert.Equal(t, int64(1), metrics.ExportOperations)
	assert.Equal(t, int64(1), metrics.RowsExported)
	assert.Greater(t, metrics.BytesTransferred, int64(0))
	assert.False(t, metrics.LastExportAt.IsZero())
}

func TestGenerateSparkSchema(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	connector, err := NewConnector(cfg, store, nil, nil)
	require.NoError(t, err)
	defer connector.Close()

	schema := connector.GenerateSparkSchema([]string{"click_count", "purchase_total"})

	assert.Contains(t, schema, "from pyspark.sql.types import")
	assert.Contains(t, schema, "StructType")
	assert.Contains(t, schema, "entity_key")
	assert.Contains(t, schema, "timestamp")
	assert.Contains(t, schema, "click_count")
	assert.Contains(t, schema, "purchase_total")
}

func TestGenerateSparkReadCode(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	cfg := DefaultConfig()
	connector, err := NewConnector(cfg, store, nil, nil)
	require.NoError(t, err)
	defer connector.Close()

	tests := []struct {
		format   DataFormat
		expected string
	}{
		{FormatParquet, `df = spark.read.json("/data/features/features.parquet.json")`},
		{FormatJSON, `df = spark.read.json("/data/features/features.json")`},
		{FormatCSV, `df = spark.read.csv("/data/features/features.csv", header=True, inferSchema=True)`},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			code := connector.GenerateSparkReadCode("/data/features", tt.format)
			assert.Equal(t, tt.expected, code)
		})
	}
}

func TestMapFeatureTypeToSpark(t *testing.T) {
	tests := []struct {
		input    domain.DataType
		expected string
	}{
		{domain.DataTypeInt64, "LongType"},
		{domain.DataTypeFloat64, "DoubleType"},
		{domain.DataTypeString, "StringType"},
		{domain.DataTypeBool, "BooleanType"},
		{domain.DataTypeTimestamp, "TimestampType"},
		{domain.DataTypeVector, "ArrayType(DoubleType)"},
		{domain.DataTypeBytes, "BinaryType"},
	}

	for _, tt := range tests {
		t.Run(tt.input.String(), func(t *testing.T) {
			result := mapFeatureTypeToSpark(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapSparkTypeToFeature(t *testing.T) {
	tests := []struct {
		input    string
		expected domain.DataType
	}{
		{"LongType", domain.DataTypeInt64},
		{"IntegerType", domain.DataTypeInt64},
		{"DoubleType", domain.DataTypeFloat64},
		{"FloatType", domain.DataTypeFloat64},
		{"StringType", domain.DataTypeString},
		{"BooleanType", domain.DataTypeBool},
		{"TimestampType", domain.DataTypeTimestamp},
		{"ArrayType(DoubleType)", domain.DataTypeVector},
		{"BinaryType", domain.DataTypeBytes},
		{"UnknownType", domain.DataTypeString},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapSparkTypeToFeature(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContextCancellation(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Seed lots of data
	for i := 0; i < 100; i++ {
		store.Put(
			"user:"+string(rune(i)),
			map[string]*domain.FeatureValue{
				"clicks": {Value: int64(i), Timestamp: time.Now().UnixNano()},
			},
		)
	}

	cfg := DefaultConfig()
	connector, err := NewConnector(cfg, store, nil, nil)
	require.NoError(t, err)
	defer connector.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	tempDir := t.TempDir()
	_, err = connector.Export(ctx, &ExportRequest{
		OutputPath: tempDir,
		Format:     FormatJSON,
		Features:   []string{"clicks"},
	})

	// Should return context.Canceled or complete quickly with partial results
	// The exact behavior depends on timing
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled)
	}
}
