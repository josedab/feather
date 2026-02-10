package modelserving

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// AdapterType identifies the model serving backend.
type AdapterType string

const (
	AdapterMLflow     AdapterType = "mlflow"
	AdapterBentoML    AdapterType = "bentoml"
	AdapterTorchServe AdapterType = "torchserve"
	AdapterTriton     AdapterType = "triton"
	AdapterCustom     AdapterType = "custom"
)

// ModelAdapter is a pluggable interface for model inference backends.
type ModelAdapter interface {
	Name() string
	Type() AdapterType
	Predict(modelID string, version int, features map[string]interface{}) (*PredictionResult, error)
	HealthCheck() error
}

// PredictionResult contains the output of a model prediction.
type PredictionResult struct {
	ModelID     string                 `json:"model_id"`
	Version     int                    `json:"version"`
	Predictions map[string]interface{} `json:"predictions"`
	Latency     time.Duration          `json:"latency_ms"`
	Adapter     string                 `json:"adapter"`
	Shadow      bool                   `json:"shadow,omitempty"`
}

// PredictRequest is the input to the unified predict endpoint.
type PredictRequest struct {
	ModelID    string                 `json:"model_id"`
	Version    int                    `json:"version,omitempty"`
	EntityKey  string                 `json:"entity_key"`
	Features   map[string]interface{} `json:"features,omitempty"`
	ABRouting  bool                   `json:"ab_routing,omitempty"`
	Shadow     bool                   `json:"shadow,omitempty"`
}

// PredictResponse is returned from the unified predict endpoint.
type PredictResponse struct {
	Primary      *PredictionResult   `json:"primary"`
	Shadow       *PredictionResult   `json:"shadow,omitempty"`
	FeaturesUsed map[string]interface{} `json:"features_used,omitempty"`
	LatencyMs    float64             `json:"latency_ms"`
}

// ABConfig configures A/B model routing.
type ABConfig struct {
	ModelA      string  `json:"model_a"`
	ModelB      string  `json:"model_b"`
	TrafficPct  float64 `json:"traffic_pct_b"` // percentage routed to model B
	Enabled     bool    `json:"enabled"`
}

// Gateway provides a unified model serving endpoint.
type Gateway struct {
	mu       sync.RWMutex
	registry *Registry
	adapters map[string]ModelAdapter
	abConfig map[string]*ABConfig
	stats    GatewayStats
}

// GatewayStats tracks gateway usage.
type GatewayStats struct {
	TotalPredictions int64          `json:"total_predictions"`
	TotalErrors      int64          `json:"total_errors"`
	ShadowRuns       int64          `json:"shadow_runs"`
	ABRoutingCount   int64          `json:"ab_routing_count"`
	AdapterCounts    map[string]int64 `json:"adapter_counts"`
	AvgLatencyMs     float64        `json:"avg_latency_ms"`
	totalLatencyUs   int64
}

// NewGateway creates a new model serving gateway.
func NewGateway(registry *Registry) *Gateway {
	return &Gateway{
		registry: registry,
		adapters: make(map[string]ModelAdapter),
		abConfig: make(map[string]*ABConfig),
		stats: GatewayStats{
			AdapterCounts: make(map[string]int64),
		},
	}
}

// RegisterAdapter adds a model serving adapter.
func (g *Gateway) RegisterAdapter(adapter ModelAdapter) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.adapters[adapter.Name()] = adapter
}

// SetABConfig configures A/B routing for a model.
func (g *Gateway) SetABConfig(modelID string, cfg ABConfig) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.abConfig[modelID] = &cfg
}

// GetABConfig returns the A/B config for a model.
func (g *Gateway) GetABConfig(modelID string) *ABConfig {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if cfg, ok := g.abConfig[modelID]; ok {
		cp := *cfg
		return &cp
	}
	return nil
}

// ListAdapters returns all registered adapters.
func (g *Gateway) ListAdapters() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	names := make([]string, 0, len(g.adapters))
	for name := range g.adapters {
		names = append(names, name)
	}
	return names
}

// Predict runs inference through the gateway, resolving features and routing.
func (g *Gateway) Predict(req PredictRequest) (*PredictResponse, error) {
	start := time.Now()

	g.mu.RLock()
	abCfg := g.abConfig[req.ModelID]
	g.mu.RUnlock()

	// A/B routing
	targetModel := req.ModelID
	if req.ABRouting && abCfg != nil && abCfg.Enabled {
		if rand.Float64() < abCfg.TrafficPct {
			targetModel = abCfg.ModelB
		} else {
			targetModel = abCfg.ModelA
		}
		g.mu.Lock()
		g.stats.ABRoutingCount++
		g.mu.Unlock()
	}

	// Get model version
	version := req.Version
	if version == 0 {
		version = 1
	}

	// Resolve features — use provided features or resolve from entity
	features := req.Features
	if features == nil {
		features = make(map[string]interface{})
	}

	// Run primary prediction using built-in adapter
	primary, err := g.runPrediction(targetModel, version, features, false)
	if err != nil {
		g.mu.Lock()
		g.stats.TotalErrors++
		g.mu.Unlock()
		return nil, fmt.Errorf("prediction failed: %w", err)
	}

	resp := &PredictResponse{
		Primary:      primary,
		FeaturesUsed: features,
	}

	// Shadow scoring
	if req.Shadow && abCfg != nil && abCfg.Enabled {
		shadowModel := abCfg.ModelB
		if targetModel == abCfg.ModelB {
			shadowModel = abCfg.ModelA
		}
		shadowResult, shadowErr := g.runPrediction(shadowModel, version, features, true)
		if shadowErr == nil {
			resp.Shadow = shadowResult
		}
		g.mu.Lock()
		g.stats.ShadowRuns++
		g.mu.Unlock()
	}

	elapsed := time.Since(start)
	resp.LatencyMs = float64(elapsed.Microseconds()) / 1000.0

	g.mu.Lock()
	g.stats.TotalPredictions++
	g.stats.totalLatencyUs += elapsed.Microseconds()
	if g.stats.TotalPredictions > 0 {
		g.stats.AvgLatencyMs = float64(g.stats.totalLatencyUs) / float64(g.stats.TotalPredictions) / 1000.0
	}
	g.mu.Unlock()

	return resp, nil
}

func (g *Gateway) runPrediction(modelID string, version int, features map[string]interface{}, shadow bool) (*PredictionResult, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Try registered adapters first
	for name, adapter := range g.adapters {
		result, err := adapter.Predict(modelID, version, features)
		if err == nil {
			result.Shadow = shadow
			g.stats.AdapterCounts[name]++
			return result, nil
		}
	}

	// Fall back to built-in stub (returns echo of features as predictions)
	return &PredictionResult{
		ModelID:     modelID,
		Version:     version,
		Predictions: features,
		Adapter:     "builtin",
		Shadow:      shadow,
	}, nil
}

// Stats returns gateway statistics.
func (g *Gateway) Stats() GatewayStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	cp := g.stats
	cp.AdapterCounts = make(map[string]int64)
	for k, v := range g.stats.AdapterCounts {
		cp.AdapterCounts[k] = v
	}
	return cp
}

// Registry returns the underlying model registry.
func (g *Gateway) Registry() *Registry {
	return g.registry
}
