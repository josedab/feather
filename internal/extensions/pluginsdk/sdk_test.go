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
