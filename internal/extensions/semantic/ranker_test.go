package semantic

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRankerTest(t *testing.T) (*EnhancedIndexer, *HybridRanker) {
	t.Helper()
	search := NewSearch(NewLocalEmbedder(384), nil)
	indexer := NewEnhancedIndexer(search)
	ranker := NewHybridRanker(indexer, DefaultRankerConfig())
	return indexer, ranker
}

func TestDefaultRankerConfig(t *testing.T) {
	config := DefaultRankerConfig()

	// Check weights sum to approximately 1.0
	totalWeight := config.SemanticWeight + config.MetadataWeight +
		config.PopularityWeight + config.QualityWeight +
		config.FreshnessWeight + config.LineageWeight
	assert.InDelta(t, 1.0, totalWeight, 0.01)

	// Check default values
	assert.Equal(t, 0.40, config.SemanticWeight)
	assert.Equal(t, 0.25, config.MetadataWeight)
	assert.Equal(t, 1.5, config.ExactMatchBoost)
	assert.Equal(t, 0.3, config.MinSemanticScore)
}

func TestNewHybridRanker(t *testing.T) {
	_, ranker := setupRankerTest(t)
	assert.NotNil(t, ranker)
}

func TestHybridRanker_Rank_Basic(t *testing.T) {
	indexer, ranker := setupRankerTest(t)
	ctx := context.Background()

	// Index features with metadata
	features := []*FeatureMetadata{
		{
			FeatureID:    "user_purchase_count",
			Name:         "User Purchase Count",
			Description:  "Total number of purchases made by user",
			Category:     "revenue",
			Tags:         []string{"user", "purchases", "count"},
			QualityScore: 0.9,
			DataQuality:  "high",
			Freshness:    "daily",
		},
		{
			FeatureID:    "user_total_spend",
			Name:         "User Total Spend",
			Description:  "Total amount spent by user",
			Category:     "revenue",
			Tags:         []string{"user", "spend", "money"},
			QualityScore: 0.85,
			DataQuality:  "high",
			Freshness:    "hourly",
		},
		{
			FeatureID:    "product_views",
			Name:         "Product Views",
			Description:  "Number of product page views",
			Category:     "engagement",
			Tags:         []string{"product", "views"},
			QualityScore: 0.7,
			DataQuality:  "medium",
			Freshness:    "real-time",
		},
	}

	for _, f := range features {
		indexer.IndexFeatureWithMetadata(ctx, f)
	}

	// Rank features for a query
	req := RankRequest{
		Query: "user purchasing spending behavior",
		Limit: 10,
	}

	results, err := ranker.Rank(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, results)

	// Verify results have scores
	for _, r := range results {
		assert.Greater(t, r.TotalScore, 0.0)
		assert.NotNil(t, r.Feature)
	}
}

func TestHybridRanker_Rank_WithFilters(t *testing.T) {
	indexer, ranker := setupRankerTest(t)
	ctx := context.Background()

	// Index features in different categories
	features := []*FeatureMetadata{
		{
			FeatureID:  "revenue_feature",
			Name:       "Revenue Feature",
			Category:   "revenue",
			EntityType: "user",
			Domain:     "finance",
		},
		{
			FeatureID:  "engagement_feature",
			Name:       "Engagement Feature",
			Category:   "engagement",
			EntityType: "user",
			Domain:     "marketing",
		},
	}

	for _, f := range features {
		indexer.IndexFeatureWithMetadata(ctx, f)
	}

	// Rank with category filter
	req := RankRequest{
		Query:      "feature",
		Categories: []string{"revenue"},
		Limit:      10,
	}

	results, err := ranker.Rank(ctx, req)
	require.NoError(t, err)

	// All results should be in revenue category
	for _, r := range results {
		assert.Equal(t, "revenue", r.Feature.Metadata.Category)
	}
}

func TestHybridRanker_Rank_ExcludeFeatures(t *testing.T) {
	indexer, ranker := setupRankerTest(t)
	ctx := context.Background()

	// Index features
	features := []*FeatureMetadata{
		{FeatureID: "f1", Name: "Feature One"},
		{FeatureID: "f2", Name: "Feature Two"},
		{FeatureID: "f3", Name: "Feature Three"},
	}

	for _, f := range features {
		indexer.IndexFeatureWithMetadata(ctx, f)
	}

	// Rank excluding f2
	req := RankRequest{
		Query:           "feature",
		ExcludeFeatures: []string{"f2"},
		Limit:           10,
	}

	results, err := ranker.Rank(ctx, req)
	require.NoError(t, err)

	// f2 should not be in results
	for _, r := range results {
		assert.NotEqual(t, "f2", r.Feature.Metadata.FeatureID)
	}
}

func TestHybridRanker_ScoreBreakdown(t *testing.T) {
	indexer, ranker := setupRankerTest(t)
	ctx := context.Background()

	// Index a feature with full metadata
	meta := &FeatureMetadata{
		FeatureID:    "test_feature",
		Name:         "Test Feature",
		Description:  "A test feature for scoring",
		Category:     "test",
		QualityScore: 0.9,
		DataQuality:  "high",
		Freshness:    "real-time",
		Owner:        "team",
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	usage := &FeatureUsage{
		TotalReads:      50000,
		ModelCount:      3,
		PopularityScore: 0.8,
	}
	indexer.SetUsage("test_feature", usage)

	// Rank
	req := RankRequest{
		Query: "test feature",
		Limit: 1,
	}

	results, err := ranker.Rank(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, results)

	result := results[0]

	// Verify score components exist
	assert.Greater(t, result.SemanticScore, 0.0)
	assert.GreaterOrEqual(t, result.MetadataScore, 0.0)
	assert.GreaterOrEqual(t, result.PopularityScore, 0.0)
	assert.GreaterOrEqual(t, result.QualityScore, 0.0)
	assert.GreaterOrEqual(t, result.FreshnessScore, 0.0)
}

func TestHybridRanker_BoostsAndPenalties(t *testing.T) {
	indexer, ranker := setupRankerTest(t)
	ctx := context.Background()

	now := time.Now()

	// High quality, recent feature
	goodFeature := &FeatureMetadata{
		FeatureID:    "good_feature",
		Name:         "Good Feature",
		Description:  "A high quality feature",
		QualityScore: 0.95,
		DataQuality:  "high",
		Tags:         []string{"important"},
		UpdatedAt:    now,
	}
	indexer.IndexFeatureWithMetadata(ctx, goodFeature)

	// Low quality, stale feature
	badFeature := &FeatureMetadata{
		FeatureID:    "bad_feature",
		Name:         "Bad Feature",
		Description:  "A low quality feature",
		QualityScore: 0.3,
		DataQuality:  "low",
		Tags:         []string{"deprecated"},
		UpdatedAt:    now.Add(-60 * 24 * time.Hour), // 60 days old
	}
	indexer.IndexFeatureWithMetadata(ctx, badFeature)

	// Rank both
	req := RankRequest{
		Query: "feature",
		Limit: 10,
	}

	results, err := ranker.Rank(ctx, req)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// Find both results
	var goodResult, badResult *RankedResult
	for i := range results {
		if results[i].Feature.Metadata.FeatureID == "good_feature" {
			goodResult = &results[i]
		} else if results[i].Feature.Metadata.FeatureID == "bad_feature" {
			badResult = &results[i]
		}
	}

	require.NotNil(t, goodResult)
	require.NotNil(t, badResult)

	// Good feature should score higher
	assert.Greater(t, goodResult.TotalScore, badResult.TotalScore)

	// Bad feature should have penalties
	assert.NotEmpty(t, badResult.Penalties)
}

func TestHybridRanker_SuggestSimilar(t *testing.T) {
	indexer, ranker := setupRankerTest(t)
	ctx := context.Background()

	// Index related features
	features := []*FeatureMetadata{
		{
			FeatureID:   "user_clicks",
			Name:        "User Clicks",
			Description: "Number of user clicks",
			Category:    "engagement",
			EntityType:  "user",
		},
		{
			FeatureID:   "user_sessions",
			Name:        "User Sessions",
			Description: "Number of user sessions",
			Category:    "engagement",
			EntityType:  "user",
		},
		{
			FeatureID:   "product_price",
			Name:        "Product Price",
			Description: "Product selling price",
			Category:    "product",
			EntityType:  "product",
		},
	}

	for _, f := range features {
		indexer.IndexFeatureWithMetadata(ctx, f)
	}

	// Get suggestions for user_clicks
	suggestions, err := ranker.SuggestSimilar(ctx, "user_clicks", 2)
	require.NoError(t, err)

	// Should not include the source feature itself
	for _, s := range suggestions {
		assert.NotEqual(t, "user_clicks", s.Feature.Metadata.FeatureID)
	}
}

func TestHybridRanker_SuggestSimilar_NotFound(t *testing.T) {
	_, ranker := setupRankerTest(t)
	ctx := context.Background()

	_, err := ranker.SuggestSimilar(ctx, "nonexistent", 5)
	assert.Error(t, err)
}

func TestHybridRanker_RecommendForModel(t *testing.T) {
	indexer, ranker := setupRankerTest(t)
	ctx := context.Background()

	// Index features with use cases
	features := []*FeatureMetadata{
		{
			FeatureID: "fraud_signal_1",
			Name:      "Fraud Signal 1",
			Category:  "fraud",
			UseCase:   []string{"fraud_detection"},
		},
		{
			FeatureID: "fraud_signal_2",
			Name:      "Fraud Signal 2",
			Category:  "fraud",
			UseCase:   []string{"fraud_detection"},
		},
		{
			FeatureID: "rec_feature",
			Name:      "Recommendation Feature",
			Category:  "recommendations",
			UseCase:   []string{"personalization"},
		},
	}

	for _, f := range features {
		indexer.IndexFeatureWithMetadata(ctx, f)
	}

	// Recommend for a fraud model that already uses fraud_signal_1
	recommendations, err := ranker.RecommendForModel(ctx,
		[]string{"fraud_signal_1"}, "fraud_detection", 5)
	require.NoError(t, err)

	// Should not include fraud_signal_1 (already used)
	for _, r := range recommendations {
		assert.NotEqual(t, "fraud_signal_1", r.Feature.Metadata.FeatureID)
	}
}

func TestRankRequest_Fields(t *testing.T) {
	req := RankRequest{
		Query:           "test query",
		Categories:      []string{"cat1", "cat2"},
		Tags:            []string{"tag1"},
		EntityTypes:     []string{"user"},
		Domains:         []string{"finance"},
		UseCases:        []string{"fraud_detection"},
		Owner:           "team-a",
		Team:            "data",
		MinQuality:      0.5,
		OnlyFresh:       true,
		ExcludeFeatures: []string{"excluded"},
		Limit:           20,
	}

	assert.Equal(t, "test query", req.Query)
	assert.Equal(t, 2, len(req.Categories))
	assert.Equal(t, []string{"tag1"}, req.Tags)
	assert.Equal(t, []string{"user"}, req.EntityTypes)
	assert.Equal(t, []string{"finance"}, req.Domains)
	assert.Equal(t, []string{"fraud_detection"}, req.UseCases)
	assert.True(t, req.OnlyFresh)
	assert.Equal(t, 20, req.Limit)
	assert.Equal(t, "team-a", req.Owner)
	assert.Equal(t, "data", req.Team)
	assert.Equal(t, float32(0.5), req.MinQuality)
	assert.Equal(t, []string{"excluded"}, req.ExcludeFeatures)
}

func TestRankedResult_Fields(t *testing.T) {
	result := RankedResult{
		TotalScore:      0.85,
		SemanticScore:   0.9,
		MetadataScore:   0.8,
		PopularityScore: 0.7,
		QualityScore:    0.95,
		FreshnessScore:  1.0,
		LineageScore:    0.5,
		BoostFactors:    []string{"exact_name_match", "category_match"},
		Penalties:       []string{"stale"},
	}

	assert.Equal(t, 0.85, result.TotalScore)
	assert.Equal(t, 0.9, result.SemanticScore)
	assert.Equal(t, 0.8, result.MetadataScore)
	assert.Equal(t, 0.7, result.PopularityScore)
	assert.Equal(t, 0.95, result.QualityScore)
	assert.Equal(t, 1.0, result.FreshnessScore)
	assert.Equal(t, 0.5, result.LineageScore)
	assert.Equal(t, 2, len(result.BoostFactors))
	assert.Equal(t, 1, len(result.Penalties))
}

func TestRankerConfig_CustomWeights(t *testing.T) {
	indexer, _ := setupRankerTest(t)
	ctx := context.Background()

	// Create ranker with custom config emphasizing popularity
	config := RankerConfig{
		SemanticWeight:   0.20,
		MetadataWeight:   0.10,
		PopularityWeight: 0.50, // High popularity weight
		QualityWeight:    0.10,
		FreshnessWeight:  0.05,
		LineageWeight:    0.05,
		MinSemanticScore: 0.2,
		MaxResultAge:     30 * 24 * time.Hour,
	}
	ranker := NewHybridRanker(indexer, config)

	// Index features
	lowPopFeature := &FeatureMetadata{
		FeatureID:   "low_pop",
		Name:        "Low Popularity",
		Description: "A feature with low usage",
	}
	indexer.IndexFeatureWithMetadata(ctx, lowPopFeature)
	indexer.SetUsage("low_pop", &FeatureUsage{TotalReads: 100, PopularityScore: 0.1})

	highPopFeature := &FeatureMetadata{
		FeatureID:   "high_pop",
		Name:        "High Popularity",
		Description: "A feature with high usage",
	}
	indexer.IndexFeatureWithMetadata(ctx, highPopFeature)
	indexer.SetUsage("high_pop", &FeatureUsage{TotalReads: 1000000, PopularityScore: 0.95})

	// Rank
	req := RankRequest{
		Query: "feature popularity usage",
		Limit: 10,
	}

	results, err := ranker.Rank(ctx, req)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// High popularity feature should rank first due to high popularity weight
	assert.Equal(t, "high_pop", results[0].Feature.Metadata.FeatureID)
}
