package wasm

import (
	"context"
	"testing"
)

func TestRuntime_LoadPlugin(t *testing.T) {
	runtime := NewRuntime(DefaultConfig(), nil)

	// Valid WASM magic number
	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	manifest := &PluginManifest{
		Name:        "test-plugin",
		Description: "Test plugin for unit tests",
		Version:     "1.0.0",
		Type:        "aggregation",
		Functions: []FunctionSpec{
			{
				Name:        "sum",
				Description: "Sum all values",
				Inputs: []ParamSpec{
					{Name: "values", Type: "array", Required: true},
				},
				Output: ParamSpec{Name: "result", Type: "float64"},
			},
		},
	}

	err := runtime.LoadPlugin("test-1", wasmBytes, manifest)
	if err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	// Verify plugin is loaded
	plugin, err := runtime.GetPlugin("test-1")
	if err != nil {
		t.Fatalf("GetPlugin failed: %v", err)
	}

	if plugin.Name != "test-plugin" {
		t.Errorf("expected name 'test-plugin', got %s", plugin.Name)
	}

	if plugin.State != PluginStateLoaded {
		t.Errorf("expected state 'loaded', got %s", plugin.State)
	}
}

func TestRuntime_LoadInvalidWASM(t *testing.T) {
	runtime := NewRuntime(DefaultConfig(), nil)

	// Invalid WASM bytes
	invalidBytes := []byte{0x00, 0x00, 0x00, 0x00}

	manifest := &PluginManifest{
		Name: "invalid",
		Type: "aggregation",
	}

	err := runtime.LoadPlugin("invalid-1", invalidBytes, manifest)
	if err == nil {
		t.Error("expected error for invalid WASM binary")
	}
}

func TestRuntime_UnloadPlugin(t *testing.T) {
	runtime := NewRuntime(DefaultConfig(), nil)

	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	manifest := &PluginManifest{Name: "test", Type: "aggregation"}

	runtime.LoadPlugin("test-1", wasmBytes, manifest)

	err := runtime.UnloadPlugin("test-1")
	if err != nil {
		t.Fatalf("UnloadPlugin failed: %v", err)
	}

	_, err = runtime.GetPlugin("test-1")
	if err == nil {
		t.Error("expected error after unloading")
	}
}

func TestRuntime_CallAggregation(t *testing.T) {
	runtime := NewRuntime(DefaultConfig(), nil)
	ctx := context.Background()

	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	manifest := &PluginManifest{
		Name: "aggregation-plugin",
		Type: "aggregation",
		Functions: []FunctionSpec{
			{Name: "sum"},
			{Name: "avg"},
			{Name: "count"},
			{Name: "min"},
			{Name: "max"},
		},
	}

	runtime.LoadPlugin("agg-1", wasmBytes, manifest)

	tests := []struct {
		function string
		values   []interface{}
		expected float64
	}{
		{"sum", []interface{}{1.0, 2.0, 3.0, 4.0, 5.0}, 15.0},
		{"avg", []interface{}{1.0, 2.0, 3.0, 4.0, 5.0}, 3.0},
		{"min", []interface{}{5.0, 2.0, 8.0, 1.0, 9.0}, 1.0},
		{"max", []interface{}{5.0, 2.0, 8.0, 1.0, 9.0}, 9.0},
	}

	for _, tt := range tests {
		result, err := runtime.Call(ctx, "agg-1", tt.function, map[string]interface{}{
			"values": tt.values,
		})
		if err != nil {
			t.Errorf("%s failed: %v", tt.function, err)
			continue
		}

		r, ok := result.(float64)
		if !ok {
			t.Errorf("%s: expected float64, got %T", tt.function, result)
			continue
		}

		if r != tt.expected {
			t.Errorf("%s: expected %f, got %f", tt.function, tt.expected, r)
		}
	}

	// Test count (returns int)
	result, err := runtime.Call(ctx, "agg-1", "count", map[string]interface{}{
		"values": []interface{}{1.0, 2.0, 3.0},
	})
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if result.(int) != 3 {
		t.Errorf("count: expected 3, got %v", result)
	}
}

func TestRuntime_CallTransformation(t *testing.T) {
	runtime := NewRuntime(DefaultConfig(), nil)
	ctx := context.Background()

	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	manifest := &PluginManifest{
		Name: "transform-plugin",
		Type: "transformation",
		Functions: []FunctionSpec{
			{Name: "normalize"},
			{Name: "scale"},
		},
	}

	runtime.LoadPlugin("transform-1", wasmBytes, manifest)

	// Test normalize
	result, err := runtime.Call(ctx, "transform-1", "normalize", map[string]interface{}{
		"value": 50.0,
		"config": map[string]interface{}{
			"min": 0.0,
			"max": 100.0,
		},
	})
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	if result.(float64) != 0.5 {
		t.Errorf("normalize: expected 0.5, got %v", result)
	}

	// Test scale
	result, err = runtime.Call(ctx, "transform-1", "scale", map[string]interface{}{
		"value": 10.0,
		"config": map[string]interface{}{
			"factor": 2.0,
		},
	})
	if err != nil {
		t.Fatalf("scale failed: %v", err)
	}
	if result.(float64) != 20.0 {
		t.Errorf("scale: expected 20.0, got %v", result)
	}
}

func TestRuntime_CallFilter(t *testing.T) {
	runtime := NewRuntime(DefaultConfig(), nil)
	ctx := context.Background()

	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	manifest := &PluginManifest{
		Name: "filter-plugin",
		Type: "filter",
		Functions: []FunctionSpec{
			{Name: "filter"},
		},
	}

	runtime.LoadPlugin("filter-1", wasmBytes, manifest)

	tests := []struct {
		value     float64
		condition string
		threshold float64
		expected  bool
	}{
		{10.0, "gt", 5.0, true},
		{10.0, "gt", 15.0, false},
		{10.0, "lt", 15.0, true},
		{10.0, "lt", 5.0, false},
	}

	for _, tt := range tests {
		result, err := runtime.Call(ctx, "filter-1", "filter", map[string]interface{}{
			"value":     tt.value,
			"condition": tt.condition,
			"threshold": tt.threshold,
		})
		if err != nil {
			t.Errorf("filter %s failed: %v", tt.condition, err)
			continue
		}

		if result.(bool) != tt.expected {
			t.Errorf("filter %f %s %f: expected %v, got %v",
				tt.value, tt.condition, tt.threshold, tt.expected, result)
		}
	}
}

func TestRuntime_CallValidation(t *testing.T) {
	runtime := NewRuntime(DefaultConfig(), nil)
	ctx := context.Background()

	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	manifest := &PluginManifest{
		Name: "validation-plugin",
		Type: "validation",
		Functions: []FunctionSpec{
			{Name: "validate"},
		},
	}

	runtime.LoadPlugin("validate-1", wasmBytes, manifest)

	// Valid value
	result, err := runtime.Call(ctx, "validate-1", "validate", map[string]interface{}{
		"value": 50.0,
		"rules": map[string]interface{}{
			"type": "number",
			"min":  0.0,
			"max":  100.0,
		},
	})
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}

	vr := result.(*ValidationResult)
	if !vr.Valid {
		t.Errorf("expected valid result, got errors: %v", vr.Errors)
	}

	// Invalid value (out of range)
	result, err = runtime.Call(ctx, "validate-1", "validate", map[string]interface{}{
		"value": 150.0,
		"rules": map[string]interface{}{
			"max": 100.0,
		},
	})
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}

	vr = result.(*ValidationResult)
	if vr.Valid {
		t.Error("expected invalid result")
	}
	if len(vr.Errors) == 0 {
		t.Error("expected validation errors")
	}
}

func TestRuntime_EnableDisablePlugin(t *testing.T) {
	runtime := NewRuntime(DefaultConfig(), nil)

	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	manifest := &PluginManifest{Name: "test", Type: "aggregation"}

	runtime.LoadPlugin("test-1", wasmBytes, manifest)

	// Enable
	err := runtime.EnablePlugin("test-1")
	if err != nil {
		t.Fatalf("EnablePlugin failed: %v", err)
	}

	plugin, _ := runtime.GetPlugin("test-1")
	if plugin.State != PluginStateActive {
		t.Errorf("expected state 'active', got %s", plugin.State)
	}

	// Disable
	err = runtime.DisablePlugin("test-1")
	if err != nil {
		t.Fatalf("DisablePlugin failed: %v", err)
	}

	plugin, _ = runtime.GetPlugin("test-1")
	if plugin.State != PluginStateDisabled {
		t.Errorf("expected state 'disabled', got %s", plugin.State)
	}
}

func TestRuntime_Metrics(t *testing.T) {
	runtime := NewRuntime(DefaultConfig(), nil)
	ctx := context.Background()

	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	manifest := &PluginManifest{
		Name: "metrics-plugin",
		Type: "aggregation",
		Functions: []FunctionSpec{
			{Name: "sum"},
		},
	}

	runtime.LoadPlugin("metrics-1", wasmBytes, manifest)

	// Make some calls
	for i := 0; i < 10; i++ {
		runtime.Call(ctx, "metrics-1", "sum", map[string]interface{}{
			"values": []interface{}{1.0, 2.0, 3.0},
		})
	}

	metrics := runtime.GetMetrics()
	if metrics.TotalCalls != 10 {
		t.Errorf("expected 10 calls, got %d", metrics.TotalCalls)
	}

	if metrics.PluginsLoaded != 1 {
		t.Errorf("expected 1 plugin loaded, got %d", metrics.PluginsLoaded)
	}
}

func TestRuntime_MaxPlugins(t *testing.T) {
	config := DefaultConfig()
	config.MaxPlugins = 2
	runtime := NewRuntime(config, nil)

	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	manifest := &PluginManifest{Name: "test", Type: "aggregation"}

	// Load max plugins
	runtime.LoadPlugin("p1", wasmBytes, manifest)
	runtime.LoadPlugin("p2", wasmBytes, manifest)

	// Try to load one more
	err := runtime.LoadPlugin("p3", wasmBytes, manifest)
	if err == nil {
		t.Error("expected error when exceeding max plugins")
	}
}

func TestRuntime_ListPlugins(t *testing.T) {
	runtime := NewRuntime(DefaultConfig(), nil)

	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	runtime.LoadPlugin("p1", wasmBytes, &PluginManifest{Name: "Plugin 1", Type: "aggregation"})
	runtime.LoadPlugin("p2", wasmBytes, &PluginManifest{Name: "Plugin 2", Type: "transformation"})

	plugins := runtime.ListPlugins()
	if len(plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(plugins))
	}
}
