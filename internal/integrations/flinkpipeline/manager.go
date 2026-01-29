package flinkpipeline

import (
	"fmt"
	"sync"
	"time"
)

// PipelineStatus represents the current state of a pipeline.
type PipelineStatus string

const (
	StatusCreated  PipelineStatus = "created"
	StatusRunning  PipelineStatus = "running"
	StatusStopped  PipelineStatus = "stopped"
	StatusFailed   PipelineStatus = "failed"
	StatusDraining PipelineStatus = "draining"
)

// RuntimeType specifies the streaming runtime.
type RuntimeType string

const (
	RuntimeFlink        RuntimeType = "flink"
	RuntimeKafkaStreams RuntimeType = "kafka_streams"
	RuntimeBuiltin      RuntimeType = "builtin"
)

// WindowType defines the windowing strategy.
type WindowType string

const (
	WindowTumbling WindowType = "tumbling"
	WindowSliding  WindowType = "sliding"
	WindowSession  WindowType = "session"
)

// AggregationType defines aggregation functions for windowed computations.
type AggregationType string

const (
	AggCount      AggregationType = "count"
	AggSum        AggregationType = "sum"
	AggAvg        AggregationType = "avg"
	AggMin        AggregationType = "min"
	AggMax        AggregationType = "max"
	AggDistinct   AggregationType = "distinct_count"
	AggPercentile AggregationType = "percentile"
	AggLastValue  AggregationType = "last_value"
)

// Source defines an event stream source.
type Source struct {
	Type          string            `json:"type"` // "kafka", "kinesis", "http"
	Topic         string            `json:"topic,omitempty"`
	Brokers       []string          `json:"brokers,omitempty"`
	ConsumerGroup string            `json:"consumer_group,omitempty"`
	Format        string            `json:"format,omitempty"` // "json", "avro", "protobuf"
	Properties    map[string]string `json:"properties,omitempty"`
}

// Sink defines a feature store sink.
type Sink struct {
	Type         string            `json:"type"` // "feather", "kafka", "file"
	FeatureGroup string            `json:"feature_group,omitempty"`
	Topic        string            `json:"topic,omitempty"`
	Properties   map[string]string `json:"properties,omitempty"`
}

// WindowSpec defines a window configuration for a pipeline stage.
type WindowSpec struct {
	Type     WindowType    `json:"type"`
	Size     time.Duration `json:"size"`
	Slide    time.Duration `json:"slide,omitempty"`
	Gap      time.Duration `json:"gap,omitempty"`
	MaxLate  time.Duration `json:"max_late,omitempty"`
	Timezone string        `json:"timezone,omitempty"`
}

// TransformStage defines a transformation step in a pipeline.
type TransformStage struct {
	Name        string          `json:"name"`
	Type        string          `json:"type"` // "filter", "map", "aggregate", "join", "flatmap"
	Expression  string          `json:"expression,omitempty"`
	KeyBy       string          `json:"key_by,omitempty"`
	Window      *WindowSpec     `json:"window,omitempty"`
	Aggregation AggregationType `json:"aggregation,omitempty"`
	OutputField string          `json:"output_field,omitempty"`
	FilterExpr  string          `json:"filter_expr,omitempty"`
}

// Pipeline defines a streaming feature computation graph.
type Pipeline struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Description  string           `json:"description,omitempty"`
	Runtime      RuntimeType      `json:"runtime"`
	Source       Source           `json:"source"`
	Sink         Sink             `json:"sink"`
	Stages       []TransformStage `json:"stages"`
	Parallelism  int              `json:"parallelism"`
	Status       PipelineStatus   `json:"status"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	StartedAt    *time.Time       `json:"started_at,omitempty"`
	StoppedAt    *time.Time       `json:"stopped_at,omitempty"`
	Checkpoint   *Checkpoint      `json:"checkpoint,omitempty"`
	ErrorMessage string           `json:"error_message,omitempty"`
}

// Checkpoint holds pipeline checkpoint state for recovery.
type Checkpoint struct {
	Offset    int64     `json:"offset"`
	Partition int       `json:"partition"`
	Timestamp time.Time `json:"timestamp"`
}

// PipelineStats holds runtime statistics for a pipeline.
type PipelineStats struct {
	EventsIn       int64         `json:"events_in"`
	EventsOut      int64         `json:"events_out"`
	EventsDropped  int64         `json:"events_dropped"`
	BytesIn        int64         `json:"bytes_in"`
	BytesOut       int64         `json:"bytes_out"`
	AvgLatency     time.Duration `json:"avg_latency_ns"`
	P99Latency     time.Duration `json:"p99_latency_ns"`
	WindowsEmitted int64         `json:"windows_emitted"`
	ErrorCount     int64         `json:"error_count"`
	LastEventAt    time.Time     `json:"last_event_at"`
	Uptime         time.Duration `json:"uptime_ns"`
}

// ManagerConfig configures the pipeline manager.
type ManagerConfig struct {
	MaxPipelines       int           `json:"max_pipelines"`
	DefaultParallelism int           `json:"default_parallelism"`
	CheckpointInterval time.Duration `json:"checkpoint_interval"`
	MetricsInterval    time.Duration `json:"metrics_interval"`
	FlinkJobManager    string        `json:"flink_job_manager,omitempty"`
	KafkaBrokers       []string      `json:"kafka_brokers,omitempty"`
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		MaxPipelines:       100,
		DefaultParallelism: 4,
		CheckpointInterval: 30 * time.Second,
		MetricsInterval:    10 * time.Second,
	}
}

// pipelineEntry holds a pipeline and its stats together.
type pipelineEntry struct {
	pipeline *Pipeline
	stats    PipelineStats
}

// Manager orchestrates streaming pipeline lifecycle.
type Manager struct {
	mu        sync.RWMutex
	config    ManagerConfig
	pipelines map[string]*pipelineEntry
	stats     ManagerStats
}

// ManagerStats holds aggregate manager statistics.
type ManagerStats struct {
	TotalPipelines   int   `json:"total_pipelines"`
	RunningPipelines int   `json:"running_pipelines"`
	TotalEventsIn    int64 `json:"total_events_in"`
	TotalEventsOut   int64 `json:"total_events_out"`
	TotalErrors      int64 `json:"total_errors"`
}

// NewManager creates a new pipeline manager.
func NewManager(config ManagerConfig) *Manager {
	if config.MaxPipelines == 0 {
		config = DefaultManagerConfig()
	}
	return &Manager{
		config:    config,
		pipelines: make(map[string]*pipelineEntry),
	}
}

// CreatePipeline registers a new streaming pipeline.
func (m *Manager) CreatePipeline(p Pipeline) (*Pipeline, error) {
	if err := m.validatePipeline(&p); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.pipelines[p.ID]; exists {
		return nil, ErrPipelineExists
	}
	if len(m.pipelines) >= m.config.MaxPipelines {
		return nil, fmt.Errorf("max pipelines reached (%d)", m.config.MaxPipelines)
	}

	now := time.Now()
	p.Status = StatusCreated
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.Parallelism <= 0 {
		p.Parallelism = m.config.DefaultParallelism
	}

	m.pipelines[p.ID] = &pipelineEntry{pipeline: &p}
	m.stats.TotalPipelines = len(m.pipelines)
	return &p, nil
}

// GetPipeline returns a pipeline by ID.
func (m *Manager) GetPipeline(id string) (*Pipeline, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.pipelines[id]
	if !exists {
		return nil, ErrPipelineNotFound
	}
	return entry.pipeline, nil
}

// ListPipelines returns all registered pipelines.
func (m *Manager) ListPipelines() []Pipeline {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Pipeline, 0, len(m.pipelines))
	for _, e := range m.pipelines {
		result = append(result, *e.pipeline)
	}
	return result
}

// StartPipeline transitions a pipeline to running state.
func (m *Manager) StartPipeline(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.pipelines[id]
	if !exists {
		return ErrPipelineNotFound
	}

	if entry.pipeline.Status == StatusRunning {
		return nil // idempotent
	}

	now := time.Now()
	entry.pipeline.Status = StatusRunning
	entry.pipeline.StartedAt = &now
	entry.pipeline.UpdatedAt = now
	entry.pipeline.ErrorMessage = ""
	m.stats.RunningPipelines++
	return nil
}

// StopPipeline transitions a pipeline to stopped state.
func (m *Manager) StopPipeline(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.pipelines[id]
	if !exists {
		return ErrPipelineNotFound
	}

	if entry.pipeline.Status != StatusRunning && entry.pipeline.Status != StatusDraining {
		return nil // already stopped
	}

	now := time.Now()
	entry.pipeline.Status = StatusStopped
	entry.pipeline.StoppedAt = &now
	entry.pipeline.UpdatedAt = now
	m.stats.RunningPipelines--
	return nil
}

// DeletePipeline removes a pipeline.
func (m *Manager) DeletePipeline(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.pipelines[id]
	if !exists {
		return ErrPipelineNotFound
	}

	if entry.pipeline.Status == StatusRunning {
		return ErrPipelineRunning
	}

	delete(m.pipelines, id)
	m.stats.TotalPipelines = len(m.pipelines)
	return nil
}

// GetPipelineStats returns runtime statistics for a pipeline.
func (m *Manager) GetPipelineStats(id string) (*PipelineStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.pipelines[id]
	if !exists {
		return nil, ErrPipelineNotFound
	}

	stats := entry.stats
	if entry.pipeline.StartedAt != nil && entry.pipeline.Status == StatusRunning {
		stats.Uptime = time.Since(*entry.pipeline.StartedAt)
	}
	return &stats, nil
}

// IngestEvent processes an event through the appropriate pipeline (for builtin runtime).
func (m *Manager) IngestEvent(pipelineID string, event map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.pipelines[pipelineID]
	if !exists {
		return ErrPipelineNotFound
	}

	if entry.pipeline.Status != StatusRunning {
		return fmt.Errorf("pipeline %s is not running (status: %s)", pipelineID, entry.pipeline.Status)
	}

	entry.stats.EventsIn++
	entry.stats.EventsOut++
	entry.stats.LastEventAt = time.Now()
	m.stats.TotalEventsIn++
	m.stats.TotalEventsOut++
	return nil
}

// Stats returns aggregate manager statistics.
func (m *Manager) Stats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

func (m *Manager) validatePipeline(p *Pipeline) error {
	if p.ID == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidPipeline)
	}
	if p.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidPipeline)
	}
	if p.Source.Type == "" {
		return fmt.Errorf("%w: source type is required", ErrInvalidPipeline)
	}
	if p.Sink.Type == "" {
		return fmt.Errorf("%w: sink type is required", ErrInvalidPipeline)
	}
	if p.Runtime == "" {
		p.Runtime = RuntimeBuiltin
	}
	return nil
}
