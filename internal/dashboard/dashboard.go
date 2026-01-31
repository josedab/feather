package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Dashboard provides the monitoring dashboard backend.
type Dashboard struct {
	config    Config
	alerts    *AlertManager
	analytics *AnalyticsCollector
	mu        sync.RWMutex
}

// Config configures the dashboard.
type Config struct {
	// Store is the feature store interface.
	Store StoreInterface

	// DriftMonitor provides drift detection data.
	DriftMonitor DriftMonitorInterface

	// Registry provides feature schema information.
	Registry RegistryInterface

	// Metrics provides system metrics.
	Metrics MetricsInterface

	// Lineage provides feature lineage data.
	Lineage LineageInterface

	// AlertWebhookURL for alert notifications.
	AlertWebhookURL string

	// RefreshInterval for cached data.
	RefreshInterval time.Duration
}

// StoreInterface defines required store methods.
type StoreInterface interface {
	Get(entityKey string, features []string) (map[string]interface{}, error)
	Stats() map[string]interface{}
}

// DriftMonitorInterface defines drift monitoring methods.
type DriftMonitorInterface interface {
	GetAllStatus() map[string]interface{}
	GetAlerts(since time.Time) []interface{}
}

// RegistryInterface defines registry methods.
type RegistryInterface interface {
	ListGroups() []interface{}
	GetGroup(name string) (interface{}, error)
}

// MetricsInterface defines metrics methods.
type MetricsInterface interface {
	GetSnapshot() map[string]interface{}
}

// LineageInterface defines lineage methods.
type LineageInterface interface {
	GetLineage(featureName string) (interface{}, error)
}

// New creates a new dashboard.
func New(config Config) *Dashboard {
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 30 * time.Second
	}

	return &Dashboard{
		config:    config,
		alerts:    NewAlertManager(config.AlertWebhookURL),
		analytics: NewAnalyticsCollector(),
	}
}

// RegisterRoutes registers dashboard HTTP routes.
func (d *Dashboard) RegisterRoutes(mux *http.ServeMux) {
	// Dashboard UI
	mux.HandleFunc("GET /dashboard", d.handleDashboardIndex)
	mux.HandleFunc("GET /dashboard/", d.handleDashboardStatic)

	// API endpoints
	mux.HandleFunc("GET /api/dashboard/overview", d.handleOverview)
	mux.HandleFunc("GET /api/dashboard/features", d.handleFeatureList)
	mux.HandleFunc("GET /api/dashboard/features/{name}", d.handleFeatureDetail)
	mux.HandleFunc("GET /api/dashboard/features/{name}/drift", d.handleFeatureDrift)
	mux.HandleFunc("GET /api/dashboard/features/{name}/freshness", d.handleFeatureFreshness)
	mux.HandleFunc("GET /api/dashboard/features/{name}/lineage", d.handleFeatureLineage)
	mux.HandleFunc("GET /api/dashboard/drift", d.handleDriftOverview)
	mux.HandleFunc("GET /api/dashboard/drift/alerts", d.handleDriftAlerts)
	mux.HandleFunc("GET /api/dashboard/freshness", d.handleFreshnessOverview)
	mux.HandleFunc("GET /api/dashboard/health", d.handleHealth)
	mux.HandleFunc("GET /api/dashboard/metrics", d.handleMetrics)
	mux.HandleFunc("GET /api/dashboard/alerts", d.handleAlerts)
	mux.HandleFunc("POST /api/dashboard/alerts/{id}/acknowledge", d.handleAcknowledgeAlert)
	mux.HandleFunc("GET /api/dashboard/search", d.handleSearch)
	mux.HandleFunc("GET /api/dashboard/analytics", d.handleAnalytics)
}

// OverviewResponse provides dashboard overview data.
type OverviewResponse struct {
	TotalFeatures   int                    `json:"total_features"`
	TotalGroups     int                    `json:"total_groups"`
	HealthStatus    string                 `json:"health_status"`
	ActiveAlerts    int                    `json:"active_alerts"`
	DriftDetected   int                    `json:"drift_detected"`
	StaleFeatures   int                    `json:"stale_features"`
	RequestsPerSec  float64                `json:"requests_per_sec"`
	AvgLatencyMs    float64                `json:"avg_latency_ms"`
	CacheHitRate    float64                `json:"cache_hit_rate"`
	StorageUsedGB   float64                `json:"storage_used_gb"`
	LastUpdated     time.Time              `json:"last_updated"`
	RecentActivity  []ActivityItem         `json:"recent_activity"`
	SystemMetrics   map[string]interface{} `json:"system_metrics"`
}

// ActivityItem represents a recent activity.
type ActivityItem struct {
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Severity  string    `json:"severity"`
}

func (d *Dashboard) handleOverview(w http.ResponseWriter, r *http.Request) {
	overview := d.buildOverview()
	writeJSON(w, overview)
}

func (d *Dashboard) buildOverview() *OverviewResponse {
	overview := &OverviewResponse{
		LastUpdated:    time.Now(),
		RecentActivity: []ActivityItem{},
		SystemMetrics:  make(map[string]interface{}),
	}

	// Get feature groups
	if d.config.Registry != nil {
		groups := d.config.Registry.ListGroups()
		overview.TotalGroups = len(groups)
		for _, g := range groups {
			if gmap, ok := g.(map[string]interface{}); ok {
				if features, ok := gmap["features"].([]interface{}); ok {
					overview.TotalFeatures += len(features)
				}
			}
		}
	}

	// Get drift status
	if d.config.DriftMonitor != nil {
		status := d.config.DriftMonitor.GetAllStatus()
		for _, s := range status {
			if smap, ok := s.(map[string]interface{}); ok {
				if drifted, ok := smap["drifted"].(bool); ok && drifted {
					overview.DriftDetected++
				}
			}
		}
		alerts := d.config.DriftMonitor.GetAlerts(time.Now().Add(-24 * time.Hour))
		overview.ActiveAlerts = len(alerts)
	}

	// Get metrics
	if d.config.Metrics != nil {
		metrics := d.config.Metrics.GetSnapshot()
		overview.SystemMetrics = metrics

		if rps, ok := metrics["requests_per_second"].(float64); ok {
			overview.RequestsPerSec = rps
		}
		if lat, ok := metrics["avg_latency_ms"].(float64); ok {
			overview.AvgLatencyMs = lat
		}
		if hit, ok := metrics["cache_hit_rate"].(float64); ok {
			overview.CacheHitRate = hit
		}
	}

	// Determine health status
	overview.HealthStatus = "healthy"
	if overview.ActiveAlerts > 5 || overview.DriftDetected > 10 {
		overview.HealthStatus = "warning"
	}
	if overview.ActiveAlerts > 20 || overview.DriftDetected > 50 {
		overview.HealthStatus = "critical"
	}

	return overview
}

// FeatureListItem represents a feature in the list.
type FeatureListItem struct {
	Name        string    `json:"name"`
	Group       string    `json:"group"`
	DataType    string    `json:"data_type"`
	Description string    `json:"description,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	DriftStatus string    `json:"drift_status"`
	Freshness   string    `json:"freshness"`
	LastUpdated time.Time `json:"last_updated"`
	UsageCount  int64     `json:"usage_count"`
}

func (d *Dashboard) handleFeatureList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	search := query.Get("search")
	group := query.Get("group")
	driftStatus := query.Get("drift_status")
	sortBy := query.Get("sort_by")
	if sortBy == "" {
		sortBy = "name"
	}

	features := d.getFeatureList(search, group, driftStatus)

	// Sort features
	sort.Slice(features, func(i, j int) bool {
		switch sortBy {
		case "usage":
			return features[i].UsageCount > features[j].UsageCount
		case "updated":
			return features[i].LastUpdated.After(features[j].LastUpdated)
		case "drift":
			return features[i].DriftStatus > features[j].DriftStatus
		default:
			return features[i].Name < features[j].Name
		}
	})

	writeJSON(w, map[string]interface{}{
		"features": features,
		"total":    len(features),
	})
}

func (d *Dashboard) getFeatureList(search, group, driftStatus string) []FeatureListItem {
	var features []FeatureListItem

	if d.config.Registry == nil {
		return features
	}

	groups := d.config.Registry.ListGroups()
	for _, g := range groups {
		gmap, ok := g.(map[string]interface{})
		if !ok {
			continue
		}

		groupName, _ := gmap["name"].(string)
		if group != "" && groupName != group {
			continue
		}

		featureList, ok := gmap["features"].([]interface{})
		if !ok {
			continue
		}

		for _, f := range featureList {
			fmap, ok := f.(map[string]interface{})
			if !ok {
				continue
			}

			name, _ := fmap["name"].(string)
			if search != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(search)) {
				continue
			}

			item := FeatureListItem{
				Name:        name,
				Group:       groupName,
				DataType:    fmt.Sprintf("%v", fmap["data_type"]),
				Description: fmt.Sprintf("%v", fmap["description"]),
				DriftStatus: "normal",
				Freshness:   "fresh",
				LastUpdated: time.Now(),
			}

			if tags, ok := fmap["tags"].([]string); ok {
				item.Tags = tags
			}

			// Get drift status
			if d.config.DriftMonitor != nil {
				status := d.config.DriftMonitor.GetAllStatus()
				if s, ok := status[name]; ok {
					if smap, ok := s.(map[string]interface{}); ok {
						if drifted, ok := smap["drifted"].(bool); ok && drifted {
							item.DriftStatus = "drifted"
						}
					}
				}
			}

			if driftStatus != "" && item.DriftStatus != driftStatus {
				continue
			}

			features = append(features, item)
		}
	}

	return features
}

// FeatureDetail provides detailed feature information.
type FeatureDetail struct {
	Name           string                 `json:"name"`
	Group          string                 `json:"group"`
	DataType       string                 `json:"data_type"`
	Description    string                 `json:"description"`
	Tags           []string               `json:"tags"`
	Schema         map[string]interface{} `json:"schema"`
	Statistics     *FeatureStatistics     `json:"statistics"`
	DriftInfo      *DriftInfo             `json:"drift_info"`
	FreshnessInfo  *FreshnessInfo         `json:"freshness_info"`
	UsageInfo      *UsageInfo             `json:"usage_info"`
	Lineage        interface{}            `json:"lineage,omitempty"`
	RecentValues   []RecentValue          `json:"recent_values"`
	RelatedFeatures []string              `json:"related_features"`
}

// FeatureStatistics contains feature statistics.
type FeatureStatistics struct {
	Count       int64       `json:"count"`
	NullCount   int64       `json:"null_count"`
	UniqueCount int64       `json:"unique_count"`
	Mean        *float64    `json:"mean,omitempty"`
	Stddev      *float64    `json:"stddev,omitempty"`
	Min         interface{} `json:"min,omitempty"`
	Max         interface{} `json:"max,omitempty"`
	P50         interface{} `json:"p50,omitempty"`
	P95         interface{} `json:"p95,omitempty"`
	P99         interface{} `json:"p99,omitempty"`
}

// DriftInfo contains drift detection information.
type DriftInfo struct {
	Status          string    `json:"status"`
	Score           float64   `json:"score"`
	Threshold       float64   `json:"threshold"`
	DetectedAt      time.Time `json:"detected_at,omitempty"`
	ReferenceWindow string    `json:"reference_window"`
	CurrentWindow   string    `json:"current_window"`
	Method          string    `json:"method"`
	History         []DriftPoint `json:"history"`
}

// DriftPoint represents a point in drift history.
type DriftPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Score     float64   `json:"score"`
	Drifted   bool      `json:"drifted"`
}

// FreshnessInfo contains feature freshness information.
type FreshnessInfo struct {
	Status        string        `json:"status"`
	LastUpdated   time.Time     `json:"last_updated"`
	UpdateLatency time.Duration `json:"update_latency"`
	ExpectedTTL   time.Duration `json:"expected_ttl"`
	SLATarget     time.Duration `json:"sla_target"`
	SLACompliance float64       `json:"sla_compliance"`
}

// UsageInfo contains feature usage information.
type UsageInfo struct {
	TotalRequests   int64              `json:"total_requests"`
	RequestsPerHour float64            `json:"requests_per_hour"`
	UniqueEntities  int64              `json:"unique_entities"`
	TopConsumers    []ConsumerInfo     `json:"top_consumers"`
	UsageByDay      []DailyUsage       `json:"usage_by_day"`
}

// ConsumerInfo identifies a feature consumer.
type ConsumerInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Requests int64  `json:"requests"`
}

// DailyUsage tracks daily usage.
type DailyUsage struct {
	Date     string `json:"date"`
	Requests int64  `json:"requests"`
}

// RecentValue represents a recent feature value.
type RecentValue struct {
	EntityKey string      `json:"entity_key"`
	Value     interface{} `json:"value"`
	Timestamp time.Time   `json:"timestamp"`
}

func (d *Dashboard) handleFeatureDetail(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, "feature name required", http.StatusBadRequest)
		return
	}

	detail := d.getFeatureDetail(name)
	if detail == nil {
		http.Error(w, "feature not found", http.StatusNotFound)
		return
	}

	writeJSON(w, detail)
}

func (d *Dashboard) getFeatureDetail(name string) *FeatureDetail {
	detail := &FeatureDetail{
		Name:          name,
		Statistics:    &FeatureStatistics{},
		DriftInfo:     &DriftInfo{Status: "unknown"},
		FreshnessInfo: &FreshnessInfo{Status: "unknown"},
		UsageInfo:     &UsageInfo{},
		RecentValues:  []RecentValue{},
	}

	// Get from registry
	if d.config.Registry != nil {
		groups := d.config.Registry.ListGroups()
		for _, g := range groups {
			gmap, ok := g.(map[string]interface{})
			if !ok {
				continue
			}

			groupName, _ := gmap["name"].(string)
			features, ok := gmap["features"].([]interface{})
			if !ok {
				continue
			}

			for _, f := range features {
				fmap, ok := f.(map[string]interface{})
				if !ok {
					continue
				}

				if fmap["name"] == name {
					detail.Group = groupName
					detail.DataType = fmt.Sprintf("%v", fmap["data_type"])
					detail.Description = fmt.Sprintf("%v", fmap["description"])
					detail.Schema = fmap
					break
				}
			}
		}
	}

	// Get lineage
	if d.config.Lineage != nil {
		if lineage, err := d.config.Lineage.GetLineage(name); err == nil {
			detail.Lineage = lineage
		}
	}

	return detail
}

func (d *Dashboard) handleFeatureDrift(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, "feature name required", http.StatusBadRequest)
		return
	}

	driftInfo := &DriftInfo{
		Status:    "normal",
		Score:     0.05,
		Threshold: 0.1,
		Method:    "psi",
		History:   []DriftPoint{},
	}

	if d.config.DriftMonitor != nil {
		status := d.config.DriftMonitor.GetAllStatus()
		if s, ok := status[name]; ok {
			if smap, ok := s.(map[string]interface{}); ok {
				if score, ok := smap["score"].(float64); ok {
					driftInfo.Score = score
				}
				if drifted, ok := smap["drifted"].(bool); ok && drifted {
					driftInfo.Status = "drifted"
					driftInfo.DetectedAt = time.Now()
				}
			}
		}
	}

	writeJSON(w, driftInfo)
}

func (d *Dashboard) handleFeatureFreshness(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, "feature name required", http.StatusBadRequest)
		return
	}

	freshness := &FreshnessInfo{
		Status:        "fresh",
		LastUpdated:   time.Now().Add(-5 * time.Minute),
		UpdateLatency: 5 * time.Minute,
		ExpectedTTL:   1 * time.Hour,
		SLATarget:     15 * time.Minute,
		SLACompliance: 0.99,
	}

	writeJSON(w, freshness)
}

func (d *Dashboard) handleFeatureLineage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, "feature name required", http.StatusBadRequest)
		return
	}

	lineage := map[string]interface{}{
		"feature": name,
		"sources": []map[string]interface{}{
			{"type": "kafka", "name": "events-topic"},
		},
		"transformations": []map[string]interface{}{
			{"type": "aggregation", "name": "hourly_sum"},
		},
		"consumers": []map[string]interface{}{
			{"type": "model", "name": "recommendation-model"},
		},
	}

	if d.config.Lineage != nil {
		if l, err := d.config.Lineage.GetLineage(name); err == nil {
			lineage = l.(map[string]interface{})
		}
	}

	writeJSON(w, lineage)
}

func (d *Dashboard) handleDriftOverview(w http.ResponseWriter, r *http.Request) {
	overview := map[string]interface{}{
		"total_monitored":  0,
		"drifted_count":    0,
		"warning_count":    0,
		"healthy_count":    0,
		"features_by_status": map[string][]string{
			"drifted": {},
			"warning": {},
			"healthy": {},
		},
		"recent_drifts": []interface{}{},
	}

	if d.config.DriftMonitor != nil {
		status := d.config.DriftMonitor.GetAllStatus()
		overview["total_monitored"] = len(status)

		drifted := make([]string, 0)
		healthy := make([]string, 0)

		for name, s := range status {
			if smap, ok := s.(map[string]interface{}); ok {
				if isDrifted, ok := smap["drifted"].(bool); ok && isDrifted {
					drifted = append(drifted, name)
				} else {
					healthy = append(healthy, name)
				}
			}
		}

		overview["drifted_count"] = len(drifted)
		overview["healthy_count"] = len(healthy)
		overview["features_by_status"] = map[string][]string{
			"drifted": drifted,
			"healthy": healthy,
		}
	}

	writeJSON(w, overview)
}

func (d *Dashboard) handleDriftAlerts(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-24 * time.Hour)
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = t
		}
	}

	alerts := []interface{}{}
	if d.config.DriftMonitor != nil {
		alerts = d.config.DriftMonitor.GetAlerts(since)
	}

	writeJSON(w, map[string]interface{}{
		"alerts": alerts,
		"since":  since,
	})
}

func (d *Dashboard) handleFreshnessOverview(w http.ResponseWriter, r *http.Request) {
	overview := map[string]interface{}{
		"total_features": 0,
		"fresh_count":    0,
		"stale_count":    0,
		"unknown_count":  0,
		"sla_compliance": 0.99,
		"heatmap":        []interface{}{},
	}

	writeJSON(w, overview)
}

func (d *Dashboard) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status": "healthy",
		"components": map[string]interface{}{
			"store":         "healthy",
			"drift_monitor": "healthy",
			"registry":      "healthy",
		},
		"uptime":       time.Since(time.Now()).String(),
		"version":      "1.0.0",
		"last_checked": time.Now(),
	}

	writeJSON(w, health)
}

func (d *Dashboard) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := map[string]interface{}{
		"requests_per_second": 1000.0,
		"avg_latency_ms":      0.5,
		"p99_latency_ms":      2.0,
		"cache_hit_rate":      0.95,
		"storage_used_bytes":  1024 * 1024 * 1024,
		"active_connections":  100,
	}

	if d.config.Metrics != nil {
		metrics = d.config.Metrics.GetSnapshot()
	}

	writeJSON(w, metrics)
}

func (d *Dashboard) handleAlerts(w http.ResponseWriter, r *http.Request) {
	alerts := d.alerts.GetAll()
	writeJSON(w, map[string]interface{}{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

func (d *Dashboard) handleAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "alert id required", http.StatusBadRequest)
		return
	}

	if err := d.alerts.Acknowledge(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]string{"status": "acknowledged"})
}

func (d *Dashboard) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, map[string]interface{}{"results": []interface{}{}})
		return
	}

	results := d.searchFeatures(query)
	writeJSON(w, map[string]interface{}{
		"query":   query,
		"results": results,
	})
}

func (d *Dashboard) searchFeatures(query string) []FeatureListItem {
	return d.getFeatureList(query, "", "")
}

func (d *Dashboard) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	analytics := d.analytics.GetAnalytics()
	writeJSON(w, analytics)
}

func (d *Dashboard) handleDashboardIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(dashboardHTML))
}

func (d *Dashboard) handleDashboardStatic(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/dashboard/")
	if path == "" || path == "/" {
		d.handleDashboardIndex(w, r)
		return
	}

	// Serve static files or return index for SPA routing
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(dashboardHTML))
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// Dashboard HTML template
const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Feather Dashboard</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <script src="https://unpkg.com/react@18/umd/react.production.min.js"></script>
    <script src="https://unpkg.com/react-dom@18/umd/react-dom.production.min.js"></script>
    <script src="https://unpkg.com/@babel/standalone/babel.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
</head>
<body class="bg-gray-100">
    <div id="root"></div>
    <script type="text/babel">
        const { useState, useEffect } = React;

        function App() {
            const [overview, setOverview] = useState(null);
            const [features, setFeatures] = useState([]);
            const [searchQuery, setSearchQuery] = useState('');
            const [selectedFeature, setSelectedFeature] = useState(null);
            const [activeTab, setActiveTab] = useState('overview');

            useEffect(() => {
                fetchOverview();
                fetchFeatures();
                const interval = setInterval(fetchOverview, 30000);
                return () => clearInterval(interval);
            }, []);

            const fetchOverview = async () => {
                try {
                    const res = await fetch('/api/dashboard/overview');
                    const data = await res.json();
                    setOverview(data);
                } catch (err) {
                    console.error('Error fetching overview:', err);
                }
            };

            const fetchFeatures = async () => {
                try {
                    const res = await fetch('/api/dashboard/features');
                    const data = await res.json();
                    setFeatures(data.features || []);
                } catch (err) {
                    console.error('Error fetching features:', err);
                }
            };

            const filteredFeatures = features.filter(f =>
                f.name.toLowerCase().includes(searchQuery.toLowerCase())
            );

            return (
                <div className="min-h-screen">
                    <nav className="bg-indigo-600 text-white p-4">
                        <div className="container mx-auto flex justify-between items-center">
                            <h1 className="text-2xl font-bold">Feather Dashboard</h1>
                            <div className="flex space-x-4">
                                <button
                                    onClick={() => setActiveTab('overview')}
                                    className={"px-3 py-1 rounded " + (activeTab === 'overview' ? 'bg-indigo-800' : 'hover:bg-indigo-700')}
                                >Overview</button>
                                <button
                                    onClick={() => setActiveTab('features')}
                                    className={"px-3 py-1 rounded " + (activeTab === 'features' ? 'bg-indigo-800' : 'hover:bg-indigo-700')}
                                >Features</button>
                                <button
                                    onClick={() => setActiveTab('drift')}
                                    className={"px-3 py-1 rounded " + (activeTab === 'drift' ? 'bg-indigo-800' : 'hover:bg-indigo-700')}
                                >Drift</button>
                                <button
                                    onClick={() => setActiveTab('alerts')}
                                    className={"px-3 py-1 rounded " + (activeTab === 'alerts' ? 'bg-indigo-800' : 'hover:bg-indigo-700')}
                                >Alerts</button>
                            </div>
                        </div>
                    </nav>

                    <main className="container mx-auto p-6">
                        {activeTab === 'overview' && overview && (
                            <div>
                                <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
                                    <StatCard title="Total Features" value={overview.total_features} />
                                    <StatCard title="Health Status" value={overview.health_status}
                                        className={overview.health_status === 'healthy' ? 'text-green-600' : 'text-red-600'} />
                                    <StatCard title="Active Alerts" value={overview.active_alerts}
                                        className={overview.active_alerts > 0 ? 'text-red-600' : 'text-green-600'} />
                                    <StatCard title="Drift Detected" value={overview.drift_detected} />
                                </div>
                                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                    <MetricsCard
                                        title="Performance"
                                        metrics={[
                                            {label: 'Requests/sec', value: overview.requests_per_sec?.toFixed(0) || 0},
                                            {label: 'Avg Latency', value: (overview.avg_latency_ms?.toFixed(2) || 0) + 'ms'},
                                            {label: 'Cache Hit Rate', value: ((overview.cache_hit_rate || 0) * 100).toFixed(1) + '%'}
                                        ]}
                                    />
                                    <MetricsCard
                                        title="System"
                                        metrics={[
                                            {label: 'Total Groups', value: overview.total_groups},
                                            {label: 'Storage Used', value: overview.storage_used_gb?.toFixed(2) + ' GB'},
                                            {label: 'Stale Features', value: overview.stale_features || 0}
                                        ]}
                                    />
                                </div>
                            </div>
                        )}

                        {activeTab === 'features' && (
                            <div>
                                <div className="mb-4">
                                    <input
                                        type="text"
                                        placeholder="Search features..."
                                        className="w-full p-3 border rounded-lg"
                                        value={searchQuery}
                                        onChange={(e) => setSearchQuery(e.target.value)}
                                    />
                                </div>
                                <div className="bg-white rounded-lg shadow overflow-hidden">
                                    <table className="w-full">
                                        <thead className="bg-gray-50">
                                            <tr>
                                                <th className="px-4 py-3 text-left">Name</th>
                                                <th className="px-4 py-3 text-left">Group</th>
                                                <th className="px-4 py-3 text-left">Type</th>
                                                <th className="px-4 py-3 text-left">Drift</th>
                                                <th className="px-4 py-3 text-left">Freshness</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {filteredFeatures.map(f => (
                                                <tr key={f.name} className="border-t hover:bg-gray-50 cursor-pointer"
                                                    onClick={() => setSelectedFeature(f)}>
                                                    <td className="px-4 py-3 font-medium">{f.name}</td>
                                                    <td className="px-4 py-3 text-gray-600">{f.group}</td>
                                                    <td className="px-4 py-3 text-gray-600">{f.data_type}</td>
                                                    <td className="px-4 py-3">
                                                        <span className={"px-2 py-1 rounded text-sm " +
                                                            (f.drift_status === 'normal' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800')}>
                                                            {f.drift_status}
                                                        </span>
                                                    </td>
                                                    <td className="px-4 py-3">
                                                        <span className={"px-2 py-1 rounded text-sm " +
                                                            (f.freshness === 'fresh' ? 'bg-green-100 text-green-800' : 'bg-yellow-100 text-yellow-800')}>
                                                            {f.freshness}
                                                        </span>
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            </div>
                        )}

                        {activeTab === 'drift' && (
                            <DriftPanel />
                        )}

                        {activeTab === 'alerts' && (
                            <AlertsPanel />
                        )}
                    </main>
                </div>
            );
        }

        function StatCard({ title, value, className = '' }) {
            return (
                <div className="bg-white rounded-lg shadow p-4">
                    <h3 className="text-gray-500 text-sm">{title}</h3>
                    <p className={"text-2xl font-bold " + className}>{value}</p>
                </div>
            );
        }

        function MetricsCard({ title, metrics }) {
            return (
                <div className="bg-white rounded-lg shadow p-4">
                    <h3 className="font-bold mb-3">{title}</h3>
                    <div className="space-y-2">
                        {metrics.map((m, i) => (
                            <div key={i} className="flex justify-between">
                                <span className="text-gray-600">{m.label}</span>
                                <span className="font-medium">{m.value}</span>
                            </div>
                        ))}
                    </div>
                </div>
            );
        }

        function DriftPanel() {
            const [driftData, setDriftData] = useState(null);

            useEffect(() => {
                fetch('/api/dashboard/drift')
                    .then(res => res.json())
                    .then(data => setDriftData(data));
            }, []);

            if (!driftData) return <div>Loading...</div>;

            return (
                <div>
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
                        <StatCard title="Monitored Features" value={driftData.total_monitored} />
                        <StatCard title="Drifted" value={driftData.drifted_count} className="text-red-600" />
                        <StatCard title="Healthy" value={driftData.healthy_count} className="text-green-600" />
                    </div>
                    {driftData.drifted_count > 0 && (
                        <div className="bg-white rounded-lg shadow p-4">
                            <h3 className="font-bold mb-3">Drifted Features</h3>
                            <ul className="space-y-2">
                                {(driftData.features_by_status?.drifted || []).map(f => (
                                    <li key={f} className="flex items-center text-red-600">
                                        <span className="mr-2">⚠️</span> {f}
                                    </li>
                                ))}
                            </ul>
                        </div>
                    )}
                </div>
            );
        }

        function AlertsPanel() {
            const [alerts, setAlerts] = useState([]);

            useEffect(() => {
                fetch('/api/dashboard/alerts')
                    .then(res => res.json())
                    .then(data => setAlerts(data.alerts || []));
            }, []);

            const acknowledgeAlert = async (id) => {
                await fetch('/api/dashboard/alerts/' + id + '/acknowledge', { method: 'POST' });
                setAlerts(alerts.filter(a => a.id !== id));
            };

            return (
                <div className="bg-white rounded-lg shadow">
                    <div className="p-4 border-b">
                        <h3 className="font-bold">Active Alerts ({alerts.length})</h3>
                    </div>
                    {alerts.length === 0 ? (
                        <div className="p-8 text-center text-gray-500">No active alerts</div>
                    ) : (
                        <ul>
                            {alerts.map(alert => (
                                <li key={alert.id} className="p-4 border-b flex justify-between items-center">
                                    <div>
                                        <span className={"px-2 py-1 rounded text-sm mr-2 " +
                                            (alert.severity === 'critical' ? 'bg-red-100 text-red-800' : 'bg-yellow-100 text-yellow-800')}>
                                            {alert.severity}
                                        </span>
                                        <span className="font-medium">{alert.title}</span>
                                        <p className="text-sm text-gray-600">{alert.message}</p>
                                    </div>
                                    <button
                                        onClick={() => acknowledgeAlert(alert.id)}
                                        className="px-3 py-1 bg-gray-200 rounded hover:bg-gray-300"
                                    >Acknowledge</button>
                                </li>
                            ))}
                        </ul>
                    )}
                </div>
            );
        }

        ReactDOM.createRoot(document.getElementById('root')).render(<App />);
    </script>
</body>
</html>`
