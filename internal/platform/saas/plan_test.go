package saas

import (
	"errors"
	"testing"
)

func TestNewPlanRegistry(t *testing.T) {
	registry := NewPlanRegistry()
	if registry == nil {
		t.Fatal("Expected registry to be non-nil")
	}
}

func TestPlanRegistry_DefaultPlans(t *testing.T) {
	registry := NewPlanRegistry()

	plans := registry.ListPlans()
	if len(plans) < 4 {
		t.Errorf("Expected at least 4 default plans, got %d", len(plans))
	}

	// Check free plan exists
	freePlan, err := registry.GetPlan("free")
	if err != nil {
		t.Fatalf("Expected free plan, got error: %v", err)
	}
	if freePlan.Tier != TierFree {
		t.Errorf("Expected tier 'free', got '%s'", freePlan.Tier)
	}
	if freePlan.Pricing.MonthlyPrice != 0 {
		t.Errorf("Expected free plan to have $0 monthly price")
	}

	// Check pro plan exists
	proPlan, err := registry.GetPlan("pro")
	if err != nil {
		t.Fatalf("Expected pro plan, got error: %v", err)
	}
	if proPlan.Pricing.MonthlyPrice <= 0 {
		t.Error("Expected pro plan to have positive price")
	}
}

func TestPlanRegistry_GetPlan_NotFound(t *testing.T) {
	registry := NewPlanRegistry()

	_, err := registry.GetPlan("nonexistent")
	if !errors.Is(err, ErrPlanNotFound) {
		t.Errorf("Expected ErrPlanNotFound, got %v", err)
	}
}

func TestPlanRegistry_RegisterPlan(t *testing.T) {
	registry := NewPlanRegistry()

	customPlan := &Plan{
		ID:   "custom",
		Name: "Custom Plan",
		Tier: TierPro,
		Pricing: PlanPricing{
			MonthlyPrice: 99,
			Currency:     "USD",
		},
		Active: true,
	}

	err := registry.RegisterPlan(customPlan)
	if err != nil {
		t.Fatalf("RegisterPlan failed: %v", err)
	}

	retrieved, err := registry.GetPlan("custom")
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}
	if retrieved.Name != "Custom Plan" {
		t.Errorf("Expected name 'Custom Plan', got '%s'", retrieved.Name)
	}
}

func TestPlanRegistry_RegisterPlan_Duplicate(t *testing.T) {
	registry := NewPlanRegistry()

	plan := &Plan{
		ID:   "dup",
		Name: "Duplicate Plan",
	}

	_ = registry.RegisterPlan(plan)
	err := registry.RegisterPlan(plan)
	if !errors.Is(err, ErrPlanAlreadyExists) {
		t.Errorf("Expected ErrPlanAlreadyExists, got %v", err)
	}
}

func TestPlanRegistry_RegisterPlan_Invalid(t *testing.T) {
	registry := NewPlanRegistry()

	// Missing ID
	plan := &Plan{
		Name: "No ID",
	}
	err := registry.RegisterPlan(plan)
	if !errors.Is(err, ErrInvalidPlan) {
		t.Errorf("Expected ErrInvalidPlan, got %v", err)
	}

	// Missing Name
	plan2 := &Plan{
		ID: "no-name",
	}
	err = registry.RegisterPlan(plan2)
	if !errors.Is(err, ErrInvalidPlan) {
		t.Errorf("Expected ErrInvalidPlan, got %v", err)
	}
}

func TestPlanRegistry_UpdatePlan(t *testing.T) {
	registry := NewPlanRegistry()

	plan, _ := registry.GetPlan("free")
	plan.Description = "Updated description"

	err := registry.UpdatePlan(plan)
	if err != nil {
		t.Fatalf("UpdatePlan failed: %v", err)
	}

	updated, _ := registry.GetPlan("free")
	if updated.Description != "Updated description" {
		t.Errorf("Expected updated description")
	}
}

func TestPlanRegistry_UpdatePlan_NotFound(t *testing.T) {
	registry := NewPlanRegistry()

	plan := &Plan{
		ID:   "nonexistent",
		Name: "Nonexistent",
	}

	err := registry.UpdatePlan(plan)
	if !errors.Is(err, ErrPlanNotFound) {
		t.Errorf("Expected ErrPlanNotFound, got %v", err)
	}
}

func TestPlanRegistry_DeactivatePlan(t *testing.T) {
	registry := NewPlanRegistry()

	err := registry.DeactivatePlan("starter")
	if err != nil {
		t.Fatalf("DeactivatePlan failed: %v", err)
	}

	plan, _ := registry.GetPlan("starter")
	if plan.Active {
		t.Error("Expected plan to be inactive")
	}
}

func TestPlanRegistry_DeactivatePlan_NotFound(t *testing.T) {
	registry := NewPlanRegistry()

	err := registry.DeactivatePlan("nonexistent")
	if !errors.Is(err, ErrPlanNotFound) {
		t.Errorf("Expected ErrPlanNotFound, got %v", err)
	}
}

func TestPlanRegistry_ComparePlans(t *testing.T) {
	registry := NewPlanRegistry()

	comparison, err := registry.ComparePlans("free", "pro")
	if err != nil {
		t.Fatalf("ComparePlans failed: %v", err)
	}

	if comparison.Plan1.ID != "free" {
		t.Errorf("Expected plan1 to be 'free'")
	}
	if comparison.Plan2.ID != "pro" {
		t.Errorf("Expected plan2 to be 'pro'")
	}
	if comparison.PriceDiff <= 0 {
		t.Error("Expected positive price difference between free and pro")
	}
}

func TestPlanRegistry_ComparePlans_NotFound(t *testing.T) {
	registry := NewPlanRegistry()

	_, err := registry.ComparePlans("free", "nonexistent")
	if !errors.Is(err, ErrPlanNotFound) {
		t.Errorf("Expected ErrPlanNotFound, got %v", err)
	}

	_, err = registry.ComparePlans("nonexistent", "free")
	if !errors.Is(err, ErrPlanNotFound) {
		t.Errorf("Expected ErrPlanNotFound, got %v", err)
	}
}

func TestPlanQuotas_Defaults(t *testing.T) {
	registry := NewPlanRegistry()

	freePlan, _ := registry.GetPlan("free")
	if freePlan.Quotas.MaxStorageGB != 1 {
		t.Errorf("Expected free plan max storage 1GB, got %d", freePlan.Quotas.MaxStorageGB)
	}
	if freePlan.Quotas.AllowMultiTenant {
		t.Error("Expected free plan to not allow multi-tenant")
	}

	proPlan, _ := registry.GetPlan("pro")
	if proPlan.Quotas.MaxStorageGB < 50 {
		t.Error("Expected pro plan to have more storage")
	}
	if !proPlan.Quotas.AllowMultiTenant {
		t.Error("Expected pro plan to allow multi-tenant")
	}

	entPlan, _ := registry.GetPlan("enterprise")
	if !entPlan.Quotas.AllowFederation {
		t.Error("Expected enterprise plan to allow federation")
	}
	if !entPlan.Quotas.AllowSSO {
		t.Error("Expected enterprise plan to allow SSO")
	}
}

func TestPlanFeatures(t *testing.T) {
	registry := NewPlanRegistry()

	freePlan, _ := registry.GetPlan("free")
	if !freePlan.Features["hot_storage"] {
		t.Error("Expected free plan to have hot_storage")
	}
	if freePlan.Features["drift_detection"] {
		t.Error("Expected free plan to not have drift_detection")
	}

	proPlan, _ := registry.GetPlan("pro")
	if !proPlan.Features["drift_detection"] {
		t.Error("Expected pro plan to have drift_detection")
	}
	if !proPlan.Features["audit_logs"] {
		t.Error("Expected pro plan to have audit_logs")
	}
}

func TestSubscriptionStatus(t *testing.T) {
	sub := &Subscription{
		Status: SubscriptionActive,
	}

	if sub.Status != SubscriptionActive {
		t.Errorf("Expected status active, got %s", sub.Status)
	}

	sub.Status = SubscriptionCanceled
	if sub.Status != SubscriptionCanceled {
		t.Errorf("Expected status canceled, got %s", sub.Status)
	}
}

func TestPlanPricing_OverageRates(t *testing.T) {
	registry := NewPlanRegistry()

	starterPlan, _ := registry.GetPlan("starter")
	if len(starterPlan.Pricing.OverageRates) == 0 {
		t.Error("Expected starter plan to have overage rates")
	}

	if starterPlan.Pricing.OverageRates["requests"] <= 0 {
		t.Error("Expected positive request overage rate")
	}
}
