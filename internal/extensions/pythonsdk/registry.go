package pythonsdk

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// TransformType specifies the kind of Python transform.
type TransformType string

const (
	TransformOnDemand  TransformType = "on_demand"
	TransformBatch     TransformType = "batch"
	TransformStreaming TransformType = "streaming"
)

// TransformStatus represents the current state of a transform.
type TransformStatus string

const (
	StatusRegistered TransformStatus = "registered"
	StatusValidated  TransformStatus = "validated"
	StatusDeployed   TransformStatus = "deployed"
	StatusFailed     TransformStatus = "failed"
)

// FieldSchema defines a single input or output field.
type FieldSchema struct {
	Name     string      `json:"name"`
	DType    string      `json:"dtype"` // "int64", "float64", "string", "bool", "timestamp"
	Required bool        `json:"required"`
	Default  interface{} `json:"default,omitempty"`
}

// TransformDef defines a Python feature transformation.
type TransformDef struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	Type         TransformType     `json:"type"`
	SourceCode   string            `json:"source_code"`
	EntryPoint   string            `json:"entry_point"` // Python function name
	Inputs       []FieldSchema     `json:"inputs"`
	Outputs      []FieldSchema     `json:"outputs"`
	Dependencies []string          `json:"dependencies,omitempty"` // pip packages
	FeatureGroup string            `json:"feature_group,omitempty"`
	Status       TransformStatus   `json:"status"`
	Version      int               `json:"version"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// ExecutionResult holds the result of executing a transform.
type ExecutionResult struct {
	TransformID string                 `json:"transform_id"`
	Outputs     map[string]interface{} `json:"outputs"`
	Duration    time.Duration          `json:"duration_ns"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// RegistryConfig configures the transform registry.
type RegistryConfig struct {
	MaxTransforms    int           `json:"max_transforms"`
	WorkerEndpoint   string        `json:"worker_endpoint"`
	ExecutionTimeout time.Duration `json:"execution_timeout"`
}

// DefaultRegistryConfig returns sensible defaults.
// The worker endpoint can be overridden via the FEATHER_PYTHON_WORKER_ENDPOINT
// environment variable.
func DefaultRegistryConfig() RegistryConfig {
	endpoint := "localhost:50052"
	if v := os.Getenv("FEATHER_PYTHON_WORKER_ENDPOINT"); v != "" {
		endpoint = v
	}
	return RegistryConfig{
		MaxTransforms:    10000,
		WorkerEndpoint:   endpoint,
		ExecutionTimeout: 30 * time.Second,
	}
}

// RegistryStats holds aggregate registry statistics.
type RegistryStats struct {
	TotalTransforms    int            `json:"total_transforms"`
	ByType             map[string]int `json:"by_type"`
	ByStatus           map[string]int `json:"by_status"`
	TotalExecutions    int64          `json:"total_executions"`
	SuccessfulExecs    int64          `json:"successful_executions"`
	FailedExecs        int64          `json:"failed_executions"`
	AvgExecutionTimeMs float64        `json:"avg_execution_time_ms"`
}

// Registry manages Python transform definitions.
type Registry struct {
	mu            sync.RWMutex
	config        RegistryConfig
	transforms    map[string]*TransformDef
	executions    []ExecutionResult
	totalExecs    int64
	successExecs  int64
	failedExecs   int64
	totalExecTime time.Duration
}

// NewRegistry creates a new transform registry.
func NewRegistry(config RegistryConfig) *Registry {
	if config.MaxTransforms == 0 {
		config = DefaultRegistryConfig()
	}
	return &Registry{
		config:     config,
		transforms: make(map[string]*TransformDef),
	}
}

// Register adds a new transform definition.
func (r *Registry) Register(def TransformDef) (*TransformDef, error) {
	if err := r.validateDef(&def); err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.transforms[def.ID]; exists {
		return nil, ErrTransformExists
	}
	if len(r.transforms) >= r.config.MaxTransforms {
		return nil, fmt.Errorf("max transforms reached (%d)", r.config.MaxTransforms)
	}

	now := time.Now()
	def.Status = StatusRegistered
	def.Version = 1
	def.CreatedAt = now
	def.UpdatedAt = now

	r.transforms[def.ID] = &def
	return &def, nil
}

// Get returns a transform by ID.
func (r *Registry) Get(id string) (*TransformDef, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	def, exists := r.transforms[id]
	if !exists {
		return nil, ErrTransformNotFound
	}
	return def, nil
}

// List returns all registered transforms.
func (r *Registry) List() []TransformDef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]TransformDef, 0, len(r.transforms))
	for _, def := range r.transforms {
		result = append(result, *def)
	}
	return result
}

// Update updates an existing transform definition (creates new version).
func (r *Registry) Update(def TransformDef) (*TransformDef, error) {
	if err := r.validateDef(&def); err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.transforms[def.ID]
	if !exists {
		return nil, ErrTransformNotFound
	}

	def.Version = existing.Version + 1
	def.CreatedAt = existing.CreatedAt
	def.UpdatedAt = time.Now()
	def.Status = StatusRegistered

	r.transforms[def.ID] = &def
	return &def, nil
}

// Delete removes a transform definition.
func (r *Registry) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.transforms[id]; !exists {
		return ErrTransformNotFound
	}
	delete(r.transforms, id)
	return nil
}

// Execute simulates executing a transform with given inputs.
func (r *Registry) Execute(id string, inputs map[string]interface{}) (*ExecutionResult, error) {
	r.mu.RLock()
	def, exists := r.transforms[id]
	if !exists {
		r.mu.RUnlock()
		return nil, ErrTransformNotFound
	}
	r.mu.RUnlock()

	start := time.Now()

	// Simulate transform execution
	outputs := make(map[string]interface{})
	for _, out := range def.Outputs {
		outputs[out.Name] = nil // Placeholder; real execution via Python worker
	}

	result := &ExecutionResult{
		TransformID: id,
		Outputs:     outputs,
		Duration:    time.Since(start),
		Success:     true,
		Timestamp:   time.Now(),
	}

	r.mu.Lock()
	r.totalExecs++
	r.successExecs++
	r.totalExecTime += result.Duration
	r.executions = append(r.executions, *result)
	if len(r.executions) > 10000 {
		r.executions = r.executions[1:]
	}
	r.mu.Unlock()

	return result, nil
}

// Validate checks a transform definition without registering it.
func (r *Registry) Validate(def TransformDef) error {
	return r.validateDef(&def)
}

// Deploy marks a transform as deployed.
func (r *Registry) Deploy(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	def, exists := r.transforms[id]
	if !exists {
		return ErrTransformNotFound
	}
	def.Status = StatusDeployed
	def.UpdatedAt = time.Now()
	return nil
}

// Stats returns registry statistics.
func (r *Registry) Stats() RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := RegistryStats{
		TotalTransforms: len(r.transforms),
		ByType:          make(map[string]int),
		ByStatus:        make(map[string]int),
		TotalExecutions: r.totalExecs,
		SuccessfulExecs: r.successExecs,
		FailedExecs:     r.failedExecs,
	}

	for _, def := range r.transforms {
		stats.ByType[string(def.Type)]++
		stats.ByStatus[string(def.Status)]++
	}

	if r.totalExecs > 0 {
		stats.AvgExecutionTimeMs = float64(r.totalExecTime.Milliseconds()) / float64(r.totalExecs)
	}

	return stats
}

func (r *Registry) validateDef(def *TransformDef) error {
	if def.ID == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidTransform)
	}
	if def.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidTransform)
	}
	if def.SourceCode == "" {
		return fmt.Errorf("%w: source_code is required", ErrInvalidTransform)
	}
	if def.EntryPoint == "" {
		def.EntryPoint = "transform"
	}
	if def.Type == "" {
		def.Type = TransformOnDemand
	}
	return nil
}
