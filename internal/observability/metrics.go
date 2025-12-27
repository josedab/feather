package observability

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"
)

// FeatureMetrics tracks usage and performance metrics for features.
type FeatureMetrics struct {
	Name           string    `json:"name"`
	ReadCount      int64     `json:"read_count"`
	WriteCount     int64     `json:"write_count"`
	CacheHitCount  int64     `json:"cache_hit_count"`
	CacheMissCount int64     `json:"cache_miss_count"`
	ErrorCount     int64     `json:"error_count"`
	LastRead       time.Time `json:"last_read"`
	LastWrite      time.Time `json:"last_write"`
	AvgLatencyUs   float64   `json:"avg_latency_us"`
	P50LatencyUs   float64   `json:"p50_latency_us"`
	P95LatencyUs   float64   `json:"p95_latency_us"`
	P99LatencyUs   float64   `json:"p99_latency_us"`
	TotalBytes     int64     `json:"total_bytes"`
}

// MetricsCollector collects feature metrics.
type MetricsCollector struct {
	metrics   map[string]*featureMetricsInternal
	latencies map[string]*latencyBuffer
	mu        sync.RWMutex
}

type featureMetricsInternal struct {
	readCount      int64
	writeCount     int64
	cacheHitCount  int64
	cacheMissCount int64
	errorCount     int64
	lastRead       time.Time
	lastWrite      time.Time
	totalLatencyNs int64
	latencyCount   int64
	totalBytes     int64
}

type latencyBuffer struct {
	values []float64
	mu     sync.Mutex
	maxLen int
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics:   make(map[string]*featureMetricsInternal),
		latencies: make(map[string]*latencyBuffer),
	}
}

// RecordRead records a feature read operation.
func (c *MetricsCollector) RecordRead(feature string, latency time.Duration, cacheHit bool, bytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	m := c.getOrCreateMetrics(feature)
	m.readCount++
	m.lastRead = time.Now()
	m.totalLatencyNs += latency.Nanoseconds()
	m.latencyCount++
	m.totalBytes += bytes

	if cacheHit {
		m.cacheHitCount++
	} else {
		m.cacheMissCount++
	}

	c.recordLatency(feature, float64(latency.Microseconds()))
}

// RecordWrite records a feature write operation.
func (c *MetricsCollector) RecordWrite(feature string, latency time.Duration, bytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	m := c.getOrCreateMetrics(feature)
	m.writeCount++
	m.lastWrite = time.Now()
	m.totalLatencyNs += latency.Nanoseconds()
	m.latencyCount++
	m.totalBytes += bytes

	c.recordLatency(feature, float64(latency.Microseconds()))
}

// RecordError records an error for a feature.
func (c *MetricsCollector) RecordError(feature string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	m := c.getOrCreateMetrics(feature)
	m.errorCount++
}

// GetMetrics returns metrics for a specific feature.
func (c *MetricsCollector) GetMetrics(feature string) *FeatureMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	m, ok := c.metrics[feature]
	if !ok {
		return nil
	}

	return c.buildFeatureMetrics(feature, m)
}

// GetAllMetrics returns metrics for all features.
func (c *MetricsCollector) GetAllMetrics() []*FeatureMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*FeatureMetrics, 0, len(c.metrics))
	for feature, m := range c.metrics {
		result = append(result, c.buildFeatureMetrics(feature, m))
	}

	return result
}

// GetTopFeatures returns the top N features by read count.
func (c *MetricsCollector) GetTopFeatures(n int) []*FeatureMetrics {
	metrics := c.GetAllMetrics()

	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].ReadCount > metrics[j].ReadCount
	})

	if len(metrics) > n {
		metrics = metrics[:n]
	}

	return metrics
}

// Reset resets all metrics.
func (c *MetricsCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics = make(map[string]*featureMetricsInternal)
	c.latencies = make(map[string]*latencyBuffer)
}

func (c *MetricsCollector) getOrCreateMetrics(feature string) *featureMetricsInternal {
	m, ok := c.metrics[feature]
	if !ok {
		m = &featureMetricsInternal{}
		c.metrics[feature] = m
	}
	return m
}

func (c *MetricsCollector) recordLatency(feature string, latencyUs float64) {
	buf, ok := c.latencies[feature]
	if !ok {
		buf = &latencyBuffer{
			values: make([]float64, 0, 1000),
			maxLen: 1000,
		}
		c.latencies[feature] = buf
	}

	buf.mu.Lock()
	defer buf.mu.Unlock()

	if len(buf.values) >= buf.maxLen {
		buf.values = buf.values[1:]
	}
	buf.values = append(buf.values, latencyUs)
}

func (c *MetricsCollector) buildFeatureMetrics(feature string, m *featureMetricsInternal) *FeatureMetrics {
	fm := &FeatureMetrics{
		Name:           feature,
		ReadCount:      m.readCount,
		WriteCount:     m.writeCount,
		CacheHitCount:  m.cacheHitCount,
		CacheMissCount: m.cacheMissCount,
		ErrorCount:     m.errorCount,
		LastRead:       m.lastRead,
		LastWrite:      m.lastWrite,
		TotalBytes:     m.totalBytes,
	}

	if m.latencyCount > 0 {
		fm.AvgLatencyUs = float64(m.totalLatencyNs) / float64(m.latencyCount) / 1000.0
	}

	if buf, ok := c.latencies[feature]; ok {
		fm.P50LatencyUs, fm.P95LatencyUs, fm.P99LatencyUs = buf.percentiles()
	}

	return fm
}

func (b *latencyBuffer) percentiles() (p50, p95, p99 float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.values) == 0 {
		return 0, 0, 0
	}

	sorted := make([]float64, len(b.values))
	copy(sorted, b.values)
	sort.Float64s(sorted)

	p50 = percentile(sorted, 0.50)
	p95 = percentile(sorted, 0.95)
	p99 = percentile(sorted, 0.99)

	return
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// FreshnessChecker monitors feature freshness.
type FreshnessChecker struct {
	collector   *MetricsCollector
	thresholds  map[string]time.Duration
	defaultTTL  time.Duration
	mu          sync.RWMutex
}

// FreshnessAlert represents a freshness violation.
type FreshnessAlert struct {
	Feature     string        `json:"feature"`
	LastUpdate  time.Time     `json:"last_update"`
	Threshold   time.Duration `json:"threshold"`
	StaleDur    time.Duration `json:"stale_duration"`
	Severity    string        `json:"severity"`
}

// NewFreshnessChecker creates a new freshness checker.
func NewFreshnessChecker(collector *MetricsCollector, defaultTTL time.Duration) *FreshnessChecker {
	return &FreshnessChecker{
		collector:  collector,
		thresholds: make(map[string]time.Duration),
		defaultTTL: defaultTTL,
	}
}

// SetThreshold sets a freshness threshold for a feature.
func (f *FreshnessChecker) SetThreshold(feature string, threshold time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.thresholds[feature] = threshold
}

// Check checks all features for freshness violations.
func (f *FreshnessChecker) Check(ctx context.Context) []*FreshnessAlert {
	f.mu.RLock()
	defer f.mu.RUnlock()

	metrics := f.collector.GetAllMetrics()
	now := time.Now()
	var alerts []*FreshnessAlert

	for _, m := range metrics {
		threshold, ok := f.thresholds[m.Name]
		if !ok {
			threshold = f.defaultTTL
		}

		if m.LastWrite.IsZero() {
			continue
		}

		staleDur := now.Sub(m.LastWrite)
		if staleDur > threshold {
			severity := "warning"
			if staleDur > threshold*2 {
				severity = "critical"
			}

			alerts = append(alerts, &FreshnessAlert{
				Feature:    m.Name,
				LastUpdate: m.LastWrite,
				Threshold:  threshold,
				StaleDur:   staleDur,
				Severity:   severity,
			})
		}
	}

	return alerts
}

// UsageTracker tracks feature usage patterns.
type UsageTracker struct {
	collector *MetricsCollector
	history   map[string]*usageHistory
	mu        sync.RWMutex
}

type usageHistory struct {
	hourlyReads  [24]int64
	hourlyWrites [24]int64
	dailyReads   [7]int64
	dailyWrites  [7]int64
	lastHour     int
	lastDay      int
}

// UsagePattern represents usage patterns for a feature.
type UsagePattern struct {
	Feature       string    `json:"feature"`
	HourlyReads   []int64   `json:"hourly_reads"`
	HourlyWrites  []int64   `json:"hourly_writes"`
	DailyReads    []int64   `json:"daily_reads"`
	DailyWrites   []int64   `json:"daily_writes"`
	PeakHour      int       `json:"peak_hour"`
	PeakDay       int       `json:"peak_day"`
	AvgDailyReads float64   `json:"avg_daily_reads"`
	TrendSlope    float64   `json:"trend_slope"`
}

// NewUsageTracker creates a new usage tracker.
func NewUsageTracker(collector *MetricsCollector) *UsageTracker {
	return &UsageTracker{
		collector: collector,
		history:   make(map[string]*usageHistory),
	}
}

// Record records current usage into history.
func (t *UsageTracker) Record(ctx context.Context) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	hour := now.Hour()
	day := int(now.Weekday())

	metrics := t.collector.GetAllMetrics()

	for _, m := range metrics {
		h, ok := t.history[m.Name]
		if !ok {
			h = &usageHistory{lastHour: -1, lastDay: -1}
			t.history[m.Name] = h
		}

		// Rotate hourly buckets
		if hour != h.lastHour {
			h.hourlyReads[hour] = m.ReadCount
			h.hourlyWrites[hour] = m.WriteCount
			h.lastHour = hour
		}

		// Rotate daily buckets
		if day != h.lastDay {
			h.dailyReads[day] = m.ReadCount
			h.dailyWrites[day] = m.WriteCount
			h.lastDay = day
		}
	}
}

// GetPattern returns usage patterns for a feature.
func (t *UsageTracker) GetPattern(feature string) *UsagePattern {
	t.mu.RLock()
	defer t.mu.RUnlock()

	h, ok := t.history[feature]
	if !ok {
		return nil
	}

	pattern := &UsagePattern{
		Feature:      feature,
		HourlyReads:  h.hourlyReads[:],
		HourlyWrites: h.hourlyWrites[:],
		DailyReads:   h.dailyReads[:],
		DailyWrites:  h.dailyWrites[:],
	}

	// Find peak hour
	var maxHourly int64
	for i, v := range h.hourlyReads {
		if v > maxHourly {
			maxHourly = v
			pattern.PeakHour = i
		}
	}

	// Find peak day
	var maxDaily int64
	for i, v := range h.dailyReads {
		if v > maxDaily {
			maxDaily = v
			pattern.PeakDay = i
		}
	}

	// Calculate average daily reads
	var totalDaily int64
	for _, v := range h.dailyReads {
		totalDaily += v
	}
	pattern.AvgDailyReads = float64(totalDaily) / 7.0

	// Simple trend calculation (linear regression slope)
	pattern.TrendSlope = calculateTrend(h.dailyReads[:])

	return pattern
}

func calculateTrend(values []int64) float64 {
	n := float64(len(values))
	if n < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i, v := range values {
		x := float64(i)
		y := float64(v)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}

	return (n*sumXY - sumX*sumY) / denominator
}
