package mesh

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func TestRegisterOrganization_Valid(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	err := mp.RegisterOrganization(Organization{
		ID:       "org-1",
		Name:     "Acme Corp",
		Endpoint: "https://acme.example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	orgs := mp.ListOrganizations()
	if len(orgs) != 1 {
		t.Fatalf("expected 1 org, got %d", len(orgs))
	}
	if orgs[0].Status != "active" {
		t.Errorf("expected active status, got %s", orgs[0].Status)
	}
	if orgs[0].JoinedAt.IsZero() {
		t.Error("expected JoinedAt to be set")
	}
}

func TestRegisterOrganization_EmptyIDError(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	err := mp.RegisterOrganization(Organization{Name: "Acme"})
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestRegisterOrganization_EmptyNameError(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	err := mp.RegisterOrganization(Organization{ID: "org-1"})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestRegisterOrganization_Duplicate(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	_ = mp.RegisterOrganization(Organization{ID: "org-1", Name: "Acme"})
	err := mp.RegisterOrganization(Organization{ID: "org-1", Name: "Acme2"})
	if err == nil {
		t.Fatal("expected error for duplicate org")
	}
}

func TestGrantAccess_Valid(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	_ = mp.RegisterOrganization(Organization{ID: "org-1", Name: "Acme"})

	err := mp.GrantAccess(AccessGrant{
		GranterOrg: "local-org",
		GranteeOrg: "org-1",
		Features:   []string{"feature_a"},
		Level:      AccessRead,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	grants := mp.ListGrants("")
	if len(grants) != 1 {
		t.Fatalf("expected 1 grant, got %d", len(grants))
	}
	if grants[0].Level != AccessRead {
		t.Errorf("expected read level, got %s", grants[0].Level)
	}
}

func TestGrantAccess_ExpiredGrant(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	_ = mp.RegisterOrganization(Organization{ID: "org-1", Name: "Acme"})

	past := time.Now().Add(-1 * time.Hour)
	err := mp.GrantAccess(AccessGrant{
		GranterOrg: "local-org",
		GranteeOrg: "org-1",
		Features:   []string{"feature_a"},
		ExpiresAt:  &past,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	level, _ := mp.CheckAccess("org-1", []string{"feature_a"})
	if level != AccessNone {
		t.Errorf("expected none for expired grant, got %s", level)
	}
}

func TestGrantAccess_UnknownOrg(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	err := mp.GrantAccess(AccessGrant{
		GranterOrg: "local-org",
		GranteeOrg: "unknown-org",
		Features:   []string{"f"},
	})
	if err == nil {
		t.Fatal("expected error for unknown grantee org")
	}
}

func TestRevokeAccess_Valid(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	_ = mp.RegisterOrganization(Organization{ID: "org-1", Name: "Acme"})
	_ = mp.GrantAccess(AccessGrant{
		GranterOrg: "local-org",
		GranteeOrg: "org-1",
		Features:   []string{"feature_a"},
	})

	grants := mp.ListGrants("")
	err := mp.RevokeAccess(grants[0].ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	level, _ := mp.CheckAccess("org-1", []string{"feature_a"})
	if level != AccessNone {
		t.Errorf("expected none after revocation, got %s", level)
	}
}

func TestRevokeAccess_AlreadyRevoked(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	_ = mp.RegisterOrganization(Organization{ID: "org-1", Name: "Acme"})
	_ = mp.GrantAccess(AccessGrant{
		GranterOrg: "local-org",
		GranteeOrg: "org-1",
		Features:   []string{"feature_a"},
	})

	grants := mp.ListGrants("")
	_ = mp.RevokeAccess(grants[0].ID)
	err := mp.RevokeAccess(grants[0].ID)
	if err == nil {
		t.Fatal("expected error for already-revoked grant")
	}
}

func TestRevokeAccess_NonExistent(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	err := mp.RevokeAccess("nonexistent-grant")
	if err == nil {
		t.Fatal("expected error for non-existent grant")
	}
}

func TestCheckAccess_ReadLevel(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	_ = mp.RegisterOrganization(Organization{ID: "org-1", Name: "Acme"})
	_ = mp.GrantAccess(AccessGrant{
		GranterOrg: "local-org",
		GranteeOrg: "org-1",
		Features:   []string{"feature_a"},
		Level:      AccessRead,
	})

	level, err := mp.CheckAccess("org-1", []string{"feature_a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if level != AccessRead {
		t.Errorf("expected read, got %s", level)
	}
}

func TestCheckAccess_ReadWriteLevel(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	_ = mp.RegisterOrganization(Organization{ID: "org-1", Name: "Acme"})
	_ = mp.GrantAccess(AccessGrant{
		GranterOrg: "local-org",
		GranteeOrg: "org-1",
		Features:   []string{"feature_a"},
		Level:      AccessReadWrite,
	})

	level, _ := mp.CheckAccess("org-1", []string{"feature_a"})
	if level != AccessReadWrite {
		t.Errorf("expected read_write, got %s", level)
	}
}

func TestCheckAccess_AdminLevel(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	_ = mp.RegisterOrganization(Organization{ID: "org-1", Name: "Acme"})
	_ = mp.GrantAccess(AccessGrant{
		GranterOrg: "local-org",
		GranteeOrg: "org-1",
		Features:   []string{"feature_a"},
		Level:      AccessAdmin,
	})

	level, _ := mp.CheckAccess("org-1", []string{"feature_a"})
	if level != AccessAdmin {
		t.Errorf("expected admin, got %s", level)
	}
}

func TestCheckAccess_Denied(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	level, err := mp.CheckAccess("unknown-org", []string{"feature_a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if level != AccessNone {
		t.Errorf("expected none, got %s", level)
	}
}

func TestCheckAccess_ExpiredGrant(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	_ = mp.RegisterOrganization(Organization{ID: "org-1", Name: "Acme"})
	past := time.Now().Add(-1 * time.Hour)
	_ = mp.GrantAccess(AccessGrant{
		GranterOrg: "local-org",
		GranteeOrg: "org-1",
		Features:   []string{"feature_a"},
		Level:      AccessRead,
		ExpiresAt:  &past,
	})

	level, _ := mp.CheckAccess("org-1", []string{"feature_a"})
	if level != AccessNone {
		t.Errorf("expected none for expired, got %s", level)
	}
}

func TestRequestFeatures_ValidTransfer(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	_ = mp.RegisterOrganization(Organization{
		ID:           "org-1",
		Name:         "Acme",
		SharedSecret: "secret-key",
	})
	_ = mp.GrantAccess(AccessGrant{
		GranterOrg: "local-org",
		GranteeOrg: "org-1",
		Features:   []string{"feature_a"},
		Level:      AccessRead,
	})

	sig := computeSignature("req-1", "secret-key")
	resp, err := mp.RequestFeatures(FeatureTransferRequest{
		RequestID: "req-1",
		FromOrg:   "org-1",
		ToOrg:     "local-org",
		Features:  []string{"feature_a"},
		Signature: sig,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.RequestID != "req-1" {
		t.Errorf("expected request ID req-1, got %s", resp.RequestID)
	}

	stats := mp.Stats()
	if stats.SuccessfulTransfers != 1 {
		t.Errorf("expected 1 successful transfer, got %d", stats.SuccessfulTransfers)
	}
}

func TestRequestFeatures_Denied(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	_, err := mp.RequestFeatures(FeatureTransferRequest{
		RequestID: "req-1",
		FromOrg:   "unknown-org",
		Features:  []string{"feature_a"},
	})
	if err == nil {
		t.Fatal("expected access denied error")
	}

	stats := mp.Stats()
	if stats.DeniedTransfers != 1 {
		t.Errorf("expected 1 denied transfer, got %d", stats.DeniedTransfers)
	}
}

func TestRequestFeatures_WrongSignature(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	_ = mp.RegisterOrganization(Organization{
		ID:           "org-1",
		Name:         "Acme",
		SharedSecret: "secret-key",
	})
	_ = mp.GrantAccess(AccessGrant{
		GranterOrg: "local-org",
		GranteeOrg: "org-1",
		Features:   []string{"feature_a"},
		Level:      AccessRead,
	})

	_, err := mp.RequestFeatures(FeatureTransferRequest{
		RequestID: "req-1",
		FromOrg:   "org-1",
		Features:  []string{"feature_a"},
		Signature: "wrong-signature",
	})
	if err == nil {
		t.Fatal("expected invalid signature error")
	}
}

func TestComputeSignature_Correctness(t *testing.T) {
	sig := computeSignature("test-data", "test-secret")

	h := hmac.New(sha256.New, []byte("test-secret"))
	h.Write([]byte("test-data"))
	expected := hex.EncodeToString(h.Sum(nil))

	if sig != expected {
		t.Errorf("signature mismatch: got %s, want %s", sig, expected)
	}
}

func TestComputeSignature_Deterministic(t *testing.T) {
	s1 := computeSignature("data", "key")
	s2 := computeSignature("data", "key")
	if s1 != s2 {
		t.Error("expected deterministic output")
	}
}

func TestListOrganizations(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	_ = mp.RegisterOrganization(Organization{ID: "org-1", Name: "A"})
	_ = mp.RegisterOrganization(Organization{ID: "org-2", Name: "B"})

	orgs := mp.ListOrganizations()
	if len(orgs) != 2 {
		t.Errorf("expected 2 orgs, got %d", len(orgs))
	}
}

func TestListGrants(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	_ = mp.RegisterOrganization(Organization{ID: "org-1", Name: "A"})
	_ = mp.GrantAccess(AccessGrant{
		GranterOrg: "local-org",
		GranteeOrg: "org-1",
		Features:   []string{"f1"},
	})
	_ = mp.GrantAccess(AccessGrant{
		GranterOrg: "local-org",
		GranteeOrg: "org-1",
		Features:   []string{"f2"},
	})

	all := mp.ListGrants("")
	if len(all) != 2 {
		t.Errorf("expected 2 grants, got %d", len(all))
	}

	filtered := mp.ListGrants("local-org")
	if len(filtered) != 2 {
		t.Errorf("expected 2 grants for local-org, got %d", len(filtered))
	}
}

func TestStats(t *testing.T) {
	mp := NewMeshProtocol("local-org")
	_ = mp.RegisterOrganization(Organization{ID: "org-1", Name: "A"})
	_ = mp.GrantAccess(AccessGrant{
		GranterOrg: "local-org",
		GranteeOrg: "org-1",
		Features:   []string{"f1"},
	})

	stats := mp.Stats()
	if stats.TotalOrganizations != 1 {
		t.Errorf("expected 1 org, got %d", stats.TotalOrganizations)
	}
	if stats.ActiveGrants != 1 {
		t.Errorf("expected 1 active grant, got %d", stats.ActiveGrants)
	}
}
