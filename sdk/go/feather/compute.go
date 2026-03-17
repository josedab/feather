package feather

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"
)

// ComputeClient handles feature computation operations.
type ComputeClient struct {
	client *Client
}

// NewComputeClient creates a new compute client.
func NewComputeClient(client *Client) *ComputeClient {
	return &ComputeClient{client: client}
}

// FeaturePipeline defines a feature computation pipeline.
type FeaturePipeline struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Steps       []ComputeStep `json:"steps"`
	Schedule    string        `json:"schedule,omitempty"`
	Owner       string        `json:"owner,omitempty"`
	Tags        []string      `json:"tags,omitempty"`
}

// ComputeStep represents a step in the computation pipeline.
type ComputeStep struct {
	Name       string                 `json:"name"`
	Type       ComputeStepType        `json:"type"`
	Inputs     []string               `json:"inputs"`
	Output     string                 `json:"output"`
	Config     map[string]interface{} `json:"config,omitempty"`
	Expression string                 `json:"expression,omitempty"`
	DependsOn  []string               `json:"depends_on,omitempty"`
}

// ComputeStepType defines the type of compute step.
type ComputeStepType string

const (
	// StepTypeAggregation performs aggregations.
	StepTypeAggregation ComputeStepType = "aggregation"
	// StepTypeTransform applies a transformation.
	StepTypeTransform ComputeStepType = "transform"
	// StepTypeJoin joins inputs.
	StepTypeJoin ComputeStepType = "join"
	// StepTypeFilter filters inputs.
	StepTypeFilter ComputeStepType = "filter"
	// StepTypeWindow applies a window function.
	StepTypeWindow ComputeStepType = "window"
	// StepTypeExpression evaluates an expression.
	StepTypeExpression ComputeStepType = "expression"
	// StepTypeLookup performs a lookup.
	StepTypeLookup ComputeStepType = "lookup"
	// StepTypeEmbedding generates embeddings.
	StepTypeEmbedding ComputeStepType = "embedding"
	// StepTypeNormalize normalizes values.
	StepTypeNormalize ComputeStepType = "normalize"
	// StepTypeBucketize bucketizes values.
	StepTypeBucketize ComputeStepType = "bucketize"
	// StepTypeOneHotEncode one-hot encodes values.
	StepTypeOneHotEncode ComputeStepType = "one_hot_encode"
	// StepTypeCustom runs a custom step.
	StepTypeCustom ComputeStepType = "custom"
)

// CreatePipeline creates a feature computation pipeline.
func (c *ComputeClient) CreatePipeline(ctx context.Context, pipeline *FeaturePipeline) error {
	return c.client.request(ctx, "POST", "/v1/compute/pipelines", pipeline, pipeline)
}

// GetPipeline retrieves a pipeline by name.
func (c *ComputeClient) GetPipeline(ctx context.Context, name string) (*FeaturePipeline, error) {
	var pipeline FeaturePipeline
	err := c.client.request(ctx, "GET", "/v1/compute/pipelines/"+url.PathEscape(name), nil, &pipeline)
	if err != nil {
		return nil, err
	}
	return &pipeline, nil
}

// ListPipelines lists all pipelines.
func (c *ComputeClient) ListPipelines(ctx context.Context) ([]*FeaturePipeline, error) {
	var resp struct {
		Pipelines []*FeaturePipeline `json:"pipelines"`
	}
	err := c.client.request(ctx, "GET", "/v1/compute/pipelines", nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Pipelines, nil
}

// DeletePipeline deletes a pipeline.
func (c *ComputeClient) DeletePipeline(ctx context.Context, name string) error {
	return c.client.request(ctx, "DELETE", "/v1/compute/pipelines/"+url.PathEscape(name), nil, nil)
}

// ExecutionRequest represents a pipeline execution request.
type ExecutionRequest struct {
	PipelineName string                 `json:"pipeline_name"`
	EntityID     string                 `json:"entity_id,omitempty"`
	EntityIDs    []string               `json:"entity_ids,omitempty"`
	Inputs       map[string]interface{} `json:"inputs,omitempty"`
	Async        bool                   `json:"async,omitempty"`
}

// ExecutionResult represents the result of pipeline execution.
type ExecutionResult struct {
	ExecutionID string                            `json:"execution_id"`
	Status      string                            `json:"status"`
	Results     map[string]map[string]interface{} `json:"results,omitempty"`
	Errors      []string                          `json:"errors,omitempty"`
	StartedAt   time.Time                         `json:"started_at"`
	CompletedAt *time.Time                        `json:"completed_at,omitempty"`
	Duration    time.Duration                     `json:"duration,omitempty"`
}

// Execute executes a pipeline.
func (c *ComputeClient) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	var result ExecutionResult
	err := c.client.request(ctx, "POST", "/v1/compute/execute", req, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ExecuteAndWait executes a pipeline and waits for completion.
func (c *ComputeClient) ExecuteAndWait(ctx context.Context, req *ExecutionRequest, pollInterval time.Duration) (*ExecutionResult, error) {
	req.Async = true
	result, err := c.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			status, err := c.GetExecution(ctx, result.ExecutionID)
			if err != nil {
				return nil, err
			}
			if status.Status == "completed" || status.Status == "failed" {
				return status, nil
			}
		}
	}
}

// GetExecution gets the status of an execution.
func (c *ComputeClient) GetExecution(ctx context.Context, executionID string) (*ExecutionResult, error) {
	var result ExecutionResult
	err := c.client.request(ctx, "GET", "/v1/compute/executions/"+url.PathEscape(executionID), nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// PipelineBuilder provides a fluent API for building pipelines.
type PipelineBuilder struct {
	pipeline FeaturePipeline
}

// NewPipelineBuilder creates a new pipeline builder.
func NewPipelineBuilder(name string) *PipelineBuilder {
	return &PipelineBuilder{
		pipeline: FeaturePipeline{
			Name:  name,
			Steps: make([]ComputeStep, 0),
		},
	}
}

// Description sets the pipeline description.
func (b *PipelineBuilder) Description(desc string) *PipelineBuilder {
	b.pipeline.Description = desc
	return b
}

// Owner sets the pipeline owner.
func (b *PipelineBuilder) Owner(owner string) *PipelineBuilder {
	b.pipeline.Owner = owner
	return b
}

// Tags sets the pipeline tags.
func (b *PipelineBuilder) Tags(tags ...string) *PipelineBuilder {
	b.pipeline.Tags = tags
	return b
}

// Schedule sets the pipeline schedule.
func (b *PipelineBuilder) Schedule(schedule string) *PipelineBuilder {
	b.pipeline.Schedule = schedule
	return b
}

// AddStep adds a compute step to the pipeline.
func (b *PipelineBuilder) AddStep(step ComputeStep) *PipelineBuilder {
	b.pipeline.Steps = append(b.pipeline.Steps, step)
	return b
}

// Aggregate adds an aggregation step.
func (b *PipelineBuilder) Aggregate(name, input, output string, aggType string, config map[string]interface{}) *PipelineBuilder {
	if config == nil {
		config = make(map[string]interface{})
	}
	config["aggregation"] = aggType
	return b.AddStep(ComputeStep{
		Name:   name,
		Type:   StepTypeAggregation,
		Inputs: []string{input},
		Output: output,
		Config: config,
	})
}

// Transform adds a transform step.
func (b *PipelineBuilder) Transform(name, input, output, expression string) *PipelineBuilder {
	return b.AddStep(ComputeStep{
		Name:       name,
		Type:       StepTypeTransform,
		Inputs:     []string{input},
		Output:     output,
		Expression: expression,
	})
}

// Join adds a join step.
func (b *PipelineBuilder) Join(name string, inputs []string, output string, joinType string, joinKey string) *PipelineBuilder {
	return b.AddStep(ComputeStep{
		Name:   name,
		Type:   StepTypeJoin,
		Inputs: inputs,
		Output: output,
		Config: map[string]interface{}{
			"join_type": joinType,
			"join_key":  joinKey,
		},
	})
}

// Filter adds a filter step.
func (b *PipelineBuilder) Filter(name, input, output, condition string) *PipelineBuilder {
	return b.AddStep(ComputeStep{
		Name:   name,
		Type:   StepTypeFilter,
		Inputs: []string{input},
		Output: output,
		Config: map[string]interface{}{
			"condition": condition,
		},
	})
}

// Window adds a window step.
func (b *PipelineBuilder) Window(name, input, output string, windowSize time.Duration, windowSlide time.Duration) *PipelineBuilder {
	return b.AddStep(ComputeStep{
		Name:   name,
		Type:   StepTypeWindow,
		Inputs: []string{input},
		Output: output,
		Config: map[string]interface{}{
			"window_size":  windowSize.String(),
			"window_slide": windowSlide.String(),
		},
	})
}

// Lookup adds a lookup step to join with another feature.
func (b *PipelineBuilder) Lookup(name, input, lookupFeature, output string, lookupKey string) *PipelineBuilder {
	return b.AddStep(ComputeStep{
		Name:   name,
		Type:   StepTypeLookup,
		Inputs: []string{input, lookupFeature},
		Output: output,
		Config: map[string]interface{}{
			"lookup_key": lookupKey,
		},
	})
}

// Normalize adds a normalization step.
func (b *PipelineBuilder) Normalize(name, input, output string, method string) *PipelineBuilder {
	return b.AddStep(ComputeStep{
		Name:   name,
		Type:   StepTypeNormalize,
		Inputs: []string{input},
		Output: output,
		Config: map[string]interface{}{
			"method": method, // "min_max", "z_score", "l2"
		},
	})
}

// Bucketize adds a bucketization step.
func (b *PipelineBuilder) Bucketize(name, input, output string, boundaries []float64) *PipelineBuilder {
	return b.AddStep(ComputeStep{
		Name:   name,
		Type:   StepTypeBucketize,
		Inputs: []string{input},
		Output: output,
		Config: map[string]interface{}{
			"boundaries": boundaries,
		},
	})
}

// OneHotEncode adds a one-hot encoding step.
func (b *PipelineBuilder) OneHotEncode(name, input, output string, categories []string) *PipelineBuilder {
	return b.AddStep(ComputeStep{
		Name:   name,
		Type:   StepTypeOneHotEncode,
		Inputs: []string{input},
		Output: output,
		Config: map[string]interface{}{
			"categories": categories,
		},
	})
}

// Embedding adds an embedding computation step.
func (b *PipelineBuilder) Embedding(name, input, output string, model string, dimensions int) *PipelineBuilder {
	return b.AddStep(ComputeStep{
		Name:   name,
		Type:   StepTypeEmbedding,
		Inputs: []string{input},
		Output: output,
		Config: map[string]interface{}{
			"model":      model,
			"dimensions": dimensions,
		},
	})
}

// Expression adds an expression evaluation step.
func (b *PipelineBuilder) Expression(name string, inputs []string, output, expr string) *PipelineBuilder {
	return b.AddStep(ComputeStep{
		Name:       name,
		Type:       StepTypeExpression,
		Inputs:     inputs,
		Output:     output,
		Expression: expr,
	})
}

// Custom adds a custom computation step.
func (b *PipelineBuilder) Custom(name string, inputs []string, output string, config map[string]interface{}) *PipelineBuilder {
	return b.AddStep(ComputeStep{
		Name:   name,
		Type:   StepTypeCustom,
		Inputs: inputs,
		Output: output,
		Config: config,
	})
}

// Build returns the constructed pipeline.
func (b *PipelineBuilder) Build() *FeaturePipeline {
	return &b.pipeline
}

// LocalCompute provides client-side feature computation.
type LocalCompute struct {
	mu        sync.RWMutex
	functions map[string]ComputeFunc
}

// ComputeFunc is a function that computes a feature.
type ComputeFunc func(ctx context.Context, inputs map[string]interface{}) (interface{}, error)

// NewLocalCompute creates a new local compute engine.
func NewLocalCompute() *LocalCompute {
	lc := &LocalCompute{
		functions: make(map[string]ComputeFunc),
	}
	// Register built-in functions
	lc.registerBuiltins()
	return lc
}

// Register registers a custom compute function.
func (lc *LocalCompute) Register(name string, fn ComputeFunc) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.functions[name] = fn
}

// Compute executes a compute function.
func (lc *LocalCompute) Compute(ctx context.Context, name string, inputs map[string]interface{}) (interface{}, error) {
	lc.mu.RLock()
	fn, ok := lc.functions[name]
	lc.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("compute function not found: %s", name)
	}

	return fn(ctx, inputs)
}

// ExecutePipeline executes a pipeline locally.
func (lc *LocalCompute) ExecutePipeline(ctx context.Context, pipeline *FeaturePipeline, inputs map[string]interface{}) (map[string]interface{}, error) {
	results := make(map[string]interface{})

	// Copy inputs to results
	for k, v := range inputs {
		results[k] = v
	}

	// Execute steps in order (respecting dependencies)
	executed := make(map[string]bool)
	for {
		progress := false
		for _, step := range pipeline.Steps {
			if executed[step.Name] {
				continue
			}

			// Check dependencies
			depsOK := true
			for _, dep := range step.DependsOn {
				if !executed[dep] {
					depsOK = false
					break
				}
			}
			if !depsOK {
				continue
			}

			// Check inputs available
			inputsOK := true
			stepInputs := make(map[string]interface{})
			for _, input := range step.Inputs {
				if val, ok := results[input]; ok {
					stepInputs[input] = val
				} else {
					inputsOK = false
					break
				}
			}
			if !inputsOK {
				continue
			}

			// Add config to inputs
			for k, v := range step.Config {
				stepInputs["_config_"+k] = v
			}
			stepInputs["_expression"] = step.Expression

			// Execute step
			result, err := lc.executeStep(ctx, step, stepInputs)
			if err != nil {
				return nil, fmt.Errorf("step %s failed: %w", step.Name, err)
			}

			results[step.Output] = result
			executed[step.Name] = true
			progress = true
		}

		if !progress {
			break
		}
	}

	return results, nil
}

func (lc *LocalCompute) executeStep(ctx context.Context, step ComputeStep, inputs map[string]interface{}) (interface{}, error) {
	funcName := string(step.Type)
	return lc.Compute(ctx, funcName, inputs)
}

func (lc *LocalCompute) registerBuiltins() {
	// Aggregation
	lc.Register("aggregation", func(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
		aggType, _ := inputs["_config_aggregation"].(string)
		var values []float64

		for k, v := range inputs {
			if k[0] == '_' {
				continue
			}
			switch val := v.(type) {
			case float64:
				values = append(values, val)
			case int:
				values = append(values, float64(val))
			case int64:
				values = append(values, float64(val))
			case []float64:
				values = append(values, val...)
			}
		}

		if len(values) == 0 {
			return nil, nil
		}

		switch aggType {
		case "sum":
			var sum float64
			for _, v := range values {
				sum += v
			}
			return sum, nil
		case "avg", "mean":
			var sum float64
			for _, v := range values {
				sum += v
			}
			return sum / float64(len(values)), nil
		case "count":
			return len(values), nil
		case "min":
			minValue := values[0]
			for _, v := range values[1:] {
				if v < minValue {
					minValue = v
				}
			}
			return minValue, nil
		case "max":
			maxValue := values[0]
			for _, v := range values[1:] {
				if v > maxValue {
					maxValue = v
				}
			}
			return maxValue, nil
		default:
			return nil, fmt.Errorf("unknown aggregation type: %s", aggType)
		}
	})

	// Transform (expression evaluation placeholder)
	lc.Register("transform", func(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
		// Simple expression evaluation - in production use expr library
		expr, _ := inputs["_expression"].(string)
		if expr == "" {
			// Return first input as-is
			for k, v := range inputs {
				if k[0] != '_' {
					return v, nil
				}
			}
		}
		return nil, nil
	})

	// Normalize
	lc.Register("normalize", func(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
		method, _ := inputs["_config_method"].(string)
		var val float64

		for k, v := range inputs {
			if k[0] == '_' {
				continue
			}
			switch v := v.(type) {
			case float64:
				val = v
			case int:
				val = float64(v)
			}
		}

		switch method {
		case "min_max":
			// Would need stats - placeholder
			return val, nil
		case "z_score":
			return val, nil
		case "l2":
			return val, nil
		default:
			return val, nil
		}
	})

	// Bucketize
	lc.Register("bucketize", func(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
		boundariesRaw, _ := inputs["_config_boundaries"].([]interface{})
		var boundaries []float64
		for _, b := range boundariesRaw {
			if f, ok := b.(float64); ok {
				boundaries = append(boundaries, f)
			}
		}

		var val float64
		for k, v := range inputs {
			if k[0] == '_' {
				continue
			}
			switch v := v.(type) {
			case float64:
				val = v
			case int:
				val = float64(v)
			}
		}

		bucket := 0
		for i, b := range boundaries {
			if val < b {
				bucket = i
				break
			}
			bucket = i + 1
		}

		return bucket, nil
	})

	// One-hot encode
	lc.Register("one_hot_encode", func(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
		categoriesRaw, _ := inputs["_config_categories"].([]interface{})
		var categories []string
		for _, c := range categoriesRaw {
			if s, ok := c.(string); ok {
				categories = append(categories, s)
			}
		}

		var val string
		for k, v := range inputs {
			if k[0] == '_' {
				continue
			}
			if s, ok := v.(string); ok {
				val = s
			}
		}

		result := make([]int, len(categories))
		for i, cat := range categories {
			if cat == val {
				result[i] = 1
			}
		}

		return result, nil
	})

	// Other step types
	lc.Register("filter", func(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
		// Filter would need condition evaluation
		for k, v := range inputs {
			if k[0] != '_' {
				return v, nil
			}
		}
		return nil, nil
	})

	lc.Register("expression", func(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
		// Expression evaluation placeholder
		return nil, nil
	})

	lc.Register("window", func(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
		// Window computation placeholder
		return nil, nil
	})

	lc.Register("join", func(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
		// Join computation placeholder
		return nil, nil
	})

	lc.Register("lookup", func(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
		// Lookup computation placeholder
		return nil, nil
	})

	lc.Register("embedding", func(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
		// Embedding computation placeholder
		return nil, nil
	})

	lc.Register("custom", func(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
		// Custom computation placeholder
		return nil, nil
	})
}
