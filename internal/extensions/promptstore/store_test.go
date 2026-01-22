package promptstore

import (
	"testing"
)

func TestNewStore(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestCreateAndGet(t *testing.T) {
	s := NewStore(DefaultStoreConfig())

	tmpl, err := s.Create(PromptTemplate{
		ID:       "greeting",
		Name:     "Greeting Prompt",
		Template: "Hello {{name}}, welcome to {{service}}!",
		Model:    "gpt-4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.Version != 1 {
		t.Errorf("expected version 1, got %d", tmpl.Version)
	}
	if len(tmpl.Variables) != 2 {
		t.Errorf("expected 2 variables, got %d", len(tmpl.Variables))
	}

	got, err := s.Get("greeting")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "greeting" {
		t.Errorf("expected ID 'greeting', got %q", got.ID)
	}
}

func TestCreateDuplicate(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	_, _ = s.Create(PromptTemplate{ID: "test", Template: "hello"})
	_, err := s.Create(PromptTemplate{ID: "test", Template: "hello"})
	if err != ErrPromptExists {
		t.Fatalf("expected ErrPromptExists, got %v", err)
	}
}

func TestUpdateVersioning(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	_, _ = s.Create(PromptTemplate{ID: "test", Template: "v1"})

	updated, err := s.Update("test", PromptTemplate{Template: "v2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("expected version 2, got %d", updated.Version)
	}

	versions, _ := s.ListVersions("test")
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}
}

func TestRender(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	_, _ = s.Create(PromptTemplate{
		ID:       "greeting",
		Template: "Hello {{name}}!",
	})

	result, err := s.Render("greeting", map[string]string{"name": "World"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rendered != "Hello World!" {
		t.Errorf("expected 'Hello World!', got %q", result.Rendered)
	}
	if result.TokenEstimate <= 0 {
		t.Error("expected positive token estimate")
	}
}

func TestDeletePrompt(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	_, _ = s.Create(PromptTemplate{ID: "test", Template: "hello"})

	if err := s.Delete("test"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("test"); err != ErrPromptNotFound {
		t.Fatalf("expected ErrPromptNotFound, got %v", err)
	}
}

func TestUsageTracking(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	_, _ = s.Create(PromptTemplate{ID: "test", Template: "hello"})

	s.RecordUsage("test", 1, 100, 50.0, nil)
	s.RecordUsage("test", 1, 200, 30.0, nil)
	s.RecordScore("test", 1, 0.95)

	usage := s.GetUsage("test")
	if len(usage) != 1 {
		t.Fatalf("expected 1 usage entry, got %d", len(usage))
	}
	if usage[0].Invocations != 2 {
		t.Errorf("expected 2 invocations, got %d", usage[0].Invocations)
	}
	if usage[0].TotalTokens != 300 {
		t.Errorf("expected 300 tokens, got %d", usage[0].TotalTokens)
	}
}

func TestStats(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	_, _ = s.Create(PromptTemplate{ID: "p1", Template: "hello"})
	_, _ = s.Create(PromptTemplate{ID: "p2", Template: "world"})

	stats := s.Stats()
	if stats.TotalPrompts != 2 {
		t.Errorf("expected 2 prompts, got %d", stats.TotalPrompts)
	}
}
