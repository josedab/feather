package realtimemonitor

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// AlertSeverity represents alert severity levels.
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// AlertStatus represents the current state of an alert.
type AlertStatus string

const (
	AlertActive   AlertStatus = "active"
	AlertResolved AlertStatus = "resolved"
	AlertSilenced AlertStatus = "silenced"
)

// HealthStatus represents component health.
type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
)

// FreshnessMetric tracks how fresh a feature's data is.
type FreshnessMetric struct {
	Feature      string        `json:"feature"`
	Group        string        `json:"group,omitempty"`
	LastUpdated  time.Time     `json:"last_updated"`
	Staleness    time.Duration `json:"staleness_ns"`
	Threshold    time.Duration `json:"threshold_ns"`
	IsStale      bool          `json:"is_stale"`
}

// LatencyMetric tracks serving latency percentiles.
type LatencyMetric struct {
	Endpoint  string        `json:"endpoint"`
	P50       time.Duration `json:"p50_ns"`
	P95       time.Duration `json:"p95_ns"`
	P99       time.Duration `json:"p99_ns"`
	Avg       time.Duration `json:"avg_ns"`
	Max       time.Duration `json:"max_ns"`
	Count     int64         `json:"count"`
	ErrorRate float64       `json:"error_rate"`
	Window    time.Duration `json:"window_ns"`
}

// Alert represents a monitoring alert.
type Alert struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Severity  AlertSeverity `json:"severity"`
	Status    AlertStatus   `json:"status"`
	Message   string        `json:"message"`
	Source    string        `json:"source"`
	FiredAt   time.Time     `json:"fired_at"`
	ResolvedAt *time.Time   `json:"resolved_at,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// PipelineHealth tracks the health of a streaming pipeline.
type PipelineHealth struct {
	PipelineID    string       `json:"pipeline_id"`
	Status        HealthStatus `json:"status"`
	EventsPerSec  float64      `json:"events_per_sec"`
	Lag           int64        `json:"lag"`
	ErrorRate     float64      `json:"error_rate"`
	LastCheckAt   time.Time    `json:"last_check_at"`
}

// DashboardSnapshot is a point-in-time view of all monitoring data.
type DashboardSnapshot struct {
	Timestamp      time.Time          `json:"timestamp"`
	OverallHealth  HealthStatus       `json:"overall_health"`
	Freshness      []FreshnessMetric  `json:"freshness"`
	Latency        []LatencyMetric    `json:"latency"`
	ActiveAlerts   []Alert            `json:"active_alerts"`
	PipelineHealth []PipelineHealth   `json:"pipeline_health"`
	Summary        DashboardSummary   `json:"summary"`
}

// DashboardSummary provides aggregate counts.
type DashboardSummary struct {
	TotalFeatures     int `json:"total_features"`
	StaleFeatures     int `json:"stale_features"`
	TotalAlerts       int `json:"total_alerts"`
	CriticalAlerts    int `json:"critical_alerts"`
	HealthyPipelines  int `json:"healthy_pipelines"`
	TotalPipelines    int `json:"total_pipelines"`
}

// DashboardConfig configures the monitoring dashboard.
type DashboardConfig struct {
	DefaultFreshnessThreshold time.Duration `json:"default_freshness_threshold"`
	LatencyWindow             time.Duration `json:"latency_window"`
	MaxAlerts                 int           `json:"max_alerts"`
	MaxLatencySamples         int           `json:"max_latency_samples"`
}

// DefaultDashboardConfig returns sensible defaults.
func DefaultDashboardConfig() DashboardConfig {
	return DashboardConfig{
		DefaultFreshnessThreshold: 5 * time.Minute,
		LatencyWindow:             1 * time.Minute,
		MaxAlerts:                 10000,
		MaxLatencySamples:         10000,
	}
}

// Dashboard aggregates and serves real-time monitoring metrics.
type Dashboard struct {
	mu             sync.RWMutex
	config         DashboardConfig
	freshness      map[string]*FreshnessMetric  // feature -> metric
	latencySamples map[string][]time.Duration   // endpoint -> samples
	latencyCounts  map[string]int64             // endpoint -> request count
	latencyErrors  map[string]int64             // endpoint -> error count
	alerts         map[string]*Alert            // alert ID -> alert
	pipelines      map[string]*PipelineHealth   // pipeline ID -> health
	alertSeq       int64
}

// NewDashboard creates a new monitoring dashboard.
func NewDashboard(config DashboardConfig) *Dashboard {
	if config.MaxAlerts == 0 {
		config = DefaultDashboardConfig()
	}
	return &Dashboard{
		config:         config,
		freshness:      make(map[string]*FreshnessMetric),
		latencySamples: make(map[string][]time.Duration),
		latencyCounts:  make(map[string]int64),
		latencyErrors:  make(map[string]int64),
		alerts:         make(map[string]*Alert),
		pipelines:      make(map[string]*PipelineHealth),
	}
}

// RecordFreshness updates the freshness metric for a feature.
func (d *Dashboard) RecordFreshness(feature, group string, updatedAt time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	staleness := time.Since(updatedAt)
	threshold := d.config.DefaultFreshnessThreshold

	d.freshness[feature] = &FreshnessMetric{
		Feature:     feature,
		Group:       group,
		LastUpdated: updatedAt,
		Staleness:   staleness,
		Threshold:   threshold,
		IsStale:     staleness > threshold,
	}
}

// RecordLatency records a latency sample for an endpoint.
func (d *Dashboard) RecordLatency(endpoint string, latency time.Duration, isError bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	samples := d.latencySamples[endpoint]
	if len(samples) >= d.config.MaxLatencySamples {
		samples = samples[1:]
	}
	d.latencySamples[endpoint] = append(samples, latency)
	d.latencyCounts[endpoint]++
	if isError {
		d.latencyErrors[endpoint]++
	}
}

// FireAlert creates a new alert.
func (d *Dashboard) FireAlert(name string, severity AlertSeverity, message, source string, labels map[string]string) *Alert {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.alertSeq++
	alert := &Alert{
		ID:       fmt.Sprintf("alert-%d", d.alertSeq),
		Name:     name,
		Severity: severity,
		Status:   AlertActive,
		Message:  message,
		Source:   source,
		FiredAt:  time.Now(),
		Labels:   labels,
	}
	d.alerts[alert.ID] = alert
	return alert
}

// ResolveAlert resolves an active alert.
func (d *Dashboard) ResolveAlert(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	alert, exists := d.alerts[id]
	if !exists {
		return fmt.Errorf("alert %s not found", id)
	}
	now := time.Now()
	alert.Status = AlertResolved
	alert.ResolvedAt = &now
	return nil
}

// UpdatePipelineHealth updates health status for a pipeline.
func (d *Dashboard) UpdatePipelineHealth(health PipelineHealth) {
	d.mu.Lock()
	defer d.mu.Unlock()

	health.LastCheckAt = time.Now()
	d.pipelines[health.PipelineID] = &health
}

// Snapshot returns a point-in-time dashboard snapshot.
func (d *Dashboard) Snapshot() *DashboardSnapshot {
	d.mu.RLock()
	defer d.mu.RUnlock()

	snap := &DashboardSnapshot{
		Timestamp:      time.Now(),
		Freshness:      make([]FreshnessMetric, 0, len(d.freshness)),
		Latency:        make([]LatencyMetric, 0, len(d.latencySamples)),
		ActiveAlerts:   make([]Alert, 0),
		PipelineHealth: make([]PipelineHealth, 0, len(d.pipelines)),
	}

	// Freshness
	for _, m := range d.freshness {
		// Recalculate staleness
		updated := *m
		updated.Staleness = time.Since(m.LastUpdated)
		updated.IsStale = updated.Staleness > m.Threshold
		snap.Freshness = append(snap.Freshness, updated)
		snap.Summary.TotalFeatures++
		if updated.IsStale {
			snap.Summary.StaleFeatures++
		}
	}

	// Latency
	for endpoint, samples := range d.latencySamples {
		if len(samples) == 0 {
			continue
		}
		lm := computeLatencyMetric(endpoint, samples, d.latencyCounts[endpoint], d.latencyErrors[endpoint], d.config.LatencyWindow)
		snap.Latency = append(snap.Latency, lm)
	}

	// Alerts
	for _, a := range d.alerts {
		if a.Status == AlertActive {
			snap.ActiveAlerts = append(snap.ActiveAlerts, *a)
			snap.Summary.TotalAlerts++
			if a.Severity == SeverityCritical {
				snap.Summary.CriticalAlerts++
			}
		}
	}

	// Pipelines
	for _, p := range d.pipelines {
		snap.PipelineHealth = append(snap.PipelineHealth, *p)
		snap.Summary.TotalPipelines++
		if p.Status == HealthHealthy {
			snap.Summary.HealthyPipelines++
		}
	}

	// Overall health
	snap.OverallHealth = HealthHealthy
	if snap.Summary.CriticalAlerts > 0 || snap.Summary.StaleFeatures > snap.Summary.TotalFeatures/2 {
		snap.OverallHealth = HealthUnhealthy
	} else if snap.Summary.TotalAlerts > 0 || snap.Summary.StaleFeatures > 0 {
		snap.OverallHealth = HealthDegraded
	}

	return snap
}

// GetAlerts returns all alerts, optionally filtered by status.
func (d *Dashboard) GetAlerts(status string) []Alert {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]Alert, 0, len(d.alerts))
	for _, a := range d.alerts {
		if status == "" || string(a.Status) == status {
			result = append(result, *a)
		}
	}
	return result
}

func computeLatencyMetric(endpoint string, samples []time.Duration, count, errors int64, window time.Duration) LatencyMetric {
	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var total time.Duration
	for _, s := range sorted {
		total += s
	}

	n := len(sorted)
	errRate := float64(0)
	if count > 0 {
		errRate = float64(errors) / float64(count)
	}

	return LatencyMetric{
		Endpoint:  endpoint,
		P50:       sorted[n/2],
		P95:       sorted[n*95/100],
		P99:       sorted[n*99/100],
		Avg:       total / time.Duration(n),
		Max:       sorted[n-1],
		Count:     count,
		ErrorRate: errRate,
		Window:    window,
	}
}
