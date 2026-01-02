package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIEmbedder generates embeddings using OpenAI's API.
type OpenAIEmbedder struct {
	apiKey    string
	model     string
	baseURL   string
	dimension int
	client    *http.Client
}

// OpenAIConfig configures the OpenAI embedder.
type OpenAIConfig struct {
	APIKey    string
	Model     string // e.g., "text-embedding-3-small"
	BaseURL   string // Optional custom endpoint
	Dimension int    // Output dimension
}

// NewOpenAIEmbedder creates a new OpenAI embedder.
func NewOpenAIEmbedder(config OpenAIConfig) *OpenAIEmbedder {
	if config.Model == "" {
		config.Model = "text-embedding-3-small"
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://api.openai.com/v1"
	}
	if config.Dimension == 0 {
		config.Dimension = 1536
	}

	return &OpenAIEmbedder{
		apiKey:    config.APIKey,
		model:     config.Model,
		baseURL:   config.BaseURL,
		dimension: config.Dimension,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (e *OpenAIEmbedder) Dimension() int {
	return e.dimension
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

func (e *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := map[string]interface{}{
		"input":      texts,
		"model":      e.model,
		"dimensions": e.dimension,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		e.baseURL+"/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	embeddings := make([][]float32, len(texts))
	for _, d := range result.Data {
		embeddings[d.Index] = d.Embedding
	}

	return embeddings, nil
}

// OllamaEmbedder generates embeddings using Ollama locally.
type OllamaEmbedder struct {
	baseURL   string
	model     string
	dimension int
	client    *http.Client
}

// OllamaConfig configures the Ollama embedder.
type OllamaConfig struct {
	BaseURL   string
	Model     string
	Dimension int
}

// NewOllamaEmbedder creates a new Ollama embedder.
func NewOllamaEmbedder(config OllamaConfig) *OllamaEmbedder {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:11434"
	}
	if config.Model == "" {
		config.Model = "nomic-embed-text"
	}
	if config.Dimension == 0 {
		config.Dimension = 768
	}

	return &OllamaEmbedder{
		baseURL:   config.BaseURL,
		model:     config.Model,
		dimension: config.Dimension,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (e *OllamaEmbedder) Dimension() int {
	return e.dimension
}

func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"model":  e.model,
		"prompt": text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		e.baseURL+"/api/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Embedding, nil
}

func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := e.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed text %d: %w", i, err)
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

// LocalEmbedder provides a simple local embedding without external APIs.
// It uses TF-IDF style embeddings for basic semantic similarity.
type LocalEmbedder struct {
	dimension int
	vocab     map[string]int
	idf       map[string]float32
}

// NewLocalEmbedder creates a new local embedder.
func NewLocalEmbedder(dimension int) *LocalEmbedder {
	if dimension == 0 {
		dimension = 384
	}
	return &LocalEmbedder{
		dimension: dimension,
		vocab:     make(map[string]int),
		idf:       make(map[string]float32),
	}
}

func (e *LocalEmbedder) Dimension() int {
	return e.dimension
}

func (e *LocalEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	words := tokenize(text)
	embedding := make([]float32, e.dimension)

	// Build word frequency
	wordFreq := make(map[string]int)
	for _, word := range words {
		wordFreq[word]++
	}

	// Generate embedding using hashing trick
	for word, freq := range wordFreq {
		hash := simpleHash(word)
		idx := hash % uint32(e.dimension)

		// Use TF-IDF style weight
		tf := float32(freq) / float32(len(words))
		idf := e.idf[word]
		if idf == 0 {
			idf = 1.0
		}

		embedding[idx] += tf * idf
	}

	normalize(embedding)

	return embedding, nil
}

func (e *LocalEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// First pass: update IDF
	docFreq := make(map[string]int)
	for _, text := range texts {
		words := tokenize(text)
		seen := make(map[string]bool)
		for _, word := range words {
			if !seen[word] {
				docFreq[word]++
				seen[word] = true
			}
		}
	}

	// Compute IDF
	n := float32(len(texts))
	for word, df := range docFreq {
		e.idf[word] = float32(1.0) + float32(1.0)/float32(1+df/int(n+1))
	}

	// Generate embeddings
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := e.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = emb
	}

	return embeddings, nil
}
