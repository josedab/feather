package operator

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// FeaturePipeline represents a CRD for declarative feature pipelines.
type FeaturePipeline struct {
	TypeMeta   TypeMeta           `json:"typeMeta"`
	ObjectMeta ObjectMeta         `json:"metadata"`
	Spec       FeaturePipelineSpec   `json:"spec"`
	Status     FeaturePipelineStatus `json:"status"`
}

// FeaturePipelineSpec defines the desired state of a feature pipeline.
type FeaturePipelineSpec struct {
	// Source defines where data comes from.
	Source PipelineSource `json:"source"`
	// Transforms defines the transformation chain.
	Transforms []PipelineTransform `json:"transforms,omitempty"`
	// Sink defines where processed features are stored.
	Sink PipelineSink `json:"sink"`
	// Schedule defines the pipeline execution schedule (cron format).
	Schedule string `json:"schedule,omitempty"`
	// Trigger defines event-based triggers.
	Trigger *PipelineTrigger `json:"trigger,omitempty"`
	// Resources defines compute resource requirements.
	Resources ResourceRequirements `json:"resources,omitempty"`
	// Parallelism sets the maximum parallel workers.
	Parallelism int `json:"parallelism,omitempty"`
	// RetryPolicy configures retry behavior.
	RetryPolicy *RetryPolicy `json:"retryPolicy,omitempty"`
}

// PipelineSource defines a data source for a pipeline.
type PipelineSource struct {
	Type       string            `json:"type"` // kafka, http, batch, warehouse
	Config     map[string]string `json:"config,omitempty"`
	EntityType string            `json:"entityType,omitempty"`
}

// PipelineTransform defines a transformation step.
type PipelineTransform struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"` // sql, expression, wasm, python
	Expression string            `json:"expression,omitempty"`
	Config     map[string]string `json:"config,omitempty"`
}

// PipelineSink defines the output destination.
type PipelineSink struct {
	FeatureGroup string `json:"featureGroup"`
	FeatureName  string `json:"featureName,omitempty"`
	WriteMode    string `json:"writeMode,omitempty"` // upsert, append, overwrite
}

// PipelineTrigger defines event-based pipeline triggers.
type PipelineTrigger struct {
	Type   string `json:"type"` // on_change, on_schedule, on_demand
	Source string `json:"source,omitempty"`
	Event  string `json:"event,omitempty"`
}

// RetryPolicy configures retry behavior.
type RetryPolicy struct {
	MaxRetries    int           `json:"maxRetries"`
	BackoffDelay  time.Duration `json:"backoffDelay"`
	BackoffFactor float64       `json:"backoffFactor"`
}

// FeaturePipelineStatus represents the observed state of a pipeline.
type FeaturePipelineStatus struct {
	Phase            string       `json:"phase"` // Pending, Running, Succeeded, Failed
	LastRunTime      *time.Time   `json:"lastRunTime,omitempty"`
	LastSuccessTime  *time.Time   `json:"lastSuccessTime,omitempty"`
	RunCount         int64        `json:"runCount"`
	SuccessCount     int64        `json:"successCount"`
	FailureCount     int64        `json:"failureCount"`
	ProcessedRecords int64        `json:"processedRecords"`
	Conditions       []Condition  `json:"conditions,omitempty"`
	Message          string       `json:"message,omitempty"`
}

// AutoScalePolicy defines auto-scaling behavior for FeatureStore instances.
type AutoScalePolicy struct {
	Enabled              bool    `json:"enabled"`
	MinReplicas          int     `json:"minReplicas"`
	MaxReplicas          int     `json:"maxReplicas"`
	TargetCPUPercent     int     `json:"targetCPUPercent,omitempty"`
	TargetMemoryPercent  int     `json:"targetMemoryPercent,omitempty"`
	TargetLatencyP99Ms   int     `json:"targetLatencyP99Ms,omitempty"`
	ScaleUpCooldown      time.Duration `json:"scaleUpCooldown,omitempty"`
	ScaleDownCooldown    time.Duration `json:"scaleDownCooldown,omitempty"`
	CustomMetrics        []CustomMetric `json:"customMetrics,omitempty"`
}

// CustomMetric defines a custom metric for auto-scaling decisions.
type CustomMetric struct {
	Name       string  `json:"name"`
	Query      string  `json:"query"` // Prometheus query
	Threshold  float64 `json:"threshold"`
	ScaleUp    bool    `json:"scaleUp"`
}

// DefaultAutoScalePolicy returns sensible autoscaling defaults.
func DefaultAutoScalePolicy() AutoScalePolicy {
	return AutoScalePolicy{
		Enabled:            true,
		MinReplicas:        1,
		MaxReplicas:        10,
		TargetCPUPercent:   70,
		TargetMemoryPercent: 80,
		TargetLatencyP99Ms: 10,
		ScaleUpCooldown:    60 * time.Second,
		ScaleDownCooldown:  300 * time.Second,
	}
}

// ScaleDecision represents an auto-scaling recommendation.
type ScaleDecision struct {
	CurrentReplicas int       `json:"current_replicas"`
	DesiredReplicas int       `json:"desired_replicas"`
	Reason          string    `json:"reason"`
	Metric          string    `json:"metric"`
	MetricValue     float64   `json:"metric_value"`
	Threshold       float64   `json:"threshold"`
	DecidedAt       time.Time `json:"decided_at"`
}

// AutoScaler evaluates metrics and recommends scaling actions.
type AutoScaler struct {
	mu       sync.RWMutex
	policy   AutoScalePolicy
	current  int
	history  []ScaleDecision
}

// NewAutoScaler creates a new autoscaler with the given policy.
func NewAutoScaler(policy AutoScalePolicy) *AutoScaler {
	return &AutoScaler{
		policy:  policy,
		current: policy.MinReplicas,
	}
}

// Evaluate checks metrics and returns a scaling decision.
func (a *AutoScaler) Evaluate(ctx context.Context, metrics map[string]float64) *ScaleDecision {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.policy.Enabled {
		return nil
	}

	desired := a.current

	// Check CPU.
	if cpu, ok := metrics["cpu_percent"]; ok && a.policy.TargetCPUPercent > 0 {
		if cpu > float64(a.policy.TargetCPUPercent) {
			desired = min(a.current+1, a.policy.MaxReplicas)
			if desired != a.current {
				return a.recordDecision(desired, "cpu_percent", cpu, float64(a.policy.TargetCPUPercent), "CPU above target")
			}
		} else if cpu < float64(a.policy.TargetCPUPercent)*0.5 && a.current > a.policy.MinReplicas {
			desired = max(a.current-1, a.policy.MinReplicas)
			if desired != a.current {
				return a.recordDecision(desired, "cpu_percent", cpu, float64(a.policy.TargetCPUPercent), "CPU well below target")
			}
		}
	}

	// Check memory.
	if mem, ok := metrics["memory_percent"]; ok && a.policy.TargetMemoryPercent > 0 {
		if mem > float64(a.policy.TargetMemoryPercent) {
			desired = min(a.current+1, a.policy.MaxReplicas)
			if desired != a.current {
				return a.recordDecision(desired, "memory_percent", mem, float64(a.policy.TargetMemoryPercent), "Memory above target")
			}
		}
	}

	// Check latency.
	if lat, ok := metrics["latency_p99_ms"]; ok && a.policy.TargetLatencyP99Ms > 0 {
		if lat > float64(a.policy.TargetLatencyP99Ms) {
			desired = min(a.current+1, a.policy.MaxReplicas)
			if desired != a.current {
				return a.recordDecision(desired, "latency_p99_ms", lat, float64(a.policy.TargetLatencyP99Ms), "Latency above target")
			}
		}
	}

	return nil
}

// CurrentReplicas returns the current replica count.
func (a *AutoScaler) CurrentReplicas() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.current
}

// History returns recent scaling decisions.
func (a *AutoScaler) History() []ScaleDecision {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]ScaleDecision, len(a.history))
	copy(result, a.history)
	return result
}

func (a *AutoScaler) recordDecision(desired int, metric string, value, threshold float64, reason string) *ScaleDecision {
	decision := &ScaleDecision{
		CurrentReplicas: a.current,
		DesiredReplicas: desired,
		Reason:          reason,
		Metric:          metric,
		MetricValue:     value,
		Threshold:       threshold,
		DecidedAt:       time.Now(),
	}
	a.current = desired
	a.history = append(a.history, *decision)
	if len(a.history) > 100 {
		a.history = a.history[len(a.history)-100:]
	}
	return decision
}

// PipelineController manages FeaturePipeline resources.
type PipelineController struct {
	mu        sync.RWMutex
	pipelines map[string]*FeaturePipeline
	callbacks []func(*FeaturePipeline) error
}

// NewPipelineController creates a new pipeline controller.
func NewPipelineController() *PipelineController {
	return &PipelineController{
		pipelines: make(map[string]*FeaturePipeline),
	}
}

// OnChange registers a callback for pipeline changes.
func (c *PipelineController) OnChange(fn func(*FeaturePipeline) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callbacks = append(c.callbacks, fn)
}

// Create creates a new FeaturePipeline.
func (c *PipelineController) Create(pipeline *FeaturePipeline) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s/%s", pipeline.ObjectMeta.Namespace, pipeline.ObjectMeta.Name)
	if _, exists := c.pipelines[key]; exists {
		return fmt.Errorf("pipeline %q already exists", key)
	}

	if pipeline.Spec.Source.Type == "" {
		return fmt.Errorf("pipeline source type is required")
	}
	if pipeline.Spec.Sink.FeatureGroup == "" {
		return fmt.Errorf("pipeline sink feature group is required")
	}

	now := time.Now()
	pipeline.ObjectMeta.CreationTimestamp = now
	pipeline.Status.Phase = "Pending"
	c.pipelines[key] = pipeline

	for _, cb := range c.callbacks {
		_ = cb(pipeline)
	}
	return nil
}

// Get retrieves a pipeline by namespace and name.
func (c *PipelineController) Get(namespace, name string) (*FeaturePipeline, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	p, ok := c.pipelines[key]
	if !ok {
		return nil, fmt.Errorf("pipeline %q not found", key)
	}
	return p, nil
}

// List returns all pipelines in a namespace.
func (c *PipelineController) List(namespace string) []*FeaturePipeline {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*FeaturePipeline
	for _, p := range c.pipelines {
		if namespace == "" || p.ObjectMeta.Namespace == namespace {
			result = append(result, p)
		}
	}
	return result
}

// Delete removes a pipeline.
func (c *PipelineController) Delete(namespace, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	if _, ok := c.pipelines[key]; !ok {
		return fmt.Errorf("pipeline %q not found", key)
	}
	delete(c.pipelines, key)
	return nil
}

// UpdateStatus updates a pipeline's status.
func (c *PipelineController) UpdateStatus(namespace, name string, status FeaturePipelineStatus) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	p, ok := c.pipelines[key]
	if !ok {
		return fmt.Errorf("pipeline %q not found", key)
	}
	p.Status = status
	return nil
}
