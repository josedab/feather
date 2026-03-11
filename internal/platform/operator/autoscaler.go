package operator

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// ScalingPolicy defines how a FeatureStore scales.
type ScalingPolicy struct {
	MinReplicas       int32         `json:"min_replicas" yaml:"min_replicas"`
	MaxReplicas       int32         `json:"max_replicas" yaml:"max_replicas"`
	TargetQPS         float64       `json:"target_qps" yaml:"target_qps"`
	TargetLatencyMs   float64       `json:"target_latency_ms" yaml:"target_latency_ms"`
	ScaleUpCooldown   time.Duration `json:"scale_up_cooldown" yaml:"scale_up_cooldown"`
	ScaleDownCooldown time.Duration `json:"scale_down_cooldown" yaml:"scale_down_cooldown"`
	ScaleUpPercent    float64       `json:"scale_up_percent" yaml:"scale_up_percent"`
	ScaleDownPercent  float64       `json:"scale_down_percent" yaml:"scale_down_percent"`
}

// DefaultScalingPolicy returns sensible defaults.
func DefaultScalingPolicy() ScalingPolicy {
	return ScalingPolicy{
		MinReplicas:       1,
		MaxReplicas:       10,
		TargetQPS:         1000,
		TargetLatencyMs:   50,
		ScaleUpCooldown:   3 * time.Minute,
		ScaleDownCooldown: 5 * time.Minute,
		ScaleUpPercent:    50,
		ScaleDownPercent:  25,
	}
}

// MetricsSnapshot represents observed metrics at a point in time.
type MetricsSnapshot struct {
	CurrentQPS      float64   `json:"current_qps"`
	P99LatencyMs    float64   `json:"p99_latency_ms"`
	CPUPercent      float64   `json:"cpu_percent"`
	MemoryPercent   float64   `json:"memory_percent"`
	CurrentReplicas int32     `json:"current_replicas"`
	Timestamp       time.Time `json:"timestamp"`
}

// ScalingDecision represents the autoscaler's output.
type ScalingDecision struct {
	FeatureStore    string          `json:"feature_store"`
	CurrentReplicas int32           `json:"current_replicas"`
	DesiredReplicas int32           `json:"desired_replicas"`
	Reason          string          `json:"reason"`
	Metrics         MetricsSnapshot `json:"metrics"`
	DecidedAt       time.Time       `json:"decided_at"`
}

// AutoscalerStats tracks autoscaler activity.
type AutoscalerStats struct {
	TotalDecisions int64 `json:"total_decisions"`
	ScaleUps       int64 `json:"scale_ups"`
	ScaleDowns     int64 `json:"scale_downs"`
	NoChanges      int64 `json:"no_changes"`
}

// Autoscaler manages replica scaling for FeatureStore resources.
type Autoscaler struct {
	mu            sync.RWMutex
	policies      map[string]*ScalingPolicy
	lastScaleUp   map[string]time.Time
	lastScaleDown map[string]time.Time
	history       []ScalingDecision
	stats         AutoscalerStats
}

// NewAutoscaler creates a new autoscaler.
func NewAutoscaler() *Autoscaler {
	return &Autoscaler{
		policies:      make(map[string]*ScalingPolicy),
		lastScaleUp:   make(map[string]time.Time),
		lastScaleDown: make(map[string]time.Time),
	}
}

// SetPolicy configures scaling policy for a FeatureStore.
func (a *Autoscaler) SetPolicy(name string, policy ScalingPolicy) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.policies[name] = &policy
}

// Evaluate computes the desired replica count based on current metrics.
func (a *Autoscaler) Evaluate(name string, snapshot MetricsSnapshot) (*ScalingDecision, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	policy, exists := a.policies[name]
	if !exists {
		return nil, fmt.Errorf("no scaling policy for %s", name)
	}

	a.stats.TotalDecisions++
	current := snapshot.CurrentReplicas
	desired := current

	// Scale based on QPS
	if policy.TargetQPS > 0 && snapshot.CurrentQPS > 0 {
		qpsRatio := snapshot.CurrentQPS / (policy.TargetQPS * float64(current))
		if qpsRatio > 1.0 {
			scaledVal := math.Ceil(float64(current) * qpsRatio)
			if scaledVal > float64(policy.MaxReplicas) {
				desired = policy.MaxReplicas
			} else {
				desired = int32(scaledVal)
			}
		}
	}

	// Scale based on latency
	if policy.TargetLatencyMs > 0 && snapshot.P99LatencyMs > policy.TargetLatencyMs {
		latencyRatio := snapshot.P99LatencyMs / policy.TargetLatencyMs
		scaledVal := math.Ceil(float64(current) * latencyRatio)
		if scaledVal > float64(policy.MaxReplicas) {
			scaledVal = float64(policy.MaxReplicas)
		}
		scaleTarget := int32(scaledVal)
		if scaleTarget > desired {
			desired = scaleTarget
		}
	}

	// Scale based on CPU
	if snapshot.CPUPercent > 80 {
		scaledVal := math.Ceil(float64(current) * (snapshot.CPUPercent / 70))
		if scaledVal > float64(policy.MaxReplicas) {
			scaledVal = float64(policy.MaxReplicas)
		}
		cpuTarget := int32(scaledVal)
		if cpuTarget > desired {
			desired = cpuTarget
		}
	}

	// Apply limits
	if desired < policy.MinReplicas {
		desired = policy.MinReplicas
	}
	if desired > policy.MaxReplicas {
		desired = policy.MaxReplicas
	}

	reason := "no change"
	if desired > current {
		// Check cooldown
		if last, ok := a.lastScaleUp[name]; ok && time.Since(last) < policy.ScaleUpCooldown {
			desired = current
			reason = "scale up cooldown"
		} else {
			a.lastScaleUp[name] = time.Now()
			a.stats.ScaleUps++
			reason = fmt.Sprintf("scaling up: qps=%.0f latency=%.1fms", snapshot.CurrentQPS, snapshot.P99LatencyMs)
		}
	} else if desired < current {
		if last, ok := a.lastScaleDown[name]; ok && time.Since(last) < policy.ScaleDownCooldown {
			desired = current
			reason = "scale down cooldown"
		} else {
			a.lastScaleDown[name] = time.Now()
			a.stats.ScaleDowns++
			reason = fmt.Sprintf("scaling down: qps=%.0f latency=%.1fms", snapshot.CurrentQPS, snapshot.P99LatencyMs)
		}
	} else {
		a.stats.NoChanges++
	}

	decision := &ScalingDecision{
		FeatureStore:    name,
		CurrentReplicas: current,
		DesiredReplicas: desired,
		Reason:          reason,
		Metrics:         snapshot,
		DecidedAt:       time.Now(),
	}

	a.history = append(a.history, *decision)
	if len(a.history) > 1000 {
		a.history = a.history[len(a.history)-500:]
	}

	return decision, nil
}

// GetHistory returns recent scaling decisions for a FeatureStore.
func (a *Autoscaler) GetHistory(name string, limit int) []ScalingDecision {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var result []ScalingDecision
	for i := len(a.history) - 1; i >= 0; i-- {
		if a.history[i].FeatureStore == name {
			result = append(result, a.history[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

// Stats returns autoscaler statistics.
func (a *Autoscaler) Stats() AutoscalerStats {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.stats
}
