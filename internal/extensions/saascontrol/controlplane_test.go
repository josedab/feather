package saascontrol

import (
	"testing"
)

func TestNewControlPlane(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())
	if cp == nil {
		t.Fatal("expected non-nil control plane")
	}
	plans := cp.ListPlans()
	if len(plans) != 4 {
		t.Errorf("expected 4 default plans, got %d", len(plans))
	}
}

func TestTenantLifecycle(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())

	tenant, err := cp.CreateTenant("t1", "Acme Corp", "admin@acme.com", "starter")
	if err != nil {
		t.Fatal(err)
	}
	if tenant.Status != "active" {
		t.Errorf("expected active status, got %s", tenant.Status)
	}

	// Duplicate
	_, err = cp.CreateTenant("t1", "Duplicate", "dup@test.com", "free")
	if err != ErrTenantExists {
		t.Fatalf("expected ErrTenantExists, got %v", err)
	}

	// Suspend
	if err := cp.SuspendTenant("t1"); err != nil {
		t.Fatal(err)
	}
	got, _ := cp.GetTenant("t1")
	if got.Status != "suspended" {
		t.Errorf("expected suspended, got %s", got.Status)
	}
}

func TestInvalidPlan(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())
	_, err := cp.CreateTenant("t1", "Test", "test@test.com", "nonexistent")
	if err != ErrInvalidPlan {
		t.Fatalf("expected ErrInvalidPlan, got %v", err)
	}
}

func TestInstanceProvisioning(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())
	_, _ = cp.CreateTenant("t1", "Test", "test@test.com", "starter")

	inst, err := cp.ProvisionInstance("t1", "us-west-2", 2)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Status != "running" {
		t.Errorf("expected running, got %s", inst.Status)
	}
	if inst.Replicas != 2 {
		t.Errorf("expected 2 replicas, got %d", inst.Replicas)
	}
}

func TestQuotaEnforcement(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())
	_, _ = cp.CreateTenant("t1", "Test", "test@test.com", "free")

	// Free plan allows 1 instance
	_, err := cp.ProvisionInstance("t1", "", 1)
	if err != nil {
		t.Fatal(err)
	}

	// Second should fail
	_, err = cp.ProvisionInstance("t1", "", 1)
	if err == nil {
		t.Fatal("expected quota error")
	}
}

func TestScaleAndTerminate(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())
	_, _ = cp.CreateTenant("t1", "Test", "test@test.com", "pro")
	inst, _ := cp.ProvisionInstance("t1", "", 1)

	if err := cp.ScaleInstance(inst.ID, 5); err != nil {
		t.Fatal(err)
	}
	got, _ := cp.GetInstance(inst.ID)
	if got.Replicas != 5 {
		t.Errorf("expected 5 replicas, got %d", got.Replicas)
	}

	if err := cp.TerminateInstance(inst.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = cp.GetInstance(inst.ID)
	if got.Status != "terminated" {
		t.Errorf("expected terminated, got %s", got.Status)
	}
}

func TestStats(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())
	_, _ = cp.CreateTenant("t1", "Test", "test@test.com", "free")
	_, _ = cp.ProvisionInstance("t1", "", 1)

	stats := cp.Stats()
	if stats.TotalTenants != 1 {
		t.Errorf("expected 1 tenant, got %d", stats.TotalTenants)
	}
	if stats.ActiveTenants != 1 {
		t.Errorf("expected 1 active tenant, got %d", stats.ActiveTenants)
	}
	if stats.RunningInstances != 1 {
		t.Errorf("expected 1 running instance, got %d", stats.RunningInstances)
	}
}
