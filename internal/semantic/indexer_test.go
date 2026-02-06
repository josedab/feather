package semantic

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupIndexerTest(t *testing.T) *EnhancedIndexer {
	t.Helper()
	search := NewSearch(NewLocalEmbedder(384), nil)
	return NewEnhancedIndexer(search)
}

func TestNewEnhancedIndexer(t *testing.T) {
	indexer := setupIndexerTest(t)
	assert.NotNil(t, indexer)
	assert.NotNil(t, indexer.Search())
}

func TestEnhancedIndexer_IndexFeatureWithMetadata(t *testing.T) {
	indexer := setupIndexerTest(t)
	ctx := context.Background()

	meta := &FeatureMetadata{
		FeatureID:    "user_purchase_count",
		Name:         "User Purchase Count",
		Description:  "Total purchases made by user",
		Category:     "user_behavior",
		Tags:         []string{"user", "purchases"},
		Owner:        "data-team",
		QualityScore: 0.95,
	}

	err := indexer.IndexFeatureWithMetadata(ctx, meta)
	require.NoError(t, err)

	// Verify metadata is stored
	retrieved, err := indexer.GetMetadata("user_purchase_count")
	require.NoError(t, err)
	assert.Equal(t, "User Purchase Count", retrieved.Name)
	assert.Equal(t, "user_behavior", retrieved.Category)
}

func TestEnhancedIndexer_IndexFeatureWithMetadata_MissingID(t *testing.T) {
	indexer := setupIndexerTest(t)
	ctx := context.Background()

	meta := &FeatureMetadata{
		Name: "Test Feature",
	}

	err := indexer.IndexFeatureWithMetadata(ctx, meta)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "feature ID is required")
}

func TestEnhancedIndexer_SetStatistics(t *testing.T) {
	indexer := setupIndexerTest(t)
	ctx := context.Background()

	// First index a feature
	meta := &FeatureMetadata{
		FeatureID: "test_feature",
		Name:      "Test Feature",
	}
	err := indexer.IndexFeatureWithMetadata(ctx, meta)
	require.NoError(t, err)

	// Set statistics
	stats := &FeatureStatistics{
		Min:            0,
		Max:            100,
		Mean:           50,
		StdDev:         15,
		NullPercentage: 2.5,
		SampleSize:     10000,
	}
	err = indexer.SetStatistics("test_feature", stats)
	require.NoError(t, err)

	// Verify statistics are stored
	retrieved, err := indexer.GetStatistics("test_feature")
	require.NoError(t, err)
	assert.Equal(t, 50.0, retrieved.Mean)
	assert.Equal(t, 15.0, retrieved.StdDev)
}

func TestEnhancedIndexer_SetStatistics_FeatureNotFound(t *testing.T) {
	indexer := setupIndexerTest(t)

	stats := &FeatureStatistics{Mean: 50}
	err := indexer.SetStatistics("nonexistent", stats)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "feature not found")
}

func TestEnhancedIndexer_SetLineage(t *testing.T) {
	indexer := setupIndexerTest(t)
	ctx := context.Background()

	// First index a feature
	meta := &FeatureMetadata{
		FeatureID: "derived_feature",
		Name:      "Derived Feature",
	}
	err := indexer.IndexFeatureWithMetadata(ctx, meta)
	require.NoError(t, err)

	// Set lineage
	lineage := &FeatureLineage{
		SourceFeatures:     []string{"raw_feature_a", "raw_feature_b"},
		TransformationType: "aggregation",
		DependentModels:    []string{"model_v1"},
	}
	err = indexer.SetLineage("derived_feature", lineage)
	require.NoError(t, err)

	// Verify lineage is stored
	retrieved, err := indexer.GetLineage("derived_feature")
	require.NoError(t, err)
	assert.Equal(t, []string{"raw_feature_a", "raw_feature_b"}, retrieved.SourceFeatures)
	assert.Equal(t, "aggregation", retrieved.TransformationType)
}

func TestEnhancedIndexer_SetLineage_FeatureNotFound(t *testing.T) {
	indexer := setupIndexerTest(t)

	lineage := &FeatureLineage{SourceFeatures: []string{"a"}}
	err := indexer.SetLineage("nonexistent", lineage)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "feature not found")
}

func TestEnhancedIndexer_SetUsage(t *testing.T) {
	indexer := setupIndexerTest(t)
	ctx := context.Background()

	// First index a feature
	meta := &FeatureMetadata{
		FeatureID: "popular_feature",
		Name:      "Popular Feature",
	}
	err := indexer.IndexFeatureWithMetadata(ctx, meta)
	require.NoError(t, err)

	// Set usage
	usage := &FeatureUsage{
		TotalReads:      100000,
		UniqueReaders:   500,
		ModelsUsing:     []string{"fraud_model", "rec_model"},
		ModelCount:      2,
		PopularityScore: 0.85,
	}
	err = indexer.SetUsage("popular_feature", usage)
	require.NoError(t, err)

	// Verify usage is stored
	retrieved, err := indexer.GetUsage("popular_feature")
	require.NoError(t, err)
	assert.Equal(t, int64(100000), retrieved.TotalReads)
	assert.Equal(t, 2, retrieved.ModelCount)
}

func TestEnhancedIndexer_SetUsage_FeatureNotFound(t *testing.T) {
	indexer := setupIndexerTest(t)

	usage := &FeatureUsage{TotalReads: 100}
	err := indexer.SetUsage("nonexistent", usage)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "feature not found")
}

func TestEnhancedIndexer_RecordRead(t *testing.T) {
	indexer := setupIndexerTest(t)
	ctx := context.Background()

	// Index a feature
	meta := &FeatureMetadata{
		FeatureID: "test_feature",
		Name:      "Test Feature",
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	// Record reads
	indexer.RecordRead("test_feature")
	indexer.RecordRead("test_feature")
	indexer.RecordRead("test_feature")

	// Check usage was recorded
	usage, err := indexer.GetUsage("test_feature")
	require.NoError(t, err)
	assert.Equal(t, int64(3), usage.TotalReads)
	assert.False(t, usage.LastReadAt.IsZero())
}

func TestEnhancedIndexer_GetEnrichedFeature(t *testing.T) {
	indexer := setupIndexerTest(t)
	ctx := context.Background()

	// Index feature with all metadata
	meta := &FeatureMetadata{
		FeatureID:   "enriched_feature",
		Name:        "Enriched Feature",
		Description: "A fully enriched feature",
		Category:    "test",
		Owner:       "data-team",
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	stats := &FeatureStatistics{Mean: 50, Max: 100}
	indexer.SetStatistics("enriched_feature", stats)

	lineage := &FeatureLineage{SourceFeatures: []string{"raw"}}
	indexer.SetLineage("enriched_feature", lineage)

	usage := &FeatureUsage{TotalReads: 1000}
	indexer.SetUsage("enriched_feature", usage)

	// Get enriched feature
	enriched, err := indexer.GetEnrichedFeature("enriched_feature")
	require.NoError(t, err)
	assert.NotNil(t, enriched.Metadata)
	assert.NotNil(t, enriched.Statistics)
	assert.NotNil(t, enriched.Lineage)
	assert.NotNil(t, enriched.Usage)
	assert.Equal(t, "Enriched Feature", enriched.Metadata.Name)
	assert.Equal(t, 50.0, enriched.Statistics.Mean)
}

func TestEnhancedIndexer_GetEnrichedFeature_NotFound(t *testing.T) {
	indexer := setupIndexerTest(t)

	_, err := indexer.GetEnrichedFeature("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "feature not found")
}

func TestEnhancedIndexer_SearchWithMetadata(t *testing.T) {
	indexer := setupIndexerTest(t)
	ctx := context.Background()

	// Index several features
	features := []*FeatureMetadata{
		{
			FeatureID:   "user_clicks",
			Name:        "User Clicks",
			Description: "Number of clicks by user",
			Category:    "engagement",
			Tags:        []string{"user", "clicks"},
		},
		{
			FeatureID:   "user_purchases",
			Name:        "User Purchases",
			Description: "Total purchases by user",
			Category:    "revenue",
			Tags:        []string{"user", "purchases"},
		},
		{
			FeatureID:   "product_views",
			Name:        "Product Views",
			Description: "Product page views",
			Category:    "engagement",
			Tags:        []string{"product", "views"},
		},
	}

	for _, f := range features {
		indexer.IndexFeatureWithMetadata(ctx, f)
	}

	// Search
	opts := SearchOptions{Limit: 10, MinScore: 0}
	results, err := indexer.Search().Search(ctx, "user engagement clicks", opts)
	require.NoError(t, err)
	assert.NotEmpty(t, results)

	// First result should be user-related
	found := false
	for _, r := range results {
		if r.Feature.ID == "user_clicks" || r.Feature.ID == "user_purchases" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected to find user-related features in results")
}

func TestEnhancedIndexer_ListMetadata(t *testing.T) {
	indexer := setupIndexerTest(t)
	ctx := context.Background()

	// Index features
	for i := 0; i < 5; i++ {
		meta := &FeatureMetadata{
			FeatureID: "feature_" + string(rune('a'+i)),
			Name:      "Feature " + string(rune('A'+i)),
		}
		indexer.IndexFeatureWithMetadata(ctx, meta)
	}

	features := indexer.ListMetadata()
	assert.Len(t, features, 5)
}

func TestFeatureMetadata_Fields(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	meta := &FeatureMetadata{
		FeatureID:     "test",
		Name:          "Test Feature",
		Description:   "Description",
		DataType:      "float64",
		ValueType:     "numeric",
		EntityType:    "user",
		Nullable:      true,
		Category:      "test",
		Subcategory:   "subtest",
		Tags:          []string{"tag1", "tag2"},
		Labels:        []string{"label1"},
		Domain:        "user",
		BusinessUnit:  "marketing",
		UseCase:       []string{"fraud_detection"},
		Owner:         "owner",
		Team:          "data",
		Maintainers:   []string{"dev1"},
		QualityScore:  0.9,
		DataQuality:   "high",
		Completeness:  0.98,
		Freshness:     "hourly",
		CreatedAt:     now,
		UpdatedAt:     now,
		Source:        "kafka",
		SourceTable:   "events",
		Documentation: "docs",
		Examples:      []string{"example1"},
		CustomFields:  map[string]string{"custom": "value"},
	}

	assert.Equal(t, "test", meta.FeatureID)
	assert.Equal(t, "Test Feature", meta.Name)
	assert.Equal(t, "numeric", meta.ValueType)
	assert.Equal(t, "user", meta.Domain)
	assert.True(t, meta.Nullable)
	assert.InDelta(t, float32(0.9), meta.QualityScore, 0.0001)
	assert.Equal(t, "Description", meta.Description)
	assert.Equal(t, "float64", meta.DataType)
	assert.Equal(t, "user", meta.EntityType)
	assert.Equal(t, "test", meta.Category)
	assert.Equal(t, "subtest", meta.Subcategory)
	assert.Equal(t, []string{"tag1", "tag2"}, meta.Tags)
	assert.Equal(t, []string{"label1"}, meta.Labels)
	assert.Equal(t, "marketing", meta.BusinessUnit)
	assert.Equal(t, []string{"fraud_detection"}, meta.UseCase)
	assert.Equal(t, "owner", meta.Owner)
	assert.Equal(t, "data", meta.Team)
	assert.Equal(t, []string{"dev1"}, meta.Maintainers)
	assert.Equal(t, "high", meta.DataQuality)
	assert.InDelta(t, float32(0.98), meta.Completeness, 0.0001)
	assert.Equal(t, "hourly", meta.Freshness)
	assert.Equal(t, now, meta.CreatedAt)
	assert.Equal(t, now, meta.UpdatedAt)
	assert.Equal(t, "kafka", meta.Source)
	assert.Equal(t, "events", meta.SourceTable)
	assert.Equal(t, "docs", meta.Documentation)
	assert.Equal(t, []string{"example1"}, meta.Examples)
	assert.Equal(t, map[string]string{"custom": "value"}, meta.CustomFields)
}

func TestFeatureStatistics_Fields(t *testing.T) {
	stats := &FeatureStatistics{
		FeatureID:      "test",
		Min:            0,
		Max:            100,
		Mean:           50,
		Median:         48,
		StdDev:         15,
		Percentiles:    map[string]float64{"p50": 48, "p99": 98},
		UniqueCount:    500,
		TopValues:      []TopValue{{Value: "A", Count: 100, Pct: 20}},
		ValueCounts:    map[string]int64{"A": 100, "B": 80},
		Distribution:   "normal",
		Skewness:       0.1,
		Kurtosis:       2.9,
		NullCount:      50,
		NullPercentage: 5,
		OutlierCount:   10,
		Volatility:     0.2,
		Trend:          "stable",
		SampleSize:     1000,
	}

	assert.Equal(t, 50.0, stats.Mean)
	assert.Equal(t, "normal", stats.Distribution)
	assert.Equal(t, int64(500), stats.UniqueCount)
	assert.Equal(t, "test", stats.FeatureID)
	assert.Equal(t, 0.0, stats.Min)
	assert.Equal(t, 100.0, stats.Max)
	assert.Equal(t, 48.0, stats.Median)
	assert.Equal(t, 15.0, stats.StdDev)
	assert.Equal(t, map[string]float64{"p50": 48, "p99": 98}, stats.Percentiles)
	assert.Equal(t, []TopValue{{Value: "A", Count: 100, Pct: 20}}, stats.TopValues)
	assert.Equal(t, map[string]int64{"A": 100, "B": 80}, stats.ValueCounts)
	assert.Equal(t, 0.1, stats.Skewness)
	assert.Equal(t, 2.9, stats.Kurtosis)
	assert.Equal(t, int64(50), stats.NullCount)
	assert.Equal(t, 5.0, stats.NullPercentage)
	assert.Equal(t, int64(10), stats.OutlierCount)
	assert.Equal(t, 0.2, stats.Volatility)
	assert.Equal(t, "stable", stats.Trend)
	assert.Equal(t, int64(1000), stats.SampleSize)
}

func TestFeatureLineage_Fields(t *testing.T) {
	lineage := &FeatureLineage{
		FeatureID:          "test",
		SourceFeatures:     []string{"raw_a", "raw_b"},
		SourceTables:       []string{"events_table"},
		ExternalSources:    []string{"api"},
		DependentFeatures:  []string{"derived_1"},
		DependentModels:    []string{"model_v1"},
		TransformationType: "aggregation",
		TransformationCode: "SUM(amount) OVER ...",
		Version:            "v1.0",
		PipelineID:         "pipe-123",
		PipelineName:       "daily_agg",
	}

	assert.Equal(t, []string{"raw_a", "raw_b"}, lineage.SourceFeatures)
	assert.Equal(t, "aggregation", lineage.TransformationType)
	assert.Equal(t, "test", lineage.FeatureID)
	assert.Equal(t, []string{"events_table"}, lineage.SourceTables)
	assert.Equal(t, []string{"api"}, lineage.ExternalSources)
	assert.Equal(t, []string{"derived_1"}, lineage.DependentFeatures)
	assert.Equal(t, []string{"model_v1"}, lineage.DependentModels)
	assert.Equal(t, "SUM(amount) OVER ...", lineage.TransformationCode)
	assert.Equal(t, "v1.0", lineage.Version)
	assert.Equal(t, "pipe-123", lineage.PipelineID)
	assert.Equal(t, "daily_agg", lineage.PipelineName)
}

func TestFeatureUsage_Fields(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	usage := &FeatureUsage{
		FeatureID:       "test",
		TotalReads:      100000,
		ReadRate:        500,
		LastReadAt:      now,
		UniqueReaders:   1000,
		ModelsUsing:     []string{"model1", "model2"},
		ModelCount:      2,
		PopularityScore: 0.85,
		TrendScore:      0.1,
		ImportanceScore: 0.9,
		CriticalModels:  []string{"model1"},
		WindowStart:     now.Add(-24 * time.Hour),
		WindowEnd:       now,
	}

	assert.Equal(t, int64(100000), usage.TotalReads)
	assert.Equal(t, 0.85, usage.PopularityScore)
	assert.Equal(t, 2, usage.ModelCount)
	assert.Equal(t, "test", usage.FeatureID)
	assert.Equal(t, 500.0, usage.ReadRate)
	assert.Equal(t, now, usage.LastReadAt)
	assert.Equal(t, int64(1000), usage.UniqueReaders)
	assert.Equal(t, []string{"model1", "model2"}, usage.ModelsUsing)
	assert.Equal(t, 0.1, usage.TrendScore)
	assert.Equal(t, 0.9, usage.ImportanceScore)
	assert.Equal(t, []string{"model1"}, usage.CriticalModels)
	assert.Equal(t, now.Add(-24*time.Hour), usage.WindowStart)
	assert.Equal(t, now, usage.WindowEnd)
}
