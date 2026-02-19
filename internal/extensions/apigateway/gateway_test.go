package apigateway

import (
	"testing"
)

func TestNewGateway(t *testing.T) {
	cfg := DefaultGatewayConfig()
	gw := NewGateway(cfg)
	if gw == nil {
		t.Fatal("expected non-nil gateway")
	}
	if gw.config.MaxBackends != 100 {
		t.Errorf("expected MaxBackends=100, got %d", gw.config.MaxBackends)
	}
}

func TestAddRemoveBackend(t *testing.T) {
	gw := NewGateway(DefaultGatewayConfig())

	b := Backend{ID: "b1", URL: "http://localhost:8080", Status: Healthy, Weight: 1}
	if err := gw.AddBackend(b); err != nil {
		t.Fatalf("AddBackend failed: %v", err)
	}

	// Duplicate add should fail
	if err := gw.AddBackend(b); err == nil {
		t.Error("expected error on duplicate add")
	}

	backends := gw.ListBackends()
	if len(backends) != 1 {
		t.Fatalf("expected 1 backend, got %d", len(backends))
	}

	if err := gw.RemoveBackend("b1"); err != nil {
		t.Fatalf("RemoveBackend failed: %v", err)
	}

	if len(gw.ListBackends()) != 0 {
		t.Error("expected 0 backends after remove")
	}

	// Remove non-existent should fail
	if err := gw.RemoveBackend("b1"); err == nil {
		t.Error("expected error on removing non-existent backend")
	}
}

func TestRoute(t *testing.T) {
	gw := NewGateway(DefaultGatewayConfig())

	b1 := Backend{ID: "b1", URL: "http://host1:8080", Status: Healthy, Weight: 1}
	b2 := Backend{ID: "b2", URL: "http://host2:8080", Status: Healthy, Weight: 1}
	_ = gw.AddBackend(b1)
	_ = gw.AddBackend(b2)

	routedTo := make(map[string]int)
	for i := 0; i < 10; i++ {
		result, err := gw.Route("tenant1", "entity"+string(rune('0'+i)))
		if err != nil {
			t.Fatalf("Route failed: %v", err)
		}
		if result.BackendID == "" && !result.Coalesced {
			t.Error("expected a backend ID for non-coalesced result")
		}
		if result.BackendID != "" {
			routedTo[result.BackendID]++
		}
	}

	if len(routedTo) == 0 {
		t.Error("expected at least one backend to receive requests")
	}

	stats := gw.Stats()
	if stats.TotalBackends != 2 {
		t.Errorf("expected 2 backends, got %d", stats.TotalBackends)
	}
}

func TestRouteHealthyOnly(t *testing.T) {
	gw := NewGateway(DefaultGatewayConfig())

	_ = gw.AddBackend(Backend{ID: "b1", URL: "http://host1:8080", Status: Healthy, Weight: 1})
	_ = gw.AddBackend(Backend{ID: "b2", URL: "http://host2:8080", Status: Healthy, Weight: 1})
	_ = gw.UpdateBackendStatus("b1", Down, 0)

	for i := 0; i < 10; i++ {
		result, err := gw.Route("tenant1", "entity"+string(rune('A'+i)))
		if err != nil {
			t.Fatalf("Route failed: %v", err)
		}
		if result.BackendID == "b1" {
			t.Error("should not route to down backend")
		}
	}
}

func TestRateLimit(t *testing.T) {
	cfg := DefaultGatewayConfig()
	cfg.RateLimitPerSec = 5
	gw := NewGateway(cfg)

	_ = gw.AddBackend(Backend{ID: "b1", URL: "http://host1:8080", Status: Healthy, Weight: 1})

	for i := 0; i < 5; i++ {
		_, err := gw.Route("tenant1", "e"+string(rune('0'+i)))
		if err != nil {
			t.Fatalf("Route %d should succeed: %v", i, err)
		}
	}

	_, err := gw.Route("tenant1", "extra")
	if err != ErrRateLimited {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}

	stats := gw.Stats()
	if stats.TotalRateLimited == 0 {
		t.Error("expected rate limited count > 0")
	}
}

func TestCoalescing(t *testing.T) {
	cfg := DefaultGatewayConfig()
	cfg.CoalesceWindowMs = 500 // wide window for test
	gw := NewGateway(cfg)

	_ = gw.AddBackend(Backend{ID: "b1", URL: "http://host1:8080", Status: Healthy, Weight: 1})

	// First request opens the coalescing window
	r1, err := gw.Route("tenant1", "same-entity")
	if err != nil {
		t.Fatalf("first route failed: %v", err)
	}
	if r1.Coalesced {
		t.Error("first request should not be coalesced")
	}

	// Second request for same entity within window should coalesce
	r2, err := gw.Route("tenant1", "same-entity")
	if err != nil {
		t.Fatalf("second route failed: %v", err)
	}
	if !r2.Coalesced {
		t.Error("second request for same entity should be coalesced")
	}

	stats := gw.Stats()
	if stats.TotalCoalesced == 0 {
		t.Error("expected coalesced count > 0")
	}
}

func TestStats(t *testing.T) {
	gw := NewGateway(DefaultGatewayConfig())

	_ = gw.AddBackend(Backend{ID: "b1", URL: "http://host1:8080", Status: Healthy, Weight: 1})
	_ = gw.AddBackend(Backend{ID: "b2", URL: "http://host2:8080", Status: Degraded, Weight: 1})

	stats := gw.Stats()
	if stats.TotalBackends != 2 {
		t.Errorf("expected 2 backends, got %d", stats.TotalBackends)
	}
	if stats.HealthyBackends != 1 {
		t.Errorf("expected 1 healthy backend, got %d", stats.HealthyBackends)
	}
	if stats.TotalRouted != 0 {
		t.Errorf("expected 0 routed, got %d", stats.TotalRouted)
	}
}
