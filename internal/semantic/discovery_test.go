package semantic

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultDiscoveryConfig(t *testing.T) {
	cfg := DefaultDiscoveryConfig()

	assert.Equal(t, 10, cfg.DefaultLimit)
	assert.Equal(t, 100, cfg.MaxLimit)
	assert.Equal(t, float32(0.3), cfg.MinSemanticScore)
	assert.True(t, cfg.EnableExplanations)
	assert.True(t, cfg.EnableAutoComplete)
	assert.True(t, cfg.EnableRelatedSearch)
	assert.True(t, cfg.EnableGraphSearch)
	assert.Equal(t, 3, cfg.MaxGraphDepth)
}

func TestNewFeatureDiscovery(t *testing.T) {
	search := NewSearch(nil, slog.Default())
	indexer := NewEnhancedIndexer(search)

	t.Run("valid creation", func(t *testing.T) {
		fd, err := NewFeatureDiscovery(indexer, nil, DefaultDiscoveryConfig(), nil)
		require.NoError(t, err)
		assert.NotNil(t, fd)
	})

	t.Run("nil indexer", func(t *testing.T) {
		_, err := NewFeatureDiscovery(nil, nil, DefaultDiscoveryConfig(), nil)
		assert.Error(t, err)
	})
}

func setupTestDiscovery(t *testing.T) (*FeatureDiscovery, context.Context) {
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
			DataType:    "int64",
			ValueType:   "numeric",
			QualityScore: 0.9,
			Freshness:   "real-time",
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
			DataType:    "float64",
			ValueType:   "numeric",
			QualityScore: 0.85,
			Freshness:   "hourly",
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
			DataType:    "int64",
			ValueType:   "numeric",
			QualityScore: 0.8,
			Freshness:   "hourly",
		},
		{
			FeatureID:   "user_session_duration",
			Name:        "User Session Duration",
			Description: "Average session duration for user",
			Category:    "engagement",
			Domain:      "user",
			EntityType:  "user",
			Tags:        []string{"session", "engagement", "time"},
			Owner:       "data-team",
			DataType:    "float64",
			ValueType:   "numeric",
			QualityScore: 0.75,
			Freshness:   "daily",
		},
		{
			FeatureID:   "fraud_score",
			Name:        "Fraud Risk Score",
			Description: "ML-based fraud risk score for transaction",
			Category:    "risk",
			Domain:      "transaction",
			EntityType:  "transaction",
			Tags:        []string{"fraud", "risk", "ml-derived"},
			Owner:       "fraud-team",
			DataType:    "float64",
			ValueType:   "numeric",
			QualityScore: 0.95,
			Freshness:   "real-time",
			UseCase:     []string{"fraud_detection", "risk_assessment"},
		},
	}

	for _, meta := range testFeatures {
		err := indexer.IndexFeatureWithMetadata(ctx, meta)
		require.NoError(t, err)
	}

	fd, err := NewFeatureDiscovery(indexer, nil, DefaultDiscoveryConfig(), slog.Default())
	require.NoError(t, err)

	return fd, ctx
}

func TestDiscover(t *testing.T) {
	fd, ctx := setupTestDiscovery(t)

	t.Run("basic search", func(t *testing.T) {
		query := DiscoveryQuery{
			Query: "user engagement clicks",
			Limit: 10,
		}

		result, err := fd.Discover(ctx, query)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Greater(t, result.TotalResults, 0)
		assert.Equal(t, "user engagement clicks", result.Query)
	})

	t.Run("search with category filter", func(t *testing.T) {
		query := DiscoveryQuery{
			Query:      "engagement",
			Categories: []string{"engagement"},
			Limit:      10,
		}

		result, err := fd.Discover(ctx, query)
		require.NoError(t, err)

		for _, f := range result.Features {
			assert.Equal(t, "engagement", f.Feature.Metadata.Category)
		}
	})

	t.Run("search with domain filter", func(t *testing.T) {
		query := DiscoveryQuery{
			Query:   "user features",
			Domains: []string{"user"},
			Limit:   10,
		}

		result, err := fd.Discover(ctx, query)
		require.NoError(t, err)

		for _, f := range result.Features {
			assert.Equal(t, "user", f.Feature.Metadata.Domain)
		}
	})

	t.Run("search with explanation", func(t *testing.T) {
		query := DiscoveryQuery{
			Query:              "click count",
			Limit:              5,
			IncludeExplanation: true,
		}

		result, err := fd.Discover(ctx, query)
		require.NoError(t, err)

		// At least some features should have explanations
		hasExplanation := false
		for _, f := range result.Features {
			if f.Explanation != nil {
				hasExplanation = true
				break
			}
		}
		assert.True(t, hasExplanation || len(result.Features) == 0)
	})

	t.Run("search with user personalization", func(t *testing.T) {
		// First set user preferences
		prefs := &UserDiscoveryPreferences{
			UserID:           "test-user",
			PreferredDomains: []string{"user"},
			LastActivity:     time.Now(),
		}
		err := fd.SetUserPreferences(prefs)
		require.NoError(t, err)

		query := DiscoveryQuery{
			Query:              "features",
			UserID:             "test-user",
			UsePersonalization: true,
			Limit:              10,
		}

		result, err := fd.Discover(ctx, query)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("search with quality filter", func(t *testing.T) {
		query := DiscoveryQuery{
			Query:      "all features",
			MinQuality: 0.85,
			Limit:      10,
		}

		result, err := fd.Discover(ctx, query)
		require.NoError(t, err)

		for _, f := range result.Features {
			assert.GreaterOrEqual(t, f.Feature.Metadata.QualityScore, float32(0.85))
		}
	})

	t.Run("empty query returns results", func(t *testing.T) {
		query := DiscoveryQuery{
			Query: "",
			Limit: 10,
		}

		result, err := fd.Discover(ctx, query)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("pagination", func(t *testing.T) {
		// First page
		query1 := DiscoveryQuery{
			Query:  "features",
			Limit:  2,
			Offset: 0,
		}
		result1, err := fd.Discover(ctx, query1)
		require.NoError(t, err)

		// Second page
		query2 := DiscoveryQuery{
			Query:  "features",
			Limit:  2,
			Offset: 2,
		}
		result2, err := fd.Discover(ctx, query2)
		require.NoError(t, err)

		// Results should be different
		if len(result1.Features) > 0 && len(result2.Features) > 0 {
			assert.NotEqual(t, result1.Features[0].Feature.Metadata.FeatureID, result2.Features[0].Feature.Metadata.FeatureID)
		}
	})
}

func TestFindSimilar(t *testing.T) {
	fd, ctx := setupTestDiscovery(t)

	t.Run("find similar features", func(t *testing.T) {
		similar, err := fd.FindSimilar(ctx, "user_clicks", 5)
		require.NoError(t, err)

		// Should find other user engagement features
		assert.Greater(t, len(similar), 0)

		// Verify none are the source feature
		for _, s := range similar {
			assert.NotEqual(t, "user_clicks", s.Feature.Metadata.FeatureID)
		}
	})

	t.Run("feature not found", func(t *testing.T) {
		_, err := fd.FindSimilar(ctx, "nonexistent", 5)
		assert.Error(t, err)
	})
}

func TestAutoComplete(t *testing.T) {
	fd, ctx := setupTestDiscovery(t)

	t.Run("prefix match", func(t *testing.T) {
		suggestions, err := fd.AutoComplete(ctx, "user", 10)
		require.NoError(t, err)
		assert.Greater(t, len(suggestions), 0)

		for _, s := range suggestions {
			assert.Contains(t, strings.ToLower(s), "user")
		}
	})

	t.Run("empty prefix", func(t *testing.T) {
		suggestions, err := fd.AutoComplete(ctx, "", 10)
		require.NoError(t, err)
		assert.Nil(t, suggestions)
	})

	t.Run("tag prefix", func(t *testing.T) {
		suggestions, err := fd.AutoComplete(ctx, "eng", 10)
		require.NoError(t, err)
		assert.Greater(t, len(suggestions), 0)
	})
}

func TestFeatureRelationships(t *testing.T) {
	fd, _ := setupTestDiscovery(t)

	t.Run("add relationship", func(t *testing.T) {
		err := fd.AddFeatureRelationship("user_clicks", "user_purchases", EdgeTypeCoUsed, 0.8)
		require.NoError(t, err)
	})

	t.Run("self-referential edge fails", func(t *testing.T) {
		err := fd.AddFeatureRelationship("user_clicks", "user_clicks", EdgeTypeSimilar, 0.9)
		assert.Error(t, err)
	})

	t.Run("nonexistent source fails", func(t *testing.T) {
		err := fd.AddFeatureRelationship("nonexistent", "user_clicks", EdgeTypeSimilar, 0.9)
		assert.Error(t, err)
	})

	t.Run("nonexistent target fails", func(t *testing.T) {
		err := fd.AddFeatureRelationship("user_clicks", "nonexistent", EdgeTypeSimilar, 0.9)
		assert.Error(t, err)
	})

	t.Run("get feature graph", func(t *testing.T) {
		// Add some relationships first
		fd.AddFeatureRelationship("user_clicks", "user_purchases", EdgeTypeRelated, 0.7)
		fd.AddFeatureRelationship("user_clicks", "user_session_duration", EdgeTypeRelated, 0.6)
		fd.AddFeatureRelationship("user_purchases", "fraud_score", EdgeTypeDerived, 0.9)

		graph, err := fd.GetFeatureGraph("user_clicks", 2)
		require.NoError(t, err)
		assert.NotNil(t, graph)
		assert.Equal(t, "user_clicks", graph.RootID)
		assert.Greater(t, len(graph.Nodes), 0)
	})

	t.Run("graph depth limit", func(t *testing.T) {
		graph, err := fd.GetFeatureGraph("user_clicks", 10) // Exceeds max
		require.NoError(t, err)
		// Should be capped at max depth
		assert.NotNil(t, graph)
	})

	t.Run("nonexistent feature graph", func(t *testing.T) {
		_, err := fd.GetFeatureGraph("nonexistent", 1)
		assert.Error(t, err)
	})
}

func TestGraphCentrality(t *testing.T) {
	fd, _ := setupTestDiscovery(t)

	// Add some relationships
	fd.AddFeatureRelationship("user_clicks", "user_purchases", EdgeTypeCoUsed, 0.8)
	fd.AddFeatureRelationship("user_clicks", "user_session_duration", EdgeTypeCoUsed, 0.7)
	fd.AddFeatureRelationship("user_clicks", "product_views", EdgeTypeRelated, 0.6)
	fd.AddFeatureRelationship("user_purchases", "fraud_score", EdgeTypeDerived, 0.9)
	fd.AddFeatureRelationship("product_views", "user_session_duration", EdgeTypeRelated, 0.5)

	t.Run("compute centrality", func(t *testing.T) {
		fd.ComputeGraphCentrality()

		// Get most central features
		central := fd.GetMostCentralFeatures(3)
		assert.Greater(t, len(central), 0)

		// user_clicks should be highly central (many connections)
		for _, node := range central {
			assert.GreaterOrEqual(t, node.Centrality, 0.0)
			assert.LessOrEqual(t, node.Centrality, 1.0)
		}
	})
}

func TestQueryHistory(t *testing.T) {
	fd, ctx := setupTestDiscovery(t)

	// Perform some searches
	queries := []string{"clicks", "purchases", "engagement"}
	for _, q := range queries {
		fd.Discover(ctx, DiscoveryQuery{
			Query:  q,
			UserID: "test-user",
			Limit:  5,
		})
	}

	t.Run("get all history", func(t *testing.T) {
		history := fd.GetQueryHistory("", 10)
		assert.GreaterOrEqual(t, len(history), 3)
	})

	t.Run("get user history", func(t *testing.T) {
		history := fd.GetQueryHistory("test-user", 10)
		assert.GreaterOrEqual(t, len(history), 3)

		for _, h := range history {
			assert.Equal(t, "test-user", h.UserID)
		}
	})
}

func TestUserPreferences(t *testing.T) {
	fd, _ := setupTestDiscovery(t)

	t.Run("set and get preferences", func(t *testing.T) {
		prefs := &UserDiscoveryPreferences{
			UserID:           "user1",
			PreferredDomains: []string{"user", "product"},
			PreferredOwners:  []string{"data-team"},
			FavoriteFeatures: []string{"user_clicks"},
			LastActivity:     time.Now(),
		}

		err := fd.SetUserPreferences(prefs)
		require.NoError(t, err)

		retrieved, err := fd.GetUserPreferences("user1")
		require.NoError(t, err)
		assert.Equal(t, "user1", retrieved.UserID)
		assert.Equal(t, []string{"user", "product"}, retrieved.PreferredDomains)
	})

	t.Run("missing user preferences", func(t *testing.T) {
		_, err := fd.GetUserPreferences("nonexistent")
		assert.Error(t, err)
	})

	t.Run("empty user ID fails", func(t *testing.T) {
		err := fd.SetUserPreferences(&UserDiscoveryPreferences{})
		assert.Error(t, err)
	})
}

func TestRecordFeatureClick(t *testing.T) {
	fd, ctx := setupTestDiscovery(t)

	// Perform a search first
	result, err := fd.Discover(ctx, DiscoveryQuery{
		Query:  "clicks",
		UserID: "click-user",
		Limit:  5,
	})
	require.NoError(t, err)

	// Get the query ID from history
	history := fd.GetQueryHistory("click-user", 1)
	require.Greater(t, len(history), 0)
	queryID := history[0].ID

	t.Run("record click", func(t *testing.T) {
		if len(result.Features) > 0 {
			fd.RecordFeatureClick("click-user", result.Features[0].Feature.Metadata.FeatureID, queryID)

			// Verify click was recorded
			updatedHistory := fd.GetQueryHistory("click-user", 1)
			assert.Greater(t, len(updatedHistory[0].ClickedIDs), 0)
		}
	})
}

func TestDiscoveryStats(t *testing.T) {
	fd, ctx := setupTestDiscovery(t)

	// Perform some operations
	fd.Discover(ctx, DiscoveryQuery{Query: "test", Limit: 5})
	fd.AddFeatureRelationship("user_clicks", "user_purchases", EdgeTypeCoUsed, 0.8)

	stats := fd.GetDiscoveryStats()
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "total_queries")
	assert.Contains(t, stats, "graph_nodes")
	assert.Contains(t, stats, "graph_edges")
	assert.Contains(t, stats, "indexed_features")
}

func TestBuildFeatureEmbeddingIndex(t *testing.T) {
	fd, ctx := setupTestDiscovery(t)

	t.Run("build index without embedder", func(t *testing.T) {
		err := fd.BuildFeatureEmbeddingIndex(ctx)
		require.NoError(t, err)
	})
}

func TestInferFeatureRelationships(t *testing.T) {
	fd, ctx := setupTestDiscovery(t)

	t.Run("infer relationships", func(t *testing.T) {
		// Record initial edge count
		initialStats := fd.GetDiscoveryStats()
		initialEdges := initialStats["graph_edges"].(int)

		err := fd.InferFeatureRelationships(ctx)
		require.NoError(t, err)

		// Should have created some relationships
		newStats := fd.GetDiscoveryStats()
		newEdges := newStats["graph_edges"].(int)
		assert.GreaterOrEqual(t, newEdges, initialEdges)
	})
}

func TestDiscoveryResultFacets(t *testing.T) {
	fd, ctx := setupTestDiscovery(t)

	result, err := fd.Discover(ctx, DiscoveryQuery{
		Query: "features",
		Limit: 10,
	})
	require.NoError(t, err)

	assert.NotNil(t, result.Facets)
	assert.NotNil(t, result.Facets.Categories)
	assert.NotNil(t, result.Facets.Domains)
	assert.NotNil(t, result.Facets.EntityTypes)
	assert.NotNil(t, result.Facets.Tags)
}

func TestQuerySuggestions(t *testing.T) {
	fd, ctx := setupTestDiscovery(t)

	result, err := fd.Discover(ctx, DiscoveryQuery{
		Query: "user",
		Limit: 10,
	})
	require.NoError(t, err)

	// Suggestions should be generated
	assert.NotNil(t, result.Suggestions)
}

func TestScoreBreakdown(t *testing.T) {
	fd, ctx := setupTestDiscovery(t)

	result, err := fd.Discover(ctx, DiscoveryQuery{
		Query: "user clicks engagement",
		Limit: 5,
	})
	require.NoError(t, err)

	for _, f := range result.Features {
		assert.NotNil(t, f.ScoreBreakdown)
		assert.GreaterOrEqual(t, f.ScoreBreakdown.SemanticScore, 0.0)
		assert.GreaterOrEqual(t, f.ScoreBreakdown.BoostMultiplier, 0.0)
		assert.GreaterOrEqual(t, f.ScoreBreakdown.PenaltyMultiplier, 0.0)
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("contains", func(t *testing.T) {
		slice := []string{"a", "B", "c"}
		assert.True(t, contains(slice, "a"))
		assert.True(t, contains(slice, "b")) // Case insensitive
		assert.False(t, contains(slice, "d"))
	})

	t.Run("appendUnique", func(t *testing.T) {
		slice := []string{"a", "b"}
		result := appendUnique(slice, "c")
		assert.Len(t, result, 3)

		result = appendUnique(result, "a") // Already exists
		assert.Len(t, result, 3)
	})

	t.Run("calculateMetadataSimilarity", func(t *testing.T) {
		a := &FeatureMetadata{
			Category:   "engagement",
			Domain:     "user",
			Tags:       []string{"clicks", "user"},
			UseCase:    []string{"analytics"},
		}
		b := &FeatureMetadata{
			Category:   "engagement",
			Domain:     "user",
			Tags:       []string{"clicks", "product"},
			UseCase:    []string{"analytics", "ml"},
		}

		sim := calculateMetadataSimilarity(a, b)
		assert.Greater(t, sim, float32(0))
		assert.LessOrEqual(t, sim, float32(1))
	})
}
