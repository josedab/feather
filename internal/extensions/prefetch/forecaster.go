package prefetch

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// ForecasterConfig configures the time-series access forecaster.
type ForecasterConfig struct {
	SmoothingFactor   float64 // alpha for exponential smoothing (0-1)
	SeasonalPeriods   int     // periods for seasonal detection (e.g., 24 for hourly)
	MinDataPoints     int     // minimum observations before forecasting
	ClusterCount      int     // number of entity clusters
	MemoryBudgetBytes int64   // max memory for pre-warmed features
}

// DefaultForecasterConfig returns sensible defaults.
func DefaultForecasterConfig() ForecasterConfig {
	return ForecasterConfig{
		SmoothingFactor:   0.3,
		SeasonalPeriods:   24,
		MinDataPoints:     5,
		ClusterCount:      4,
		MemoryBudgetBytes: 256 * 1024 * 1024,
	}
}

// AccessForecast is a prediction for future access of a feature/entity pair.
type AccessForecast struct {
	Feature    string    `json:"feature"`
	Entity     string    `json:"entity"`
	Predicted  float64   `json:"predicted_access_rate"` // accesses per period
	Confidence float64   `json:"confidence"`
	NextAccess time.Time `json:"next_access_estimate"`
	Priority   float64   `json:"priority"` // higher = more important to warm
}

// EntityCluster groups entities with similar access patterns.
type EntityCluster struct {
	ID       int       `json:"id"`
	Centroid []float64 `json:"centroid"` // access pattern centroid
	Members  []string  `json:"members"`  // entity IDs
	Size     int       `json:"size"`
}

// ForecasterStats holds runtime statistics for the forecaster.
type ForecasterStats struct {
	TotalRecorded    int64 `json:"total_recorded"`
	TotalForecasts   int64 `json:"total_forecasts"`
	EntitiesTracked  int   `json:"entities_tracked"`
	FeaturesTracked  int   `json:"features_tracked"`
	ClustersComputed int   `json:"clusters_computed"`
}

// accessSeries stores per-feature, per-entity time-series data.
type accessSeries struct {
	timestamps []time.Time
	hourly     [24]float64 // access count per hour-of-day
	smoothed   float64     // exponentially smoothed rate
	total      int64
}

// Forecaster predicts future feature access patterns using exponential
// smoothing and time-of-day seasonality.
type Forecaster struct {
	config ForecasterConfig

	// series[entity][feature] -> accessSeries
	series map[string]map[string]*accessSeries

	totalRecorded  atomic.Int64
	totalForecasts atomic.Int64

	mu sync.RWMutex
}

// NewForecaster creates a new access pattern forecaster.
func NewForecaster(cfg ForecasterConfig) *Forecaster {
	if cfg.SmoothingFactor <= 0 || cfg.SmoothingFactor > 1 {
		cfg.SmoothingFactor = 0.3
	}
	if cfg.SeasonalPeriods <= 0 {
		cfg.SeasonalPeriods = 24
	}
	if cfg.MinDataPoints <= 0 {
		cfg.MinDataPoints = 5
	}
	if cfg.ClusterCount <= 0 {
		cfg.ClusterCount = 4
	}
	if cfg.MemoryBudgetBytes <= 0 {
		cfg.MemoryBudgetBytes = 256 * 1024 * 1024
	}
	return &Forecaster{
		config: cfg,
		series: make(map[string]map[string]*accessSeries),
	}
}

// RecordAccess records an access event for forecasting.
func (f *Forecaster) RecordAccess(entity, feature string, timestamp time.Time) {
	f.totalRecorded.Add(1)

	f.mu.Lock()
	defer f.mu.Unlock()

	entitySeries, ok := f.series[entity]
	if !ok {
		entitySeries = make(map[string]*accessSeries)
		f.series[entity] = entitySeries
	}

	s, ok := entitySeries[feature]
	if !ok {
		s = &accessSeries{}
		entitySeries[feature] = s
	}

	s.timestamps = append(s.timestamps, timestamp)
	s.hourly[timestamp.Hour()]++
	s.total++

	// Update exponential smoothing: estimate inter-arrival rate.
	if len(s.timestamps) >= 2 {
		prev := s.timestamps[len(s.timestamps)-2]
		interval := timestamp.Sub(prev).Seconds()
		if interval > 0 {
			instantRate := 1.0 / interval
			alpha := f.config.SmoothingFactor
			s.smoothed = alpha*instantRate + (1-alpha)*s.smoothed
		}
	} else {
		s.smoothed = 1.0 // initial estimate
	}

	// Bound stored timestamps to avoid unbounded growth.
	maxPoints := f.config.MinDataPoints * 100
	if maxPoints < 500 {
		maxPoints = 500
	}
	if len(s.timestamps) > maxPoints {
		s.timestamps = s.timestamps[len(s.timestamps)-maxPoints:]
	}
}

// Forecast returns access predictions for a feature across all tracked entities
// within the given horizon.
func (f *Forecaster) Forecast(feature string, horizon time.Duration) []AccessForecast {
	f.totalForecasts.Add(1)

	f.mu.RLock()
	defer f.mu.RUnlock()

	now := time.Now()
	currentHour := now.Hour()

	forecasts := make([]AccessForecast, 0, len(f.series))

	for entity, featureMap := range f.series {
		s, ok := featureMap[feature]
		if !ok {
			continue
		}
		if int(s.total) < f.config.MinDataPoints {
			continue
		}

		// Predicted rate from exponential smoothing.
		predictedRate := s.smoothed

		// Apply seasonal (hourly) adjustment.
		seasonalFactor := f.seasonalFactor(s, currentHour)
		adjustedRate := predictedRate * seasonalFactor

		// Confidence based on data volume.
		confidence := math.Min(1.0, float64(s.total)/float64(f.config.MinDataPoints*4))

		// Estimate next access time.
		var nextAccess time.Time
		if adjustedRate > 0 {
			secsUntilNext := 1.0 / adjustedRate
			nextAccess = now.Add(time.Duration(secsUntilNext * float64(time.Second)))
			// Clamp to horizon.
			if nextAccess.After(now.Add(horizon)) {
				nextAccess = now.Add(horizon)
			}
		} else {
			nextAccess = now.Add(horizon)
		}

		// Priority: blend rate and confidence.
		priority := adjustedRate * confidence

		forecasts = append(forecasts, AccessForecast{
			Feature:    feature,
			Entity:     entity,
			Predicted:  math.Round(adjustedRate*1000) / 1000,
			Confidence: math.Round(confidence*1000) / 1000,
			NextAccess: nextAccess,
			Priority:   math.Round(priority*1000) / 1000,
		})
	}

	// Sort by priority descending.
	for i := 1; i < len(forecasts); i++ {
		for j := i; j > 0 && forecasts[j].Priority > forecasts[j-1].Priority; j-- {
			forecasts[j], forecasts[j-1] = forecasts[j-1], forecasts[j]
		}
	}

	return forecasts
}

// seasonalFactor returns a multiplier based on hour-of-day distribution.
func (f *Forecaster) seasonalFactor(s *accessSeries, hour int) float64 {
	var totalAccesses float64
	for _, v := range s.hourly {
		totalAccesses += v
	}
	if totalAccesses == 0 {
		return 1.0
	}

	avgPerHour := totalAccesses / float64(f.config.SeasonalPeriods)
	if avgPerHour == 0 {
		return 1.0
	}

	return s.hourly[hour] / avgPerHour
}

// ClusterEntities groups entities by access pattern similarity using a simple
// k-means approach on hourly access vectors.
func (f *Forecaster) ClusterEntities() []EntityCluster {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Build per-entity aggregate hourly vectors.
	type entityVector struct {
		entity string
		vec    [24]float64
	}

	entities := make([]entityVector, 0, len(f.series))
	for entity, featureMap := range f.series {
		var agg [24]float64
		for _, s := range featureMap {
			for h := 0; h < 24; h++ {
				agg[h] += s.hourly[h]
			}
		}
		// Normalize the vector.
		var norm float64
		for _, v := range agg {
			norm += v * v
		}
		norm = math.Sqrt(norm)
		if norm > 0 {
			for h := range agg {
				agg[h] /= norm
			}
		}
		entities = append(entities, entityVector{entity: entity, vec: agg})
	}

	if len(entities) == 0 {
		return nil
	}

	k := f.config.ClusterCount
	if k > len(entities) {
		k = len(entities)
	}

	// Initialize centroids from first k entities.
	centroids := make([][24]float64, k)
	for i := 0; i < k; i++ {
		centroids[i] = entities[i].vec
	}

	// Assign entities to clusters.
	assignments := make([]int, len(entities))
	const maxIter = 20
	for iter := 0; iter < maxIter; iter++ {
		changed := false
		// Assign step.
		for i, ev := range entities {
			bestCluster := 0
			bestDist := math.MaxFloat64
			for c := 0; c < k; c++ {
				d := euclideanDist(ev.vec, centroids[c])
				if d < bestDist {
					bestDist = d
					bestCluster = c
				}
			}
			if assignments[i] != bestCluster {
				assignments[i] = bestCluster
				changed = true
			}
		}
		if !changed {
			break
		}

		// Update centroids.
		counts := make([]int, k)
		newCentroids := make([][24]float64, k)
		for i, ev := range entities {
			c := assignments[i]
			counts[c]++
			for h := 0; h < 24; h++ {
				newCentroids[c][h] += ev.vec[h]
			}
		}
		for c := 0; c < k; c++ {
			if counts[c] > 0 {
				for h := 0; h < 24; h++ {
					newCentroids[c][h] /= float64(counts[c])
				}
			}
		}
		centroids = newCentroids
	}

	// Build cluster results.
	clusterMembers := make([][]string, k)
	for i := 0; i < k; i++ {
		clusterMembers[i] = []string{}
	}
	for i, ev := range entities {
		clusterMembers[assignments[i]] = append(clusterMembers[assignments[i]], ev.entity)
	}

	var clusters []EntityCluster
	for i := 0; i < k; i++ {
		if len(clusterMembers[i]) == 0 {
			continue
		}
		centroid := make([]float64, 24)
		for h := 0; h < 24; h++ {
			centroid[h] = math.Round(centroids[i][h]*1000) / 1000
		}
		clusters = append(clusters, EntityCluster{
			ID:       i,
			Centroid: centroid,
			Members:  clusterMembers[i],
			Size:     len(clusterMembers[i]),
		})
	}

	return clusters
}

// GetWarmingPlan generates a warming plan within the given memory budget
// by forecasting across all tracked features and entities.
func (f *Forecaster) GetWarmingPlan(budgetBytes int64) *WarmingPlan {
	f.mu.RLock()

	// Collect all feature names.
	featureSet := make(map[string]struct{})
	for _, featureMap := range f.series {
		for feat := range featureMap {
			featureSet[feat] = struct{}{}
		}
	}
	f.mu.RUnlock()

	const defaultHorizon = 1 * time.Hour
	const estimatedSizePerFeature int64 = 256

	var allCandidates []WarmingCandidate
	for feat := range featureSet {
		forecasts := f.Forecast(feat, defaultHorizon)
		for _, fc := range forecasts {
			if fc.Priority <= 0 {
				continue
			}
			allCandidates = append(allCandidates, WarmingCandidate{
				Entity:        fc.Entity,
				Feature:       fc.Feature,
				Priority:      fc.Priority,
				EstimatedSize: estimatedSizePerFeature,
				Reason:        "forecasted access",
			})
		}
	}

	// Sort by priority descending.
	for i := 1; i < len(allCandidates); i++ {
		for j := i; j > 0 && allCandidates[j].Priority > allCandidates[j-1].Priority; j-- {
			allCandidates[j], allCandidates[j-1] = allCandidates[j-1], allCandidates[j]
		}
	}

	// Select candidates within budget.
	selected := make([]WarmingCandidate, 0, len(allCandidates))
	var totalBytes int64
	for _, c := range allCandidates {
		if totalBytes+c.EstimatedSize > budgetBytes {
			continue
		}
		totalBytes += c.EstimatedSize
		selected = append(selected, c)
	}

	return &WarmingPlan{
		Candidates:     selected,
		EstimatedBytes: totalBytes,
		BudgetBytes:    budgetBytes,
		GeneratedAt:    time.Now(),
	}
}

// Stats returns forecaster runtime statistics.
func (f *Forecaster) Stats() ForecasterStats {
	f.mu.RLock()
	defer f.mu.RUnlock()

	featureSet := make(map[string]struct{})
	for _, featureMap := range f.series {
		for feat := range featureMap {
			featureSet[feat] = struct{}{}
		}
	}

	return ForecasterStats{
		TotalRecorded:   f.totalRecorded.Load(),
		TotalForecasts:  f.totalForecasts.Load(),
		EntitiesTracked: len(f.series),
		FeaturesTracked: len(featureSet),
	}
}

// euclideanDist computes the Euclidean distance between two 24-element vectors.
func euclideanDist(a, b [24]float64) float64 {
	var sum float64
	for i := 0; i < 24; i++ {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum)
}
