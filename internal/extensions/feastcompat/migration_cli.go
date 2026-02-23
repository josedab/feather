package feastcompat

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// MigrationPlan describes the steps a migration will perform.
type MigrationPlan struct {
	SourceFormat  string            `json:"source_format"`
	Actions       []MigrationAction `json:"actions"`
	Warnings      []string          `json:"warnings"`
	FeatureGroups int               `json:"feature_groups"`
	FeatureViews  int               `json:"feature_views"`
}

// MigrationAction describes a single migration step.
type MigrationAction struct {
	Type        string `json:"type"` // create, map, configure
	Resource    string `json:"resource"`
	Description string `json:"description"`
}

// MigrationCLI provides CLI helpers for migrating from Feast to Feather.
type MigrationCLI struct{}

// NewMigrationCLI creates a new migration CLI helper.
func NewMigrationCLI() *MigrationCLI {
	return &MigrationCLI{}
}

// Plan parses a Feast feature_store.yaml and returns a dry-run migration plan.
func (m *MigrationCLI) Plan(feastConfig string) (*MigrationPlan, error) {
	var feast FeastConfig
	if err := yaml.Unmarshal([]byte(feastConfig), &feast); err != nil {
		return nil, fmt.Errorf("parsing feast config: %w", err)
	}

	plan := &MigrationPlan{
		SourceFormat: "feast",
	}

	// Plan storage configuration
	if feast.OnlineStore != nil {
		plan.Actions = append(plan.Actions, MigrationAction{
			Type:        "configure",
			Resource:    "storage",
			Description: fmt.Sprintf("Configure storage from Feast online store type %q", feast.OnlineStore.Type),
		})

		switch strings.ToLower(feast.OnlineStore.Type) {
		case "redis":
			plan.Warnings = append(plan.Warnings,
				"Redis online store will be mapped to Feather hot tier; consider increasing max_memory")
		case "sqlite":
			if feast.OnlineStore.Path != "" {
				plan.Actions = append(plan.Actions, MigrationAction{
					Type:        "map",
					Resource:    "storage.warm.path",
					Description: fmt.Sprintf("Map SQLite path %q to warm storage", feast.OnlineStore.Path),
				})
			}
		default:
			plan.Warnings = append(plan.Warnings,
				fmt.Sprintf("Unknown online store type %q; Feather defaults will be used", feast.OnlineStore.Type))
		}
	}

	// Plan offline store
	if feast.OfflineStore != nil {
		plan.Warnings = append(plan.Warnings,
			fmt.Sprintf("Offline store type %q has no direct Feather equivalent; warm tier will be used", feast.OfflineStore.Type))
	}

	// Plan project mapping
	if feast.Project != "" {
		plan.Actions = append(plan.Actions, MigrationAction{
			Type:        "create",
			Resource:    "feature_group:" + feast.Project,
			Description: fmt.Sprintf("Create default feature group for project %q", feast.Project),
		})
		plan.FeatureGroups++
	}

	// Plan serving configuration
	plan.Actions = append(plan.Actions, MigrationAction{
		Type:        "configure",
		Resource:    "serving",
		Description: "Configure HTTP (8080) and gRPC (50051) serving endpoints",
	})

	plan.Actions = append(plan.Actions, MigrationAction{
		Type:        "configure",
		Resource:    "ingestion",
		Description: "Enable HTTP ingestion on port 8081",
	})

	return plan, nil
}

// Execute performs the migration from Feast config to Feather config.
func (m *MigrationCLI) Execute(feastConfig string) (*MigrationResult, error) {
	return MigrateFromFeastConfig([]byte(feastConfig))
}

// Validate checks a migration result for potential issues.
func (m *MigrationCLI) Validate(result *MigrationResult) []string {
	if result == nil {
		return []string{"migration result is nil"}
	}

	var issues []string

	if !result.Success {
		issues = append(issues, "migration was not successful")
	}

	if result.Config == nil {
		issues = append(issues, "generated config is nil")
		return issues
	}

	if result.Config.Serving.HTTP.Port == 0 {
		issues = append(issues, "HTTP port is not configured")
	}

	if result.Config.Serving.GRPC.Port == 0 {
		issues = append(issues, "gRPC port is not configured")
	}

	if result.Config.Storage.Warm.Path == "" {
		issues = append(issues, "warm storage path is empty")
	}

	if result.Config.Storage.Hot.MaxMemory == "" {
		issues = append(issues, "hot storage max_memory is not set")
	}

	if result.FeatherYAML == "" {
		issues = append(issues, "generated YAML is empty")
	}

	return issues
}
