package marketplace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalog_PublishAndGet(t *testing.T) {
	c := NewCatalog()

	feat := &PublishedFeature{
		ID:         "feat-clicks",
		Name:       "User Click Count",
		Owner:      "ml-team",
		Team:       "recommendations",
		DataType:   "int64",
		EntityType: "user",
		Status:     FeatureStatusPublished,
		Quality:    QualityTierGold,
		Tags:       []string{"engagement", "real-time"},
	}

	err := c.Publish(feat)
	require.NoError(t, err)

	got, err := c.Get("feat-clicks")
	require.NoError(t, err)
	assert.Equal(t, "User Click Count", got.Name)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.PublishedAt.IsZero())
}

func TestCatalog_PublishValidation(t *testing.T) {
	c := NewCatalog()

	tests := []struct {
		name string
		feat *PublishedFeature
	}{
		{"no ID", &PublishedFeature{Name: "test", Owner: "x"}},
		{"no name", &PublishedFeature{ID: "test", Owner: "x"}},
		{"no owner", &PublishedFeature{ID: "test", Name: "test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Publish(tt.feat)
			require.Error(t, err)
		})
	}
}

func TestCatalog_Search(t *testing.T) {
	c := NewCatalog()

	_ = c.Publish(&PublishedFeature{
		ID: "f1", Name: "Click Count", Owner: "team-a",
		Status: FeatureStatusPublished, Tags: []string{"engagement"},
	})
	_ = c.Publish(&PublishedFeature{
		ID: "f2", Name: "Purchase Total", Owner: "team-b",
		Status: FeatureStatusPublished, Tags: []string{"revenue"},
	})
	_ = c.Publish(&PublishedFeature{
		ID: "f3", Name: "Page Views", Owner: "team-a",
		Status: FeatureStatusDraft, Tags: []string{"engagement"},
	})

	tests := []struct {
		name   string
		filter SearchFilter
		count  int
	}{
		{"all published", SearchFilter{Status: FeatureStatusPublished}, 2},
		{"by owner", SearchFilter{Owner: "team-a"}, 2},
		{"by tag", SearchFilter{Tags: []string{"revenue"}}, 1},
		{"by query", SearchFilter{Query: "click"}, 1},
		{"no match", SearchFilter{Query: "nonexistent"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := c.Search(tt.filter)
			assert.Len(t, results, tt.count)
		})
	}
}

func TestCatalog_Subscribe(t *testing.T) {
	c := NewCatalog()
	_ = c.Publish(&PublishedFeature{
		ID: "f1", Name: "Clicks", Owner: "team-a",
		Status: FeatureStatusPublished,
	})

	sub, err := c.Subscribe("f1", "user-1", "team-b")
	require.NoError(t, err)
	assert.True(t, sub.Active)

	// Duplicate subscribe returns same sub
	sub2, err := c.Subscribe("f1", "user-1", "team-b")
	require.NoError(t, err)
	assert.Equal(t, sub.ID, sub2.ID)

	subs := c.GetSubscribers("f1")
	assert.Len(t, subs, 1)

	mySubs := c.GetSubscriptions("user-1")
	assert.Len(t, mySubs, 1)
}

func TestCatalog_SubscribeUnpublished(t *testing.T) {
	c := NewCatalog()
	_ = c.Publish(&PublishedFeature{
		ID: "f1", Name: "Clicks", Owner: "team-a",
		Status: FeatureStatusDraft,
	})

	_, err := c.Subscribe("f1", "user-1", "team-b")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not published")
}

func TestCatalog_Unsubscribe(t *testing.T) {
	c := NewCatalog()
	_ = c.Publish(&PublishedFeature{
		ID: "f1", Name: "Clicks", Owner: "team-a",
		Status: FeatureStatusPublished,
	})
	_, _ = c.Subscribe("f1", "user-1", "team-b")

	err := c.Unsubscribe("f1", "user-1")
	require.NoError(t, err)

	subs := c.GetSubscribers("f1")
	assert.Empty(t, subs)
}

func TestCatalog_Deprecate(t *testing.T) {
	c := NewCatalog()
	_ = c.Publish(&PublishedFeature{
		ID: "f1", Name: "Clicks", Owner: "team-a",
		Status: FeatureStatusPublished,
	})

	err := c.Deprecate("f1")
	require.NoError(t, err)

	feat, _ := c.Get("f1")
	assert.Equal(t, FeatureStatusDeprecated, feat.Status)
	assert.False(t, feat.DeprecatedAt.IsZero())
}

func TestCatalog_Stats(t *testing.T) {
	c := NewCatalog()
	_ = c.Publish(&PublishedFeature{
		ID: "f1", Name: "A", Owner: "o", Status: FeatureStatusPublished,
	})
	_ = c.Publish(&PublishedFeature{
		ID: "f2", Name: "B", Owner: "o", Status: FeatureStatusDraft,
	})
	_, _ = c.Subscribe("f1", "u1", "t1")

	stats := c.Stats()
	assert.Equal(t, 2, stats["total_features"])
	assert.Equal(t, 1, stats["published"])
	assert.Equal(t, 1, stats["total_subscriptions"])
}
