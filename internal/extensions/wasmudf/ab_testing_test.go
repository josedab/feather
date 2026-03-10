package wasmudf

import (
	"testing"
)

func TestABTestManager_CreateTest(t *testing.T) {
	m := NewABTestManager()

	config := ABTestConfig{
		ModuleID:       "mod-1",
		VersionA:       "v1",
		VersionB:       "v2",
		TrafficPercent: 0.3,
		Enabled:        true,
	}

	result, err := m.CreateTest(config)
	if err != nil {
		t.Fatalf("CreateTest failed: %v", err)
	}
	if result.Config.ModuleID != "mod-1" {
		t.Errorf("expected module_id=mod-1, got %s", result.Config.ModuleID)
	}
	if result.StartedAt.IsZero() {
		t.Error("expected non-zero started_at")
	}
}

func TestABTestManager_CreateTest_Validation(t *testing.T) {
	m := NewABTestManager()

	tests := []struct {
		name   string
		config ABTestConfig
	}{
		{"empty module_id", ABTestConfig{VersionA: "v1", VersionB: "v2", TrafficPercent: 0.5}},
		{"empty version_a", ABTestConfig{ModuleID: "m", VersionB: "v2", TrafficPercent: 0.5}},
		{"empty version_b", ABTestConfig{ModuleID: "m", VersionA: "v1", TrafficPercent: 0.5}},
		{"negative traffic", ABTestConfig{ModuleID: "m", VersionA: "v1", VersionB: "v2", TrafficPercent: -0.1}},
		{"traffic over 1", ABTestConfig{ModuleID: "m", VersionA: "v1", VersionB: "v2", TrafficPercent: 1.1}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := m.CreateTest(tc.config)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestABTestManager_CreateTest_Duplicate(t *testing.T) {
	m := NewABTestManager()

	config := ABTestConfig{ModuleID: "mod-1", VersionA: "v1", VersionB: "v2", TrafficPercent: 0.5, Enabled: true}
	_, _ = m.CreateTest(config)

	_, err := m.CreateTest(config)
	if err == nil {
		t.Error("expected error for duplicate test")
	}
}

func TestABTestManager_ResolveVersion_Deterministic(t *testing.T) {
	m := NewABTestManager()

	_, _ = m.CreateTest(ABTestConfig{
		ModuleID:       "mod-1",
		VersionA:       "v1",
		VersionB:       "v2",
		TrafficPercent: 0.5,
		Enabled:        true,
	})

	// Same entity key should always resolve to the same version
	version1 := m.ResolveVersion("mod-1", "user-123")
	version2 := m.ResolveVersion("mod-1", "user-123")
	if version1 != version2 {
		t.Errorf("expected deterministic resolution, got %s and %s", version1, version2)
	}

	// Result should be either v1 or v2
	if version1 != "v1" && version1 != "v2" {
		t.Errorf("expected v1 or v2, got %s", version1)
	}
}

func TestABTestManager_ResolveVersion_NoTest(t *testing.T) {
	m := NewABTestManager()

	version := m.ResolveVersion("nonexistent", "user-1")
	if version != "" {
		t.Errorf("expected empty version for no test, got %s", version)
	}
}

func TestABTestManager_ResolveVersion_Disabled(t *testing.T) {
	m := NewABTestManager()

	_, _ = m.CreateTest(ABTestConfig{
		ModuleID:       "mod-1",
		VersionA:       "v1",
		VersionB:       "v2",
		TrafficPercent: 0.5,
		Enabled:        false, // disabled
	})

	version := m.ResolveVersion("mod-1", "user-1")
	if version != "" {
		t.Errorf("expected empty version for disabled test, got %s", version)
	}
}

func TestABTestManager_ResolveVersion_Distribution(t *testing.T) {
	m := NewABTestManager()

	_, _ = m.CreateTest(ABTestConfig{
		ModuleID:       "mod-dist",
		VersionA:       "v1",
		VersionB:       "v2",
		TrafficPercent: 0.5,
		Enabled:        true,
	})

	countA, countB := 0, 0
	for i := 0; i < 1000; i++ {
		key := "entity-" + string(rune('A'+i%26)) + string(rune('0'+i%10))
		v := m.ResolveVersion("mod-dist", key)
		if v == "v1" {
			countA++
		} else {
			countB++
		}
	}

	// With 50/50 split, both should have significant traffic
	if countA == 0 || countB == 0 {
		t.Errorf("expected distribution across both versions, got A=%d B=%d", countA, countB)
	}
}

func TestABTestManager_RecordExecution(t *testing.T) {
	m := NewABTestManager()

	_, _ = m.CreateTest(ABTestConfig{
		ModuleID:       "mod-1",
		VersionA:       "v1",
		VersionB:       "v2",
		TrafficPercent: 0.5,
		Enabled:        true,
	})

	m.RecordExecution("mod-1", "v1", 10.0, false)
	m.RecordExecution("mod-1", "v1", 20.0, false)
	m.RecordExecution("mod-1", "v2", 15.0, true)

	test, err := m.GetTest("mod-1")
	if err != nil {
		t.Fatalf("GetTest failed: %v", err)
	}
	if test.VersionAStats.Executions != 2 {
		t.Errorf("expected 2 executions for A, got %d", test.VersionAStats.Executions)
	}
	if test.VersionAStats.AvgLatency != 15.0 {
		t.Errorf("expected avg latency 15.0 for A, got %f", test.VersionAStats.AvgLatency)
	}
	if test.VersionBStats.Executions != 1 {
		t.Errorf("expected 1 execution for B, got %d", test.VersionBStats.Executions)
	}
	if test.VersionBStats.Errors != 1 {
		t.Errorf("expected 1 error for B, got %d", test.VersionBStats.Errors)
	}
}

func TestABTestManager_RecordExecution_NoTest(t *testing.T) {
	m := NewABTestManager()
	// Should not panic
	m.RecordExecution("nonexistent", "v1", 10.0, false)
}

func TestABTestManager_EndTest(t *testing.T) {
	m := NewABTestManager()

	_, _ = m.CreateTest(ABTestConfig{
		ModuleID:       "mod-1",
		VersionA:       "v1",
		VersionB:       "v2",
		TrafficPercent: 0.5,
		Enabled:        true,
	})

	// A has lower error rate
	m.RecordExecution("mod-1", "v1", 10.0, false)
	m.RecordExecution("mod-1", "v1", 10.0, false)
	m.RecordExecution("mod-1", "v2", 10.0, true)

	result, err := m.EndTest("mod-1")
	if err != nil {
		t.Fatalf("EndTest failed: %v", err)
	}
	if result.Winner != "A" {
		t.Errorf("expected winner A, got %s", result.Winner)
	}
	if result.Config.Enabled {
		t.Error("expected test to be disabled after ending")
	}

	// Test should be removed
	_, err = m.GetTest("mod-1")
	if err == nil {
		t.Error("expected error after test ended")
	}
}

func TestABTestManager_EndTest_LatencyTiebreaker(t *testing.T) {
	m := NewABTestManager()

	_, _ = m.CreateTest(ABTestConfig{
		ModuleID:       "mod-lat",
		VersionA:       "v1",
		VersionB:       "v2",
		TrafficPercent: 0.5,
		Enabled:        true,
	})

	// Same error rate, but B has lower latency
	m.RecordExecution("mod-lat", "v1", 20.0, false)
	m.RecordExecution("mod-lat", "v2", 10.0, false)

	result, err := m.EndTest("mod-lat")
	if err != nil {
		t.Fatalf("EndTest failed: %v", err)
	}
	if result.Winner != "B" {
		t.Errorf("expected winner B (lower latency), got %s", result.Winner)
	}
}

func TestABTestManager_EndTest_NotFound(t *testing.T) {
	m := NewABTestManager()
	_, err := m.EndTest("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent test")
	}
}

func TestABTestManager_GetTest_NotFound(t *testing.T) {
	m := NewABTestManager()
	_, err := m.GetTest("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent test")
	}
}

func TestABTestManager_ListTests(t *testing.T) {
	m := NewABTestManager()

	_, _ = m.CreateTest(ABTestConfig{ModuleID: "m1", VersionA: "v1", VersionB: "v2", TrafficPercent: 0.5, Enabled: true})
	_, _ = m.CreateTest(ABTestConfig{ModuleID: "m2", VersionA: "v1", VersionB: "v2", TrafficPercent: 0.3, Enabled: true})

	tests := m.ListTests()
	if len(tests) != 2 {
		t.Errorf("expected 2 tests, got %d", len(tests))
	}
}
