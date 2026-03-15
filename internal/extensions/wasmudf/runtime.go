package wasmudf

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// ModuleStatus indicates the lifecycle state of a WASM module.
type ModuleStatus string

// ModuleStatus constants.
const (
	Registered ModuleStatus = "registered"
	Compiled   ModuleStatus = "compiled"
	Failed     ModuleStatus = "failed"
)

// Module represents a registered WASM transformation function.
type Module struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Language      string            `json:"language"`
	WasmBytes     []byte            `json:"wasm_bytes,omitempty"`
	Source        string            `json:"source,omitempty"`
	Function      string            `json:"function,omitempty"`
	InputSchema   map[string]string `json:"input_schema,omitempty"`
	OutputSchema  map[string]string `json:"output_schema,omitempty"`
	MemoryLimitMB int               `json:"memory_limit_mb,omitempty"`
	TimeoutMs     int               `json:"timeout_ms,omitempty"`
	Status        ModuleStatus      `json:"status,omitempty"`
	Version       string            `json:"version,omitempty"`
	CreatedAt     time.Time         `json:"created_at,omitempty"`
	UpdatedAt     time.Time         `json:"updated_at,omitempty"`
}

// ExecutionResult captures the output of a module execution.
type ExecutionResult struct {
	ModuleID     string
	Success      bool
	Output       map[string]interface{}
	DurationMs   float64
	MemoryUsedMB int
	Error        string
}

// RuntimeConfig configures the WASM runtime.
type RuntimeConfig struct {
	MaxModules       int
	DefaultMemoryMB  int
	DefaultTimeoutMs int
	MaxConcurrent    int
}

// DefaultRuntimeConfig returns sensible defaults.
func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		MaxModules:       1000,
		DefaultMemoryMB:  64,
		DefaultTimeoutMs: 5000,
		MaxConcurrent:    32,
	}
}

// RuntimeStats holds aggregate statistics.
type RuntimeStats struct {
	TotalModules    int64
	TotalExecutions int64
	TotalErrors     int64
	AvgExecutionMs  float64
}

// Runtime manages WASM module registration and execution.
type Runtime struct {
	mu         sync.RWMutex
	config     RuntimeConfig
	modules    map[string]*Module
	execCount  int64
	execErrors int64
	totalMs    float64
	executor   WASMExecutor
	compiled   map[string]CompiledModule
}

// NewRuntime creates a new WASM runtime.
func NewRuntime(config RuntimeConfig) *Runtime {
	if config.MaxModules == 0 {
		config = DefaultRuntimeConfig()
	}
	return &Runtime{
		config:  config,
		modules: make(map[string]*Module),
	}
}

// RegisterModule registers a new WASM module.
func (r *Runtime) RegisterModule(mod Module) error {
	if mod.ID == "" {
		return fmt.Errorf("module ID is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.modules[mod.ID]; exists {
		return ErrModuleExists
	}
	if len(r.modules) >= r.config.MaxModules {
		return fmt.Errorf("max modules (%d) reached: %w", r.config.MaxModules, ErrResourceLimit)
	}

	if mod.MemoryLimitMB == 0 {
		mod.MemoryLimitMB = r.config.DefaultMemoryMB
	}
	if mod.TimeoutMs == 0 {
		mod.TimeoutMs = r.config.DefaultTimeoutMs
	}

	now := time.Now()
	mod.Status = Registered
	mod.CreatedAt = now
	mod.UpdatedAt = now
	r.modules[mod.ID] = &mod
	return nil
}

// UpdateModule updates an existing module.
func (r *Runtime) UpdateModule(id string, mod Module) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.modules[id]
	if !exists {
		return ErrModuleNotFound
	}

	mod.ID = id
	mod.CreatedAt = existing.CreatedAt
	mod.UpdatedAt = time.Now()
	if mod.Status == "" {
		mod.Status = existing.Status
	}
	if mod.MemoryLimitMB == 0 {
		mod.MemoryLimitMB = existing.MemoryLimitMB
	}
	if mod.TimeoutMs == 0 {
		mod.TimeoutMs = existing.TimeoutMs
	}

	r.modules[id] = &mod
	return nil
}

// GetModule returns a module by ID.
func (r *Runtime) GetModule(id string) (*Module, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	mod, exists := r.modules[id]
	if !exists {
		return nil, ErrModuleNotFound
	}
	copy := *mod
	return &copy, nil
}

// ListModules returns all registered modules.
func (r *Runtime) ListModules() []Module {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Module, 0, len(r.modules))
	for _, mod := range r.modules {
		out = append(out, *mod)
	}
	return out
}

// DeleteModule removes a module.
func (r *Runtime) DeleteModule(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.modules[id]; !exists {
		return ErrModuleNotFound
	}
	delete(r.modules, id)
	return nil
}

// SetExecutor sets a pluggable WASM executor for real module execution.
func (r *Runtime) SetExecutor(executor WASMExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executor = executor
	r.compiled = make(map[string]CompiledModule)
}

// Execute runs a WASM module with the given input, using the executor if set.
func (r *Runtime) Execute(moduleID string, input map[string]interface{}) (*ExecutionResult, error) {
	r.mu.RLock()
	mod, exists := r.modules[moduleID]
	if !exists {
		r.mu.RUnlock()
		return nil, ErrModuleNotFound
	}
	// Copy module data under read lock
	inputSchema := make(map[string]string)
	for k, v := range mod.InputSchema {
		inputSchema[k] = v
	}
	outputSchema := make(map[string]string)
	for k, v := range mod.OutputSchema {
		outputSchema[k] = v
	}
	r.mu.RUnlock()

	// Validate input against schema
	for field := range inputSchema {
		if _, ok := input[field]; !ok {
			r.mu.Lock()
			r.execCount++
			r.execErrors++
			r.mu.Unlock()
			return &ExecutionResult{
				ModuleID: moduleID,
				Success:  false,
				Error:    fmt.Sprintf("missing required input field: %s", field),
			}, fmt.Errorf("missing input field %q: %w", field, ErrExecutionFailed)
		}
	}

	// Use executor if available and module has WASM bytes
	r.mu.RLock()
	executor := r.executor
	r.mu.RUnlock()

	if executor != nil && mod.WasmBytes != nil && len(mod.WasmBytes) > 0 {
		return r.executeWithExecutor(moduleID, mod, input, executor)
	}

	// Simulate execution
	startTime := time.Now()
	output := make(map[string]interface{})
	for field, fieldType := range outputSchema {
		output[field] = defaultValueForType(fieldType)
	}
	durationMs := float64(time.Since(startTime).Microseconds()) / 1000.0
	memUsed := 1 + rand.Intn(10)

	r.mu.Lock()
	r.execCount++
	r.totalMs += durationMs
	r.mu.Unlock()

	return &ExecutionResult{
		ModuleID:     moduleID,
		Success:      true,
		Output:       output,
		DurationMs:   durationMs,
		MemoryUsedMB: memUsed,
	}, nil
}

// executeWithExecutor runs a module through the pluggable WASM executor.
func (r *Runtime) executeWithExecutor(moduleID string, mod *Module, input map[string]interface{}, executor WASMExecutor) (*ExecutionResult, error) {
	startTime := time.Now()

	// Compile if not already cached
	r.mu.Lock()
	compiled, exists := r.compiled[moduleID]
	if !exists {
		var err error
		compiled, err = executor.CompileModule(context.Background(), mod.Name, mod.WasmBytes)
		if err != nil {
			r.execCount++
			r.execErrors++
			r.mu.Unlock()
			return &ExecutionResult{
				ModuleID: moduleID,
				Success:  false,
				Error:    fmt.Sprintf("compilation failed: %v", err),
			}, fmt.Errorf("compiling module: %w", err)
		}
		r.compiled[moduleID] = compiled
	}
	r.mu.Unlock()

	// Marshal input to JSON
	inputBytes, err := json.Marshal(input)
	if err != nil {
		r.mu.Lock()
		r.execCount++
		r.execErrors++
		r.mu.Unlock()
		return &ExecutionResult{
			ModuleID: moduleID,
			Success:  false,
			Error:    fmt.Sprintf("marshaling input: %v", err),
		}, fmt.Errorf("marshaling input: %w", err)
	}

	// Call through executor
	outputBytes, err := compiled.Call(context.Background(), "transform", inputBytes)
	if err != nil {
		r.mu.Lock()
		r.execCount++
		r.execErrors++
		r.mu.Unlock()
		return &ExecutionResult{
			ModuleID: moduleID,
			Success:  false,
			Error:    fmt.Sprintf("execution failed: %v", err),
		}, fmt.Errorf("executing module: %w", err)
	}

	// Unmarshal output
	var output map[string]interface{}
	if err := json.Unmarshal(outputBytes, &output); err != nil {
		r.mu.Lock()
		r.execCount++
		r.execErrors++
		r.mu.Unlock()
		return &ExecutionResult{
			ModuleID: moduleID,
			Success:  false,
			Error:    fmt.Sprintf("unmarshaling output: %v", err),
		}, fmt.Errorf("unmarshaling output: %w", err)
	}

	durationMs := float64(time.Since(startTime).Microseconds()) / 1000.0

	r.mu.Lock()
	r.execCount++
	r.totalMs += durationMs
	r.mu.Unlock()

	return &ExecutionResult{
		ModuleID:     moduleID,
		Success:      true,
		Output:       output,
		DurationMs:   durationMs,
		MemoryUsedMB: 1,
	}, nil
}

// Stats returns aggregate statistics.
func (r *Runtime) Stats() RuntimeStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var avgMs float64
	if r.execCount > 0 {
		avgMs = r.totalMs / float64(r.execCount)
	}

	return RuntimeStats{
		TotalModules:    int64(len(r.modules)),
		TotalExecutions: r.execCount,
		TotalErrors:     r.execErrors,
		AvgExecutionMs:  avgMs,
	}
}

// defaultValueForType returns a default value for a schema type.
func defaultValueForType(fieldType string) interface{} {
	switch fieldType {
	case "int":
		return 0
	case "float":
		return 0.0
	case "string":
		return ""
	case "bool":
		return false
	default:
		return nil
	}
}
