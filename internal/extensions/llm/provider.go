// Package llm provides embedding providers and helper utilities.
package llm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Provider defines the interface for embedding generation.
type Provider interface {
	// Embed generates an embedding for a single text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension returns the embedding dimension.
	Dimension() int

	// ModelID returns the model identifier.
	ModelID() string
}

// ProviderType identifies the embedding provider.
type ProviderType string

// ProviderType values identify embedding providers.
const (
	ProviderTypeOpenAI      ProviderType = "openai"
	ProviderTypeOllama      ProviderType = "ollama"
	ProviderTypeHuggingFace ProviderType = "huggingface"
	ProviderTypeLocal       ProviderType = "local"
	ProviderTypeCohere      ProviderType = "cohere"
	ProviderTypeVoyage      ProviderType = "voyage"
)

// OpenAIProvider generates embeddings using OpenAI's API.
type OpenAIProvider struct {
	apiKey      string
	model       string
	baseURL     string
	dimension   int
	client      *http.Client
	maxRetries  int
	rateLimiter *rateLimiter
}

// OpenAIConfig configures the OpenAI provider.
type OpenAIConfig struct {
	APIKey     string
	Model      string // text-embedding-3-small, text-embedding-3-large, text-embedding-ada-002
	BaseURL    string // Optional custom endpoint (for Azure, etc.)
	Dimension  int    // Output dimension (model dependent)
	MaxRetries int    // Number of retries on failure
	RateLimit  int    // Requests per second limit
}

// NewOpenAIProvider creates a new OpenAI embedding provider.
func NewOpenAIProvider(config OpenAIConfig) *OpenAIProvider {
	if config.Model == "" {
		config.Model = "text-embedding-3-small"
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://api.openai.com/v1"
	}
	if config.Dimension == 0 {
		config.Dimension = modelDimensions[config.Model]
		if config.Dimension == 0 {
			config.Dimension = 1536
		}
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RateLimit == 0 {
		config.RateLimit = 500 // Default 500 RPM
	}

	return &OpenAIProvider{
		apiKey:      config.APIKey,
		model:       config.Model,
		baseURL:     config.BaseURL,
		dimension:   config.Dimension,
		maxRetries:  config.MaxRetries,
		rateLimiter: newRateLimiter(config.RateLimit),
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

var modelDimensions = map[string]int{
	"text-embedding-3-small": 1536,
	"text-embedding-3-large": 3072,
	"text-embedding-ada-002": 1536,
	"nomic-embed-text":       768,
	"mxbai-embed-large":      1024,
	"all-minilm":             384,
}

// Dimension returns the embedding dimension.
func (p *OpenAIProvider) Dimension() int {
	return p.dimension
}

// ModelID returns the provider model identifier.
func (p *OpenAIProvider) ModelID() string {
	return "openai:" + p.model
}

// Embed generates an embedding for a single text.
func (p *OpenAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for a batch of texts.
func (p *OpenAIProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Wait for rate limiter
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		embeddings, err := p.embedBatchOnce(ctx, texts)
		if err == nil {
			return embeddings, nil
		}
		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			return nil, err
		}

		// Exponential backoff
		//nolint:gosec // attempt is bounded by maxRetries.
		backoff := time.Duration(1<<uint64(attempt)) * 100 * time.Millisecond
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (p *OpenAIProvider) embedBatchOnce(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := map[string]interface{}{
		"input":      texts,
		"model":      p.model,
		"dimensions": p.dimension,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		p.baseURL+"/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
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

// OllamaProvider generates embeddings using Ollama locally.
type OllamaProvider struct {
	baseURL   string
	model     string
	dimension int
	client    *http.Client
}

// OllamaConfig configures the Ollama provider.
type OllamaConfig struct {
	BaseURL   string
	Model     string
	Dimension int
}

// NewOllamaProvider creates a new Ollama embedding provider.
func NewOllamaProvider(config OllamaConfig) *OllamaProvider {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:11434"
	}
	if config.Model == "" {
		config.Model = "nomic-embed-text"
	}
	if config.Dimension == 0 {
		config.Dimension = modelDimensions[config.Model]
		if config.Dimension == 0 {
			config.Dimension = 768
		}
	}

	return &OllamaProvider{
		baseURL:   config.BaseURL,
		model:     config.Model,
		dimension: config.Dimension,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Dimension returns the embedding dimension.
func (p *OllamaProvider) Dimension() int {
	return p.dimension
}

// ModelID returns the provider model identifier.
func (p *OllamaProvider) ModelID() string {
	return "ollama:" + p.model
}

// Embed generates an embedding for a single text.
func (p *OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"model":  p.model,
		"prompt": text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		p.baseURL+"/api/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Embedding, nil
}

// EmbedBatch generates embeddings for a batch of texts.
func (p *OllamaProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := p.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed text %d: %w", i, err)
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

// HuggingFaceProvider generates embeddings using HuggingFace Inference API.
type HuggingFaceProvider struct {
	apiKey    string
	model     string
	dimension int
	client    *http.Client
}

// HuggingFaceConfig configures the HuggingFace provider.
type HuggingFaceConfig struct {
	APIKey    string
	Model     string // e.g., "sentence-transformers/all-MiniLM-L6-v2"
	Dimension int
}

// NewHuggingFaceProvider creates a new HuggingFace embedding provider.
func NewHuggingFaceProvider(config HuggingFaceConfig) *HuggingFaceProvider {
	if config.Model == "" {
		config.Model = "sentence-transformers/all-MiniLM-L6-v2"
	}
	if config.Dimension == 0 {
		config.Dimension = 384
	}

	return &HuggingFaceProvider{
		apiKey:    config.APIKey,
		model:     config.Model,
		dimension: config.Dimension,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Dimension returns the embedding dimension.
func (p *HuggingFaceProvider) Dimension() int {
	return p.dimension
}

// ModelID returns the provider model identifier.
func (p *HuggingFaceProvider) ModelID() string {
	return "huggingface:" + p.model
}

// Embed generates an embedding for a single text.
func (p *HuggingFaceProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for a batch of texts.
func (p *HuggingFaceProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := map[string]interface{}{
		"inputs": texts,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("https://api-inference.huggingface.co/pipeline/feature-extraction/%s", p.model)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	var embeddings [][]float32
	if err := json.NewDecoder(resp.Body).Decode(&embeddings); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return embeddings, nil
}

// LocalProvider provides simple TF-IDF based embeddings for testing.
type LocalProvider struct {
	dimension int
	vocab     map[string]int
	idf       map[string]float32
	mu        sync.RWMutex
}

// NewLocalProvider creates a local embedding provider.
func NewLocalProvider(dimension int) *LocalProvider {
	if dimension == 0 {
		dimension = 384
	}
	return &LocalProvider{
		dimension: dimension,
		vocab:     make(map[string]int),
		idf:       make(map[string]float32),
	}
}

// Dimension returns the embedding dimension.
func (p *LocalProvider) Dimension() int {
	return p.dimension
}

// ModelID returns the provider model identifier.
func (p *LocalProvider) ModelID() string {
	return "local:tfidf"
}

// Embed generates an embedding for a single text.
func (p *LocalProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	words := tokenize(text)
	embedding := make([]float32, p.dimension)

	wordFreq := make(map[string]int)
	for _, word := range words {
		wordFreq[word]++
	}

	for word, freq := range wordFreq {
		hash := simpleHash(word)
		//nolint:gosec // dimension is configured and hash is used for indexing only.
		idx := int(hash % uint32(p.dimension))

		tf := float32(freq) / float32(len(words))
		p.mu.RLock()
		idf := p.idf[word]
		p.mu.RUnlock()
		if idf == 0 {
			idf = 1.0
		}

		embedding[idx] += tf * idf
	}

	normalize(embedding)
	return embedding, nil
}

// EmbedBatch generates embeddings for a batch of texts.
func (p *LocalProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// Update IDF
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

	p.mu.Lock()
	n := float32(len(texts))
	for word, df := range docFreq {
		p.idf[word] = 1.0 + 1.0/float32(1+float32(df)/(n+1))
	}
	p.mu.Unlock()

	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := p.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = emb
	}

	return embeddings, nil
}

// APIError represents an API error.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

func isRetryableError(err error) bool {
	if apiErr, ok := isAPIError(err); ok {
		return apiErr.StatusCode == 429 || apiErr.StatusCode >= 500
	}
	return false
}

func isAPIError(err error) (*APIError, bool) {
	if err == nil {
		return nil, false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}

// Rate limiter for API calls
type rateLimiter struct {
	tokens   chan struct{}
	interval time.Duration
	done     chan struct{}
}

func newRateLimiter(rps int) *rateLimiter {
	if rps <= 0 {
		rps = 1
	}
	r := &rateLimiter{
		tokens:   make(chan struct{}, rps),
		interval: time.Second / time.Duration(rps),
		done:     make(chan struct{}),
	}

	// Fill initial tokens
	for i := 0; i < rps; i++ {
		r.tokens <- struct{}{}
	}

	// Start token refill goroutine
	go r.refill()

	return r
}

func (r *rateLimiter) refill() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.done:
			return
		case <-ticker.C:
			select {
			case r.tokens <- struct{}{}:
			default:
			}
		}
	}
}

func (r *rateLimiter) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.tokens:
		return nil
	}
}

func (r *rateLimiter) Close() {
	close(r.done)
}

// Helper functions
func tokenize(text string) []string {
	text = strings.ToLower(text)
	var words []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

func simpleHash(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

func normalize(v []float32) {
	var sum float32
	for _, val := range v {
		sum += val * val
	}
	if sum > 0 {
		norm := float32(1.0 / float64(sum))
		for i := range v {
			v[i] *= norm
		}
	}
}

// ContentHash generates a hash of text content for caching.
func ContentHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}
