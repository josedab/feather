package feastcompat

import (
	"testing"
)

func TestNewMigrationCLI(t *testing.T) {
	cli := NewMigrationCLI()
	if cli == nil {
		t.Fatal("expected non-nil MigrationCLI")
	}
}

func TestMigrationCLIPlan(t *testing.T) {
	cli := NewMigrationCLI()
	config := `
project: my_project
provider: local
online_store:
  type: sqlite
  path: /tmp/online.db
`
	plan, err := cli.Plan(config)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SourceFormat != "feast" {
		t.Errorf("expected source_format 'feast', got %q", plan.SourceFormat)
	}
	if plan.FeatureGroups != 1 {
		t.Errorf("expected 1 feature group, got %d", plan.FeatureGroups)
	}
	if len(plan.Actions) == 0 {
		t.Fatal("expected at least one action")
	}

	// Verify we have storage and serving actions
	hasStorage := false
	hasServing := false
	for _, a := range plan.Actions {
		if a.Resource == "storage" {
			hasStorage = true
		}
		if a.Resource == "serving" {
			hasServing = true
		}
	}
	if !hasStorage {
		t.Error("expected storage configuration action")
	}
	if !hasServing {
		t.Error("expected serving configuration action")
	}
}

func TestMigrationCLIPlanRedis(t *testing.T) {
	cli := NewMigrationCLI()
	config := `
project: redis_project
provider: gcp
online_store:
  type: redis
`
	plan, err := cli.Plan(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Warnings) == 0 {
		t.Error("expected warnings for redis online store")
	}
	hasRedisWarning := false
	for _, w := range plan.Warnings {
		if w != "" {
			hasRedisWarning = true
			break
		}
	}
	if !hasRedisWarning {
		t.Error("expected redis-specific warning")
	}
}

func TestMigrationCLIPlanUnknownStore(t *testing.T) {
	cli := NewMigrationCLI()
	config := `
project: test
online_store:
  type: dynamodb
`
	plan, err := cli.Plan(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Warnings) == 0 {
		t.Error("expected warning for unknown store type")
	}
}

func TestMigrationCLIPlanInvalidYAML(t *testing.T) {
	cli := NewMigrationCLI()
	_, err := cli.Plan("{{invalid yaml")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestMigrationCLIPlanWithOfflineStore(t *testing.T) {
	cli := NewMigrationCLI()
	config := `
project: test
offline_store:
  type: bigquery
`
	plan, err := cli.Plan(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Warnings) == 0 {
		t.Error("expected warning about offline store")
	}
}

func TestMigrationCLIExecute(t *testing.T) {
	cli := NewMigrationCLI()
	config := `
project: my_project
provider: local
online_store:
  type: sqlite
  path: /tmp/online.db
`
	result, err := cli.Execute(config)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected successful migration")
	}
	if result.Config == nil {
		t.Fatal("expected non-nil config")
	}
	if result.FeatherYAML == "" {
		t.Error("expected non-empty YAML output")
	}
}

func TestMigrationCLIExecuteInvalidYAML(t *testing.T) {
	cli := NewMigrationCLI()
	_, err := cli.Execute("{{invalid yaml")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestMigrationCLIValidateSuccess(t *testing.T) {
	cli := NewMigrationCLI()
	config := `
project: my_project
provider: local
online_store:
  type: sqlite
  path: /tmp/online.db
`
	result, err := cli.Execute(config)
	if err != nil {
		t.Fatal(err)
	}

	issues := cli.Validate(result)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestMigrationCLIValidateNil(t *testing.T) {
	cli := NewMigrationCLI()
	issues := cli.Validate(nil)
	if len(issues) == 0 {
		t.Error("expected issues for nil result")
	}
}

func TestMigrationCLIValidateNilConfig(t *testing.T) {
	cli := NewMigrationCLI()
	issues := cli.Validate(&MigrationResult{Success: true})
	hasConfigIssue := false
	for _, issue := range issues {
		if issue == "generated config is nil" {
			hasConfigIssue = true
		}
	}
	if !hasConfigIssue {
		t.Error("expected 'generated config is nil' issue")
	}
}

func TestMigrationCLIValidateMissingPorts(t *testing.T) {
	cli := NewMigrationCLI()
	result := &MigrationResult{
		Success: true,
		Config: &FeatherConfig{
			Storage: FeatherStorage{
				Warm: FeatherWarmStorage{Path: "/tmp"},
				Hot:  FeatherHotStorage{MaxMemory: "1GB"},
			},
		},
		FeatherYAML: "dummy",
	}
	issues := cli.Validate(result)
	hasPortIssue := false
	for _, issue := range issues {
		if issue == "HTTP port is not configured" || issue == "gRPC port is not configured" {
			hasPortIssue = true
		}
	}
	if !hasPortIssue {
		t.Error("expected port configuration issues")
	}
}
