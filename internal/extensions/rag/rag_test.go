package rag

import (
	"context"
	"fmt"
	"math"
	"testing"
)

// --- Chunker Tests ---

func TestChunkerFixedSize(t *testing.T) {
	c := NewChunker(ChunkBySize, 10, 0)
	chunks := c.Chunk("abcdefghijklmnopqrstuvwxyz")

	if len(chunks) == 0 {
		t.Fatal("expected chunks, got none")
	}

	// First chunk should be 10 chars.
	if len([]rune(chunks[0].Content)) != 10 {
		t.Errorf("expected first chunk length 10, got %d", len([]rune(chunks[0].Content)))
	}

	// Verify all content is covered.
	total := ""
	for _, ch := range chunks {
		total += ch.Content
	}
	if total != "abcdefghijklmnopqrstuvwxyz" {
		t.Errorf("chunk content doesn't reconstruct original: got %q", total)
	}

	// Test with overlap.
	c2 := NewChunker(ChunkBySize, 10, 3)
	chunks2 := c2.Chunk("abcdefghijklmnopqrstuvwxyz")
	if len(chunks2) < 2 {
		t.Fatal("expected multiple overlapping chunks")
	}

	// Verify overlap: end of chunk 0 should match start of chunk 1.
	r0 := []rune(chunks2[0].Content)
	r1 := []rune(chunks2[1].Content)
	overlap0 := string(r0[len(r0)-3:])
	overlap1 := string(r1[:3])
	if overlap0 != overlap1 {
		t.Errorf("expected overlap %q == %q", overlap0, overlap1)
	}

	// Test empty input.
	if chunks := c.Chunk(""); chunks != nil {
		t.Errorf("expected nil for empty input, got %d chunks", len(chunks))
	}
}

func TestChunkerSentence(t *testing.T) {
	c := NewChunker(ChunkBySentence, 1000, 0)
	text := "First sentence. Second sentence! Third sentence? Fourth."
	chunks := c.Chunk(text)

	if len(chunks) == 0 {
		t.Fatal("expected chunks from sentence splitting")
	}

	// All content should be present.
	found := make(map[string]bool)
	for _, ch := range chunks {
		if contains(ch.Content, "First sentence.") {
			found["first"] = true
		}
		if contains(ch.Content, "Second sentence!") {
			found["second"] = true
		}
		if contains(ch.Content, "Third sentence?") {
			found["third"] = true
		}
	}

	for _, key := range []string{"first", "second", "third"} {
		if !found[key] {
			t.Errorf("missing %s sentence in chunks", key)
		}
	}

	// Test that small chunk size forces splitting.
	c2 := NewChunker(ChunkBySentence, 30, 0)
	chunks2 := c2.Chunk(text)
	if len(chunks2) < 2 {
		t.Errorf("expected multiple chunks with small size, got %d", len(chunks2))
	}
}

func TestChunkerParagraph(t *testing.T) {
	c := NewChunker(ChunkByParagraph, 1000, 0)
	text := "First paragraph content here.\n\nSecond paragraph content here.\n\nThird paragraph."
	chunks := c.Chunk(text)

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks for 3 paragraphs, got %d", len(chunks))
	}

	if chunks[0].Content != "First paragraph content here." {
		t.Errorf("unexpected first paragraph: %q", chunks[0].Content)
	}
	if chunks[1].Content != "Second paragraph content here." {
		t.Errorf("unexpected second paragraph: %q", chunks[1].Content)
	}
	if chunks[2].Content != "Third paragraph." {
		t.Errorf("unexpected third paragraph: %q", chunks[2].Content)
	}

	// Verify indexes.
	for i, ch := range chunks {
		if ch.Index != i {
			t.Errorf("chunk %d has index %d", i, ch.Index)
		}
	}
}

// --- Indexer Tests ---

func TestIndexerAddAndSearch(t *testing.T) {
	idx := NewIndexer(3, "cosine")

	// Add vectors.
	if err := idx.Add("v1", []float32{1, 0, 0}); err != nil {
		t.Fatalf("adding v1: %v", err)
	}
	if err := idx.Add("v2", []float32{0, 1, 0}); err != nil {
		t.Fatalf("adding v2: %v", err)
	}
	if err := idx.Add("v3", []float32{1, 1, 0}); err != nil {
		t.Fatalf("adding v3: %v", err)
	}

	if idx.Count() != 3 {
		t.Errorf("expected count 3, got %d", idx.Count())
	}

	// Search for vector close to v1.
	results := idx.Search([]float32{1, 0.1, 0}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "v1" {
		t.Errorf("expected v1 as top result, got %s", results[0].ID)
	}

	// v1 should have high score (close to 1.0).
	if results[0].Score < 0.9 {
		t.Errorf("expected high similarity for v1, got %f", results[0].Score)
	}

	// Test dimension mismatch.
	if err := idx.Add("bad", []float32{1, 2}); err == nil {
		t.Error("expected dimension mismatch error")
	}

	// Test empty vector.
	if err := idx.Add("empty", []float32{}); err == nil {
		t.Error("expected empty vector error")
	}

	// Test delete.
	if err := idx.Delete("v2"); err != nil {
		t.Fatalf("deleting v2: %v", err)
	}
	if idx.Count() != 2 {
		t.Errorf("expected count 2 after delete, got %d", idx.Count())
	}

	// Test search with topK > count.
	results = idx.Search([]float32{1, 0, 0}, 10)
	if len(results) != 2 {
		t.Errorf("expected 2 results (all remaining), got %d", len(results))
	}
}

// --- Embedder Tests ---

func TestLocalEmbedder(t *testing.T) {
	ctx := context.Background()
	emb := NewLocalEmbedder(64)

	if emb.Dimension() != 64 {
		t.Errorf("expected dimension 64, got %d", emb.Dimension())
	}

	// Single embed.
	vec, err := emb.Embed(ctx, "hello world test")
	if err != nil {
		t.Fatalf("embedding: %v", err)
	}
	if len(vec) != 64 {
		t.Fatalf("expected 64-dim vector, got %d", len(vec))
	}

	// Verify L2-normalized (magnitude ≈ 1).
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if math.Abs(norm-1.0) > 0.01 {
		t.Errorf("expected normalized vector (norm ≈ 1), got %f", norm)
	}

	// Same text should produce identical embeddings.
	vec2, _ := emb.Embed(ctx, "hello world test")
	for i := range vec {
		if vec[i] != vec2[i] {
			t.Fatal("same text produced different embeddings")
		}
	}

	// Different text should produce different embeddings.
	vec3, _ := emb.Embed(ctx, "completely different content xyz")
	same := true
	for i := range vec {
		if vec[i] != vec3[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different texts produced identical embeddings")
	}

	// Batch embed.
	texts := []string{"hello world", "foo bar", "baz qux"}
	vecs, err := emb.EmbedBatch(ctx, texts)
	if err != nil {
		t.Fatalf("batch embedding: %v", err)
	}
	if len(vecs) != 3 {
		t.Fatalf("expected 3 embeddings, got %d", len(vecs))
	}
	for i, v := range vecs {
		if len(v) != 64 {
			t.Errorf("batch embedding %d: expected 64-dim, got %d", i, len(v))
		}
	}
}

// --- Pipeline Tests ---

func TestPipelineIngest(t *testing.T) {
	ctx := context.Background()
	p := NewPipeline(DefaultPipelineConfig())
	defer p.Close()

	doc := &Document{
		Content:  "This is a test document with enough content to be chunked. It has multiple sentences. And some more text here for good measure.",
		Metadata: map[string]string{"type": "test"},
		Source:   "unit-test",
	}

	if err := p.Ingest(ctx, doc); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	if doc.ID == "" {
		t.Error("expected document ID to be assigned")
	}
	if len(doc.Chunks) == 0 {
		t.Error("expected chunks to be created")
	}

	// Verify chunks have embeddings.
	for i, ch := range doc.Chunks {
		if ch.Embedding == nil {
			t.Errorf("chunk %d has no embedding", i)
		}
		if ch.DocumentID != doc.ID {
			t.Errorf("chunk %d has wrong document ID: %s", i, ch.DocumentID)
		}
	}

	// Verify stats.
	stats := p.Stats()
	if stats.DocumentCount != 1 {
		t.Errorf("expected 1 document, got %d", stats.DocumentCount)
	}
	if stats.IndexSize == 0 {
		t.Error("expected non-zero index size")
	}

	// Verify GetDocument.
	got, err := p.GetDocument(ctx, doc.ID)
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if got.Content != doc.Content {
		t.Error("retrieved document content mismatch")
	}

	// Verify ListDocuments.
	docs := p.ListDocuments(ctx)
	if len(docs) != 1 {
		t.Errorf("expected 1 document in list, got %d", len(docs))
	}

	// Test nil document.
	if err := p.Ingest(ctx, nil); err != ErrNilDocument {
		t.Errorf("expected ErrNilDocument, got %v", err)
	}

	// Test empty content.
	if err := p.Ingest(ctx, &Document{Content: ""}); err != ErrEmptyContent {
		t.Errorf("expected ErrEmptyContent, got %v", err)
	}
}

func TestPipelineRetrieve(t *testing.T) {
	ctx := context.Background()
	p := NewPipeline(DefaultPipelineConfig())
	defer p.Close()

	// Ingest documents with distinct topics.
	docs := []*Document{
		{Content: "Machine learning algorithms process data and learn patterns from examples. Neural networks are a type of machine learning model."},
		{Content: "Cooking recipes require ingredients and step by step instructions. Baking bread needs flour water and yeast."},
		{Content: "Deep learning uses neural networks with many layers. Training deep models requires large datasets and compute resources."},
	}

	for i, doc := range docs {
		if err := p.Ingest(ctx, doc); err != nil {
			t.Fatalf("ingesting doc %d: %v", i, err)
		}
	}

	// Query about ML should rank ML documents higher.
	result, err := p.Retrieve(ctx, "machine learning neural network", 3)
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}

	if result.TotalHits == 0 {
		t.Fatal("expected at least one hit")
	}

	if result.Query != "machine learning neural network" {
		t.Errorf("expected query in result, got %q", result.Query)
	}

	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}

	// The top result should be from an ML-related document.
	topChunk := result.Chunks[0].Chunk
	if !contains(topChunk.Content, "learn") && !contains(topChunk.Content, "neural") && !contains(topChunk.Content, "machine") {
		t.Errorf("top result not ML-related: %q", topChunk.Content)
	}
}

func TestPipelineDelete(t *testing.T) {
	ctx := context.Background()
	p := NewPipeline(DefaultPipelineConfig())
	defer p.Close()

	doc := &Document{Content: "Document to be deleted. It has content that will be removed."}
	if err := p.Ingest(ctx, doc); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	docID := doc.ID
	chunkCount := len(doc.Chunks)

	// Verify it exists.
	if _, err := p.GetDocument(ctx, docID); err != nil {
		t.Fatalf("document should exist: %v", err)
	}

	// Delete it.
	if err := p.Delete(ctx, docID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Verify it's gone.
	if _, err := p.GetDocument(ctx, docID); err == nil {
		t.Error("expected error for deleted document")
	}

	// Verify chunks removed from index.
	stats := p.Stats()
	if stats.DocumentCount != 0 {
		t.Errorf("expected 0 documents, got %d", stats.DocumentCount)
	}
	if stats.IndexSize != 0 {
		t.Errorf("expected 0 index entries, got %d (had %d chunks)", stats.IndexSize, chunkCount)
	}

	// Delete non-existent.
	if err := p.Delete(ctx, "nonexistent"); err == nil {
		t.Error("expected error deleting non-existent document")
	}
}

func TestPipelineBatchIngest(t *testing.T) {
	ctx := context.Background()
	p := NewPipeline(DefaultPipelineConfig())
	defer p.Close()

	docs := make([]*Document, 5)
	for i := range docs {
		docs[i] = &Document{
			Content:  fmt.Sprintf("Batch document number %d with unique content about topic %d.", i, i),
			Metadata: map[string]string{"batch": "true", "index": fmt.Sprintf("%d", i)},
		}
	}

	if err := p.IngestBatch(ctx, docs); err != nil {
		t.Fatalf("batch ingest: %v", err)
	}

	stats := p.Stats()
	if stats.DocumentCount != 5 {
		t.Errorf("expected 5 documents, got %d", stats.DocumentCount)
	}

	// Verify all documents have IDs and chunks.
	for i, doc := range docs {
		if doc.ID == "" {
			t.Errorf("doc %d has no ID", i)
		}
		if len(doc.Chunks) == 0 {
			t.Errorf("doc %d has no chunks", i)
		}
	}

	// Verify metadata propagation.
	for _, doc := range docs {
		for _, ch := range doc.Chunks {
			if ch.Metadata["batch"] != "true" {
				t.Error("metadata not propagated to chunk")
			}
		}
	}

	// Verify retrieval across batch.
	result, err := p.Retrieve(ctx, "document topic", 3)
	if err != nil {
		t.Fatalf("retrieve after batch: %v", err)
	}
	if result.TotalHits == 0 {
		t.Error("expected results from batch-ingested documents")
	}
}

// contains checks if s contains substr (case-sensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
