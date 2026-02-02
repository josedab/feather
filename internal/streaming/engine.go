// Package streaming provides real-time feature computation from event streams.
// It supports windowed aggregations, complex event processing (CEP), and
// integration with message brokers like Kafka.
package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/domain"
)

// Engine processes streaming events and computes real-time features.
type Engine struct {
	mu         sync.RWMutex
	pipelines  map[string]*Pipeline
	windows    map[string]*WindowManager
	cep        *CEPEngine
	config     Config
	logger     *slog.Logger
	metrics    *Metrics
	outputChan chan *FeatureOutput
	ctx        context.Context
	cancel     context.CancelFunc
}

// Config configures the streaming engine.
type Config struct {
	// MaxPipelines is the maximum number of concurrent pipelines
	MaxPipelines int

	// DefaultWindowSize is the default window duration
	DefaultWindowSize time.Duration

	// DefaultSlideInterval is the default slide interval for sliding windows
	DefaultSlideInterval time.Duration

	// BufferSize is the size of internal buffers
	BufferSize int

	// CheckpointInterval is how often to checkpoint state
	CheckpointInterval time.Duration

	// LateEventTolerance is how late an event can be and still be processed
	LateEventTolerance time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxPipelines:         100,
		DefaultWindowSize:    1 * time.Minute,
		DefaultSlideInterval: 10 * time.Second,
		BufferSize:           10000,
		CheckpointInterval:   30 * time.Second,
		LateEventTolerance:   5 * time.Minute,
	}
}

// Event represents an incoming event to process.
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	EntityID  string                 `json:"entity_id"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// FeatureOutput represents a computed feature value.
type FeatureOutput struct {
	FeatureName string
	EntityID    string
	Value       interface{}
	Timestamp   time.Time
	WindowStart time.Time
	WindowEnd   time.Time
}

// Pipeline represents a feature computation pipeline.
type Pipeline struct {
	ID          string
	Name        string
	Description string
	InputType   string // Event type to listen for
	Stages      []Stage
	Windows     []WindowConfig
	CEPPatterns []CEPPattern
	OutputFunc  OutputFunction
	State       PipelineState
	CreatedAt   time.Time
	Metrics     *PipelineMetrics
}

// PipelineState represents the current state of a pipeline.
type PipelineState int

const (
	// StateStopped indicates the pipeline is stopped.
	StateStopped PipelineState = iota
	// StateRunning indicates the pipeline is running.
	StateRunning
	// StatePaused indicates the pipeline is paused.
	StatePaused
	// StateError indicates the pipeline is in an error state.
	StateError
)

func (s PipelineState) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateRunning:
		return "running"
	case StatePaused:
		return "paused"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// Stage represents a processing stage in the pipeline.
type Stage struct {
	Name      string
	Type      StageType
	Config    json.RawMessage
	Processor Processor
}

// StageType indicates the type of processing stage.
type StageType string

const (
	// StageTypeFilter applies filtering conditions.
	StageTypeFilter StageType = "filter"
	// StageTypeTransform applies transformations.
	StageTypeTransform StageType = "transform"
	// StageTypeAggregate applies aggregations.
	StageTypeAggregate StageType = "aggregate"
	// StageTypeJoin joins streams.
	StageTypeJoin StageType = "join"
	// StageTypeWindow applies windowing.
	StageTypeWindow StageType = "window"
	// StageTypeEnrich enriches events with external data.
	StageTypeEnrich StageType = "enrich"
)

// Processor processes events in a stage.
type Processor interface {
	Process(ctx context.Context, event *Event) (*Event, error)
	Name() string
}

// OutputFunction handles feature output.
type OutputFunction func(ctx context.Context, output *FeatureOutput) error

// WindowConfig configures a window for aggregation.
type WindowConfig struct {
	Name          string
	Type          WindowType
	Size          time.Duration
	SlideInterval time.Duration
	Aggregations  []AggregationConfig
}

// WindowType indicates the type of window.
type WindowType string

const (
	// WindowTypeTumbling represents fixed, non-overlapping windows.
	WindowTypeTumbling WindowType = "tumbling"
	// WindowTypeSliding represents sliding windows.
	WindowTypeSliding WindowType = "sliding"
	// WindowTypeSession represents session windows.
	WindowTypeSession WindowType = "session"
	// WindowTypeGlobal represents a global window.
	WindowTypeGlobal WindowType = "global"
)

// AggregationConfig configures an aggregation function.
type AggregationConfig struct {
	Name       string
	Field      string
	Function   AggFunction
	OutputName string
}

// AggFunction indicates the aggregation function.
type AggFunction string

const (
	// AggCount counts values.
	AggCount AggFunction = "count"
	// AggSum sums values.
	AggSum AggFunction = "sum"
	// AggAvg averages values.
	AggAvg AggFunction = "avg"
	// AggMin finds the minimum value.
	AggMin AggFunction = "min"
	// AggMax finds the maximum value.
	AggMax AggFunction = "max"
	// AggFirst takes the first value.
	AggFirst AggFunction = "first"
	// AggLast takes the last value.
	AggLast AggFunction = "last"
	// AggDistinct counts distinct values.
	AggDistinct AggFunction = "distinct"
	// AggPercentile computes percentiles.
	AggPercentile AggFunction = "percentile"
	// AggStdDev computes standard deviation.
	AggStdDev AggFunction = "stddev"
	// AggVariance computes variance.
	AggVariance AggFunction = "variance"
)

// PipelineMetrics tracks pipeline performance.
type PipelineMetrics struct {
	mu               sync.RWMutex
	EventsProcessed  int64
	EventsDropped    int64
	LateEvents       int64
	ProcessingErrors int64
	AvgLatencyMs     float64
	LastEventTime    time.Time
	WindowsComputed  int64
}

// Metrics tracks engine-level metrics.
type Metrics struct {
	mu              sync.RWMutex
	TotalEvents     int64
	TotalOutputs    int64
	ActivePipelines int
	TotalErrors     int64
}

// NewEngine creates a new streaming engine.
func NewEngine(config Config, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
		pipelines:  make(map[string]*Pipeline),
		windows:    make(map[string]*WindowManager),
		cep:        NewCEPEngine(),
		config:     config,
		logger:     logger,
		metrics:    &Metrics{},
		outputChan: make(chan *FeatureOutput, config.BufferSize),
		ctx:        ctx,
		cancel:     cancel,
	}

	return e
}

// Start starts the streaming engine.
func (e *Engine) Start() error {
	e.logger.Info("Starting streaming engine")

	// Start output processor
	go e.processOutputs()

	// Start window maintenance
	go e.maintainWindows()

	return nil
}

// Stop stops the streaming engine.
func (e *Engine) Stop() error {
	e.logger.Info("Stopping streaming engine")
	e.cancel()

	// Stop all pipelines
	e.mu.Lock()
	for _, p := range e.pipelines {
		p.State = StateStopped
	}
	e.mu.Unlock()

	return nil
}

// CreatePipeline creates a new feature computation pipeline.
func (e *Engine) CreatePipeline(pipeline *Pipeline) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.pipelines) >= e.config.MaxPipelines {
		return fmt.Errorf("maximum number of pipelines reached: %d", e.config.MaxPipelines)
	}

	if pipeline.ID == "" {
		pipeline.ID = fmt.Sprintf("pipeline-%d", time.Now().UnixNano())
	}

	pipeline.CreatedAt = time.Now()
	pipeline.State = StateStopped
	pipeline.Metrics = &PipelineMetrics{}

	// Initialize windows for this pipeline
	for _, wc := range pipeline.Windows {
		wm := NewWindowManager(wc, e.config.LateEventTolerance)
		e.windows[pipeline.ID+":"+wc.Name] = wm
	}

	// Register CEP patterns
	for _, pattern := range pipeline.CEPPatterns {
		e.cep.RegisterPattern(pipeline.ID, pattern)
	}

	e.pipelines[pipeline.ID] = pipeline
	e.metrics.ActivePipelines = len(e.pipelines)

	e.logger.Info("Created pipeline", "id", pipeline.ID, "name", pipeline.Name)

	return nil
}

// StartPipeline starts a pipeline.
func (e *Engine) StartPipeline(pipelineID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	pipeline, ok := e.pipelines[pipelineID]
	if !ok {
		return fmt.Errorf("pipeline not found: %s", pipelineID)
	}

	pipeline.State = StateRunning
	e.logger.Info("Started pipeline", "id", pipelineID)

	return nil
}

// StopPipeline stops a pipeline.
func (e *Engine) StopPipeline(pipelineID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	pipeline, ok := e.pipelines[pipelineID]
	if !ok {
		return fmt.Errorf("pipeline not found: %s", pipelineID)
	}

	pipeline.State = StateStopped
	e.logger.Info("Stopped pipeline", "id", pipelineID)

	return nil
}

// DeletePipeline deletes a pipeline.
func (e *Engine) DeletePipeline(pipelineID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.pipelines[pipelineID]; !ok {
		return fmt.Errorf("pipeline not found: %s", pipelineID)
	}

	delete(e.pipelines, pipelineID)
	e.metrics.ActivePipelines = len(e.pipelines)

	// Clean up windows
	for key := range e.windows {
		if len(key) > len(pipelineID) && key[:len(pipelineID)+1] == pipelineID+":" {
			delete(e.windows, key)
		}
	}

	e.logger.Info("Deleted pipeline", "id", pipelineID)

	return nil
}

// GetPipeline returns a pipeline by ID.
func (e *Engine) GetPipeline(pipelineID string) (*Pipeline, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	pipeline, ok := e.pipelines[pipelineID]
	if !ok {
		return nil, fmt.Errorf("pipeline not found: %s", pipelineID)
	}

	return pipeline, nil
}

// ListPipelines returns all pipelines.
func (e *Engine) ListPipelines() []*Pipeline {
	e.mu.RLock()
	defer e.mu.RUnlock()

	pipelines := make([]*Pipeline, 0, len(e.pipelines))
	for _, p := range e.pipelines {
		pipelines = append(pipelines, p)
	}
	return pipelines
}

// ProcessEvent processes an incoming event.
func (e *Engine) ProcessEvent(ctx context.Context, event *Event) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.metrics.mu.Lock()
	e.metrics.TotalEvents++
	e.metrics.mu.Unlock()

	// Find matching pipelines
	for _, pipeline := range e.pipelines {
		if pipeline.State != StateRunning {
			continue
		}

		if pipeline.InputType != "" && pipeline.InputType != event.Type {
			continue
		}

		if err := e.processEventInPipeline(ctx, pipeline, event); err != nil {
			e.logger.Error("Error processing event in pipeline",
				"pipeline", pipeline.ID, "error", err)
			pipeline.Metrics.mu.Lock()
			pipeline.Metrics.ProcessingErrors++
			pipeline.Metrics.mu.Unlock()
		}
	}

	return nil
}

func (e *Engine) processEventInPipeline(ctx context.Context, pipeline *Pipeline, event *Event) error {
	startTime := time.Now()

	// Check for late events
	if time.Since(event.Timestamp) > e.config.LateEventTolerance {
		pipeline.Metrics.mu.Lock()
		pipeline.Metrics.LateEvents++
		pipeline.Metrics.mu.Unlock()
		return nil
	}

	// Process through stages
	currentEvent := event
	for _, stage := range pipeline.Stages {
		if stage.Processor == nil {
			continue
		}

		processed, err := stage.Processor.Process(ctx, currentEvent)
		if err != nil {
			return fmt.Errorf("stage %s: %w", stage.Name, err)
		}

		if processed == nil {
			// Event filtered out
			return nil
		}

		currentEvent = processed
	}

	// Add to windows
	for _, wc := range pipeline.Windows {
		wm, ok := e.windows[pipeline.ID+":"+wc.Name]
		if !ok {
			continue
		}

		wm.AddEvent(currentEvent)
	}

	// Check CEP patterns
	matches := e.cep.MatchEvent(pipeline.ID, currentEvent)
	for _, match := range matches {
		output := &FeatureOutput{
			FeatureName: match.PatternName + "_matched",
			EntityID:    currentEvent.EntityID,
			Value:       match,
			Timestamp:   time.Now(),
		}
		e.emitOutput(output)
	}

	// Update metrics
	pipeline.Metrics.mu.Lock()
	pipeline.Metrics.EventsProcessed++
	pipeline.Metrics.LastEventTime = time.Now()

	// Update average latency
	latency := float64(time.Since(startTime).Milliseconds())
	pipeline.Metrics.AvgLatencyMs = (pipeline.Metrics.AvgLatencyMs*float64(pipeline.Metrics.EventsProcessed-1) + latency) / float64(pipeline.Metrics.EventsProcessed)
	pipeline.Metrics.mu.Unlock()

	return nil
}

func (e *Engine) emitOutput(output *FeatureOutput) {
	select {
	case e.outputChan <- output:
		e.metrics.mu.Lock()
		e.metrics.TotalOutputs++
		e.metrics.mu.Unlock()
	default:
		// Buffer full, drop output
		e.logger.Warn("Output buffer full, dropping output", "feature", output.FeatureName)
	}
}

func (e *Engine) processOutputs() {
	for {
		select {
		case <-e.ctx.Done():
			return
		case output := <-e.outputChan:
			// Find pipeline and call output function
			e.mu.RLock()
			for _, pipeline := range e.pipelines {
				if pipeline.OutputFunc != nil {
					if err := pipeline.OutputFunc(e.ctx, output); err != nil {
						e.logger.Error("Error in output function",
							"pipeline", pipeline.ID, "error", err)
					}
				}
			}
			e.mu.RUnlock()
		}
	}
}

func (e *Engine) maintainWindows() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.mu.RLock()
			for key, wm := range e.windows {
				// Find pipeline
				pipelineID := key[:len(key)-len(wm.config.Name)-1]
				pipeline, ok := e.pipelines[pipelineID]
				if !ok {
					continue
				}

				// Compute and emit window results
				results := wm.ComputeAndEvict(time.Now())
				for _, result := range results {
					output := &FeatureOutput{
						FeatureName: result.Name,
						EntityID:    result.EntityID,
						Value:       result.Value,
						Timestamp:   time.Now(),
						WindowStart: result.WindowStart,
						WindowEnd:   result.WindowEnd,
					}
					e.emitOutput(output)
					pipeline.Metrics.mu.Lock()
					pipeline.Metrics.WindowsComputed++
					pipeline.Metrics.mu.Unlock()
				}
			}
			e.mu.RUnlock()
		}
	}
}

// GetOutputChannel returns the output channel for consumers.
func (e *Engine) GetOutputChannel() <-chan *FeatureOutput {
	return e.outputChan
}

// GetMetrics returns engine metrics.
func (e *Engine) GetMetrics() *Metrics {
	e.metrics.mu.RLock()
	defer e.metrics.mu.RUnlock()

	return &Metrics{
		TotalEvents:     e.metrics.TotalEvents,
		TotalOutputs:    e.metrics.TotalOutputs,
		ActivePipelines: e.metrics.ActivePipelines,
		TotalErrors:     e.metrics.TotalErrors,
	}
}

// CreateFeatureValue converts a FeatureOutput to a domain.FeatureValue.
func (o *FeatureOutput) CreateFeatureValue() domain.FeatureValue {
	return domain.FeatureValue{
		Value:     o.Value,
		Timestamp: o.Timestamp.UnixNano(),
	}
}
