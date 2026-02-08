package wasmudf

import (
	"context"
	"testing"
)

func TestSandbox_ExecuteWithLimits_Success(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	_ = rt.RegisterModule(Module{
		ID:            "add-module",
		Name:          "Add Numbers",
		Language:      "rust",
		InputSchema:   map[string]string{"a": "float", "b": "float"},
		OutputSchema:  map[string]string{"result": "float"},
		MemoryLimitMB: 32,
		TimeoutMs:     50,
	})

	sandbox := NewSandbox(rt, DefaultSandboxConfig())

	result, usage, err := sandbox.ExecuteWithLimits(context.Background(), "add-module",
		map[string]interface{}{"a": 1.0, "b": 2.0})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !result.Success {
		t.Fatal("expected successful execution")
	}
	if !usage.WithinLimits {
		t.Errorf("expected within limits, violations: %v", usage.Violations)
	}
}

func TestSandbox_ExecuteWithLimits_MemoryExceeded(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	_ = rt.RegisterModule(Module{
		ID:            "big-module",
		Name:          "Big Module",
		Language:      "go",
		InputSchema:   map[string]string{},
		OutputSchema:  map[string]string{},
		MemoryLimitMB: 128, // Exceeds sandbox limit of 64
		TimeoutMs:     50,
	})

	sandbox := NewSandbox(rt, DefaultSandboxConfig())

	_, usage, err := sandbox.ExecuteWithLimits(context.Background(), "big-module", nil)
	if err == nil {
		t.Fatal("expected error for memory limit exceeded")
	}
	if usage.WithinLimits {
		t.Error("expected within_limits=false")
	}
}

func TestSandbox_ExecuteWithLimits_TimeoutExceeded(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	_ = rt.RegisterModule(Module{
		ID:            "slow-module",
		Name:          "Slow Module",
		InputSchema:   map[string]string{},
		OutputSchema:  map[string]string{},
		MemoryLimitMB: 32,
		TimeoutMs:     200, // Exceeds sandbox limit of 100
	})

	sandbox := NewSandbox(rt, DefaultSandboxConfig())

	_, usage, err := sandbox.ExecuteWithLimits(context.Background(), "slow-module", nil)
	if err == nil {
		t.Fatal("expected error for timeout limit exceeded")
	}
	if usage.WithinLimits {
		t.Error("expected within_limits=false")
	}
}

func TestSandbox_VersionManagement(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	sandbox := NewSandbox(rt, DefaultSandboxConfig())

	_ = sandbox.RegisterVersion("mod-1", ModuleVersion{
		Version:  "1.0.0",
		WasmSize: 1000,
		Language: "rust",
	})
	_ = sandbox.RegisterVersion("mod-1", ModuleVersion{
		Version:  "1.1.0",
		WasmSize: 1200,
		Language: "rust",
	})

	versions := sandbox.GetVersionHistory("mod-1")
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}
	if !versions[1].IsActive {
		t.Error("expected latest version to be active")
	}
	if versions[0].IsActive {
		t.Error("expected older version to be inactive")
	}
}

func TestSandbox_RollbackVersion(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	sandbox := NewSandbox(rt, DefaultSandboxConfig())

	_ = sandbox.RegisterVersion("mod-1", ModuleVersion{Version: "1.0.0", WasmSize: 100})
	_ = sandbox.RegisterVersion("mod-1", ModuleVersion{Version: "2.0.0", WasmSize: 200})

	if err := sandbox.RollbackVersion("mod-1", "1.0.0"); err != nil {
		t.Fatal(err)
	}

	versions := sandbox.GetVersionHistory("mod-1")
	for _, v := range versions {
		if v.Version == "1.0.0" && !v.IsActive {
			t.Error("expected 1.0.0 to be active after rollback")
		}
		if v.Version == "2.0.0" && v.IsActive {
			t.Error("expected 2.0.0 to be inactive after rollback")
		}
	}
}

func TestSandbox_HotReload(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	_ = rt.RegisterModule(Module{
		ID:   "reload-mod",
		Name: "Reloadable",
	})

	sandbox := NewSandbox(rt, DefaultSandboxConfig())

	// Subscribe before reload
	ch := sandbox.SubscribeHotReload("reload-mod")

	err := sandbox.HotReload(context.Background(), "reload-mod", Module{
		Name: "Reloadable V2",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Check notification
	select {
	case notif := <-ch:
		if notif != "reload-mod" {
			t.Errorf("expected reload-mod notification, got %s", notif)
		}
	default:
		t.Error("expected hot-reload notification")
	}
}

func TestSandbox_HotReloadDisabled(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	cfg := DefaultSandboxConfig()
	cfg.EnableHotReload = false
	sandbox := NewSandbox(rt, cfg)

	err := sandbox.HotReload(context.Background(), "mod-1", Module{})
	if err == nil {
		t.Error("expected error when hot reload disabled")
	}
}

func TestSandbox_ModuleSizeLimit(t *testing.T) {
	rt := NewRuntime(DefaultRuntimeConfig())
	cfg := DefaultSandboxConfig()
	cfg.MaxModuleSize = 1000
	sandbox := NewSandbox(rt, cfg)

	err := sandbox.RegisterVersion("mod-1", ModuleVersion{
		Version:  "1.0.0",
		WasmSize: 2000, // Exceeds 1000
	})
	if err == nil {
		t.Error("expected error for oversized module")
	}
}

func TestSupportedLanguages(t *testing.T) {
	langs := SupportedLanguages()
	if len(langs) != 3 {
		t.Errorf("expected 3 supported languages, got %d", len(langs))
	}
	names := map[string]bool{}
	for _, l := range langs {
		names[l.Name] = true
	}
	for _, expected := range []string{"rust", "go", "assemblyscript"} {
		if !names[expected] {
			t.Errorf("expected %s in supported languages", expected)
		}
	}
}
