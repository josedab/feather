package wasmudf

import (
	"errors"
	"testing"
)

func TestNewRuntime(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	if rt == nil {
		t.Fatal("expected non-nil runtime")
	}
	mods := rt.ListModules()
	if len(mods) != 0 {
		t.Errorf("expected 0 modules, got %d", len(mods))
	}
}

func TestRegisterModule(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())

	err := rt.RegisterModule(Module{
		ID:       "mod-1",
		Name:     "normalize",
		Language: "rust",
		InputSchema: map[string]string{
			"value": "float",
			"min":   "float",
			"max":   "float",
		},
		OutputSchema: map[string]string{
			"normalized": "float",
		},
		Version: "v1.0",
	})
	if err != nil {
		t.Fatalf("RegisterModule failed: %v", err)
	}

	mod, err := rt.GetModule("mod-1")
	if err != nil {
		t.Fatalf("GetModule failed: %v", err)
	}
	if mod.Name != "normalize" {
		t.Errorf("expected name normalize, got %s", mod.Name)
	}
	if mod.Status != Registered {
		t.Errorf("expected status Registered, got %s", mod.Status)
	}
}

func TestDuplicateModule(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())

	_ = rt.RegisterModule(Module{ID: "dup-1", Name: "test"})

	err := rt.RegisterModule(Module{ID: "dup-1", Name: "test2"})
	if !errors.Is(err, ErrModuleExists) {
		t.Errorf("expected ErrModuleExists, got %v", err)
	}
}

func TestExecuteModule(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())

	_ = rt.RegisterModule(Module{
		ID:   "exec-mod",
		Name: "transform",
		InputSchema: map[string]string{
			"value": "float",
		},
		OutputSchema: map[string]string{
			"result": "float",
		},
	})

	result, err := rt.Execute("exec-mod", map[string]interface{}{
		"value": 42.0,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if _, ok := result.Output["result"]; !ok {
		t.Error("expected 'result' in output")
	}
	if result.DurationMs < 0 {
		t.Errorf("expected non-negative duration, got %f", result.DurationMs)
	}
}

func TestExecuteInvalidInput(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())

	_ = rt.RegisterModule(Module{
		ID:   "validate-mod",
		Name: "validator",
		InputSchema: map[string]string{
			"required_field": "string",
		},
		OutputSchema: map[string]string{
			"output": "string",
		},
	})

	// Missing required field
	_, err := rt.Execute("validate-mod", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing input field")
	}
	if !errors.Is(err, ErrExecutionFailed) {
		t.Errorf("expected ErrExecutionFailed, got %v", err)
	}
}

func TestExecuteNotFound(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())

	_, err := rt.Execute("nonexistent", map[string]interface{}{})
	if !errors.Is(err, ErrModuleNotFound) {
		t.Errorf("expected ErrModuleNotFound, got %v", err)
	}
}

func TestUpdateModule(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())

	_ = rt.RegisterModule(Module{
		ID:      "update-mod",
		Name:    "original",
		Version: "v1",
	})

	err := rt.UpdateModule("update-mod", Module{
		Name:    "updated",
		Version: "v2",
	})
	if err != nil {
		t.Fatalf("UpdateModule failed: %v", err)
	}

	mod, _ := rt.GetModule("update-mod")
	if mod.Name != "updated" {
		t.Errorf("expected name updated, got %s", mod.Name)
	}
	if mod.Version != "v2" {
		t.Errorf("expected version v2, got %s", mod.Version)
	}

	err = rt.UpdateModule("nonexistent", Module{Name: "test"})
	if !errors.Is(err, ErrModuleNotFound) {
		t.Errorf("expected ErrModuleNotFound, got %v", err)
	}
}

func TestDeleteModule(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())

	_ = rt.RegisterModule(Module{ID: "del-mod", Name: "to-delete"})

	err := rt.DeleteModule("del-mod")
	if err != nil {
		t.Fatalf("DeleteModule failed: %v", err)
	}

	mods := rt.ListModules()
	if len(mods) != 0 {
		t.Errorf("expected 0 modules after delete, got %d", len(mods))
	}

	err = rt.DeleteModule("nonexistent")
	if !errors.Is(err, ErrModuleNotFound) {
		t.Errorf("expected ErrModuleNotFound, got %v", err)
	}
}

func TestStats(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())

	stats := rt.Stats()
	if stats.TotalModules != 0 {
		t.Errorf("expected 0 modules, got %d", stats.TotalModules)
	}

	_ = rt.RegisterModule(Module{
		ID:   "stats-mod",
		Name: "test",
		InputSchema: map[string]string{
			"x": "float",
		},
		OutputSchema: map[string]string{
			"y": "float",
		},
	})

	_, _ = rt.Execute("stats-mod", map[string]interface{}{"x": 1.0})
	_, _ = rt.Execute("stats-mod", map[string]interface{}{"x": 2.0})

	stats = rt.Stats()
	if stats.TotalModules != 1 {
		t.Errorf("expected 1 module, got %d", stats.TotalModules)
	}
	if stats.TotalExecutions != 2 {
		t.Errorf("expected 2 executions, got %d", stats.TotalExecutions)
	}
}
