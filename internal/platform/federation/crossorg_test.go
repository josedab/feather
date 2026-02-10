package federation

import (
	"strings"
	"testing"
	"time"
)

func TestCrossOrgFederation_RegisterOrg(t *testing.T) {
	fed := NewCrossOrgFederation(DefaultCrossOrgConfig())

	err := fed.RegisterOrg(Organization{ID: "org1", Name: "Acme Corp"})
	if err != nil {
		t.Fatalf("RegisterOrg: %v", err)
	}

	orgs := fed.ListOrgs()
	if len(orgs) != 1 {
		t.Fatalf("expected 1 org, got %d", len(orgs))
	}
	if orgs[0].TrustLevel != "limited" {
		t.Fatalf("expected default trust level 'limited', got %q", orgs[0].TrustLevel)
	}
}

func TestCrossOrgFederation_RegisterOrg_Validation(t *testing.T) {
	fed := NewCrossOrgFederation(DefaultCrossOrgConfig())
	if err := fed.RegisterOrg(Organization{}); err == nil {
		t.Fatal("expected error for empty org")
	}
}

func TestCrossOrgFederation_CreateAgreement(t *testing.T) {
	fed := NewCrossOrgFederation(DefaultCrossOrgConfig())
	fed.RegisterOrg(Organization{ID: "org1", Name: "Org 1"})
	fed.RegisterOrg(Organization{ID: "org2", Name: "Org 2"})

	agreement, err := fed.CreateAgreement(SharingAgreement{
		SourceOrg: "org1",
		TargetOrg: "org2",
		Features:  []string{"user_age", "user_score"},
		Privacy: PrivacyPolicy{
			Mechanism:   "laplace",
			Epsilon:     1.0,
			Sensitivity: 1.0,
		},
	})
	if err != nil {
		t.Fatalf("CreateAgreement: %v", err)
	}
	if !agreement.Active {
		t.Fatal("expected active agreement")
	}
}

func TestCrossOrgFederation_CreateAgreement_Validation(t *testing.T) {
	fed := NewCrossOrgFederation(DefaultCrossOrgConfig())

	_, err := fed.CreateAgreement(SharingAgreement{})
	if err == nil {
		t.Fatal("expected error for empty agreement")
	}

	fed.RegisterOrg(Organization{ID: "org1", Name: "Org 1"})
	_, err = fed.CreateAgreement(SharingAgreement{SourceOrg: "org1", TargetOrg: "unknown", Features: []string{"f"}})
	if err == nil {
		t.Fatal("expected error for unknown target org")
	}
}

func TestCrossOrgFederation_ProcessRequest(t *testing.T) {
	cfg := DefaultCrossOrgConfig()
	cfg.OrgID = "org1"
	cfg.SigningKey = "" // disable signature verification for testing
	fed := NewCrossOrgFederation(cfg)

	fed.RegisterOrg(Organization{ID: "org1", Name: "Owner"})
	fed.RegisterOrg(Organization{ID: "org2", Name: "Consumer"})

	fed.CreateAgreement(SharingAgreement{
		SourceOrg: "org1",
		TargetOrg: "org2",
		Features:  []string{"score"},
		Privacy:   PrivacyPolicy{Mechanism: "laplace", Epsilon: 1.0, Sensitivity: 1.0},
	})

	req := CrossOrgRequest{
		SourceOrg:  "org2",
		Features:   []string{"score"},
		EntityKeys: []string{"user1"},
		Timestamp:  time.Now(),
	}

	resp, err := fed.ProcessRequest(req, map[string]interface{}{"score": 95.0})
	if err != nil {
		t.Fatalf("ProcessRequest: %v", err)
	}
	if !resp.NoiseAdded {
		t.Fatal("expected noise to be added")
	}
	if resp.Epsilon != 1.0 {
		t.Fatalf("expected epsilon 1.0, got %f", resp.Epsilon)
	}

	// Value should be different from original (noise added)
	// Note: with low probability, noise could be 0, so we just check it exists
	if _, ok := resp.Features["score"]; !ok {
		t.Fatal("expected score in response")
	}
}

func TestCrossOrgFederation_CrossOrgBudget(t *testing.T) {
	cfg := DefaultCrossOrgConfig()
	cfg.MaxEpsilonBudget = 2.0
	fed := NewCrossOrgFederation(cfg)

	fed.RegisterOrg(Organization{ID: "src", Name: "Source"})
	fed.RegisterOrg(Organization{ID: "dst", Name: "Destination"})

	fed.CreateAgreement(SharingAgreement{
		SourceOrg: "src",
		TargetOrg: "dst",
		Features:  []string{"f"},
		Privacy:   PrivacyPolicy{Mechanism: "laplace", Epsilon: 1.0, Sensitivity: 1.0},
	})

	req := CrossOrgRequest{SourceOrg: "dst", Features: []string{"f"}, Timestamp: time.Now()}

	// First request succeeds
	_, err := fed.ProcessRequest(req, map[string]interface{}{"f": 1.0})
	if err != nil {
		t.Fatalf("first request: %v", err)
	}

	// Second request succeeds (budget = 2.0, each costs 1.0)
	_, err = fed.ProcessRequest(req, map[string]interface{}{"f": 1.0})
	if err != nil {
		t.Fatalf("second request: %v", err)
	}

	// Third request should fail (budget exhausted)
	_, err = fed.ProcessRequest(req, map[string]interface{}{"f": 1.0})
	if err == nil || !strings.Contains(err.Error(), "budget exhausted") {
		t.Fatalf("expected budget exhausted error, got %v", err)
	}

	// Check budget
	budget, _ := fed.GetCrossOrgBudget("dst")
	if budget.Consumed != 2.0 {
		t.Fatalf("expected consumed 2.0, got %f", budget.Consumed)
	}
}

func TestCrossOrgFederation_GaussianNoise(t *testing.T) {
	cfg := DefaultCrossOrgConfig()
	fed := NewCrossOrgFederation(cfg)

	fed.RegisterOrg(Organization{ID: "src", Name: "Source"})
	fed.RegisterOrg(Organization{ID: "dst", Name: "Destination"})

	fed.CreateAgreement(SharingAgreement{
		SourceOrg: "src",
		TargetOrg: "dst",
		Features:  []string{"f"},
		Privacy:   PrivacyPolicy{Mechanism: "gaussian", Epsilon: 1.0, Delta: 1e-5, Sensitivity: 1.0},
	})

	req := CrossOrgRequest{SourceOrg: "dst", Features: []string{"f"}, Timestamp: time.Now()}
	resp, err := fed.ProcessRequest(req, map[string]interface{}{"f": 100.0})
	if err != nil {
		t.Fatalf("ProcessRequest: %v", err)
	}
	if !resp.NoiseAdded {
		t.Fatal("expected noise")
	}
}

func TestCrossOrgFederation_AuditLog(t *testing.T) {
	fed := NewCrossOrgFederation(DefaultCrossOrgConfig())
	fed.RegisterOrg(Organization{ID: "org1", Name: "Org1"})

	log := fed.GetAuditLog(10)
	if len(log) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(log))
	}
	if log[0].Operation != "register_org" {
		t.Fatalf("expected 'register_org', got %q", log[0].Operation)
	}
}

func TestCrossOrgFederation_Stats(t *testing.T) {
	fed := NewCrossOrgFederation(DefaultCrossOrgConfig())
	fed.RegisterOrg(Organization{ID: "org1", Name: "Org1"})
	fed.RegisterOrg(Organization{ID: "org2", Name: "Org2"})

	stats := fed.Stats()
	if stats.TotalOrgs != 2 {
		t.Fatalf("expected 2 orgs, got %d", stats.TotalOrgs)
	}
}

func TestCrossOrgFederation_UnknownOrg(t *testing.T) {
	fed := NewCrossOrgFederation(DefaultCrossOrgConfig())
	req := CrossOrgRequest{SourceOrg: "unknown", Timestamp: time.Now()}
	_, err := fed.ProcessRequest(req, nil)
	if err == nil {
		t.Fatal("expected error for unknown org")
	}
}
