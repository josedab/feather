package feastcompat

import (
	"fmt"
	"testing"
)

func TestNewAdapter(t *testing.T) {
	a := NewAdapter(DefaultAdapterConfig())
	if a == nil {
		t.Fatal("expected non-nil adapter")
	}
}

func TestRegisterMapping(t *testing.T) {
	a := NewAdapter(DefaultAdapterConfig())
	err := a.RegisterMapping(FeatureViewMapping{
		FeastView:    "user_features",
		FeatherGroup: "users",
		FeatureMapping: map[string]string{
			"age": "user_age",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	mappings := a.ListMappings()
	if len(mappings) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(mappings))
	}
}

func TestDuplicateMapping(t *testing.T) {
	a := NewAdapter(DefaultAdapterConfig())
	_ = a.RegisterMapping(FeatureViewMapping{FeastView: "v1", FeatherGroup: "g1"})
	err := a.RegisterMapping(FeatureViewMapping{FeastView: "v1", FeatherGroup: "g2"})
	if err != ErrMappingExists {
		t.Fatalf("expected ErrMappingExists, got %v", err)
	}
}

func TestGetOnlineFeatures(t *testing.T) {
	a := NewAdapter(DefaultAdapterConfig())
	_ = a.RegisterMapping(FeatureViewMapping{
		FeastView:    "user_features",
		FeatherGroup: "users",
	})

	resp, err := a.GetOnlineFeatures(OnlineFeatureRequest{
		Features: []string{"user_features:age", "user_features:name"},
		Entities: map[string][]interface{}{
			"user_id": {"user1", "user2"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(resp.Results))
	}
	if len(resp.Metadata.FeatureNames) != 2 {
		t.Errorf("expected 2 feature names, got %d", len(resp.Metadata.FeatureNames))
	}
}

func TestParseFeatureRef(t *testing.T) {
	tests := []struct {
		ref     string
		view    string
		feature string
	}{
		{"view:feature", "view", "feature"},
		{"view__feature", "view", "feature"},
		{"standalone", "standalone", "standalone"},
	}

	for _, tt := range tests {
		v, f := parseFeatureRef(tt.ref)
		if v != tt.view || f != tt.feature {
			t.Errorf("parseFeatureRef(%q) = (%q, %q), want (%q, %q)", tt.ref, v, f, tt.view, tt.feature)
		}
	}
}

func TestDeleteMapping(t *testing.T) {
	a := NewAdapter(DefaultAdapterConfig())
	_ = a.RegisterMapping(FeatureViewMapping{FeastView: "v1", FeatherGroup: "g1"})

	if err := a.DeleteMapping("v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetMapping("v1"); err != ErrFeatureViewNotFound {
		t.Fatalf("expected ErrFeatureViewNotFound, got %v", err)
	}
}

func TestGetOnlineFeaturesWithLookup(t *testing.T) {
	a := NewAdapter(DefaultAdapterConfig())
	_ = a.RegisterMapping(FeatureViewMapping{
		FeastView:    "user_features",
		FeatherGroup: "users",
	})

	a.SetLookupFunc(func(entityID string, features []string) (map[string]interface{}, error) {
		if entityID == "user1" {
			return map[string]interface{}{"age": 30, "name": "Alice"}, nil
		}
		return nil, fmt.Errorf("not found")
	})

	resp, err := a.GetOnlineFeatures(OnlineFeatureRequest{
		Features: []string{"user_features:age"},
		Entities: map[string][]interface{}{
			"user_id": {"user1", "user2"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	r := resp.Results[0]
	if r.Statuses[0] != "PRESENT" {
		t.Errorf("expected PRESENT for user1, got %s", r.Statuses[0])
	}
	if r.Values[0] != 30 {
		t.Errorf("expected value 30 for user1, got %v", r.Values[0])
	}
	if r.Statuses[1] != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND for user2, got %s", r.Statuses[1])
	}
}

func TestGetOnlineFeaturesNoLookup(t *testing.T) {
	a := NewAdapter(DefaultAdapterConfig())
	_ = a.RegisterMapping(FeatureViewMapping{
		FeastView:    "user_features",
		FeatherGroup: "users",
	})

	resp, err := a.GetOnlineFeatures(OnlineFeatureRequest{
		Features: []string{"user_features:age"},
		Entities: map[string][]interface{}{
			"user_id": {"user1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	r := resp.Results[0]
	if r.Statuses[0] != "PRESENT" {
		t.Errorf("expected PRESENT (stub), got %s", r.Statuses[0])
	}
	if r.Values[0] != nil {
		t.Errorf("expected nil value (stub), got %v", r.Values[0])
	}
}
