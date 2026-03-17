// Package feather provides a Go client for the Feather feature store.
package feather

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
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

// Close releases resources held by the client, including idle HTTP connections.
func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}

// isRetryable returns true if the HTTP method is safe to retry.
func isRetryable(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

// request performs an HTTP request with retry for idempotent methods only.
func (c *Client) request(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
	}

	maxAttempts := 1
	if isRetryable(method) {
		maxAttempts = c.config.MaxRetries + 1
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
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

		// Limit response body to 64MB to prevent memory exhaustion.
		const maxResponseSize = 64 << 20
		respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		closeErr := resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if closeErr != nil {
			lastErr = closeErr
			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = &APIError{StatusCode: resp.StatusCode, Message: string(respBody)}
			continue
		}

		if resp.StatusCode >= 400 {
			return parseErrorResponse(respBody, resp.StatusCode)
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

// parseErrorResponse parses the API error envelope.
// The server returns: {"success":false,"error":{"code":"...","message":"..."}}
func parseErrorResponse(body []byte, statusCode int) error {
	var envelope struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error != nil {
		return &APIError{
			StatusCode: statusCode,
			Code:       envelope.Error.Code,
			Message:    envelope.Error.Message,
		}
	}
	// Fallback: try flat {"error":"message"} format
	var flat struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &flat); err == nil && flat.Error != "" {
		return &APIError{StatusCode: statusCode, Message: flat.Error}
	}
	return &APIError{StatusCode: statusCode, Message: string(body)}
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
		jitterOffset := (jitterFloat64()*2 - 1) * jitterRange
		backoff = time.Duration(float64(backoff) + jitterOffset)
	}

	return backoff
}

func jitterFloat64() float64 {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0.5
	}
	return float64(binary.LittleEndian.Uint64(buf[:])) / float64(^uint64(0))
}

// APIError represents an API error response.
type APIError struct {
	StatusCode int
	Code       string // Server error code (e.g., "VALIDATION_FAILED")
	Message    string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("API error %d [%s]: %s", e.StatusCode, e.Code, e.Message)
	}
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
		Entities []string `json:"entities"`
		Features []string `json:"features"`
	}{
		Entities: entityIDs,
		Features: features,
	}

	var resp struct {
		Entities map[string]*GetResponse `json:"entities"`
	}

	err := f.client.request(ctx, "POST", "/v1/features/batch", req, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Entities, nil
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
	err := c.client.request(ctx, "GET", "/v1/catalog/features/"+url.PathEscape(name), nil, &def)
	if err != nil {
		return nil, err
	}
	return &def, nil
}

// ListOptions configures pagination for list operations.
type ListOptions struct {
	// Limit is the maximum number of items to return. 0 uses server default.
	Limit int
	// Offset is the number of items to skip. 0 starts from the beginning.
	Offset int
}

// ListResult contains paginated list results.
type ListResult struct {
	Features []*FeatureDefinition `json:"features"`
	Total    int                  `json:"total"`
	Limit    int                  `json:"limit"`
	Offset   int                  `json:"offset"`
}

// List lists feature definitions with optional pagination and filters.
func (c *CatalogClient) List(ctx context.Context, filter map[string]string) ([]*FeatureDefinition, error) {
	result, err := c.ListWithOptions(ctx, filter, nil)
	if err != nil {
		return nil, err
	}
	return result.Features, nil
}

// ListWithOptions lists feature definitions with pagination and filters.
func (c *CatalogClient) ListWithOptions(ctx context.Context, filter map[string]string, opts *ListOptions) (*ListResult, error) {
	params := url.Values{}
	for k, v := range filter {
		params.Set(k, v)
	}
	if opts != nil {
		if opts.Limit > 0 {
			params.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.Offset > 0 {
			params.Set("offset", strconv.Itoa(opts.Offset))
		}
	}

	var resp ListResult
	err := c.client.request(ctx, "GET", "/v1/catalog/features?"+params.Encode(), nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
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
	return c.client.request(ctx, "DELETE", "/v1/catalog/features/"+url.PathEscape(name), nil, nil)
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

	err := t.client.request(ctx, "POST", "/v1/transforms/"+url.PathEscape(name)+"/execute", req, &resp)
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

	return v.client.request(ctx, "POST", "/v1/vectors/"+url.PathEscape(indexName)+"/upsert", req, nil)
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

	err := v.client.request(ctx, "POST", "/v1/vectors/"+url.PathEscape(indexName)+"/search", req, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Results, nil
}

// StreamingClient handles streaming operations via polling.
type StreamingClient struct {
	client *Client
}

// SubscribeOptions configures the streaming subscription.
type SubscribeOptions struct {
	// EntityID is the entity to watch for updates.
	EntityID string
	// PollInterval is how often to check for updates. Defaults to 1 second.
	PollInterval time.Duration
}

// Subscribe watches for feature value changes on a specific entity and calls
// the handler when any subscribed feature changes. Blocks until the context
// is cancelled. Uses polling since the server does not support push-based streaming.
func (s *StreamingClient) Subscribe(ctx context.Context, opts SubscribeOptions, features []string, handler func(FeatureUpdate)) error {
	if opts.EntityID == "" {
		return fmt.Errorf("entity_id is required for subscription")
	}
	if len(features) == 0 {
		return fmt.Errorf("at least one feature is required for subscription")
	}

	pollInterval := opts.PollInterval
	if pollInterval <= 0 {
		pollInterval = time.Second
	}

	// Track last known values to detect changes
	lastValues := make(map[string]interface{})

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			resp, err := s.client.Features.Get(ctx, opts.EntityID, features)
			if err != nil {
				continue // Transient errors — retry on next tick
			}

			for name, fv := range resp.Features {
				prev, seen := lastValues[name]
				if !seen || prev != fv.Value {
					lastValues[name] = fv.Value
					if seen { // Only fire handler after first baseline is established
						handler(FeatureUpdate{
							EntityID:  opts.EntityID,
							Feature:   name,
							Value:     fv.Value,
							Timestamp: fv.Timestamp,
						})
					}
				}
			}
		}
	}
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
