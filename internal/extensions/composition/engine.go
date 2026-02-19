package composition

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/core/storage"
	"github.com/feather-store/feather/internal/extensions/composition/expr"
	"github.com/feather-store/feather/internal/platform/transform"
)

// Engine provides the main interface for feature composition.
type Engine struct {
	store    *storage.Store
	pipeline *transform.Pipeline
	executor *Executor
	dags     map[string]*DAG
	mu       sync.RWMutex
}

// EngineConfig configures the composition engine.
type EngineConfig struct {
	Store          *storage.Store
	Pipeline       *transform.Pipeline
	ExecutorConfig ExecutorConfig
}

// NewEngine creates a new composition engine.
func NewEngine(config EngineConfig) *Engine {
	executor := NewExecutor(config.ExecutorConfig)

	e := &Engine{
		store:    config.Store,
		pipeline: config.Pipeline,
		executor: executor,
		dags:     make(map[string]*DAG),
	}

	// Register built-in compute functions
	e.registerBuiltinFunctions()

	return e
}

func (e *Engine) registerBuiltinFunctions() {
	// Source function - retrieves features from store
	e.executor.SetSourceFunc(func(ctx context.Context, node *Node, entityID string) (interface{}, error) {
		if e.store == nil {
			return nil, fmt.Errorf("store not configured")
		}

		featureName := node.Expression
		if featureName == "" {
			featureName = node.Name
		}

		values, err := e.store.Get(ctx, entityID, []string{featureName})
		if err != nil {
			return nil, err
		}

		if fv, ok := values[featureName]; ok {
			return fv.Value, nil
		}
		return nil, fmt.Errorf("feature %s not found for entity %s", featureName, entityID)
	})

	// Transform compute function - uses the transform pipeline
	e.executor.RegisterComputeFunc(NodeTypeTransform, func(ctx context.Context, node *Node, inputs map[string]interface{}) (interface{}, error) {
		if e.pipeline == nil {
			return nil, fmt.Errorf("pipeline not configured")
		}

		// Check if there's a transform registered
		transformName := node.Expression
		if transformName == "" {
			transformName = node.Name
		}

		t, err := e.pipeline.GetTransform(transformName)
		if err != nil {
			// Not a registered transform, use the expression evaluator
			return e.evaluateExpression(node.Expression, inputs)
		}

		// Use the pipeline executor
		executor := getExecutorForType(t.Type)
		if executor == nil {
			return nil, fmt.Errorf("no executor for transform type %s", t.Type)
		}

		return executor.Execute(ctx, t, inputs)
	})

	// Aggregate compute function
	e.executor.RegisterComputeFunc(NodeTypeAggregate, func(ctx context.Context, node *Node, inputs map[string]interface{}) (interface{}, error) {
		aggType, _ := node.Config["aggregate"].(string)

		var values []float64
		for _, v := range inputs {
			if num, ok := toFloat64(v); ok {
				values = append(values, num)
			}
		}

		if len(values) == 0 {
			return nil, fmt.Errorf("no numeric values for aggregation")
		}

		switch aggType {
		case "sum":
			sum := 0.0
			for _, v := range values {
				sum += v
			}
			return sum, nil
		case "avg":
			sum := 0.0
			for _, v := range values {
				sum += v
			}
			return sum / float64(len(values)), nil
		case "min":
			minVal := values[0]
			for _, v := range values[1:] {
				if v < minVal {
					minVal = v
				}
			}
			return minVal, nil
		case "max":
			maxVal := values[0]
			for _, v := range values[1:] {
				if v > maxVal {
					maxVal = v
				}
			}
			return maxVal, nil
		case "count":
			return float64(len(values)), nil
		default:
			return nil, fmt.Errorf("unknown aggregate type: %s", aggType)
		}
	})

	// Join compute function
	e.executor.RegisterComputeFunc(NodeTypeJoin, func(ctx context.Context, node *Node, inputs map[string]interface{}) (interface{}, error) {
		joinType, _ := node.Config["join_type"].(string)

		switch joinType {
		case "concat":
			// Concatenate all input values into a slice
			var result []interface{}
			for _, v := range inputs {
				result = append(result, v)
			}
			return result, nil
		case "merge":
			// Merge maps
			result := make(map[string]interface{})
			for key, v := range inputs {
				if m, ok := v.(map[string]interface{}); ok {
					for k, val := range m {
						result[k] = val
					}
				} else {
					result[key] = v
				}
			}
			return result, nil
		default:
			// Default: return as map
			return inputs, nil
		}
	})

	// Filter compute function
	e.executor.RegisterComputeFunc(NodeTypeFilter, func(ctx context.Context, node *Node, inputs map[string]interface{}) (interface{}, error) {
		condition, _ := node.Config["condition"].(string)
		threshold, _ := node.Config["threshold"].(float64)

		// Simple filter implementation
		for _, v := range inputs {
			num, ok := toFloat64(v)
			if !ok {
				continue
			}

			switch condition {
			case "gt":
				if num > threshold {
					return v, nil
				}
			case "gte":
				if num >= threshold {
					return v, nil
				}
			case "lt":
				if num < threshold {
					return v, nil
				}
			case "lte":
				if num <= threshold {
					return v, nil
				}
			case "eq":
				if num == threshold {
					return v, nil
				}
			}
		}

		return nil, nil
	})

	// Custom compute function
	e.executor.RegisterComputeFunc(NodeTypeCustom, func(ctx context.Context, node *Node, inputs map[string]interface{}) (interface{}, error) {
		// Custom functions can be registered separately
		return e.evaluateExpression(node.Expression, inputs)
	})
}

func (e *Engine) evaluateExpression(expression string, inputs map[string]interface{}) (interface{}, error) {
	// Handle simple keyword expressions for backward compatibility
	switch expression {
	case "sum":
		sum := 0.0
		for _, v := range inputs {
			if num, ok := toFloat64(v); ok {
				sum += num
			}
		}
		return sum, nil
	case "product":
		product := 1.0
		for _, v := range inputs {
			if num, ok := toFloat64(v); ok {
				product *= num
			}
		}
		return product, nil
	case "first":
		for _, v := range inputs {
			return v, nil
		}
		return nil, nil
	}

	// Use the full expression parser for complex expressions
	return expr.Evaluate(expression, inputs)
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint64:
		return float64(val), true
	default:
		return 0, false
	}
}

func getExecutorForType(t transform.Type) transform.Executor {
	// Return appropriate executor based on type
	// This would be populated from the pipeline in a real implementation
	return nil
}

// RegisterDAG adds a new DAG to the engine.
func (e *Engine) RegisterDAG(dag *DAG) error {
	if err := dag.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := dag.ComputeTopology(); err != nil {
		return fmt.Errorf("topology computation failed: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.dags[dag.ID] = dag
	return nil
}

// UnregisterDAG removes a DAG from the engine.
func (e *Engine) UnregisterDAG(dagID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.dags[dagID]; !exists {
		return fmt.Errorf("DAG %s not found", dagID)
	}

	delete(e.dags, dagID)
	return nil
}

// GetDAG retrieves a DAG by ID.
func (e *Engine) GetDAG(dagID string) (*DAG, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	dag, exists := e.dags[dagID]
	if !exists {
		return nil, fmt.Errorf("DAG %s not found", dagID)
	}

	return dag.Clone(), nil
}

// ListDAGs returns all registered DAGs.
func (e *Engine) ListDAGs() []*DAG {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*DAG, 0, len(e.dags))
	for _, dag := range e.dags {
		result = append(result, dag.Clone())
	}
	return result
}

// Compose executes a DAG for an entity and returns the results.
func (e *Engine) Compose(ctx context.Context, dagID string, entityID string) (map[string]interface{}, error) {
	e.mu.RLock()
	dag, exists := e.dags[dagID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("DAG %s not found", dagID)
	}

	results, err := e.executor.Execute(ctx, dag, entityID)
	if err != nil {
		return nil, err
	}

	// Convert to simple values
	output := make(map[string]interface{})
	for nodeID, result := range results {
		if result.Error == "" {
			output[nodeID] = result.Value
		}
	}

	return output, nil
}

// ComposeBatch executes a DAG for multiple entities.
func (e *Engine) ComposeBatch(ctx context.Context, dagID string, entityIDs []string) (map[string]map[string]interface{}, error) {
	e.mu.RLock()
	dag, exists := e.dags[dagID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("DAG %s not found", dagID)
	}

	results := make(map[string]map[string]interface{})
	var resultsMu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(entityIDs))

	for _, entityID := range entityIDs {
		wg.Add(1)
		go func(eid string) {
			defer wg.Done()

			entityResults, err := e.executor.Execute(ctx, dag, eid)
			if err != nil {
				errCh <- fmt.Errorf("entity %s: %w", eid, err)
				return
			}

			output := make(map[string]interface{})
			for nodeID, result := range entityResults {
				if result.Error == "" {
					output[nodeID] = result.Value
				}
			}

			resultsMu.Lock()
			results[eid] = output
			resultsMu.Unlock()
		}(entityID)
	}

	wg.Wait()
	close(errCh)

	// Collect errors
	errs := make([]error, 0, len(entityIDs))
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return results, fmt.Errorf("%d entities failed", len(errs))
	}

	return results, nil
}

// CreateDAGBuilder returns a builder for constructing DAGs fluently.
func (e *Engine) CreateDAGBuilder(id, name string) *DAGBuilder {
	return &DAGBuilder{
		dag:    NewDAG(id, name),
		engine: e,
	}
}

// DAGBuilder provides a fluent interface for building DAGs.
type DAGBuilder struct {
	dag    *DAG
	engine *Engine
	err    error
}

// SetDescription sets the DAG description.
func (b *DAGBuilder) SetDescription(desc string) *DAGBuilder {
	b.dag.Description = desc
	return b
}

// AddSourceNode adds a source node that retrieves a feature.
func (b *DAGBuilder) AddSourceNode(id, featureName string) *DAGBuilder {
	if b.err != nil {
		return b
	}

	node := &Node{
		ID:         id,
		Name:       featureName,
		Type:       NodeTypeSource,
		Expression: featureName,
		Inputs:     []string{},
	}

	b.err = b.dag.AddNode(node)
	return b
}

// AddTransformNode adds a transform node.
func (b *DAGBuilder) AddTransformNode(id, name, expression string, inputs []string) *DAGBuilder {
	if b.err != nil {
		return b
	}

	node := &Node{
		ID:         id,
		Name:       name,
		Type:       NodeTypeTransform,
		Expression: expression,
		Inputs:     inputs,
	}

	b.err = b.dag.AddNode(node)
	return b
}

// AddAggregateNode adds an aggregation node.
func (b *DAGBuilder) AddAggregateNode(id, name, aggType string, inputs []string) *DAGBuilder {
	if b.err != nil {
		return b
	}

	node := &Node{
		ID:     id,
		Name:   name,
		Type:   NodeTypeAggregate,
		Inputs: inputs,
		Config: map[string]interface{}{
			"aggregate": aggType,
		},
	}

	b.err = b.dag.AddNode(node)
	return b
}

// AddJoinNode adds a join node.
func (b *DAGBuilder) AddJoinNode(id, name, joinType string, inputs []string) *DAGBuilder {
	if b.err != nil {
		return b
	}

	node := &Node{
		ID:     id,
		Name:   name,
		Type:   NodeTypeJoin,
		Inputs: inputs,
		Config: map[string]interface{}{
			"join_type": joinType,
		},
	}

	b.err = b.dag.AddNode(node)
	return b
}

// AddFilterNode adds a filter node.
func (b *DAGBuilder) AddFilterNode(id, name, condition string, threshold float64, inputs []string) *DAGBuilder {
	if b.err != nil {
		return b
	}

	node := &Node{
		ID:     id,
		Name:   name,
		Type:   NodeTypeFilter,
		Inputs: inputs,
		Config: map[string]interface{}{
			"condition": condition,
			"threshold": threshold,
		},
	}

	b.err = b.dag.AddNode(node)
	return b
}

// WithCaching enables caching for the last added node.
func (b *DAGBuilder) WithCaching(ttl time.Duration) *DAGBuilder {
	if b.err != nil || len(b.dag.Nodes) == 0 {
		return b
	}

	// Find the last added node (this is a simplified approach)
	for _, node := range b.dag.Nodes {
		node.CacheEnabled = true
		node.CacheTTL = ttl
	}

	return b
}

// SetOutputs sets the output nodes.
func (b *DAGBuilder) SetOutputs(outputs []string) *DAGBuilder {
	if b.err != nil {
		return b
	}

	b.err = b.dag.SetOutputs(outputs)
	return b
}

// Build validates and returns the DAG.
func (b *DAGBuilder) Build() (*DAG, error) {
	if b.err != nil {
		return nil, b.err
	}

	if err := b.dag.Validate(); err != nil {
		return nil, err
	}

	return b.dag, nil
}

// Register validates the DAG and registers it with the engine.
func (b *DAGBuilder) Register() error {
	if b.err != nil {
		return b.err
	}

	return b.engine.RegisterDAG(b.dag)
}

// EngineStats returns statistics about the engine.
type EngineStats struct {
	DAGCount      int           `json:"dag_count"`
	ExecutorStats ExecutorStats `json:"executor_stats"`
}

// Stats returns engine statistics.
func (e *Engine) Stats() EngineStats {
	e.mu.RLock()
	dagCount := len(e.dags)
	e.mu.RUnlock()

	return EngineStats{
		DAGCount:      dagCount,
		ExecutorStats: e.executor.Stats(),
	}
}

// ClearCache clears the executor cache.
func (e *Engine) ClearCache() {
	e.executor.ClearCache()
}
