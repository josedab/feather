package feastcompat

import (
	"testing"
)

func TestGAGatewayGetOnlineFeatures(t *testing.T) {
	t.Parallel()
	gw := NewGAGateway(DefaultGAConfig())

	// Pre-set a feature value.
	gw.SetFeature("user123", "click_count", 42)

	result, err := gw.GetOnlineFeatures(
		[]map[string]interface{}{{"user_id": "user123"}},
		[]string{"click_count"},
	)
	if err != nil {
		t.Fatal(err)
	}

	results, ok := result["results"].([]map[string]interface{})
	if !ok || len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0]["click_count"] != 42 {
		t.Errorf("expected 42, got %v", results[0]["click_count"])
	}
}

func TestGAGatewayGetOnlineFeaturesNullWhenMissing(t *testing.T) {
	t.Parallel()
	gw := NewGAGateway(DefaultGAConfig())

	result, err := gw.GetOnlineFeatures(
		[]map[string]interface{}{{"user_id": "unknown"}},
		[]string{"missing_feature"},
	)
	if err != nil {
		t.Fatal(err)
	}

	results := result["results"].([]map[string]interface{})
	if results[0]["missing_feature"] != nil {
		t.Error("expected nil for missing feature")
	}
}

func TestGAGatewayEmptyEntityRows(t *testing.T) {
	t.Parallel()
	gw := NewGAGateway(DefaultGAConfig())
	_, err := gw.GetOnlineFeatures(nil, []string{"f"})
	if err == nil {
		t.Error("expected error for empty entity rows")
	}
}

func TestGAGatewayEmptyFeatureRefs(t *testing.T) {
	t.Parallel()
	gw := NewGAGateway(DefaultGAConfig())
	_, err := gw.GetOnlineFeatures([]map[string]interface{}{{"x": 1}}, nil)
	if err == nil {
		t.Error("expected error for empty feature refs")
	}
}

func TestGAGatewayStats(t *testing.T) {
	t.Parallel()
	gw := NewGAGateway(DefaultGAConfig())
	_, _ = gw.GetOnlineFeatures([]map[string]interface{}{{"x": 1}}, []string{"f"})

	stats := gw.Stats()
	if stats.TotalRequests != 1 {
		t.Errorf("expected 1 request, got %d", stats.TotalRequests)
	}
}

func TestGAGatewayListTests(t *testing.T) {
	t.Parallel()
	gw := NewGAGateway(DefaultGAConfig())
	tests := gw.ListTests()
	if len(tests) == 0 {
		t.Error("expected at least one compat test")
	}
}
