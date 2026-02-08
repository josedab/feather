package freshness

import (
	"errors"
	"testing"
	"time"
)

func TestNewPolicyRegistry(t *testing.T) {
	registry := NewPolicyRegistry()

	if registry == nil {
		t.Fatal("Expected registry to be non-nil")
	}
}

func TestPolicyRegistry_Register(t *testing.T) {
	registry := NewPolicyRegistry()

	policy := NewFixedPolicy("test-policy", "Test Policy", "*", 5*time.Minute, 10)
	err := registry.Register(policy)

	if err != nil {
		t.Fatalf("Failed to register policy: %v", err)
	}
}

func TestPolicyRegistry_Register_EmptyID(t *testing.T) {
	registry := NewPolicyRegistry()

	policy := &Policy{Name: "No ID"}
	err := registry.Register(policy)

	if !errors.Is(err, ErrInvalidPolicy) {
		t.Errorf("Expected ErrInvalidPolicy, got %v", err)
	}
}

func TestPolicyRegistry_Register_InvalidFixed(t *testing.T) {
	registry := NewPolicyRegistry()

	policy := &Policy{
		ID:   "invalid",
		Type: PolicyTypeFixed,
		Config: PolicyConfig{
			FixedTTL: 0, // Invalid
		},
	}
	err := registry.Register(policy)

	if err == nil {
		t.Error("Expected error for invalid fixed policy")
	}
}

func TestPolicyRegistry_Register_InvalidAdaptive(t *testing.T) {
	registry := NewPolicyRegistry()

	policy := &Policy{
		ID:   "invalid",
		Type: PolicyTypeAdaptive,
		Config: PolicyConfig{
			MinTTL: 10 * time.Minute, // Min > Max
			MaxTTL: 5 * time.Minute,
		},
	}
	err := registry.Register(policy)

	if err == nil {
		t.Error("Expected error for invalid adaptive policy")
	}
}

func TestPolicyRegistry_Register_InvalidTime(t *testing.T) {
	registry := NewPolicyRegistry()

	policy := &Policy{
		ID:   "invalid",
		Type: PolicyTypeTime,
		Config: PolicyConfig{
			PeakHoursStart: 25, // Invalid hour
		},
	}
	err := registry.Register(policy)

	if err == nil {
		t.Error("Expected error for invalid time policy")
	}
}

func TestPolicyRegistry_Update(t *testing.T) {
	registry := NewPolicyRegistry()

	policy := NewFixedPolicy("test", "Test", "*", 5*time.Minute, 10)
	_ = registry.Register(policy)

	// Update
	policy.Config.FixedTTL = 10 * time.Minute
	err := registry.Update(policy)

	if err != nil {
		t.Fatalf("Failed to update policy: %v", err)
	}

	// Verify update
	updated, _ := registry.Get("test")
	if updated.Config.FixedTTL != 10*time.Minute {
		t.Errorf("Expected TTL 10m, got %v", updated.Config.FixedTTL)
	}
}

func TestPolicyRegistry_Update_NotFound(t *testing.T) {
	registry := NewPolicyRegistry()

	policy := NewFixedPolicy("nonexistent", "Test", "*", 5*time.Minute, 10)
	err := registry.Update(policy)

	if !errors.Is(err, ErrPolicyNotFound) {
		t.Errorf("Expected ErrPolicyNotFound, got %v", err)
	}
}

func TestPolicyRegistry_Delete(t *testing.T) {
	registry := NewPolicyRegistry()

	policy := NewFixedPolicy("test", "Test", "*", 5*time.Minute, 10)
	_ = registry.Register(policy)

	err := registry.Delete("test")
	if err != nil {
		t.Fatalf("Failed to delete policy: %v", err)
	}

	_, err = registry.Get("test")
	if !errors.Is(err, ErrPolicyNotFound) {
		t.Error("Expected policy to be deleted")
	}
}

func TestPolicyRegistry_Delete_NotFound(t *testing.T) {
	registry := NewPolicyRegistry()

	err := registry.Delete("nonexistent")
	if !errors.Is(err, ErrPolicyNotFound) {
		t.Errorf("Expected ErrPolicyNotFound, got %v", err)
	}
}

func TestPolicyRegistry_Get(t *testing.T) {
	registry := NewPolicyRegistry()

	policy := NewFixedPolicy("test", "Test Policy", "*", 5*time.Minute, 10)
	_ = registry.Register(policy)

	retrieved, err := registry.Get("test")
	if err != nil {
		t.Fatalf("Failed to get policy: %v", err)
	}

	if retrieved.Name != "Test Policy" {
		t.Errorf("Expected name 'Test Policy', got '%s'", retrieved.Name)
	}
}

func TestPolicyRegistry_Get_NotFound(t *testing.T) {
	registry := NewPolicyRegistry()

	_, err := registry.Get("nonexistent")
	if !errors.Is(err, ErrPolicyNotFound) {
		t.Errorf("Expected ErrPolicyNotFound, got %v", err)
	}
}

func TestPolicyRegistry_List(t *testing.T) {
	registry := NewPolicyRegistry()

	_ = registry.Register(NewFixedPolicy("p1", "Policy 1", "*", 5*time.Minute, 10))
	_ = registry.Register(NewFixedPolicy("p2", "Policy 2", "*", 10*time.Minute, 20))

	policies := registry.List()
	if len(policies) != 2 {
		t.Errorf("Expected 2 policies, got %d", len(policies))
	}
}

func TestPolicyRegistry_FindPolicies(t *testing.T) {
	registry := NewPolicyRegistry()

	_ = registry.Register(NewFixedPolicy("all", "All Features", "*", 5*time.Minute, 10))
	_ = registry.Register(NewFixedPolicy("user", "User Features", "user_*", 1*time.Minute, 20))
	_ = registry.Register(NewFixedPolicy("product", "Product Features", "product_*", 10*time.Minute, 15))

	// Should match user_* pattern and * pattern
	policies := registry.FindPolicies("user_age")
	if len(policies) != 2 {
		t.Errorf("Expected 2 matching policies, got %d", len(policies))
	}

	// Higher priority first
	if policies[0].Priority != 20 {
		t.Errorf("Expected highest priority policy first, got priority %d", policies[0].Priority)
	}
}

func TestPolicyRegistry_GetEffectivePolicy(t *testing.T) {
	registry := NewPolicyRegistry()

	_ = registry.Register(NewFixedPolicy("all", "All Features", "*", 5*time.Minute, 10))
	_ = registry.Register(NewFixedPolicy("user", "User Features", "user_*", 1*time.Minute, 20))

	// Should get user policy for user_* features
	policy := registry.GetEffectivePolicy("user_age")
	if policy == nil {
		t.Fatal("Expected to find effective policy")
	}
	if policy.ID != "user" {
		t.Errorf("Expected 'user' policy, got '%s'", policy.ID)
	}

	// Should get 'all' policy for other features
	policy = registry.GetEffectivePolicy("other_feature")
	if policy == nil {
		t.Fatal("Expected to find effective policy")
	}
	if policy.ID != "all" {
		t.Errorf("Expected 'all' policy, got '%s'", policy.ID)
	}
}

func TestPolicyRegistry_DisabledPolicy(t *testing.T) {
	registry := NewPolicyRegistry()

	policy := NewFixedPolicy("test", "Test", "*", 5*time.Minute, 100)
	policy.Enabled = false
	_ = registry.Register(policy)

	policies := registry.FindPolicies("any_feature")
	if len(policies) != 0 {
		t.Error("Expected no matching policies when policy is disabled")
	}
}

func TestPolicyEvaluator_NoPolicy(t *testing.T) {
	registry := NewPolicyRegistry()
	monitor := NewMonitor(DefaultMonitorConfig())
	predictor := NewPredictor(DefaultPredictorConfig(), monitor)
	defer predictor.Stop()

	evaluator := NewPolicyEvaluator(registry, monitor, predictor)

	result := evaluator.Evaluate("some_feature")

	if result == nil {
		t.Fatal("Expected evaluation result")
	}
	// Should fall back to predictor
	if result.TTL <= 0 {
		t.Error("Expected positive TTL")
	}
}

func TestPolicyEvaluator_FixedPolicy(t *testing.T) {
	registry := NewPolicyRegistry()
	monitor := NewMonitor(DefaultMonitorConfig())
	predictor := NewPredictor(DefaultPredictorConfig(), monitor)
	defer predictor.Stop()

	_ = registry.Register(NewFixedPolicy("fixed", "Fixed", "*", 3*time.Minute, 10))

	evaluator := NewPolicyEvaluator(registry, monitor, predictor)

	result := evaluator.Evaluate("any_feature")

	if result.TTL != 3*time.Minute {
		t.Errorf("Expected TTL 3m, got %v", result.TTL)
	}
	if result.PolicyType != PolicyTypeFixed {
		t.Errorf("Expected fixed policy type, got %s", result.PolicyType)
	}
}

func TestPolicyEvaluator_AdaptivePolicy(t *testing.T) {
	registry := NewPolicyRegistry()
	monitorConfig := DefaultMonitorConfig()
	monitorConfig.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(monitorConfig)
	predictor := NewPredictor(DefaultPredictorConfig(), monitor)
	defer predictor.Stop()

	_ = registry.Register(NewAdaptivePolicy("adaptive", "Adaptive", "*", 1*time.Second, 10*time.Minute, 10))

	evaluator := NewPolicyEvaluator(registry, monitor, predictor)

	result := evaluator.Evaluate("any_feature")

	if result.TTL < 1*time.Second || result.TTL > 10*time.Minute {
		t.Errorf("Expected TTL between 1s and 10m, got %v", result.TTL)
	}
	if result.PolicyType != PolicyTypeAdaptive {
		t.Errorf("Expected adaptive policy type, got %s", result.PolicyType)
	}
}

func TestPolicyEvaluator_TimePolicy(t *testing.T) {
	registry := NewPolicyRegistry()
	monitor := NewMonitor(DefaultMonitorConfig())
	predictor := NewPredictor(DefaultPredictorConfig(), monitor)
	defer predictor.Stop()

	// Create policy that covers current hour
	currentHour := time.Now().Hour()
	peakStart := currentHour
	peakEnd := (currentHour + 1) % 24

	policy := NewTimePolicy("time", "Time", "*", peakStart, peakEnd, 1*time.Minute, 10*time.Minute, 10)
	_ = registry.Register(policy)

	evaluator := NewPolicyEvaluator(registry, monitor, predictor)

	result := evaluator.Evaluate("any_feature")

	if result.PolicyType != PolicyTypeTime {
		t.Errorf("Expected time policy type, got %s", result.PolicyType)
	}
	// Should be peak TTL
	if result.TTL != 1*time.Minute {
		t.Errorf("Expected peak TTL 1m, got %v", result.TTL)
	}
}

func TestPolicyEvaluator_ThresholdPolicy(t *testing.T) {
	registry := NewPolicyRegistry()
	monitorConfig := DefaultMonitorConfig()
	monitorConfig.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(monitorConfig)
	predictor := NewPredictor(DefaultPredictorConfig(), monitor)
	defer predictor.Stop()

	policy := NewThresholdPolicy("threshold", "Threshold", "*",
		10.0, 1*time.Minute, 5*time.Minute, // Access threshold
		0.5, 30*time.Second, // Drift threshold
		10)
	_ = registry.Register(policy)

	evaluator := NewPolicyEvaluator(registry, monitor, predictor)

	// Without metrics, should default to low access TTL
	result := evaluator.Evaluate("any_feature")

	if result.PolicyType != PolicyTypeThreshold {
		t.Errorf("Expected threshold policy type, got %s", result.PolicyType)
	}
}

func TestPolicyEvaluator_ThresholdPolicy_HighDrift(t *testing.T) {
	registry := NewPolicyRegistry()
	monitorConfig := DefaultMonitorConfig()
	monitorConfig.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(monitorConfig)
	predictor := NewPredictor(DefaultPredictorConfig(), monitor)
	defer predictor.Stop()

	policy := NewThresholdPolicy("threshold", "Threshold", "*",
		10.0, 1*time.Minute, 5*time.Minute,
		0.5, 30*time.Second,
		10)
	_ = registry.Register(policy)

	// Record high drift
	monitor.RecordDriftScore("drifting_feature", 0.8)

	evaluator := NewPolicyEvaluator(registry, monitor, predictor)

	result := evaluator.Evaluate("drifting_feature")

	// Should use high drift TTL
	if result.TTL != 30*time.Second {
		t.Errorf("Expected high drift TTL 30s, got %v", result.TTL)
	}
}

func TestPolicyEvaluator_EvaluateAll(t *testing.T) {
	registry := NewPolicyRegistry()
	monitorConfig := DefaultMonitorConfig()
	monitorConfig.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(monitorConfig)
	predictor := NewPredictor(DefaultPredictorConfig(), monitor)
	defer predictor.Stop()

	_ = registry.Register(NewFixedPolicy("all", "All", "*", 5*time.Minute, 10))

	// Track some features
	monitor.RecordAccess("feature1", 10*time.Millisecond, true)
	monitor.RecordAccess("feature2", 20*time.Millisecond, false)

	evaluator := NewPolicyEvaluator(registry, monitor, predictor)

	results := evaluator.EvaluateAll()

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"*", "anything", true},
		{"", "anything", true},
		{"user_*", "user_age", true},
		{"user_*", "product_price", false},
		{"*_age", "user_age", true},
		{"*_age", "user_price", false},
		{"exact_match", "exact_match", true},
		{"exact_match", "different", false},
	}

	for _, tt := range tests {
		got := matchPattern(tt.pattern, tt.name)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
		}
	}
}

func TestSortPolicies(t *testing.T) {
	policies := []*Policy{
		{ID: "low", Priority: 10},
		{ID: "high", Priority: 100},
		{ID: "mid", Priority: 50},
	}

	sortPolicies(policies)

	if policies[0].ID != "high" || policies[1].ID != "mid" || policies[2].ID != "low" {
		t.Error("Policies not sorted by priority (descending)")
	}
}

func TestClampDuration(t *testing.T) {
	minD := 1 * time.Second
	maxD := 10 * time.Second

	if clampDuration(5*time.Second, minD, maxD) != 5*time.Second {
		t.Error("Should not clamp value within range")
	}
	if clampDuration(0, minD, maxD) != minD {
		t.Error("Should clamp to minimum")
	}
	if clampDuration(20*time.Second, minD, maxD) != maxD {
		t.Error("Should clamp to maximum")
	}
}

func TestNewFixedPolicy(t *testing.T) {
	policy := NewFixedPolicy("test", "Test", "user_*", 5*time.Minute, 10)

	if policy.ID != "test" {
		t.Errorf("Expected ID 'test', got '%s'", policy.ID)
	}
	if policy.Type != PolicyTypeFixed {
		t.Errorf("Expected type fixed, got %s", policy.Type)
	}
	if policy.Config.FixedTTL != 5*time.Minute {
		t.Errorf("Expected TTL 5m, got %v", policy.Config.FixedTTL)
	}
}

func TestNewAdaptivePolicy(t *testing.T) {
	policy := NewAdaptivePolicy("test", "Test", "*", 1*time.Second, 1*time.Hour, 10)

	if policy.Type != PolicyTypeAdaptive {
		t.Errorf("Expected type adaptive, got %s", policy.Type)
	}
	if policy.Config.MinTTL != 1*time.Second {
		t.Errorf("Expected min TTL 1s, got %v", policy.Config.MinTTL)
	}
	if policy.Config.MaxTTL != 1*time.Hour {
		t.Errorf("Expected max TTL 1h, got %v", policy.Config.MaxTTL)
	}
}

func TestNewTimePolicy(t *testing.T) {
	policy := NewTimePolicy("test", "Test", "*", 9, 17, 1*time.Minute, 10*time.Minute, 10)

	if policy.Type != PolicyTypeTime {
		t.Errorf("Expected type time, got %s", policy.Type)
	}
	if policy.Config.PeakHoursStart != 9 {
		t.Errorf("Expected peak start 9, got %d", policy.Config.PeakHoursStart)
	}
	if policy.Config.PeakHoursEnd != 17 {
		t.Errorf("Expected peak end 17, got %d", policy.Config.PeakHoursEnd)
	}
}

func TestNewThresholdPolicy(t *testing.T) {
	policy := NewThresholdPolicy("test", "Test", "*",
		100.0, 1*time.Minute, 5*time.Minute,
		0.5, 30*time.Second,
		10)

	if policy.Type != PolicyTypeThreshold {
		t.Errorf("Expected type threshold, got %s", policy.Type)
	}
	if policy.Config.AccessRateThreshold != 100.0 {
		t.Errorf("Expected access threshold 100, got %f", policy.Config.AccessRateThreshold)
	}
	if policy.Config.DriftThreshold != 0.5 {
		t.Errorf("Expected drift threshold 0.5, got %f", policy.Config.DriftThreshold)
	}
}
