package wasmudf

import (
	"context"
	"encoding/json"
	"testing"
)

func TestBuiltinExecutor_CompileAndCall(t *testing.T) {
	executor := NewBuiltinExecutor()
	defer executor.Close()

	mod, err := executor.CompileModule(context.Background(), "test-mod", []byte("(module test)"))
	if err != nil {
		t.Fatalf("CompileModule failed: %v", err)
	}
	defer mod.Close()

	input, _ := json.Marshal(map[string]interface{}{"value": 42})
	output, err := mod.Call(context.Background(), "transform", input)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if result["_module"] != "test-mod" {
		t.Errorf("expected _module=test-mod, got %v", result["_module"])
	}
	if _, ok := result["_checksum"]; !ok {
		t.Error("expected _checksum in output")
	}
	if result["value"] != float64(42) {
		t.Errorf("expected value=42, got %v", result["value"])
	}
}

func TestBuiltinExecutor_CallHostFunction(t *testing.T) {
	executor := NewBuiltinExecutor()
	defer executor.Close()

	mod, err := executor.CompileModule(context.Background(), "host-mod", []byte("(module host)"))
	if err != nil {
		t.Fatalf("CompileModule failed: %v", err)
	}
	defer mod.Close()

	input, _ := json.Marshal(map[string]interface{}{"entity": "user1", "feature": "age"})
	output, err := mod.Call(context.Background(), "feather.get_feature", input)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 result, got %d", len(result))
	}
}

func TestBuiltinExecutor_InvalidInput(t *testing.T) {
	executor := NewBuiltinExecutor()
	defer executor.Close()

	mod, _ := executor.CompileModule(context.Background(), "test", []byte("(module)"))
	defer mod.Close()

	_, err := mod.Call(context.Background(), "transform", []byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON input")
	}
}

func TestHostFunctionRegistry_RegisterAndCall(t *testing.T) {
	r := NewHostFunctionRegistry()

	r.Register("custom.add", func(ctx context.Context, params []interface{}) ([]interface{}, error) {
		a := params[0].(float64)
		b := params[1].(float64)
		return []interface{}{a + b}, nil
	})

	result, err := r.Call(context.Background(), "custom.add", []interface{}{1.0, 2.0})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if result[0].(float64) != 3.0 {
		t.Errorf("expected 3.0, got %v", result[0])
	}
}

func TestHostFunctionRegistry_CallNotFound(t *testing.T) {
	r := NewHostFunctionRegistry()

	_, err := r.Call(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent host function")
	}
}

func TestHostFunctionRegistry_List(t *testing.T) {
	r := NewHostFunctionRegistry()

	names := r.List()
	if len(names) != 3 {
		t.Errorf("expected 3 built-in functions, got %d", len(names))
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, expected := range []string{"feather.get_feature", "feather.log", "feather.timestamp"} {
		if !nameSet[expected] {
			t.Errorf("expected %s in list", expected)
		}
	}
}

func TestHostFunctionRegistry_BuiltinTimestamp(t *testing.T) {
	r := NewHostFunctionRegistry()

	result, err := r.Call(context.Background(), "feather.timestamp", nil)
	if err != nil {
		t.Fatalf("timestamp call failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	ts, ok := result[0].(int64)
	if !ok {
		t.Fatalf("expected int64 timestamp, got %T", result[0])
	}
	if ts <= 0 {
		t.Errorf("expected positive timestamp, got %d", ts)
	}
}

func TestHostFunctionRegistry_BuiltinGetFeature_TooFewParams(t *testing.T) {
	r := NewHostFunctionRegistry()

	_, err := r.Call(context.Background(), "feather.get_feature", []interface{}{"only_one"})
	if err == nil {
		t.Fatal("expected error for too few params")
	}
}

func TestBuiltinExecutor_HostFunctions(t *testing.T) {
	executor := NewBuiltinExecutor()
	hf := executor.HostFunctions()
	if hf == nil {
		t.Fatal("expected non-nil host function registry")
	}
	names := hf.List()
	if len(names) == 0 {
		t.Error("expected built-in host functions")
	}
}

func TestRuntimeWithExecutor(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	executor := NewBuiltinExecutor()
	rt.SetExecutor(executor)

	_ = rt.RegisterModule(Module{
		ID:        "exec-test",
		Name:      "test-transform",
		WasmBytes: []byte("(module test)"),
		InputSchema: map[string]string{
			"value": "float",
		},
		OutputSchema: map[string]string{
			"result": "float",
		},
	})

	result, err := rt.Execute("exec-test", map[string]interface{}{
		"value": 42.0,
	})
	if err != nil {
		t.Fatalf("Execute with executor failed: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Output["_module"] != "test-transform" {
		t.Errorf("expected _module=test-transform, got %v", result.Output["_module"])
	}
}

func TestRuntimeWithoutExecutor_FallsBack(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	// No executor set — should use simulation

	_ = rt.RegisterModule(Module{
		ID:   "fallback-mod",
		Name: "fallback",
		InputSchema: map[string]string{
			"x": "float",
		},
		OutputSchema: map[string]string{
			"y": "float",
		},
	})

	result, err := rt.Execute("fallback-mod", map[string]interface{}{"x": 1.0})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
}
