package plugin

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	return NewRegistry(DefaultRegistryConfig())
}

func newTestPlugin(name string, ptype PluginType) *Plugin {
	return &Plugin{
		Name:    name,
		Version: "1.0.0",
		Type:    ptype,
		Hooks:   []HookPoint{HookPreWrite, HookPostWrite},
	}
}

func registerTestPlugin(t *testing.T, r *Registry, name string, ptype PluginType) *Plugin {
	t.Helper()
	p := newTestPlugin(name, ptype)
	if err := r.Register(context.Background(), p); err != nil {
		t.Fatalf("registering test plugin %q: %v", name, err)
	}
	return p
}

func TestRegistryRegister(t *testing.T) {
	r := newTestRegistry(t)

	p := newTestPlugin("test-plugin", PluginTypeTransform)
	err := r.Register(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected plugin ID to be assigned")
	}
	if p.Status != PluginStatusLoaded {
		t.Fatalf("expected status %q, got %q", PluginStatusLoaded, p.Status)
	}
	if p.Metrics == nil {
		t.Fatal("expected metrics to be initialized")
	}

	// Duplicate ID should fail.
	dup := newTestPlugin("dup-plugin", PluginTypeCustom)
	dup.ID = p.ID
	err = r.Register(context.Background(), dup)
	if err == nil {
		t.Fatal("expected error for duplicate plugin ID")
	}

	// Nil plugin should fail.
	err = r.Register(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil plugin")
	}

	// Missing name should fail.
	err = r.Register(context.Background(), &Plugin{Version: "1.0.0"})
	if err == nil {
		t.Fatal("expected error for missing name")
	}

	// Missing version should fail.
	err = r.Register(context.Background(), &Plugin{Name: "no-version"})
	if err == nil {
		t.Fatal("expected error for missing version")
	}

	// Max plugins limit.
	cfg := DefaultRegistryConfig()
	cfg.MaxPlugins = 1
	r2 := NewRegistry(cfg)
	_ = r2.Register(context.Background(), newTestPlugin("first", PluginTypeCustom))
	err = r2.Register(context.Background(), newTestPlugin("second", PluginTypeCustom))
	if err == nil {
		t.Fatal("expected error when max plugins exceeded")
	}
}

func TestRegistryUnregister(t *testing.T) {
	r := newTestRegistry(t)
	p := registerTestPlugin(t, r, "unreg-plugin", PluginTypeTransform)

	// Register a hook for this plugin.
	err := r.RegisterHook(p.ID, HookPreWrite, 1, func(ctx context.Context, data *HookData) (*HookData, error) {
		return data, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = r.Unregister(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = r.Get(p.ID)
	if err == nil {
		t.Fatal("expected error after unregister")
	}

	// Hooks should be cleaned up.
	r.mu.RLock()
	hooks := r.hooks[HookPreWrite]
	r.mu.RUnlock()
	for _, h := range hooks {
		if h.PluginID == p.ID {
			t.Fatal("expected hooks to be removed after unregister")
		}
	}

	// Unregistering unknown plugin should fail.
	err = r.Unregister(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown plugin")
	}
}

func TestRegistryGetList(t *testing.T) {
	r := newTestRegistry(t)

	p1 := registerTestPlugin(t, r, "plugin-a", PluginTypeStorage)
	p2 := registerTestPlugin(t, r, "plugin-b", PluginTypeTransform)
	_ = registerTestPlugin(t, r, "plugin-c", PluginTypeStorage)

	// Get by ID.
	got, err := r.Get(p1.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "plugin-a" {
		t.Fatalf("expected name %q, got %q", "plugin-a", got.Name)
	}

	// Get unknown.
	_, err = r.Get("unknown-id")
	if err == nil {
		t.Fatal("expected error for unknown ID")
	}

	// List all.
	all := r.List()
	if len(all) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(all))
	}

	// List by type.
	storagePlugins := r.ListByType(PluginTypeStorage)
	if len(storagePlugins) != 2 {
		t.Fatalf("expected 2 storage plugins, got %d", len(storagePlugins))
	}

	transformPlugins := r.ListByType(PluginTypeTransform)
	if len(transformPlugins) != 1 {
		t.Fatalf("expected 1 transform plugin, got %d", len(transformPlugins))
	}
	if transformPlugins[0].ID != p2.ID {
		t.Fatalf("expected plugin ID %q, got %q", p2.ID, transformPlugins[0].ID)
	}
}

func TestPluginEnable(t *testing.T) {
	r := newTestRegistry(t)
	p := registerTestPlugin(t, r, "enable-test", PluginTypeCustom)

	if p.Status != PluginStatusLoaded {
		t.Fatalf("expected initial status %q, got %q", PluginStatusLoaded, p.Status)
	}

	err := r.Enable(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := r.Get(p.ID)
	if got.Status != PluginStatusActive {
		t.Fatalf("expected status %q, got %q", PluginStatusActive, got.Status)
	}

	// Enable unknown plugin.
	err = r.Enable(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown plugin")
	}
}

func TestPluginDisable(t *testing.T) {
	r := newTestRegistry(t)
	p := registerTestPlugin(t, r, "disable-test", PluginTypeCustom)

	_ = r.Enable(context.Background(), p.ID)
	err := r.Disable(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := r.Get(p.ID)
	if got.Status != PluginStatusDisabled {
		t.Fatalf("expected status %q, got %q", PluginStatusDisabled, got.Status)
	}

	// Disable unknown plugin.
	err = r.Disable(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown plugin")
	}
}

func TestHookRegistration(t *testing.T) {
	r := newTestRegistry(t)
	p := registerTestPlugin(t, r, "hook-test", PluginTypeTransform)

	handler := func(ctx context.Context, data *HookData) (*HookData, error) {
		return data, nil
	}

	err := r.RegisterHook(p.ID, HookPreWrite, 10, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nil handler should fail.
	err = r.RegisterHook(p.ID, HookPreWrite, 10, nil)
	if err == nil {
		t.Fatal("expected error for nil handler")
	}

	// Unknown plugin should fail.
	err = r.RegisterHook("nonexistent", HookPreWrite, 10, handler)
	if err == nil {
		t.Fatal("expected error for unknown plugin")
	}

	// Disallowed hook should fail.
	cfg := DefaultRegistryConfig()
	cfg.AllowedHooks = []HookPoint{HookPreRead}
	r2 := NewRegistry(cfg)
	p2 := newTestPlugin("restricted", PluginTypeCustom)
	_ = r2.Register(context.Background(), p2)
	err = r2.RegisterHook(p2.ID, HookPostWrite, 1, handler)
	if err == nil {
		t.Fatal("expected error for disallowed hook")
	}
}

func TestHookExecution(t *testing.T) {
	r := newTestRegistry(t)
	p := registerTestPlugin(t, r, "exec-test", PluginTypeTransform)
	_ = r.Enable(context.Background(), p.ID)

	var order []int
	var mu sync.Mutex

	// Register hooks with different priorities – lower priority value runs first.
	err := r.RegisterHook(p.ID, HookPreWrite, 20, func(ctx context.Context, data *HookData) (*HookData, error) {
		mu.Lock()
		order = append(order, 20)
		mu.Unlock()
		return data, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = r.RegisterHook(p.ID, HookPreWrite, 5, func(ctx context.Context, data *HookData) (*HookData, error) {
		mu.Lock()
		order = append(order, 5)
		mu.Unlock()
		return data, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = r.RegisterHook(p.ID, HookPostWrite, 1, func(ctx context.Context, data *HookData) (*HookData, error) {
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
		return data, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := &HookData{EntityKey: "user:1"}

	_, err = r.ExecuteHooks(context.Background(), HookPreWrite, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Priority 5 should execute before 20.
	if len(order) != 2 {
		t.Fatalf("expected 2 hook executions, got %d", len(order))
	}
	if order[0] != 5 || order[1] != 20 {
		t.Fatalf("expected execution order [5, 20], got %v", order)
	}

	// PostWrite hook should execute separately.
	order = nil
	_, err = r.ExecuteHooks(context.Background(), HookPostWrite, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 1 || order[0] != 1 {
		t.Fatalf("expected execution order [1], got %v", order)
	}

	// Disabled plugin hooks should not execute.
	_ = r.Disable(context.Background(), p.ID)
	order = nil
	_, err = r.ExecuteHooks(context.Background(), HookPreWrite, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 0 {
		t.Fatalf("expected no hook executions for disabled plugin, got %d", len(order))
	}
}

func TestHookDataPropagation(t *testing.T) {
	r := newTestRegistry(t)
	p := registerTestPlugin(t, r, "propagation-test", PluginTypeTransform)
	_ = r.Enable(context.Background(), p.ID)

	// First hook adds a feature.
	err := r.RegisterHook(p.ID, HookPreWrite, 1, func(ctx context.Context, data *HookData) (*HookData, error) {
		if data.Features == nil {
			data.Features = make(map[string]interface{})
		}
		data.Features["added_by_hook1"] = "value1"
		return data, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second hook reads the feature added by the first and adds its own.
	err = r.RegisterHook(p.ID, HookPreWrite, 2, func(ctx context.Context, data *HookData) (*HookData, error) {
		if v, ok := data.Features["added_by_hook1"]; !ok || v != "value1" {
			return nil, fmt.Errorf("expected feature from hook1")
		}
		data.Features["added_by_hook2"] = "value2"
		return data, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := &HookData{EntityKey: "user:2"}
	result, err := r.ExecuteHooks(context.Background(), HookPreWrite, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Features["added_by_hook1"] != "value1" {
		t.Fatal("expected feature from hook1 in result")
	}
	if result.Features["added_by_hook2"] != "value2" {
		t.Fatal("expected feature from hook2 in result")
	}
}

// mockStorageExtension implements StorageExtension for testing.
type mockStorageExtension struct {
	data map[string]interface{}
}

func (m *mockStorageExtension) Get(_ context.Context, key string) (interface{}, error) {
	v, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("key %q not found", key)
	}
	return v, nil
}

func (m *mockStorageExtension) Put(_ context.Context, key string, value interface{}) error {
	m.data[key] = value
	return nil
}

func (m *mockStorageExtension) Delete(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockStorageExtension) List(_ context.Context, _ string) ([]string, error) {
	keys := make([]string, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys, nil
}

// mockTransformExtension implements TransformExtension for testing.
type mockTransformExtension struct{}

func (m *mockTransformExtension) Transform(_ context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	input["transformed"] = true
	return input, nil
}

func (m *mockTransformExtension) Validate(_ map[string]interface{}) error {
	return nil
}

func TestExtensionManager(t *testing.T) {
	em := NewExtensionManager()

	// Storage extension.
	storageExt := &mockStorageExtension{data: make(map[string]interface{})}
	err := em.RegisterStorage("storage-plugin", storageExt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotStorage, err := em.GetStorage("storage-plugin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := gotStorage.Put(context.Background(), "key1", "val1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, err := gotStorage.Get(context.Background(), "key1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "val1" {
		t.Fatalf("expected %q, got %q", "val1", val)
	}

	// Duplicate registration should fail.
	err = em.RegisterStorage("storage-plugin", storageExt)
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}

	// Unknown extension should fail.
	_, err = em.GetStorage("unknown")
	if err == nil {
		t.Fatal("expected error for unknown extension")
	}

	// Nil extension should fail.
	err = em.RegisterStorage("nil-plugin", nil)
	if err == nil {
		t.Fatal("expected error for nil extension")
	}

	// Transform extension.
	transformExt := &mockTransformExtension{}
	err = em.RegisterTransform("transform-plugin", transformExt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotTransform, err := em.GetTransform("transform-plugin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, err := gotTransform.Transform(context.Background(), map[string]interface{}{"input": 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["transformed"] != true {
		t.Fatal("expected transformed flag")
	}

	// ListExtensions.
	exts := em.ListExtensions()
	if len(exts[PluginTypeStorage]) != 1 {
		t.Fatalf("expected 1 storage extension, got %d", len(exts[PluginTypeStorage]))
	}
	if len(exts[PluginTypeTransform]) != 1 {
		t.Fatalf("expected 1 transform extension, got %d", len(exts[PluginTypeTransform]))
	}
}

func TestPluginMetrics(t *testing.T) {
	r := newTestRegistry(t)
	p := registerTestPlugin(t, r, "metrics-test", PluginTypeTransform)
	_ = r.Enable(context.Background(), p.ID)

	callCount := 0
	err := r.RegisterHook(p.ID, HookPreWrite, 1, func(ctx context.Context, data *HookData) (*HookData, error) {
		callCount++
		return data, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := &HookData{EntityKey: "user:3"}

	// Execute hooks multiple times.
	for i := 0; i < 5; i++ {
		_, err := r.ExecuteHooks(context.Background(), HookPreWrite, data)
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
	}

	if callCount != 5 {
		t.Fatalf("expected 5 invocations, got %d", callCount)
	}

	metrics, err := r.GetPluginMetrics(p.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metrics.InvocationCount != 5 {
		t.Fatalf("expected invocation count 5, got %d", metrics.InvocationCount)
	}
	if metrics.ErrorCount != 0 {
		t.Fatalf("expected error count 0, got %d", metrics.ErrorCount)
	}
	if metrics.TotalDuration <= 0 {
		t.Fatal("expected positive total duration")
	}
	if metrics.LastInvoked.IsZero() {
		t.Fatal("expected last invoked to be set")
	}

	// Unknown plugin metrics.
	_, err = r.GetPluginMetrics("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown plugin")
	}

	// Verify stats.
	stats := r.Stats()
	if stats.TotalPlugins != 1 {
		t.Fatalf("expected 1 total plugin, got %d", stats.TotalPlugins)
	}
	if stats.ActivePlugins != 1 {
		t.Fatalf("expected 1 active plugin, got %d", stats.ActivePlugins)
	}
	if stats.TotalHooks != 1 {
		t.Fatalf("expected 1 total hook, got %d", stats.TotalHooks)
	}
}
