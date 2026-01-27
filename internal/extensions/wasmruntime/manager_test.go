package wasmruntime

import (
	"errors"
	"testing"
)

func TestNewEdgeManager(t *testing.T) {
	m := NewEdgeManager(DefaultEdgeManagerConfig())
	if m == nil {
		t.Fatal("expected non-nil edge manager")
	}
}

func TestRegisterModule(t *testing.T) {
	m := NewEdgeManager(DefaultEdgeManagerConfig())
	mod, err := m.RegisterModule(Module{
		ID: "age_bucket", Name: "Age Bucketing", WasmBytes: 1024,
		Inputs: []string{"age"}, Outputs: []string{"age_bucket"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if mod.Status != ModuleRegistered {
		t.Errorf("expected registered, got %s", mod.Status)
	}
}

func TestDuplicateModule(t *testing.T) {
	m := NewEdgeManager(DefaultEdgeManagerConfig())
	_, _ = m.RegisterModule(Module{ID: "m1", Name: "M1"})
	_, err := m.RegisterModule(Module{ID: "m1", Name: "M1 dup"})
	if !errors.Is(err, ErrModuleExists) {
		t.Fatalf("expected ErrModuleExists, got %v", err)
	}
}

func TestRegisterDevice(t *testing.T) {
	m := NewEdgeManager(DefaultEdgeManagerConfig())
	dev, err := m.RegisterDevice(Device{ID: "d1", Name: "Edge Device 1", Region: "us-west"})
	if err != nil {
		t.Fatal(err)
	}
	if dev.Status != DeviceOnline {
		t.Errorf("expected online, got %s", dev.Status)
	}
}

func TestDeployModule(t *testing.T) {
	m := NewEdgeManager(DefaultEdgeManagerConfig())
	_, _ = m.RegisterModule(Module{ID: "m1", Name: "M1"})
	_, _ = m.RegisterDevice(Device{ID: "d1", Name: "D1"})

	if err := m.DeployModule("d1", "m1"); err != nil {
		t.Fatal(err)
	}

	dev, _ := m.GetDevice("d1")
	if len(dev.DeployedModules) != 1 {
		t.Errorf("expected 1 deployed module, got %d", len(dev.DeployedModules))
	}
}

func TestSyncDevice(t *testing.T) {
	m := NewEdgeManager(DefaultEdgeManagerConfig())
	_, _ = m.RegisterDevice(Device{ID: "d1", Name: "D1"})

	result, err := m.SyncDevice("d1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected sync success")
	}
}

func TestHeartbeat(t *testing.T) {
	m := NewEdgeManager(DefaultEdgeManagerConfig())
	_, _ = m.RegisterDevice(Device{ID: "d1", Name: "D1"})

	if err := m.Heartbeat("d1"); err != nil {
		t.Fatal(err)
	}
}

func TestDeviceNotFound(t *testing.T) {
	m := NewEdgeManager(DefaultEdgeManagerConfig())
	_, err := m.GetDevice("nonexistent")
	if !errors.Is(err, ErrDeviceNotFound) {
		t.Fatalf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestEdgeManagerStats(t *testing.T) {
	m := NewEdgeManager(DefaultEdgeManagerConfig())
	_, _ = m.RegisterModule(Module{ID: "m1", Name: "M1"})
	_, _ = m.RegisterDevice(Device{ID: "d1", Name: "D1"})

	stats := m.Stats()
	if stats["total_modules"] != 1 {
		t.Errorf("expected 1 module, got %v", stats["total_modules"])
	}
	if stats["total_devices"] != 1 {
		t.Errorf("expected 1 device, got %v", stats["total_devices"])
	}
}
