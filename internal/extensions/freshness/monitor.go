// Package freshness provides adaptive feature freshness management
// with ML-driven TTL adjustment based on access patterns, volatility, and drift signals.
package freshness

import (
	"sync"
	"time"
)

// AccessMetrics tracks access patterns for a feature.
type AccessMetrics struct {
	FeatureName    string        `json:"feature_name"`
	TotalAccesses  int64         `json:"total_accesses"`
	RecentAccesses int64         `json:"recent_accesses"` // Within last window
	AccessRate     float64       `json:"access_rate"`     // Accesses per second
	LastAccess     time.Time     `json:"last_access"`
	P50Latency     time.Duration `json:"p50_latency"`
	P95Latency     time.Duration `json:"p95_latency"`
	P99Latency     time.Duration `json:"p99_latency"`
	CacheHitRate   float64       `json:"cache_hit_rate"`
	StaleServes    int64         `json:"stale_serves"`
}

// ChangeMetrics tracks change patterns for a feature.
type ChangeMetrics struct {
	FeatureName        string    `json:"feature_name"`
	TotalUpdates       int64     `json:"total_updates"`
	RecentUpdates      int64     `json:"recent_updates"`
	UpdateRate         float64   `json:"update_rate"` // Updates per second
	LastUpdate         time.Time `json:"last_update"`
	AvgChangeMagnitude float64   `json:"avg_change_magnitude"`
	Volatility         float64   `json:"volatility"` // Standard deviation of changes
	DriftScore         float64   `json:"drift_score"`
}

// Monitor tracks feature access and change patterns for freshness analysis.
type Monitor struct {
	accessMetrics map[string]*accessState
	changeMetrics map[string]*changeState
	windowSize    time.Duration
	mu            sync.RWMutex
}

type accessState struct {
	metrics     AccessMetrics
	recentTimes []time.Time
	latencies   []time.Duration
	hits        int64
	misses      int64
}

type changeState struct {
	metrics      ChangeMetrics
	recentTimes  []time.Time
	changeValues []float64
}

// MonitorConfig configures the freshness monitor.
type MonitorConfig struct {
	WindowSize        time.Duration // Time window for recent metrics
	MaxHistoryEntries int           // Maximum entries to keep for calculations
	CleanupInterval   time.Duration // How often to clean old entries
}

// DefaultMonitorConfig returns sensible defaults.
func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		WindowSize:        5 * time.Minute,
		MaxHistoryEntries: 1000,
		CleanupInterval:   1 * time.Minute,
	}
}

// NewMonitor creates a new freshness monitor.
func NewMonitor(config MonitorConfig) *Monitor {
	m := &Monitor{
		accessMetrics: make(map[string]*accessState),
		changeMetrics: make(map[string]*changeState),
		windowSize:    config.WindowSize,
	}

	// Start cleanup goroutine
	go m.cleanupLoop(config.CleanupInterval, config.MaxHistoryEntries)

	return m
}

// RecordAccess records a feature access event.
func (m *Monitor) RecordAccess(featureName string, latency time.Duration, cacheHit bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.accessMetrics[featureName]
	if !exists {
		state = &accessState{
			metrics: AccessMetrics{
				FeatureName: featureName,
			},
			recentTimes: make([]time.Time, 0, 1000),
			latencies:   make([]time.Duration, 0, 1000),
		}
		m.accessMetrics[featureName] = state
	}

	now := time.Now()
	state.metrics.TotalAccesses++
	state.metrics.LastAccess = now
	state.recentTimes = append(state.recentTimes, now)
	state.latencies = append(state.latencies, latency)

	if cacheHit {
		state.hits++
	} else {
		state.misses++
	}
}

// RecordStaleServe records when a stale value was served.
func (m *Monitor) RecordStaleServe(featureName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.accessMetrics[featureName]
	if exists {
		state.metrics.StaleServes++
	}
}

// RecordChange records a feature value change.
func (m *Monitor) RecordChange(featureName string, oldValue, newValue float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.changeMetrics[featureName]
	if !exists {
		state = &changeState{
			metrics: ChangeMetrics{
				FeatureName: featureName,
			},
			recentTimes:  make([]time.Time, 0, 1000),
			changeValues: make([]float64, 0, 1000),
		}
		m.changeMetrics[featureName] = state
	}

	now := time.Now()
	changeMagnitude := abs(newValue - oldValue)

	state.metrics.TotalUpdates++
	state.metrics.LastUpdate = now
	state.recentTimes = append(state.recentTimes, now)
	state.changeValues = append(state.changeValues, changeMagnitude)
}

// RecordDriftScore records a drift score for a feature.
func (m *Monitor) RecordDriftScore(featureName string, score float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.changeMetrics[featureName]
	if !exists {
		state = &changeState{
			metrics: ChangeMetrics{
				FeatureName: featureName,
			},
			recentTimes:  make([]time.Time, 0, 1000),
			changeValues: make([]float64, 0, 1000),
		}
		m.changeMetrics[featureName] = state
	}

	state.metrics.DriftScore = score
}

// GetAccessMetrics returns computed access metrics for a feature.
func (m *Monitor) GetAccessMetrics(featureName string) (*AccessMetrics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.accessMetrics[featureName]
	if !exists {
		return nil, false
	}

	metrics := m.computeAccessMetrics(state)
	return &metrics, true
}

// GetChangeMetrics returns computed change metrics for a feature.
func (m *Monitor) GetChangeMetrics(featureName string) (*ChangeMetrics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.changeMetrics[featureName]
	if !exists {
		return nil, false
	}

	metrics := m.computeChangeMetrics(state)
	return &metrics, true
}

// GetAllAccessMetrics returns access metrics for all features.
func (m *Monitor) GetAllAccessMetrics() []AccessMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]AccessMetrics, 0, len(m.accessMetrics))
	for _, state := range m.accessMetrics {
		result = append(result, m.computeAccessMetrics(state))
	}
	return result
}

// GetAllChangeMetrics returns change metrics for all features.
func (m *Monitor) GetAllChangeMetrics() []ChangeMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ChangeMetrics, 0, len(m.changeMetrics))
	for _, state := range m.changeMetrics {
		result = append(result, m.computeChangeMetrics(state))
	}
	return result
}

func (m *Monitor) computeAccessMetrics(state *accessState) AccessMetrics {
	metrics := state.metrics
	cutoff := time.Now().Add(-m.windowSize)

	// Count recent accesses
	recentCount := int64(0)
	for _, t := range state.recentTimes {
		if t.After(cutoff) {
			recentCount++
		}
	}
	metrics.RecentAccesses = recentCount

	// Calculate access rate
	if m.windowSize.Seconds() > 0 {
		metrics.AccessRate = float64(recentCount) / m.windowSize.Seconds()
	}

	// Calculate latency percentiles
	if len(state.latencies) > 0 {
		sorted := make([]time.Duration, len(state.latencies))
		copy(sorted, state.latencies)
		sortDurations(sorted)

		metrics.P50Latency = percentile(sorted, 50)
		metrics.P95Latency = percentile(sorted, 95)
		metrics.P99Latency = percentile(sorted, 99)
	}

	// Calculate cache hit rate
	total := state.hits + state.misses
	if total > 0 {
		metrics.CacheHitRate = float64(state.hits) / float64(total)
	}

	return metrics
}

func (m *Monitor) computeChangeMetrics(state *changeState) ChangeMetrics {
	metrics := state.metrics
	cutoff := time.Now().Add(-m.windowSize)

	// Count recent updates
	recentCount := int64(0)
	for _, t := range state.recentTimes {
		if t.After(cutoff) {
			recentCount++
		}
	}
	metrics.RecentUpdates = recentCount

	// Calculate update rate
	if m.windowSize.Seconds() > 0 {
		metrics.UpdateRate = float64(recentCount) / m.windowSize.Seconds()
	}

	// Calculate average change magnitude and volatility
	if len(state.changeValues) > 0 {
		sum := 0.0
		for _, v := range state.changeValues {
			sum += v
		}
		metrics.AvgChangeMagnitude = sum / float64(len(state.changeValues))

		// Calculate volatility (standard deviation)
		if len(state.changeValues) > 1 {
			variance := 0.0
			for _, v := range state.changeValues {
				diff := v - metrics.AvgChangeMagnitude
				variance += diff * diff
			}
			variance /= float64(len(state.changeValues) - 1)
			metrics.Volatility = sqrt(variance)
		}
	}

	return metrics
}

func (m *Monitor) cleanupLoop(interval time.Duration, maxEntries int) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanup(maxEntries)
	}
}

func (m *Monitor) cleanup(maxEntries int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-m.windowSize)

	// Cleanup access metrics
	for _, state := range m.accessMetrics {
		// Trim old times
		newTimes := make([]time.Time, 0)
		for _, t := range state.recentTimes {
			if t.After(cutoff) {
				newTimes = append(newTimes, t)
			}
		}
		if len(newTimes) > maxEntries {
			newTimes = newTimes[len(newTimes)-maxEntries:]
		}
		state.recentTimes = newTimes

		// Trim latencies
		if len(state.latencies) > maxEntries {
			state.latencies = state.latencies[len(state.latencies)-maxEntries:]
		}
	}

	// Cleanup change metrics
	for _, state := range m.changeMetrics {
		newTimes := make([]time.Time, 0)
		newValues := make([]float64, 0)
		for i, t := range state.recentTimes {
			if t.After(cutoff) {
				newTimes = append(newTimes, t)
				if i < len(state.changeValues) {
					newValues = append(newValues, state.changeValues[i])
				}
			}
		}
		if len(newTimes) > maxEntries {
			newTimes = newTimes[len(newTimes)-maxEntries:]
			newValues = newValues[len(newValues)-maxEntries:]
		}
		state.recentTimes = newTimes
		state.changeValues = newValues
	}
}

// MonitorStats returns statistics about the monitor.
type MonitorStats struct {
	TrackedFeatures int   `json:"tracked_features"`
	TotalAccesses   int64 `json:"total_accesses"`
	TotalUpdates    int64 `json:"total_updates"`
}

// Stats returns monitor statistics.
func (m *Monitor) Stats() MonitorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := MonitorStats{
		TrackedFeatures: len(m.accessMetrics),
	}

	for _, state := range m.accessMetrics {
		stats.TotalAccesses += state.metrics.TotalAccesses
	}

	for _, state := range m.changeMetrics {
		stats.TotalUpdates += state.metrics.TotalUpdates
	}

	return stats
}

// Helper functions

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Newton's method for square root
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func sortDurations(d []time.Duration) {
	// Simple insertion sort for small arrays
	for i := 1; i < len(d); i++ {
		key := d[i]
		j := i - 1
		for j >= 0 && d[j] > key {
			d[j+1] = d[j]
			j--
		}
		d[j+1] = key
	}
}

func percentile(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := (len(sorted) * p) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
