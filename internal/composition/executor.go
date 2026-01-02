package composition

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ExecutionResult holds the result of node execution.
type ExecutionResult struct {
	NodeID    string        `json:"node_id"`
	Value     interface{}   `json:"value"`
	Duration  time.Duration `json:"duration"`
	CacheHit  bool          `json:"cache_hit"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// ComputeFunc is a function that computes a node's output from its inputs.
type ComputeFunc func(ctx context.Context, node *Node, inputs map[string]interface{}) (interface{}, error)

// SourceFunc is a function that retrieves source data for a node.
type SourceFunc func(ctx context.Context, node *Node, entityID string) (interface{}, error)

// ExecutorConfig configures the DAG executor.
type ExecutorConfig struct {
	MaxParallel    int           // Maximum parallel node executions
	DefaultTimeout time.Duration // Default timeout for node execution
	EnableCaching  bool          // Enable intermediate result caching
	CacheTTL       time.Duration // Default cache TTL
	RetryAttempts  int           // Number of retry attempts
	RetryDelay     time.Duration // Delay between retries
}

// DefaultExecutorConfig returns sensible defaults.
func DefaultExecutorConfig() ExecutorConfig {
	return ExecutorConfig{
		MaxParallel:    10,
		DefaultTimeout: 30 * time.Second,
		EnableCaching:  true,
		CacheTTL:       5 * time.Minute,
		RetryAttempts:  3,
		RetryDelay:     100 * time.Millisecond,
	}
}

// Executor executes DAG computations with parallel processing.
type Executor struct {
	config       ExecutorConfig
	computeFuncs map[NodeType]ComputeFunc
	sourceFunc   SourceFunc
	cache        *resultCache
	mu           sync.RWMutex
}

// NewExecutor creates a new DAG executor.
func NewExecutor(config ExecutorConfig) *Executor {
	return &Executor{
		config:       config,
		computeFuncs: make(map[NodeType]ComputeFunc),
		cache:        newResultCache(config.CacheTTL),
	}
}

// RegisterComputeFunc registers a compute function for a node type.
func (e *Executor) RegisterComputeFunc(nodeType NodeType, fn ComputeFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.computeFuncs[nodeType] = fn
}

// SetSourceFunc sets the function used to retrieve source data.
func (e *Executor) SetSourceFunc(fn SourceFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.sourceFunc = fn
}

// ExecutionPlan represents a pre-computed execution plan.
type ExecutionPlan struct {
	DAG         *DAG
	EntityID    string
	Levels      [][]string // Nodes at each execution level
	TotalNodes  int
	EstimatedMs int64
}

// Plan creates an execution plan for a DAG.
func (e *Executor) Plan(dag *DAG, entityID string) (*ExecutionPlan, error) {
	// Ensure topology is computed
	if err := dag.ComputeTopology(); err != nil {
		return nil, fmt.Errorf("computing topology: %w", err)
	}

	maxLevel := dag.GetMaxLevel()
	levels := make([][]string, maxLevel+1)

	for i := 0; i <= maxLevel; i++ {
		nodes := dag.GetNodesAtLevel(i)
		for _, node := range nodes {
			levels[i] = append(levels[i], node.ID)
		}
	}

	return &ExecutionPlan{
		DAG:        dag,
		EntityID:   entityID,
		Levels:     levels,
		TotalNodes: len(dag.Nodes),
	}, nil
}

// Execute runs a DAG for a given entity and returns all output results.
func (e *Executor) Execute(ctx context.Context, dag *DAG, entityID string) (map[string]*ExecutionResult, error) {
	plan, err := e.Plan(dag, entityID)
	if err != nil {
		return nil, err
	}

	return e.ExecutePlan(ctx, plan)
}

// ExecutePlan executes a pre-computed execution plan.
func (e *Executor) ExecutePlan(ctx context.Context, plan *ExecutionPlan) (map[string]*ExecutionResult, error) {
	results := make(map[string]*ExecutionResult)
	var resultsMu sync.Mutex

	// Execute level by level (nodes at same level can run in parallel)
	for levelIdx, nodeIDs := range plan.Levels {
		if len(nodeIDs) == 0 {
			continue
		}

		levelResults, err := e.executeLevel(ctx, plan.DAG, plan.EntityID, nodeIDs, results, levelIdx)
		if err != nil {
			return results, fmt.Errorf("level %d: %w", levelIdx, err)
		}

		resultsMu.Lock()
		for nodeID, result := range levelResults {
			results[nodeID] = result
		}
		resultsMu.Unlock()
	}

	// Filter to only output nodes
	outputResults := make(map[string]*ExecutionResult)
	for _, outputID := range plan.DAG.Outputs {
		if result, ok := results[outputID]; ok {
			outputResults[outputID] = result
		}
	}

	return outputResults, nil
}

func (e *Executor) executeLevel(
	ctx context.Context,
	dag *DAG,
	entityID string,
	nodeIDs []string,
	previousResults map[string]*ExecutionResult,
	level int,
) (map[string]*ExecutionResult, error) {
	results := make(map[string]*ExecutionResult)
	var resultsMu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(nodeIDs))

	// Limit parallelism
	sem := make(chan struct{}, e.config.MaxParallel)

	for _, nodeID := range nodeIDs {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(nid string) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			node, err := dag.GetNode(nid)
			if err != nil {
				errCh <- fmt.Errorf("node %s: %w", nid, err)
				return
			}

			result, err := e.executeNode(ctx, dag, entityID, node, previousResults)
			if err != nil {
				result = &ExecutionResult{
					NodeID:    nid,
					Error:     err.Error(),
					Timestamp: time.Now(),
				}
			}

			resultsMu.Lock()
			results[nid] = result
			resultsMu.Unlock()
		}(nodeID)
	}

	wg.Wait()
	close(errCh)

	// Collect any errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return results, fmt.Errorf("%d nodes failed at level %d", len(errs), level)
	}

	return results, nil
}

func (e *Executor) executeNode(
	ctx context.Context,
	dag *DAG,
	entityID string,
	node *Node,
	previousResults map[string]*ExecutionResult,
) (*ExecutionResult, error) {
	start := time.Now()

	// Check cache if enabled
	if e.config.EnableCaching && node.CacheEnabled {
		cacheKey := e.cacheKey(dag.ID, node.ID, entityID)
		if cached, ok := e.cache.get(cacheKey); ok {
			return &ExecutionResult{
				NodeID:    node.ID,
				Value:     cached,
				Duration:  time.Since(start),
				CacheHit:  true,
				Timestamp: time.Now(),
			}, nil
		}
	}

	// Set up timeout
	timeout := node.Timeout
	if timeout == 0 {
		timeout = e.config.DefaultTimeout
	}
	nodeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Gather inputs from previous results
	inputs := make(map[string]interface{})
	for _, inputID := range node.Inputs {
		if result, ok := previousResults[inputID]; ok && result.Error == "" {
			inputs[inputID] = result.Value
		} else {
			return nil, fmt.Errorf("missing or failed input %s for node %s", inputID, node.ID)
		}
	}

	// Execute based on node type
	var value interface{}
	var err error

	if node.Type == NodeTypeSource {
		// Source nodes retrieve data
		e.mu.RLock()
		sourceFunc := e.sourceFunc
		e.mu.RUnlock()

		if sourceFunc == nil {
			return nil, fmt.Errorf("no source function configured")
		}
		value, err = sourceFunc(nodeCtx, node, entityID)
	} else {
		// Compute nodes transform data
		e.mu.RLock()
		computeFunc, ok := e.computeFuncs[node.Type]
		e.mu.RUnlock()

		if !ok {
			return nil, fmt.Errorf("no compute function for type %s", node.Type)
		}

		// Retry logic
		attempts := node.Retries
		if attempts == 0 {
			attempts = e.config.RetryAttempts
		}

		for attempt := 0; attempt <= attempts; attempt++ {
			value, err = computeFunc(nodeCtx, node, inputs)
			if err == nil {
				break
			}
			if attempt < attempts {
				time.Sleep(e.config.RetryDelay)
			}
		}
	}

	if err != nil {
		return nil, err
	}

	// Cache result if enabled
	if e.config.EnableCaching && node.CacheEnabled {
		ttl := node.CacheTTL
		if ttl == 0 {
			ttl = e.config.CacheTTL
		}
		cacheKey := e.cacheKey(dag.ID, node.ID, entityID)
		e.cache.set(cacheKey, value, ttl)
	}

	return &ExecutionResult{
		NodeID:    node.ID,
		Value:     value,
		Duration:  time.Since(start),
		CacheHit:  false,
		Timestamp: time.Now(),
	}, nil
}

func (e *Executor) cacheKey(dagID, nodeID, entityID string) string {
	return fmt.Sprintf("%s:%s:%s", dagID, nodeID, entityID)
}

// ClearCache clears all cached results.
func (e *Executor) ClearCache() {
	e.cache.clear()
}

// ClearCacheForEntity clears cached results for a specific entity.
func (e *Executor) ClearCacheForEntity(dagID, entityID string) {
	e.cache.clearPrefix(fmt.Sprintf("%s:", dagID) + entityID)
}

// resultCache provides TTL-based caching for intermediate results.
type resultCache struct {
	data    map[string]cacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
	cleaner *time.Ticker
}

type cacheEntry struct {
	value   interface{}
	expires time.Time
}

func newResultCache(ttl time.Duration) *resultCache {
	c := &resultCache{
		data:    make(map[string]cacheEntry),
		ttl:     ttl,
		cleaner: time.NewTicker(ttl / 2),
	}

	// Start background cleaner
	go c.cleanLoop()

	return c
}

func (c *resultCache) get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.data[key]
	if !ok || time.Now().After(entry.expires) {
		return nil, false
	}
	return entry.value, true
}

func (c *resultCache) set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = cacheEntry{
		value:   value,
		expires: time.Now().Add(ttl),
	}
}

func (c *resultCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[string]cacheEntry)
}

func (c *resultCache) clearPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.data {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.data, key)
		}
	}
}

func (c *resultCache) cleanLoop() {
	for range c.cleaner.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.data {
			if now.After(entry.expires) {
				delete(c.data, key)
			}
		}
		c.mu.Unlock()
	}
}

// Stats returns executor statistics.
type ExecutorStats struct {
	CacheSize     int  `json:"cache_size"`
	ComputeFuncs  int  `json:"compute_funcs"`
	HasSourceFunc bool `json:"has_source_func"`
	MaxParallel   int  `json:"max_parallel"`
	CacheEnabled  bool `json:"cache_enabled"`
}

// Stats returns executor statistics.
func (e *Executor) Stats() ExecutorStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.cache.mu.RLock()
	cacheSize := len(e.cache.data)
	e.cache.mu.RUnlock()

	return ExecutorStats{
		CacheSize:     cacheSize,
		ComputeFuncs:  len(e.computeFuncs),
		HasSourceFunc: e.sourceFunc != nil,
		MaxParallel:   e.config.MaxParallel,
		CacheEnabled:  e.config.EnableCaching,
	}
}
