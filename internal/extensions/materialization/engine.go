package materialization

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// PipelineStatus represents the state of a pipeline or step.
type PipelineStatus string

const (
	StatusPending   PipelineStatus = "pending"
	StatusRunning   PipelineStatus = "running"
	StatusCompleted PipelineStatus = "completed"
	StatusFailed    PipelineStatus = "failed"
	StatusSkipped   PipelineStatus = "skipped"
)

// TriggerType defines how a pipeline is triggered.
type TriggerType string

const (
	TriggerCron    TriggerType = "cron"
	TriggerEvent   TriggerType = "event"
	TriggerManual  TriggerType = "manual"
)

var (
	ErrPipelineNotFound    = errors.New("pipeline not found")
	ErrPipelineExists      = errors.New("pipeline already exists")
	ErrCyclicDependency    = errors.New("cyclic dependency detected")
	ErrStepNotFound        = errors.New("step not found")
	ErrInvalidPipeline     = errors.New("invalid pipeline definition")
	ErrPipelineRunning     = errors.New("pipeline is already running")
	ErrRunNotFound         = errors.New("pipeline run not found")
)

// TransformFunc is a user-defined transformation applied during a step.
type TransformFunc func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)

// Step defines a single node in the pipeline DAG.
type Step struct {
	// Name uniquely identifies this step within the pipeline.
	Name string `json:"name"`
	// Description explains what this step does.
	Description string `json:"description,omitempty"`
	// DependsOn lists step names that must complete first.
	DependsOn []string `json:"depends_on,omitempty"`
	// Expression is a computation expression (e.g., "sum(clicks) / count(impressions)").
	Expression string `json:"expression,omitempty"`
	// OutputFeature is the feature name this step produces.
	OutputFeature string `json:"output_feature,omitempty"`
	// OutputGroup is the feature group for the output.
	OutputGroup string `json:"output_group,omitempty"`
	// Transform is an optional programmatic transform (not serializable).
	Transform TransformFunc `json:"-"`
	// RetryCount is how many times to retry on failure.
	RetryCount int `json:"retry_count,omitempty"`
	// Timeout for this step.
	Timeout time.Duration `json:"timeout,omitempty"`
}

// Pipeline defines a complete materialization pipeline.
type Pipeline struct {
	// Name uniquely identifies this pipeline.
	Name string `json:"name"`
	// Description explains the pipeline purpose.
	Description string `json:"description,omitempty"`
	// Steps are the DAG nodes.
	Steps []Step `json:"steps"`
	// Trigger defines how the pipeline is started.
	Trigger TriggerType `json:"trigger"`
	// Schedule is the cron expression (for TriggerCron).
	Schedule string `json:"schedule,omitempty"`
	// CreatedAt is when the pipeline was registered.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the pipeline was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// StepResult records the outcome of a single step execution.
type StepResult struct {
	StepName  string         `json:"step_name"`
	Status    PipelineStatus `json:"status"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   time.Time     `json:"ended_at"`
	Error     string         `json:"error,omitempty"`
	Output    map[string]interface{} `json:"output,omitempty"`
}

// Run records a single pipeline execution.
type Run struct {
	// ID uniquely identifies this run.
	ID string `json:"id"`
	// PipelineName is the pipeline that was executed.
	PipelineName string `json:"pipeline_name"`
	// Status is the overall run status.
	Status PipelineStatus `json:"status"`
	// Steps holds per-step results.
	Steps []StepResult `json:"steps"`
	// StartedAt is when the run began.
	StartedAt time.Time `json:"started_at"`
	// EndedAt is when the run completed.
	EndedAt time.Time `json:"ended_at"`
	// Trigger indicates how this run was initiated.
	Trigger TriggerType `json:"trigger"`
}

// EngineConfig configures the materialization engine.
type EngineConfig struct {
	// MaxConcurrentSteps limits parallel step execution within a pipeline.
	MaxConcurrentSteps int
	// DefaultStepTimeout is the default timeout per step.
	DefaultStepTimeout time.Duration
	// MaxRunHistory is the maximum number of runs to retain per pipeline.
	MaxRunHistory int
}

// DefaultEngineConfig returns sensible defaults.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		MaxConcurrentSteps: 4,
		DefaultStepTimeout: 5 * time.Minute,
		MaxRunHistory:      100,
	}
}

// Engine manages pipeline definitions and executions.
type Engine struct {
	mu        sync.RWMutex
	pipelines map[string]*Pipeline
	runs      map[string][]*Run // pipeline name -> runs
	config    EngineConfig
	runCount  int64
	stopCh    chan struct{}
}

// NewEngine creates a new materialization engine.
func NewEngine(config EngineConfig) *Engine {
	if config.MaxConcurrentSteps == 0 {
		config = DefaultEngineConfig()
	}
	return &Engine{
		pipelines: make(map[string]*Pipeline),
		runs:      make(map[string][]*Run),
		config:    config,
		stopCh:    make(chan struct{}),
	}
}

// RegisterPipeline adds a new pipeline definition.
func (e *Engine) RegisterPipeline(p *Pipeline) error {
	if p.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidPipeline)
	}
	if len(p.Steps) == 0 {
		return fmt.Errorf("%w: at least one step is required", ErrInvalidPipeline)
	}
	if err := e.validateDAG(p); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.pipelines[p.Name]; exists {
		return ErrPipelineExists
	}

	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	e.pipelines[p.Name] = p
	return nil
}

// UpdatePipeline updates an existing pipeline.
func (e *Engine) UpdatePipeline(p *Pipeline) error {
	if len(p.Steps) == 0 {
		return fmt.Errorf("%w: at least one step is required", ErrInvalidPipeline)
	}
	if err := e.validateDAG(p); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.pipelines[p.Name]; !exists {
		return ErrPipelineNotFound
	}

	p.UpdatedAt = time.Now()
	e.pipelines[p.Name] = p
	return nil
}

// DeletePipeline removes a pipeline.
func (e *Engine) DeletePipeline(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.pipelines[name]; !exists {
		return ErrPipelineNotFound
	}
	delete(e.pipelines, name)
	return nil
}

// GetPipeline retrieves a pipeline by name.
func (e *Engine) GetPipeline(name string) (*Pipeline, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	p, exists := e.pipelines[name]
	if !exists {
		return nil, ErrPipelineNotFound
	}
	return p, nil
}

// ListPipelines returns all registered pipelines.
func (e *Engine) ListPipelines() []*Pipeline {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Pipeline, 0, len(e.pipelines))
	for _, p := range e.pipelines {
		result = append(result, p)
	}
	return result
}

// ExecutePipeline runs a pipeline synchronously and returns the run result.
func (e *Engine) ExecutePipeline(ctx context.Context, name string, trigger TriggerType) (*Run, error) {
	e.mu.RLock()
	pipeline, exists := e.pipelines[name]
	e.mu.RUnlock()

	if !exists {
		return nil, ErrPipelineNotFound
	}

	e.mu.Lock()
	e.runCount++
	runID := fmt.Sprintf("%s-run-%d", name, e.runCount)
	e.mu.Unlock()

	run := &Run{
		ID:           runID,
		PipelineName: name,
		Status:       StatusRunning,
		Steps:        make([]StepResult, 0, len(pipeline.Steps)),
		StartedAt:    time.Now(),
		Trigger:      trigger,
	}

	// Topologically sort steps
	order, err := e.topologicalSort(pipeline)
	if err != nil {
		run.Status = StatusFailed
		run.EndedAt = time.Now()
		e.storeRun(run)
		return run, err
	}

	// Execute steps in order, respecting dependencies
	stepOutputs := make(map[string]map[string]interface{})
	for _, stepName := range order {
		step := e.findStep(pipeline, stepName)
		if step == nil {
			continue
		}

		result := e.executeStep(ctx, step, stepOutputs)
		run.Steps = append(run.Steps, result)

		if result.Status == StatusCompleted && result.Output != nil {
			stepOutputs[stepName] = result.Output
		}

		if result.Status == StatusFailed {
			run.Status = StatusFailed
			run.EndedAt = time.Now()
			e.storeRun(run)
			return run, fmt.Errorf("step %s failed: %s", stepName, result.Error)
		}
	}

	run.Status = StatusCompleted
	run.EndedAt = time.Now()
	e.storeRun(run)
	return run, nil
}

// GetRuns returns recent runs for a pipeline.
func (e *Engine) GetRuns(pipelineName string) []*Run {
	e.mu.RLock()
	defer e.mu.RUnlock()

	runs := e.runs[pipelineName]
	result := make([]*Run, len(runs))
	copy(result, runs)
	return result
}

// GetRun retrieves a specific run by ID.
func (e *Engine) GetRun(pipelineName, runID string) (*Run, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, r := range e.runs[pipelineName] {
		if r.ID == runID {
			return r, nil
		}
	}
	return nil, ErrRunNotFound
}

// Backfill executes a pipeline for a historical time range.
func (e *Engine) Backfill(ctx context.Context, pipelineName string, start, end time.Time, interval time.Duration) ([]*Run, error) {
	if interval <= 0 {
		interval = time.Hour
	}

	var runs []*Run
	for t := start; t.Before(end); t = t.Add(interval) {
		run, err := e.ExecutePipeline(ctx, pipelineName, TriggerManual)
		if err != nil {
			return runs, fmt.Errorf("backfill at %s: %w", t.Format(time.RFC3339), err)
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func (e *Engine) storeRun(run *Run) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.runs[run.PipelineName] = append(e.runs[run.PipelineName], run)
	if len(e.runs[run.PipelineName]) > e.config.MaxRunHistory {
		e.runs[run.PipelineName] = e.runs[run.PipelineName][1:]
	}
}

func (e *Engine) executeStep(ctx context.Context, step *Step, outputs map[string]map[string]interface{}) StepResult {
	result := StepResult{
		StepName:  step.Name,
		Status:    StatusRunning,
		StartedAt: time.Now(),
	}

	timeout := step.Timeout
	if timeout == 0 {
		timeout = e.config.DefaultStepTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Collect inputs from dependencies
	input := make(map[string]interface{})
	for _, dep := range step.DependsOn {
		if out, ok := outputs[dep]; ok {
			for k, v := range out {
				input[dep+"."+k] = v
			}
		}
	}

	retries := step.RetryCount
	if retries == 0 {
		retries = 1
	}

	var lastErr error
	for attempt := 0; attempt < retries; attempt++ {
		if step.Transform != nil {
			out, err := step.Transform(ctx, input)
			if err != nil {
				lastErr = err
				continue
			}
			result.Output = out
			result.Status = StatusCompleted
			result.EndedAt = time.Now()
			return result
		}

		// If no transform, treat as a passthrough (expression evaluated externally)
		result.Output = map[string]interface{}{
			"expression": step.Expression,
			"feature":    step.OutputFeature,
			"group":      step.OutputGroup,
		}
		result.Status = StatusCompleted
		result.EndedAt = time.Now()
		return result
	}

	result.Status = StatusFailed
	result.EndedAt = time.Now()
	if lastErr != nil {
		result.Error = lastErr.Error()
	}
	return result
}

func (e *Engine) findStep(p *Pipeline, name string) *Step {
	for i := range p.Steps {
		if p.Steps[i].Name == name {
			return &p.Steps[i]
		}
	}
	return nil
}

// validateDAG checks for cycles in the step dependency graph.
func (e *Engine) validateDAG(p *Pipeline) error {
	stepNames := make(map[string]bool)
	for _, s := range p.Steps {
		stepNames[s.Name] = true
	}
	for _, s := range p.Steps {
		for _, dep := range s.DependsOn {
			if !stepNames[dep] {
				return fmt.Errorf("%w: step %q depends on unknown step %q", ErrInvalidPipeline, s.Name, dep)
			}
		}
	}

	_, err := e.topologicalSort(p)
	return err
}

// topologicalSort returns steps in dependency order using Kahn's algorithm.
func (e *Engine) topologicalSort(p *Pipeline) ([]string, error) {
	inDegree := make(map[string]int)
	graph := make(map[string][]string)

	for _, s := range p.Steps {
		if _, exists := inDegree[s.Name]; !exists {
			inDegree[s.Name] = 0
		}
		for _, dep := range s.DependsOn {
			graph[dep] = append(graph[dep], s.Name)
			inDegree[s.Name]++
		}
	}

	var queue []string
	for _, s := range p.Steps {
		if inDegree[s.Name] == 0 {
			queue = append(queue, s.Name)
		}
	}
	sort.Strings(queue)

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		neighbors := graph[node]
		sort.Strings(neighbors)
		for _, next := range neighbors {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(order) != len(p.Steps) {
		return nil, ErrCyclicDependency
	}
	return order, nil
}
