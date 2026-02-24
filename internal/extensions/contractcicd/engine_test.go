package contractcicd

import (
	"testing"
)

func TestEngineRegisterContract(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	contract := &Contract{
		APIVersion: "feather/v1",
		Kind:       "FeatureContract",
		Metadata:   ContractMeta{Name: "user_features", Owner: "ml-team"},
		Spec: ContractSpec{
			EntityType: "user",
			Features: []FeatureContract{
				{Name: "click_count", Type: "int64"},
				{Name: "last_login", Type: "timestamp"},
			},
		},
	}
	if err := engine.RegisterContract(contract); err != nil {
		t.Fatal(err)
	}

	got, err := engine.GetContract("user_features")
	if err != nil {
		t.Fatal(err)
	}
	if got.Metadata.Name != "user_features" {
		t.Errorf("expected user_features, got %s", got.Metadata.Name)
	}
}

func TestEngineDuplicateContract(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	contract := &Contract{
		Metadata: ContractMeta{Name: "test"},
		Spec:     ContractSpec{EntityType: "user", Features: []FeatureContract{{Name: "x", Type: "int64"}}},
	}
	_ = engine.RegisterContract(contract)
	err := engine.RegisterContract(contract)
	if err != ErrContractExists {
		t.Errorf("expected ErrContractExists, got %v", err)
	}
}

func TestEngineDetectBreakingChanges(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	original := &Contract{
		Metadata: ContractMeta{Name: "features"},
		Spec: ContractSpec{
			EntityType: "user",
			Features: []FeatureContract{
				{Name: "age", Type: "int64"},
				{Name: "name", Type: "string"},
			},
		},
	}
	_ = engine.RegisterContract(original)

	// Remove a feature (breaking).
	updated := &Contract{
		Metadata: ContractMeta{Name: "features"},
		Spec: ContractSpec{
			EntityType: "user",
			Features: []FeatureContract{
				{Name: "age", Type: "int64"},
			},
		},
	}
	plan, err := engine.UpdateContract(updated)
	if err != ErrBreakingChange {
		t.Errorf("expected ErrBreakingChange, got %v", err)
	}
	if plan == nil || !plan.HasBreakingChanges() {
		t.Error("expected breaking changes in plan")
	}
}

func TestEngineDetectTypeChange(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	original := &Contract{
		Metadata: ContractMeta{Name: "features"},
		Spec: ContractSpec{
			EntityType: "user",
			Features:   []FeatureContract{{Name: "score", Type: "int64"}},
		},
	}
	_ = engine.RegisterContract(original)

	updated := &Contract{
		Metadata: ContractMeta{Name: "features"},
		Spec: ContractSpec{
			EntityType: "user",
			Features:   []FeatureContract{{Name: "score", Type: "float64"}},
		},
	}
	plan, err := engine.UpdateContract(updated)
	if err != ErrBreakingChange {
		t.Errorf("expected ErrBreakingChange, got %v", err)
	}
	if plan.BreakingChanges != 1 {
		t.Errorf("expected 1 breaking change, got %d", plan.BreakingChanges)
	}
}

func TestEngineNonBreakingUpdate(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())
	original := &Contract{
		Metadata: ContractMeta{Name: "features"},
		Spec: ContractSpec{
			EntityType: "user",
			Features:   []FeatureContract{{Name: "age", Type: "int64"}},
		},
	}
	_ = engine.RegisterContract(original)

	// Add a feature (non-breaking).
	updated := &Contract{
		Metadata: ContractMeta{Name: "features"},
		Spec: ContractSpec{
			EntityType: "user",
			Features: []FeatureContract{
				{Name: "age", Type: "int64"},
				{Name: "email", Type: "string"},
			},
		},
	}
	plan, err := engine.UpdateContract(updated)
	if err != nil {
		t.Fatal(err)
	}
	if plan.HasBreakingChanges() {
		t.Error("expected no breaking changes")
	}
	if plan.Additions != 1 {
		t.Errorf("expected 1 addition, got %d", plan.Additions)
	}
}

func TestEngineValidate(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())

	errs := engine.Validate(&Contract{})
	if len(errs) < 2 {
		t.Errorf("expected validation errors, got %d", len(errs))
	}

	errs = engine.Validate(&Contract{
		Metadata: ContractMeta{Name: "test"},
		Spec: ContractSpec{
			EntityType: "user",
			Features:   []FeatureContract{{Name: "x", Type: "invalid"}},
		},
	})
	if len(errs) != 1 {
		t.Errorf("expected 1 validation error for invalid type, got %d", len(errs))
	}
}

func TestEngineGenerateCITemplate(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())

	gh := engine.GenerateCITemplate("github")
	if gh.Provider != "github" {
		t.Errorf("expected github provider")
	}
	if gh.Content == "" {
		t.Error("expected non-empty content")
	}

	gl := engine.GenerateCITemplate("gitlab")
	if gl.Provider != "gitlab" {
		t.Errorf("expected gitlab provider")
	}

	generic := engine.GenerateCITemplate("other")
	if generic.Provider != "generic" {
		t.Errorf("expected generic provider")
	}
}

func TestEnginePlanFromContracts(t *testing.T) {
	t.Parallel()
	engine := NewEngine(DefaultEngineConfig())

	contracts := []*Contract{
		{
			Metadata: ContractMeta{Name: "new_features"},
			Spec: ContractSpec{
				EntityType: "user",
				Features: []FeatureContract{
					{Name: "score", Type: "float64"},
				},
			},
		},
	}

	plan, err := engine.PlanFromContracts(contracts)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Additions != 1 {
		t.Errorf("expected 1 addition, got %d", plan.Additions)
	}
}

func TestEngineDeprecateFeature(t *testing.T) {
t.Parallel()
engine := NewEngine(DefaultEngineConfig())
contract := &Contract{
Metadata: ContractMeta{Name: "features"},
Spec: ContractSpec{
EntityType: "user",
Features: []FeatureContract{
{Name: "old_metric", Type: "float64"},
},
},
}
_ = engine.RegisterContract(contract)
if err := engine.DeprecateFeature("features", "old_metric", "use new_metric"); err != nil {
t.Fatal(err)
}
c, _ := engine.GetContract("features")
for _, f := range c.Spec.Features {
if f.Name == "old_metric" && !f.Deprecated {
t.Error("expected old_metric to be deprecated")
}
}
}

func TestEngineMigrationGeneration(t *testing.T) {
t.Parallel()
engine := NewEngine(DefaultEngineConfig())
_ = engine.RegisterContract(&Contract{
Metadata: ContractMeta{Name: "f"},
Spec:     ContractSpec{EntityType: "user", Features: []FeatureContract{{Name: "age", Type: "int64"}}},
})
plan, err := engine.UpdateContract(&Contract{
Metadata: ContractMeta{Name: "f"},
Spec:     ContractSpec{EntityType: "user", Features: []FeatureContract{{Name: "age", Type: "int64"}, {Name: "email", Type: "string"}}},
})
if err != nil {
t.Fatal(err)
}
if len(plan.Migrations) == 0 {
t.Error("expected migrations to be generated")
}
}

func TestEngineDiffSummary(t *testing.T) {
t.Parallel()
engine := NewEngine(DefaultEngineConfig())
_ = engine.RegisterContract(&Contract{
Metadata: ContractMeta{Name: "f"},
Spec:     ContractSpec{EntityType: "user", Features: []FeatureContract{{Name: "x", Type: "int64"}}},
})
plan, _ := engine.UpdateContract(&Contract{
Metadata: ContractMeta{Name: "f"},
Spec:     ContractSpec{EntityType: "user", Features: []FeatureContract{{Name: "x", Type: "int64"}, {Name: "y", Type: "string"}}},
})
summary, err := engine.DiffSummary(plan.ID)
if err != nil {
t.Fatal(err)
}
if summary == "" {
t.Error("expected non-empty summary")
}
}

func TestEngineDeleteContract(t *testing.T) {
t.Parallel()
engine := NewEngine(DefaultEngineConfig())
_ = engine.RegisterContract(&Contract{
Metadata: ContractMeta{Name: "temp"},
Spec:     ContractSpec{EntityType: "user", Features: []FeatureContract{{Name: "x", Type: "int64"}}},
})
if err := engine.DeleteContract("temp"); err != nil {
t.Fatal(err)
}
_, err := engine.GetContract("temp")
if err != ErrContractNotFound {
t.Error("expected contract to be deleted")
}
}
