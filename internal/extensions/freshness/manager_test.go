package freshness

import (
	"errors"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	if manager == nil {
		t.Fatal("Expected manager to be non-nil")
	}

	manager.Stop()
}

func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()

	if config.Monitor.WindowSize != 5*time.Minute {
		t.Errorf("Expected monitor window size 5m, got %v", config.Monitor.WindowSize)
	}
	if config.Predictor.DefaultTTL != 5*time.Minute {
		t.Errorf("Expected predictor default TTL 5m, got %v", config.Predictor.DefaultTTL)
	}
}

func TestManager_RecordAccess(t *testing.T) {
	config := DefaultManagerConfig()
	config.Monitor.CleanupInterval = 1 * time.Hour
	manager := NewManager(config)
	defer manager.Stop()

	manager.RecordAccess("feature1", 10*time.Millisecond, true)
	manager.RecordAccess("feature1", 20*time.Millisecond, false)

	metrics, found := manager.GetAccessMetrics("feature1")
	if !found {
		t.Fatal("Expected to find metrics")
	}
	if metrics.TotalAccesses != 2 {
		t.Errorf("Expected 2 accesses, got %d", metrics.TotalAccesses)
	}
}

func TestManager_RecordChange(t *testing.T) {
	config := DefaultManagerConfig()
	config.Monitor.CleanupInterval = 1 * time.Hour
	manager := NewManager(config)
	defer manager.Stop()

	manager.RecordChange("feature1", 100.0, 110.0)

	metrics, found := manager.GetChangeMetrics("feature1")
	if !found {
		t.Fatal("Expected to find metrics")
	}
	if metrics.TotalUpdates != 1 {
		t.Errorf("Expected 1 update, got %d", metrics.TotalUpdates)
	}
}

func TestManager_RecordStaleServe(t *testing.T) {
	config := DefaultManagerConfig()
	config.Monitor.CleanupInterval = 1 * time.Hour
	manager := NewManager(config)
	defer manager.Stop()

	manager.RecordAccess("feature1", 10*time.Millisecond, true)
	manager.RecordStaleServe("feature1")

	metrics, _ := manager.GetAccessMetrics("feature1")
	if metrics.StaleServes != 1 {
		t.Errorf("Expected 1 stale serve, got %d", metrics.StaleServes)
	}
}

func TestManager_RecordDriftScore(t *testing.T) {
	config := DefaultManagerConfig()
	config.Monitor.CleanupInterval = 1 * time.Hour
	manager := NewManager(config)
	defer manager.Stop()

	manager.RecordDriftScore("feature1", 0.75)

	metrics, _ := manager.GetChangeMetrics("feature1")
	if metrics.DriftScore != 0.75 {
		t.Errorf("Expected drift score 0.75, got %f", metrics.DriftScore)
	}
}

func TestManager_GetRecommendedTTL(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())
	defer manager.Stop()

	ttl := manager.GetRecommendedTTL("any_feature")

	if ttl <= 0 {
		t.Error("Expected positive TTL")
	}
}

func TestManager_GetTTLWithReason(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())
	defer manager.Stop()

	result := manager.GetTTLWithReason("any_feature")

	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}
	if result.TTL <= 0 {
		t.Error("Expected positive TTL")
	}
	if result.Reason == "" {
		t.Error("Expected non-empty reason")
	}
}

func TestManager_GetPrediction(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())
	defer manager.Stop()

	prediction := manager.GetPrediction("any_feature")

	if prediction == nil {
		t.Fatal("Expected prediction to be non-nil")
	}
}

func TestManager_GetAllMetrics(t *testing.T) {
	config := DefaultManagerConfig()
	config.Monitor.CleanupInterval = 1 * time.Hour
	manager := NewManager(config)
	defer manager.Stop()

	manager.RecordAccess("feature1", 10*time.Millisecond, true)
	manager.RecordAccess("feature2", 20*time.Millisecond, false)
	manager.RecordChange("feature1", 100.0, 110.0)

	metrics := manager.GetAllMetrics()

	if len(metrics) != 2 {
		t.Errorf("Expected 2 features, got %d", len(metrics))
	}

	if m, ok := metrics["feature1"]; ok {
		if m.Change == nil {
			t.Error("Expected change metrics for feature1")
		}
	}
}

func TestManager_PolicyCRUD(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())
	defer manager.Stop()

	// Create
	policy := NewFixedPolicy("test", "Test", "*", 5*time.Minute, 10)
	err := manager.RegisterPolicy(policy)
	if err != nil {
		t.Fatalf("Failed to register policy: %v", err)
	}

	// Read
	retrieved, err := manager.GetPolicy("test")
	if err != nil {
		t.Fatalf("Failed to get policy: %v", err)
	}
	if retrieved.Name != "Test" {
		t.Errorf("Expected name 'Test', got '%s'", retrieved.Name)
	}

	// Update
	policy.Config.FixedTTL = 10 * time.Minute
	err = manager.UpdatePolicy(policy)
	if err != nil {
		t.Fatalf("Failed to update policy: %v", err)
	}

	// Verify update
	retrieved, _ = manager.GetPolicy("test")
	if retrieved.Config.FixedTTL != 10*time.Minute {
		t.Errorf("Expected updated TTL 10m, got %v", retrieved.Config.FixedTTL)
	}

	// List
	policies := manager.ListPolicies()
	if len(policies) != 1 {
		t.Errorf("Expected 1 policy, got %d", len(policies))
	}

	// Delete
	err = manager.DeletePolicy("test")
	if err != nil {
		t.Fatalf("Failed to delete policy: %v", err)
	}

	_, err = manager.GetPolicy("test")
	if !errors.Is(err, ErrPolicyNotFound) {
		t.Error("Expected policy to be deleted")
	}
}

func TestManager_PolicyAffectsTTL(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewManager(config)
	defer manager.Stop()

	// Without policy, get default behavior
	ttl1 := manager.GetRecommendedTTL("user_age")

	// Add fixed policy
	policy := NewFixedPolicy("fixed", "Fixed", "user_*", 3*time.Minute, 10)
	_ = manager.RegisterPolicy(policy)

	// Now should get fixed TTL
	ttl2 := manager.GetRecommendedTTL("user_age")

	if ttl2 != 3*time.Minute {
		t.Errorf("Expected TTL 3m with policy, got %v", ttl2)
	}

	// ttl1 and ttl2 should be different (unless by coincidence)
	t.Logf("TTL without policy: %v, with policy: %v", ttl1, ttl2)
}

func TestManager_EvaluateAll(t *testing.T) {
	config := DefaultManagerConfig()
	config.Monitor.CleanupInterval = 1 * time.Hour
	manager := NewManager(config)
	defer manager.Stop()

	manager.RecordAccess("feature1", 10*time.Millisecond, true)
	manager.RecordAccess("feature2", 20*time.Millisecond, false)

	results := manager.EvaluateAll()

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestManager_GetAllPredictions(t *testing.T) {
	config := DefaultManagerConfig()
	config.Monitor.CleanupInterval = 1 * time.Hour
	manager := NewManager(config)
	defer manager.Stop()

	// Generate predictions by accessing features
	manager.GetPrediction("feature1")
	manager.GetPrediction("feature2")
	manager.GetPrediction("feature3")

	predictions := manager.GetAllPredictions()

	if len(predictions) != 3 {
		t.Errorf("Expected 3 predictions, got %d", len(predictions))
	}
}

func TestManager_Stats(t *testing.T) {
	config := DefaultManagerConfig()
	config.Monitor.CleanupInterval = 1 * time.Hour
	manager := NewManager(config)
	defer manager.Stop()

	manager.RecordAccess("feature1", 10*time.Millisecond, true)
	manager.RecordChange("feature1", 100.0, 110.0)
	_ = manager.RegisterPolicy(NewFixedPolicy("test", "Test", "*", 5*time.Minute, 10))

	stats := manager.Stats()

	if stats.Monitor.TotalAccesses != 1 {
		t.Errorf("Expected 1 access, got %d", stats.Monitor.TotalAccesses)
	}
	if stats.Monitor.TotalUpdates != 1 {
		t.Errorf("Expected 1 update, got %d", stats.Monitor.TotalUpdates)
	}
	if stats.Policies != 1 {
		t.Errorf("Expected 1 policy, got %d", stats.Policies)
	}
}

func TestManager_IntegrationScenario(t *testing.T) {
	// Full integration test simulating real usage
	config := DefaultManagerConfig()
	config.Monitor.CleanupInterval = 1 * time.Hour
	manager := NewManager(config)
	defer manager.Stop()

	// Register an adaptive policy for high-traffic features
	adaptivePolicy := NewAdaptivePolicy("adaptive", "Adaptive", "high_traffic_*",
		1*time.Second, 5*time.Minute, 100)
	_ = manager.RegisterPolicy(adaptivePolicy)

	// Register a fixed policy for stable features
	fixedPolicy := NewFixedPolicy("stable", "Stable", "stable_*",
		10*time.Minute, 50)
	_ = manager.RegisterPolicy(fixedPolicy)

	// Simulate high traffic feature with good cache hits
	for i := 0; i < 100; i++ {
		manager.RecordAccess("high_traffic_feature", 5*time.Millisecond, true)
	}

	// Simulate stable feature with few changes
	manager.RecordAccess("stable_feature", 10*time.Millisecond, true)

	// Simulate volatile feature
	for i := 0; i < 10; i++ {
		manager.RecordChange("volatile_feature", float64(i*100), float64((i+1)*100))
	}
	manager.RecordDriftScore("volatile_feature", 0.8)

	// Get TTL recommendations
	highTrafficTTL := manager.GetRecommendedTTL("high_traffic_feature")
	stableTTL := manager.GetRecommendedTTL("stable_feature")
	volatileTTL := manager.GetRecommendedTTL("volatile_feature")

	t.Logf("High traffic TTL: %v", highTrafficTTL)
	t.Logf("Stable TTL: %v", stableTTL)
	t.Logf("Volatile TTL: %v", volatileTTL)

	// Stable should have fixed 10m TTL
	if stableTTL != 10*time.Minute {
		t.Errorf("Expected stable TTL 10m, got %v", stableTTL)
	}

	// High traffic should be within adaptive bounds
	if highTrafficTTL < 1*time.Second || highTrafficTTL > 5*time.Minute {
		t.Errorf("High traffic TTL %v outside adaptive bounds", highTrafficTTL)
	}

	// Verify stats
	stats := manager.Stats()
	if stats.Policies != 2 {
		t.Errorf("Expected 2 policies, got %d", stats.Policies)
	}
}
