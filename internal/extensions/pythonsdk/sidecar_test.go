package pythonsdk

import (
	"context"
	"fmt"
	"testing"
)

func TestSidecarManagerLifecycle(t *testing.T) {
	t.Parallel()
	registry := NewRegistry(DefaultRegistryConfig())
	mgr := NewSidecarManager(DefaultSidecarConfig(), registry)

	if err := mgr.Start(); err != nil {
		t.Fatal(err)
	}
	defer mgr.Stop()

	deps := []Dependency{{Name: "pandas", Version: "2.0"}}
	worker, err := mgr.DeployTransform(context.Background(), "udf-clicks", deps)
	if err != nil {
		t.Fatal(err)
	}
	if worker.Status != "running" {
		t.Errorf("expected running, got %s", worker.Status)
	}
	if worker.TransformID != "udf-clicks" {
		t.Errorf("expected udf-clicks, got %s", worker.TransformID)
	}
}

func TestSidecarManagerExecute(t *testing.T) {
	t.Parallel()
	registry := NewRegistry(DefaultRegistryConfig())
	mgr := NewSidecarManager(DefaultSidecarConfig(), registry)
	_ = mgr.Start()
	defer mgr.Stop()

	_, _ = mgr.DeployTransform(context.Background(), "test-udf", nil)

	result, err := mgr.ExecuteTransform(context.Background(), "test-udf", map[string]interface{}{"x": 42})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Error("expected non-nil result")
	}

	stats := mgr.Stats()
	if stats.TotalExecutions != 1 {
		t.Errorf("expected 1 execution, got %d", stats.TotalExecutions)
	}
}

func TestSidecarManagerHotReload(t *testing.T) {
	t.Parallel()
	registry := NewRegistry(DefaultRegistryConfig())
	mgr := NewSidecarManager(DefaultSidecarConfig(), registry)
	_ = mgr.Start()
	defer mgr.Stop()

	_, _ = mgr.DeployTransform(context.Background(), "udf", nil)
	// Deploy again should hot-reload.
	worker, err := mgr.DeployTransform(context.Background(), "udf", nil)
	if err != nil {
		t.Fatal(err)
	}
	if worker.Status != "running" {
		t.Errorf("expected running after reload")
	}

	stats := mgr.Stats()
	if stats.HotReloads != 1 {
		t.Errorf("expected 1 hot reload, got %d", stats.HotReloads)
	}
}

func TestSidecarManagerUndeploy(t *testing.T) {
	t.Parallel()
	registry := NewRegistry(DefaultRegistryConfig())
	mgr := NewSidecarManager(DefaultSidecarConfig(), registry)
	_ = mgr.Start()
	defer mgr.Stop()

	_, _ = mgr.DeployTransform(context.Background(), "udf", nil)
	if err := mgr.UndeployTransform("udf"); err != nil {
		t.Fatal(err)
	}

	workers := mgr.ListWorkers()
	if len(workers) != 0 {
		t.Errorf("expected 0 workers, got %d", len(workers))
	}
}

func TestSidecarManagerNotRunning(t *testing.T) {
	t.Parallel()
	registry := NewRegistry(DefaultRegistryConfig())
	mgr := NewSidecarManager(DefaultSidecarConfig(), registry)

	_, err := mgr.DeployTransform(context.Background(), "udf", nil)
	if err == nil {
		t.Error("expected error when not running")
	}
}

func TestSidecarManagerSandboxRejectModule(t *testing.T) {
t.Parallel()
registry := NewRegistry(DefaultRegistryConfig())
mgr := NewSidecarManager(DefaultSidecarConfig(), registry)
_ = mgr.Start()
defer mgr.Stop()

// "forbidden_module" is not in allowed list.
_, err := mgr.DeployTransform(context.Background(), "udf", []Dependency{{Name: "forbidden_module"}})
if err == nil {
t.Error("expected sandbox to reject forbidden module")
}
}

func TestSidecarManagerCustomExecutor(t *testing.T) {
t.Parallel()
registry := NewRegistry(DefaultRegistryConfig())
mgr := NewSidecarManager(DefaultSidecarConfig(), registry)
_ = mgr.Start()
defer mgr.Stop()

mgr.SetExecutor(func(_ context.Context, _ string, inputs map[string]interface{}) (interface{}, error) {
return map[string]interface{}{"doubled": inputs["x"].(int) * 2}, nil
})

_, _ = mgr.DeployTransform(context.Background(), "double", nil)
result, err := mgr.ExecuteTransform(context.Background(), "double", map[string]interface{}{"x": 5})
if err != nil {
t.Fatal(err)
}
res := result.(map[string]interface{})
if res["doubled"] != 10 {
t.Errorf("expected 10, got %v", res["doubled"])
}
}

func TestSidecarManagerHealthCheck(t *testing.T) {
t.Parallel()
registry := NewRegistry(DefaultRegistryConfig())
mgr := NewSidecarManager(DefaultSidecarConfig(), registry)

if err := mgr.HealthCheck(); err == nil {
t.Error("expected unhealthy when not running")
}

_ = mgr.Start()
if err := mgr.HealthCheck(); err != nil {
t.Errorf("expected healthy, got %v", err)
}
}

func TestSidecarManagerExecutorError(t *testing.T) {
t.Parallel()
registry := NewRegistry(DefaultRegistryConfig())
mgr := NewSidecarManager(DefaultSidecarConfig(), registry)
_ = mgr.Start()
defer mgr.Stop()

mgr.SetExecutor(func(_ context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
return nil, fmt.Errorf("python crash")
})

_, _ = mgr.DeployTransform(context.Background(), "crasher", nil)
_, err := mgr.ExecuteTransform(context.Background(), "crasher", nil)
if err == nil {
t.Error("expected executor error to propagate")
}

stats := mgr.Stats()
if stats.TotalErrors != 1 {
t.Errorf("expected 1 error, got %d", stats.TotalErrors)
}
}
