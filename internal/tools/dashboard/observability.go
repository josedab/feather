package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ObservabilityConsole provides a unified observability view with freshness heatmaps,
// latency sparklines, drift alerts, pipeline DAGs, and cost attribution.
type ObservabilityConsole struct {
	mu              sync.RWMutex
	config          ConsoleConfig
	freshnessData   map[string]*FreshnessHeatmapEntry
	latencyData     map[string]*LatencySparkline
	costAttribution map[string]*CostEntry
	pipelineDAG     map[string]*PipelineNode
	alertChannels   []AlertChannel
}

// ConsoleConfig configures the observability console.
type ConsoleConfig struct {
	RetentionPeriod   time.Duration `json:"retention_period" yaml:"retention_period"`
	RefreshInterval   time.Duration `json:"refresh_interval" yaml:"refresh_interval"`
	MaxDataPoints     int           `json:"max_data_points" yaml:"max_data_points"`
	EnableCostTracking bool         `json:"enable_cost_tracking" yaml:"enable_cost_tracking"`
}

// DefaultConsoleConfig returns sensible defaults.
func DefaultConsoleConfig() ConsoleConfig {
	return ConsoleConfig{
		RetentionPeriod:    24 * time.Hour,
		RefreshInterval:    30 * time.Second,
		MaxDataPoints:      1000,
		EnableCostTracking: true,
	}
}

// FreshnessHeatmapEntry represents freshness data for a single feature.
type FreshnessHeatmapEntry struct {
	FeatureName   string        `json:"feature_name"`
	FeatureGroup  string        `json:"feature_group"`
	LastUpdated   time.Time     `json:"last_updated"`
	StalenessMs   int64         `json:"staleness_ms"`
	HeatmapColor  string        `json:"heatmap_color"` // "green", "yellow", "orange", "red"
	UpdateFreqHz  float64       `json:"update_freq_hz"`
	SLATargetMs   int64         `json:"sla_target_ms,omitempty"`
	SLACompliance float64       `json:"sla_compliance"` // 0.0 to 1.0
	History       []FreshnessPoint `json:"history,omitempty"`
}

// FreshnessPoint is a time-series data point for freshness.
type FreshnessPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	StalenessMs int64     `json:"staleness_ms"`
}

// LatencySparkline represents latency data for sparkline rendering.
type LatencySparkline struct {
	Endpoint   string          `json:"endpoint"`
	P50Ms      float64         `json:"p50_ms"`
	P95Ms      float64         `json:"p95_ms"`
	P99Ms      float64         `json:"p99_ms"`
	AvgMs      float64         `json:"avg_ms"`
	MaxMs      float64         `json:"max_ms"`
	DataPoints []LatencyPoint  `json:"data_points"`
	TotalReqs  int64           `json:"total_requests"`
	ErrorRate  float64         `json:"error_rate"`
}

// LatencyPoint is a time-series data point for latency.
type LatencyPoint struct {
	Timestamp time.Time `json:"timestamp"`
	P50Ms     float64   `json:"p50_ms"`
	P95Ms     float64   `json:"p95_ms"`
	P99Ms     float64   `json:"p99_ms"`
	Count     int64     `json:"count"`
}

// CostEntry tracks cost attribution for a feature or pipeline.
type CostEntry struct {
	ResourceName  string    `json:"resource_name"`
	ResourceType  string    `json:"resource_type"` // "feature", "pipeline", "storage"
	CostPerHour   float64   `json:"cost_per_hour"`
	CostPerDay    float64   `json:"cost_per_day"`
	ComputeCost   float64   `json:"compute_cost"`
	StorageCost   float64   `json:"storage_cost"`
	NetworkCost   float64   `json:"network_cost"`
	Currency      string    `json:"currency"`
	LastUpdated   time.Time `json:"last_updated"`
}

// PipelineNode represents a node in a feature pipeline DAG.
type PipelineNode struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"` // "source", "transform", "sink"
	Status      string   `json:"status"` // "running", "stopped", "error"
	Upstream    []string `json:"upstream,omitempty"`
	Downstream  []string `json:"downstream,omitempty"`
	Throughput  float64  `json:"throughput_per_sec"`
	ErrorCount  int64    `json:"error_count"`
	LastRun     time.Time `json:"last_run"`
}

// AlertChannel defines a destination for alert notifications.
type AlertChannel struct {
	Type     string `json:"type"` // "slack", "pagerduty", "webhook", "email"
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	APIKey   string `json:"-"` // Hidden from JSON
	Channel  string `json:"channel,omitempty"` // Slack channel
	Enabled  bool   `json:"enabled"`
}

// NewObservabilityConsole creates a new observability console.
func NewObservabilityConsole(config ConsoleConfig) *ObservabilityConsole {
	if config.MaxDataPoints == 0 {
		config = DefaultConsoleConfig()
	}
	return &ObservabilityConsole{
		config:          config,
		freshnessData:   make(map[string]*FreshnessHeatmapEntry),
		latencyData:     make(map[string]*LatencySparkline),
		costAttribution: make(map[string]*CostEntry),
		pipelineDAG:     make(map[string]*PipelineNode),
	}
}

// RegisterAlertChannel adds an alert destination.
func (oc *ObservabilityConsole) RegisterAlertChannel(channel AlertChannel) {
	oc.mu.Lock()
	defer oc.mu.Unlock()
	oc.alertChannels = append(oc.alertChannels, channel)
}

// RecordFreshness records freshness data for a feature.
func (oc *ObservabilityConsole) RecordFreshness(featureName, featureGroup string, lastUpdated time.Time, slaTargetMs int64) {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	now := time.Now()
	stalenessMs := now.Sub(lastUpdated).Milliseconds()

	entry, exists := oc.freshnessData[featureName]
	if !exists {
		entry = &FreshnessHeatmapEntry{
			FeatureName:  featureName,
			FeatureGroup: featureGroup,
			SLATargetMs:  slaTargetMs,
		}
		oc.freshnessData[featureName] = entry
	}

	entry.LastUpdated = lastUpdated
	entry.StalenessMs = stalenessMs
	entry.HeatmapColor = computeHeatmapColor(stalenessMs, slaTargetMs)

	point := FreshnessPoint{Timestamp: now, StalenessMs: stalenessMs}
	entry.History = append(entry.History, point)
	if len(entry.History) > oc.config.MaxDataPoints {
		entry.History = entry.History[len(entry.History)-oc.config.MaxDataPoints:]
	}

	// Compute SLA compliance from history
	if slaTargetMs > 0 && len(entry.History) > 0 {
		compliant := 0
		for _, p := range entry.History {
			if p.StalenessMs <= slaTargetMs {
				compliant++
			}
		}
		entry.SLACompliance = float64(compliant) / float64(len(entry.History))
	}
}

// RecordLatency records latency data for an endpoint.
func (oc *ObservabilityConsole) RecordLatency(endpoint string, p50, p95, p99 float64, count int64, errorRate float64) {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	sparkline, exists := oc.latencyData[endpoint]
	if !exists {
		sparkline = &LatencySparkline{Endpoint: endpoint}
		oc.latencyData[endpoint] = sparkline
	}

	sparkline.P50Ms = p50
	sparkline.P95Ms = p95
	sparkline.P99Ms = p99
	sparkline.TotalReqs += count
	sparkline.ErrorRate = errorRate

	point := LatencyPoint{
		Timestamp: time.Now(),
		P50Ms:     p50,
		P95Ms:     p95,
		P99Ms:     p99,
		Count:     count,
	}
	sparkline.DataPoints = append(sparkline.DataPoints, point)
	if len(sparkline.DataPoints) > oc.config.MaxDataPoints {
		sparkline.DataPoints = sparkline.DataPoints[len(sparkline.DataPoints)-oc.config.MaxDataPoints:]
	}
}

// RecordCost records cost attribution.
func (oc *ObservabilityConsole) RecordCost(name, resourceType string, compute, storage, network float64) {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	total := compute + storage + network
	oc.costAttribution[name] = &CostEntry{
		ResourceName: name,
		ResourceType: resourceType,
		CostPerHour:  total,
		CostPerDay:   total * 24,
		ComputeCost:  compute,
		StorageCost:  storage,
		NetworkCost:  network,
		Currency:     "USD",
		LastUpdated:  time.Now(),
	}
}

// SetPipelineNode updates a node in the pipeline DAG.
func (oc *ObservabilityConsole) SetPipelineNode(node PipelineNode) {
	oc.mu.Lock()
	defer oc.mu.Unlock()
	oc.pipelineDAG[node.ID] = &node
}

// GetFreshnessHeatmap returns all freshness heatmap entries.
func (oc *ObservabilityConsole) GetFreshnessHeatmap() []FreshnessHeatmapEntry {
	oc.mu.RLock()
	defer oc.mu.RUnlock()

	result := make([]FreshnessHeatmapEntry, 0, len(oc.freshnessData))
	for _, e := range oc.freshnessData {
		result = append(result, *e)
	}
	return result
}

// GetLatencySparklines returns all latency sparkline data.
func (oc *ObservabilityConsole) GetLatencySparklines() []LatencySparkline {
	oc.mu.RLock()
	defer oc.mu.RUnlock()

	result := make([]LatencySparkline, 0, len(oc.latencyData))
	for _, s := range oc.latencyData {
		result = append(result, *s)
	}
	return result
}

// GetCostAttribution returns all cost entries.
func (oc *ObservabilityConsole) GetCostAttribution() []CostEntry {
	oc.mu.RLock()
	defer oc.mu.RUnlock()

	result := make([]CostEntry, 0, len(oc.costAttribution))
	for _, c := range oc.costAttribution {
		result = append(result, *c)
	}
	return result
}

// GetPipelineDAG returns the pipeline DAG.
func (oc *ObservabilityConsole) GetPipelineDAG() []PipelineNode {
	oc.mu.RLock()
	defer oc.mu.RUnlock()

	result := make([]PipelineNode, 0, len(oc.pipelineDAG))
	for _, n := range oc.pipelineDAG {
		result = append(result, *n)
	}
	return result
}

// SendAlert dispatches an alert to all configured channels.
func (oc *ObservabilityConsole) SendAlert(ctx context.Context, title, message string, severity AlertSeverity) error {
	oc.mu.RLock()
	channels := make([]AlertChannel, len(oc.alertChannels))
	copy(channels, oc.alertChannels)
	oc.mu.RUnlock()

	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}
		if err := oc.dispatchAlert(ctx, ch, title, message, severity); err != nil {
			return fmt.Errorf("dispatching to %s: %w", ch.Name, err)
		}
	}
	return nil
}

func (oc *ObservabilityConsole) dispatchAlert(_ context.Context, ch AlertChannel, title, message string, _ AlertSeverity) error {
	switch ch.Type {
	case "slack":
		return oc.sendSlackAlert(ch, title, message)
	case "pagerduty":
		return oc.sendPagerDutyAlert(ch, title, message)
	case "webhook":
		return oc.sendWebhookAlert(ch, title, message)
	default:
		return fmt.Errorf("unsupported channel type: %s", ch.Type)
	}
}

func (oc *ObservabilityConsole) sendSlackAlert(ch AlertChannel, _, _ string) error {
	if ch.Endpoint == "" {
		return fmt.Errorf("slack webhook URL required")
	}
	// In production, POST to Slack webhook URL with structured message
	return nil
}

func (oc *ObservabilityConsole) sendPagerDutyAlert(ch AlertChannel, _, _ string) error {
	if ch.APIKey == "" {
		return fmt.Errorf("PagerDuty API key required")
	}
	// In production, POST to PagerDuty Events API v2
	return nil
}

func (oc *ObservabilityConsole) sendWebhookAlert(ch AlertChannel, _, _ string) error {
	if ch.Endpoint == "" {
		return fmt.Errorf("webhook URL required")
	}
	// In production, POST JSON payload to webhook endpoint
	return nil
}

// GetOverviewSnapshot returns a complete observability snapshot.
func (oc *ObservabilityConsole) GetOverviewSnapshot() *ObservabilitySnapshot {
	return &ObservabilitySnapshot{
		Freshness:       oc.GetFreshnessHeatmap(),
		Latency:         oc.GetLatencySparklines(),
		CostAttribution: oc.GetCostAttribution(),
		PipelineDAG:     oc.GetPipelineDAG(),
		GeneratedAt:     time.Now(),
	}
}

// ObservabilitySnapshot is a complete point-in-time view of the system.
type ObservabilitySnapshot struct {
	Freshness       []FreshnessHeatmapEntry `json:"freshness_heatmap"`
	Latency         []LatencySparkline      `json:"latency_sparklines"`
	CostAttribution []CostEntry             `json:"cost_attribution"`
	PipelineDAG     []PipelineNode          `json:"pipeline_dag"`
	GeneratedAt     time.Time               `json:"generated_at"`
}

// RegisterRoutes registers observability console API routes.
func (oc *ObservabilityConsole) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/observability/overview", oc.handleOverview)
	mux.HandleFunc("GET /v1/observability/freshness", oc.handleFreshness)
	mux.HandleFunc("GET /v1/observability/latency", oc.handleLatency)
	mux.HandleFunc("GET /v1/observability/costs", oc.handleCosts)
	mux.HandleFunc("GET /v1/observability/pipeline", oc.handlePipeline)
	mux.HandleFunc("GET /v1/observability/channels", oc.handleChannels)
	mux.HandleFunc("POST /v1/observability/channels", oc.handleAddChannel)
}

func (oc *ObservabilityConsole) handleOverview(w http.ResponseWriter, r *http.Request) {
	snapshot := oc.GetOverviewSnapshot()
	writeObsJSON(w, http.StatusOK, snapshot)
}

func (oc *ObservabilityConsole) handleFreshness(w http.ResponseWriter, r *http.Request) {
	writeObsJSON(w, http.StatusOK, map[string]interface{}{
		"heatmap": oc.GetFreshnessHeatmap(),
	})
}

func (oc *ObservabilityConsole) handleLatency(w http.ResponseWriter, r *http.Request) {
	writeObsJSON(w, http.StatusOK, map[string]interface{}{
		"sparklines": oc.GetLatencySparklines(),
	})
}

func (oc *ObservabilityConsole) handleCosts(w http.ResponseWriter, r *http.Request) {
	writeObsJSON(w, http.StatusOK, map[string]interface{}{
		"costs": oc.GetCostAttribution(),
	})
}

func (oc *ObservabilityConsole) handlePipeline(w http.ResponseWriter, r *http.Request) {
	writeObsJSON(w, http.StatusOK, map[string]interface{}{
		"dag": oc.GetPipelineDAG(),
	})
}

func (oc *ObservabilityConsole) handleChannels(w http.ResponseWriter, r *http.Request) {
	oc.mu.RLock()
	channels := make([]AlertChannel, len(oc.alertChannels))
	copy(channels, oc.alertChannels)
	oc.mu.RUnlock()

	writeObsJSON(w, http.StatusOK, map[string]interface{}{
		"channels": channels,
	})
}

func (oc *ObservabilityConsole) handleAddChannel(w http.ResponseWriter, r *http.Request) {
	var ch AlertChannel
	if err := decodeObsJSON(r, &ch); err != nil {
		writeObsJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if ch.Type == "" || ch.Name == "" {
		writeObsJSON(w, http.StatusBadRequest, map[string]string{"error": "type and name are required"})
		return
	}
	oc.RegisterAlertChannel(ch)
	writeObsJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"channel": ch,
	})
}

func computeHeatmapColor(stalenessMs, slaTargetMs int64) string {
	if slaTargetMs <= 0 {
		slaTargetMs = 60000 // Default 1 minute
	}
	ratio := float64(stalenessMs) / float64(slaTargetMs)
	switch {
	case ratio <= 0.5:
		return "green"
	case ratio <= 1.0:
		return "yellow"
	case ratio <= 2.0:
		return "orange"
	default:
		return "red"
	}
}

func writeObsJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func decodeObsJSON(r *http.Request, v interface{}) error {
	defer func() { _ = r.Body.Close() }()
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}
