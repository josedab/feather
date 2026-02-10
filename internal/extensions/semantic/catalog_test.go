package semantic

import (
	"testing"
)

func TestCatalog_IndexAndSearch(t *testing.T) {
	cat := NewCatalog(DefaultCatalogConfig())

	entries := []CatalogEntry{
		{Name: "user_age", Description: "age of the user in years", EntityType: "user", DataType: "int64"},
		{Name: "user_income", Description: "annual income of the user", EntityType: "user", DataType: "float64"},
		{Name: "order_total", Description: "total order amount", EntityType: "order", DataType: "float64"},
		{Name: "user_signup_date", Description: "date when user signed up", EntityType: "user", DataType: "timestamp"},
	}

	for _, e := range entries {
		if err := cat.Index(e); err != nil {
			t.Fatalf("Index: %v", err)
		}
	}

	results := cat.Search("user age", 10)
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	// user_age should score highest
	if results[0].Entry.Name != "user_age" {
		t.Fatalf("expected user_age as top result, got %q", results[0].Entry.Name)
	}
}

func TestCatalog_SearchOrder(t *testing.T) {
	cat := NewCatalog(DefaultCatalogConfig())

	cat.Index(CatalogEntry{Name: "purchase_amount", Description: "dollar amount of purchase"})
	cat.Index(CatalogEntry{Name: "purchase_count", Description: "number of purchases"})
	cat.Index(CatalogEntry{Name: "user_name", Description: "name of the user"})

	results := cat.Search("purchase", 10)
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	// Both purchase features should rank above user_name
	for _, r := range results[:2] {
		if r.Entry.Name == "user_name" {
			t.Fatal("user_name should not be in top 2 for 'purchase' query")
		}
	}
}

func TestCatalog_DetectDuplicates(t *testing.T) {
	cfg := DefaultCatalogConfig()
	cfg.DuplicateThreshold = 0.7
	cat := NewCatalog(cfg)

	// Very similar features
	cat.Index(CatalogEntry{Name: "user_total_spend", Description: "total spending by user"})
	cat.Index(CatalogEntry{Name: "user_total_spending", Description: "total spend amount for user"})
	// Different feature
	cat.Index(CatalogEntry{Name: "order_count", Description: "number of orders"})

	dupes := cat.DetectDuplicates()
	if len(dupes) == 0 {
		t.Fatal("expected at least one duplicate candidate")
	}

	found := false
	for _, d := range dupes {
		if (d.FeatureA == "user_total_spend" && d.FeatureB == "user_total_spending") ||
			(d.FeatureA == "user_total_spending" && d.FeatureB == "user_total_spend") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected duplicate candidate between user_total_spend and user_total_spending")
	}
}

func TestCatalog_Get(t *testing.T) {
	cat := NewCatalog(DefaultCatalogConfig())
	cat.Index(CatalogEntry{Name: "test_feature"})

	entry, err := cat.Get("test_feature")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry.Name != "test_feature" {
		t.Fatalf("expected 'test_feature', got %q", entry.Name)
	}

	_, err = cat.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent feature")
	}
}

func TestCatalog_List(t *testing.T) {
	cat := NewCatalog(DefaultCatalogConfig())
	cat.Index(CatalogEntry{Name: "a"})
	cat.Index(CatalogEntry{Name: "b"})

	list := cat.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
}

func TestCatalog_RecordUsage(t *testing.T) {
	cat := NewCatalog(DefaultCatalogConfig())
	cat.Index(CatalogEntry{Name: "f1"})
	cat.RecordUsage("f1")
	cat.RecordUsage("f1")

	entry, _ := cat.Get("f1")
	if entry.UsageCount != 2 {
		t.Fatalf("expected usage count 2, got %d", entry.UsageCount)
	}
}

func TestCatalog_Stats(t *testing.T) {
	cat := NewCatalog(DefaultCatalogConfig())
	cat.Index(CatalogEntry{Name: "a", Group: "g1"})
	cat.Index(CatalogEntry{Name: "b", Group: "g1"})
	cat.Index(CatalogEntry{Name: "c", Group: "g2"})

	stats := cat.Stats()
	if stats.TotalEntries != 3 {
		t.Fatalf("expected 3 entries, got %d", stats.TotalEntries)
	}
	if stats.TotalGroups != 2 {
		t.Fatalf("expected 2 groups, got %d", stats.TotalGroups)
	}
}

func TestCatalog_EmptyName(t *testing.T) {
	cat := NewCatalog(DefaultCatalogConfig())
	err := cat.Index(CatalogEntry{})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCatalog_MaxEntries(t *testing.T) {
	cfg := DefaultCatalogConfig()
	cfg.MaxEntries = 2
	cat := NewCatalog(cfg)

	cat.Index(CatalogEntry{Name: "a"})
	cat.Index(CatalogEntry{Name: "b"})
	err := cat.Index(CatalogEntry{Name: "c"})
	if err == nil {
		t.Fatal("expected capacity error")
	}
}

func TestCatalogCosineSimilarity(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{1, 0, 0}
	if sim := catalogCosineSimilarity(a, b); sim < 0.99 {
		t.Fatalf("expected ~1.0, got %f", sim)
	}

	c := []float64{0, 1, 0}
	if sim := catalogCosineSimilarity(a, c); sim > 0.01 {
		t.Fatalf("expected ~0.0, got %f", sim)
	}
}

func TestLevenshteinRatio(t *testing.T) {
	if r := levenshteinRatio("hello", "hello"); r != 1.0 {
		t.Fatalf("expected 1.0, got %f", r)
	}
	if r := levenshteinRatio("abc", "xyz"); r > 0.5 {
		t.Fatalf("expected low ratio, got %f", r)
	}
}
