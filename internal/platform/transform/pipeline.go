package transform

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

// Pipeline errors.
var (
	ErrTransformNotFound = errors.New("transform not found")
	ErrInvalidExpression = errors.New("invalid expression")
	ErrTypeMismatch      = errors.New("type mismatch")
	ErrDependencyCycle   = errors.New("dependency cycle detected")
	ErrMissingDependency = errors.New("missing dependency")
)

// Type represents the type of transformation.
type Type string

const (
	// TypeArithmetic performs arithmetic transforms.
	TypeArithmetic Type = "arithmetic"
	// TypeAggregation performs aggregation transforms.
	TypeAggregation Type = "aggregation"
	// TypeWindow performs window transforms.
	TypeWindow Type = "window"
	// TypeConditional performs conditional transforms.
	TypeConditional Type = "conditional"
	// TypeString performs string transforms.
	TypeString Type = "string"
	// TypeTimestamp performs timestamp transforms.
	TypeTimestamp Type = "timestamp"
	// TypeLookup performs lookup transforms.
	TypeLookup Type = "lookup"
	// TypeCustom performs custom transforms.
	TypeCustom Type = "custom"
)

// Transform defines a feature transformation.
type Transform struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	Type         Type                   `json:"type"`
	Expression   string                 `json:"expression"`
	Inputs       []string               `json:"inputs"`
	Output       string                 `json:"output"`
	OutputType   domain.DataType        `json:"output_type"`
	Config       map[string]interface{} `json:"config,omitempty"`
	Enabled      bool                   `json:"enabled"`
	Mode         ExecutionMode          `json:"mode"`
	Dependencies []string               `json:"dependencies,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// ExecutionMode determines when the transform runs.
type ExecutionMode string

const (
	// ModeOnRead computes when a feature is read.
	ModeOnRead ExecutionMode = "on_read"
	// ModeOnWrite computes when input features are written.
	ModeOnWrite ExecutionMode = "on_write"
	// ModeSchedule computes on a schedule.
	ModeSchedule ExecutionMode = "schedule"
	// ModeBatch computes in batch jobs.
	ModeBatch ExecutionMode = "batch"
)

// Pipeline manages feature transformations.
type Pipeline struct {
	store        *storage.Store
	transforms   map[string]*Transform
	dependencies map[string][]string // output -> inputs
	executors    map[Type]Executor
	mu           sync.RWMutex
}

// Executor executes a specific type of transformation.
type Executor interface {
	Execute(ctx context.Context, transform *Transform, inputs map[string]interface{}) (interface{}, error)
	Validate(transform *Transform) error
}

// NewPipeline creates a new transformation pipeline.
func NewPipeline(store *storage.Store) *Pipeline {
	p := &Pipeline{
		store:        store,
		transforms:   make(map[string]*Transform),
		dependencies: make(map[string][]string),
		executors:    make(map[Type]Executor),
	}

	// Register built-in executors
	p.executors[TypeArithmetic] = &ArithmeticExecutor{}
	p.executors[TypeAggregation] = &AggregationExecutor{store: store}
	p.executors[TypeWindow] = &WindowExecutor{store: store}
	p.executors[TypeConditional] = &ConditionalExecutor{}
	p.executors[TypeString] = &StringExecutor{}
	p.executors[TypeTimestamp] = &TimestampExecutor{}
	p.executors[TypeLookup] = &LookupExecutor{store: store}

	return p
}

// RegisterTransform adds a new transformation.
func (p *Pipeline) RegisterTransform(t *Transform) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Validate transform
	executor, ok := p.executors[t.Type]
	if !ok {
		return fmt.Errorf("unknown transform type: %s", t.Type)
	}

	if err := executor.Validate(t); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check for dependency cycles
	if err := p.checkCycles(t); err != nil {
		return err
	}

	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	t.Enabled = true

	p.transforms[t.Name] = t
	p.dependencies[t.Output] = t.Inputs

	return nil
}

// UnregisterTransform removes a transformation.
func (p *Pipeline) UnregisterTransform(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	t, ok := p.transforms[name]
	if !ok {
		return ErrTransformNotFound
	}

	delete(p.transforms, name)
	delete(p.dependencies, t.Output)

	return nil
}

// GetTransform retrieves a transformation by name.
func (p *Pipeline) GetTransform(name string) (*Transform, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	t, ok := p.transforms[name]
	if !ok {
		return nil, ErrTransformNotFound
	}

	return t, nil
}

// ListTransforms returns all registered transforms.
func (p *Pipeline) ListTransforms() []*Transform {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]*Transform, 0, len(p.transforms))
	for _, t := range p.transforms {
		result = append(result, t)
	}

	return result
}

// Execute runs a transformation for an entity.
func (p *Pipeline) Execute(ctx context.Context, transformName string, entityID string) (interface{}, error) {
	p.mu.RLock()
	t, ok := p.transforms[transformName]
	p.mu.RUnlock()

	if !ok {
		return nil, ErrTransformNotFound
	}

	if !t.Enabled {
		return nil, fmt.Errorf("transform %s is disabled", transformName)
	}

	// Get input features
	inputs, err := p.getInputs(ctx, entityID, t.Inputs)
	if err != nil {
		return nil, fmt.Errorf("getting inputs: %w", err)
	}

	// Execute transform
	executor := p.executors[t.Type]
	result, err := executor.Execute(ctx, t, inputs)
	if err != nil {
		return nil, fmt.Errorf("executing transform: %w", err)
	}

	return result, nil
}

// ExecuteAndStore runs a transformation and stores the result.
func (p *Pipeline) ExecuteAndStore(ctx context.Context, transformName string, entityID string) error {
	result, err := p.Execute(ctx, transformName, entityID)
	if err != nil {
		return err
	}

	p.mu.RLock()
	t := p.transforms[transformName]
	p.mu.RUnlock()

	// Store the computed feature
	return p.store.Put(entityID, map[string]*domain.FeatureValue{
		t.Output: {
			Value:     result,
			Timestamp: time.Now().UnixNano(),
			Version:   1,
		},
	})
}

// ExecuteChain executes a chain of dependent transforms.
func (p *Pipeline) ExecuteChain(ctx context.Context, outputFeature string, entityID string) (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Find the transform that produces this output
	var targetTransform *Transform
	for _, t := range p.transforms {
		if t.Output == outputFeature {
			targetTransform = t
			break
		}
	}

	if targetTransform == nil {
		// Not a computed feature, try to get from store
		values, err := p.store.Get(entityID, []string{outputFeature})
		if err != nil {
			return nil, err
		}
		if v, ok := values[outputFeature]; ok {
			return v.Value, nil
		}
		return nil, ErrMissingDependency
	}

	// Recursively compute dependencies
	inputs := make(map[string]interface{})
	for _, input := range targetTransform.Inputs {
		value, err := p.executeChainUnlocked(ctx, input, entityID)
		if err != nil {
			return nil, fmt.Errorf("computing dependency %s: %w", input, err)
		}
		inputs[input] = value
	}

	// Execute the transform
	executor := p.executors[targetTransform.Type]
	return executor.Execute(ctx, targetTransform, inputs)
}

func (p *Pipeline) executeChainUnlocked(ctx context.Context, feature string, entityID string) (interface{}, error) {
	// Check if this is a computed feature
	var transform *Transform
	for _, t := range p.transforms {
		if t.Output == feature {
			transform = t
			break
		}
	}

	if transform == nil {
		// Raw feature, get from store
		values, err := p.store.Get(entityID, []string{feature})
		if err != nil {
			return nil, err
		}
		if v, ok := values[feature]; ok {
			return v.Value, nil
		}
		return nil, ErrMissingDependency
	}

	// Recursively compute
	inputs := make(map[string]interface{})
	for _, input := range transform.Inputs {
		value, err := p.executeChainUnlocked(ctx, input, entityID)
		if err != nil {
			return nil, err
		}
		inputs[input] = value
	}

	executor := p.executors[transform.Type]
	return executor.Execute(ctx, transform, inputs)
}

func (p *Pipeline) getInputs(ctx context.Context, entityID string, inputNames []string) (map[string]interface{}, error) {
	values, err := p.store.Get(entityID, inputNames)
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{}, len(values))
	for name, fv := range values {
		result[name] = fv.Value
	}

	return result, nil
}

func (p *Pipeline) checkCycles(t *Transform) error {
	visited := make(map[string]bool)
	return p.dfs(t.Output, t.Inputs, visited)
}

func (p *Pipeline) dfs(output string, inputs []string, visited map[string]bool) error {
	if visited[output] {
		return ErrDependencyCycle
	}
	visited[output] = true

	for _, input := range inputs {
		// Check if this input creates a cycle back to any visited node
		if visited[input] {
			return ErrDependencyCycle
		}
		if deps, ok := p.dependencies[input]; ok {
			if err := p.dfs(input, deps, visited); err != nil {
				return err
			}
		}
	}

	return nil
}
