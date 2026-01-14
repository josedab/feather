package llmfeature

import (
	"testing"
	"time"
)

func TestStore_TemplateCRUD(t *testing.T) {
	s := NewStore(DefaultStoreConfig())

	tmpl := &PromptTemplate{
		ID:        "tmpl-1",
		Name:      "summarize",
		Template:  "Summarize: {{text}}",
		Variables: []string{"text"},
		Model:     "gpt-4",
	}

	// Create
	if err := s.CreateTemplate(tmpl); err != nil {
		t.Fatal(err)
	}
	if tmpl.Version != 1 {
		t.Fatalf("expected version 1, got %d", tmpl.Version)
	}

	// Duplicate
	if err := s.CreateTemplate(tmpl); err != ErrTemplateExists {
		t.Fatalf("expected exists error, got %v", err)
	}

	// Get
	got, err := s.GetTemplate("tmpl-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "summarize" {
		t.Fatalf("expected summarize, got %s", got.Name)
	}

	// Update
	tmpl.Template = "Please summarize: {{text}}"
	if err := s.UpdateTemplate(tmpl); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetTemplate("tmpl-1")
	if got.Version != 2 {
		t.Fatalf("expected version 2, got %d", got.Version)
	}

	// List
	list := s.ListTemplates()
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}

	// Delete
	if err := s.DeleteTemplate("tmpl-1"); err != nil {
		t.Fatal(err)
	}
	_, err = s.GetTemplate("tmpl-1")
	if err != ErrTemplateNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestStore_CompletionAndUsage(t *testing.T) {
	s := NewStore(DefaultStoreConfig())

	rec := &CompletionRecord{
		ID:               "comp-1",
		Prompt:           "hello",
		Completion:       "world",
		Model:            "gpt-4",
		TokensPrompt:     10,
		TokensCompletion: 20,
		TokensTotal:      30,
	}

	s.StoreCompletion(rec)

	// Retrieve
	got, err := s.GetCompletion("comp-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Completion != "world" {
		t.Fatal("wrong completion")
	}

	// Check cost was calculated
	if got.CostUSD <= 0 {
		t.Fatal("expected cost > 0")
	}

	// Usage tracking
	usage := s.GetUsage("gpt-4")
	if usage.RequestCount != 1 {
		t.Fatalf("expected 1 request, got %d", usage.RequestCount)
	}
	if usage.TotalPrompt != 10 {
		t.Fatalf("expected 10 prompt tokens, got %d", usage.TotalPrompt)
	}

	// All usage
	all := s.GetAllUsage()
	if len(all) != 1 {
		t.Fatalf("expected 1 model, got %d", len(all))
	}
}

func TestStore_CompletionExpiry(t *testing.T) {
	s := NewStore(DefaultStoreConfig())

	rec := &CompletionRecord{
		ID:        "exp-1",
		Model:     "gpt-4",
		ExpiresAt: time.Now().Add(-time.Hour), // already expired
	}
	s.StoreCompletion(rec)

	_, err := s.GetCompletion("exp-1")
	if err != ErrCompletionNotFound {
		t.Fatalf("expected not found for expired, got %v", err)
	}
}

func TestStore_CacheHitTracking(t *testing.T) {
	s := NewStore(DefaultStoreConfig())

	rec := &CompletionRecord{
		ID:               "cache-1",
		Model:            "gpt-4",
		TokensPrompt:     100,
		TokensCompletion: 200,
		TokensTotal:      300,
		CacheHit:         true,
	}
	s.StoreCompletion(rec)

	stats := s.Stats()
	if stats["cache_hits"].(int64) != 1 {
		t.Fatalf("expected 1 cache hit, got %v", stats["cache_hits"])
	}

	usage := s.GetUsage("gpt-4")
	if usage.CacheHits != 1 {
		t.Fatalf("expected 1 cache hit in usage, got %d", usage.CacheHits)
	}
	if usage.CacheSavings <= 0 {
		t.Fatal("expected cache savings > 0")
	}
}

func TestStore_Stats(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	s.CreateTemplate(&PromptTemplate{ID: "t1", Name: "test", Template: "hi"})
	s.StoreCompletion(&CompletionRecord{ID: "c1", Model: "gpt-4", TokensTotal: 100})

	stats := s.Stats()
	if stats["total_templates"] != 1 {
		t.Fatal("wrong template count")
	}
	if stats["total_completions"] != 1 {
		t.Fatal("wrong completion count")
	}
}

func TestStore_InvalidTemplate(t *testing.T) {
	s := NewStore(DefaultStoreConfig())

	err := s.CreateTemplate(&PromptTemplate{ID: "bad"})
	if err == nil {
		t.Fatal("expected error for empty name/template")
	}
}
