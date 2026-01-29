package compute

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Errors for the compute package.
var (
	ErrFeatureNotFound   = errors.New("feature definition not found")
	ErrFeatureExists     = errors.New("feature definition already exists")
	ErrInvalidDefinition = errors.New("invalid feature definition")
	ErrComputeFailed     = errors.New("compute failed")
	ErrCyclicDependency  = errors.New("cyclic dependency detected")
	ErrMissingInput      = errors.New("missing required input")
)

// ComputeMode determines when a feature is computed.
type ComputeMode string

const (
	// ComputeModeOnDemand computes the feature when requested.
	ComputeModeOnDemand ComputeMode = "on_demand"
	// ComputeModeScheduled computes the feature on a cron schedule.
	ComputeModeScheduled ComputeMode = "scheduled"
	// ComputeModeStreaming computes the feature as inputs arrive.
	ComputeModeStreaming ComputeMode = "streaming"
	// ComputeModeBatch computes the feature in bulk over entity sets.
	ComputeModeBatch ComputeMode = "batch"
)

// FeatureDefinition defines a computed feature.
type FeatureDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Expression  string                 `json:"expression"`
	Inputs      []string               `json:"inputs"`
	OutputType  string                 `json:"output_type"`
	Mode        ComputeMode            `json:"mode"`
	Schedule    string                 `json:"schedule,omitempty"`
	Incremental bool                   `json:"incremental"`
	CacheTTL    time.Duration          `json:"cache_ttl,omitempty"`
	Tags        map[string]string      `json:"tags,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Version     int                    `json:"version"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// ComputeResult holds the result of a feature computation.
type ComputeResult struct {
	Name      string        `json:"name"`
	Value     interface{}   `json:"value"`
	Duration  time.Duration `json:"duration"`
	CacheHit  bool          `json:"cache_hit"`
	Version   int           `json:"version"`
	Timestamp time.Time     `json:"timestamp"`
}

// ComputeLineage describes the dependency lineage of a feature.
type ComputeLineage struct {
	Feature      string              `json:"feature"`
	Dependencies []string            `json:"dependencies"`
	Dependents   []string            `json:"dependents"`
	Depth        int                 `json:"depth"`
	Graph        map[string][]string `json:"graph"`
}

// ComputeStats reports engine statistics.
type ComputeStats struct {
	DefinitionCount int64   `json:"definition_count"`
	ComputeCount    int64   `json:"compute_count"`
	CacheHits       int64   `json:"cache_hits"`
	CacheMisses     int64   `json:"cache_misses"`
	ErrorCount      int64   `json:"error_count"`
	AvgDurationMs   float64 `json:"avg_duration_ms"`
}

// ComputeMetrics tracks engine metrics.
type ComputeMetrics struct {
	computeCount  atomic.Int64
	cacheHits     atomic.Int64
	cacheMisses   atomic.Int64
	errorCount    atomic.Int64
	totalDuration atomic.Int64 // nanoseconds
}

// ComputeConfig configures the compute engine.
type ComputeConfig struct {
	// MaxDefinitions is the maximum number of feature definitions.
	MaxDefinitions int

	// DefaultCacheTTL is the default cache TTL for computed features.
	DefaultCacheTTL time.Duration

	// MaxBatchSize is the maximum number of entities in a batch compute.
	MaxBatchSize int

	// ComputeTimeout is the default timeout for a single computation.
	ComputeTimeout time.Duration

	// EnableIncrementalCache enables the incremental computation cache.
	EnableIncrementalCache bool
}

// DefaultComputeConfig returns sensible defaults.
func DefaultComputeConfig() ComputeConfig {
	return ComputeConfig{
		MaxDefinitions:         10000,
		DefaultCacheTTL:        5 * time.Minute,
		MaxBatchSize:           1000,
		ComputeTimeout:         30 * time.Second,
		EnableIncrementalCache: true,
	}
}

// computeCache stores cached computation results for incremental mode.
type computeCache struct {
	value     interface{}
	inputHash string
	expiresAt time.Time
}

// ComputeEngine is the main feature computation engine.
type ComputeEngine struct {
	definitions map[string]*FeatureDefinition
	evaluator   *Evaluator
	scheduler   *MaterializationScheduler
	cache       map[string]*computeCache
	config      ComputeConfig
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	metrics     *ComputeMetrics
}

// NewComputeEngine creates a new compute engine with the given configuration.
func NewComputeEngine(config ComputeConfig) *ComputeEngine {
	ctx, cancel := context.WithCancel(context.Background())

	e := &ComputeEngine{
		definitions: make(map[string]*FeatureDefinition),
		evaluator:   NewEvaluator(),
		cache:       make(map[string]*computeCache),
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
		metrics:     &ComputeMetrics{},
	}

	e.scheduler = NewMaterializationScheduler(e)
	return e
}

// Define registers a new feature definition.
func (e *ComputeEngine) Define(ctx context.Context, def *FeatureDefinition) error {
	if err := e.Validate(ctx, def); err != nil {
		return fmt.Errorf("defining feature %s: %w", def.Name, err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.definitions[def.Name]; exists {
		return fmt.Errorf("defining feature %s: %w", def.Name, ErrFeatureExists)
	}

	if len(e.definitions) >= e.config.MaxDefinitions {
		return fmt.Errorf("defining feature %s: max definitions (%d) reached", def.Name, e.config.MaxDefinitions)
	}

	now := time.Now()
	def.CreatedAt = now
	def.UpdatedAt = now
	if def.Version == 0 {
		def.Version = 1
	}
	if def.CacheTTL == 0 {
		def.CacheTTL = e.config.DefaultCacheTTL
	}

	e.definitions[def.Name] = def
	return nil
}

// Undefine removes a feature definition.
func (e *ComputeEngine) Undefine(ctx context.Context, name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.definitions[name]; !exists {
		return fmt.Errorf("undefining feature %s: %w", name, ErrFeatureNotFound)
	}

	delete(e.definitions, name)
	delete(e.cache, name)
	return nil
}

// Get retrieves a feature definition by name.
func (e *ComputeEngine) Get(ctx context.Context, name string) (*FeatureDefinition, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	def, exists := e.definitions[name]
	if !exists {
		return nil, fmt.Errorf("getting feature %s: %w", name, ErrFeatureNotFound)
	}

	return def, nil
}

// List returns all registered feature definitions.
func (e *ComputeEngine) List(ctx context.Context) []*FeatureDefinition {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*FeatureDefinition, 0, len(e.definitions))
	for _, def := range e.definitions {
		result = append(result, def)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Compute computes a feature value given input values.
func (e *ComputeEngine) Compute(ctx context.Context, name string, inputs map[string]interface{}) (*ComputeResult, error) {
	start := time.Now()

	e.mu.RLock()
	def, exists := e.definitions[name]
	e.mu.RUnlock()

	if !exists {
		e.metrics.errorCount.Add(1)
		return nil, fmt.Errorf("computing feature %s: %w", name, ErrFeatureNotFound)
	}

	// Check incremental cache
	if def.Incremental && e.config.EnableIncrementalCache {
		inputHash := computeInputHash(inputs)

		e.mu.RLock()
		cached, hasCached := e.cache[name]
		e.mu.RUnlock()

		if hasCached && cached.inputHash == inputHash && time.Now().Before(cached.expiresAt) {
			e.metrics.cacheHits.Add(1)
			e.metrics.computeCount.Add(1)
			return &ComputeResult{
				Name:      name,
				Value:     cached.value,
				Duration:  time.Since(start),
				CacheHit:  true,
				Version:   def.Version,
				Timestamp: time.Now(),
			}, nil
		}
		e.metrics.cacheMisses.Add(1)
	}

	// Validate all required inputs are present
	for _, input := range def.Inputs {
		if _, ok := inputs[input]; !ok {
			e.metrics.errorCount.Add(1)
			return nil, fmt.Errorf("computing feature %s: %w: %s", name, ErrMissingInput, input)
		}
	}

	// Evaluate expression
	computeCtx, cancel := context.WithTimeout(ctx, e.config.ComputeTimeout)
	defer cancel()

	// Check context before evaluation
	select {
	case <-computeCtx.Done():
		e.metrics.errorCount.Add(1)
		return nil, fmt.Errorf("computing feature %s: %w", name, computeCtx.Err())
	default:
	}

	value, err := e.evaluator.Evaluate(def.Expression, inputs)
	if err != nil {
		e.metrics.errorCount.Add(1)
		return nil, fmt.Errorf("computing feature %s: %w", name, err)
	}

	// Update incremental cache
	if def.Incremental && e.config.EnableIncrementalCache {
		e.mu.Lock()
		e.cache[name] = &computeCache{
			value:     value,
			inputHash: computeInputHash(inputs),
			expiresAt: time.Now().Add(def.CacheTTL),
		}
		e.mu.Unlock()
	}

	duration := time.Since(start)
	e.metrics.computeCount.Add(1)
	e.metrics.totalDuration.Add(duration.Nanoseconds())

	return &ComputeResult{
		Name:      name,
		Value:     value,
		Duration:  duration,
		CacheHit:  false,
		Version:   def.Version,
		Timestamp: time.Now(),
	}, nil
}

// ComputeBatch computes a feature for multiple entity input sets.
func (e *ComputeEngine) ComputeBatch(ctx context.Context, name string, entities []map[string]interface{}) ([]*ComputeResult, error) {
	if len(entities) > e.config.MaxBatchSize {
		return nil, fmt.Errorf("batch computing feature %s: batch size %d exceeds max %d", name, len(entities), e.config.MaxBatchSize)
	}

	results := make([]*ComputeResult, len(entities))
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs := make([]error, 0)

	for i, inputs := range entities {
		wg.Add(1)
		go func(idx int, inp map[string]interface{}) {
			defer wg.Done()

			result, err := e.Compute(ctx, name, inp)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("entity %d: %w", idx, err))
				return
			}
			results[idx] = result
		}(i, inputs)
	}

	wg.Wait()

	if len(errs) > 0 {
		return results, fmt.Errorf("batch computing feature %s: %d entities failed: %w", name, len(errs), errs[0])
	}

	return results, nil
}

// Validate validates a feature definition.
func (e *ComputeEngine) Validate(ctx context.Context, def *FeatureDefinition) error {
	if def == nil {
		return fmt.Errorf("validating definition: %w: definition is nil", ErrInvalidDefinition)
	}
	if def.Name == "" {
		return fmt.Errorf("validating definition: %w: name is required", ErrInvalidDefinition)
	}
	if def.Expression == "" {
		return fmt.Errorf("validating definition: %w: expression is required", ErrInvalidDefinition)
	}
	if def.OutputType == "" {
		return fmt.Errorf("validating definition: %w: output_type is required", ErrInvalidDefinition)
	}

	// Validate compute mode
	switch def.Mode {
	case ComputeModeOnDemand, ComputeModeScheduled, ComputeModeStreaming, ComputeModeBatch, "":
		// valid
	default:
		return fmt.Errorf("validating definition %s: %w: unknown mode %q", def.Name, ErrInvalidDefinition, def.Mode)
	}

	// Validate output type
	switch def.OutputType {
	case "float64", "int64", "string", "bool":
		// valid
	default:
		return fmt.Errorf("validating definition %s: %w: unknown output_type %q", def.Name, ErrInvalidDefinition, def.OutputType)
	}

	// Scheduled features need a schedule expression
	if def.Mode == ComputeModeScheduled && def.Schedule == "" {
		return fmt.Errorf("validating definition %s: %w: schedule required for scheduled mode", def.Name, ErrInvalidDefinition)
	}

	// Check for self-referencing inputs
	for _, input := range def.Inputs {
		if input == def.Name {
			return fmt.Errorf("validating definition %s: %w: feature references itself", def.Name, ErrCyclicDependency)
		}
	}

	return nil
}

// GetLineage returns the dependency lineage for a feature.
func (e *ComputeEngine) GetLineage(ctx context.Context, name string) (*ComputeLineage, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	def, exists := e.definitions[name]
	if !exists {
		return nil, fmt.Errorf("getting lineage for %s: %w", name, ErrFeatureNotFound)
	}

	// Build dependency graph
	graph := make(map[string][]string)
	graph[name] = def.Inputs

	// Find all transitive dependencies
	allDeps := make(map[string]bool)
	e.collectDependencies(name, allDeps, 0)

	// Find dependents (features that use this feature as input)
	dependents := make([]string, 0)
	for _, d := range e.definitions {
		for _, input := range d.Inputs {
			if input == name {
				dependents = append(dependents, d.Name)
				break
			}
		}
	}

	// Calculate depth
	depth := e.calculateDepth(name)

	deps := make([]string, 0, len(allDeps))
	for dep := range allDeps {
		deps = append(deps, dep)
	}
	sort.Strings(deps)
	sort.Strings(dependents)

	return &ComputeLineage{
		Feature:      name,
		Dependencies: deps,
		Dependents:   dependents,
		Depth:        depth,
		Graph:        graph,
	}, nil
}

func (e *ComputeEngine) collectDependencies(name string, visited map[string]bool, depth int) {
	def, exists := e.definitions[name]
	if !exists || depth > 100 {
		return
	}

	for _, input := range def.Inputs {
		if visited[input] {
			continue
		}
		// Only include inputs that are themselves defined features
		if _, isDefined := e.definitions[input]; isDefined {
			visited[input] = true
			e.collectDependencies(input, visited, depth+1)
		}
	}
}

func (e *ComputeEngine) calculateDepth(name string) int {
	def, exists := e.definitions[name]
	if !exists || len(def.Inputs) == 0 {
		return 0
	}

	maxDepth := 0
	for _, input := range def.Inputs {
		// Only count depth through defined features
		if _, isDefined := e.definitions[input]; isDefined {
			d := e.calculateDepth(input)
			if d+1 > maxDepth {
				maxDepth = d + 1
			}
		}
	}
	return maxDepth
}

// Stats returns engine statistics.
func (e *ComputeEngine) Stats() *ComputeStats {
	e.mu.RLock()
	defCount := int64(len(e.definitions))
	e.mu.RUnlock()

	computeCount := e.metrics.computeCount.Load()
	var avgDuration float64
	if computeCount > 0 {
		avgDuration = float64(e.metrics.totalDuration.Load()) / float64(computeCount) / 1e6
	}

	return &ComputeStats{
		DefinitionCount: defCount,
		ComputeCount:    computeCount,
		CacheHits:       e.metrics.cacheHits.Load(),
		CacheMisses:     e.metrics.cacheMisses.Load(),
		ErrorCount:      e.metrics.errorCount.Load(),
		AvgDurationMs:   avgDuration,
	}
}

// Close shuts down the compute engine.
func (e *ComputeEngine) Close() error {
	e.cancel()

	if e.scheduler != nil {
		if err := e.scheduler.Close(); err != nil {
			return fmt.Errorf("closing scheduler: %w", err)
		}
	}

	e.mu.Lock()
	e.cache = make(map[string]*computeCache)
	e.mu.Unlock()

	return nil
}

// computeInputHash creates a simple hash of input values for cache comparison.
func computeInputHash(inputs map[string]interface{}) string {
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	hash := ""
	for _, k := range keys {
		hash += fmt.Sprintf("%s=%v;", k, inputs[k])
	}
	return hash
}
