package dashboard

import (
	"sync"
	"sync/atomic"
	"time"
)

// AnalyticsCollector collects usage analytics.
type AnalyticsCollector struct {
	mu             sync.RWMutex
	featureAccess  map[string]*FeatureAccessStats
	entityAccess   map[string]*EntityAccessStats
	hourlyRequests [24]int64
	currentHour    int
	currentDay     int
	totalRequests  int64
	totalLatencyUs int64
	cacheHits      int64
	cacheMisses    int64
}

// FeatureAccessStats tracks access to a feature.
type FeatureAccessStats struct {
	Name          string    `json:"name"`
	TotalRequests int64     `json:"total_requests"`
	LastAccessed  time.Time `json:"last_accessed"`
	AvgLatencyUs  int64     `json:"avg_latency_us"`
}

// EntityAccessStats tracks access by entity.
type EntityAccessStats struct {
	EntityType    string    `json:"entity_type"`
	TotalRequests int64     `json:"total_requests"`
	UniqueKeys    int64     `json:"unique_keys"`
	LastAccessed  time.Time `json:"last_accessed"`
}

// NewAnalyticsCollector creates a new analytics collector.
func NewAnalyticsCollector() *AnalyticsCollector {
	ac := &AnalyticsCollector{
		featureAccess: make(map[string]*FeatureAccessStats),
		entityAccess:  make(map[string]*EntityAccessStats),
		currentHour:   time.Now().Hour(),
		currentDay:    time.Now().Day(),
	}

	go ac.rotationLoop()

	return ac
}

// RecordFeatureAccess records a feature access.
func (c *AnalyticsCollector) RecordFeatureAccess(featureName string, latencyUs int64) {
	atomic.AddInt64(&c.totalRequests, 1)
	atomic.AddInt64(&c.totalLatencyUs, latencyUs)

	c.mu.Lock()
	defer c.mu.Unlock()

	stats, ok := c.featureAccess[featureName]
	if !ok {
		stats = &FeatureAccessStats{Name: featureName}
		c.featureAccess[featureName] = stats
	}

	stats.TotalRequests++
	stats.LastAccessed = time.Now()
	stats.AvgLatencyUs = (stats.AvgLatencyUs*stats.TotalRequests + latencyUs) / (stats.TotalRequests)

	// Update hourly counter
	hour := time.Now().Hour()
	if hour != c.currentHour {
		c.currentHour = hour
	}
	c.hourlyRequests[hour]++
}

// RecordEntityAccess records an entity access.
func (c *AnalyticsCollector) RecordEntityAccess(entityType string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stats, ok := c.entityAccess[entityType]
	if !ok {
		stats = &EntityAccessStats{EntityType: entityType}
		c.entityAccess[entityType] = stats
	}

	stats.TotalRequests++
	stats.LastAccessed = time.Now()
}

// RecordCacheHit records a cache hit.
func (c *AnalyticsCollector) RecordCacheHit() {
	atomic.AddInt64(&c.cacheHits, 1)
}

// RecordCacheMiss records a cache miss.
func (c *AnalyticsCollector) RecordCacheMiss() {
	atomic.AddInt64(&c.cacheMisses, 1)
}

// GetAnalytics returns analytics data.
func (c *AnalyticsCollector) GetAnalytics() *Analytics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalReq := atomic.LoadInt64(&c.totalRequests)
	totalLat := atomic.LoadInt64(&c.totalLatencyUs)
	hits := atomic.LoadInt64(&c.cacheHits)
	misses := atomic.LoadInt64(&c.cacheMisses)

	avgLatency := float64(0)
	if totalReq > 0 {
		avgLatency = float64(totalLat) / float64(totalReq)
	}

	cacheHitRate := float64(0)
	if hits+misses > 0 {
		cacheHitRate = float64(hits) / float64(hits+misses)
	}

	// Get top features
	topFeatures := c.getTopFeatures(10)

	// Get hourly breakdown
	hourly := make([]HourlyStats, 24)
	for i := 0; i < 24; i++ {
		hourly[i] = HourlyStats{
			Hour:     i,
			Requests: c.hourlyRequests[i],
		}
	}

	return &Analytics{
		TotalRequests:     totalReq,
		AvgLatencyUs:      avgLatency,
		CacheHitRate:      cacheHitRate,
		UniqueFeatures:    int64(len(c.featureAccess)),
		UniqueEntityTypes: int64(len(c.entityAccess)),
		TopFeatures:       topFeatures,
		HourlyBreakdown:   hourly,
		Period:            "24h",
		GeneratedAt:       time.Now(),
	}
}

func (c *AnalyticsCollector) getTopFeatures(n int) []TopFeature {
	type kv struct {
		name     string
		requests int64
	}

	sorted := make([]kv, 0, len(c.featureAccess))
	for name, stats := range c.featureAccess {
		sorted = append(sorted, kv{name, stats.TotalRequests})
	}

	// Sort by requests descending
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].requests > sorted[i].requests {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	if len(sorted) > n {
		sorted = sorted[:n]
	}

	result := make([]TopFeature, len(sorted))
	for i, kv := range sorted {
		result[i] = TopFeature{
			Name:     kv.name,
			Requests: kv.requests,
		}
	}

	return result
}

func (c *AnalyticsCollector) rotationLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		c.mu.Lock()
		// Reset hourly counter at hour boundary
		if now.Hour() != c.currentHour {
			c.hourlyRequests[c.currentHour] = 0
			c.currentHour = now.Hour()
		}
		c.mu.Unlock()
	}
}

// Analytics contains aggregated analytics data.
type Analytics struct {
	TotalRequests     int64         `json:"total_requests"`
	AvgLatencyUs      float64       `json:"avg_latency_us"`
	CacheHitRate      float64       `json:"cache_hit_rate"`
	UniqueFeatures    int64         `json:"unique_features"`
	UniqueEntityTypes int64         `json:"unique_entity_types"`
	TopFeatures       []TopFeature  `json:"top_features"`
	HourlyBreakdown   []HourlyStats `json:"hourly_breakdown"`
	Period            string        `json:"period"`
	GeneratedAt       time.Time     `json:"generated_at"`
}

// TopFeature represents a frequently accessed feature.
type TopFeature struct {
	Name     string `json:"name"`
	Requests int64  `json:"requests"`
}

// HourlyStats represents hourly statistics.
type HourlyStats struct {
	Hour     int   `json:"hour"`
	Requests int64 `json:"requests"`
}
