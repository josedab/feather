package cloudcontrol

import (
	"errors"
	"testing"
)

func TestNewControlPlane(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())
	if cp == nil {
		t.Fatal("expected non-nil control plane")
	}
}

func TestTenantLifecycle(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())

	tenant, err := cp.CreateTenant(Tenant{ID: "t1", Name: "Acme Corp"})
	if err != nil {
		t.Fatal(err)
	}
	if tenant.MaxInstances != 5 {
		t.Errorf("expected default max instances 5, got %d", tenant.MaxInstances)
	}

	fetched, err := cp.GetTenant("t1")
	if err != nil {
		t.Fatal(err)
	}
	if fetched.Name != "Acme Corp" {
		t.Errorf("expected Acme Corp, got %s", fetched.Name)
	}

	tenants := cp.ListTenants()
	if len(tenants) != 1 {
		t.Fatalf("expected 1 tenant, got %d", len(tenants))
	}
}

func TestDuplicateTenant(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())
	_, _ = cp.CreateTenant(Tenant{ID: "t1", Name: "T1"})
	_, err := cp.CreateTenant(Tenant{ID: "t1", Name: "T1 dup"})
	if !errors.Is(err, ErrTenantExists) {
		t.Fatalf("expected ErrTenantExists, got %v", err)
	}
}

func TestProvisionInstance(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())
	_, _ = cp.CreateTenant(Tenant{ID: "t1", Name: "T1"})

	inst, err := cp.ProvisionInstance(Instance{
		ID: "i1", Name: "prod-1", TenantID: "t1", Region: "us-west-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if inst.Status != InstanceRunning {
		t.Errorf("expected running, got %s", inst.Status)
	}
	if inst.Endpoint == "" {
		t.Error("expected endpoint to be set")
	}

	fetched, err := cp.GetInstance("i1")
	if err != nil {
		t.Fatal(err)
	}
	if fetched.Region != "us-west-2" {
		t.Errorf("expected us-west-2, got %s", fetched.Region)
	}
}

func TestProvisionWithoutTenant(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())
	_, err := cp.ProvisionInstance(Instance{ID: "i1", Name: "test", TenantID: "nonexistent"})
	if !errors.Is(err, ErrTenantNotFound) {
		t.Fatalf("expected ErrTenantNotFound, got %v", err)
	}
}

func TestScaleInstance(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())
	_, _ = cp.CreateTenant(Tenant{ID: "t1", Name: "T1"})
	_, _ = cp.ProvisionInstance(Instance{ID: "i1", Name: "test", TenantID: "t1"})

	inst, err := cp.ScaleInstance("i1", ScaleRequest{Replicas: 5})
	if err != nil {
		t.Fatal(err)
	}
	if inst.Replicas != 5 {
		t.Errorf("expected 5 replicas, got %d", inst.Replicas)
	}
}

func TestScaleExceedsQuota(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())
	_, _ = cp.CreateTenant(Tenant{ID: "t1", Name: "T1", MaxReplicas: 3})
	_, _ = cp.ProvisionInstance(Instance{ID: "i1", Name: "test", TenantID: "t1"})

	_, err := cp.ScaleInstance("i1", ScaleRequest{Replicas: 10})
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestTerminateInstance(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())
	_, _ = cp.CreateTenant(Tenant{ID: "t1", Name: "T1"})
	_, _ = cp.ProvisionInstance(Instance{ID: "i1", Name: "test", TenantID: "t1"})

	if err := cp.TerminateInstance("i1"); err != nil {
		t.Fatal(err)
	}
	_, err := cp.GetInstance("i1")
	if !errors.Is(err, ErrInstanceNotFound) {
		t.Fatalf("expected ErrInstanceNotFound, got %v", err)
	}
}

func TestSetAutoscalePolicy(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())
	_, _ = cp.CreateTenant(Tenant{ID: "t1", Name: "T1"})
	_, _ = cp.ProvisionInstance(Instance{ID: "i1", Name: "test", TenantID: "t1"})

	inst, err := cp.SetAutoscalePolicy("i1", AutoscalePolicy{
		Enabled: true, MinReplicas: 2, MaxReplicas: 10, TargetCPU: 70.0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if inst.Autoscale == nil || !inst.Autoscale.Enabled {
		t.Error("expected autoscale to be enabled")
	}
}

func TestControlPlaneStats(t *testing.T) {
	cp := NewControlPlane(DefaultControlPlaneConfig())
	_, _ = cp.CreateTenant(Tenant{ID: "t1", Name: "T1"})
	_, _ = cp.ProvisionInstance(Instance{ID: "i1", Name: "test", TenantID: "t1", Region: "us-east-1"})

	stats := cp.Stats()
	if stats.TotalTenants != 1 {
		t.Errorf("expected 1 tenant, got %d", stats.TotalTenants)
	}
	if stats.TotalInstances != 1 {
		t.Errorf("expected 1 instance, got %d", stats.TotalInstances)
	}
}
