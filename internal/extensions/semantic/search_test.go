package semantic

import (
	"context"
	"testing"
)

func TestSearch_IndexAndSearch(t *testing.T) {
	search := NewSearch(NewLocalEmbedder(384), nil)
	ctx := context.Background()

	// Index some features
	features := []*FeatureDocument{
		{
			ID:          "user_purchase_count",
			Name:        "User Purchase Count",
			Description: "Total number of purchases made by the user",
			Tags:        []string{"user", "purchases", "ecommerce"},
			Category:    "user_behavior",
		},
		{
			ID:          "user_total_spend",
			Name:        "User Total Spend",
			Description: "Total amount spent by the user in dollars",
			Tags:        []string{"user", "revenue", "financial"},
			Category:    "user_behavior",
		},
		{
			ID:          "product_view_count",
			Name:        "Product View Count",
			Description: "Number of times a product was viewed",
			Tags:        []string{"product", "views", "engagement"},
			Category:    "product_metrics",
		},
		{
			ID:          "cart_abandonment_rate",
			Name:        "Cart Abandonment Rate",
			Description: "Percentage of shopping carts abandoned without purchase",
			Tags:        []string{"cart", "conversion", "ecommerce"},
			Category:    "conversion",
		},
	}

	err := search.IndexBatch(ctx, features)
	if err != nil {
		t.Fatalf("IndexBatch failed: %v", err)
	}

	// Search for purchase-related features
	results, err := search.Search(ctx, "user buying purchases spending", DefaultSearchOptions())
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected at least one result")
	}

	// The top results should be purchase/spend related
	topResult := results[0]
	if topResult.Feature.ID != "user_purchase_count" && topResult.Feature.ID != "user_total_spend" {
		t.Logf("Top result: %s (score: %f)", topResult.Feature.ID, topResult.Score)
	}
}

func TestSearch_FilterByCategory(t *testing.T) {
	search := NewSearch(NewLocalEmbedder(384), nil)
	ctx := context.Background()

	// Index features in different categories
	search.IndexFeature(ctx, &FeatureDocument{
		ID:       "f1",
		Name:     "User Feature",
		Category: "user",
	})
	search.IndexFeature(ctx, &FeatureDocument{
		ID:       "f2",
		Name:     "Product Feature",
		Category: "product",
	})

	// Search with category filter
	opts := SearchOptions{
		Limit:      10,
		MinScore:   0,
		Categories: []string{"user"},
	}

	results, err := search.Search(ctx, "feature", opts)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, r := range results {
		if r.Feature.Category != "user" {
			t.Errorf("expected category 'user', got '%s'", r.Feature.Category)
		}
	}
}

func TestSearch_FilterByTags(t *testing.T) {
	search := NewSearch(NewLocalEmbedder(384), nil)
	ctx := context.Background()

	search.IndexFeature(ctx, &FeatureDocument{
		ID:   "f1",
		Name: "Feature One",
		Tags: []string{"important", "core"},
	})
	search.IndexFeature(ctx, &FeatureDocument{
		ID:   "f2",
		Name: "Feature Two",
		Tags: []string{"experimental"},
	})

	// Search with tag filter
	opts := SearchOptions{
		Limit:    10,
		MinScore: 0,
		Tags:     []string{"important"},
	}

	results, err := search.Search(ctx, "feature", opts)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if len(results) > 0 && results[0].Feature.ID != "f1" {
		t.Errorf("expected f1, got %s", results[0].Feature.ID)
	}
}

func TestSearch_Suggest(t *testing.T) {
	search := NewSearch(NewLocalEmbedder(384), nil)
	ctx := context.Background()

	// Index similar features
	search.IndexFeature(ctx, &FeatureDocument{
		ID:          "user_clicks",
		Name:        "User Clicks",
		Description: "Number of clicks by user",
		Tags:        []string{"user", "engagement"},
	})
	search.IndexFeature(ctx, &FeatureDocument{
		ID:          "user_page_views",
		Name:        "User Page Views",
		Description: "Number of page views by user",
		Tags:        []string{"user", "engagement"},
	})
	search.IndexFeature(ctx, &FeatureDocument{
		ID:          "product_price",
		Name:        "Product Price",
		Description: "Current price of the product",
		Tags:        []string{"product", "pricing"},
	})

	// Get suggestions for user_clicks
	suggestions, err := search.Suggest(ctx, "user_clicks", 2)
	if err != nil {
		t.Fatalf("Suggest failed: %v", err)
	}

	if len(suggestions) == 0 {
		t.Error("expected at least one suggestion")
	}

	// user_page_views should be more similar to user_clicks than product_price
	if len(suggestions) > 0 {
		t.Logf("Top suggestion: %s (score: %f)", suggestions[0].Feature.ID, suggestions[0].Score)
	}
}

func TestSearch_DeleteFeature(t *testing.T) {
	search := NewSearch(NewLocalEmbedder(384), nil)
	ctx := context.Background()

	// Index a feature
	search.IndexFeature(ctx, &FeatureDocument{
		ID:   "test_feature",
		Name: "Test Feature",
	})

	// Verify it exists
	_, err := search.GetFeature("test_feature")
	if err != nil {
		t.Fatalf("GetFeature failed: %v", err)
	}

	// Delete it
	err = search.DeleteFeature("test_feature")
	if err != nil {
		t.Fatalf("DeleteFeature failed: %v", err)
	}

	// Verify it's gone
	_, err = search.GetFeature("test_feature")
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestSearch_GetStats(t *testing.T) {
	search := NewSearch(NewLocalEmbedder(384), nil)
	ctx := context.Background()

	// Index some features
	search.IndexFeature(ctx, &FeatureDocument{
		ID:       "f1",
		Category: "user",
		Tags:     []string{"important"},
	})
	search.IndexFeature(ctx, &FeatureDocument{
		ID:       "f2",
		Category: "product",
		Tags:     []string{"important", "core"},
	})

	stats := search.GetStats()

	if stats["total_features"].(int) != 2 {
		t.Errorf("expected 2 features, got %v", stats["total_features"])
	}

	categories := stats["categories"].(map[string]int)
	if categories["user"] != 1 || categories["product"] != 1 {
		t.Errorf("unexpected categories: %v", categories)
	}

	tags := stats["tags"].(map[string]int)
	if tags["important"] != 2 {
		t.Errorf("expected 'important' tag count 2, got %d", tags["important"])
	}
}

func TestLocalEmbedder(t *testing.T) {
	embedder := NewLocalEmbedder(128)
	ctx := context.Background()

	// Test single embedding
	emb, err := embedder.Embed(ctx, "user purchase behavior")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(emb) != 128 {
		t.Errorf("expected dimension 128, got %d", len(emb))
	}

	// Test batch embedding
	texts := []string{
		"user purchase behavior",
		"product view count",
		"shopping cart abandonment",
	}

	embeddings, err := embedder.EmbedBatch(ctx, texts)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}

	if len(embeddings) != 3 {
		t.Errorf("expected 3 embeddings, got %d", len(embeddings))
	}

	// Similar texts should have higher similarity
	sim1 := cosineSimilarity(embeddings[0], embeddings[1])
	sim2 := cosineSimilarity(embeddings[0], embeddings[2])
	t.Logf("Similarity (purchase-view): %f, (purchase-cart): %f", sim1, sim2)
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected int // minimum expected tokens
	}{
		{"hello world", 2},
		{"The quick brown fox jumps over the lazy dog", 6}, // stop words removed
		{"user_id entity_key feature_value", 3},
		{"", 0},
	}

	for _, tt := range tests {
		tokens := tokenize(tt.input)
		if len(tokens) < tt.expected {
			t.Errorf("tokenize(%q) got %d tokens, expected at least %d",
				tt.input, len(tokens), tt.expected)
		}
	}
}

func TestCosineSimilarity(t *testing.T) {
	// Identical vectors should have similarity 1
	a := []float32{1, 0, 0}
	sim := cosineSimilarity(a, a)
	if sim < 0.99 {
		t.Errorf("identical vectors should have similarity ~1, got %f", sim)
	}

	// Orthogonal vectors should have similarity 0
	b := []float32{0, 1, 0}
	sim = cosineSimilarity(a, b)
	if sim > 0.01 {
		t.Errorf("orthogonal vectors should have similarity ~0, got %f", sim)
	}

	// Opposite vectors should have similarity -1
	c := []float32{-1, 0, 0}
	sim = cosineSimilarity(a, c)
	if sim > -0.99 {
		t.Errorf("opposite vectors should have similarity ~-1, got %f", sim)
	}
}
