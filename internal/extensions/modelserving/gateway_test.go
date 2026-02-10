package modelserving

import (
	"testing"
)

func TestGateway_Predict(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	gw := NewGateway(reg)

	req := PredictRequest{
		ModelID:  "test_model",
		Version:  1,
		Features: map[string]interface{}{"age": 25.0, "income": 50000.0},
	}

	resp, err := gw.Predict(req)
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if resp.Primary == nil {
		t.Fatal("expected primary result")
	}
	if resp.Primary.ModelID != "test_model" {
		t.Fatalf("expected model_id 'test_model', got %q", resp.Primary.ModelID)
	}
	if resp.Primary.Adapter != "builtin" {
		t.Fatalf("expected builtin adapter, got %q", resp.Primary.Adapter)
	}
}

func TestGateway_ABRouting(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	gw := NewGateway(reg)

	gw.SetABConfig("model_a", ABConfig{
		ModelA:     "model_a",
		ModelB:     "model_b",
		TrafficPct: 0.5,
		Enabled:    true,
	})

	req := PredictRequest{
		ModelID:   "model_a",
		Version:   1,
		Features:  map[string]interface{}{"x": 1.0},
		ABRouting: true,
	}

	// Run multiple predictions to exercise A/B routing
	for i := 0; i < 10; i++ {
		resp, err := gw.Predict(req)
		if err != nil {
			t.Fatalf("Predict: %v", err)
		}
		if resp.Primary == nil {
			t.Fatal("expected primary result")
		}
	}

	stats := gw.Stats()
	if stats.TotalPredictions != 10 {
		t.Fatalf("expected 10 predictions, got %d", stats.TotalPredictions)
	}
	if stats.ABRoutingCount != 10 {
		t.Fatalf("expected 10 AB routing, got %d", stats.ABRoutingCount)
	}
}

func TestGateway_ShadowScoring(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	gw := NewGateway(reg)

	gw.SetABConfig("primary", ABConfig{
		ModelA:     "primary",
		ModelB:     "shadow",
		TrafficPct: 0.0, // always route to model A
		Enabled:    true,
	})

	req := PredictRequest{
		ModelID:   "primary",
		Version:   1,
		Features:  map[string]interface{}{"x": 1.0},
		ABRouting: true,
		Shadow:    true,
	}

	resp, err := gw.Predict(req)
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if resp.Shadow == nil {
		t.Fatal("expected shadow result")
	}
	if !resp.Shadow.Shadow {
		t.Fatal("expected shadow flag to be true")
	}

	stats := gw.Stats()
	if stats.ShadowRuns != 1 {
		t.Fatalf("expected 1 shadow run, got %d", stats.ShadowRuns)
	}
}

func TestGateway_GetABConfig(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	gw := NewGateway(reg)

	if cfg := gw.GetABConfig("nonexistent"); cfg != nil {
		t.Fatal("expected nil for non-existent config")
	}

	gw.SetABConfig("m1", ABConfig{ModelA: "m1", ModelB: "m2", TrafficPct: 0.3, Enabled: true})
	cfg := gw.GetABConfig("m1")
	if cfg == nil {
		t.Fatal("expected AB config")
	}
	if cfg.TrafficPct != 0.3 {
		t.Fatalf("expected traffic 0.3, got %f", cfg.TrafficPct)
	}
}

func TestGateway_ListAdapters(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	gw := NewGateway(reg)

	adapters := gw.ListAdapters()
	if len(adapters) != 0 {
		t.Fatalf("expected 0 adapters, got %d", len(adapters))
	}
}

func TestGateway_Stats(t *testing.T) {
	reg := NewRegistry(DefaultRegistryConfig())
	gw := NewGateway(reg)

	gw.Predict(PredictRequest{ModelID: "m", Features: map[string]interface{}{"x": 1.0}})
	stats := gw.Stats()
	if stats.TotalPredictions != 1 {
		t.Fatalf("expected 1 prediction, got %d", stats.TotalPredictions)
	}
}
