package federateddiscovery

import (
	"errors"
	"testing"
)

func TestNewCatalog(t *testing.T) {
	c := NewCatalog(DefaultCatalogConfig())
	if c == nil {
		t.Fatal("expected non-nil catalog")
	}
	entries := c.ListAll()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestPublishAndGet(t *testing.T) {
	c := NewCatalog(DefaultCatalogConfig())

	err := c.Publish(CatalogEntry{
		ID:          "feat-1",
		Name:        "user-embeddings",
		Description: "User embedding features for recommendations",
		Owner:       "ml-team",
		Quality:     0.95,
		Tags:        []string{"ml", "embeddings"},
		Schema:      map[string]string{"vector": "float[]"},
	})
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	entry, err := c.Get("feat-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if entry.Name != "user-embeddings" {
		t.Errorf("expected name user-embeddings, got %s", entry.Name)
	}
	if entry.Quality != 0.95 {
		t.Errorf("expected quality 0.95, got %f", entry.Quality)
	}

	_, err = c.Get("nonexistent")
	if !errors.Is(err, ErrCatalogNotFound) {
		t.Errorf("expected ErrCatalogNotFound, got %v", err)
	}
}

func TestSearch(t *testing.T) {
	c := NewCatalog(DefaultCatalogConfig())

	_ = c.Publish(CatalogEntry{ID: "f1", Name: "user-clicks", Tags: []string{"clickstream"}, Owner: "team-a", Quality: 0.9})
	_ = c.Publish(CatalogEntry{ID: "f2", Name: "product-views", Tags: []string{"clickstream"}, Owner: "team-b", Quality: 0.8})
	_ = c.Publish(CatalogEntry{ID: "f3", Name: "user-demographics", Tags: []string{"profile"}, Owner: "team-a", Quality: 0.7})

	// Search by tag
	results := c.Search(SearchQuery{Tags: []string{"clickstream"}})
	if len(results) != 2 {
		t.Errorf("expected 2 results for tag clickstream, got %d", len(results))
	}

	// Search by text
	results = c.Search(SearchQuery{Text: "user"})
	if len(results) != 2 {
		t.Errorf("expected 2 results for text 'user', got %d", len(results))
	}

	// Search by owner
	results = c.Search(SearchQuery{Owner: "team-a"})
	if len(results) != 2 {
		t.Errorf("expected 2 results for owner team-a, got %d", len(results))
	}
}

func TestSearchByQuality(t *testing.T) {
	c := NewCatalog(DefaultCatalogConfig())

	_ = c.Publish(CatalogEntry{ID: "q1", Name: "high-quality", Quality: 0.95})
	_ = c.Publish(CatalogEntry{ID: "q2", Name: "medium-quality", Quality: 0.7})
	_ = c.Publish(CatalogEntry{ID: "q3", Name: "low-quality", Quality: 0.3})

	results := c.Search(SearchQuery{MinQuality: 0.8})
	if len(results) != 1 {
		t.Errorf("expected 1 result with quality >= 0.8, got %d", len(results))
	}
}

func TestSubscribe(t *testing.T) {
	c := NewCatalog(DefaultCatalogConfig())

	_ = c.Publish(CatalogEntry{ID: "sub-feat", Name: "test-feature"})

	sub, err := c.Subscribe("sub-feat", "consumer-1")
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	if sub.Subscriber != "consumer-1" {
		t.Errorf("expected subscriber consumer-1, got %s", sub.Subscriber)
	}

	// Duplicate subscription
	_, err = c.Subscribe("sub-feat", "consumer-1")
	if !errors.Is(err, ErrSubscriptionExists) {
		t.Errorf("expected ErrSubscriptionExists, got %v", err)
	}

	// Subscribe to nonexistent
	_, err = c.Subscribe("nonexistent", "consumer-1")
	if !errors.Is(err, ErrCatalogNotFound) {
		t.Errorf("expected ErrCatalogNotFound, got %v", err)
	}
}

func TestUnsubscribe(t *testing.T) {
	c := NewCatalog(DefaultCatalogConfig())

	_ = c.Publish(CatalogEntry{ID: "unsub-feat", Name: "test-feature"})
	_, _ = c.Subscribe("unsub-feat", "consumer-1")

	err := c.Unsubscribe("unsub-feat", "consumer-1")
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}

	subscribers := c.GetSubscribers("unsub-feat")
	if len(subscribers) != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", len(subscribers))
	}
}

func TestGetSubscriptions(t *testing.T) {
	c := NewCatalog(DefaultCatalogConfig())

	_ = c.Publish(CatalogEntry{ID: "gs1", Name: "feature-a"})
	_ = c.Publish(CatalogEntry{ID: "gs2", Name: "feature-b"})
	_, _ = c.Subscribe("gs1", "consumer-x")
	_, _ = c.Subscribe("gs2", "consumer-x")

	entries := c.GetSubscriptions("consumer-x")
	if len(entries) != 2 {
		t.Errorf("expected 2 subscriptions, got %d", len(entries))
	}
}

func TestStats(t *testing.T) {
	c := NewCatalog(DefaultCatalogConfig())

	stats := c.Stats()
	if stats.TotalEntries != 0 {
		t.Errorf("expected 0 entries, got %d", stats.TotalEntries)
	}

	_ = c.Publish(CatalogEntry{ID: "st1", Name: "f1", Owner: "team-a", Quality: 0.9})
	_ = c.Publish(CatalogEntry{ID: "st2", Name: "f2", Owner: "team-b", Quality: 0.8})
	_, _ = c.Subscribe("st1", "sub-1")
	_, _ = c.Subscribe("st2", "sub-1")
	_, _ = c.Subscribe("st2", "sub-2")

	stats = c.Stats()
	if stats.TotalEntries != 2 {
		t.Errorf("expected 2 entries, got %d", stats.TotalEntries)
	}
	if stats.TotalSubscriptions != 3 {
		t.Errorf("expected 3 subscriptions, got %d", stats.TotalSubscriptions)
	}
	if stats.UniqueOwners != 2 {
		t.Errorf("expected 2 unique owners, got %d", stats.UniqueOwners)
	}
	if stats.UniqueSubscribers != 2 {
		t.Errorf("expected 2 unique subscribers, got %d", stats.UniqueSubscribers)
	}
	if stats.AvgQuality < 0.84 || stats.AvgQuality > 0.86 {
		t.Errorf("expected avg quality ~0.85, got %f", stats.AvgQuality)
	}
}
