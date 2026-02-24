package obsconsole

import (
	"fmt"
	"sync"
	"time"
)

// ConsoleConfig configures the observability console.
type ConsoleConfig struct {
	RefreshInterval time.Duration `json:"refresh_interval" yaml:"refresh_interval"`
	RetentionDays   int           `json:"retention_days" yaml:"retention_days"`
}

// DefaultConsoleConfig returns sensible defaults.
func DefaultConsoleConfig() ConsoleConfig {
	return ConsoleConfig{
		RefreshInterval: 30 * time.Second,
		RetentionDays:   30,
	}
}

// AlertSeverity classifies alert urgency.
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// Alert represents an observability alert.
type Alert struct {
	ID        string        `json:"id"`
	Type      string        `json:"type"`
	Severity  AlertSeverity `json:"severity"`
	Feature   string        `json:"feature"`
	Message   string        `json:"message"`
	Timestamp time.Time     `json:"timestamp"`
	Resolved  bool          `json:"resolved"`
}

// FreshnessStatus tracks feature freshness.
type FreshnessStatus struct {
	Feature    string        `json:"feature"`
	LastUpdate time.Time     `json:"last_update"`
	SLATarget  time.Duration `json:"sla_target"`
	IsStale    bool          `json:"is_stale"`
	StaleSince *time.Time    `json:"stale_since,omitempty"`
}

// QualityScore represents a feature quality assessment.
type QualityScore struct {
	Feature       string  `json:"feature"`
	Completeness  float64 `json:"completeness"`
	Consistency   float64 `json:"consistency"`
	Timeliness    float64 `json:"timeliness"`
	OverallScore  float64 `json:"overall_score"`
}

// DashboardSnapshot is a point-in-time view of all observability metrics.
type DashboardSnapshot struct {
	Timestamp       time.Time         `json:"timestamp"`
	TotalFeatures   int               `json:"total_features"`
	HealthyFeatures int               `json:"healthy_features"`
	StaleFeatures   int               `json:"stale_features"`
	DriftAlerts     int               `json:"drift_alerts"`
	ActiveAlerts    int               `json:"active_alerts"`
	AvgQuality      float64           `json:"avg_quality"`
	Freshness       []FreshnessStatus `json:"freshness"`
	Quality         []QualityScore    `json:"quality"`
	RecentAlerts    []Alert           `json:"recent_alerts"`
	CostByFeature   map[string]float64 `json:"cost_by_feature,omitempty"`
}

// Console aggregates observability metrics into a unified view.
type Console struct {
	config    ConsoleConfig
	mu        sync.RWMutex
	alerts    []Alert
	freshness map[string]*FreshnessStatus
	quality   map[string]*QualityScore
	costs     map[string]float64
	nextAlertID int
}

// NewConsole creates a new observability console.
func NewConsole(cfg ConsoleConfig) *Console {
	return &Console{
		config:    cfg,
		alerts:    make([]Alert, 0),
		freshness: make(map[string]*FreshnessStatus),
		quality:   make(map[string]*QualityScore),
		costs:     make(map[string]float64),
	}
}

// RegisterFeature registers a feature for monitoring.
func (c *Console) RegisterFeature(feature string, slaDuration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.freshness[feature] = &FreshnessStatus{
		Feature:    feature,
		LastUpdate: time.Now(),
		SLATarget:  slaDuration,
	}
	c.quality[feature] = &QualityScore{
		Feature:      feature,
		Completeness: 1.0,
		Consistency:  1.0,
		Timeliness:   1.0,
		OverallScore: 1.0,
	}
}

// RecordUpdate marks a feature as updated.
func (c *Console) RecordUpdate(feature string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if f, ok := c.freshness[feature]; ok {
		f.LastUpdate = time.Now()
		f.IsStale = false
		f.StaleSince = nil
	}
}

// UpdateQuality updates quality scores for a feature.
// Scores are clamped to [0, 1].
func (c *Console) UpdateQuality(feature string, completeness, consistency, timeliness float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	q, ok := c.quality[feature]
	if !ok {
		q = &QualityScore{Feature: feature}
		c.quality[feature] = q
	}
	q.Completeness = clamp01(completeness)
	q.Consistency = clamp01(consistency)
	q.Timeliness = clamp01(timeliness)
	q.OverallScore = (q.Completeness + q.Consistency + q.Timeliness) / 3.0
}

// AddAlert creates a new alert.
func (c *Console) AddAlert(alertType string, severity AlertSeverity, feature, message string) *Alert {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextAlertID++
	alert := Alert{
		ID:        fmt.Sprintf("alert-%d", c.nextAlertID),
		Type:      alertType,
		Severity:  severity,
		Feature:   feature,
		Message:   message,
		Timestamp: time.Now(),
	}
	c.alerts = append(c.alerts, alert)
	return &alert
}

// ResolveAlert marks an alert as resolved.
func (c *Console) ResolveAlert(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.alerts {
		if c.alerts[i].ID == id {
			c.alerts[i].Resolved = true
			return true
		}
	}
	return false
}

// SetCost records cost for a feature.
func (c *Console) SetCost(feature string, cost float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.costs[feature] = cost
}

// GetSnapshot returns a point-in-time dashboard view.
func (c *Console) GetSnapshot() *DashboardSnapshot {
	c.mu.Lock() // Write lock needed: we update staleness state.
	defer c.mu.Unlock()

	snap := &DashboardSnapshot{
		Timestamp:     time.Now(),
		TotalFeatures: len(c.freshness),
		CostByFeature: make(map[string]float64),
	}

	now := time.Now()
	for _, f := range c.freshness {
		if now.Sub(f.LastUpdate) > f.SLATarget && f.SLATarget > 0 {
			f.IsStale = true
			if f.StaleSince == nil {
				staleSince := now
				f.StaleSince = &staleSince
			}
			snap.StaleFeatures++
		} else {
			f.IsStale = false
			f.StaleSince = nil
			snap.HealthyFeatures++
		}
		snap.Freshness = append(snap.Freshness, *f)
	}

	var totalQuality float64
	for _, q := range c.quality {
		snap.Quality = append(snap.Quality, *q)
		totalQuality += q.OverallScore
	}
	if len(c.quality) > 0 {
		snap.AvgQuality = totalQuality / float64(len(c.quality))
	}

	for _, a := range c.alerts {
		if !a.Resolved {
			snap.ActiveAlerts++
			if a.Type == "drift" {
				snap.DriftAlerts++
			}
		}
	}

	// Recent alerts (last 10).
	start := len(c.alerts) - 10
	if start < 0 {
		start = 0
	}
	snap.RecentAlerts = make([]Alert, len(c.alerts[start:]))
	copy(snap.RecentAlerts, c.alerts[start:])

	for k, v := range c.costs {
		snap.CostByFeature[k] = v
	}

	return snap
}

// GetAlerts returns alerts, optionally filtered.
func (c *Console) GetAlerts(activeOnly bool) []Alert {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result []Alert
	for _, a := range c.alerts {
		if activeOnly && a.Resolved {
			continue
		}
		result = append(result, a)
	}
	return result
}

// GenerateGrafanaDashboard returns a Grafana dashboard JSON template.
func (c *Console) GenerateGrafanaDashboard() string {
	return grafanaDashboardTemplate
}

var grafanaDashboardTemplate = `{
  "dashboard": {
    "title": "Feather Feature Store",
    "panels": [
      {
        "title": "Feature Freshness",
        "type": "stat",
        "targets": [{"expr": "feather_feature_freshness_seconds"}]
      },
      {
        "title": "Drift Alerts",
        "type": "graph",
        "targets": [{"expr": "feather_drift_alerts_total"}]
      },
      {
        "title": "Quality Scores",
        "type": "gauge",
        "targets": [{"expr": "feather_quality_score"}]
      },
      {
        "title": "Cost Attribution",
        "type": "piechart",
        "targets": [{"expr": "feather_cost_per_feature"}]
      },
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [{"expr": "rate(feather_requests_total[5m])"}]
      },
      {
        "title": "P99 Latency",
        "type": "graph",
        "targets": [{"expr": "histogram_quantile(0.99, feather_request_duration_seconds_bucket)"}]
      }
    ]
  }
}`

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
