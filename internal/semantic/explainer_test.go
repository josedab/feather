package semantic

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLLMClient implements LLMClient for testing.
type MockLLMClient struct {
	available  bool
	response   string
	err        error
	callCount  int
	lastPrompt string
}

func (m *MockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	m.callCount++
	m.lastPrompt = prompt
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *MockLLMClient) IsAvailable() bool {
	return m.available
}

func setupExplainerTest(t *testing.T) (*EnhancedIndexer, *Explainer) {
	t.Helper()
	search := NewSearch(NewLocalEmbedder(384), nil)
	indexer := NewEnhancedIndexer(search)
	explainer := NewExplainer(indexer, nil, DefaultExplainerConfig())
	return indexer, explainer
}

func TestDefaultExplainerConfig(t *testing.T) {
	config := DefaultExplainerConfig()

	assert.Equal(t, 500, config.MaxExplanationLength)
	assert.Equal(t, 15*time.Minute, config.CacheTTL)
	assert.True(t, config.IncludeStatistics)
	assert.True(t, config.IncludeLineage)
	assert.True(t, config.IncludeUsage)
	assert.True(t, config.FallbackToTemplate)
}

func TestNewExplainer(t *testing.T) {
	indexer, explainer := setupExplainerTest(t)

	assert.NotNil(t, explainer)
	assert.Equal(t, indexer, explainer.indexer)
	assert.NotNil(t, explainer.cache)
}

func TestNewExplainer_WithLLMClient(t *testing.T) {
	search := NewSearch(NewLocalEmbedder(384), nil)
	indexer := NewEnhancedIndexer(search)
	mockLLM := &MockLLMClient{available: true, response: "Test explanation"}
	config := DefaultExplainerConfig()

	explainer := NewExplainer(indexer, mockLLM, config)

	assert.NotNil(t, explainer)
	assert.Equal(t, mockLLM, explainer.llmClient)
}

func TestExplainer_Explain_TemplateOnly(t *testing.T) {
	indexer, explainer := setupExplainerTest(t)
	ctx := context.Background()

	// Index a feature
	meta := &FeatureMetadata{
		FeatureID:    "test_feature",
		Name:         "Test Feature",
		Description:  "A test feature for explanation",
		Category:     "testing",
		Tags:         []string{"test", "example"},
		QualityScore: 0.9,
		DataQuality:  "high",
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	// Generate explanation
	explanation, err := explainer.Explain(ctx, "test_feature", "test feature example", 0.85)
	require.NoError(t, err)
	require.NotNil(t, explanation)

	assert.Equal(t, "test_feature", explanation.FeatureID)
	assert.Equal(t, "test feature example", explanation.Query)
	assert.NotEmpty(t, explanation.Summary)
	assert.Equal(t, "template", explanation.GeneratedBy)
	assert.Equal(t, 0.85, explanation.Confidence)
	assert.NotEmpty(t, explanation.MatchReasons)
	assert.False(t, explanation.GeneratedAt.IsZero())
}

func TestExplainer_Explain_WithLLM(t *testing.T) {
	search := NewSearch(NewLocalEmbedder(384), nil)
	indexer := NewEnhancedIndexer(search)
	ctx := context.Background()

	mockLLM := &MockLLMClient{
		available: true,
		response:  "This feature provides user purchase data which is highly relevant for analyzing buying behavior.",
	}
	config := DefaultExplainerConfig()
	explainer := NewExplainer(indexer, mockLLM, config)

	// Index a feature
	meta := &FeatureMetadata{
		FeatureID:    "user_purchases",
		Name:         "User Purchases",
		Description:  "Total purchases made by user",
		Category:     "revenue",
		EntityType:   "user",
		QualityScore: 0.95,
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	// Generate explanation with LLM
	explanation, err := explainer.Explain(ctx, "user_purchases", "user buying behavior", 0.9)
	require.NoError(t, err)
	require.NotNil(t, explanation)

	assert.Equal(t, "llm", explanation.GeneratedBy)
	assert.Contains(t, explanation.Summary, "user purchase data")
	assert.Equal(t, 1, mockLLM.callCount)
}

func TestExplainer_Explain_LLMFallbackToTemplate(t *testing.T) {
	search := NewSearch(NewLocalEmbedder(384), nil)
	indexer := NewEnhancedIndexer(search)
	ctx := context.Background()

	// LLM that fails
	mockLLM := &MockLLMClient{
		available: true,
		err:       errors.New("LLM service unavailable"),
	}
	config := DefaultExplainerConfig()
	explainer := NewExplainer(indexer, mockLLM, config)

	// Index a feature
	meta := &FeatureMetadata{
		FeatureID: "fallback_feature",
		Name:      "Fallback Feature",
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	// Should fall back to template
	explanation, err := explainer.Explain(ctx, "fallback_feature", "test query", 0.7)
	require.NoError(t, err)
	require.NotNil(t, explanation)

	assert.Equal(t, "template", explanation.GeneratedBy)
}

func TestExplainer_Explain_LLMUnavailable(t *testing.T) {
	search := NewSearch(NewLocalEmbedder(384), nil)
	indexer := NewEnhancedIndexer(search)
	ctx := context.Background()

	// LLM that's not available
	mockLLM := &MockLLMClient{
		available: false,
		response:  "Should not be called",
	}
	config := DefaultExplainerConfig()
	explainer := NewExplainer(indexer, mockLLM, config)

	// Index a feature
	meta := &FeatureMetadata{
		FeatureID: "unavailable_test",
		Name:      "Unavailable Test",
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	// Should use template since LLM is unavailable
	explanation, err := explainer.Explain(ctx, "unavailable_test", "test", 0.5)
	require.NoError(t, err)
	require.NotNil(t, explanation)

	assert.Equal(t, "template", explanation.GeneratedBy)
	assert.Equal(t, 0, mockLLM.callCount) // LLM should not be called
}

func TestExplainer_Explain_Caching(t *testing.T) {
	search := NewSearch(NewLocalEmbedder(384), nil)
	indexer := NewEnhancedIndexer(search)
	ctx := context.Background()

	mockLLM := &MockLLMClient{
		available: true,
		response:  "Cached explanation",
	}
	config := DefaultExplainerConfig()
	explainer := NewExplainer(indexer, mockLLM, config)

	// Index a feature
	meta := &FeatureMetadata{
		FeatureID: "cached_feature",
		Name:      "Cached Feature",
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	// First call
	explanation1, err := explainer.Explain(ctx, "cached_feature", "test query", 0.8)
	require.NoError(t, err)
	assert.Equal(t, 1, mockLLM.callCount)

	// Second call with same parameters should use cache
	explanation2, err := explainer.Explain(ctx, "cached_feature", "test query", 0.8)
	require.NoError(t, err)
	assert.Equal(t, 1, mockLLM.callCount) // Still 1, cache was used

	// Both explanations should be the same
	assert.Equal(t, explanation1.Summary, explanation2.Summary)
}

func TestExplainer_Explain_DifferentQueries(t *testing.T) {
	search := NewSearch(NewLocalEmbedder(384), nil)
	indexer := NewEnhancedIndexer(search)
	ctx := context.Background()

	mockLLM := &MockLLMClient{
		available: true,
		response:  "Explanation for query",
	}
	config := DefaultExplainerConfig()
	explainer := NewExplainer(indexer, mockLLM, config)

	// Index a feature
	meta := &FeatureMetadata{
		FeatureID: "multi_query",
		Name:      "Multi Query Feature",
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	// Different queries should result in separate LLM calls
	_, err := explainer.Explain(ctx, "multi_query", "query one", 0.8)
	require.NoError(t, err)
	assert.Equal(t, 1, mockLLM.callCount)

	_, err = explainer.Explain(ctx, "multi_query", "query two", 0.8)
	require.NoError(t, err)
	assert.Equal(t, 2, mockLLM.callCount)
}

func TestExplainer_Explain_FeatureNotFound(t *testing.T) {
	_, explainer := setupExplainerTest(t)
	ctx := context.Background()

	_, err := explainer.Explain(ctx, "nonexistent", "test query", 0.5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "feature not found")
}

func TestExplainer_Explain_WithStatistics(t *testing.T) {
	indexer, explainer := setupExplainerTest(t)
	ctx := context.Background()

	// Index a feature with statistics
	meta := &FeatureMetadata{
		FeatureID:    "stats_feature",
		Name:         "Stats Feature",
		QualityScore: 0.9,
		Freshness:    "real-time",
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	stats := &FeatureStatistics{
		Mean:           50.0,
		StdDev:         10.0,
		Min:            0,
		Max:            100,
		NullPercentage: 0.5,
		Distribution:   "normal",
	}
	indexer.SetStatistics("stats_feature", stats)

	explanation, err := explainer.Explain(ctx, "stats_feature", "stats", 0.8)
	require.NoError(t, err)
	require.NotNil(t, explanation.DataInsights)

	assert.Contains(t, explanation.DataInsights.Distribution, "normal")
	assert.Contains(t, explanation.DataInsights.ValueRange, "0.00")
	assert.Contains(t, explanation.DataInsights.ValueRange, "100.00")
	assert.Contains(t, explanation.DataInsights.QualitySummary, "completeness")
	assert.Contains(t, explanation.DataInsights.FreshnessSummary, "real-time")
}

func TestExplainer_Explain_WithUsage(t *testing.T) {
	indexer, explainer := setupExplainerTest(t)
	ctx := context.Background()

	// Index a feature with usage
	meta := &FeatureMetadata{
		FeatureID: "usage_feature",
		Name:      "Usage Feature",
		UseCase:   []string{"fraud_detection", "personalization"},
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	usage := &FeatureUsage{
		TotalReads:  150000,
		ModelsUsing: []string{"fraud_model", "rec_model"},
		ModelCount:  2,
	}
	indexer.SetUsage("usage_feature", usage)

	explanation, err := explainer.Explain(ctx, "usage_feature", "usage", 0.8)
	require.NoError(t, err)
	require.NotNil(t, explanation.UsageContext)

	assert.Contains(t, explanation.UsageContext.Popularity, "Highly popular")
	assert.Equal(t, []string{"fraud_model", "rec_model"}, explanation.UsageContext.ModelsUsing)
	assert.Equal(t, []string{"fraud_detection", "personalization"}, explanation.UsageContext.CommonUseCases)
}

func TestExplainer_Explain_MatchReasons(t *testing.T) {
	indexer, explainer := setupExplainerTest(t)
	ctx := context.Background()

	meta := &FeatureMetadata{
		FeatureID:  "match_feature",
		Name:       "User Click Count",
		Category:   "engagement",
		Tags:       []string{"user", "clicks", "behavior"},
		Domain:     "analytics",
		EntityType: "user",
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	// Query that matches multiple aspects
	explanation, err := explainer.Explain(ctx, "match_feature", "user clicks engagement", 0.9)
	require.NoError(t, err)
	require.NotEmpty(t, explanation.MatchReasons)

	// Should have semantic match
	hasSemanticReason := false
	hasTagReason := false
	hasCategoryReason := false
	for _, reason := range explanation.MatchReasons {
		if reason.Type == "semantic" {
			hasSemanticReason = true
		}
		if reason.Type == "tag" {
			hasTagReason = true
		}
		if reason.Type == "category" {
			hasCategoryReason = true
		}
	}
	assert.True(t, hasSemanticReason, "Should have semantic match reason")
	assert.True(t, hasTagReason, "Should have tag match reason")
	assert.True(t, hasCategoryReason, "Should have category match reason")
}

func TestExplainer_Explain_RelevanceAnalysis(t *testing.T) {
	indexer, explainer := setupExplainerTest(t)
	ctx := context.Background()

	meta := &FeatureMetadata{
		FeatureID:   "relevance_feature",
		Name:        "Product Price",
		Description: "The selling price of products",
		Category:    "pricing",
		EntityType:  "product",
		Domain:      "commerce",
		UseCase:     []string{"pricing_optimization", "revenue_analysis"},
		ValueType:   "numeric",
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	explanation, err := explainer.Explain(ctx, "relevance_feature", "product price analysis", 0.85)
	require.NoError(t, err)

	assert.NotEmpty(t, explanation.Relevance.SemanticMatch)
	assert.NotEmpty(t, explanation.Relevance.ContextualFit)
	assert.Contains(t, explanation.Relevance.ContextualFit, "product entities")
	assert.NotEmpty(t, explanation.Relevance.PotentialUseCase)
}

func TestExplainer_Explain_Recommendations(t *testing.T) {
	indexer, explainer := setupExplainerTest(t)
	ctx := context.Background()

	// Low quality feature without documentation
	meta := &FeatureMetadata{
		FeatureID:       "rec_feature",
		Name:            "Recommendation Feature",
		QualityScore:    0.3,
		Freshness:       "weekly",
		RelatedFeatures: []string{"related1", "related2", "related3", "related4"},
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	// Set low usage
	indexer.SetUsage("rec_feature", &FeatureUsage{TotalReads: 50})

	explanation, err := explainer.Explain(ctx, "rec_feature", "recommendations", 0.6)
	require.NoError(t, err)
	require.NotEmpty(t, explanation.Recommendations)

	// Should recommend quality check, freshness check, and low usage warning
	hasQualityRec := false
	hasFreshnessRec := false
	hasUsageRec := false
	hasRelatedRec := false
	for _, rec := range explanation.Recommendations {
		if containsSubstr(rec, "quality") {
			hasQualityRec = true
		}
		if containsSubstr(rec, "freshness") {
			hasFreshnessRec = true
		}
		if containsSubstr(rec, "usage") || containsSubstr(rec, "Low") {
			hasUsageRec = true
		}
		if containsSubstr(rec, "related") || containsSubstr(rec, "Related") {
			hasRelatedRec = true
		}
	}
	assert.True(t, hasQualityRec, "Should have quality recommendation")
	assert.True(t, hasFreshnessRec, "Should have freshness recommendation")
	assert.True(t, hasUsageRec, "Should have usage recommendation")
	assert.True(t, hasRelatedRec, "Should have related features recommendation")
}

func TestExplainer_ExplainResults(t *testing.T) {
	indexer, _ := setupExplainerTest(t)
	ctx := context.Background()

	// Create explainer with template fallback
	config := DefaultExplainerConfig()
	explainer := NewExplainer(indexer, nil, config)

	// Index multiple features
	features := []*FeatureMetadata{
		{FeatureID: "f1", Name: "Feature One"},
		{FeatureID: "f2", Name: "Feature Two"},
		{FeatureID: "f3", Name: "Feature Three"},
	}

	for _, f := range features {
		indexer.IndexFeatureWithMetadata(ctx, f)
	}

	// Create ranked results
	results := []RankedResult{
		{
			Feature: &EnrichedFeature{
				Metadata: features[0],
			},
			TotalScore: 0.9,
		},
		{
			Feature: &EnrichedFeature{
				Metadata: features[1],
			},
			TotalScore: 0.8,
		},
	}

	explanations, err := explainer.ExplainResults(ctx, results, "feature query")
	require.NoError(t, err)
	assert.Len(t, explanations, 2)

	for _, exp := range explanations {
		assert.NotEmpty(t, exp.Summary)
		assert.Equal(t, "feature query", exp.Query)
	}
}

func TestExplainer_ExplainResults_SkipsFailures(t *testing.T) {
	indexer, _ := setupExplainerTest(t)
	ctx := context.Background()

	config := DefaultExplainerConfig()
	explainer := NewExplainer(indexer, nil, config)

	// Only index one feature
	meta := &FeatureMetadata{
		FeatureID: "only_one",
		Name:      "Only One",
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	// Create results including one that doesn't exist
	results := []RankedResult{
		{
			Feature: &EnrichedFeature{
				Metadata: meta,
			},
			TotalScore: 0.9,
		},
		{
			Feature: &EnrichedFeature{
				Metadata: &FeatureMetadata{FeatureID: "nonexistent"},
			},
			TotalScore: 0.7,
		},
	}

	explanations, err := explainer.ExplainResults(ctx, results, "test")
	require.NoError(t, err)
	// Should only have 1 explanation (the nonexistent one is skipped)
	assert.Len(t, explanations, 1)
	assert.Equal(t, "only_one", explanations[0].FeatureID)
}

func TestTemplateExplainer_Explain(t *testing.T) {
	search := NewSearch(NewLocalEmbedder(384), nil)
	indexer := NewEnhancedIndexer(search)
	ctx := context.Background()

	templateExplainer := NewTemplateExplainer(indexer)

	// Index a feature
	meta := &FeatureMetadata{
		FeatureID:    "template_test",
		Name:         "Template Test",
		Description:  "A feature for testing templates",
		Category:     "testing",
		QualityScore: 0.95,
		DataQuality:  "high",
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	explanation, err := templateExplainer.Explain("template_test", "template test", 0.9)
	require.NoError(t, err)
	require.NotNil(t, explanation)

	assert.Equal(t, "template", explanation.GeneratedBy)
	assert.Equal(t, "template_test", explanation.FeatureID)
	assert.NotEmpty(t, explanation.Summary)
	assert.Contains(t, explanation.Summary, "Template Test")
}

func TestTemplateExplainer_Explain_NotFound(t *testing.T) {
	search := NewSearch(NewLocalEmbedder(384), nil)
	indexer := NewEnhancedIndexer(search)

	templateExplainer := NewTemplateExplainer(indexer)

	_, err := templateExplainer.Explain("nonexistent", "test", 0.5)
	assert.Error(t, err)
}

func TestExplainerConfig_Fields(t *testing.T) {
	config := ExplainerConfig{
		MaxExplanationLength: 1000,
		CacheTTL:             30 * time.Minute,
		IncludeStatistics:    false,
		IncludeLineage:       true,
		IncludeUsage:         false,
		FallbackToTemplate:   false,
	}

	assert.Equal(t, 1000, config.MaxExplanationLength)
	assert.Equal(t, 30*time.Minute, config.CacheTTL)
	assert.False(t, config.IncludeStatistics)
	assert.True(t, config.IncludeLineage)
	assert.False(t, config.IncludeUsage)
	assert.False(t, config.FallbackToTemplate)
}

func TestFeatureExplanation_Fields(t *testing.T) {
	now := time.Now()
	explanation := &FeatureExplanation{
		FeatureID: "test",
		Query:     "test query",
		Summary:   "Test summary",
		MatchReasons: []MatchReason{
			{Type: "semantic", Description: "Semantic match", Score: 0.9},
		},
		Relevance: RelevanceAnalysis{
			SemanticMatch:    "Semantic analysis",
			ContextualFit:    "Context analysis",
			PotentialUseCase: "Use case suggestion",
			Limitations:      "Some limitations",
		},
		DataInsights: &DataInsights{
			Distribution:     "normal",
			ValueRange:       "0-100",
			QualitySummary:   "high quality",
			FreshnessSummary: "real-time",
		},
		UsageContext: &UsageContext{
			Popularity:      "high",
			ModelsUsing:     []string{"model1"},
			CommonUseCases:  []string{"use1"},
			RelatedFeatures: []string{"related1"},
		},
		Recommendations: []string{"rec1", "rec2"},
		Confidence:      0.85,
		GeneratedAt:     now,
		GeneratedBy:     "template",
	}

	assert.Equal(t, "test", explanation.FeatureID)
	assert.Equal(t, "test query", explanation.Query)
	assert.Equal(t, 0.85, explanation.Confidence)
	assert.Equal(t, 1, len(explanation.MatchReasons))
	assert.NotNil(t, explanation.DataInsights)
	assert.NotNil(t, explanation.UsageContext)
	assert.Equal(t, 2, len(explanation.Recommendations))
}

func TestMatchReason_Fields(t *testing.T) {
	reason := MatchReason{
		Type:        "semantic",
		Description: "High semantic similarity",
		Score:       0.95,
	}

	assert.Equal(t, "semantic", reason.Type)
	assert.Equal(t, "High semantic similarity", reason.Description)
	assert.Equal(t, 0.95, reason.Score)
}

func TestExplainer_BuildPrompt(t *testing.T) {
	search := NewSearch(NewLocalEmbedder(384), nil)
	indexer := NewEnhancedIndexer(search)
	ctx := context.Background()

	mockLLM := &MockLLMClient{
		available: true,
		response:  "Test explanation",
	}
	config := DefaultExplainerConfig()
	explainer := NewExplainer(indexer, mockLLM, config)

	// Index feature with full metadata
	meta := &FeatureMetadata{
		FeatureID:   "prompt_test",
		Name:        "Prompt Test Feature",
		Description: "A feature for testing prompt building",
		Category:    "testing",
		EntityType:  "test",
		DataType:    "float64",
		Tags:        []string{"test", "prompt"},
	}
	indexer.IndexFeatureWithMetadata(ctx, meta)

	stats := &FeatureStatistics{
		Mean:           25.0,
		StdDev:         5.0,
		UniqueCount:    100,
		NullPercentage: 2.0,
	}
	indexer.SetStatistics("prompt_test", stats)

	usage := &FeatureUsage{
		TotalReads: 5000,
		ModelCount: 3,
	}
	indexer.SetUsage("prompt_test", usage)

	// Generate explanation to trigger prompt building
	_, err := explainer.Explain(ctx, "prompt_test", "test prompt", 0.8)
	require.NoError(t, err)

	// Verify prompt was built correctly
	prompt := mockLLM.lastPrompt
	assert.Contains(t, prompt, "Prompt Test Feature")
	assert.Contains(t, prompt, "testing prompt building")
	assert.Contains(t, prompt, "testing")
	assert.Contains(t, prompt, "test, prompt")
	assert.Contains(t, prompt, "Mean: 25.00")
	assert.Contains(t, prompt, "Total Reads: 5000")
}

func TestExplainer_DataInsightsVariousQuality(t *testing.T) {
	indexer, explainer := setupExplainerTest(t)
	ctx := context.Background()

	testCases := []struct {
		name             string
		nullPct          float64
		expectedContains string
	}{
		{"very_high", 0.5, "Very high completeness"},
		{"good", 3.0, "Good completeness"},
		{"moderate", 10.0, "Moderate completeness"},
		{"low", 25.0, "Consider data quality"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			featureID := "quality_" + tc.name
			meta := &FeatureMetadata{
				FeatureID: featureID,
				Name:      tc.name + " Quality",
			}
			indexer.IndexFeatureWithMetadata(ctx, meta)
			indexer.SetStatistics(featureID, &FeatureStatistics{
				NullPercentage: tc.nullPct,
			})

			explanation, err := explainer.Explain(ctx, featureID, "quality", 0.7)
			require.NoError(t, err)
			require.NotNil(t, explanation.DataInsights)
			assert.Contains(t, explanation.DataInsights.QualitySummary, tc.expectedContains)
		})
	}
}

func TestExplainer_UsagePopularityLevels(t *testing.T) {
	indexer, explainer := setupExplainerTest(t)
	ctx := context.Background()

	testCases := []struct {
		name             string
		reads            int64
		expectedContains string
	}{
		{"highly_popular", 150000, "Highly popular"},
		{"popular", 50000, "Popular"},
		{"moderate", 5000, "Moderately used"},
		{"low", 500, "Less commonly used"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			featureID := "pop_" + tc.name
			meta := &FeatureMetadata{
				FeatureID: featureID,
				Name:      tc.name + " Feature",
			}
			indexer.IndexFeatureWithMetadata(ctx, meta)
			indexer.SetUsage(featureID, &FeatureUsage{TotalReads: tc.reads})

			explanation, err := explainer.Explain(ctx, featureID, "popularity", 0.7)
			require.NoError(t, err)
			require.NotNil(t, explanation.UsageContext)
			assert.Contains(t, explanation.UsageContext.Popularity, tc.expectedContains)
		})
	}
}

// Helper function for substring checks in tests
func containsSubstr(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
