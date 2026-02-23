package feastcompat

import (
	"testing"
)

func TestNewFeatureServiceManager(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestFeatureServiceCreate(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	svc, err := m.Create("fraud_detection", []string{"user_features", "tx_features"}, "Fraud detection service")
	if err != nil {
		t.Fatal(err)
	}
	if svc.Name != "fraud_detection" {
		t.Errorf("expected name fraud_detection, got %s", svc.Name)
	}
	if len(svc.FeatureViews) != 2 {
		t.Errorf("expected 2 views, got %d", len(svc.FeatureViews))
	}
}

func TestFeatureServiceCreateDuplicate(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, _ = m.Create("svc1", []string{"v1"}, "")
	_, err := m.Create("svc1", []string{"v2"}, "")
	if err == nil {
		t.Fatal("expected error for duplicate service")
	}
}

func TestFeatureServiceCreateEmptyName(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, err := m.Create("", []string{"v1"}, "")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestFeatureServiceMaxServices(t *testing.T) {
	m := NewFeatureServiceManager(FeatureServiceConfig{MaxServices: 2, EnableVersioning: true})
	_, _ = m.Create("svc1", []string{"v1"}, "")
	_, _ = m.Create("svc2", []string{"v2"}, "")
	_, err := m.Create("svc3", []string{"v3"}, "")
	if err == nil {
		t.Fatal("expected error for max services reached")
	}
}

func TestFeatureServiceGet(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, _ = m.Create("svc1", []string{"v1"}, "test")

	svc, err := m.Get("svc1")
	if err != nil {
		t.Fatal(err)
	}
	if svc.Description != "test" {
		t.Errorf("expected description 'test', got %q", svc.Description)
	}
}

func TestFeatureServiceGetNotFound(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, err := m.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent service")
	}
}

func TestFeatureServiceList(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, _ = m.Create("svc1", []string{"v1"}, "")
	_, _ = m.Create("svc2", []string{"v2"}, "")

	services := m.List()
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
}

func TestFeatureServiceUpdate(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, _ = m.Create("svc1", []string{"v1"}, "original")

	svc, err := m.Update("svc1", []string{"v1", "v2", "v3"})
	if err != nil {
		t.Fatal(err)
	}
	if len(svc.FeatureViews) != 3 {
		t.Errorf("expected 3 views after update, got %d", len(svc.FeatureViews))
	}

	stats := m.Stats()
	if stats.TotalUpdates != 1 {
		t.Errorf("expected 1 update, got %d", stats.TotalUpdates)
	}
}

func TestFeatureServiceUpdateNotFound(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, err := m.Update("nonexistent", []string{"v1"})
	if err == nil {
		t.Fatal("expected error for nonexistent service")
	}
}

func TestFeatureServiceDelete(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, _ = m.Create("svc1", []string{"v1"}, "")

	if err := m.Delete("svc1"); err != nil {
		t.Fatal(err)
	}

	_, err := m.Get("svc1")
	if err == nil {
		t.Fatal("expected error after delete")
	}

	stats := m.Stats()
	if stats.TotalServices != 0 {
		t.Errorf("expected 0 services after delete, got %d", stats.TotalServices)
	}
}

func TestFeatureServiceDeleteNotFound(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	err := m.Delete("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent service")
	}
}

func TestFeatureServiceVersioning(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, _ = m.Create("svc1", []string{"v1"}, "initial")
	_, _ = m.Update("svc1", []string{"v1", "v2"})
	_, _ = m.Update("svc1", []string{"v1", "v2", "v3"})

	v1, err := m.GetVersion("svc1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(v1.Views) != 1 {
		t.Errorf("version 1 should have 1 view, got %d", len(v1.Views))
	}

	v2, err := m.GetVersion("svc1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(v2.Views) != 2 {
		t.Errorf("version 2 should have 2 views, got %d", len(v2.Views))
	}

	stats := m.Stats()
	if stats.TotalVersions != 3 {
		t.Errorf("expected 3 total versions, got %d", stats.TotalVersions)
	}
}

func TestFeatureServiceGetVersionNotFound(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, _ = m.Create("svc1", []string{"v1"}, "")

	_, err := m.GetVersion("svc1", 99)
	if err == nil {
		t.Fatal("expected error for nonexistent version")
	}

	_, err = m.GetVersion("nonexistent", 1)
	if err == nil {
		t.Fatal("expected error for nonexistent service")
	}
}

func TestFeatureServiceRollback(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, _ = m.Create("svc1", []string{"v1"}, "initial")
	_, _ = m.Update("svc1", []string{"v1", "v2"})
	_, _ = m.Update("svc1", []string{"v1", "v2", "v3"})

	if err := m.Rollback("svc1", 1); err != nil {
		t.Fatal(err)
	}

	svc, _ := m.Get("svc1")
	if len(svc.FeatureViews) != 1 {
		t.Errorf("expected 1 view after rollback, got %d", len(svc.FeatureViews))
	}
	if svc.FeatureViews[0] != "v1" {
		t.Errorf("expected view 'v1' after rollback, got %q", svc.FeatureViews[0])
	}

	stats := m.Stats()
	if stats.TotalRollbacks != 1 {
		t.Errorf("expected 1 rollback, got %d", stats.TotalRollbacks)
	}
	// Rollback creates a new version entry
	if stats.TotalVersions != 4 {
		t.Errorf("expected 4 versions after rollback, got %d", stats.TotalVersions)
	}
}

func TestFeatureServiceRollbackNotFound(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	err := m.Rollback("nonexistent", 1)
	if err == nil {
		t.Fatal("expected error for nonexistent service")
	}
}

func TestFeatureServiceRollbackVersionNotFound(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, _ = m.Create("svc1", []string{"v1"}, "")
	err := m.Rollback("svc1", 99)
	if err == nil {
		t.Fatal("expected error for nonexistent version")
	}
}

func TestFeatureServiceRollbackVersioningDisabled(t *testing.T) {
	m := NewFeatureServiceManager(FeatureServiceConfig{MaxServices: 10, EnableVersioning: false})
	_, _ = m.Create("svc1", []string{"v1"}, "")
	err := m.Rollback("svc1", 1)
	if err == nil {
		t.Fatal("expected error when versioning is disabled")
	}
}

func TestFeatureServiceSearch(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, _ = m.Create("fraud_detection", []string{"v1"}, "Detects fraud")
	_, _ = m.Create("recommendations", []string{"v2"}, "Product recommendations")

	results := m.Search("fraud")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'fraud', got %d", len(results))
	}
	if results[0].Name != "fraud_detection" {
		t.Errorf("expected fraud_detection, got %s", results[0].Name)
	}
}

func TestFeatureServiceSearchDescription(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, _ = m.Create("svc1", []string{"v1"}, "Handles ML scoring")
	_, _ = m.Create("svc2", []string{"v2"}, "Data pipeline")

	results := m.Search("scoring")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'scoring', got %d", len(results))
	}
}

func TestFeatureServiceSearchCaseInsensitive(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, _ = m.Create("FraudService", []string{"v1"}, "")

	results := m.Search("fraud")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for case-insensitive search, got %d", len(results))
	}
}

func TestFeatureServiceSearchNoResults(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, _ = m.Create("svc1", []string{"v1"}, "")

	results := m.Search("nonexistent")
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestFeatureServiceStats(t *testing.T) {
	m := NewFeatureServiceManager(DefaultFeatureServiceConfig())
	_, _ = m.Create("svc1", []string{"v1"}, "")
	_, _ = m.Create("svc2", []string{"v2"}, "")
	_, _ = m.Update("svc1", []string{"v1", "v2"})

	stats := m.Stats()
	if stats.TotalServices != 2 {
		t.Errorf("expected 2 services, got %d", stats.TotalServices)
	}
	if stats.TotalVersions != 3 {
		t.Errorf("expected 3 versions (2 creates + 1 update), got %d", stats.TotalVersions)
	}
	if stats.TotalUpdates != 1 {
		t.Errorf("expected 1 update, got %d", stats.TotalUpdates)
	}
}
