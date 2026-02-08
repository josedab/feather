package autoscaler

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// MetricType identifies a custom HPA metric.
type MetricType string

const (
	MetricQPS          MetricType = "feature_qps"
	MetricP99Latency   MetricType = "p99_latency_ms"
	MetricCacheHitRate MetricType = "cache_hit_rate"
	MetricCPUUsage     MetricType = "cpu_usage_pct"
	MetricMemoryUsage  MetricType = "memory_usage_pct"
	MetricShardBalance MetricType = "shard_balance_score"
)

// ScaleDirection indicates the scaling action.
type ScaleDirection string

const (
	ScaleUp   ScaleDirection = "up"
	ScaleDown ScaleDirection = "down"
	ScaleNone ScaleDirection = "none"
)

// Config holds autoscaler tuning parameters.
type Config struct {
	MinReplicas         int
	MaxReplicas         int
	ScaleUpCooldown     time.Duration
	ScaleDownCooldown   time.Duration
	TargetQPS           float64
	TargetP99Ms         float64
	TargetCacheHit      float64
	TargetCPU           float64
	TargetMemory        float64
	ShardsPerReplica    int
	PredictiveEnabled   bool
	StabilizationWindow time.Duration
}

// DefaultConfig returns production-ready defaults.
func DefaultConfig() Config {
	return Config{
		MinReplicas:         1,
		MaxReplicas:         20,
		ScaleUpCooldown:     60 * time.Second,
		ScaleDownCooldown:   300 * time.Second,
		TargetQPS:           1000,
		TargetP99Ms:         10,
		TargetCacheHit:      0.8,
		TargetCPU:           70,
		TargetMemory:        80,
		ShardsPerReplica:    16,
		PredictiveEnabled:   true,
		StabilizationWindow: 5 * time.Minute,
	}
}

// Autoscaler computes scaling recommendations from custom metrics.
type Autoscaler struct {
	config          Config
	mu              sync.RWMutex
	currentReplicas int
	metrics         map[MetricType]*MetricWindow
	lastScaleUp     time.Time
	lastScaleDown   time.Time
	scaleHistory    []ScaleEvent
	shardMap        map[int]int // shard -> replica assignment
	policies        []ScalePolicy
}

// MetricWindow stores a sliding window of metric samples.
type MetricWindow struct {
	values     []MetricSample
	maxSamples int
}

// MetricSample is a single timestamped metric observation.
type MetricSample struct {
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// ScalePolicy defines a single metric-based scaling rule.
type ScalePolicy struct {
	Name      string     `json:"name"`
	Metric    MetricType `json:"metric"`
	Target    float64    `json:"target"`
	Tolerance float64    `json:"tolerance"` // 0-1, e.g. 0.1 = ±10%
	Weight    float64    `json:"weight"`    // relative importance
}

// ScaleRecommendation is the output of an Evaluate call.
type ScaleRecommendation struct {
	Direction       ScaleDirection     `json:"direction"`
	CurrentReplicas int                `json:"current_replicas"`
	DesiredReplicas int                `json:"desired_replicas"`
	Reason          string             `json:"reason"`
	Metrics         map[string]float64 `json:"metrics"`
	Confidence      float64            `json:"confidence"` // 0-1
	Cooldown        bool               `json:"in_cooldown"`
}

// ScaleEvent records a historical scaling action.
type ScaleEvent struct {
	Timestamp    time.Time      `json:"timestamp"`
	Direction    ScaleDirection `json:"direction"`
	FromReplicas int            `json:"from_replicas"`
	ToReplicas   int            `json:"to_replicas"`
	Reason       string         `json:"reason"`
}

// ShardAssignment maps a shard to a replica.
type ShardAssignment struct {
	ShardID   int `json:"shard_id"`
	ReplicaID int `json:"replica_id"`
}

// CustomMetric is an HPA-consumable metric snapshot.
type CustomMetric struct {
	Name      string     `json:"name"`
	Type      MetricType `json:"type"`
	Value     float64    `json:"value"`
	Timestamp time.Time  `json:"timestamp"`
}

// AutoscalerStats provides a read-only view of the autoscaler state.
type AutoscalerStats struct {
	CurrentReplicas int                `json:"current_replicas"`
	MinReplicas     int                `json:"min_replicas"`
	MaxReplicas     int                `json:"max_replicas"`
	TotalShards     int                `json:"total_shards"`
	ScaleEvents     int                `json:"scale_events"`
	LastScaleUp     *time.Time         `json:"last_scale_up,omitempty"`
	LastScaleDown   *time.Time         `json:"last_scale_down,omitempty"`
	MetricSummary   map[string]float64 `json:"metric_summary"`
	PolicyCount     int                `json:"policy_count"`
}

const defaultWindowSize = 60

// NewAutoscaler creates an autoscaler with default policies derived from cfg.
func NewAutoscaler(cfg Config) *Autoscaler {
	if cfg.MinReplicas < 1 {
		cfg.MinReplicas = 1
	}
	if cfg.MaxReplicas < cfg.MinReplicas {
		cfg.MaxReplicas = cfg.MinReplicas
	}
	if cfg.ShardsPerReplica < 1 {
		cfg.ShardsPerReplica = 16
	}

	a := &Autoscaler{
		config:          cfg,
		currentReplicas: cfg.MinReplicas,
		metrics:         make(map[MetricType]*MetricWindow),
		scaleHistory:    make([]ScaleEvent, 0),
		shardMap:        make(map[int]int),
		policies:        defaultPolicies(cfg),
	}

	// Pre-create metric windows.
	for _, p := range a.policies {
		a.metrics[p.Metric] = &MetricWindow{
			values:     make([]MetricSample, 0, defaultWindowSize),
			maxSamples: defaultWindowSize,
		}
	}

	a.rebalanceShards()
	return a
}

func defaultPolicies(cfg Config) []ScalePolicy {
	return []ScalePolicy{
		{Name: "qps", Metric: MetricQPS, Target: cfg.TargetQPS, Tolerance: 0.1, Weight: 0.3},
		{Name: "p99_latency", Metric: MetricP99Latency, Target: cfg.TargetP99Ms, Tolerance: 0.15, Weight: 0.25},
		{Name: "cache_hit", Metric: MetricCacheHitRate, Target: cfg.TargetCacheHit, Tolerance: 0.05, Weight: 0.1},
		{Name: "cpu", Metric: MetricCPUUsage, Target: cfg.TargetCPU, Tolerance: 0.1, Weight: 0.2},
		{Name: "memory", Metric: MetricMemoryUsage, Target: cfg.TargetMemory, Tolerance: 0.1, Weight: 0.15},
	}
}

// RecordMetric adds a sample to the metric window.
func (a *Autoscaler) RecordMetric(metric MetricType, value float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	w, ok := a.metrics[metric]
	if !ok {
		w = &MetricWindow{
			values:     make([]MetricSample, 0, defaultWindowSize),
			maxSamples: defaultWindowSize,
		}
		a.metrics[metric] = w
	}

	w.values = append(w.values, MetricSample{
		Value:     value,
		Timestamp: time.Now(),
	})
	if len(w.values) > w.maxSamples {
		w.values = w.values[len(w.values)-w.maxSamples:]
	}
}

// Evaluate computes a scaling recommendation from current metrics.
func (a *Autoscaler) Evaluate() *ScaleRecommendation {
	a.mu.RLock()
	defer a.mu.RUnlock()

	now := time.Now()
	rec := &ScaleRecommendation{
		CurrentReplicas: a.currentReplicas,
		DesiredReplicas: a.currentReplicas,
		Direction:       ScaleNone,
		Metrics:         make(map[string]float64),
	}

	// Check cooldowns.
	if !a.lastScaleUp.IsZero() && now.Sub(a.lastScaleUp) < a.config.ScaleUpCooldown {
		rec.Cooldown = true
		rec.Reason = "scale-up cooldown active"
		return rec
	}
	if !a.lastScaleDown.IsZero() && now.Sub(a.lastScaleDown) < a.config.ScaleDownCooldown {
		rec.Cooldown = true
		rec.Reason = "scale-down cooldown active"
		return rec
	}

	var totalWeight float64
	var weightedDesired float64

	for _, p := range a.policies {
		avg, ok := a.windowAvg(p.Metric)
		if !ok {
			continue
		}
		rec.Metrics[string(p.Metric)] = avg

		// For cache hit rate the relationship is inverted: lower current
		// means we need more replicas.
		var ratio float64
		if p.Metric == MetricCacheHitRate {
			if avg == 0 {
				ratio = 1.0
			} else {
				ratio = p.Target / avg
			}
		} else {
			if p.Target == 0 {
				continue
			}
			ratio = avg / p.Target
		}

		// Within tolerance band → no change for this policy.
		if math.Abs(ratio-1.0) <= p.Tolerance {
			ratio = 1.0
		}

		desired := float64(a.currentReplicas) * ratio
		weightedDesired += desired * p.Weight
		totalWeight += p.Weight
	}

	if totalWeight == 0 {
		rec.Reason = "no metric data available"
		return rec
	}

	avgDesired := weightedDesired / totalWeight

	// Direction-aware rounding.
	var desired int
	if avgDesired > float64(a.currentReplicas) {
		desired = int(math.Ceil(avgDesired))
	} else {
		desired = int(math.Floor(avgDesired))
	}

	// Clamp to bounds.
	desired = clamp(desired, a.config.MinReplicas, a.config.MaxReplicas)

	rec.DesiredReplicas = desired
	switch {
	case desired > a.currentReplicas:
		rec.Direction = ScaleUp
		rec.Reason = fmt.Sprintf("metrics indicate need for %d replicas (current %d)", desired, a.currentReplicas)
	case desired < a.currentReplicas:
		rec.Direction = ScaleDown
		rec.Reason = fmt.Sprintf("metrics indicate %d replicas sufficient (current %d)", desired, a.currentReplicas)
	default:
		rec.Direction = ScaleNone
		rec.Reason = "current replica count is optimal"
	}

	// Confidence based on number of metrics with data.
	metricsWithData := 0
	for _, p := range a.policies {
		if _, ok := a.windowAvg(p.Metric); ok {
			metricsWithData++
		}
	}
	if len(a.policies) > 0 {
		rec.Confidence = float64(metricsWithData) / float64(len(a.policies))
	}

	return rec
}

// Apply executes a scale recommendation, updating replicas and shards.
func (a *Autoscaler) Apply(rec *ScaleRecommendation) error {
	if rec == nil {
		return fmt.Errorf("applying scale recommendation: nil recommendation")
	}
	if rec.Direction == ScaleNone {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	desired := clamp(rec.DesiredReplicas, a.config.MinReplicas, a.config.MaxReplicas)

	event := ScaleEvent{
		Timestamp:    time.Now(),
		Direction:    rec.Direction,
		FromReplicas: a.currentReplicas,
		ToReplicas:   desired,
		Reason:       rec.Reason,
	}

	a.currentReplicas = desired
	a.scaleHistory = append(a.scaleHistory, event)

	switch rec.Direction {
	case ScaleUp:
		a.lastScaleUp = event.Timestamp
	case ScaleDown:
		a.lastScaleDown = event.Timestamp
	}

	a.rebalanceShards()
	return nil
}

// GetMetrics returns the latest value for each tracked metric type.
func (a *Autoscaler) GetMetrics() []CustomMetric {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]CustomMetric, 0, len(a.metrics))
	for mt, w := range a.metrics {
		if len(w.values) == 0 {
			continue
		}
		latest := w.values[len(w.values)-1]
		result = append(result, CustomMetric{
			Name:      string(mt),
			Type:      mt,
			Value:     latest.Value,
			Timestamp: latest.Timestamp,
		})
	}
	return result
}

// GetShardAssignments returns the current shard-to-replica mapping.
func (a *Autoscaler) GetShardAssignments() []ShardAssignment {
	a.mu.RLock()
	defer a.mu.RUnlock()

	assignments := make([]ShardAssignment, 0, len(a.shardMap))
	for shard, replica := range a.shardMap {
		assignments = append(assignments, ShardAssignment{
			ShardID:   shard,
			ReplicaID: replica,
		})
	}
	return assignments
}

// RebalanceShards distributes shards evenly and returns the new assignments.
func (a *Autoscaler) RebalanceShards() []ShardAssignment {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.rebalanceShards()

	assignments := make([]ShardAssignment, 0, len(a.shardMap))
	for shard, replica := range a.shardMap {
		assignments = append(assignments, ShardAssignment{
			ShardID:   shard,
			ReplicaID: replica,
		})
	}
	return assignments
}

// rebalanceShards performs round-robin shard assignment (caller must hold mu).
func (a *Autoscaler) rebalanceShards() {
	totalShards := a.currentReplicas * a.config.ShardsPerReplica
	a.shardMap = make(map[int]int, totalShards)
	for i := 0; i < totalShards; i++ {
		a.shardMap[i] = i % a.currentReplicas
	}
}

// GetScaleHistory returns the most recent scale events up to limit.
func (a *Autoscaler) GetScaleHistory(limit int) []ScaleEvent {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if limit <= 0 || limit > len(a.scaleHistory) {
		limit = len(a.scaleHistory)
	}
	start := len(a.scaleHistory) - limit
	out := make([]ScaleEvent, limit)
	copy(out, a.scaleHistory[start:])
	return out
}

// CurrentReplicas returns the active replica count.
func (a *Autoscaler) CurrentReplicas() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentReplicas
}

// Stats returns a snapshot of the autoscaler state.
func (a *Autoscaler) Stats() AutoscalerStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := AutoscalerStats{
		CurrentReplicas: a.currentReplicas,
		MinReplicas:     a.config.MinReplicas,
		MaxReplicas:     a.config.MaxReplicas,
		TotalShards:     len(a.shardMap),
		ScaleEvents:     len(a.scaleHistory),
		MetricSummary:   make(map[string]float64),
		PolicyCount:     len(a.policies),
	}

	if !a.lastScaleUp.IsZero() {
		t := a.lastScaleUp
		stats.LastScaleUp = &t
	}
	if !a.lastScaleDown.IsZero() {
		t := a.lastScaleDown
		stats.LastScaleDown = &t
	}

	for mt, w := range a.metrics {
		if avg, ok := windowAvgSlice(w.values); ok {
			stats.MetricSummary[string(mt)] = avg
		}
	}

	return stats
}

// windowAvg computes the average of a metric window (caller must hold mu).
func (a *Autoscaler) windowAvg(mt MetricType) (float64, bool) {
	w, ok := a.metrics[mt]
	if !ok || len(w.values) == 0 {
		return 0, false
	}
	return windowAvgSlice(w.values)
}

func windowAvgSlice(samples []MetricSample) (float64, bool) {
	if len(samples) == 0 {
		return 0, false
	}
	var sum float64
	for _, s := range samples {
		sum += s.Value
	}
	return sum / float64(len(samples)), true
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
