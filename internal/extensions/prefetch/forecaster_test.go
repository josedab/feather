package prefetch

import (
	"testing"
	"time"
)

func TestNewForecaster(t *testing.T) {
	f := NewForecaster(DefaultForecasterConfig())
	if f == nil {
		t.Fatal("NewForecaster returned nil")
	}
	stats := f.Stats()
	if stats.TotalRecorded != 0 {
		t.Errorf("TotalRecorded = %d, want 0", stats.TotalRecorded)
	}
}

func TestNewForecaster_InvalidConfig(t *testing.T) {
	cfg := ForecasterConfig{
		SmoothingFactor:   -1,
		SeasonalPeriods:   0,
		MinDataPoints:     0,
		ClusterCount:      0,
		MemoryBudgetBytes: 0,
	}
	f := NewForecaster(cfg)
	if f == nil {
		t.Fatal("NewForecaster should handle invalid config")
	}
}

func TestForecaster_RecordAccess(t *testing.T) {
	f := NewForecaster(DefaultForecasterConfig())
	now := time.Now()

	f.RecordAccess("user:1", "clicks", now)
	f.RecordAccess("user:1", "clicks", now.Add(time.Second))
	f.RecordAccess("user:2", "views", now)

	stats := f.Stats()
	if stats.TotalRecorded != 3 {
		t.Errorf("TotalRecorded = %d, want 3", stats.TotalRecorded)
	}
	if stats.EntitiesTracked != 2 {
		t.Errorf("EntitiesTracked = %d, want 2", stats.EntitiesTracked)
	}
	if stats.FeaturesTracked != 2 {
		t.Errorf("FeaturesTracked = %d, want 2", stats.FeaturesTracked)
	}
}

func TestForecaster_Forecast_NoData(t *testing.T) {
	f := NewForecaster(DefaultForecasterConfig())
	forecasts := f.Forecast("unknown_feature", time.Hour)
	if len(forecasts) != 0 {
		t.Errorf("expected 0 forecasts for unknown feature, got %d", len(forecasts))
	}
}

func TestForecaster_Forecast_InsufficientData(t *testing.T) {
	cfg := DefaultForecasterConfig()
	cfg.MinDataPoints = 10
	f := NewForecaster(cfg)

	now := time.Now()
	// Record fewer than MinDataPoints.
	for i := 0; i < 5; i++ {
		f.RecordAccess("user:1", "clicks", now.Add(time.Duration(i)*time.Second))
	}

	forecasts := f.Forecast("clicks", time.Hour)
	if len(forecasts) != 0 {
		t.Errorf("expected 0 forecasts with insufficient data, got %d", len(forecasts))
	}
}

func TestForecaster_Forecast_WithData(t *testing.T) {
	cfg := DefaultForecasterConfig()
	cfg.MinDataPoints = 3
	f := NewForecaster(cfg)

	now := time.Now()
	for i := 0; i < 10; i++ {
		f.RecordAccess("user:1", "clicks", now.Add(time.Duration(i)*time.Second))
	}

	forecasts := f.Forecast("clicks", time.Hour)
	if len(forecasts) == 0 {
		t.Fatal("expected at least one forecast")
	}

	fc := forecasts[0]
	if fc.Feature != "clicks" {
		t.Errorf("Feature = %q, want %q", fc.Feature, "clicks")
	}
	if fc.Entity != "user:1" {
		t.Errorf("Entity = %q, want %q", fc.Entity, "user:1")
	}
	if fc.Predicted <= 0 {
		t.Errorf("Predicted = %f, want > 0", fc.Predicted)
	}
	if fc.Confidence <= 0 || fc.Confidence > 1 {
		t.Errorf("Confidence = %f, want (0,1]", fc.Confidence)
	}
	if fc.NextAccess.IsZero() {
		t.Error("NextAccess should not be zero")
	}
}

func TestForecaster_Forecast_MultipleEntities(t *testing.T) {
	cfg := DefaultForecasterConfig()
	cfg.MinDataPoints = 3
	f := NewForecaster(cfg)

	now := time.Now()
	for i := 0; i < 10; i++ {
		ts := now.Add(time.Duration(i) * time.Second)
		f.RecordAccess("user:1", "clicks", ts)
		f.RecordAccess("user:2", "clicks", ts)
	}

	forecasts := f.Forecast("clicks", time.Hour)
	if len(forecasts) != 2 {
		t.Errorf("expected 2 forecasts, got %d", len(forecasts))
	}
}

func TestForecaster_Forecast_SortedByPriority(t *testing.T) {
	cfg := DefaultForecasterConfig()
	cfg.MinDataPoints = 3
	f := NewForecaster(cfg)

	now := time.Now()
	// user:1 accesses frequently.
	for i := 0; i < 20; i++ {
		f.RecordAccess("user:1", "clicks", now.Add(time.Duration(i)*100*time.Millisecond))
	}
	// user:2 accesses infrequently.
	for i := 0; i < 5; i++ {
		f.RecordAccess("user:2", "clicks", now.Add(time.Duration(i)*10*time.Second))
	}

	forecasts := f.Forecast("clicks", time.Hour)
	if len(forecasts) < 2 {
		t.Skip("not enough forecasts to verify ordering")
	}
	if forecasts[0].Priority < forecasts[1].Priority {
		t.Error("forecasts should be sorted by priority descending")
	}
}

func TestForecaster_ClusterEntities_NoData(t *testing.T) {
	f := NewForecaster(DefaultForecasterConfig())
	clusters := f.ClusterEntities()
	if len(clusters) != 0 {
		t.Errorf("expected 0 clusters with no data, got %d", len(clusters))
	}
}

func TestForecaster_ClusterEntities_SingleEntity(t *testing.T) {
	f := NewForecaster(DefaultForecasterConfig())
	now := time.Now()
	for i := 0; i < 5; i++ {
		f.RecordAccess("user:1", "clicks", now.Add(time.Duration(i)*time.Second))
	}

	clusters := f.ClusterEntities()
	if len(clusters) != 1 {
		t.Errorf("expected 1 cluster for single entity, got %d", len(clusters))
	}
	if clusters[0].Size != 1 {
		t.Errorf("cluster size = %d, want 1", clusters[0].Size)
	}
}

func TestForecaster_ClusterEntities_SimilarPatterns(t *testing.T) {
	cfg := DefaultForecasterConfig()
	cfg.ClusterCount = 2
	f := NewForecaster(cfg)

	now := time.Now()
	// Group A: morning users (hour adjusted via timestamps).
	morningBase := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		f.RecordAccess("morning:1", "clicks", morningBase.Add(time.Duration(i)*time.Minute))
		f.RecordAccess("morning:2", "clicks", morningBase.Add(time.Duration(i)*time.Minute))
	}

	// Group B: evening users.
	eveningBase := time.Date(2024, 1, 1, 21, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		f.RecordAccess("evening:1", "clicks", eveningBase.Add(time.Duration(i)*time.Minute))
		f.RecordAccess("evening:2", "clicks", eveningBase.Add(time.Duration(i)*time.Minute))
	}

	_ = now
	clusters := f.ClusterEntities()
	if len(clusters) == 0 {
		t.Fatal("expected at least one cluster")
	}

	// Verify all entities are assigned.
	totalMembers := 0
	for _, c := range clusters {
		totalMembers += c.Size
		if c.Size != len(c.Members) {
			t.Errorf("cluster %d: Size=%d but len(Members)=%d", c.ID, c.Size, len(c.Members))
		}
		if len(c.Centroid) != 24 {
			t.Errorf("cluster %d: centroid length=%d, want 24", c.ID, len(c.Centroid))
		}
	}
	if totalMembers != 4 {
		t.Errorf("total members across clusters = %d, want 4", totalMembers)
	}
}

func TestForecaster_GetWarmingPlan_NoData(t *testing.T) {
	f := NewForecaster(DefaultForecasterConfig())
	plan := f.GetWarmingPlan(1024 * 1024)
	if plan == nil {
		t.Fatal("GetWarmingPlan returned nil")
	}
	if len(plan.Candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(plan.Candidates))
	}
	if plan.BudgetBytes != 1024*1024 {
		t.Errorf("BudgetBytes = %d, want %d", plan.BudgetBytes, 1024*1024)
	}
}

func TestForecaster_GetWarmingPlan_WithData(t *testing.T) {
	cfg := DefaultForecasterConfig()
	cfg.MinDataPoints = 3
	f := NewForecaster(cfg)

	now := time.Now()
	for i := 0; i < 10; i++ {
		f.RecordAccess("user:1", "clicks", now.Add(time.Duration(i)*time.Second))
		f.RecordAccess("user:2", "views", now.Add(time.Duration(i)*time.Second))
	}

	plan := f.GetWarmingPlan(1024 * 1024)
	if plan == nil {
		t.Fatal("GetWarmingPlan returned nil")
	}
	if len(plan.Candidates) == 0 {
		t.Error("expected warming candidates")
	}
	if plan.EstimatedBytes > plan.BudgetBytes {
		t.Errorf("EstimatedBytes %d exceeds BudgetBytes %d", plan.EstimatedBytes, plan.BudgetBytes)
	}
	if plan.GeneratedAt.IsZero() {
		t.Error("GeneratedAt should not be zero")
	}
}

func TestForecaster_GetWarmingPlan_BudgetExceeded(t *testing.T) {
	cfg := DefaultForecasterConfig()
	cfg.MinDataPoints = 3
	f := NewForecaster(cfg)

	now := time.Now()
	for i := 0; i < 20; i++ {
		for j := 0; j < 10; j++ {
			f.RecordAccess(
				"user:"+string(rune('A'+j)),
				"feature_"+string(rune('a'+j)),
				now.Add(time.Duration(i)*time.Second),
			)
		}
	}

	// Very small budget: only 256 bytes = 1 candidate (each ~256 bytes).
	plan := f.GetWarmingPlan(256)
	if plan == nil {
		t.Fatal("GetWarmingPlan returned nil")
	}
	if plan.EstimatedBytes > 256 {
		t.Errorf("EstimatedBytes %d exceeds budget of 256", plan.EstimatedBytes)
	}
}

func TestForecaster_Stats(t *testing.T) {
	cfg := DefaultForecasterConfig()
	cfg.MinDataPoints = 2
	f := NewForecaster(cfg)

	now := time.Now()
	f.RecordAccess("user:1", "clicks", now)
	f.RecordAccess("user:1", "clicks", now.Add(time.Second))
	f.Forecast("clicks", time.Hour)

	stats := f.Stats()
	if stats.TotalRecorded != 2 {
		t.Errorf("TotalRecorded = %d, want 2", stats.TotalRecorded)
	}
	if stats.TotalForecasts != 1 {
		t.Errorf("TotalForecasts = %d, want 1", stats.TotalForecasts)
	}
	if stats.EntitiesTracked != 1 {
		t.Errorf("EntitiesTracked = %d, want 1", stats.EntitiesTracked)
	}
	if stats.FeaturesTracked != 1 {
		t.Errorf("FeaturesTracked = %d, want 1", stats.FeaturesTracked)
	}
}

func TestEuclideanDist(t *testing.T) {
	a := [24]float64{}
	b := [24]float64{}
	if d := euclideanDist(a, b); d != 0 {
		t.Errorf("distance between identical vectors = %f, want 0", d)
	}

	a[0] = 3
	a[1] = 4
	d := euclideanDist(a, b)
	if d < 4.99 || d > 5.01 {
		t.Errorf("distance = %f, want 5.0", d)
	}
}
