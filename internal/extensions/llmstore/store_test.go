package llmstore

import (
	"errors"
	"testing"
)

func TestNewStore(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestCreateAndGetPrompt(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	p, err := s.CreatePrompt(PromptTemplate{
		ID: "summarize", Name: "Summarize", Template: "Summarize: {{text}}",
		Variables: []string{"text"}, Model: "gpt-4",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != 1 {
		t.Errorf("expected version 1, got %d", p.Version)
	}

	fetched, err := s.GetPrompt("summarize")
	if err != nil {
		t.Fatal(err)
	}
	if fetched.Template != "Summarize: {{text}}" {
		t.Errorf("unexpected template: %s", fetched.Template)
	}
}

func TestDuplicatePrompt(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	_, _ = s.CreatePrompt(PromptTemplate{ID: "p1", Name: "P1", Template: "test"})
	_, err := s.CreatePrompt(PromptTemplate{ID: "p1", Name: "P1 dup", Template: "test"})
	if !errors.Is(err, ErrPromptExists) {
		t.Fatalf("expected ErrPromptExists, got %v", err)
	}
}

func TestUpdatePrompt(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	_, _ = s.CreatePrompt(PromptTemplate{ID: "p1", Name: "P1", Template: "v1"})

	updated, err := s.UpdatePrompt(PromptTemplate{ID: "p1", Name: "P1", Template: "v2"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 {
		t.Errorf("expected version 2, got %d", updated.Version)
	}
}

func TestDeletePrompt(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	_, _ = s.CreatePrompt(PromptTemplate{ID: "p1", Name: "P1", Template: "test"})

	if err := s.DeletePrompt("p1"); err != nil {
		t.Fatal(err)
	}
	_, err := s.GetPrompt("p1")
	if !errors.Is(err, ErrPromptNotFound) {
		t.Fatalf("expected ErrPromptNotFound, got %v", err)
	}
}

func TestStoreAndGetEmbedding(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	emb, err := s.StoreEmbedding(Embedding{
		ID: "e1", Vector: []float64{0.1, 0.2, 0.3}, Text: "hello world",
	})
	if err != nil {
		t.Fatal(err)
	}
	if emb.ID != "e1" {
		t.Errorf("expected e1, got %s", emb.ID)
	}

	fetched, err := s.GetEmbedding("e1")
	if err != nil {
		t.Fatal(err)
	}
	if len(fetched.Vector) != 3 {
		t.Errorf("expected 3-dim vector, got %d", len(fetched.Vector))
	}
}

func TestSearchSimilar(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	_, _ = s.StoreEmbedding(Embedding{ID: "e1", Vector: []float64{1, 0, 0}, Text: "apple"})
	_, _ = s.StoreEmbedding(Embedding{ID: "e2", Vector: []float64{0.9, 0.1, 0}, Text: "orange"})
	_, _ = s.StoreEmbedding(Embedding{ID: "e3", Vector: []float64{0, 0, 1}, Text: "car"})

	results := s.SearchSimilar([]float64{1, 0, 0}, 2, 0.5)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "e1" {
		t.Errorf("expected e1 as top result, got %s", results[0].ID)
	}
}

func TestCosineSimilarity(t *testing.T) {
	score := cosineSimilarity([]float64{1, 0}, []float64{1, 0})
	if score < 0.99 {
		t.Errorf("expected ~1.0, got %f", score)
	}

	score = cosineSimilarity([]float64{1, 0}, []float64{0, 1})
	if score > 0.01 {
		t.Errorf("expected ~0.0, got %f", score)
	}
}

func TestCreateAndQueryRAGPipeline(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	_, _ = s.CreatePrompt(PromptTemplate{
		ID: "qa", Name: "QA", Template: "Answer based on context: {{query}}",
	})
	_, _ = s.StoreEmbedding(Embedding{ID: "doc1", Vector: []float64{1, 0}, Text: "Go is a programming language"})

	_, err := s.CreatePipeline(RAGPipeline{
		ID: "qa_pipeline", Name: "QA Pipeline",
		PromptTemplateID: "qa", EmbeddingModel: "ada-002", TopK: 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := s.QueryRAG(RAGRequest{PipelineID: "qa_pipeline", Query: "What is Go?"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.PipelineID != "qa_pipeline" {
		t.Errorf("expected qa_pipeline, got %s", resp.PipelineID)
	}
}

func TestStoreStats(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	_, _ = s.CreatePrompt(PromptTemplate{ID: "p1", Name: "P1", Template: "test"})
	_, _ = s.StoreEmbedding(Embedding{ID: "e1", Vector: []float64{1, 0}})

	stats := s.Stats()
	if stats.TotalPrompts != 1 {
		t.Errorf("expected 1 prompt, got %d", stats.TotalPrompts)
	}
	if stats.TotalEmbeddings != 1 {
		t.Errorf("expected 1 embedding, got %d", stats.TotalEmbeddings)
	}
}

func TestListPrompts(t *testing.T) {
	s := NewStore(DefaultStoreConfig())
	_, _ = s.CreatePrompt(PromptTemplate{ID: "p1", Name: "P1", Template: "t1"})
	_, _ = s.CreatePrompt(PromptTemplate{ID: "p2", Name: "P2", Template: "t2"})

	prompts := s.ListPrompts()
	if len(prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(prompts))
	}
}
