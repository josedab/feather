package fedlearning

import (
	"context"
	"testing"
)

func TestNewAdapter(t *testing.T) {
	a := NewAdapter(DefaultConfig())
	if a == nil {
		t.Fatal("NewAdapter returned nil")
	}
	stats := a.Stats()
	if stats["active_orgs"] != 0 {
		t.Errorf("active_orgs = %d, want 0", stats["active_orgs"])
	}
}

func TestRegisterOrg(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		cfg     OrgConfig
		wantErr bool
	}{
		{
			name: "valid org",
			id:   "org-1",
			cfg:  OrgConfig{Name: "Acme", Region: "us-east-1"},
		},
		{
			name:    "empty ID",
			id:      "",
			cfg:     OrgConfig{Name: "Acme"},
			wantErr: true,
		},
		{
			name:    "empty name",
			id:      "org-1",
			cfg:     OrgConfig{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAdapter(DefaultConfig())
			err := a.RegisterOrg(tt.id, tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Duplicate registration should fail
			if err := a.RegisterOrg(tt.id, tt.cfg); err == nil {
				t.Error("expected error for duplicate registration")
			}
		})
	}
}

func TestRegisterOrg_MaxOrganizations(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxOrganizations = 2
	a := NewAdapter(cfg)

	a.RegisterOrg("o1", OrgConfig{Name: "A"})
	a.RegisterOrg("o2", OrgConfig{Name: "B"})
	err := a.RegisterOrg("o3", OrgConfig{Name: "C"})
	if err == nil {
		t.Error("expected error when max organizations reached")
	}
}

func TestListOrgs(t *testing.T) {
	a := NewAdapter(DefaultConfig())
	a.RegisterOrg("o1", OrgConfig{Name: "Acme", Region: "us-east-1"})
	a.RegisterOrg("o2", OrgConfig{Name: "Beta", Region: "eu-west-1"})

	orgs := a.ListOrgs()
	if len(orgs) != 2 {
		t.Errorf("expected 2 orgs, got %d", len(orgs))
	}
}

func TestDeregisterOrg(t *testing.T) {
	a := NewAdapter(DefaultConfig())
	a.RegisterOrg("o1", OrgConfig{Name: "Acme"})

	if err := a.DeregisterOrg("o1"); err != nil {
		t.Fatalf("DeregisterOrg: %v", err)
	}
	if orgs := a.ListOrgs(); len(orgs) != 0 {
		t.Errorf("expected 0 orgs after deregister, got %d", len(orgs))
	}

	// Deregistering again should fail
	if err := a.DeregisterOrg("o1"); err == nil {
		t.Error("expected error for deregistering unknown org")
	}
}

func TestSetFeaturePolicy(t *testing.T) {
	tests := []struct {
		name    string
		feature string
		wantErr bool
	}{
		{"valid", "clicks", false},
		{"empty feature", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAdapter(DefaultConfig())
			err := a.SetFeaturePolicy(tt.feature, FeaturePolicy{AllowedOrgs: []string{"o1"}})
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCheckPolicy(t *testing.T) {
	a := NewAdapter(DefaultConfig())
	a.RegisterOrg("o1", OrgConfig{Name: "Acme", DataResidency: "us"})
	a.RegisterOrg("o2", OrgConfig{Name: "Beta", DataResidency: "eu"})

	a.SetFeaturePolicy("clicks", FeaturePolicy{
		AllowedOrgs:   []string{"o1"},
		DataResidency: "us",
	})

	tests := []struct {
		name        string
		orgID       string
		feature     string
		wantAllowed bool
	}{
		{"allowed org", "o1", "clicks", true},
		{"disallowed org", "o2", "clicks", false},
		{"no policy", "o1", "views", true},
		{"unknown org", "o999", "clicks", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, reason := a.CheckPolicy(tt.orgID, tt.feature)
			if allowed != tt.wantAllowed {
				t.Errorf("allowed = %v, want %v (reason: %s)", allowed, tt.wantAllowed, reason)
			}
		})
	}
}

func TestSecureAggregate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Adapter)
		req     AggregationRequest
		wantErr bool
	}{
		{
			name: "valid aggregation",
			setup: func(a *Adapter) {
				a.RegisterOrg("o1", OrgConfig{Name: "A"})
				a.RegisterOrg("o2", OrgConfig{Name: "B"})
				a.SubmitGradient("o1", "clicks", []float64{1.0})
				a.SubmitGradient("o2", "clicks", []float64{2.0})
			},
			req: AggregationRequest{
				Feature:      "clicks",
				Participants: []string{"o1", "o2"},
				AggType:      AggTypeAvg,
				Round:        1,
			},
		},
		{
			name: "missing feature",
			setup: func(a *Adapter) {
				a.RegisterOrg("o1", OrgConfig{Name: "A"})
				a.RegisterOrg("o2", OrgConfig{Name: "B"})
			},
			req:     AggregationRequest{Participants: []string{"o1", "o2"}},
			wantErr: true,
		},
		{
			name: "too few participants",
			setup: func(a *Adapter) {
				a.RegisterOrg("o1", OrgConfig{Name: "A"})
			},
			req:     AggregationRequest{Feature: "f", Participants: []string{"o1"}},
			wantErr: true,
		},
		{
			name: "unknown participant",
			setup: func(a *Adapter) {
				a.RegisterOrg("o1", OrgConfig{Name: "A"})
			},
			req:     AggregationRequest{Feature: "f", Participants: []string{"o1", "unknown"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAdapter(DefaultConfig())
			tt.setup(a)

			result, err := a.SecureAggregate(context.Background(), tt.req)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Feature != tt.req.Feature {
				t.Errorf("Feature = %q, want %q", result.Feature, tt.req.Feature)
			}
			if result.Participants != len(tt.req.Participants) {
				t.Errorf("Participants = %d, want %d", result.Participants, len(tt.req.Participants))
			}
		})
	}
}

func TestSubmitAndGetAggregatedGradient(t *testing.T) {
	a := NewAdapter(DefaultConfig())
	a.RegisterOrg("o1", OrgConfig{Name: "A"})
	a.RegisterOrg("o2", OrgConfig{Name: "B"})

	a.SubmitGradient("o1", "clicks", []float64{2.0, 4.0})
	a.SubmitGradient("o2", "clicks", []float64{6.0, 8.0})

	grad, err := a.GetAggregatedGradient("clicks")
	if err != nil {
		t.Fatalf("GetAggregatedGradient: %v", err)
	}
	if len(grad) != 2 {
		t.Fatalf("expected 2 gradient values, got %d", len(grad))
	}
	// Average of [2,6] = 4.0 and [4,8] = 6.0
	if grad[0] != 4.0 {
		t.Errorf("grad[0] = %f, want 4.0", grad[0])
	}
	if grad[1] != 6.0 {
		t.Errorf("grad[1] = %f, want 6.0", grad[1])
	}
}

func TestGetAggregatedGradient_NoData(t *testing.T) {
	a := NewAdapter(DefaultConfig())
	_, err := a.GetAggregatedGradient("nonexistent")
	if err == nil {
		t.Error("expected error for feature with no gradients")
	}
}

func TestStats(t *testing.T) {
	a := NewAdapter(DefaultConfig())
	a.RegisterOrg("o1", OrgConfig{Name: "A"})
	a.RegisterOrg("o2", OrgConfig{Name: "B"})
	a.SubmitGradient("o1", "f", []float64{1.0})

	stats := a.Stats()
	if stats["active_orgs"] != 2 {
		t.Errorf("active_orgs = %d, want 2", stats["active_orgs"])
	}
	if stats["gradients_exchanged"] != 1 {
		t.Errorf("gradients_exchanged = %d, want 1", stats["gradients_exchanged"])
	}
}
