// Package dbt provides integration with dbt (data build tool) for syncing
// feature definitions from dbt models to Feather's feature catalog.
package dbt

import "time"

// Manifest represents a dbt manifest.json file structure.
type Manifest struct {
	Metadata ManifestMetadata   `json:"metadata"`
	Nodes    map[string]Node    `json:"nodes"`
	Sources  map[string]Source  `json:"sources"`
	Metrics  map[string]Metric  `json:"metrics"`
	Docs     map[string]Doc     `json:"docs"`
}

// ManifestMetadata contains metadata about the dbt project.
type ManifestMetadata struct {
	DBTSchemaVersion string    `json:"dbt_schema_version"`
	DBTVersion       string    `json:"dbt_version"`
	GeneratedAt      time.Time `json:"generated_at"`
	InvocationID     string    `json:"invocation_id"`
	ProjectName      string    `json:"project_name"`
	ProjectID        string    `json:"project_id,omitempty"`
	UserID           string    `json:"user_id,omitempty"`
	SendAnonymous    bool      `json:"send_anonymous_usage_stats"`
}

// Node represents a dbt model, seed, snapshot, or other node type.
type Node struct {
	UniqueID       string            `json:"unique_id"`
	Name           string            `json:"name"`
	ResourceType   string            `json:"resource_type"` // model, seed, snapshot, test, source
	PackageName    string            `json:"package_name"`
	Path           string            `json:"path"`
	OriginalPath   string            `json:"original_file_path"`
	Description    string            `json:"description"`
	Schema         string            `json:"schema"`
	Database       string            `json:"database"`
	Alias          string            `json:"alias,omitempty"`
	Columns        map[string]Column `json:"columns"`
	Config         NodeConfig        `json:"config"`
	Tags           []string          `json:"tags"`
	Meta           map[string]any    `json:"meta"`
	DependsOn      DependsOn         `json:"depends_on"`
	Refs           [][]string        `json:"refs"`
	Sources        [][]string        `json:"sources"`
	Materialized   string            `json:"materialized,omitempty"`
	RelationName   string            `json:"relation_name,omitempty"`
	CreatedAt      float64           `json:"created_at"`
}

// Column represents a column in a dbt model.
type Column struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	DataType    string         `json:"data_type,omitempty"`
	Meta        map[string]any `json:"meta"`
	Tags        []string       `json:"tags"`
	Constraints []Constraint   `json:"constraints,omitempty"`
}

// Constraint represents a column constraint.
type Constraint struct {
	Type       string `json:"type"`
	Expression string `json:"expression,omitempty"`
}

// NodeConfig represents dbt node configuration.
type NodeConfig struct {
	Enabled       bool           `json:"enabled"`
	Materialized  string         `json:"materialized,omitempty"`
	Schema        string         `json:"schema,omitempty"`
	Tags          []string       `json:"tags"`
	Meta          map[string]any `json:"meta"`
	GrantAccessTo []GrantAccess  `json:"grant_access_to,omitempty"`
}

// GrantAccess represents grant access configuration.
type GrantAccess struct {
	Database string `json:"database"`
	Project  string `json:"project"`
}

// DependsOn represents node dependencies.
type DependsOn struct {
	Macros []string `json:"macros"`
	Nodes  []string `json:"nodes"`
}

// Source represents a dbt source.
type Source struct {
	UniqueID     string            `json:"unique_id"`
	Name         string            `json:"name"`
	SourceName   string            `json:"source_name"`
	Description  string            `json:"description"`
	Database     string            `json:"database"`
	Schema       string            `json:"schema"`
	Identifier   string            `json:"identifier"`
	Columns      map[string]Column `json:"columns"`
	Meta         map[string]any    `json:"meta"`
	Tags         []string          `json:"tags"`
	RelationName string            `json:"relation_name"`
}

// Metric represents a dbt metric.
type Metric struct {
	UniqueID    string         `json:"unique_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Label       string         `json:"label"`
	Type        string         `json:"type"` // simple, derived, cumulative
	TypeParams  MetricParams   `json:"type_params"`
	Filter      string         `json:"filter,omitempty"`
	Meta        map[string]any `json:"meta"`
	Tags        []string       `json:"tags"`
}

// MetricParams represents metric type parameters.
type MetricParams struct {
	Measure     string   `json:"measure,omitempty"`
	Expression  string   `json:"expression,omitempty"`
	Window      string   `json:"window,omitempty"`
	GrainToDate string   `json:"grain_to_date,omitempty"`
	Metrics     []string `json:"metrics,omitempty"`
}

// Doc represents a dbt documentation block.
type Doc struct {
	UniqueID     string `json:"unique_id"`
	Name         string `json:"name"`
	PackageName  string `json:"package_name"`
	BlockContent string `json:"block_contents"`
}

// FeatureDefinition represents a feature definition derived from dbt.
type FeatureDefinition struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	DataType    string            `json:"data_type"`
	EntityType  string            `json:"entity_type"`
	Owner       string            `json:"owner,omitempty"`
	Team        string            `json:"team,omitempty"`
	Tags        []string          `json:"tags"`
	Category    string            `json:"category,omitempty"`
	Status      string            `json:"status"`
	Version     int               `json:"version"`
	Metadata    map[string]string `json:"metadata"`
	Source      FeatureSource     `json:"source"`
}

// FeatureSource tracks the dbt source of a feature.
type FeatureSource struct {
	Type         string `json:"type"` // dbt
	DBTUniqueID  string `json:"dbt_unique_id"`
	DBTModelName string `json:"dbt_model_name"`
	DBTProject   string `json:"dbt_project"`
	ColumnName   string `json:"column_name"`
	SyncedAt     string `json:"synced_at"`
}

// SyncResult represents the result of a dbt sync operation.
type SyncResult struct {
	Success          bool                `json:"success"`
	FeaturesCreated  int                 `json:"features_created"`
	FeaturesUpdated  int                 `json:"features_updated"`
	FeaturesSkipped  int                 `json:"features_skipped"`
	Errors           []SyncError         `json:"errors,omitempty"`
	Features         []FeatureDefinition `json:"features,omitempty"`
	SyncedAt         time.Time           `json:"synced_at"`
	ManifestVersion  string              `json:"manifest_version"`
	ProjectName      string              `json:"project_name"`
}

// SyncError represents an error during sync.
type SyncError struct {
	ModelName string `json:"model_name"`
	Column    string `json:"column,omitempty"`
	Message   string `json:"message"`
}

// SyncOptions configures the sync behavior.
type SyncOptions struct {
	// DryRun if true, validates but doesn't persist changes
	DryRun bool `json:"dry_run"`
	// Tags filters models by tag (empty means all)
	Tags []string `json:"tags,omitempty"`
	// Models filters by model name patterns (supports glob)
	Models []string `json:"models,omitempty"`
	// IncludeSources includes dbt sources in sync
	IncludeSources bool `json:"include_sources"`
	// IncludeMetrics includes dbt metrics as features
	IncludeMetrics bool `json:"include_metrics"`
	// EntityTypeMapping maps dbt model tags to entity types
	EntityTypeMapping map[string]string `json:"entity_type_mapping,omitempty"`
	// DefaultEntityType used when no mapping matches
	DefaultEntityType string `json:"default_entity_type"`
	// Owner to assign to all features
	Owner string `json:"owner,omitempty"`
	// Team to assign to all features
	Team string `json:"team,omitempty"`
}

// DefaultSyncOptions returns default sync options.
func DefaultSyncOptions() *SyncOptions {
	return &SyncOptions{
		DryRun:            false,
		IncludeSources:    false,
		IncludeMetrics:    true,
		DefaultEntityType: "unknown",
	}
}
