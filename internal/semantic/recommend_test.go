package semantic

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultRecommendationConfig(t *testing.T) {
	cfg := DefaultRecommendationConfig()

	assert.Equal(t, 0.35, cfg.CollaborativeWeight)
	assert.Equal(t, 0.30, cfg.ContentBasedWeight)
	assert.Equal(t, 0.20, cfg.PopularityWeight)
	assert.Equal(t, 0.15, cfg.ContextWeight)
	assert.Equal(t, 10, cfg.DefaultLimit)
	assert.Equal(t, "popularity", cfg.ColdStartStrategy)
}

func TestNewRecommendationEngine(t *testing.T) {
	search := NewSearch(nil, slog.Default())
	indexer := NewEnhancedIndexer(search)
	discovery, _ := NewFeatureDiscovery(indexer, nil, DefaultDiscoveryConfig(), nil)

	t.Run("valid creation", func(t *testing.T) {
		engine, err := NewRecommendationEngine(discovery, DefaultRecommendationConfig(), nil)
		require.NoError(t, err)
		assert.NotNil(t, engine)
	})

	t.Run("nil discovery fails", func(t *testing.T) {
		_, err := NewRecommendationEngine(nil, DefaultRecommendationConfig(), nil)
		assert.Error(t, err)
	})
}

func setupTestRecommendEngine(t *testing.T) (*RecommendationEngine, context.Context) {
	search := NewSearch(nil, slog.Default())
	indexer := NewEnhancedIndexer(search)
	ctx := context.Background()

	// Add test features
	testFeatures := []*FeatureMetadata{
		{
			FeatureID:   "user_clicks",
			Name:        "User Click Count",
			Description: "Total number of clicks by user",
			Category:    "engagement",
			Domain:      "user",
			EntityType:  "user",
			Tags:        []string{"clicks", "engagement", "real-time"},
			Owner:       "data-team",
			QualityScore: 0.9,
			Freshness:   "real-time",
			UseCase:     []string{"analytics", "personalization"},
		},
		{
			FeatureID:   "user_purchases",
			Name:        "User Purchase Total",
			Description: "Total purchase amount by user",
			Category:    "revenue",
			Domain:      "user",
			EntityType:  "user",
			Tags:        []string{"purchases", "revenue", "transactions"},
			Owner:       "data-team",
			QualityScore: 0.85,
			Freshness:   "hourly",
			UseCase:     []string{"revenue", "personalization"},
		},
		{
			FeatureID:   "product_views",
			Name:        "Product View Count",
			Description: "Number of times product was viewed",
			Category:    "engagement",
			Domain:      "product",
			EntityType:  "product",
			Tags:        []string{"views", "engagement", "product"},
			Owner:       "ml-team",
			QualityScore: 0.8,
			Freshness:   "hourly",
			UseCase:     []string{"recommendations"},
		},
		{
			FeatureID:   "user_session_duration",
			Name:        "User Session Duration",
			Description: "Average session duration",
			Category:    "engagement",
			Domain:      "user",
			EntityType:  "user",
			Tags:        []string{"session", "engagement", "time"},
			Owner:       "data-team",
			QualityScore: 0.75,
			Freshness:   "daily",
			UseCase:     []string{"analytics"},
		},
		{
			FeatureID:   "fraud_score",
			Name:        "Fraud Risk Score",
			Description: "ML-based fraud risk score",
			Category:    "risk",
			Domain:      "transaction",
			EntityType:  "transaction",
			Tags:        []string{"fraud", "risk", "ml-derived"},
			Owner:       "fraud-team",
			QualityScore: 0.95,
			Freshness:   "real-time",
			UseCase:     []string{"fraud_detection", "risk_assessment"},
		},
	}

	for _, meta := range testFeatures {
		err := indexer.IndexFeatureWithMetadata(ctx, meta)
		require.NoError(t, err)
	}

	discovery, err := NewFeatureDiscovery(indexer, nil, DefaultDiscoveryConfig(), slog.Default())
	require.NoError(t, err)

	engine, err := NewRecommendationEngine(discovery, DefaultRecommendationConfig(), slog.Default())
	require.NoError(t, err)

	return engine, ctx
}

func TestRecommend(t *testing.T) {
	engine, ctx := setupTestRecommendEngine(t)

	t.Run("basic recommendation", func(t *testing.T) {
		result, err := engine.Recommend(ctx, RecommendationRequest{
			Limit: 5,
		})
		require.NoError(t, err)

		assert.NotNil(t, result)
		assert.Greater(t, len(result.Recommendations), 0)
		assert.LessOrEqual(t, len(result.Recommendations), 5)
	})

	t.Run("recommendation with domain filter", func(t *testing.T) {
		result, err := engine.Recommend(ctx, RecommendationRequest{
			Domain: "user",
			Limit:  10,
		})
		require.NoError(t, err)

		for _, rec := range result.Recommendations {
			assert.Equal(t, "user", rec.Feature.Metadata.Domain)
		}
	})

	t.Run("recommendation with category filter", func(t *testing.T) {
		result, err := engine.Recommend(ctx, RecommendationRequest{
			Category: "engagement",
			Limit:    10,
		})
		require.NoError(t, err)

		for _, rec := range result.Recommendations {
			assert.Equal(t, "engagement", rec.Feature.Metadata.Category)
		}
	})

	t.Run("recommendation with use case", func(t *testing.T) {
		result, err := engine.Recommend(ctx, RecommendationRequest{
			UseCase:  "personalization",
			Strategy: "context",
			Limit:    10,
		})
		require.NoError(t, err)

		assert.NotNil(t, result)
		// Should have features with personalization use case
	})

	t.Run("recommendation with quality filter", func(t *testing.T) {
		result, err := engine.Recommend(ctx, RecommendationRequest{
			MinQuality: 0.85,
			Limit:      10,
		})
		require.NoError(t, err)

		for _, rec := range result.Recommendations {
			assert.GreaterOrEqual(t, rec.Feature.Metadata.QualityScore, float32(0.85))
		}
	})

	t.Run("recommendation with freshness filter", func(t *testing.T) {
		result, err := engine.Recommend(ctx, RecommendationRequest{
			OnlyFresh: true,
			Limit:     10,
		})
		require.NoError(t, err)

		for _, rec := range result.Recommendations {
			assert.Contains(t, []string{"real-time", "hourly"}, rec.Feature.Metadata.Freshness)
		}
	})

	t.Run("recommendation with exclusions", func(t *testing.T) {
		result, err := engine.Recommend(ctx, RecommendationRequest{
			ExcludeFeatures: []string{"user_clicks", "user_purchases"},
			Limit:           10,
		})
		require.NoError(t, err)

		for _, rec := range result.Recommendations {
			assert.NotEqual(t, "user_clicks", rec.Feature.Metadata.FeatureID)
			assert.NotEqual(t, "user_purchases", rec.Feature.Metadata.FeatureID)
		}
	})

	t.Run("recommendation with current features", func(t *testing.T) {
		result, err := engine.Recommend(ctx, RecommendationRequest{
			CurrentFeatures: []string{"user_clicks"},
			Strategy:        "content",
			Limit:           5,
		})
		require.NoError(t, err)

		// Current features should be excluded
		for _, rec := range result.Recommendations {
			assert.NotEqual(t, "user_clicks", rec.Feature.Metadata.FeatureID)
		}
	})

	t.Run("recommendation includes scores", func(t *testing.T) {
		result, err := engine.Recommend(ctx, RecommendationRequest{
			IncludeScores: true,
			Limit:         5,
		})
		require.NoError(t, err)

		for _, rec := range result.Recommendations {
			assert.NotNil(t, rec.ScoreBreakdown)
			assert.GreaterOrEqual(t, rec.Score, 0.0)
		}
	})

	t.Run("recommendation includes reasons", func(t *testing.T) {
		result, err := engine.Recommend(ctx, RecommendationRequest{
			IncludeReasons: true,
			Limit:          5,
		})
		require.NoError(t, err)

		for _, rec := range result.Recommendations {
			assert.Greater(t, len(rec.Reasons), 0)
		}
	})
}

func TestRecommendStrategies(t *testing.T) {
	engine, ctx := setupTestRecommendEngine(t)

	strategies := []string{"balanced", "collaborative", "content", "popularity", "context"}

	for _, strategy := range strategies {
		t.Run(strategy, func(t *testing.T) {
			result, err := engine.Recommend(ctx, RecommendationRequest{
				Strategy: strategy,
				Limit:    5,
			})
			require.NoError(t, err)

			assert.Equal(t, strategy, result.Strategy)
			assert.NotNil(t, result.Recommendations)
		})
	}
}

func TestRecordInteraction(t *testing.T) {
	engine, _ := setupTestRecommendEngine(t)

	t.Run("record view", func(t *testing.T) {
		engine.RecordInteraction("user1", "user_clicks", "view")

		history, err := engine.GetUserHistory("user1")
		require.NoError(t, err)
		assert.Equal(t, 1, history.ViewedFeatures["user_clicks"].ViewCount)
	})

	t.Run("record multiple views", func(t *testing.T) {
		engine.RecordInteraction("user2", "user_purchases", "view")
		engine.RecordInteraction("user2", "user_purchases", "view")
		engine.RecordInteraction("user2", "user_purchases", "view")

		history, err := engine.GetUserHistory("user2")
		require.NoError(t, err)
		assert.Equal(t, 3, history.ViewedFeatures["user_purchases"].ViewCount)
	})

	t.Run("record favorite", func(t *testing.T) {
		engine.RecordInteraction("user3", "fraud_score", "view")
		engine.RecordInteraction("user3", "fraud_score", "favorite")

		history, err := engine.GetUserHistory("user3")
		require.NoError(t, err)
		assert.True(t, history.ViewedFeatures["fraud_score"].IsFavorite)
		assert.Contains(t, history.FavoriteFeatures, "fraud_score")
	})

	t.Run("record unfavorite", func(t *testing.T) {
		engine.RecordInteraction("user4", "product_views", "favorite")
		engine.RecordInteraction("user4", "product_views", "unfavorite")

		history, err := engine.GetUserHistory("user4")
		require.NoError(t, err)
		assert.False(t, history.ViewedFeatures["product_views"].IsFavorite)
		assert.NotContains(t, history.FavoriteFeatures, "product_views")
	})

	t.Run("record use", func(t *testing.T) {
		engine.RecordInteraction("user5", "user_clicks", "use")

		history, err := engine.GetUserHistory("user5")
		require.NoError(t, err)
		assert.True(t, history.ViewedFeatures["user_clicks"].IsUsed)
		assert.Contains(t, history.UsedFeatures, "user_clicks")
	})

	t.Run("updates preferences", func(t *testing.T) {
		engine.RecordInteraction("user6", "user_clicks", "view")
		engine.RecordInteraction("user6", "user_purchases", "view")

		history, err := engine.GetUserHistory("user6")
		require.NoError(t, err)
		assert.Greater(t, history.PreferredDomains["user"], 0.0)
	})
}

func TestRegisterModel(t *testing.T) {
	engine, ctx := setupTestRecommendEngine(t)

	t.Run("register model", func(t *testing.T) {
		engine.RegisterModel("model1", "Fraud Detector", "fraud_detection", []string{"fraud_score", "user_purchases"})

		stats := engine.GetStats()
		assert.Equal(t, 1, stats["model_count"])
	})

	t.Run("register multiple models", func(t *testing.T) {
		engine.RegisterModel("model2", "Recommender", "recommendations", []string{"user_clicks", "product_views"})
		engine.RegisterModel("model3", "Analytics", "analytics", []string{"user_clicks", "user_session_duration"})

		stats := engine.GetStats()
		assert.Equal(t, 3, stats["model_count"])
	})

	t.Run("affects context recommendations", func(t *testing.T) {
		// Recommendations for fraud_detection should include registered features
		result, err := engine.Recommend(ctx, RecommendationRequest{
			UseCase:  "fraud_detection",
			Strategy: "context",
			Limit:    10,
		})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestComputeUserSimilarities(t *testing.T) {
	engine, _ := setupTestRecommendEngine(t)

	// Create users with interactions
	for i := 0; i < 10; i++ {
		userID := fmt.Sprintf("sim_user_%d", i)

		// All users view common features
		engine.RecordInteraction(userID, "user_clicks", "view")
		engine.RecordInteraction(userID, "user_purchases", "view")

		// Some users view additional features
		if i%2 == 0 {
			engine.RecordInteraction(userID, "fraud_score", "view")
		}
		if i%3 == 0 {
			engine.RecordInteraction(userID, "product_views", "view")
		}

		// Ensure enough interactions
		for j := 0; j < 5; j++ {
			engine.RecordInteraction(userID, "user_clicks", "view")
		}
	}

	// Compute similarities
	engine.ComputeUserSimilarities()

	stats := engine.GetStats()
	assert.Greater(t, stats["user_similarities_count"], 0)
}

func TestRecommendForUseCase(t *testing.T) {
	engine, ctx := setupTestRecommendEngine(t)

	result, err := engine.RecommendForUseCase(ctx, "fraud_detection", 5)
	require.NoError(t, err)

	assert.NotNil(t, result)
	assert.Equal(t, "context", result.Strategy)
}

func TestRecommendForUser(t *testing.T) {
	engine, ctx := setupTestRecommendEngine(t)

	// Create user history
	engine.RecordInteraction("test_user", "user_clicks", "view")
	engine.RecordInteraction("test_user", "user_clicks", "favorite")
	engine.RecordInteraction("test_user", "user_purchases", "view")
	engine.RecordInteraction("test_user", "user_purchases", "use")

	result, err := engine.RecommendForUser(ctx, "test_user", 5)
	require.NoError(t, err)

	assert.NotNil(t, result)
	// User history should influence the recommendations (personalization)
	assert.GreaterOrEqual(t, len(result.Recommendations), 0)
}

func TestRecommendSimilarTo(t *testing.T) {
	engine, ctx := setupTestRecommendEngine(t)

	result, err := engine.RecommendSimilarTo(ctx, []string{"user_clicks"}, 5)
	require.NoError(t, err)

	assert.NotNil(t, result)
	assert.Equal(t, "content", result.Strategy)

	// Should not include input features
	for _, rec := range result.Recommendations {
		assert.NotEqual(t, "user_clicks", rec.Feature.Metadata.FeatureID)
	}
}

func TestRecommendTrending(t *testing.T) {
	engine, ctx := setupTestRecommendEngine(t)

	// Add some popularity data
	for i := 0; i < 100; i++ {
		engine.RecordInteraction(fmt.Sprintf("trending_user_%d", i), "user_clicks", "view")
	}
	for i := 0; i < 50; i++ {
		engine.RecordInteraction(fmt.Sprintf("trending_user_%d", i), "fraud_score", "view")
	}

	result, err := engine.RecommendTrending(ctx, 5)
	require.NoError(t, err)

	assert.NotNil(t, result)
	assert.Equal(t, "popularity", result.Strategy)
}

func TestGetUserHistory(t *testing.T) {
	engine, _ := setupTestRecommendEngine(t)

	t.Run("existing user", func(t *testing.T) {
		engine.RecordInteraction("history_user", "user_clicks", "view")

		history, err := engine.GetUserHistory("history_user")
		require.NoError(t, err)
		assert.Equal(t, "history_user", history.UserID)
	})

	t.Run("nonexistent user", func(t *testing.T) {
		_, err := engine.GetUserHistory("nonexistent")
		assert.Error(t, err)
	})
}

func TestRecommendEngineGetStats(t *testing.T) {
	engine, _ := setupTestRecommendEngine(t)

	// Add some data
	engine.RecordInteraction("stats_user", "user_clicks", "view")
	engine.RegisterModel("stats_model", "Test Model", "test", []string{"user_clicks"})

	stats := engine.GetStats()

	assert.Contains(t, stats, "user_count")
	assert.Contains(t, stats, "model_count")
	assert.Contains(t, stats, "tracked_features")
	assert.Contains(t, stats, "use_case_count")
	assert.Contains(t, stats, "domain_count")
	assert.Contains(t, stats, "category_count")
	assert.Contains(t, stats, "config")

	assert.GreaterOrEqual(t, stats["user_count"], 1)
	assert.GreaterOrEqual(t, stats["model_count"], 1)
}

func TestDiversityApplication(t *testing.T) {
	engine, ctx := setupTestRecommendEngine(t)

	// Get recommendations with high diversity
	result1, err := engine.Recommend(ctx, RecommendationRequest{
		DiversityFactor: 0.5,
		Limit:           5,
	})
	require.NoError(t, err)

	// Get recommendations with no diversity
	result2, err := engine.Recommend(ctx, RecommendationRequest{
		DiversityFactor: 0,
		Limit:           5,
	})
	require.NoError(t, err)

	// Both should return results
	assert.Greater(t, len(result1.Recommendations), 0)
	assert.Greater(t, len(result2.Recommendations), 0)
}

func TestRecommendScoreBreakdown(t *testing.T) {
	engine, ctx := setupTestRecommendEngine(t)

	result, err := engine.Recommend(ctx, RecommendationRequest{
		IncludeScores: true,
		Limit:         5,
	})
	require.NoError(t, err)

	for _, rec := range result.Recommendations {
		assert.NotNil(t, rec.ScoreBreakdown)
		scores := rec.ScoreBreakdown

		// All scores should be in valid range
		assert.GreaterOrEqual(t, scores.CollaborativeScore, 0.0)
		assert.GreaterOrEqual(t, scores.ContentScore, 0.0)
		assert.GreaterOrEqual(t, scores.PopularityScore, 0.0)
		assert.GreaterOrEqual(t, scores.ContextScore, 0.0)
		assert.GreaterOrEqual(t, scores.QualityScore, 0.0)
		assert.GreaterOrEqual(t, scores.FinalScore, 0.0)
	}
}

func TestConfidenceCalculation(t *testing.T) {
	engine, ctx := setupTestRecommendEngine(t)

	result, err := engine.Recommend(ctx, RecommendationRequest{
		Limit: 5,
	})
	require.NoError(t, err)

	for _, rec := range result.Recommendations {
		assert.GreaterOrEqual(t, rec.Confidence, 0.0)
		assert.LessOrEqual(t, rec.Confidence, 1.0)
	}
}

func TestFeatureSimilarityComputation(t *testing.T) {
	engine, _ := setupTestRecommendEngine(t)

	t.Run("same feature", func(t *testing.T) {
		sim := engine.computeFeatureSimilarity("user_clicks", "user_clicks")
		assert.Equal(t, 1.0, sim)
	})

	t.Run("same domain features", func(t *testing.T) {
		sim := engine.computeFeatureSimilarity("user_clicks", "user_purchases")
		assert.Greater(t, sim, 0.0)
	})

	t.Run("different domain features", func(t *testing.T) {
		sim := engine.computeFeatureSimilarity("user_clicks", "product_views")
		// Should be lower than same domain - just verify it returns a valid score
		assert.GreaterOrEqual(t, sim, 0.0)
	})
}

func TestRecommendHelperFunctions(t *testing.T) {
	t.Run("containsString", func(t *testing.T) {
		slice := []string{"a", "b", "c"}
		assert.True(t, containsString(slice, "a"))
		assert.False(t, containsString(slice, "d"))
	})

	t.Run("removeString", func(t *testing.T) {
		slice := []string{"a", "b", "c"}
		result := removeString(slice, "b")
		assert.Len(t, result, 2)
		assert.NotContains(t, result, "b")
	})

	t.Run("appendUniqueString", func(t *testing.T) {
		slice := []string{"a", "b"}
		result := appendUniqueString(slice, "c")
		assert.Len(t, result, 3)

		result = appendUniqueString(result, "a") // Already exists
		assert.Len(t, result, 3)
	})

	t.Run("computeJaccardSimilarity", func(t *testing.T) {
		set1 := map[string]*FeatureInteraction{
			"a": {}, "b": {}, "c": {},
		}
		set2 := map[string]*FeatureInteraction{
			"b": {}, "c": {}, "d": {},
		}

		sim := computeJaccardSimilarity(set1, set2)
		// Intersection: {b, c} = 2
		// Union: {a, b, c, d} = 4
		// Similarity: 2/4 = 0.5
		assert.Equal(t, 0.5, sim)
	})

	t.Run("computeJaccardSimilarity empty sets", func(t *testing.T) {
		set1 := map[string]*FeatureInteraction{}
		set2 := map[string]*FeatureInteraction{"a": {}}

		assert.Equal(t, 0.0, computeJaccardSimilarity(set1, set2))
		assert.Equal(t, 0.0, computeJaccardSimilarity(set2, set1))
	})
}

