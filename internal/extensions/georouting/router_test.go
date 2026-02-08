package georouting

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRouter() *Router {
	cfg := DefaultRouterConfig()
	cfg.ResidencyPolicies = map[string]ResidencyPolicy{
		"eu:": ResidencyEU,
		"us:": ResidencyUS,
	}
	r := NewRouter(cfg)

	_ = r.AddRegion(&Region{
		ID: "us-east-1", Name: "US East", Provider: CloudAWS,
		Location: "us", Latitude: 39.0, Longitude: -77.0, Healthy: true,
		Endpoint: "https://us-east-1.feather.cloud",
	})
	_ = r.AddRegion(&Region{
		ID: "eu-west-1", Name: "EU West", Provider: CloudAWS,
		Location: "eu", Latitude: 53.0, Longitude: -6.0, Healthy: true,
		Endpoint: "https://eu-west-1.feather.cloud",
	})
	_ = r.AddRegion(&Region{
		ID: "ap-southeast-1", Name: "APAC Southeast", Provider: CloudGCP,
		Location: "apac", Latitude: 1.3, Longitude: 103.8, Healthy: true,
		Endpoint: "https://ap-southeast-1.feather.cloud",
	})

	r.UpdateMetrics("us-east-1", 5.0, 0.01)
	r.UpdateMetrics("eu-west-1", 10.0, 0.02)
	r.UpdateMetrics("ap-southeast-1", 20.0, 0.01)

	return r
}

func TestRouter_RouteByLatency(t *testing.T) {
	r := setupRouter()
	ctx := context.Background()

	// No residency constraint: should pick lowest latency (us-east-1)
	decision, err := r.Route(ctx, "global:user123")
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", decision.SelectedRegion)
	assert.Equal(t, "lowest_latency", decision.Reason)
}

func TestRouter_RouteWithEUResidency(t *testing.T) {
	r := setupRouter()
	ctx := context.Background()

	// EU residency: must route to eu-west-1
	decision, err := r.Route(ctx, "eu:user456")
	require.NoError(t, err)
	assert.Equal(t, "eu-west-1", decision.SelectedRegion)
}

func TestRouter_RouteWithUSResidency(t *testing.T) {
	r := setupRouter()
	ctx := context.Background()

	decision, err := r.Route(ctx, "us:user789")
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", decision.SelectedRegion)
}

func TestRouter_Failover(t *testing.T) {
	r := setupRouter()
	ctx := context.Background()

	// Mark all regions unhealthy
	r.UpdateMetrics("us-east-1", 100, 0.9)
	r.UpdateMetrics("eu-west-1", 100, 0.9)
	r.UpdateMetrics("ap-southeast-1", 100, 0.9)

	decision, err := r.Route(ctx, "global:user123")
	require.NoError(t, err)
	assert.True(t, decision.Fallback)
	assert.Equal(t, "failover", decision.Reason)
}

func TestRouter_FailoverDisabled(t *testing.T) {
	cfg := DefaultRouterConfig()
	cfg.FailoverEnabled = false
	r := NewRouter(cfg)

	_ = r.AddRegion(&Region{ID: "r1", Location: "us", Healthy: false})
	r.UpdateMetrics("r1", 100, 0.9)

	_, err := r.Route(context.Background(), "user:123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no healthy regions")
}

func TestRouter_NoRegions(t *testing.T) {
	r := NewRouter(DefaultRouterConfig())

	_, err := r.Route(context.Background(), "user:123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no regions configured")
}

func TestRouter_AddRemoveRegion(t *testing.T) {
	r := NewRouter(DefaultRouterConfig())

	err := r.AddRegion(&Region{ID: "r1", Location: "us", Healthy: true})
	require.NoError(t, err)
	assert.Len(t, r.GetRegions(), 1)

	r.RemoveRegion("r1")
	assert.Empty(t, r.GetRegions())
}

func TestRouter_AddRegionValidation(t *testing.T) {
	r := NewRouter(DefaultRouterConfig())
	err := r.AddRegion(&Region{})
	require.Error(t, err)
}

func TestRouter_GetMetrics(t *testing.T) {
	r := setupRouter()
	metrics := r.GetMetrics()
	assert.Len(t, metrics, 3)
	assert.InDelta(t, 5.0, metrics["us-east-1"].LatencyMs, 0.01)
}

func TestRouter_Stats(t *testing.T) {
	r := setupRouter()
	stats := r.Stats()
	assert.Equal(t, 3, stats["total_regions"])
	assert.Equal(t, 3, stats["healthy_regions"])
	assert.Equal(t, 2, stats["policies"])
}

func TestHaversineDistance(t *testing.T) {
	// NYC to London ≈ 5570 km
	dist := HaversineDistance(40.7128, -74.0060, 51.5074, -0.1278)
	assert.InDelta(t, 5570, dist, 50)

	// Same point = 0
	assert.Equal(t, float64(0), HaversineDistance(0, 0, 0, 0))
}

func TestResidencyPolicy(t *testing.T) {
	r := setupRouter()

	tests := []struct {
		entity string
		expect ResidencyPolicy
	}{
		{"eu:user1", ResidencyEU},
		{"us:user2", ResidencyUS},
		{"global:user3", ResidencyNone},
		{"user4", ResidencyNone},
	}

	for _, tt := range tests {
		t.Run(tt.entity, func(t *testing.T) {
			got := r.getResidencyPolicy(tt.entity)
			assert.Equal(t, tt.expect, got)
		})
	}
}
