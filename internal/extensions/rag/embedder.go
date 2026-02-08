package rag

import (
	"context"
	"hash/fnv"
	"math"
	"strings"
	"sync"
)

// Embedder generates vector embeddings from text.
type Embedder interface {
	// Embed produces an embedding for a single text.
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch produces embeddings for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	// Dimension returns the embedding dimensionality.
	Dimension() int
}

// LocalEmbedder produces bag-of-words style embeddings by hashing words
// into fixed-dimension vectors. Suitable for testing without external APIs.
type LocalEmbedder struct {
	dim   int
	vocab map[string]int
	mu    sync.RWMutex
}

// NewLocalEmbedder creates a LocalEmbedder with the given dimension.
func NewLocalEmbedder(dimension int) *LocalEmbedder {
	if dimension <= 0 {
		dimension = 128
	}
	return &LocalEmbedder{
		dim:   dimension,
		vocab: make(map[string]int),
	}
}

// Embed produces a bag-of-words embedding for the given text.
func (e *LocalEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	return e.embed(text), nil
}

// EmbedBatch produces embeddings for multiple texts.
func (e *LocalEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, t := range texts {
		results[i] = e.embed(t)
	}
	return results, nil
}

// Dimension returns the embedding dimensionality.
func (e *LocalEmbedder) Dimension() int {
	return e.dim
}

// embed generates an embedding by hashing each word to a position and accumulating counts,
// then L2-normalizing the result.
func (e *LocalEmbedder) embed(text string) []float32 {
	vec := make([]float32, e.dim)
	words := tokenize(text)
	if len(words) == 0 {
		return vec
	}

	for _, w := range words {
		idx := e.wordIndex(w)
		vec[idx] += 1.0
	}

	// L2 normalize
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if norm > 0 {
		norm = math.Sqrt(norm)
		for i := range vec {
			vec[i] = float32(float64(vec[i]) / norm)
		}
	}

	return vec
}

// wordIndex returns a stable index in [0, dim) for the given word,
// using FNV hashing and caching the result in the vocab map.
func (e *LocalEmbedder) wordIndex(word string) int {
	e.mu.RLock()
	idx, ok := e.vocab[word]
	e.mu.RUnlock()
	if ok {
		return idx
	}

	h := fnv.New32a()
	h.Write([]byte(word))
	idx = int(h.Sum32()) % e.dim
	if idx < 0 {
		idx = -idx
	}

	e.mu.Lock()
	e.vocab[word] = idx
	e.mu.Unlock()

	return idx
}

// tokenize splits text into lowercase words.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	fields := strings.Fields(text)
	words := make([]string, 0, len(fields))
	for _, f := range fields {
		// Strip common punctuation.
		w := strings.TrimFunc(f, func(r rune) bool {
			return r == '.' || r == ',' || r == '!' || r == '?' ||
				r == ':' || r == ';' || r == '"' || r == '\'' ||
				r == '(' || r == ')' || r == '[' || r == ']'
		})
		if len(w) > 0 {
			words = append(words, w)
		}
	}
	return words
}
