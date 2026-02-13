package llm

import (
	"context"
	"math"
	"testing"
)

// --- LocalProvider ---

func TestLocalProvider_Dimension(t *testing.T) {
	p := NewLocalProvider(128)
	if p.Dimension() != 128 {
		t.Fatalf("expected dimension 128, got %d", p.Dimension())
	}
}

func TestLocalProvider_DefaultDimension(t *testing.T) {
	p := NewLocalProvider(0)
	if p.Dimension() != 384 {
		t.Fatalf("expected default dimension 384, got %d", p.Dimension())
	}
}

func TestLocalProvider_ModelID(t *testing.T) {
	p := NewLocalProvider(128)
	if p.ModelID() != "local:tfidf" {
		t.Fatalf("expected model ID local:tfidf, got %s", p.ModelID())
	}
}

func TestLocalProvider_Embed(t *testing.T) {
	p := NewLocalProvider(128)
	ctx := context.Background()

	emb, err := p.Embed(ctx, "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emb) != 128 {
		t.Fatalf("expected embedding of length 128, got %d", len(emb))
	}

	// Should have non-zero values
	hasNonZero := false
	for _, v := range emb {
		if v != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Fatal("expected non-zero embedding")
	}
}

func TestLocalProvider_Embed_EmptyInput(t *testing.T) {
	p := NewLocalProvider(128)
	ctx := context.Background()

	emb, err := p.Embed(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emb) != 128 {
		t.Fatalf("expected embedding of length 128, got %d", len(emb))
	}
	// Empty input should produce zero embedding
	for _, v := range emb {
		if v != 0 {
			t.Fatal("expected all-zero embedding for empty input")
		}
	}
}

func TestLocalProvider_Embed_SingleWord(t *testing.T) {
	p := NewLocalProvider(128)
	ctx := context.Background()

	emb, err := p.Embed(ctx, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emb) != 128 {
		t.Fatalf("expected embedding of length 128, got %d", len(emb))
	}
}

func TestLocalProvider_Embed_Deterministic(t *testing.T) {
	p := NewLocalProvider(128)
	ctx := context.Background()

	emb1, _ := p.Embed(ctx, "test phrase")
	emb2, _ := p.Embed(ctx, "test phrase")

	for i := range emb1 {
		if emb1[i] != emb2[i] {
			t.Fatalf("expected deterministic embeddings, differ at index %d", i)
		}
	}
}

func TestLocalProvider_EmbedBatch(t *testing.T) {
	p := NewLocalProvider(128)
	ctx := context.Background()

	texts := []string{"hello world", "foo bar", "test embedding"}
	embeddings, err := p.EmbedBatch(ctx, texts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(embeddings) != 3 {
		t.Fatalf("expected 3 embeddings, got %d", len(embeddings))
	}
	for i, emb := range embeddings {
		if len(emb) != 128 {
			t.Fatalf("embedding %d: expected length 128, got %d", i, len(emb))
		}
	}
}

func TestLocalProvider_EmbedBatch_Empty(t *testing.T) {
	p := NewLocalProvider(128)
	ctx := context.Background()

	embeddings, err := p.EmbedBatch(ctx, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(embeddings) != 0 {
		t.Fatalf("expected 0 embeddings, got %d", len(embeddings))
	}
}

func TestLocalProvider_EmbedBatch_UpdatesIDF(t *testing.T) {
	p := NewLocalProvider(128)
	ctx := context.Background()

	// EmbedBatch updates IDF weights
	texts := []string{"hello world", "hello there", "world peace"}
	_, err := p.EmbedBatch(ctx, texts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p.mu.RLock()
	_, hasHello := p.idf["hello"]
	_, hasWorld := p.idf["world"]
	p.mu.RUnlock()

	if !hasHello || !hasWorld {
		t.Fatal("expected IDF entries for hello and world")
	}
}

// --- isRetryableError ---

func TestIsRetryableError_RateLimit(t *testing.T) {
	err := &APIError{StatusCode: 429, Message: "rate limited"}
	if !isRetryableError(err) {
		t.Fatal("expected 429 to be retryable")
	}
}

func TestIsRetryableError_ServerError(t *testing.T) {
	err := &APIError{StatusCode: 500, Message: "internal server error"}
	if !isRetryableError(err) {
		t.Fatal("expected 500 to be retryable")
	}
}

func TestIsRetryableError_502(t *testing.T) {
	err := &APIError{StatusCode: 502, Message: "bad gateway"}
	if !isRetryableError(err) {
		t.Fatal("expected 502 to be retryable")
	}
}

func TestIsRetryableError_ClientError(t *testing.T) {
	err := &APIError{StatusCode: 400, Message: "bad request"}
	if isRetryableError(err) {
		t.Fatal("expected 400 to not be retryable")
	}
}

func TestIsRetryableError_Unauthorized(t *testing.T) {
	err := &APIError{StatusCode: 401, Message: "unauthorized"}
	if isRetryableError(err) {
		t.Fatal("expected 401 to not be retryable")
	}
}

func TestIsRetryableError_NonAPIError(t *testing.T) {
	err := context.DeadlineExceeded
	if isRetryableError(err) {
		t.Fatal("expected non-API error to not be retryable")
	}
}

// --- isAPIError ---

func TestIsAPIError_Nil(t *testing.T) {
	_, ok := isAPIError(nil)
	if ok {
		t.Fatal("expected nil to not be API error")
	}
}

func TestIsAPIError_APIError(t *testing.T) {
	err := &APIError{StatusCode: 400, Message: "bad request"}
	apiErr, ok := isAPIError(err)
	if !ok {
		t.Fatal("expected to detect API error")
	}
	if apiErr.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", apiErr.StatusCode)
	}
}

// --- APIError ---

func TestAPIError_Error(t *testing.T) {
	err := &APIError{StatusCode: 500, Message: "oops"}
	expected := "API error 500: oops"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

// --- rateLimiter ---

func TestRateLimiter_Wait(t *testing.T) {
	rl := newRateLimiter(10)
	defer rl.Close()
	ctx := context.Background()

	// Should be able to consume initial tokens
	err := rl.Wait(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRateLimiter_WaitCancelled(t *testing.T) {
	rl := newRateLimiter(1)
	defer rl.Close()

	// Consume the initial token
	ctx := context.Background()
	_ = rl.Wait(ctx)

	// Now try with cancelled context
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	err := rl.Wait(cancelCtx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// --- tokenize ---

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"Hello World!", []string{"hello", "world"}},
		{"foo-bar_baz", []string{"foo", "bar", "baz"}},
		{"  spaces  ", []string{"spaces"}},
		{"", nil},
		{"123 abc", []string{"123", "abc"}},
	}

	for _, tt := range tests {
		result := tokenize(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("tokenize(%q): expected %v, got %v", tt.input, tt.expected, result)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("tokenize(%q)[%d]: expected %q, got %q", tt.input, i, tt.expected[i], result[i])
			}
		}
	}
}

// --- normalize ---

func TestNormalize_ZeroVector(t *testing.T) {
	v := []float32{0, 0, 0}
	normalize(v)
	for _, val := range v {
		if val != 0 {
			t.Fatal("expected zero vector to remain zero")
		}
	}
}

func TestNormalize_NonZeroVector(t *testing.T) {
	v := []float32{3, 4, 0}
	normalize(v)
	// After normalization, values should be non-zero
	if v[0] == 0 && v[1] == 0 {
		t.Fatal("expected non-zero values after normalization")
	}
}

// --- ContentHash ---

func TestContentHash_Provider(t *testing.T) {
	h1 := ContentHash("hello")
	h2 := ContentHash("hello")
	h3 := ContentHash("world")

	if h1 != h2 {
		t.Fatal("expected same hash for same input")
	}
	if h1 == h3 {
		t.Fatal("expected different hash for different input")
	}
	if len(h1) != 64 {
		t.Fatalf("expected 64 char hex hash, got %d", len(h1))
	}
}

// --- simpleHash ---

func TestSimpleHash_Deterministic(t *testing.T) {
	h1 := simpleHash("test")
	h2 := simpleHash("test")
	if h1 != h2 {
		t.Fatal("expected deterministic hash")
	}
}

func TestSimpleHash_DifferentInputs(t *testing.T) {
	h1 := simpleHash("abc")
	h2 := simpleHash("xyz")
	if h1 == h2 {
		t.Fatal("expected different hashes for different inputs")
	}
}

// --- OpenAI Provider config ---

func TestOpenAIProvider_Config(t *testing.T) {
	p := NewOpenAIProvider(OpenAIConfig{APIKey: "test-key"})
	defer p.rateLimiter.Close()

	if p.ModelID() != "openai:text-embedding-3-small" {
		t.Fatalf("expected default model, got %s", p.ModelID())
	}
	if p.Dimension() != 1536 {
		t.Fatalf("expected dimension 1536, got %d", p.Dimension())
	}
}

func TestOpenAIProvider_EmbedBatch_EmptyInput(t *testing.T) {
	p := NewOpenAIProvider(OpenAIConfig{APIKey: "test-key"})
	defer p.rateLimiter.Close()

	result, err := p.EmbedBatch(context.Background(), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for empty input, got %v", result)
	}
}

// --- OllamaProvider config ---

func TestOllamaProvider_Config(t *testing.T) {
	p := NewOllamaProvider(OllamaConfig{})
	if p.ModelID() != "ollama:nomic-embed-text" {
		t.Fatalf("expected default model, got %s", p.ModelID())
	}
	if p.Dimension() != 768 {
		t.Fatalf("expected dimension 768, got %d", p.Dimension())
	}
}

// --- HuggingFace Provider config ---

func TestHuggingFaceProvider_Config(t *testing.T) {
	p := NewHuggingFaceProvider(HuggingFaceConfig{})
	if p.ModelID() != "huggingface:sentence-transformers/all-MiniLM-L6-v2" {
		t.Fatalf("expected default model, got %s", p.ModelID())
	}
	if p.Dimension() != 384 {
		t.Fatalf("expected dimension 384, got %d", p.Dimension())
	}
}

// --- Embedding similarity ---

func TestLocalProvider_DifferentTextsProduceDifferentEmbeddings(t *testing.T) {
	p := NewLocalProvider(256)
	ctx := context.Background()

	emb1, _ := p.Embed(ctx, "machine learning is great")
	emb2, _ := p.Embed(ctx, "the cat sat on the mat")

	// Compute cosine distance — embeddings should differ
	var dot, norm1, norm2 float32
	for i := range emb1 {
		dot += emb1[i] * emb2[i]
		norm1 += emb1[i] * emb1[i]
		norm2 += emb2[i] * emb2[i]
	}

	if norm1 == 0 || norm2 == 0 {
		t.Fatal("expected non-zero norms")
	}

	similarity := float64(dot) / (math.Sqrt(float64(norm1)) * math.Sqrt(float64(norm2)))
	if similarity > 0.99 {
		t.Fatalf("expected different embeddings for different texts, similarity: %f", similarity)
	}
}
