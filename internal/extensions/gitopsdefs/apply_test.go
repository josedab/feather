package gitopsdefs

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateSpec_Valid(t *testing.T) {
	spec := &DeclarativeSpec{
		APIVersion: "feather/v1",
		Kind:       "FeatureGroup",
		Metadata:   SpecMetadata{Name: "user_features"},
		Spec: FeatureGroupSpec{
			EntityType: "user",
			Features: []FeatureSpec{
				{Name: "age", Type: "int64"},
				{Name: "name", Type: "string"},
			},
		},
	}

	result := ValidateSpec(spec)
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Errors)
	}
}

func TestValidateSpec_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		spec    DeclarativeSpec
		wantErr string
	}{
		{
			name:    "missing apiVersion",
			spec:    DeclarativeSpec{Kind: "FeatureGroup", Metadata: SpecMetadata{Name: "x"}},
			wantErr: "apiVersion",
		},
		{
			name:    "missing kind",
			spec:    DeclarativeSpec{APIVersion: "v1", Metadata: SpecMetadata{Name: "x"}},
			wantErr: "kind is required",
		},
		{
			name:    "unknown kind",
			spec:    DeclarativeSpec{APIVersion: "v1", Kind: "BadKind", Metadata: SpecMetadata{Name: "x"}},
			wantErr: "unknown kind",
		},
		{
			name: "missing feature name",
			spec: DeclarativeSpec{
				APIVersion: "v1", Kind: "FeatureGroup", Metadata: SpecMetadata{Name: "x"},
				Spec: FeatureGroupSpec{Features: []FeatureSpec{{Type: "int64"}}},
			},
			wantErr: "name is required",
		},
		{
			name: "invalid type",
			spec: DeclarativeSpec{
				APIVersion: "v1", Kind: "FeatureGroup", Metadata: SpecMetadata{Name: "x"},
				Spec: FeatureGroupSpec{Features: []FeatureSpec{{Name: "f", Type: "invalid"}}},
			},
			wantErr: "invalid type",
		},
		{
			name: "duplicate feature",
			spec: DeclarativeSpec{
				APIVersion: "v1", Kind: "FeatureGroup", Metadata: SpecMetadata{Name: "x"},
				Spec: FeatureGroupSpec{Features: []FeatureSpec{{Name: "f", Type: "int64"}, {Name: "f", Type: "string"}}},
			},
			wantErr: "duplicate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSpec(&tt.spec)
			if result.Valid {
				t.Fatal("expected invalid")
			}
			found := false
			for _, e := range result.Errors {
				if strings.Contains(e, tt.wantErr) {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, result.Errors)
			}
		})
	}
}

func TestPlanAndApply(t *testing.T) {
	r := NewReconciler(DefaultReconcilerConfig())
	r.LoadDefinition(FeatureDefinition{Name: "users", EntityType: "user"})

	plan := r.Plan()
	if plan.Summary.Creates != 1 {
		t.Fatalf("expected 1 create, got %d", plan.Summary.Creates)
	}

	result := r.Apply()
	if !result.Success {
		t.Fatal("expected successful apply")
	}
	if result.Summary.Creates != 1 {
		t.Fatalf("expected 1 create in apply, got %d", result.Summary.Creates)
	}

	// Second apply should be no-op
	plan2 := r.Plan()
	if len(plan2.Changes) != 0 {
		t.Fatalf("expected no changes after apply, got %d", len(plan2.Changes))
	}
}

func TestLoadSpecJSON(t *testing.T) {
	r := NewReconciler(DefaultReconcilerConfig())

	specJSON := `{
		"apiVersion": "feather/v1",
		"kind": "FeatureGroup",
		"metadata": {"name": "user_activity"},
		"spec": {
			"entityType": "user",
			"features": [
				{"name": "login_count", "type": "int64"},
				{"name": "last_login", "type": "timestamp"}
			]
		}
	}`

	spec, validation, err := r.LoadSpecJSON([]byte(specJSON))
	if err != nil {
		t.Fatalf("LoadSpecJSON: %v", err)
	}
	if !validation.Valid {
		t.Fatalf("expected valid spec, got errors: %v", validation.Errors)
	}
	if spec.Metadata.Name != "user_activity" {
		t.Fatalf("expected name 'user_activity', got %q", spec.Metadata.Name)
	}

	// Verify it was loaded as a definition
	defs := r.ListDefinitions()
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}
}

func TestLoadSpecJSON_InvalidJSON(t *testing.T) {
	r := NewReconciler(DefaultReconcilerConfig())
	_, _, err := r.LoadSpecJSON([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadSpecJSON_InvalidSpec(t *testing.T) {
	r := NewReconciler(DefaultReconcilerConfig())
	data, _ := json.Marshal(DeclarativeSpec{Kind: "FeatureGroup", Metadata: SpecMetadata{Name: "x"}})
	_, validation, _ := r.LoadSpecJSON(data)
	if validation.Valid {
		t.Fatal("expected invalid spec (missing apiVersion)")
	}
}

func TestFormatPlan(t *testing.T) {
	plan := &PlanResult{
		Changes: []PlanChange{
			{Name: "users", Action: ActionCreate},
			{Name: "orders", Action: ActionUpdate, Diff: []string{"ttl"}},
			{Name: "old_feature", Action: ActionDelete},
		},
		Summary: PlanSummary{Creates: 1, Updates: 1, Deletes: 1},
	}

	output := FormatPlan(plan)
	if !strings.Contains(output, "+ users") {
		t.Fatal("expected create marker for users")
	}
	if !strings.Contains(output, "~ orders") {
		t.Fatal("expected update marker for orders")
	}
	if !strings.Contains(output, "- old_feature") {
		t.Fatal("expected delete marker for old_feature")
	}
}

func TestRollback(t *testing.T) {
	r := NewReconciler(DefaultReconcilerConfig())
	r.LoadDefinition(FeatureDefinition{Name: "test", EntityType: "user"})
	r.Apply()

	result, err := r.Rollback("test")
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if result.Action != ActionDelete {
		t.Fatalf("expected delete action, got %s", result.Action)
	}
}

func TestRollback_NotFound(t *testing.T) {
	r := NewReconciler(DefaultReconcilerConfig())
	_, err := r.Rollback("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent definition")
	}
}
