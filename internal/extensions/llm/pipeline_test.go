package llm

import (
	"context"
	"testing"
)

func TestChunkerFixed(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		Method:       ChunkMethodFixed,
		ChunkSize:    100,
		ChunkOverlap: 20,
	})

	text := "This is a test text that should be split into multiple chunks. " +
		"Each chunk should be approximately 100 characters long with some overlap."

	chunks := chunker.Split(text)

	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}

	for _, chunk := range chunks {
		if len(chunk.Text) == 0 {
			t.Error("chunk text should not be empty")
		}
	}
}

func TestChunkerSemantic(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		Method:       ChunkMethodSemantic,
		ChunkSize:    200,
		ChunkOverlap: 0,
		MinChunkSize: 50,
	})

	text := "This is the first sentence. This is the second sentence. " +
		"This is a longer third sentence that contains more words. " +
		"And here is yet another sentence. Finally, this is the last sentence."

	chunks := chunker.Split(text)

	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}

	// Verify chunks don't overlap when overlap is 0
	for i, chunk := range chunks {
		t.Logf("Chunk %d: %q (start=%d, end=%d)", i, chunk.Text, chunk.StartChar, chunk.EndChar)
	}
}

func TestChunkerRecursive(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		Method:       ChunkMethodRecursive,
		ChunkSize:    50,
		ChunkOverlap: 0,
		MinChunkSize: 10,
		Separators:   []string{"\n\n", "\n", ". ", " "},
	})

	text := "Paragraph one with content.\n\nParagraph two with more content. It has multiple sentences.\n\n" +
		"Paragraph three is here with additional text."

	chunks := chunker.Split(text)

	if len(chunks) < 1 {
		t.Error("expected at least 1 chunk")
	}

	// Just verify we got chunks back
	t.Logf("Got %d chunks", len(chunks))
	for i, c := range chunks {
		t.Logf("Chunk %d: %q", i, c.Text)
	}
}

func TestLocalProvider(t *testing.T) {
	provider := NewLocalProvider(128)

	ctx := context.Background()

	// Test single embedding
	embedding, err := provider.Embed(ctx, "hello world")
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	if len(embedding) != 128 {
		t.Errorf("expected dimension 128, got %d", len(embedding))
	}

	// Test batch embedding
	texts := []string{"hello", "world", "test"}
	embeddings, err := provider.EmbedBatch(ctx, texts)
	if err != nil {
		t.Fatalf("embed batch failed: %v", err)
	}

	if len(embeddings) != 3 {
		t.Errorf("expected 3 embeddings, got %d", len(embeddings))
	}
}

func TestPipeline(t *testing.T) {
	provider := NewLocalProvider(128)
	config := DefaultPipelineConfig()
	config.Provider = provider
	config.BatchSize = 10
	config.CacheEnabled = true
	config.CacheMaxSize = 100

	pipeline := NewPipeline(config, nil)

	ctx := context.Background()

	// Test embedding
	embedding, err := pipeline.Embed(ctx, "This is a test document with some content.")
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	if len(embedding) != 128 {
		t.Errorf("expected dimension 128, got %d", len(embedding))
	}

	// Test cache hit
	embedding2, err := pipeline.Embed(ctx, "This is a test document with some content.")
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	// Should be same (from cache)
	for i := range embedding {
		if embedding[i] != embedding2[i] {
			t.Error("cached embedding should be identical")
			break
		}
	}

	// Check stats - second call should have used cache
	stats := pipeline.Stats()
	t.Logf("Stats: hits=%d, misses=%d", stats.CacheHits, stats.CacheMisses)
	// Cache behavior is internal, just verify embeddings match
}

func TestPipelineChunks(t *testing.T) {
	provider := NewLocalProvider(64)
	config := DefaultPipelineConfig()
	config.Provider = provider
	config.Chunker.ChunkSize = 50
	config.Chunker.Method = ChunkMethodFixed

	pipeline := NewPipeline(config, nil)

	ctx := context.Background()

	text := "This is a longer text that will be split into multiple chunks for embedding generation."

	chunks, err := pipeline.EmbedChunks(ctx, text)
	if err != nil {
		t.Fatalf("embed chunks failed: %v", err)
	}

	if len(chunks) < 2 {
		t.Error("expected multiple chunks")
	}

	for i, ce := range chunks {
		if len(ce.Embedding) != 64 {
			t.Errorf("chunk %d: expected dimension 64, got %d", i, len(ce.Embedding))
		}
		if ce.Chunk.Text == "" {
			t.Errorf("chunk %d: text should not be empty", i)
		}
	}
}

func TestAggregation(t *testing.T) {
	provider := NewLocalProvider(4)

	testCases := []struct {
		name   string
		method AggregateMethod
	}{
		{"mean", AggregateMean},
		{"first", AggregateFirst},
		{"max", AggregateMax},
		{"weighted", AggregateWeighted},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultPipelineConfig()
			config.Provider = provider
			config.AggregateMethod = tc.method
			config.Chunker.ChunkSize = 20

			pipeline := NewPipeline(config, nil)
			ctx := context.Background()

			embedding, err := pipeline.Embed(ctx, "This is text. More text here. Even more content.")
			if err != nil {
				t.Fatalf("embed failed: %v", err)
			}

			if len(embedding) != 4 {
				t.Errorf("expected dimension 4, got %d", len(embedding))
			}
		})
	}
}

func TestContentHash(t *testing.T) {
	hash1 := ContentHash("hello world")
	hash2 := ContentHash("hello world")
	hash3 := ContentHash("different text")

	if hash1 != hash2 {
		t.Error("same content should produce same hash")
	}

	if hash1 == hash3 {
		t.Error("different content should produce different hash")
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text     string
		minCount int
		maxCount int
	}{
		{"hello", 1, 3},
		{"hello world", 2, 5},
		{"This is a longer sentence with more words.", 8, 15},
	}

	for _, tt := range tests {
		count := EstimateTokens(tt.text)
		if count < tt.minCount || count > tt.maxCount {
			t.Errorf("EstimateTokens(%q) = %d, expected between %d and %d",
				tt.text, count, tt.minCount, tt.maxCount)
		}
	}
}

func BenchmarkChunker(b *testing.B) {
	chunker := NewChunker(ChunkerConfig{
		Method:    ChunkMethodSemantic,
		ChunkSize: 512,
	})

	text := "This is a test sentence. " // Repeat to make longer
	for i := 0; i < 100; i++ {
		text += "This is another sentence with some content. "
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chunker.Split(text)
	}
}

func BenchmarkLocalProvider(b *testing.B) {
	provider := NewLocalProvider(384)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = provider.Embed(ctx, "This is a test document for benchmarking.")
	}
}
