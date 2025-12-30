// Package feather provides a Go client for the Feather feature store.
package feather

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Client is the main Feather client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	config     *ClientConfig

	// Sub-clients
	Features  *FeatureClient
	Catalog   *CatalogClient
	Transform *TransformClient
	Vectors   *VectorClient
	Streaming *StreamingClient
}

// ClientConfig contains client configuration options.
type ClientConfig struct {
	Timeout         time.Duration
	MaxRetries      int
	RetryBackoff    time.Duration // Base delay for exponential backoff
	MaxRetryBackoff time.Duration // Maximum delay between retries
	RetryJitter     float64       // Jitter factor (0.0-1.0) for randomization
	MaxIdleConns    int
	IdleConnTimeout time.Duration
}

// DefaultConfig returns the default client configuration.
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		Timeout:         30 * time.Second,
		MaxRetries:      3,
		RetryBackoff:    100 * time.Millisecond,
		MaxRetryBackoff: 10 * time.Second,
		RetryJitter:     0.2, // ±20% jitter
		MaxIdleConns:    100,
		IdleConnTimeout: 90 * time.Second,
	}
}

// NewClient creates a new Feather client.
func NewClient(baseURL, apiKey string, config *ClientConfig) *Client {
	if config == nil {
		config = DefaultConfig()
	}

	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:       config.MaxIdleConns,
			IdleConnTimeout:    config.IdleConnTimeout,
			DisableCompression: false,
			DisableKeepAlives:  false,
		},
	}

	c := &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: httpClient,
		config:     config,
	}

	c.Features = &FeatureClient{client: c}
	c.Catalog = &CatalogClient{client: c}
	c.Transform = &TransformClient{client: c}
	c.Vectors = &VectorClient{client: c}
	c.Streaming = &StreamingClient{client: c}

	return c
}

// request performs an HTTP request with exponential backoff retry and jitter.
func (c *Client) request(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		// Create fresh reader for each attempt (body may have been consumed)
		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = &APIError{StatusCode: resp.StatusCode, Message: string(respBody)}
			continue
		}

		if resp.StatusCode >= 400 {
			var errResp struct {
				Error string `json:"error"`
			}
			json.Unmarshal(respBody, &errResp)
			return &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
		}

		if result != nil {
			if err := json.Unmarshal(respBody, result); err != nil {
				return fmt.Errorf("unmarshal response: %w", err)
			}
		}

		return nil
	}

	return lastErr
}

// calculateBackoff calculates the delay for exponential backoff with jitter.
// Uses the formula: min(maxBackoff, baseDelay * 2^attempt) * (1 ± jitter)
func (c *Client) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^(attempt-1)
	backoff := c.config.RetryBackoff
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff > c.config.MaxRetryBackoff {
			backoff = c.config.MaxRetryBackoff
			break
		}
	}

	// Apply jitter: multiply by (1 - jitter + rand * 2 * jitter)
	// This gives a range of [1-jitter, 1+jitter]
	if c.config.RetryJitter > 0 {
		jitterRange := float64(backoff) * c.config.RetryJitter
		jitterOffset := (rand.Float64()*2 - 1) * jitterRange
		backoff = time.Duration(float64(backoff) + jitterOffset)
	}

	return backoff
}

// APIError represents an API error response.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

// FeatureClient handles feature operations.
type FeatureClient struct {
	client *Client
}

// FeatureValue represents a feature value.
type FeatureValue struct {
	Feature   string      `json:"feature"`
	Value     interface{} `json:"value"`
	Timestamp time.Time   `json:"timestamp"`
	Version   int64       `json:"version,omitempty"`
}

// GetRequest represents a feature get request.
type GetRequest struct {
	EntityID string   `json:"entity_id"`
	Features []string `json:"features"`
}

// GetResponse represents a feature get response.
type GetResponse struct {
	EntityID string                  `json:"entity_id"`
	Features map[string]FeatureValue `json:"features"`
}

// Get retrieves features for an entity.
func (f *FeatureClient) Get(ctx context.Context, entityID string, features []string) (*GetResponse, error) {
	params := url.Values{}
	params.Set("entity", entityID)
	for _, feat := range features {
		params.Add("feature", feat)
	}

	var resp GetResponse
	err := f.client.request(ctx, "GET", "/v1/features?"+params.Encode(), nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// GetBatch retrieves features for multiple entities.
func (f *FeatureClient) GetBatch(ctx context.Context, entityIDs []string, features []string) (map[string]*GetResponse, error) {
	req := struct {
		EntityIDs []string `json:"entity_ids"`
		Features  []string `json:"features"`
	}{
		EntityIDs: entityIDs,
		Features:  features,
	}

	var resp struct {
		Results map[string]*GetResponse `json:"results"`
	}

	err := f.client.request(ctx, "POST", "/v1/features/batch", req, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Results, nil
}

// PutRequest represents a feature put request.
type PutRequest struct {
	EntityID  string                 `json:"entity_id"`
	Features  map[string]interface{} `json:"features"`
	Timestamp *time.Time             `json:"timestamp,omitempty"`
}

// Put stores features for an entity.
func (f *FeatureClient) Put(ctx context.Context, req *PutRequest) error {
	return f.client.request(ctx, "POST", "/v1/features", req, nil)
}

// GetAsOf retrieves features as of a specific time.
func (f *FeatureClient) GetAsOf(ctx context.Context, entityID string, features []string, asOf time.Time) (*GetResponse, error) {
	params := url.Values{}
	params.Set("entity", entityID)
	params.Set("as_of", asOf.Format(time.RFC3339))
	for _, feat := range features {
		params.Add("feature", feat)
	}

	var resp GetResponse
	err := f.client.request(ctx, "GET", "/v1/features/history?"+params.Encode(), nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// CatalogClient handles catalog operations.
type CatalogClient struct {
	client *Client
}

// FeatureDefinition represents a feature definition in the catalog.
type FeatureDefinition struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	DataType    string            `json:"data_type"`
	EntityType  string            `json:"entity_type"`
	Owner       string            `json:"owner"`
	Team        string            `json:"team"`
	Tags        []string          `json:"tags"`
	Category    string            `json:"category"`
	Status      string            `json:"status"`
	Version     int               `json:"version"`
	Metadata    map[string]string `json:"metadata"`
}

// Register registers a feature definition.
func (c *CatalogClient) Register(ctx context.Context, def *FeatureDefinition) error {
	return c.client.request(ctx, "POST", "/v1/catalog/features", def, def)
}

// Get retrieves a feature definition.
func (c *CatalogClient) Get(ctx context.Context, name string) (*FeatureDefinition, error) {
	var def FeatureDefinition
	err := c.client.request(ctx, "GET", "/v1/catalog/features/"+name, nil, &def)
	if err != nil {
		return nil, err
	}
	return &def, nil
}

// List lists feature definitions.
func (c *CatalogClient) List(ctx context.Context, filter map[string]string) ([]*FeatureDefinition, error) {
	params := url.Values{}
	for k, v := range filter {
		params.Set(k, v)
	}

	var resp struct {
		Features []*FeatureDefinition `json:"features"`
	}

	err := c.client.request(ctx, "GET", "/v1/catalog/features?"+params.Encode(), nil, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Features, nil
}

// Search searches for features.
func (c *CatalogClient) Search(ctx context.Context, query string, limit int) ([]*FeatureDefinition, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("limit", strconv.Itoa(limit))

	var resp struct {
		Results []*FeatureDefinition `json:"results"`
	}

	err := c.client.request(ctx, "GET", "/v1/catalog/search?"+params.Encode(), nil, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Results, nil
}

// Delete deletes a feature definition.
func (c *CatalogClient) Delete(ctx context.Context, name string) error {
	return c.client.request(ctx, "DELETE", "/v1/catalog/features/"+name, nil, nil)
}

// TransformClient handles transform operations.
type TransformClient struct {
	client *Client
}

// Transform represents a feature transformation.
type Transform struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Inputs     []string               `json:"inputs"`
	Output     string                 `json:"output"`
	Config     map[string]interface{} `json:"config"`
	Expression string                 `json:"expression,omitempty"`
}

// Create creates a transform.
func (t *TransformClient) Create(ctx context.Context, transform *Transform) error {
	return t.client.request(ctx, "POST", "/v1/transforms", transform, transform)
}

// Execute executes a transform.
func (t *TransformClient) Execute(ctx context.Context, name string, inputs map[string]interface{}) (interface{}, error) {
	req := struct {
		Inputs map[string]interface{} `json:"inputs"`
	}{
		Inputs: inputs,
	}

	var resp struct {
		Result interface{} `json:"result"`
	}

	err := t.client.request(ctx, "POST", "/v1/transforms/"+name+"/execute", req, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Result, nil
}

// List lists transforms.
func (t *TransformClient) List(ctx context.Context) ([]*Transform, error) {
	var resp struct {
		Transforms []*Transform `json:"transforms"`
	}

	err := t.client.request(ctx, "GET", "/v1/transforms", nil, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Transforms, nil
}

// VectorClient handles vector operations.
type VectorClient struct {
	client *Client
}

// VectorIndex represents a vector index.
type VectorIndex struct {
	Name       string `json:"name"`
	Dimensions int    `json:"dimensions"`
	Metric     string `json:"metric"`
}

// CreateIndex creates a vector index.
func (v *VectorClient) CreateIndex(ctx context.Context, index *VectorIndex) error {
	return v.client.request(ctx, "POST", "/v1/vectors", index, index)
}

// Upsert upserts vectors into an index.
func (v *VectorClient) Upsert(ctx context.Context, indexName string, vectors map[string][]float64, metadata map[string]map[string]interface{}) error {
	req := struct {
		Vectors  map[string][]float64              `json:"vectors"`
		Metadata map[string]map[string]interface{} `json:"metadata,omitempty"`
	}{
		Vectors:  vectors,
		Metadata: metadata,
	}

	return v.client.request(ctx, "POST", "/v1/vectors/"+indexName+"/upsert", req, nil)
}

// SearchResult represents a vector search result.
type SearchResult struct {
	ID       string                 `json:"id"`
	Score    float64                `json:"score"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Search searches for similar vectors.
func (v *VectorClient) Search(ctx context.Context, indexName string, vector []float64, topK int) ([]*SearchResult, error) {
	req := struct {
		Vector []float64 `json:"vector"`
		TopK   int       `json:"top_k"`
	}{
		Vector: vector,
		TopK:   topK,
	}

	var resp struct {
		Results []*SearchResult `json:"results"`
	}

	err := v.client.request(ctx, "POST", "/v1/vectors/"+indexName+"/search", req, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Results, nil
}

// StreamingClient handles streaming operations.
type StreamingClient struct {
	client *Client
}

// Subscribe subscribes to feature updates.
func (s *StreamingClient) Subscribe(ctx context.Context, features []string, handler func(FeatureUpdate)) error {
	// This is a placeholder - actual implementation would use WebSocket or SSE
	return fmt.Errorf("streaming not yet implemented in SDK")
}

// FeatureUpdate represents a real-time feature update.
type FeatureUpdate struct {
	EntityID  string      `json:"entity_id"`
	Feature   string      `json:"feature"`
	Value     interface{} `json:"value"`
	Timestamp time.Time   `json:"timestamp"`
}

// ConnectionPool manages a pool of clients for high-throughput scenarios.
type ConnectionPool struct {
	clients []*Client
	index   int
	mu      sync.Mutex
}

// NewConnectionPool creates a pool of clients.
func NewConnectionPool(baseURL, apiKey string, size int, config *ClientConfig) *ConnectionPool {
	clients := make([]*Client, size)
	for i := 0; i < size; i++ {
		clients[i] = NewClient(baseURL, apiKey, config)
	}

	return &ConnectionPool{
		clients: clients,
	}
}

// Get returns the next client from the pool (round-robin).
func (p *ConnectionPool) Get() *Client {
	p.mu.Lock()
	defer p.mu.Unlock()

	client := p.clients[p.index]
	p.index = (p.index + 1) % len(p.clients)
	return client
}

// Close closes all clients in the pool.
func (p *ConnectionPool) Close() {
	for _, client := range p.clients {
		client.httpClient.CloseIdleConnections()
	}
}
