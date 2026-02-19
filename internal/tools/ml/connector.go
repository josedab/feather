// Package ml provides connectors to external ML model serving systems.
package ml

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/core/storage"
)

// Connector-specific errors.
var (
	ErrConnectorNotFound = errors.New("connector not found")
	ErrConnectorExists   = errors.New("connector already exists")
)

// maxErrorResponseSize limits the size of error response bodies read from external services.
const maxErrorResponseSize = 1 << 20 // 1MB

// closeBody closes an HTTP response body and logs any error at debug level.
func closeBody(resp *http.Response) {
	if err := resp.Body.Close(); err != nil {
		slog.Debug("failed to close response body", "error", err)
	}
}

// Connector represents a connection to an ML serving system.
type Connector interface {
	// Name returns the connector name
	Name() string

	// Type returns the connector type (tensorflow, mlflow, sagemaker, etc.)
	Type() string

	// Connect establishes connection to the ML system
	Connect(ctx context.Context) error

	// Disconnect closes the connection
	Disconnect(ctx context.Context) error

	// IsConnected returns connection status
	IsConnected() bool

	// GetFeatures fetches features from the feature store for inference
	GetFeatures(ctx context.Context, entityID string, featureNames []string) (map[string]interface{}, error)

	// BatchGetFeatures fetches features for multiple entities
	BatchGetFeatures(ctx context.Context, entityIDs []string, featureNames []string) ([]map[string]interface{}, error)

	// Predict sends features to the model and returns predictions
	Predict(ctx context.Context, req *PredictRequest) (*PredictResponse, error)

	// BatchPredict performs batch predictions
	BatchPredict(ctx context.Context, req *BatchPredictRequest) (*BatchPredictResponse, error)
}

// PredictRequest represents a prediction request.
type PredictRequest struct {
	ModelName    string                 `json:"model_name"`
	ModelVersion string                 `json:"model_version,omitempty"`
	EntityID     string                 `json:"entity_id"`
	Features     map[string]interface{} `json:"features,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// PredictResponse represents a prediction response.
type PredictResponse struct {
	ModelName    string                 `json:"model_name"`
	ModelVersion string                 `json:"model_version"`
	Predictions  interface{}            `json:"predictions"`
	Latency      time.Duration          `json:"latency_ns"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// BatchPredictRequest represents a batch prediction request.
type BatchPredictRequest struct {
	ModelName    string                   `json:"model_name"`
	ModelVersion string                   `json:"model_version,omitempty"`
	EntityIDs    []string                 `json:"entity_ids"`
	Features     []map[string]interface{} `json:"features,omitempty"`
	Metadata     map[string]interface{}   `json:"metadata,omitempty"`
}

// BatchPredictResponse represents a batch prediction response.
type BatchPredictResponse struct {
	ModelName    string                 `json:"model_name"`
	ModelVersion string                 `json:"model_version"`
	Predictions  []interface{}          `json:"predictions"`
	Latency      time.Duration          `json:"latency_ns"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ConnectorConfig contains common configuration for connectors.
type ConnectorConfig struct {
	Name         string
	Endpoint     string
	Timeout      time.Duration
	MaxRetries   int
	RetryBackoff time.Duration
	Headers      map[string]string
	Store        *storage.Store
}

// BaseConnector provides common functionality for connectors.
type BaseConnector struct {
	config      ConnectorConfig
	httpClient  *http.Client
	connected   bool
	connectedMu sync.RWMutex
}

// NewBaseConnector creates a new base connector.
func NewBaseConnector(config ConnectorConfig) *BaseConnector {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryBackoff == 0 {
		config.RetryBackoff = 100 * time.Millisecond
	}

	return &BaseConnector{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Name returns the connector name.
func (c *BaseConnector) Name() string {
	return c.config.Name
}

// IsConnected reports whether the connector is currently connected.
func (c *BaseConnector) IsConnected() bool {
	c.connectedMu.RLock()
	defer c.connectedMu.RUnlock()
	return c.connected
}

func (c *BaseConnector) setConnected(connected bool) {
	c.connectedMu.Lock()
	defer c.connectedMu.Unlock()
	c.connected = connected
}

// GetFeatures retrieves feature values for an entity from the feature store.
func (c *BaseConnector) GetFeatures(ctx context.Context, entityID string, featureNames []string) (map[string]interface{}, error) {
	if c.config.Store == nil {
		return nil, fmt.Errorf("store not configured")
	}

	values, err := c.config.Store.Get(ctx, entityID, featureNames)
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{}, len(values))
	for name, fv := range values {
		result[name] = fv.Value
	}

	return result, nil
}

// BatchGetFeatures retrieves feature values for multiple entities.
func (c *BaseConnector) BatchGetFeatures(ctx context.Context, entityIDs []string, featureNames []string) ([]map[string]interface{}, error) {
	results := make([]map[string]interface{}, len(entityIDs))

	for i, entityID := range entityIDs {
		features, err := c.GetFeatures(ctx, entityID, featureNames)
		if err != nil {
			return nil, fmt.Errorf("getting features for %s: %w", entityID, err)
		}
		results[i] = features
	}

	return results, nil
}

func (c *BaseConnector) doRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.config.RetryBackoff * time.Duration(attempt)):
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")
		for k, v := range c.config.Headers {
			req.Header.Set(k, v)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 500 {
			closeBody(resp)
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", c.config.MaxRetries+1, lastErr)
}

// TensorFlowConnector connects to TensorFlow Serving.
type TensorFlowConnector struct {
	*BaseConnector
}

// TensorFlowConfig configures TensorFlow Serving connection.
type TensorFlowConfig struct {
	ConnectorConfig
	RESTEndpoint string // REST API endpoint (default: /v1/models)
}

// NewTensorFlowConnector creates a TensorFlow Serving connector.
func NewTensorFlowConnector(config TensorFlowConfig) *TensorFlowConnector {
	if config.RESTEndpoint == "" {
		config.RESTEndpoint = "/v1/models"
	}
	return &TensorFlowConnector{
		BaseConnector: NewBaseConnector(config.ConnectorConfig),
	}
}

// Type returns the connector type identifier.
func (c *TensorFlowConnector) Type() string {
	return "tensorflow"
}

// Connect establishes a connection to the TensorFlow Serving endpoint.
func (c *TensorFlowConnector) Connect(ctx context.Context) error {
	// Test connection by checking model status
	url := fmt.Sprintf("%s%s", c.config.Endpoint, "/v1/models")
	resp, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("connecting to TensorFlow Serving: %w", err)
	}
	defer closeBody(resp)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	c.setConnected(true)
	return nil
}

// Disconnect marks the connector as disconnected.
func (c *TensorFlowConnector) Disconnect(ctx context.Context) error {
	c.setConnected(false)
	return nil
}

// Predict performs a prediction request against TensorFlow Serving.
func (c *TensorFlowConnector) Predict(ctx context.Context, req *PredictRequest) (*PredictResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	// Get features if not provided
	features := req.Features
	if features == nil {
		features = make(map[string]interface{})
	}
	if len(features) > 0 && req.EntityID != "" && c.config.Store != nil {
		featureNames := make([]string, 0, len(features))
		for name := range features {
			featureNames = append(featureNames, name)
		}
		fetched, err := c.GetFeatures(ctx, req.EntityID, featureNames)
		if err != nil {
			return nil, fmt.Errorf("fetching features: %w", err)
		}
		features = fetched
	}

	// Build TensorFlow Serving request
	tfReq := map[string]interface{}{
		"instances": []interface{}{features},
	}

	body, err := json.Marshal(tfReq)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/v1/models/%s:predict", c.config.Endpoint, req.ModelName)
	if req.ModelVersion != "" {
		url = fmt.Sprintf("%s/v1/models/%s/versions/%s:predict", c.config.Endpoint, req.ModelName, req.ModelVersion)
	}

	start := time.Now()
	resp, err := c.doRequest(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer closeBody(resp)
	latency := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorResponseSize))
		if readErr != nil {
			body = []byte("(failed to read response body)")
		}
		return nil, fmt.Errorf("prediction failed: %d: %s", resp.StatusCode, string(body))
	}

	var tfResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tfResp); err != nil {
		return nil, err
	}

	return &PredictResponse{
		ModelName:    req.ModelName,
		ModelVersion: req.ModelVersion,
		Predictions:  tfResp["predictions"],
		Latency:      latency,
	}, nil
}

// BatchPredict performs batch predictions against TensorFlow Serving.
func (c *TensorFlowConnector) BatchPredict(ctx context.Context, req *BatchPredictRequest) (*BatchPredictResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	// Get features for all entities
	var instances []interface{}
	if req.Features != nil {
		for _, f := range req.Features {
			instances = append(instances, f)
		}
	} else if len(req.EntityIDs) > 0 && c.config.Store != nil {
		// Get features for each entity
		for range req.EntityIDs {
			// Without knowing feature names, we create empty instances
			instances = append(instances, map[string]interface{}{})
		}
	}

	tfReq := map[string]interface{}{
		"instances": instances,
	}

	body, err := json.Marshal(tfReq)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/v1/models/%s:predict", c.config.Endpoint, req.ModelName)
	if req.ModelVersion != "" {
		url = fmt.Sprintf("%s/v1/models/%s/versions/%s:predict", c.config.Endpoint, req.ModelName, req.ModelVersion)
	}

	start := time.Now()
	resp, err := c.doRequest(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer closeBody(resp)
	latency := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorResponseSize))
		if readErr != nil {
			body = []byte("(failed to read response body)")
		}
		return nil, fmt.Errorf("batch prediction failed: %d: %s", resp.StatusCode, string(body))
	}

	var tfResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tfResp); err != nil {
		return nil, err
	}

	predictions, ok := tfResp["predictions"].([]interface{})
	if !ok {
		predictions = []interface{}{tfResp["predictions"]}
	}

	return &BatchPredictResponse{
		ModelName:    req.ModelName,
		ModelVersion: req.ModelVersion,
		Predictions:  predictions,
		Latency:      latency,
	}, nil
}

// MLflowConnector connects to MLflow Model Serving.
type MLflowConnector struct {
	*BaseConnector
	trackingURI string
}

// MLflowConfig configures MLflow connection.
type MLflowConfig struct {
	ConnectorConfig
	TrackingURI string // MLflow tracking server URI
}

// NewMLflowConnector creates an MLflow connector.
func NewMLflowConnector(config MLflowConfig) *MLflowConnector {
	return &MLflowConnector{
		BaseConnector: NewBaseConnector(config.ConnectorConfig),
		trackingURI:   config.TrackingURI,
	}
}

// Type returns the connector type identifier.
func (c *MLflowConnector) Type() string {
	return "mlflow"
}

// Connect establishes a connection to the MLflow tracking server.
func (c *MLflowConnector) Connect(ctx context.Context) error {
	// Test connection to MLflow tracking server
	url := fmt.Sprintf("%s/api/2.0/mlflow/experiments/list", c.trackingURI)
	resp, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("connecting to MLflow: %w", err)
	}
	defer closeBody(resp)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	c.setConnected(true)
	return nil
}

// Disconnect marks the connector as disconnected.
func (c *MLflowConnector) Disconnect(ctx context.Context) error {
	c.setConnected(false)
	return nil
}

// Predict performs a prediction request against MLflow serving.
func (c *MLflowConnector) Predict(ctx context.Context, req *PredictRequest) (*PredictResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	features := req.Features
	if features == nil {
		features = make(map[string]interface{})
	}

	// MLflow model serving format
	mlflowReq := map[string]interface{}{
		"inputs": map[string]interface{}{
			"data": []interface{}{features},
		},
	}

	body, err := json.Marshal(mlflowReq)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/invocations", c.config.Endpoint)

	start := time.Now()
	resp, err := c.doRequest(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer closeBody(resp)
	latency := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorResponseSize))
		if readErr != nil {
			body = []byte("(failed to read response body)")
		}
		return nil, fmt.Errorf("prediction failed: %d: %s", resp.StatusCode, string(body))
	}

	var predictions interface{}
	if err := json.NewDecoder(resp.Body).Decode(&predictions); err != nil {
		return nil, err
	}

	return &PredictResponse{
		ModelName:    req.ModelName,
		ModelVersion: req.ModelVersion,
		Predictions:  predictions,
		Latency:      latency,
	}, nil
}

// BatchPredict performs batch predictions against MLflow serving.
func (c *MLflowConnector) BatchPredict(ctx context.Context, req *BatchPredictRequest) (*BatchPredictResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	var data []interface{}
	if req.Features != nil {
		for _, f := range req.Features {
			data = append(data, f)
		}
	}

	mlflowReq := map[string]interface{}{
		"inputs": map[string]interface{}{
			"data": data,
		},
	}

	body, err := json.Marshal(mlflowReq)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/invocations", c.config.Endpoint)

	start := time.Now()
	resp, err := c.doRequest(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer closeBody(resp)
	latency := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorResponseSize))
		if readErr != nil {
			body = []byte("(failed to read response body)")
		}
		return nil, fmt.Errorf("batch prediction failed: %d: %s", resp.StatusCode, string(body))
	}

	var predictions []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&predictions); err != nil {
		return nil, err
	}

	return &BatchPredictResponse{
		ModelName:    req.ModelName,
		ModelVersion: req.ModelVersion,
		Predictions:  predictions,
		Latency:      latency,
	}, nil
}

// SageMakerConnector connects to AWS SageMaker endpoints.
type SageMakerConnector struct {
	*BaseConnector
	region   string
	endpoint string
}

// SageMakerConfig configures SageMaker connection.
type SageMakerConfig struct {
	ConnectorConfig
	Region       string
	EndpointName string
}

// NewSageMakerConnector creates a SageMaker connector.
func NewSageMakerConnector(config SageMakerConfig) *SageMakerConnector {
	return &SageMakerConnector{
		BaseConnector: NewBaseConnector(config.ConnectorConfig),
		region:        config.Region,
		endpoint:      config.EndpointName,
	}
}

// Type returns the connector type identifier.
func (c *SageMakerConnector) Type() string {
	return "sagemaker"
}

// Connect validates the SageMaker endpoint configuration.
func (c *SageMakerConnector) Connect(ctx context.Context) error {
	// SageMaker connection is stateless, just validate config
	if c.region == "" {
		return fmt.Errorf("region is required")
	}
	if c.endpoint == "" {
		return fmt.Errorf("endpoint name is required")
	}
	c.setConnected(true)
	return nil
}

// Disconnect marks the connector as disconnected.
func (c *SageMakerConnector) Disconnect(ctx context.Context) error {
	c.setConnected(false)
	return nil
}

// Predict performs a prediction request against SageMaker.
func (c *SageMakerConnector) Predict(ctx context.Context, req *PredictRequest) (*PredictResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	features := req.Features
	if features == nil {
		features = make(map[string]interface{})
	}

	// SageMaker accepts CSV or JSON format
	body, err := json.Marshal(features)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/endpoints/%s/invocations", c.config.Endpoint, c.endpoint)

	start := time.Now()
	resp, err := c.doRequest(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer closeBody(resp)
	latency := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorResponseSize))
		if readErr != nil {
			body = []byte("(failed to read response body)")
		}
		return nil, fmt.Errorf("prediction failed: %d: %s", resp.StatusCode, string(body))
	}

	var predictions interface{}
	if err := json.NewDecoder(resp.Body).Decode(&predictions); err != nil {
		return nil, err
	}

	return &PredictResponse{
		ModelName:    req.ModelName,
		ModelVersion: req.ModelVersion,
		Predictions:  predictions,
		Latency:      latency,
	}, nil
}

// BatchPredict performs batch predictions against SageMaker.
func (c *SageMakerConnector) BatchPredict(ctx context.Context, req *BatchPredictRequest) (*BatchPredictResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	body, err := json.Marshal(req.Features)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/endpoints/%s/invocations", c.config.Endpoint, c.endpoint)

	start := time.Now()
	resp, err := c.doRequest(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer closeBody(resp)
	latency := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorResponseSize))
		if readErr != nil {
			body = []byte("(failed to read response body)")
		}
		return nil, fmt.Errorf("batch prediction failed: %d: %s", resp.StatusCode, string(body))
	}

	var predictions []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&predictions); err != nil {
		return nil, err
	}

	return &BatchPredictResponse{
		ModelName:    req.ModelName,
		ModelVersion: req.ModelVersion,
		Predictions:  predictions,
		Latency:      latency,
	}, nil
}

// ConnectorRegistry manages ML connectors.
type ConnectorRegistry struct {
	connectors map[string]Connector
	mu         sync.RWMutex
}

// NewConnectorRegistry creates a new connector registry.
func NewConnectorRegistry() *ConnectorRegistry {
	return &ConnectorRegistry{
		connectors: make(map[string]Connector),
	}
}

// Register adds a connector to the registry.
func (r *ConnectorRegistry) Register(name string, connector Connector) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.connectors[name]; exists {
		return fmt.Errorf("%w: %s", ErrConnectorExists, name)
	}

	r.connectors[name] = connector
	return nil
}

// Unregister removes a connector from the registry.
func (r *ConnectorRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.connectors[name]; !exists {
		return fmt.Errorf("%w: %s", ErrConnectorNotFound, name)
	}

	delete(r.connectors, name)
	return nil
}

// Get retrieves a connector by name.
func (r *ConnectorRegistry) Get(name string) (Connector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	connector, exists := r.connectors[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrConnectorNotFound, name)
	}

	return connector, nil
}

// List returns all registered connectors.
func (r *ConnectorRegistry) List() []Connector {
	r.mu.RLock()
	defer r.mu.RUnlock()

	connectors := make([]Connector, 0, len(r.connectors))
	for _, c := range r.connectors {
		connectors = append(connectors, c)
	}

	return connectors
}

// ConnectAll connects all registered connectors.
func (r *ConnectorRegistry) ConnectAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, c := range r.connectors {
		if err := c.Connect(ctx); err != nil {
			return fmt.Errorf("connecting %s: %w", name, err)
		}
	}

	return nil
}

// DisconnectAll disconnects all registered connectors.
func (r *ConnectorRegistry) DisconnectAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var lastErr error
	for name, c := range r.connectors {
		if err := c.Disconnect(ctx); err != nil {
			lastErr = fmt.Errorf("disconnecting %s: %w", name, err)
		}
	}

	return lastErr
}
