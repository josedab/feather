// Package wasm provides a WebAssembly plugin system for custom transformations.
// It allows users to write custom aggregations and transformations in any language
// that compiles to WebAssembly (Rust, Go, Python via pyodide, etc.)
package wasm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Runtime manages WebAssembly plugin execution.
type Runtime struct {
	mu      sync.RWMutex
	plugins map[string]*Plugin
	config  Config
	logger  *slog.Logger
	metrics *Metrics
}

// Config configures the WASM runtime.
type Config struct {
	// MaxMemoryMB is the maximum memory per plugin in megabytes
	MaxMemoryMB int

	// MaxExecutionTime is the maximum execution time per call
	MaxExecutionTime time.Duration

	// MaxPlugins is the maximum number of loaded plugins
	MaxPlugins int

	// PluginDir is the directory to load plugins from
	PluginDir string

	// EnableHotReload enables automatic plugin reloading
	EnableHotReload bool
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxMemoryMB:      64,
		MaxExecutionTime: 5 * time.Second,
		MaxPlugins:       100,
		EnableHotReload:  true,
	}
}

// Plugin represents a loaded WebAssembly plugin.
type Plugin struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Author      string            `json:"author"`
	Type        PluginType        `json:"type"`
	Functions   []FunctionSpec    `json:"functions"`
	Config      map[string]string `json:"config"`
	LoadedAt    time.Time         `json:"loaded_at"`
	State       PluginState       `json:"state"`

	// Internal
	wasmBytes []byte
	instance  *PluginInstance
}

// PluginType indicates the type of plugin.
type PluginType string

const (
	// PluginTypeAggregation defines aggregation plugins.
	PluginTypeAggregation PluginType = "aggregation"
	// PluginTypeTransformation defines transformation plugins.
	PluginTypeTransformation PluginType = "transformation"
	// PluginTypeFilter defines filter plugins.
	PluginTypeFilter PluginType = "filter"
	// PluginTypeEnrichment defines enrichment plugins.
	PluginTypeEnrichment PluginType = "enrichment"
	// PluginTypeValidation defines validation plugins.
	PluginTypeValidation PluginType = "validation"
)

// PluginState indicates the current state of a plugin.
type PluginState string

const (
	// PluginStateLoaded indicates a plugin is loaded into memory.
	PluginStateLoaded PluginState = "loaded"
	// PluginStateActive indicates a plugin is active and callable.
	PluginStateActive PluginState = "active"
	// PluginStateError indicates a plugin encountered a runtime error.
	PluginStateError PluginState = "error"
	// PluginStateDisabled indicates a plugin is disabled.
	PluginStateDisabled PluginState = "disabled"
)

// FunctionSpec describes a function exported by the plugin.
type FunctionSpec struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Inputs      []ParamSpec `json:"inputs"`
	Output      ParamSpec   `json:"output"`
}

// ParamSpec describes a function parameter.
type ParamSpec struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "int64", "float64", "string", "bytes", "json"
	Required bool   `json:"required"`
}

// PluginInstance represents an active instance of a plugin.
type PluginInstance struct {
	plugin      *Plugin
	memoryLimit int
	execTimeout time.Duration
	callCount   int64
	errorCount  int64
	totalExecMs float64
}

// Metrics tracks runtime metrics.
type Metrics struct {
	mu            sync.RWMutex
	TotalCalls    int64
	TotalErrors   int64
	PluginsLoaded int
	AvgExecTimeMs float64
}

// NewRuntime creates a new WASM runtime.
func NewRuntime(config Config, logger *slog.Logger) *Runtime {
	if logger == nil {
		logger = slog.Default()
	}

	return &Runtime{
		plugins: make(map[string]*Plugin),
		config:  config,
		logger:  logger,
		metrics: &Metrics{},
	}
}

// LoadPlugin loads a WebAssembly plugin from bytes.
func (r *Runtime) LoadPlugin(id string, wasmBytes []byte, manifest *PluginManifest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.plugins) >= r.config.MaxPlugins {
		return fmt.Errorf("maximum number of plugins reached: %d", r.config.MaxPlugins)
	}

	// Validate WASM binary (check magic number)
	if len(wasmBytes) < 4 || string(wasmBytes[:4]) != "\x00asm" {
		return fmt.Errorf("invalid WebAssembly binary")
	}

	plugin := &Plugin{
		ID:          id,
		Name:        manifest.Name,
		Description: manifest.Description,
		Version:     manifest.Version,
		Author:      manifest.Author,
		Type:        PluginType(manifest.Type),
		Functions:   manifest.Functions,
		Config:      manifest.Config,
		LoadedAt:    time.Now(),
		State:       PluginStateLoaded,
		wasmBytes:   wasmBytes,
	}

	// Create instance
	plugin.instance = &PluginInstance{
		plugin:      plugin,
		memoryLimit: r.config.MaxMemoryMB * 1024 * 1024,
		execTimeout: r.config.MaxExecutionTime,
	}

	r.plugins[id] = plugin
	r.metrics.PluginsLoaded = len(r.plugins)

	r.logger.Info("Loaded plugin", "id", id, "name", manifest.Name, "type", manifest.Type)

	return nil
}

// PluginManifest describes a plugin's metadata.
type PluginManifest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Author      string            `json:"author"`
	Type        string            `json:"type"`
	Functions   []FunctionSpec    `json:"functions"`
	Config      map[string]string `json:"config"`
}

// UnloadPlugin unloads a plugin.
func (r *Runtime) UnloadPlugin(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.plugins[id]; !ok {
		return fmt.Errorf("plugin not found: %s", id)
	}

	delete(r.plugins, id)
	r.metrics.PluginsLoaded = len(r.plugins)

	r.logger.Info("Unloaded plugin", "id", id)

	return nil
}

// GetPlugin returns a plugin by ID.
func (r *Runtime) GetPlugin(id string) (*Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, ok := r.plugins[id]
	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", id)
	}

	return plugin, nil
}

// ListPlugins returns all loaded plugins.
func (r *Runtime) ListPlugins() []*Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]*Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		plugins = append(plugins, p)
	}
	return plugins
}

// Call invokes a function in a plugin.
func (r *Runtime) Call(ctx context.Context, pluginID, functionName string, args map[string]interface{}) (interface{}, error) {
	r.mu.RLock()
	plugin, ok := r.plugins[pluginID]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}

	if plugin.State != PluginStateLoaded && plugin.State != PluginStateActive {
		return nil, fmt.Errorf("plugin not active: %s", plugin.State)
	}

	// Find function
	var fn *FunctionSpec
	for _, f := range plugin.Functions {
		if f.Name == functionName {
			fn = &f
			break
		}
	}
	if fn == nil {
		return nil, fmt.Errorf("function not found: %s", functionName)
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, r.config.MaxExecutionTime)
	defer cancel()

	startTime := time.Now()

	// Execute function (simulated for now - actual WASM execution would use wazero)
	result, err := r.executeFunction(ctx, plugin, fn, args)

	execTime := time.Since(startTime).Seconds() * 1000

	// Update metrics
	r.metrics.mu.Lock()
	r.metrics.TotalCalls++
	if err != nil {
		r.metrics.TotalErrors++
		plugin.instance.errorCount++
	}
	plugin.instance.callCount++
	plugin.instance.totalExecMs += execTime
	r.metrics.AvgExecTimeMs = (r.metrics.AvgExecTimeMs*float64(r.metrics.TotalCalls-1) + execTime) / float64(r.metrics.TotalCalls)
	r.metrics.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("execution error: %w", err)
	}

	return result, nil
}

// executeFunction executes a function in the WASM plugin.
// In a real implementation, this would use wazero or similar.
func (r *Runtime) executeFunction(ctx context.Context, plugin *Plugin, fn *FunctionSpec, args map[string]interface{}) (interface{}, error) {
	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// For demonstration, we'll implement some built-in functions
	// In production, this would execute actual WASM code

	switch plugin.Type {
	case PluginTypeAggregation:
		return r.executeAggregation(ctx, plugin, fn, args)
	case PluginTypeTransformation:
		return r.executeTransformation(ctx, plugin, fn, args)
	case PluginTypeFilter:
		return r.executeFilter(ctx, plugin, fn, args)
	case PluginTypeValidation:
		return r.executeValidation(ctx, plugin, fn, args)
	default:
		return nil, fmt.Errorf("unsupported plugin type: %s", plugin.Type)
	}
}

func (r *Runtime) executeAggregation(ctx context.Context, plugin *Plugin, fn *FunctionSpec, args map[string]interface{}) (interface{}, error) {
	values, ok := args["values"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("values must be an array")
	}

	// Convert to float64
	floats := make([]float64, 0, len(values))
	for _, v := range values {
		switch val := v.(type) {
		case float64:
			floats = append(floats, val)
		case int:
			floats = append(floats, float64(val))
		case int64:
			floats = append(floats, float64(val))
		case json.Number:
			f, _ := val.Float64()
			floats = append(floats, f)
		}
	}

	// Built-in aggregations based on function name
	switch fn.Name {
	case "sum":
		var sum float64
		for _, v := range floats {
			sum += v
		}
		return sum, nil

	case "avg":
		if len(floats) == 0 {
			return 0.0, nil
		}
		var sum float64
		for _, v := range floats {
			sum += v
		}
		return sum / float64(len(floats)), nil

	case "count":
		return len(floats), nil

	case "min":
		if len(floats) == 0 {
			return nil, nil
		}
		minValue := floats[0]
		for _, v := range floats[1:] {
			if v < minValue {
				minValue = v
			}
		}
		return minValue, nil

	case "max":
		if len(floats) == 0 {
			return nil, nil
		}
		maxValue := floats[0]
		for _, v := range floats[1:] {
			if v > maxValue {
				maxValue = v
			}
		}
		return maxValue, nil

	default:
		return nil, fmt.Errorf("unknown aggregation function: %s", fn.Name)
	}
}

func (r *Runtime) executeTransformation(ctx context.Context, plugin *Plugin, fn *FunctionSpec, args map[string]interface{}) (interface{}, error) {
	value := args["value"]
	config, ok := args["config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("config must be an object")
	}

	switch fn.Name {
	case "normalize":
		v, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("value must be a number")
		}
		minValue, ok := config["min"].(float64)
		if !ok {
			return nil, fmt.Errorf("min must be a number")
		}
		maxValue, ok := config["max"].(float64)
		if !ok {
			return nil, fmt.Errorf("max must be a number")
		}
		if maxValue == minValue {
			return 0.0, nil
		}
		return (v - minValue) / (maxValue - minValue), nil

	case "scale":
		v, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("value must be a number")
		}
		factor, ok := config["factor"].(float64)
		if !ok {
			return nil, fmt.Errorf("factor must be a number")
		}
		return v * factor, nil

	case "log":
		v, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("value must be a number")
		}
		if v <= 0 {
			return nil, fmt.Errorf("log of non-positive number")
		}
		// Natural log approximation for demo
		result := 0.0
		x := (v - 1) / (v + 1)
		for i := 1; i <= 20; i += 2 {
			term := 1.0
			for j := 0; j < i; j++ {
				term *= x
			}
			result += term / float64(i)
		}
		return 2 * result, nil

	default:
		return nil, fmt.Errorf("unknown transformation function: %s", fn.Name)
	}
}

func (r *Runtime) executeFilter(ctx context.Context, plugin *Plugin, fn *FunctionSpec, args map[string]interface{}) (interface{}, error) {
	value := args["value"]
	condition, ok := args["condition"].(string)
	if !ok {
		return nil, fmt.Errorf("condition must be a string")
	}
	threshold := args["threshold"]

	switch condition {
	case "gt":
		v, _ := toFloat(value)
		t, _ := toFloat(threshold)
		return v > t, nil
	case "lt":
		v, _ := toFloat(value)
		t, _ := toFloat(threshold)
		return v < t, nil
	case "eq":
		return value == threshold, nil
	case "ne":
		return value != threshold, nil
	default:
		return nil, fmt.Errorf("unknown condition: %s", condition)
	}
}

func (r *Runtime) executeValidation(ctx context.Context, plugin *Plugin, fn *FunctionSpec, args map[string]interface{}) (interface{}, error) {
	value := args["value"]
	rules, ok := args["rules"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("rules must be an object")
	}

	result := &ValidationResult{
		Valid:  true,
		Errors: make([]string, 0),
	}

	// Check required
	if required, ok := rules["required"].(bool); ok && required {
		if value == nil {
			result.Valid = false
			result.Errors = append(result.Errors, "value is required")
		}
	}

	// Check type
	if expectedType, ok := rules["type"].(string); ok {
		if !checkType(value, expectedType) {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("expected type %s", expectedType))
		}
	}

	// Check range for numbers
	if minVal, ok := rules["min"]; ok {
		v, _ := toFloat(value)
		m, _ := toFloat(minVal)
		if v < m {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("value must be >= %v", minVal))
		}
	}

	if maxVal, ok := rules["max"]; ok {
		v, _ := toFloat(value)
		m, _ := toFloat(maxVal)
		if v > m {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("value must be <= %v", maxVal))
		}
	}

	return result, nil
}

// ValidationResult represents validation results.
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}

func toFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case json.Number:
		f, _ := val.Float64()
		return f, true
	default:
		return 0, false
	}
}

func checkType(v interface{}, expected string) bool {
	switch expected {
	case "string":
		_, ok := v.(string)
		return ok
	case "number", "float64", "float":
		_, ok := toFloat(v)
		return ok
	case "int", "integer":
		switch v.(type) {
		case int, int32, int64:
			return true
		}
		return false
	case "bool", "boolean":
		_, ok := v.(bool)
		return ok
	default:
		return true
	}
}

// GetMetrics returns runtime metrics.
func (r *Runtime) GetMetrics() *Metrics {
	r.metrics.mu.RLock()
	defer r.metrics.mu.RUnlock()

	return &Metrics{
		TotalCalls:    r.metrics.TotalCalls,
		TotalErrors:   r.metrics.TotalErrors,
		PluginsLoaded: r.metrics.PluginsLoaded,
		AvgExecTimeMs: r.metrics.AvgExecTimeMs,
	}
}

// EnablePlugin enables a plugin.
func (r *Runtime) EnablePlugin(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	plugin, ok := r.plugins[id]
	if !ok {
		return fmt.Errorf("plugin not found: %s", id)
	}

	plugin.State = PluginStateActive
	return nil
}

// DisablePlugin disables a plugin.
func (r *Runtime) DisablePlugin(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	plugin, ok := r.plugins[id]
	if !ok {
		return fmt.Errorf("plugin not found: %s", id)
	}

	plugin.State = PluginStateDisabled
	return nil
}
