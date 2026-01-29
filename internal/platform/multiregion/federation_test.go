package multiregion

import (
	"errors"
	"testing"
)

func newTestFederation() *Federation {
	cfg := DefaultFederationConfig()
	f := NewFederation(cfg)
	_ = f.AddRegion(Region{
		Name:     "us-east-1",
		Endpoint: "https://us-east-1.feather.io",
		Cloud:    "aws",
		Location: "Virginia",
		Latency:  5.0,
		Status:   RegionActive,
	})
	return f
}

// ---------------------------------------------------------------------------
// AddRegion / RemoveRegion
// ---------------------------------------------------------------------------

func TestAddRegion(t *testing.T) {
	tests := []struct {
		name    string
		region  Region
		wantErr error
	}{
		{
			name: "valid region",
			region: Region{
				Name:     "eu-west-1",
				Endpoint: "https://eu-west-1.feather.io",
				Cloud:    "aws",
			},
			wantErr: nil,
		},
		{
			name: "empty name",
			region: Region{
				Endpoint: "https://somewhere.feather.io",
			},
			wantErr: ErrRegionNameEmpty,
		},
		{
			name: "empty endpoint",
			region: Region{
				Name: "ap-south-1",
			},
			wantErr: ErrEndpointEmpty,
		},
		{
			name: "duplicate region",
			region: Region{
				Name:     "us-east-1",
				Endpoint: "https://us-east-1.feather.io",
			},
			wantErr: ErrRegionExists,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newTestFederation()
			err := f.AddRegion(tc.region)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("AddRegion() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestRemoveRegion(t *testing.T) {
	tests := []struct {
		name    string
		region  string
		wantErr error
	}{
		{
			name:    "remove non-local region",
			region:  "eu-west-1",
			wantErr: nil,
		},
		{
			name:    "remove local region",
			region:  "us-east-1",
			wantErr: ErrCannotRemoveLocal,
		},
		{
			name:    "remove unknown region",
			region:  "unknown",
			wantErr: ErrRegionNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newTestFederation()
			_ = f.AddRegion(Region{
				Name:     "eu-west-1",
				Endpoint: "https://eu-west-1.feather.io",
			})
			err := f.RemoveRegion(tc.region)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("RemoveRegion(%q) error = %v, want %v", tc.region, err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetRegion / ListRegions
// ---------------------------------------------------------------------------

func TestGetRegion(t *testing.T) {
	f := newTestFederation()

	r, err := f.GetRegion("us-east-1")
	if err != nil {
		t.Fatalf("GetRegion() unexpected error: %v", err)
	}
	if r.Name != "us-east-1" {
		t.Fatalf("GetRegion() name = %q, want %q", r.Name, "us-east-1")
	}

	_, err = f.GetRegion("nonexistent")
	if !errors.Is(err, ErrRegionNotFound) {
		t.Fatalf("GetRegion(nonexistent) error = %v, want %v", err, ErrRegionNotFound)
	}
}

func TestListRegions(t *testing.T) {
	f := newTestFederation()
	_ = f.AddRegion(Region{Name: "eu-west-1", Endpoint: "https://eu.feather.io"})

	regions := f.ListRegions()
	if len(regions) != 2 {
		t.Fatalf("ListRegions() count = %d, want 2", len(regions))
	}
}

// ---------------------------------------------------------------------------
// Routing with residency
// ---------------------------------------------------------------------------

func TestRoute(t *testing.T) {
	tests := []struct {
		name       string
		entity     string
		setup      func(f *Federation)
		wantRegion string
		wantReason string
		wantErr    error
	}{
		{
			name:   "strict residency routes to designated region",
			entity: "eu_user_123",
			setup: func(f *Federation) {
				_ = f.AddRegion(Region{
					Name: "eu-west-1", Endpoint: "https://eu.feather.io",
					Cloud: "aws", Latency: 20, Status: RegionActive,
				})
				_ = f.SetResidencyRule(ResidencyRule{
					Pattern: "eu_",
					Region:  "eu-west-1",
					Policy:  ResidencyStrict,
					Reason:  "GDPR",
				})
			},
			wantRegion: "eu-west-1",
			wantReason: "residency:GDPR",
		},
		{
			name:       "local region fallback",
			entity:     "user_456",
			setup:      func(f *Federation) {},
			wantRegion: "us-east-1",
			wantReason: "local",
		},
		{
			name:   "lowest latency when local is inactive",
			entity: "user_789",
			setup: func(f *Federation) {
				f.mu.Lock()
				f.regions["us-east-1"].Status = RegionInactive
				f.mu.Unlock()
				_ = f.AddRegion(Region{
					Name: "ap-south-1", Endpoint: "https://ap.feather.io",
					Cloud: "aws", Latency: 50, Status: RegionActive,
				})
				_ = f.AddRegion(Region{
					Name: "eu-west-1", Endpoint: "https://eu.feather.io",
					Cloud: "aws", Latency: 30, Status: RegionActive,
				})
			},
			wantRegion: "eu-west-1",
			wantReason: "lowest_latency",
		},
		{
			name:   "no active region",
			entity: "user_000",
			setup: func(f *Federation) {
				f.mu.Lock()
				f.regions["us-east-1"].Status = RegionFailed
				f.mu.Unlock()
			},
			wantErr: ErrNoActiveRegion,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newTestFederation()
			tc.setup(f)

			res, err := f.Route(tc.entity)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Route(%q) error = %v, want %v", tc.entity, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Route(%q) unexpected error: %v", tc.entity, err)
			}
			if res.Region != tc.wantRegion {
				t.Errorf("Route(%q) region = %q, want %q", tc.entity, res.Region, tc.wantRegion)
			}
			if res.Reason != tc.wantReason {
				t.Errorf("Route(%q) reason = %q, want %q", tc.entity, res.Reason, tc.wantReason)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Replication events and conflict resolution
// ---------------------------------------------------------------------------

func TestReplicateEvent(t *testing.T) {
	f := newTestFederation()
	_ = f.AddRegion(Region{
		Name: "eu-west-1", Endpoint: "https://eu.feather.io",
		Cloud: "aws", Latency: 20, Status: RegionActive,
	})

	err := f.ReplicateEvent(ReplicationEvent{
		Entity:     "user_1",
		Feature:    "click_count",
		FromRegion: "us-east-1",
		ToRegion:   "eu-west-1",
		Version:    1,
	})
	if err != nil {
		t.Fatalf("ReplicateEvent() unexpected error: %v", err)
	}

	log := f.GetReplicationLog(10)
	if len(log) != 1 {
		t.Fatalf("replication log length = %d, want 1", len(log))
	}
	if log[0].Status != "replicated" {
		t.Errorf("event status = %q, want %q", log[0].Status, "replicated")
	}
}

func TestReplicateEvent_ResidencyViolation(t *testing.T) {
	f := newTestFederation()
	_ = f.AddRegion(Region{
		Name: "eu-west-1", Endpoint: "https://eu.feather.io",
		Cloud: "aws", Status: RegionActive,
	})
	_ = f.SetResidencyRule(ResidencyRule{
		Pattern: "eu_",
		Region:  "eu-west-1",
		Policy:  ResidencyStrict,
		Reason:  "GDPR",
	})

	err := f.ReplicateEvent(ReplicationEvent{
		Entity:     "eu_user_1",
		Feature:    "login_count",
		FromRegion: "eu-west-1",
		ToRegion:   "us-east-1", // violates GDPR rule
		Version:    1,
	})
	if !errors.Is(err, ErrResidencyViolation) {
		t.Fatalf("ReplicateEvent() error = %v, want %v", err, ErrResidencyViolation)
	}
}

func TestResolveConflict_LWW(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.ConflictStrategy = ConflictLWW
	f := NewFederation(cfg)

	versions := map[string]int64{
		"us-east-1":  5,
		"eu-west-1":  8,
		"ap-south-1": 3,
	}

	info := f.ResolveConflict("user_1", "click_count", versions)
	if info.Winner != "eu-west-1" {
		t.Errorf("LWW winner = %q, want %q", info.Winner, "eu-west-1")
	}
	if info.Resolution != string(ConflictLWW) {
		t.Errorf("resolution = %q, want %q", info.Resolution, ConflictLWW)
	}
}

func TestResolveConflict_HighestVersion(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.ConflictStrategy = ConflictHighestVersion
	f := NewFederation(cfg)

	versions := map[string]int64{
		"us-east-1": 10,
		"eu-west-1": 7,
	}

	info := f.ResolveConflict("user_2", "purchase_total", versions)
	if info.Winner != "us-east-1" {
		t.Errorf("HighestVersion winner = %q, want %q", info.Winner, "us-east-1")
	}
	if info.Resolution != string(ConflictHighestVersion) {
		t.Errorf("resolution = %q, want %q", info.Resolution, ConflictHighestVersion)
	}
}

// ---------------------------------------------------------------------------
// Vector clocks
// ---------------------------------------------------------------------------

func TestVectorClock(t *testing.T) {
	f := newTestFederation()
	_ = f.AddRegion(Region{
		Name: "eu-west-1", Endpoint: "https://eu.feather.io",
		Cloud: "aws", Status: RegionActive,
	})

	_ = f.ReplicateEvent(ReplicationEvent{
		Entity: "user_1", Feature: "feat", FromRegion: "us-east-1",
		ToRegion: "eu-west-1", Version: 3,
	})
	_ = f.ReplicateEvent(ReplicationEvent{
		Entity: "user_1", Feature: "feat", FromRegion: "eu-west-1",
		ToRegion: "us-east-1", Version: 5,
	})

	vc := f.GetVectorClock("user_1")
	if vc == nil {
		t.Fatal("GetVectorClock() returned nil")
	}
	if vc["us-east-1"] != 3 {
		t.Errorf("vector clock[us-east-1] = %d, want 3", vc["us-east-1"])
	}
	if vc["eu-west-1"] != 5 {
		t.Errorf("vector clock[eu-west-1] = %d, want 5", vc["eu-west-1"])
	}

	// Unknown entity returns nil.
	if got := f.GetVectorClock("unknown"); got != nil {
		t.Errorf("GetVectorClock(unknown) = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// CheckResidency
// ---------------------------------------------------------------------------

func TestCheckResidency(t *testing.T) {
	tests := []struct {
		name       string
		entity     string
		region     string
		rules      []ResidencyRule
		wantOK     bool
		wantReason string
	}{
		{
			name:   "no rules - allowed",
			entity: "user_1",
			region: "us-east-1",
			wantOK: true,
		},
		{
			name:   "strict rule - matching region",
			entity: "eu_user_1",
			region: "eu-west-1",
			rules: []ResidencyRule{
				{Pattern: "eu_", Region: "eu-west-1", Policy: ResidencyStrict, Reason: "GDPR"},
			},
			wantOK: true,
		},
		{
			name:   "strict rule - wrong region",
			entity: "eu_user_1",
			region: "us-east-1",
			rules: []ResidencyRule{
				{Pattern: "eu_", Region: "eu-west-1", Policy: ResidencyStrict, Reason: "GDPR"},
			},
			wantOK:     false,
			wantReason: "strict residency: must stay in eu-west-1 (GDPR)",
		},
		{
			name:   "prefer rule - wrong region still allowed",
			entity: "ca_user_1",
			region: "us-east-1",
			rules: []ResidencyRule{
				{Pattern: "ca_", Region: "ca-central-1", Policy: ResidencyPrefer, Reason: "CCPA"},
			},
			wantOK: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := NewFederation(DefaultFederationConfig())
			for _, r := range tc.rules {
				_ = f.SetResidencyRule(r)
			}
			ok, reason := f.CheckResidency(tc.entity, tc.region)
			if ok != tc.wantOK {
				t.Errorf("CheckResidency() ok = %v, want %v", ok, tc.wantOK)
			}
			if tc.wantReason != "" && reason != tc.wantReason {
				t.Errorf("CheckResidency() reason = %q, want %q", reason, tc.wantReason)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

func TestStats(t *testing.T) {
	f := newTestFederation()
	_ = f.AddRegion(Region{
		Name: "eu-west-1", Endpoint: "https://eu.feather.io",
		Cloud: "gcp", Latency: 20, Status: RegionActive,
	})
	_ = f.AddRegion(Region{
		Name: "ap-south-1", Endpoint: "https://ap.feather.io",
		Cloud: "aws", Latency: 40, Status: RegionInactive,
	})

	_ = f.SetResidencyRule(ResidencyRule{
		Pattern: "eu_", Region: "eu-west-1", Policy: ResidencyStrict, Reason: "GDPR",
	})

	_ = f.ReplicateEvent(ReplicationEvent{
		Entity: "user_1", Feature: "feat", FromRegion: "us-east-1",
		ToRegion: "eu-west-1", Version: 1,
	})

	s := f.Stats()

	if s.TotalRegions != 3 {
		t.Errorf("TotalRegions = %d, want 3", s.TotalRegions)
	}
	if s.ActiveRegions != 2 {
		t.Errorf("ActiveRegions = %d, want 2", s.ActiveRegions)
	}
	if s.LocalRegion != "us-east-1" {
		t.Errorf("LocalRegion = %q, want %q", s.LocalRegion, "us-east-1")
	}
	if s.ReplicationEvents != 1 {
		t.Errorf("ReplicationEvents = %d, want 1", s.ReplicationEvents)
	}
	if s.ResidencyRules != 1 {
		t.Errorf("ResidencyRules = %d, want 1", s.ResidencyRules)
	}
	if s.ByCloud["aws"] != 2 {
		t.Errorf("ByCloud[aws] = %d, want 2", s.ByCloud["aws"])
	}
	if s.ByCloud["gcp"] != 1 {
		t.Errorf("ByCloud[gcp] = %d, want 1", s.ByCloud["gcp"])
	}

	// AvgLatency should be (5+20)/2 = 12.5 for active regions.
	if s.AvgLatency < 12.4 || s.AvgLatency > 12.6 {
		t.Errorf("AvgLatency = %f, want ~12.5", s.AvgLatency)
	}
}
