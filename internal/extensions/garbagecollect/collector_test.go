package garbagecollect

import (
	"testing"
	"time"
)

func TestCollectorPolicyCRUD(t *testing.T) {
	c := NewCollector()

	policy := DefaultGCPolicy()
	if err := c.RegisterPolicy(policy); err != nil {
		t.Fatal(err)
	}

	policies := c.ListPolicies()
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}

	if err := c.DeletePolicy("default"); err != nil {
		t.Fatal(err)
	}

	if len(c.ListPolicies()) != 0 {
		t.Fatal("expected 0 policies after delete")
	}
}

func TestCollectorAnalyze(t *testing.T) {
	c := NewCollector()

	policy := GCPolicy{
		Name:            "test",
		MaxIdleDuration: time.Hour,
		DryRun:          true,
	}
	c.RegisterPolicy(policy)

	// Record access long ago
	c.RecordAccess("old_feature", "group1", 1024)
	c.mu.Lock()
	c.accessLog["group1:old_feature"].LastAccessed = time.Now().Add(-48 * time.Hour)
	c.mu.Unlock()

	// Record recent access
	c.RecordAccess("new_feature", "group1", 512)

	candidates, err := c.Analyze("test")
	if err != nil {
		t.Fatal(err)
	}

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].FeatureName != "old_feature" {
		t.Errorf("expected old_feature candidate, got %s", candidates[0].FeatureName)
	}
}

func TestCollectorRunDryAndLive(t *testing.T) {
	c := NewCollector()

	// Dry run policy
	c.RegisterPolicy(GCPolicy{
		Name:            "dry",
		MaxIdleDuration: time.Hour,
		DryRun:          true,
	})

	// Live policy
	c.RegisterPolicy(GCPolicy{
		Name:            "live",
		MaxIdleDuration: time.Hour,
		DryRun:          false,
	})

	c.RecordAccess("stale", "g1", 2048)
	c.mu.Lock()
	c.accessLog["g1:stale"].LastAccessed = time.Now().Add(-72 * time.Hour)
	c.mu.Unlock()

	// Dry run should not delete
	result, err := c.Run("dry")
	if err != nil {
		t.Fatal(err)
	}
	if result.Candidates != 1 {
		t.Errorf("expected 1 candidate, got %d", result.Candidates)
	}
	if result.Collected != 0 {
		t.Errorf("dry run should not collect, got %d", result.Collected)
	}

	// Live run should delete
	result, err = c.Run("live")
	if err != nil {
		t.Fatal(err)
	}
	if result.Collected != 1 {
		t.Errorf("expected 1 collected, got %d", result.Collected)
	}
	if result.BytesFreed != 2048 {
		t.Errorf("expected 2048 bytes freed, got %d", result.BytesFreed)
	}
}
