package server

import (
	"context"
	"math"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/core/metrics"
	"github.com/feather-store/feather/internal/core/storage"
)

// DashboardHandler provides comprehensive monitoring dashboard APIs.
type DashboardHandler struct {
	overview *DashboardOverview
	store    *storage.Store
	metrics  *metrics.Metrics
}

// NewDashboardHandler creates a new dashboard handler.
func NewDashboardHandler(store *storage.Store, m *metrics.Metrics) *DashboardHandler {
	return &DashboardHandler{
		overview: NewDashboardOverview(),
		store:    store,
		metrics:  m,
	}
}

// RegisterRoutes registers dashboard API routes.
func (h *DashboardHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/dashboard/overview", h.handleOverview)
	mux.HandleFunc("GET /v1/dashboard/health", h.handleHealth)
	mux.HandleFunc("GET /v1/dashboard/features/stats", h.handleFeatureStats)
	mux.HandleFunc("GET /v1/dashboard/features/top", h.handleTopFeatures)
	mux.HandleFunc("GET /v1/dashboard/latency", h.handleLatency)
	mux.HandleFunc("GET /v1/dashboard/throughput", h.handleThroughput)
	mux.HandleFunc("GET /v1/dashboard/storage", h.handleStorage)
	mux.HandleFunc("GET /v1/dashboard/drift/summary", h.handleDriftSummary)
	mux.HandleFunc("GET /v1/dashboard/freshness/summary", h.handleFreshnessSummary)
	mux.HandleFunc("GET /v1/dashboard/alerts/recent", h.handleRecentAlerts)
	mux.HandleFunc("GET /v1/dashboard/timeline", h.handleTimeline)
}

// --- Overview tracking types ---

// DashboardOverview tracks system-wide dashboard state.
type DashboardOverview struct {
	startTime      time.Time
	featureStats   *FeatureDashboardStats
	latencyTracker *LatencyTracker
	throughput     *ThroughputTracker
	alerts         []*DashboardAlert
	timeline       []*TimelineEvent
	mu             sync.RWMutex
}

// FeatureDashboardStats tracks feature-level statistics.
type FeatureDashboardStats struct {
	TotalFeatures int64            `json:"total_features"`
	ByType        map[string]int64 `json:"by_type"`
	ByGroup       map[string]int64 `json:"by_group"`
	StaleCount    int64            `json:"stale_count"`
	mu            sync.RWMutex
}

// LatencyTracker tracks read and write latencies.
type LatencyTracker struct {
	reads  *LatencyHistogram
	writes *LatencyHistogram
}

// LatencyHistogram tracks latency distribution using fixed-width buckets.
// Buckets represent microsecond ranges: [0,100), [100,500), [500,1000),
// [1000,5000), [5000,10000), [10000,50000), [50000,100000), [100000+).
type LatencyHistogram struct {
	buckets []int64
	sum     int64
	count   int64
	mu      sync.Mutex
}

// ThroughputTracker records time-series throughput data.
type ThroughputTracker struct {
	intervals []ThroughputInterval
	reads     int64
	writes    int64
	mu        sync.RWMutex
}

// ThroughputInterval is a single throughput measurement.
type ThroughputInterval struct {
	Timestamp time.Time `json:"timestamp"`
	Reads     int64     `json:"reads"`
	Writes    int64     `json:"writes"`
}

// DashboardAlert represents a recent alert.
type DashboardAlert struct {
	ID        string    `json:"id"`
	Severity  string    `json:"severity"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
}

// TimelineEvent represents a system activity event.
type TimelineEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

var latencyBucketBounds = []int64{100, 500, 1000, 5000, 10000, 50000, 100000}

// NewLatencyHistogram creates a new latency histogram.
func NewLatencyHistogram() *LatencyHistogram {
	return &LatencyHistogram{
		buckets: make([]int64, len(latencyBucketBounds)+1),
	}
}

// Record records a latency value in microseconds.
func (h *LatencyHistogram) Record(microseconds int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	atomic.AddInt64(&h.sum, microseconds)
	atomic.AddInt64(&h.count, 1)

	idx := len(latencyBucketBounds)
	for i, bound := range latencyBucketBounds {
		if microseconds < bound {
			idx = i
			break
		}
	}
	h.buckets[idx]++
}

// Percentile returns the p-th percentile (0-100) latency in microseconds.
func (h *LatencyHistogram) Percentile(p float64) int64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	total := int64(0)
	for _, c := range h.buckets {
		total += c
	}
	if total == 0 {
		return 0
	}

	threshold := int64(math.Ceil(float64(total) * p / 100.0))
	cumulative := int64(0)
	for i, c := range h.buckets {
		cumulative += c
		if cumulative >= threshold {
			if i < len(latencyBucketBounds) {
				return latencyBucketBounds[i]
			}
			return latencyBucketBounds[len(latencyBucketBounds)-1] * 2
		}
	}
	return 0
}

// Stats returns summary stats for the histogram.
func (h *LatencyHistogram) Stats() (count, sum int64) {
	return atomic.LoadInt64(&h.count), atomic.LoadInt64(&h.sum)
}

// NewDashboardOverview creates a new DashboardOverview.
func NewDashboardOverview() *DashboardOverview {
	return &DashboardOverview{
		startTime: time.Now(),
		featureStats: &FeatureDashboardStats{
			ByType:  make(map[string]int64),
			ByGroup: make(map[string]int64),
		},
		latencyTracker: &LatencyTracker{
			reads:  NewLatencyHistogram(),
			writes: NewLatencyHistogram(),
		},
		throughput: &ThroughputTracker{
			intervals: make([]ThroughputInterval, 0),
		},
		alerts:   make([]*DashboardAlert, 0),
		timeline: make([]*TimelineEvent, 0),
	}
}

// RecordReadLatency records a read latency value.
func (o *DashboardOverview) RecordReadLatency(microseconds int64) {
	o.latencyTracker.reads.Record(microseconds)
}

// RecordWriteLatency records a write latency value.
func (o *DashboardOverview) RecordWriteLatency(microseconds int64) {
	o.latencyTracker.writes.Record(microseconds)
}

// RecordRead increments the read counter.
func (o *DashboardOverview) RecordRead() {
	o.throughput.mu.Lock()
	defer o.throughput.mu.Unlock()
	atomic.AddInt64(&o.throughput.reads, 1)
}

// RecordWrite increments the write counter.
func (o *DashboardOverview) RecordWrite() {
	o.throughput.mu.Lock()
	defer o.throughput.mu.Unlock()
	atomic.AddInt64(&o.throughput.writes, 1)
}

// AddAlert adds a dashboard alert.
func (o *DashboardOverview) AddAlert(alert *DashboardAlert) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.alerts = append(o.alerts, alert)
	// Keep last 100 alerts
	if len(o.alerts) > 100 {
		o.alerts = o.alerts[len(o.alerts)-100:]
	}
}

// AddTimelineEvent adds a timeline event.
func (o *DashboardOverview) AddTimelineEvent(event *TimelineEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.timeline = append(o.timeline, event)
	// Keep last 200 events
	if len(o.timeline) > 200 {
		o.timeline = o.timeline[len(o.timeline)-200:]
	}
}

// SnapshotThroughput captures current throughput counters as an interval.
func (o *DashboardOverview) SnapshotThroughput() {
	o.throughput.mu.Lock()
	defer o.throughput.mu.Unlock()

	interval := ThroughputInterval{
		Timestamp: time.Now(),
		Reads:     atomic.SwapInt64(&o.throughput.reads, 0),
		Writes:    atomic.SwapInt64(&o.throughput.writes, 0),
	}
	o.throughput.intervals = append(o.throughput.intervals, interval)
	// Keep last 60 intervals
	if len(o.throughput.intervals) > 60 {
		o.throughput.intervals = o.throughput.intervals[len(o.throughput.intervals)-60:]
	}
}

// --- JSON response types ---

type overviewResponse struct {
	Uptime        string  `json:"uptime"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	TotalFeatures int64   `json:"total_features"`
	TotalEntities int     `json:"total_entities"`
	OpsPerSec     float64 `json:"ops_per_sec"`
	HotTierSize   int64   `json:"hot_tier_size_bytes"`
	WarmTierSize  int64   `json:"warm_tier_size_bytes"`
	Goroutines    int     `json:"goroutines"`
}

type componentHealth struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Latency int64  `json:"latency_us"`
	Message string `json:"message,omitempty"`
}

type healthResponse struct {
	Status     string            `json:"status"`
	Components []componentHealth `json:"components"`
}

type featureStatsResponse struct {
	TotalFeatures int64            `json:"total_features"`
	ByType        map[string]int64 `json:"by_type"`
	ByGroup       map[string]int64 `json:"by_group"`
	StaleCount    int64            `json:"stale_count"`
}

type topFeatureEntry struct {
	Name        string `json:"name"`
	AccessCount int64  `json:"access_count"`
}

type latencyResponse struct {
	Reads  latencyPercentiles `json:"reads"`
	Writes latencyPercentiles `json:"writes"`
}

type latencyPercentiles struct {
	P50   int64   `json:"p50_us"`
	P90   int64   `json:"p90_us"`
	P99   int64   `json:"p99_us"`
	P999  int64   `json:"p999_us"`
	Count int64   `json:"count"`
	Avg   float64 `json:"avg_us"`
}

type storageResponse struct {
	HotTier  storageTierInfo `json:"hot_tier"`
	WarmTier storageTierInfo `json:"warm_tier"`
}

type storageTierInfo struct {
	SizeBytes    int64   `json:"size_bytes"`
	EntityCount  int     `json:"entity_count"`
	HitRate      float64 `json:"hit_rate"`
	EvictionRate int64   `json:"evictions"`
}

type driftSummaryResponse struct {
	MonitoredFeatures int `json:"monitored_features"`
	DriftDetected     int `json:"drift_detected"`
	Healthy           int `json:"healthy"`
}

type freshnessSummaryResponse struct {
	TotalMonitored int     `json:"total_monitored"`
	Fresh          int     `json:"fresh"`
	Stale          int     `json:"stale"`
	SLACompliance  float64 `json:"sla_compliance"`
}

// --- Handlers ---

func (h *DashboardHandler) handleOverview(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.overview.startTime)

	var totalFeatures int64
	var totalEntities int
	var hotSize int64

	if h.store != nil {
		totalEntities = h.store.Hot().EntityCount()
		hotSize = h.store.Hot().Size()
	}

	h.overview.featureStats.mu.RLock()
	totalFeatures = h.overview.featureStats.TotalFeatures
	h.overview.featureStats.mu.RUnlock()

	readCount, _ := h.overview.latencyTracker.reads.Stats()
	writeCount, _ := h.overview.latencyTracker.writes.Stats()
	totalOps := readCount + writeCount
	opsPerSec := float64(0)
	if uptime.Seconds() > 0 {
		opsPerSec = float64(totalOps) / uptime.Seconds()
	}

	resp := overviewResponse{
		Uptime:        uptime.Round(time.Second).String(),
		UptimeSeconds: uptime.Seconds(),
		TotalFeatures: totalFeatures,
		TotalEntities: totalEntities,
		OpsPerSec:     math.Round(opsPerSec*100) / 100,
		HotTierSize:   hotSize,
		Goroutines:    runtime.NumGoroutine(),
	}

	h.writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *DashboardHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	components := make([]componentHealth, 0, 3)

	// Check hot tier
	hotStatus := "healthy"
	hotLatency := int64(0)
	if h.store != nil {
		start := time.Now()
		_, _ = h.store.Get(r.Context(), "__health_check__", []string{"__ping__"})
		hotLatency = time.Since(start).Microseconds()
	} else {
		hotStatus = "unavailable"
	}
	components = append(components, componentHealth{
		Name:    "hot_tier",
		Status:  hotStatus,
		Latency: hotLatency,
	})

	// Check warm tier
	warmStatus := "healthy"
	warmLatency := int64(0)
	if h.store != nil {
		latency, err := h.store.CheckWarmHealth()
		warmLatency = latency.Microseconds()
		if err != nil {
			warmStatus = "unavailable"
		}
	} else {
		warmStatus = "unavailable"
	}
	components = append(components, componentHealth{
		Name:    "warm_tier",
		Status:  warmStatus,
		Latency: warmLatency,
	})

	// Metrics component
	metricsStatus := "healthy"
	if h.metrics == nil {
		metricsStatus = "unavailable"
	}
	components = append(components, componentHealth{
		Name:   "metrics",
		Status: metricsStatus,
	})

	overallStatus := "healthy"
	for _, c := range components {
		if c.Status != "healthy" {
			overallStatus = "degraded"
			break
		}
	}

	resp := healthResponse{
		Status:     overallStatus,
		Components: components,
	}

	h.writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *DashboardHandler) handleFeatureStats(w http.ResponseWriter, r *http.Request) {
	h.overview.featureStats.mu.RLock()
	resp := featureStatsResponse{
		TotalFeatures: h.overview.featureStats.TotalFeatures,
		ByType:        h.overview.featureStats.ByType,
		ByGroup:       h.overview.featureStats.ByGroup,
		StaleCount:    h.overview.featureStats.StaleCount,
	}
	h.overview.featureStats.mu.RUnlock()

	// Ensure maps are non-nil for JSON serialization
	if resp.ByType == nil {
		resp.ByType = make(map[string]int64)
	}
	if resp.ByGroup == nil {
		resp.ByGroup = make(map[string]int64)
	}

	h.writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *DashboardHandler) handleTopFeatures(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	// Return tracked top features from overview
	h.overview.featureStats.mu.RLock()
	type kv struct {
		name  string
		count int64
	}
	entries := make([]kv, 0, len(h.overview.featureStats.ByGroup))
	for name, count := range h.overview.featureStats.ByGroup {
		entries = append(entries, kv{name, count})
	}
	h.overview.featureStats.mu.RUnlock()

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})

	if len(entries) > limit {
		entries = entries[:limit]
	}

	result := make([]topFeatureEntry, len(entries))
	for i, e := range entries {
		result[i] = topFeatureEntry{Name: e.name, AccessCount: e.count}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"top_features": result,
		"limit":        limit,
	})
}

func (h *DashboardHandler) handleLatency(w http.ResponseWriter, r *http.Request) {
	readCount, readSum := h.overview.latencyTracker.reads.Stats()
	writeCount, writeSum := h.overview.latencyTracker.writes.Stats()

	readAvg := float64(0)
	if readCount > 0 {
		readAvg = float64(readSum) / float64(readCount)
	}
	writeAvg := float64(0)
	if writeCount > 0 {
		writeAvg = float64(writeSum) / float64(writeCount)
	}

	resp := latencyResponse{
		Reads: latencyPercentiles{
			P50:   h.overview.latencyTracker.reads.Percentile(50),
			P90:   h.overview.latencyTracker.reads.Percentile(90),
			P99:   h.overview.latencyTracker.reads.Percentile(99),
			P999:  h.overview.latencyTracker.reads.Percentile(99.9),
			Count: readCount,
			Avg:   math.Round(readAvg*100) / 100,
		},
		Writes: latencyPercentiles{
			P50:   h.overview.latencyTracker.writes.Percentile(50),
			P90:   h.overview.latencyTracker.writes.Percentile(90),
			P99:   h.overview.latencyTracker.writes.Percentile(99),
			P999:  h.overview.latencyTracker.writes.Percentile(99.9),
			Count: writeCount,
			Avg:   math.Round(writeAvg*100) / 100,
		},
	}

	h.writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *DashboardHandler) handleThroughput(w http.ResponseWriter, r *http.Request) {
	h.overview.throughput.mu.RLock()
	intervals := make([]ThroughputInterval, len(h.overview.throughput.intervals))
	copy(intervals, h.overview.throughput.intervals)
	currentReads := atomic.LoadInt64(&h.overview.throughput.reads)
	currentWrites := atomic.LoadInt64(&h.overview.throughput.writes)
	h.overview.throughput.mu.RUnlock()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"intervals":      intervals,
		"current_reads":  currentReads,
		"current_writes": currentWrites,
	})
}

func (h *DashboardHandler) handleStorage(w http.ResponseWriter, r *http.Request) {
	var hotSize int64
	var hotEntities int
	var hotHitRate float64
	var evictions int64

	if h.store != nil {
		hotSize = h.store.Hot().Size()
		hotEntities = h.store.Hot().EntityCount()
		hotMetrics := h.store.Hot().Metrics()
		evictions = hotMetrics.Evictions
		totalReads := hotMetrics.Hits + hotMetrics.Misses
		if totalReads > 0 {
			hotHitRate = float64(hotMetrics.Hits) / float64(totalReads)
		}
	}

	resp := storageResponse{
		HotTier: storageTierInfo{
			SizeBytes:    hotSize,
			EntityCount:  hotEntities,
			HitRate:      math.Round(hotHitRate*10000) / 10000,
			EvictionRate: evictions,
		},
		WarmTier: storageTierInfo{},
	}

	h.writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *DashboardHandler) handleDriftSummary(w http.ResponseWriter, r *http.Request) {
	resp := driftSummaryResponse{
		MonitoredFeatures: 0,
		DriftDetected:     0,
		Healthy:           0,
	}

	h.writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *DashboardHandler) handleFreshnessSummary(w http.ResponseWriter, r *http.Request) {
	h.overview.featureStats.mu.RLock()
	total := int(h.overview.featureStats.TotalFeatures)
	stale := int(h.overview.featureStats.StaleCount)
	h.overview.featureStats.mu.RUnlock()

	fresh := total - stale
	if fresh < 0 {
		fresh = 0
	}

	compliance := float64(1)
	if total > 0 {
		compliance = float64(fresh) / float64(total)
	}

	resp := freshnessSummaryResponse{
		TotalMonitored: total,
		Fresh:          fresh,
		Stale:          stale,
		SLACompliance:  math.Round(compliance*10000) / 10000,
	}

	h.writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *DashboardHandler) handleRecentAlerts(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	h.overview.mu.RLock()
	alerts := make([]*DashboardAlert, len(h.overview.alerts))
	copy(alerts, h.overview.alerts)
	h.overview.mu.RUnlock()

	// Return most recent first
	for i, j := 0, len(alerts)-1; i < j; i, j = i+1, j-1 {
		alerts[i], alerts[j] = alerts[j], alerts[i]
	}

	if len(alerts) > limit {
		alerts = alerts[:limit]
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

func (h *DashboardHandler) handleTimeline(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	h.overview.mu.RLock()
	events := make([]*TimelineEvent, len(h.overview.timeline))
	copy(events, h.overview.timeline)
	h.overview.mu.RUnlock()

	// Return most recent first
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}

	if len(events) > limit {
		events = events[:limit]
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"events": events,
		"total":  len(events),
	})
}

func (h *DashboardHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}
