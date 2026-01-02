package ml

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSnapshotStore(t *testing.T) {
	store := NewSnapshotStore()
	assert.NotNil(t, store)
	assert.Empty(t, store.ListSnapshots())
}

func TestSnapshotStore_CreateSnapshot(t *testing.T) {
	store := NewSnapshotStore()

	snapshot := &TrainingSnapshot{
		ID:           "snap-1",
		ModelID:      "model-1",
		ModelVersion: "v1.0",
		Description:  "Test snapshot",
		Features: map[string]*FeatureSnapshot{
			"feature_a": {Name: "feature_a", Count: 100, Mean: 5.0},
		},
	}

	err := store.CreateSnapshot(snapshot)
	require.NoError(t, err)
	assert.NotZero(t, snapshot.CreatedAt)
	assert.NotZero(t, snapshot.UpdatedAt)
}

func TestSnapshotStore_CreateSnapshot_Duplicate(t *testing.T) {
	store := NewSnapshotStore()

	snapshot := &TrainingSnapshot{ID: "snap-1", ModelID: "model-1"}
	err := store.CreateSnapshot(snapshot)
	require.NoError(t, err)

	err = store.CreateSnapshot(&TrainingSnapshot{ID: "snap-1", ModelID: "model-1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestSnapshotStore_CreateSnapshot_MissingID(t *testing.T) {
	store := NewSnapshotStore()

	snapshot := &TrainingSnapshot{ModelID: "model-1"}
	err := store.CreateSnapshot(snapshot)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ID is required")
}

func TestSnapshotStore_CreateSnapshot_MissingModelID(t *testing.T) {
	store := NewSnapshotStore()

	snapshot := &TrainingSnapshot{ID: "snap-1"}
	err := store.CreateSnapshot(snapshot)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model ID is required")
}

func TestSnapshotStore_GetSnapshot(t *testing.T) {
	store := NewSnapshotStore()

	snapshot := &TrainingSnapshot{ID: "snap-1", ModelID: "model-1", ModelVersion: "v1.0"}
	err := store.CreateSnapshot(snapshot)
	require.NoError(t, err)

	retrieved, err := store.GetSnapshot("snap-1")
	require.NoError(t, err)
	assert.Equal(t, "snap-1", retrieved.ID)
	assert.Equal(t, "model-1", retrieved.ModelID)
}

func TestSnapshotStore_GetSnapshot_NotFound(t *testing.T) {
	store := NewSnapshotStore()

	_, err := store.GetSnapshot("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSnapshotStore_GetSnapshotForModel(t *testing.T) {
	store := NewSnapshotStore()

	snapshot := &TrainingSnapshot{ID: "snap-1", ModelID: "model-1", ModelVersion: "v1.0"}
	store.CreateSnapshot(snapshot)

	retrieved, err := store.GetSnapshotForModel("model-1", "v1.0")
	require.NoError(t, err)
	assert.Equal(t, "snap-1", retrieved.ID)
}

func TestSnapshotStore_GetSnapshotForModel_NotFound(t *testing.T) {
	store := NewSnapshotStore()

	_, err := store.GetSnapshotForModel("nonexistent", "v1.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSnapshotStore_DeleteSnapshot(t *testing.T) {
	store := NewSnapshotStore()

	snapshot := &TrainingSnapshot{ID: "snap-1", ModelID: "model-1", ModelVersion: "v1.0"}
	store.CreateSnapshot(snapshot)

	err := store.DeleteSnapshot("snap-1")
	require.NoError(t, err)

	_, err = store.GetSnapshot("snap-1")
	assert.Error(t, err)
}

func TestSnapshotStore_DeleteSnapshot_NotFound(t *testing.T) {
	store := NewSnapshotStore()

	err := store.DeleteSnapshot("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSnapshotStore_ListSnapshots(t *testing.T) {
	store := NewSnapshotStore()

	for i := 0; i < 3; i++ {
		snapshot := &TrainingSnapshot{
			ID:      "snap-" + string(rune('1'+i)),
			ModelID: "model-1",
		}
		store.CreateSnapshot(snapshot)
	}

	snapshots := store.ListSnapshots()
	assert.Len(t, snapshots, 3)
}

func TestSnapshotStore_ListSnapshotsForModel(t *testing.T) {
	store := NewSnapshotStore()

	// Create snapshots for different models
	store.CreateSnapshot(&TrainingSnapshot{ID: "snap-1", ModelID: "model-1", ModelVersion: "v1"})
	store.CreateSnapshot(&TrainingSnapshot{ID: "snap-2", ModelID: "model-1", ModelVersion: "v2"})
	store.CreateSnapshot(&TrainingSnapshot{ID: "snap-3", ModelID: "model-2", ModelVersion: "v1"})

	snapshots := store.ListSnapshotsForModel("model-1")
	assert.Len(t, snapshots, 2)

	snapshots = store.ListSnapshotsForModel("model-2")
	assert.Len(t, snapshots, 1)
}

func TestSnapshotBuilder_Basic(t *testing.T) {
	builder := NewSnapshotBuilder("model-1", "v1.0", "Test snapshot")

	// Add numeric samples
	builder.AddSample("feature_a", 1.0)
	builder.AddSample("feature_a", 2.0)
	builder.AddSample("feature_a", 3.0)

	snapshot := builder.Build()

	assert.Equal(t, "model-1", snapshot.ModelID)
	assert.Equal(t, "v1.0", snapshot.ModelVersion)
	assert.NotEmpty(t, snapshot.ID)
	assert.Contains(t, snapshot.Features, "feature_a")
}

func TestSnapshotBuilder_NumericStats(t *testing.T) {
	builder := NewSnapshotBuilder("model-1", "v1.0", "")

	// Add known numeric values
	values := []interface{}{1.0, 2.0, 3.0, 4.0, 5.0}
	builder.AddSamples("feature_a", values)

	snapshot := builder.Build()
	fs := snapshot.Features["feature_a"]

	assert.Equal(t, DistTypeNumeric, fs.Type)
	assert.Equal(t, int64(5), fs.Count)
	assert.Equal(t, 1.0, fs.Min)
	assert.Equal(t, 5.0, fs.Max)
	assert.Equal(t, 3.0, fs.Mean) // (1+2+3+4+5)/5 = 3
}

func TestSnapshotBuilder_CategoricalStats(t *testing.T) {
	builder := NewSnapshotBuilder("model-1", "v1.0", "")

	// Add categorical values
	builder.AddSample("category", "A")
	builder.AddSample("category", "B")
	builder.AddSample("category", "A")
	builder.AddSample("category", "C")
	builder.AddSample("category", "A")

	snapshot := builder.Build()
	fs := snapshot.Features["category"]

	assert.Equal(t, DistTypeCategorical, fs.Type)
	assert.Equal(t, int64(5), fs.Count)
	assert.Equal(t, 3, fs.Cardinality)
	assert.NotNil(t, fs.Categories)
	assert.Equal(t, int64(3), fs.Categories["A"])
	assert.Equal(t, int64(1), fs.Categories["B"])
	assert.Equal(t, int64(1), fs.Categories["C"])
}

func TestSnapshotBuilder_WithNulls(t *testing.T) {
	builder := NewSnapshotBuilder("model-1", "v1.0", "")

	// Add values with some nulls
	builder.AddSample("feature_a", 1.0)
	builder.AddSample("feature_a", nil)
	builder.AddSample("feature_a", 3.0)
	builder.AddSample("feature_a", nil)
	builder.AddSample("feature_a", 5.0)

	snapshot := builder.Build()
	fs := snapshot.Features["feature_a"]

	assert.Equal(t, int64(5), fs.Count)
	assert.Equal(t, int64(2), fs.NullCount)
	assert.InDelta(t, 0.4, fs.NullRate, 0.001) // 2/5 = 0.4
}

func TestSnapshotBuilder_TrainingMetadata(t *testing.T) {
	builder := NewSnapshotBuilder("model-1", "v1.0", "")

	now := time.Now()
	builder.SetTrainingMetadata(TrainingMetadata{
		DatasetName:    "training_data",
		DatasetVersion: "2024-01-15",
		SampleCount:    10000,
		TrainingJobID:  "job-123",
		StartDate:      &now,
	})

	builder.AddSample("feature_a", 1.0)
	snapshot := builder.Build()

	assert.Equal(t, "training_data", snapshot.TrainingMetadata.DatasetName)
	assert.Equal(t, "2024-01-15", snapshot.TrainingMetadata.DatasetVersion)
	assert.Equal(t, int64(10000), snapshot.TrainingMetadata.SampleCount)
	assert.Equal(t, "job-123", snapshot.TrainingMetadata.TrainingJobID)
}

func TestSnapshotBuilder_Histogram(t *testing.T) {
	builder := NewSnapshotBuilder("model-1", "v1.0", "")

	// Add enough values to generate histogram
	values := make([]interface{}, 100)
	for i := 0; i < 100; i++ {
		values[i] = float64(i)
	}
	builder.AddSamples("feature_a", values)

	snapshot := builder.Build()
	fs := snapshot.Features["feature_a"]

	assert.NotEmpty(t, fs.HistogramBuckets)
	assert.NotEmpty(t, fs.HistogramCounts)
}

func TestSnapshotBuilder_Percentiles(t *testing.T) {
	builder := NewSnapshotBuilder("model-1", "v1.0", "")

	// Add ordered values for predictable percentiles
	values := make([]interface{}, 100)
	for i := 0; i < 100; i++ {
		values[i] = float64(i + 1) // 1 to 100
	}
	builder.AddSamples("feature_a", values)

	snapshot := builder.Build()
	fs := snapshot.Features["feature_a"]

	// Check percentile values (approximate)
	assert.InDelta(t, 5.0, fs.P5, 2.0)
	assert.InDelta(t, 25.0, fs.P25, 2.0)
	assert.InDelta(t, 50.0, fs.Median, 2.0)
	assert.InDelta(t, 75.0, fs.P75, 2.0)
	assert.InDelta(t, 95.0, fs.P95, 2.0)
	assert.InDelta(t, 99.0, fs.P99, 2.0)
}

func TestSnapshotBuilder_MultipleFeatures(t *testing.T) {
	builder := NewSnapshotBuilder("model-1", "v1.0", "")

	builder.AddSamples("numeric_feature", []interface{}{1.0, 2.0, 3.0})
	builder.AddSamples("categorical_feature", []interface{}{"A", "B", "C"})
	builder.AddSamples("another_numeric", []interface{}{10.0, 20.0, 30.0})

	snapshot := builder.Build()

	assert.Len(t, snapshot.Features, 3)
	assert.Contains(t, snapshot.Features, "numeric_feature")
	assert.Contains(t, snapshot.Features, "categorical_feature")
	assert.Contains(t, snapshot.Features, "another_numeric")
}

func TestFeatureSnapshot_DistributionTypes(t *testing.T) {
	tests := []struct {
		name         string
		values       []interface{}
		expectedType DistributionType
	}{
		{
			name:         "float64 values",
			values:       []interface{}{1.0, 2.0, 3.0},
			expectedType: DistTypeNumeric,
		},
		{
			name:         "int values",
			values:       []interface{}{1, 2, 3},
			expectedType: DistTypeNumeric,
		},
		{
			name:         "string values",
			values:       []interface{}{"a", "b", "c"},
			expectedType: DistTypeCategorical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewSnapshotBuilder("model-1", "v1.0", "")
			builder.AddSamples("feature", tt.values)
			snapshot := builder.Build()

			assert.Equal(t, tt.expectedType, snapshot.Features["feature"].Type)
		})
	}
}
