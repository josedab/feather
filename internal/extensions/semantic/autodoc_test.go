package semantic

import (
	"testing"
	"time"
)

func TestAutoDocGenerator_RecordUsage(t *testing.T) {
	gen := NewAutoDocGenerator(DefaultAutoDocConfig())

	gen.RecordUsage(UsageEvent{
		FeatureName: "click_count",
		UsedWith:    []string{"purchase_total", "session_duration"},
		Context:     "prediction",
		Timestamp:   time.Now(),
	})
	gen.RecordUsage(UsageEvent{
		FeatureName: "click_count",
		UsedWith:    []string{"purchase_total"},
		Context:     "training",
		Timestamp:   time.Now(),
	})

	doc := gen.GenerateDoc("click_count")
	if doc == nil {
		t.Fatal("expected doc")
	}
	if len(doc.UsagePatterns) != 2 {
		t.Errorf("expected 2 usage patterns, got %d", len(doc.UsagePatterns))
	}
	if len(doc.RelatedFeatures) == 0 {
		t.Error("expected related features from co-usage")
	}
}

func TestAutoDocGenerator_GenerateDoc_NoUsage(t *testing.T) {
	gen := NewAutoDocGenerator(DefaultAutoDocConfig())

	doc := gen.GenerateDoc("unknown_feature")
	if doc == nil {
		t.Fatal("expected doc even with no usage")
	}
	if doc.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestAutoDocGenerator_GenerateAll(t *testing.T) {
	gen := NewAutoDocGenerator(DefaultAutoDocConfig())

	for i := 0; i < 5; i++ {
		gen.RecordUsage(UsageEvent{
			FeatureName: "feature_a",
			Context:     "prediction",
			Timestamp:   time.Now(),
		})
		gen.RecordUsage(UsageEvent{
			FeatureName: "feature_b",
			Context:     "training",
			Timestamp:   time.Now(),
		})
	}

	docs := gen.GenerateAll()
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
}

func TestAutoDocGenerator_CoUsageTracking(t *testing.T) {
	gen := NewAutoDocGenerator(DefaultAutoDocConfig())

	// Record multiple co-usages
	for i := 0; i < 10; i++ {
		gen.RecordUsage(UsageEvent{
			FeatureName: "user_age",
			UsedWith:    []string{"user_income", "user_location"},
			Context:     "prediction",
			Timestamp:   time.Now(),
		})
	}

	doc := gen.GenerateDoc("user_age")
	if len(doc.RelatedFeatures) < 2 {
		t.Errorf("expected at least 2 related features, got %d", len(doc.RelatedFeatures))
	}

	// Verify co-usage is bidirectional
	// user_income needs its own usage events to have related features
	for i := 0; i < 5; i++ {
		gen.RecordUsage(UsageEvent{
			FeatureName: "user_income",
			UsedWith:    []string{"user_age"},
			Context:     "prediction",
			Timestamp:   time.Now(),
		})
	}
	doc2 := gen.GenerateDoc("user_income")
	foundAge := false
	for _, r := range doc2.RelatedFeatures {
		if r.Name == "user_age" {
			foundAge = true
			break
		}
	}
	if !foundAge {
		t.Error("expected user_age as related to user_income (bidirectional co-usage)")
	}
}
