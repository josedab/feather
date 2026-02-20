package pluginsdk

import (
	"testing"
)

type testPlugin struct {
	info PluginInfo
}

func (p *testPlugin) Info() PluginInfo { return p.info }

func (p *testPlugin) Routes() []RouteSpec {
	return []RouteSpec{
		{Method: "GET", Path: "/test", Summary: "Test endpoint"},
	}
}

func (p *testPlugin) Handle(req PluginRequest) PluginResponse {
	return PluginResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"ok":true}`),
	}
}

func newTestPlugin(id, name string) *testPlugin {
	return &testPlugin{
		info: PluginInfo{
			ID:       id,
			Name:     name,
			Version:  "1.0.0",
			Author:   "test",
			Maturity: "stable",
		},
	}
}

func TestRegisterAndGet(t *testing.T) {
	reg := NewPluginRegistry()
	plugin := newTestPlugin("test-1", "Test Plugin")

	if err := reg.Register(plugin); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := reg.Get("test-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Info().Name != "Test Plugin" {
		t.Errorf("got name %q, want %q", got.Info().Name, "Test Plugin")
	}

	// duplicate registration should fail
	if err := reg.Register(plugin); err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestUnregister(t *testing.T) {
	reg := NewPluginRegistry()
	plugin := newTestPlugin("rm-1", "Remove Me")

	_ = reg.Register(plugin)

	if err := reg.Unregister("rm-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := reg.Get("rm-1"); err == nil {
		t.Error("expected error after unregister")
	}

	if err := reg.Unregister("nonexistent"); err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}

func TestList(t *testing.T) {
	reg := NewPluginRegistry()
	_ = reg.Register(newTestPlugin("a", "Alpha"))
	_ = reg.Register(newTestPlugin("b", "Beta"))

	infos := reg.List()
	if len(infos) != 2 {
		t.Fatalf("got %d plugins, want 2", len(infos))
	}
}

func TestRoute(t *testing.T) {
	reg := NewPluginRegistry()
	_ = reg.Register(newTestPlugin("r-1", "Router"))

	req := PluginRequest{Method: "GET", Path: "/test"}
	resp, err := reg.Route("r-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("got status %d, want 200", resp.StatusCode)
	}

	// routing to unknown plugin should fail
	if _, err := reg.Route("unknown", req); err == nil {
		t.Error("expected error for unknown plugin")
	}
}

func TestRegisterEmptyID(t *testing.T) {
	reg := NewPluginRegistry()
	plugin := &testPlugin{
		info: PluginInfo{ID: "", Name: "NoID"},
	}
	err := reg.Register(plugin)
	if err == nil {
		t.Error("expected error for empty plugin ID")
	}
}

func TestGetNonexistent(t *testing.T) {
	reg := NewPluginRegistry()
	_, err := reg.Get("does-not-exist")
	if err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}

func TestRouteRequest(t *testing.T) {
	reg := NewPluginRegistry()

	handler := &testPlugin{
		info: PluginInfo{ID: "echo", Name: "Echo", Version: "1.0.0", Author: "test", Maturity: "stable"},
	}
	_ = reg.Register(handler)

	req := PluginRequest{
		Method:  "POST",
		Path:    "/test",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{"key":"value"}`),
		Params:  map[string]string{"id": "123"},
	}

	resp, err := reg.Route("echo", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Headers["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type header, got %v", resp.Headers)
	}
}

func TestPluginRoutes(t *testing.T) {
	plugin := newTestPlugin("routes-test", "Routes Test")
	routes := plugin.Routes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].Method != "GET" {
		t.Errorf("expected GET method, got %s", routes[0].Method)
	}
	if routes[0].Path != "/test" {
		t.Errorf("expected /test path, got %s", routes[0].Path)
	}
}

func TestListEmpty(t *testing.T) {
	reg := NewPluginRegistry()
	infos := reg.List()
	if len(infos) != 0 {
		t.Errorf("expected 0 plugins in empty registry, got %d", len(infos))
	}
}
