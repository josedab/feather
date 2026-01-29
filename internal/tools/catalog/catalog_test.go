package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestService() *Service {
	return NewService(DefaultConfig())
}

func registerTestFeatures(t *testing.T, svc *Service) {
	t.Helper()
	features := []CatalogEntry{
		{
			Name: "user_click_count", Description: "Number of clicks per user",
			DataType: "int64", Entity: "user", Owner: "ml-team",
			Tags: []string{"engagement", "real-time"}, Source: "clickstream",
			Status:       FeatureStatusActive,
			Quality:      QualityScore{Overall: 0.9, Freshness: 0.95, Completeness: 0.9, Consistency: 0.85},
			Dependencies: []string{"raw_clicks"},
		},
		{
			Name: "purchase_total", Description: "Total purchase amount",
			DataType: "float64", Entity: "user", Owner: "revenue-team",
			Tags: []string{"revenue", "batch"}, Source: "transactions",
			Status:       FeatureStatusActive,
			Quality:      QualityScore{Overall: 0.85, Freshness: 0.8, Completeness: 0.9, Consistency: 0.85},
			Dependencies: []string{"raw_transactions"},
		},
		{
			Name: "session_duration", Description: "Average session duration",
			DataType: "float64", Entity: "user", Owner: "ml-team",
			Tags: []string{"engagement"}, Source: "sessions",
			Status:       FeatureStatusDeprecated,
			Quality:      QualityScore{Overall: 0.6, Freshness: 0.4, Completeness: 0.7, Consistency: 0.7},
			Dependencies: []string{"raw_sessions"},
		},
		{
			Name: "click_rate", Description: "Derived click-through rate",
			DataType: "float64", Entity: "user", Owner: "ml-team",
			Tags: []string{"engagement", "derived"}, Source: "derived",
			Status:       FeatureStatusExperimental,
			Quality:      QualityScore{Overall: 0.7, Freshness: 0.8, Completeness: 0.6, Consistency: 0.7},
			Dependencies: []string{"user_click_count", "page_views"},
		},
	}
	for _, f := range features {
		err := svc.Register(f)
		require.NoError(t, err)
	}
}

func TestService_RegisterAndGet(t *testing.T) {
	svc := newTestService()

	entry := CatalogEntry{
		Name:     "user_click_count",
		DataType: "int64",
		Owner:    "ml-team",
		Tags:     []string{"engagement"},
	}
	err := svc.Register(entry)
	require.NoError(t, err)

	got, err := svc.Get("user_click_count")
	require.NoError(t, err)
	assert.Equal(t, "user_click_count", got.Name)
	assert.Equal(t, "int64", got.DataType)
	assert.Equal(t, FeatureStatusActive, got.Status)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
}

func TestService_RegisterValidation(t *testing.T) {
	svc := newTestService()

	tests := []struct {
		name  string
		entry CatalogEntry
	}{
		{"no name", CatalogEntry{DataType: "int64", Owner: "team"}},
		{"no data type", CatalogEntry{Name: "feat", Owner: "team"}},
		{"no owner", CatalogEntry{Name: "feat", DataType: "int64"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Register(tt.entry)
			require.Error(t, err)
		})
	}
}

func TestService_RegisterUpdate(t *testing.T) {
	svc := newTestService()

	err := svc.Register(CatalogEntry{
		Name: "feat", DataType: "int64", Owner: "team-a",
		Tags: []string{"v1"},
	})
	require.NoError(t, err)
	first, _ := svc.Get("feat")
	createdAt := first.CreatedAt

	err = svc.Register(CatalogEntry{
		Name: "feat", DataType: "float64", Owner: "team-a",
		Tags: []string{"v2"},
	})
	require.NoError(t, err)

	updated, _ := svc.Get("feat")
	assert.Equal(t, createdAt, updated.CreatedAt)
	assert.Equal(t, "float64", updated.DataType)
}

func TestService_GetNotFound(t *testing.T) {
	svc := newTestService()

	_, err := svc.Get("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_Delete(t *testing.T) {
	svc := newTestService()

	_ = svc.Register(CatalogEntry{
		Name: "feat", DataType: "int64", Owner: "team",
		Tags: []string{"tag1"},
	})

	err := svc.Delete("feat")
	require.NoError(t, err)

	_, err = svc.Get("feat")
	require.Error(t, err)

	assert.Empty(t, svc.GetByTag("tag1"))
	assert.Empty(t, svc.GetByOwner("team"))
}

func TestService_DeleteNotFound(t *testing.T) {
	svc := newTestService()

	err := svc.Delete("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_Search(t *testing.T) {
	svc := newTestService()
	registerTestFeatures(t, svc)

	tests := []struct {
		name  string
		query SearchQuery
		count int
	}{
		{"by text in name", SearchQuery{Text: "click"}, 2},
		{"by text in description", SearchQuery{Text: "purchase"}, 1},
		{"case insensitive", SearchQuery{Text: "CLICK"}, 2},
		{"by owner", SearchQuery{Owner: "ml-team"}, 3},
		{"by tag", SearchQuery{Tags: []string{"revenue"}}, 1},
		{"by status", SearchQuery{Status: "deprecated"}, 1},
		{"by data type", SearchQuery{DataType: "float64"}, 3},
		{"combined filters", SearchQuery{Owner: "ml-team", Tags: []string{"engagement"}}, 3},
		{"no match", SearchQuery{Text: "nonexistent"}, 0},
		{"empty query returns all", SearchQuery{}, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.Search(tt.query)
			require.NoError(t, err)
			assert.Equal(t, tt.count, result.Total)
		})
	}
}

func TestService_SearchPagination(t *testing.T) {
	svc := newTestService()
	registerTestFeatures(t, svc)

	result, err := svc.Search(SearchQuery{Limit: 2, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, result.Entries, 2)
	assert.Equal(t, 4, result.Total)
	assert.Equal(t, 2, result.Limit)

	result2, err := svc.Search(SearchQuery{Limit: 2, Offset: 2})
	require.NoError(t, err)
	assert.Len(t, result2.Entries, 2)
	assert.Equal(t, 4, result2.Total)
}

func TestService_SearchTagsInText(t *testing.T) {
	svc := newTestService()
	_ = svc.Register(CatalogEntry{
		Name: "feat1", DataType: "int64", Owner: "team",
		Tags: []string{"important-tag"},
	})

	result, err := svc.Search(SearchQuery{Text: "important"})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
}

func TestService_ListAll(t *testing.T) {
	svc := newTestService()
	registerTestFeatures(t, svc)

	all := svc.ListAll()
	assert.Len(t, all, 4)
}

func TestService_GetByOwner(t *testing.T) {
	svc := newTestService()
	registerTestFeatures(t, svc)

	mlFeatures := svc.GetByOwner("ml-team")
	assert.Len(t, mlFeatures, 3)

	revFeatures := svc.GetByOwner("revenue-team")
	assert.Len(t, revFeatures, 1)

	none := svc.GetByOwner("unknown-team")
	assert.Empty(t, none)
}

func TestService_GetByTag(t *testing.T) {
	svc := newTestService()
	registerTestFeatures(t, svc)

	engagement := svc.GetByTag("engagement")
	assert.Len(t, engagement, 3)

	revenue := svc.GetByTag("revenue")
	assert.Len(t, revenue, 1)

	none := svc.GetByTag("nonexistent")
	assert.Empty(t, none)
}

func TestService_RecordUsage(t *testing.T) {
	svc := newTestService()
	_ = svc.Register(CatalogEntry{
		Name: "feat1", DataType: "int64", Owner: "team",
	})

	svc.RecordUsage("feat1", "consumer-a")
	svc.RecordUsage("feat1", "consumer-b")
	svc.RecordUsage("feat1", "consumer-a") // duplicate consumer

	entry, _ := svc.Get("feat1")
	assert.Equal(t, int64(3), entry.UsageCount)
	assert.False(t, entry.LastAccessed.IsZero())

	stats := svc.GetUsageStats()
	rec := stats["feat1"]
	require.NotNil(t, rec)
	assert.Equal(t, int64(3), rec.AccessCount)
	assert.Len(t, rec.Consumers, 2)
}

func TestService_RecordUsageUnknownFeature(t *testing.T) {
	svc := newTestService()

	// Should not panic on unknown feature.
	svc.RecordUsage("nonexistent", "consumer")

	stats := svc.GetUsageStats()
	assert.Empty(t, stats)
}

func TestService_GetLineage(t *testing.T) {
	svc := newTestService()
	registerTestFeatures(t, svc)

	lineage, err := svc.GetLineage("click_rate")
	require.NoError(t, err)
	assert.Equal(t, "click_rate", lineage.Root)
	assert.True(t, len(lineage.Nodes) >= 2)
	assert.True(t, len(lineage.Edges) >= 2)

	// Verify upstream nodes include dependencies.
	nodeNames := make(map[string]bool)
	for _, n := range lineage.Nodes {
		nodeNames[n.Name] = true
	}
	assert.True(t, nodeNames["user_click_count"])
	assert.True(t, nodeNames["page_views"])
}

func TestService_GetLineageNotFound(t *testing.T) {
	svc := newTestService()

	_, err := svc.GetLineage("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_GetLineageDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableLineage = false
	svc := NewService(cfg)

	_ = svc.Register(CatalogEntry{
		Name: "feat1", DataType: "int64", Owner: "team",
	})

	_, err := svc.GetLineage("feat1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestService_Stats(t *testing.T) {
	svc := newTestService()
	registerTestFeatures(t, svc)

	svc.RecordUsage("user_click_count", "consumer-a")
	svc.RecordUsage("user_click_count", "consumer-b")
	svc.RecordUsage("purchase_total", "consumer-a")

	stats := svc.Stats()
	assert.Equal(t, 4, stats.TotalFeatures)
	assert.Equal(t, 2, stats.ActiveFeatures)
	assert.Equal(t, 1, stats.DeprecatedCount)
	assert.Equal(t, 3, stats.ByOwner["ml-team"])
	assert.Equal(t, 1, stats.ByOwner["revenue-team"])
	assert.Equal(t, 2, stats.ByStatus["active"])
	assert.Equal(t, 1, stats.ByStatus["deprecated"])
	assert.Equal(t, 1, stats.ByStatus["experimental"])
	assert.Equal(t, 3, stats.ByDataType["float64"])
	assert.Equal(t, 1, stats.ByDataType["int64"])
	assert.True(t, stats.AvgQualityScore > 0)
	assert.True(t, len(stats.TopUsed) > 0)
	assert.Equal(t, "user_click_count", stats.TopUsed[0])
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, 100, cfg.MaxSearchResults)
	assert.True(t, cfg.EnableLineage)
	assert.True(t, cfg.EnableUsageTracking)
	assert.Equal(t, 90, cfg.RetentionDays)
}
