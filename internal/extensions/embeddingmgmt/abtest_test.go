package embeddingmgmt

import (
	"errors"
	"testing"
	"time"
)

func TestNewABTester(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	tester := NewABTester(m)
	if tester == nil {
		t.Fatal("expected non-nil A/B tester")
	}
}

func TestABTestCreateAndList(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	tester := NewABTester(m)

	cfg := ABTestConfig{
		Name:         "test-1",
		ModelA:       "model-a",
		ModelB:       "model-b",
		TrafficSplit: 0.5,
		Collection:   "docs",
		Metrics:      []string{"latency", "similarity"},
	}

	result, err := tester.CreateTest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "test-1" {
		t.Errorf("expected test name 'test-1', got %s", result.Name)
	}
	if result.StartedAt.IsZero() {
		t.Error("expected non-zero StartedAt")
	}

	tests := tester.ListTests()
	if len(tests) != 1 {
		t.Fatalf("expected 1 test, got %d", len(tests))
	}
}

func TestABTestDuplicateName(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	tester := NewABTester(m)

	cfg := ABTestConfig{Name: "test-1", ModelA: "a", ModelB: "b", TrafficSplit: 0.5}
	_, _ = tester.CreateTest(cfg)

	_, err := tester.CreateTest(cfg)
	if !errors.Is(err, ErrTestExists) {
		t.Fatalf("expected ErrTestExists, got %v", err)
	}
}

func TestABTestEmptyName(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	tester := NewABTester(m)

	_, err := tester.CreateTest(ABTestConfig{Name: ""})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestABTestInvalidTrafficSplit(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	tester := NewABTester(m)

	_, err := tester.CreateTest(ABTestConfig{Name: "test", TrafficSplit: 1.5})
	if err == nil {
		t.Error("expected error for invalid traffic split")
	}

	_, err = tester.CreateTest(ABTestConfig{Name: "test", TrafficSplit: -0.1})
	if err == nil {
		t.Error("expected error for negative traffic split")
	}
}

func TestABTestRouteQuery(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	tester := NewABTester(m)

	// 100% to Model A
	_, _ = tester.CreateTest(ABTestConfig{
		Name:         "all-a",
		ModelA:       "model-a",
		ModelB:       "model-b",
		TrafficSplit: 0.0,
	})

	for i := 0; i < 10; i++ {
		result := tester.RouteQuery("all-a")
		if result != "model-a" {
			t.Errorf("expected model-a with 0.0 split, got %s", result)
		}
	}

	// 100% to Model B
	_, _ = tester.CreateTest(ABTestConfig{
		Name:         "all-b",
		ModelA:       "model-a",
		ModelB:       "model-b",
		TrafficSplit: 1.0,
	})

	for i := 0; i < 10; i++ {
		result := tester.RouteQuery("all-b")
		if result != "model-b" {
			t.Errorf("expected model-b with 1.0 split, got %s", result)
		}
	}
}

func TestABTestRouteNonexistent(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	tester := NewABTester(m)

	result := tester.RouteQuery("nonexistent")
	if result != "" {
		t.Errorf("expected empty string for nonexistent test, got %s", result)
	}
}

func TestABTestRecordAndGetResults(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	tester := NewABTester(m)

	_, _ = tester.CreateTest(ABTestConfig{
		Name:         "test-1",
		ModelA:       "model-a",
		ModelB:       "model-b",
		TrafficSplit: 0.5,
	})

	// Record results for model A
	for i := 0; i < 100; i++ {
		tester.RecordResult("test-1", "model-a", 10*time.Millisecond, 0.95, nil)
	}

	// Record results for model B
	for i := 0; i < 100; i++ {
		tester.RecordResult("test-1", "model-b", 15*time.Millisecond, 0.85, nil)
	}

	results, err := tester.GetResults("test-1")
	if err != nil {
		t.Fatal(err)
	}

	if results.ModelAStats.QueryCount != 100 {
		t.Errorf("expected 100 queries for model A, got %d", results.ModelAStats.QueryCount)
	}
	if results.ModelBStats.QueryCount != 100 {
		t.Errorf("expected 100 queries for model B, got %d", results.ModelBStats.QueryCount)
	}
	if results.SampleSize != [2]int{100, 100} {
		t.Errorf("expected sample size [100, 100], got %v", results.SampleSize)
	}

	// Model A has higher similarity, should be winner with enough samples
	if results.Winner != "model-a" {
		t.Errorf("expected model-a as winner, got %s", results.Winner)
	}
	if results.Confidence < 0.95 {
		t.Errorf("expected confidence >= 0.95, got %f", results.Confidence)
	}
}

func TestABTestErrorRate(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	tester := NewABTester(m)

	_, _ = tester.CreateTest(ABTestConfig{
		Name:         "errors",
		ModelA:       "model-a",
		ModelB:       "model-b",
		TrafficSplit: 0.5,
	})

	tester.RecordResult("errors", "model-a", 10*time.Millisecond, 0.9, nil)
	tester.RecordResult("errors", "model-a", 10*time.Millisecond, 0.9, errTestError)

	results, _ := tester.GetResults("errors")
	if results.ModelAStats.ErrorRate != 0.5 {
		t.Errorf("expected 0.5 error rate, got %f", results.ModelAStats.ErrorRate)
	}
}

var errTestError = testError("test error")

type testError string

func (e testError) Error() string { return string(e) }

func TestABTestGetResultsNotFound(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	tester := NewABTester(m)

	_, err := tester.GetResults("nonexistent")
	if !errors.Is(err, ErrTestNotFound) {
		t.Fatalf("expected ErrTestNotFound, got %v", err)
	}
}

func TestABTestStopTest(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	tester := NewABTester(m)

	_, _ = tester.CreateTest(ABTestConfig{
		Name:         "stop-me",
		ModelA:       "model-a",
		ModelB:       "model-b",
		TrafficSplit: 0.5,
	})

	err := tester.StopTest("stop-me")
	if err != nil {
		t.Fatal(err)
	}

	// Routing should return empty for stopped test
	result := tester.RouteQuery("stop-me")
	if result != "" {
		t.Errorf("expected empty string for stopped test, got %s", result)
	}
}

func TestABTestStopNonexistent(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	tester := NewABTester(m)

	err := tester.StopTest("nonexistent")
	if !errors.Is(err, ErrTestNotFound) {
		t.Fatalf("expected ErrTestNotFound, got %v", err)
	}
}

func TestABTestRecordIgnoresStoppedTest(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	tester := NewABTester(m)

	_, _ = tester.CreateTest(ABTestConfig{
		Name:         "stopped",
		ModelA:       "model-a",
		ModelB:       "model-b",
		TrafficSplit: 0.5,
	})

	tester.RecordResult("stopped", "model-a", 10*time.Millisecond, 0.9, nil)
	_ = tester.StopTest("stopped")
	tester.RecordResult("stopped", "model-a", 10*time.Millisecond, 0.9, nil)

	results, _ := tester.GetResults("stopped")
	if results.ModelAStats.QueryCount != 1 {
		t.Errorf("expected 1 query after stop, got %d", results.ModelAStats.QueryCount)
	}
}

func TestABTestRecordUnknownModel(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	tester := NewABTester(m)

	_, _ = tester.CreateTest(ABTestConfig{
		Name:         "test",
		ModelA:       "model-a",
		ModelB:       "model-b",
		TrafficSplit: 0.5,
	})

	// Should not panic
	tester.RecordResult("test", "unknown-model", 10*time.Millisecond, 0.9, nil)

	results, _ := tester.GetResults("test")
	if results.ModelAStats.QueryCount != 0 || results.ModelBStats.QueryCount != 0 {
		t.Error("expected 0 queries for both models when recording unknown model")
	}
}
