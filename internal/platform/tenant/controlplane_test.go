package tenant

import (
	"testing"
	"time"
)

func TestControlPlane_ConfigureAuth_OIDC(t *testing.T) {
	reg := NewTenantRegistry()
	_ = reg.CreateTenant(&Tenant{
		ID: "tenant-1", Name: "Test Tenant", Tier: TierStandard, Enabled: true,
	})

	cp := NewControlPlane(reg)

	err := cp.ConfigureAuth("tenant-1", AuthConfig{
		Provider:  AuthOIDC,
		IssuerURL: "https://accounts.google.com",
		ClientID:  "client-123",
	})
	if err != nil {
		t.Fatal(err)
	}

	config, err := cp.GetAuthConfig("tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	if config.Provider != AuthOIDC {
		t.Errorf("expected OIDC, got %s", config.Provider)
	}
}

func TestControlPlane_ConfigureAuth_SAML(t *testing.T) {
	reg := NewTenantRegistry()
	cp := NewControlPlane(reg)

	err := cp.ConfigureAuth("tenant-2", AuthConfig{
		Provider:    AuthSAML,
		MetadataURL: "https://idp.example.com/metadata.xml",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestControlPlane_CreateAPIKey(t *testing.T) {
	reg := NewTenantRegistry()
	cp := NewControlPlane(reg)

	key, rawKey, err := cp.CreateAPIKey("tenant-1", "test-key", []string{"read", "write"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if key.Name != "test-key" {
		t.Errorf("expected name test-key, got %s", key.Name)
	}
	if rawKey == "" {
		t.Error("expected non-empty raw key")
	}
	if len(key.Permissions) != 2 {
		t.Errorf("expected 2 permissions, got %d", len(key.Permissions))
	}
}

func TestControlPlane_RevokeAPIKey(t *testing.T) {
	reg := NewTenantRegistry()
	cp := NewControlPlane(reg)

	key, _, _ := cp.CreateAPIKey("tenant-1", "revokable", nil, 0)
	if err := cp.RevokeAPIKey(key.ID); err != nil {
		t.Fatal(err)
	}

	keys := cp.ListAPIKeys("tenant-1")
	for _, k := range keys {
		if k.ID == key.ID && !k.Revoked {
			t.Error("expected key to be revoked")
		}
	}
}

func TestControlPlane_UsageMetering(t *testing.T) {
	reg := NewTenantRegistry()
	cp := NewControlPlane(reg)

	cp.RecordUsage("tenant-1", 100, 50, 25, 1024*1024, 500)
	cp.RecordUsage("tenant-1", 200, 75, 30, 2*1024*1024, 800)

	records := cp.GetUsage("tenant-1", time.Now().Add(-1*time.Hour))
	if len(records) == 0 {
		t.Fatal("expected usage records")
	}
	if records[0].APIRequests != 300 {
		t.Errorf("expected 300 API requests, got %d", records[0].APIRequests)
	}
}

func TestControlPlane_CostAttribution(t *testing.T) {
	reg := NewTenantRegistry()
	cp := NewControlPlane(reg)

	cp.RecordUsage("tenant-1", 1000, 500, 100, 0, 0)
	cost := cp.GetCostAttribution("tenant-1", time.Now().Add(-1*time.Hour))
	if cost <= 0 {
		t.Errorf("expected positive cost, got %f", cost)
	}
}

func TestControlPlane_GenerateCRD(t *testing.T) {
	reg := NewTenantRegistry()
	_ = reg.CreateTenant(&Tenant{
		ID: "crd-tenant", Name: "CRD Tenant", Tier: TierPremium, Enabled: true,
	})

	cp := NewControlPlane(reg)
	crd, err := cp.GenerateCRD("crd-tenant")
	if err != nil {
		t.Fatal(err)
	}
	if crd.Kind != "FeatherTenant" {
		t.Errorf("expected kind FeatherTenant, got %s", crd.Kind)
	}
	if crd.Spec.Replicas != 3 {
		t.Errorf("expected 3 replicas for premium, got %d", crd.Spec.Replicas)
	}
}
