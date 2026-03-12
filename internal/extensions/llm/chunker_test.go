package llm

import (
	"strings"
	"testing"
)

func TestSplitSentence_MultiSentence(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		Method:    ChunkMethodSentence,
		ChunkSize: 500,
	})
	text := "This is the first sentence. This is the second sentence. And here is a third."
	chunks := chunker.Split(text)
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 sentence chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if len(strings.TrimSpace(c.Text)) == 0 {
			t.Error("chunk should not be empty")
		}
	}
}

func TestSplitSentence_SingleSentence(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		Method:    ChunkMethodSentence,
		ChunkSize: 500,
	})
	text := "Just one sentence here"
	chunks := chunker.Split(text)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for single sentence, got %d", len(chunks))
	}
}

func TestSplitSentence_EmptyInput(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		Method:    ChunkMethodSentence,
		ChunkSize: 500,
	})
	chunks := chunker.Split("")
	if chunks != nil {
		t.Errorf("expected nil for empty input, got %d chunks", len(chunks))
	}
}

func TestSplitSentence_Abbreviations(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		Method:    ChunkMethodSentence,
		ChunkSize: 500,
	})
	text := "Mr. Smith went to Washington. He met Dr. Jones there."
	chunks := chunker.Split(text)
	// Abbreviations with periods may cause sentence splitting — just verify no panic
	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}
}

func TestSplitSentence_Unicode(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		Method:    ChunkMethodSentence,
		ChunkSize: 500,
	})
	text := "こんにちは世界。 これはテストです。 日本語のテキスト。"
	chunks := chunker.Split(text)
	if len(chunks) == 0 {
		t.Error("expected chunks for unicode text")
	}
}

func TestSplitParagraph_Multiple(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		Method:    ChunkMethodParagraph,
		ChunkSize: 500,
	})
	text := "First paragraph content here.\n\nSecond paragraph content here.\n\nThird paragraph."
	chunks := chunker.Split(text)
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 paragraph chunks, got %d", len(chunks))
	}
}

func TestSplitParagraph_Single(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		Method:    ChunkMethodParagraph,
		ChunkSize: 500,
	})
	text := "Just one paragraph with no double newlines."
	chunks := chunker.Split(text)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for single paragraph, got %d", len(chunks))
	}
}

func TestSplitParagraph_MixedLineEndings(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		Method:    ChunkMethodParagraph,
		ChunkSize: 500,
	})
	// \r\n will be normalized by normalizeWhitespace
	text := "Paragraph one.\r\n\r\nParagraph two."
	chunks := chunker.Split(text)
	// After whitespace normalization, \r\n becomes \n, so \r\n\r\n becomes \n\n
	if len(chunks) == 0 {
		t.Error("expected chunks for text with mixed line endings")
	}
}

func TestMergeSmallChunks_BelowMin(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		ChunkSize:    500,
		MinChunkSize: 50,
	})

	chunks := []Chunk{
		{Text: "Hi", Index: 0, StartChar: 0, EndChar: 2},
		{Text: "This is a much longer chunk that should exceed the minimum size threshold for merging.", Index: 1, StartChar: 3, EndChar: 88},
	}

	merged := chunker.mergeSmallChunks(chunks)
	if len(merged) != 1 {
		t.Errorf("expected 1 merged chunk, got %d", len(merged))
	}
}

func TestMergeSmallChunks_AllAboveMin(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		ChunkSize:    500,
		MinChunkSize: 10,
	})

	chunks := []Chunk{
		{Text: "This chunk is above the minimum size.", Index: 0},
		{Text: "This chunk is also above the minimum.", Index: 1},
	}

	merged := chunker.mergeSmallChunks(chunks)
	if len(merged) != 2 {
		t.Errorf("expected 2 chunks (all above min), got %d", len(merged))
	}
}

func TestMergeSmallChunks_SingleTiny(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		ChunkSize:    500,
		MinChunkSize: 50,
	})

	chunks := []Chunk{
		{Text: "tiny", Index: 0, StartChar: 0, EndChar: 4},
	}

	merged := chunker.mergeSmallChunks(chunks)
	if len(merged) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(merged))
	}
}

func TestGetOverlapText_WordBoundary(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog"
	overlap := getOverlapText(text, 10)
	if len(overlap) == 0 {
		t.Error("expected non-empty overlap text")
	}
	// Should start at a word boundary
	if overlap[0] == ' ' {
		t.Error("overlap should not start with space")
	}
}

func TestGetOverlapText_LargerThanText(t *testing.T) {
	text := "Short"
	overlap := getOverlapText(text, 100)
	if overlap != text {
		t.Errorf("expected full text for overlap larger than text, got %q", overlap)
	}
}

func TestGetOverlapText_ZeroOverlap(t *testing.T) {
	text := "Some text here"
	overlap := getOverlapText(text, 0)
	if len(overlap) > len(text) {
		t.Errorf("zero overlap should return minimal text, got %q", overlap)
	}
}

func TestSplit_SentenceStrategy(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		Method:    ChunkMethodSentence,
		ChunkSize: 500,
	})
	text := "First sentence. Second sentence. Third sentence."
	chunks := chunker.Split(text)
	if len(chunks) == 0 {
		t.Error("expected chunks")
	}
	// Verify all text is captured
	var all string
	for _, c := range chunks {
		all += c.Text + " "
	}
	all = strings.TrimSpace(all)
	if !strings.Contains(all, "First") || !strings.Contains(all, "Third") {
		t.Error("expected all sentences to be present in chunks")
	}
}

func TestSplit_ParagraphStrategy(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		Method:    ChunkMethodParagraph,
		ChunkSize: 500,
	})
	text := "Para one.\n\nPara two.\n\nPara three."
	chunks := chunker.Split(text)
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks for paragraph split, got %d", len(chunks))
	}
}
