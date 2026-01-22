package featuredashboard

import (
	"math"
	"sync"
	"time"
)

// DashboardConfig configures the observability dashboard.
type DashboardConfig struct {
	RetentionPeriod time.Duration `json:"retention_period"`
	MaxFeatures     int           `json:"max_features"`
	SnapshotInterval time.Duration `json:"snapshot_interval"`
}

// DefaultDashboardConfig returns sensible defaults.
func DefaultDashboardConfig() DashboardConfig {
	return DashboardConfig{
		RetentionPeriod:  24 * time.Hour,
		MaxFeatures:      10000,
		SnapshotInterval: 1 * time.Minute,
	}
}

// FeatureHealth represents the health status of a feature.
type FeatureHealth struct {
	Name         string         `json:"name"`
	Status       HealthStatus   `json:"status"`
	Latency      LatencyStats   `json:"latency"`
	Freshness    FreshnessInfo  `json:"freshness"`
	DriftScore   float64        `json:"drift_score"`
	QualityScore float64        `json:"quality_score"`
	RequestRate  float64        `json:"request_rate"`
	ErrorRate    float64        `json:"error_rate"`
	LastUpdated  time.Time      `json:"last_updated"`
	Dependencies []string       `json:"dependencies,omitempty"`
}

// HealthStatus indicates the overall health of a feature.
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusDegraded  HealthStatus = "degraded"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusUnknown   HealthStatus = "unknown"
)

// LatencyStats contains latency percentile information.
type LatencyStats struct {
	P50  float64 `json:"p50_ms"`
	P95  float64 `json:"p95_ms"`
	P99  float64 `json:"p99_ms"`
	P999 float64 `json:"p999_ms"`
	Avg  float64 `json:"avg_ms"`
	Max  float64 `json:"max_ms"`
}

// FreshnessInfo describes the staleness of a feature.
type FreshnessInfo struct {
	LastIngested  time.Time     `json:"last_ingested"`
	StaleDuration time.Duration `json:"stale_duration"`
	SLATarget     time.Duration `json:"sla_target,omitempty"`
	WithinSLA     bool          `json:"within_sla"`
}

// DashboardSnapshot represents a point-in-time view of all features.
type DashboardSnapshot struct {
	Timestamp      time.Time       `json:"timestamp"`
	TotalFeatures  int             `json:"total_features"`
	HealthySummary HealthSummary   `json:"health_summary"`
	Features       []FeatureHealth `json:"features"`
}

// HealthSummary provides aggregate health statistics.
type HealthSummary struct {
	Healthy     int     `json:"healthy"`
	Degraded    int     `json:"degraded"`
	Unhealthy   int     `json:"unhealthy"`
	Unknown     int     `json:"unknown"`
	AvgLatency  float64 `json:"avg_latency_ms"`
	AvgDrift    float64 `json:"avg_drift_score"`
	AvgQuality  float64 `json:"avg_quality_score"`
}

type featureTracker struct {
	name         string
	latencies    []float64
	requests     int64
	errors       int64
	driftScore   float64
	qualityScore float64
	lastIngested time.Time
	slaDuration  time.Duration
	dependencies []string
}

// Dashboard aggregates feature observability metrics.
type Dashboard struct {
	mu       sync.RWMutex
	config   DashboardConfig
	features map[string]*featureTracker
	snapshots []DashboardSnapshot
}

// NewDashboard creates a new observability dashboard.
func NewDashboard(config DashboardConfig) *Dashboard {
	if config.MaxFeatures == 0 {
		config = DefaultDashboardConfig()
	}
	return &Dashboard{
		config:    config,
		features:  make(map[string]*featureTracker),
		snapshots: make([]DashboardSnapshot, 0),
	}
}

// TrackFeature starts tracking a feature.
func (d *Dashboard) TrackFeature(name string, deps []string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.features[name] = &featureTracker{
		name:         name,
		latencies:    make([]float64, 0, 1000),
		dependencies: deps,
	}
}

// RecordLatency records a latency observation for a feature.
func (d *Dashboard) RecordLatency(name string, latencyMs float64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	ft, exists := d.features[name]
	if !exists {
		ft = &featureTracker{name: name, latencies: make([]float64, 0, 1000)}
		d.features[name] = ft
	}

	ft.latencies = append(ft.latencies, latencyMs)
	ft.requests++
	if len(ft.latencies) > 1000 {
		ft.latencies = ft.latencies[1:]
	}
}

// RecordError records an error for a feature.
func (d *Dashboard) RecordError(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if ft, exists := d.features[name]; exists {
		ft.errors++
	}
}

// UpdateDriftScore updates the drift score for a feature.
func (d *Dashboard) UpdateDriftScore(name string, score float64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if ft, exists := d.features[name]; exists {
		ft.driftScore = score
	}
}

// UpdateQualityScore updates the quality score for a feature.
func (d *Dashboard) UpdateQualityScore(name string, score float64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if ft, exists := d.features[name]; exists {
		ft.qualityScore = score
	}
}

// RecordIngestion records an ingestion event for a feature.
func (d *Dashboard) RecordIngestion(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if ft, exists := d.features[name]; exists {
		ft.lastIngested = time.Now()
	}
}

// TakeSnapshot creates a point-in-time snapshot.
func (d *Dashboard) TakeSnapshot() DashboardSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	snapshot := DashboardSnapshot{
		Timestamp:     now,
		TotalFeatures: len(d.features),
		Features:      make([]FeatureHealth, 0, len(d.features)),
	}

	var totalLatency, totalDrift, totalQuality float64
	healthyCount := 0

	for _, ft := range d.features {
		health := d.computeHealth(ft, now)
		snapshot.Features = append(snapshot.Features, health)

		switch health.Status {
		case StatusHealthy:
			snapshot.HealthySummary.Healthy++
		case StatusDegraded:
			snapshot.HealthySummary.Degraded++
		case StatusUnhealthy:
			snapshot.HealthySummary.Unhealthy++
		default:
			snapshot.HealthySummary.Unknown++
		}

		totalLatency += health.Latency.Avg
		totalDrift += health.DriftScore
		totalQuality += health.QualityScore
		healthyCount++
	}

	if healthyCount > 0 {
		snapshot.HealthySummary.AvgLatency = totalLatency / float64(healthyCount)
		snapshot.HealthySummary.AvgDrift = totalDrift / float64(healthyCount)
		snapshot.HealthySummary.AvgQuality = totalQuality / float64(healthyCount)
	}

	d.snapshots = append(d.snapshots, snapshot)
	if len(d.snapshots) > 100 {
		d.snapshots = d.snapshots[1:]
	}

	return snapshot
}

// GetSnapshot returns the latest snapshot.
func (d *Dashboard) GetSnapshot() DashboardSnapshot {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.snapshots) == 0 {
		return d.takeSnapshotLocked()
	}
	return d.snapshots[len(d.snapshots)-1]
}

func (d *Dashboard) takeSnapshotLocked() DashboardSnapshot {
	now := time.Now()
	snapshot := DashboardSnapshot{
		Timestamp:     now,
		TotalFeatures: len(d.features),
		Features:      make([]FeatureHealth, 0, len(d.features)),
	}
	for _, ft := range d.features {
		snapshot.Features = append(snapshot.Features, d.computeHealth(ft, now))
	}
	return snapshot
}

// GetFeatureHealth returns health info for a specific feature.
func (d *Dashboard) GetFeatureHealth(name string) (FeatureHealth, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	ft, exists := d.features[name]
	if !exists {
		return FeatureHealth{}, ErrFeatureNotTracked
	}
	return d.computeHealth(ft, time.Now()), nil
}

// GetHistory returns historical snapshots.
func (d *Dashboard) GetHistory(limit int) []DashboardSnapshot {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if limit <= 0 || limit > len(d.snapshots) {
		limit = len(d.snapshots)
	}
	result := make([]DashboardSnapshot, limit)
	copy(result, d.snapshots[len(d.snapshots)-limit:])
	return result
}

func (d *Dashboard) computeHealth(ft *featureTracker, now time.Time) FeatureHealth {
	health := FeatureHealth{
		Name:         ft.name,
		Status:       StatusUnknown,
		DriftScore:   ft.driftScore,
		QualityScore: ft.qualityScore,
		LastUpdated:  now,
		Dependencies: ft.dependencies,
	}

	if len(ft.latencies) > 0 {
		health.Latency = computeLatencyStats(ft.latencies)
	}

	if !ft.lastIngested.IsZero() {
		stale := now.Sub(ft.lastIngested)
		health.Freshness = FreshnessInfo{
			LastIngested:  ft.lastIngested,
			StaleDuration: stale,
			SLATarget:     ft.slaDuration,
			WithinSLA:     ft.slaDuration == 0 || stale <= ft.slaDuration,
		}
	}

	if ft.requests > 0 {
		health.RequestRate = float64(ft.requests)
		health.ErrorRate = float64(ft.errors) / float64(ft.requests)
	}

	// Determine status
	health.Status = determineStatus(health)

	return health
}

func determineStatus(h FeatureHealth) HealthStatus {
	if h.ErrorRate > 0.05 || h.DriftScore > 0.3 {
		return StatusUnhealthy
	}
	if h.ErrorRate > 0.01 || h.DriftScore > 0.1 || h.Latency.P99 > 100 {
		return StatusDegraded
	}
	if h.RequestRate > 0 || h.Latency.Avg > 0 {
		return StatusHealthy
	}
	return StatusUnknown
}

func computeLatencyStats(latencies []float64) LatencyStats {
	if len(latencies) == 0 {
		return LatencyStats{}
	}

	sorted := make([]float64, len(latencies))
	copy(sorted, latencies)
	// Simple sort for percentiles
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var sum, maxVal float64
	for _, v := range sorted {
		sum += v
		if v > maxVal {
			maxVal = v
		}
	}

	n := len(sorted)
	return LatencyStats{
		P50:  sorted[int(math.Floor(float64(n)*0.50))],
		P95:  sorted[int(math.Min(float64(n-1), math.Floor(float64(n)*0.95)))],
		P99:  sorted[int(math.Min(float64(n-1), math.Floor(float64(n)*0.99)))],
		P999: sorted[int(math.Min(float64(n-1), math.Floor(float64(n)*0.999)))],
		Avg:  sum / float64(n),
		Max:  maxVal,
	}
}
