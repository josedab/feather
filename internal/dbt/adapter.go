package dbt

import (
	"fmt"
	"strings"
	"time"
)

// Adapter converts dbt manifest data to Feather feature definitions.
type Adapter struct {
	options *SyncOptions
}

// NewAdapter creates a new dbt adapter with the given options.
func NewAdapter(options *SyncOptions) *Adapter {
	if options == nil {
		options = DefaultSyncOptions()
	}
	return &Adapter{options: options}
}

// SyncManifest converts a dbt manifest to Feather feature definitions.
func (a *Adapter) SyncManifest(manifest *Manifest) (*SyncResult, error) {
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("validating manifest: %w", err)
	}

	result := &SyncResult{
		Success:         true,
		SyncedAt:        time.Now(),
		ManifestVersion: manifest.Metadata.DBTSchemaVersion,
		ProjectName:     manifest.Metadata.ProjectName,
	}

	// Process models
	models := manifest.FilterModels(a.options.Tags, a.options.Models)
	for _, model := range models {
		features, errors := a.convertModelToFeatures(model, manifest)
		result.Features = append(result.Features, features...)
		result.Errors = append(result.Errors, errors...)
	}

	// Process sources if enabled
	if a.options.IncludeSources {
		for _, source := range manifest.Sources {
			features, errors := a.convertSourceToFeatures(source, manifest)
			result.Features = append(result.Features, features...)
			result.Errors = append(result.Errors, errors...)
		}
	}

	// Process metrics if enabled
	if a.options.IncludeMetrics {
		for _, metric := range manifest.Metrics {
			feature, err := a.convertMetricToFeature(metric, manifest)
			if err != nil {
				result.Errors = append(result.Errors, SyncError{
					ModelName: metric.Name,
					Message:   err.Error(),
				})
				continue
			}
			result.Features = append(result.Features, *feature)
		}
	}

	// Count features
	result.FeaturesCreated = len(result.Features)
	if len(result.Errors) > 0 {
		result.Success = false
	}

	return result, nil
}

// convertModelToFeatures converts a dbt model to feature definitions.
func (a *Adapter) convertModelToFeatures(model Node, manifest *Manifest) ([]FeatureDefinition, []SyncError) {
	features := make([]FeatureDefinition, 0, len(model.Columns))
	errors := make([]SyncError, 0, len(model.Columns))

	// Skip disabled models
	if !model.Config.Enabled {
		return nil, nil
	}

	// Determine entity type from tags or mapping
	entityType := a.determineEntityType(model.Tags)

	// Convert each column to a feature
	for colName, column := range model.Columns {
		feature, err := a.convertColumnToFeature(model, column, colName, entityType, manifest)
		if err != nil {
			errors = append(errors, SyncError{
				ModelName: model.Name,
				Column:    colName,
				Message:   err.Error(),
			})
			continue
		}
		features = append(features, *feature)
	}

	return features, errors
}

// convertColumnToFeature converts a dbt column to a feature definition.
func (a *Adapter) convertColumnToFeature(model Node, column Column, colName, entityType string, manifest *Manifest) (*FeatureDefinition, error) {
	// Generate feature name: model_name.column_name
	featureName := fmt.Sprintf("%s.%s", model.Name, colName)

	// Map dbt data type to Feather data type
	dataType := a.mapDataType(column.DataType)

	// Build tags
	tags := append(model.Tags, column.Tags...)
	tags = append(tags, "dbt")

	// Extract metadata from column meta
	metadata := make(map[string]string)
	for k, v := range column.Meta {
		metadata[k] = fmt.Sprintf("%v", v)
	}
	metadata["dbt_model"] = model.Name
	metadata["dbt_project"] = manifest.Metadata.ProjectName

	// Determine owner and team
	owner := a.options.Owner
	team := a.options.Team
	if v, ok := model.Meta["owner"].(string); ok && v != "" {
		owner = v
	}
	if v, ok := model.Meta["team"].(string); ok && v != "" {
		team = v
	}

	// Determine category from tags
	category := "uncategorized"
	for _, tag := range tags {
		if strings.HasPrefix(tag, "category:") {
			category = strings.TrimPrefix(tag, "category:")
			break
		}
	}

	return &FeatureDefinition{
		Name:        featureName,
		Description: column.Description,
		DataType:    dataType,
		EntityType:  entityType,
		Owner:       owner,
		Team:        team,
		Tags:        unique(tags),
		Category:    category,
		Status:      "active",
		Version:     1,
		Metadata:    metadata,
		Source: FeatureSource{
			Type:         "dbt",
			DBTUniqueID:  model.UniqueID,
			DBTModelName: model.Name,
			DBTProject:   manifest.Metadata.ProjectName,
			ColumnName:   colName,
			SyncedAt:     time.Now().Format(time.RFC3339),
		},
	}, nil
}

// convertSourceToFeatures converts a dbt source to feature definitions.
func (a *Adapter) convertSourceToFeatures(source Source, manifest *Manifest) ([]FeatureDefinition, []SyncError) {
	features := make([]FeatureDefinition, 0, len(source.Columns))
	errors := make([]SyncError, 0, len(source.Columns))

	entityType := a.determineEntityType(source.Tags)

	for colName, column := range source.Columns {
		featureName := fmt.Sprintf("%s.%s.%s", source.SourceName, source.Name, colName)
		dataType := a.mapDataType(column.DataType)

		tags := append(source.Tags, column.Tags...)
		tags = append(tags, "dbt", "source")

		metadata := make(map[string]string)
		for k, v := range column.Meta {
			metadata[k] = fmt.Sprintf("%v", v)
		}
		metadata["dbt_source"] = source.SourceName
		metadata["dbt_source_table"] = source.Name

		feature := &FeatureDefinition{
			Name:        featureName,
			Description: column.Description,
			DataType:    dataType,
			EntityType:  entityType,
			Owner:       a.options.Owner,
			Team:        a.options.Team,
			Tags:        unique(tags),
			Category:    "source",
			Status:      "active",
			Version:     1,
			Metadata:    metadata,
			Source: FeatureSource{
				Type:         "dbt",
				DBTUniqueID:  source.UniqueID,
				DBTModelName: source.Name,
				DBTProject:   manifest.Metadata.ProjectName,
				ColumnName:   colName,
				SyncedAt:     time.Now().Format(time.RFC3339),
			},
		}
		features = append(features, *feature)
	}

	return features, errors
}

// convertMetricToFeature converts a dbt metric to a feature definition.
func (a *Adapter) convertMetricToFeature(metric Metric, manifest *Manifest) (*FeatureDefinition, error) {
	tags := append(metric.Tags, "dbt", "metric")

	metadata := make(map[string]string)
	for k, v := range metric.Meta {
		metadata[k] = fmt.Sprintf("%v", v)
	}
	metadata["metric_type"] = metric.Type

	return &FeatureDefinition{
		Name:        metric.Name,
		Description: metric.Description,
		DataType:    "float64", // Most metrics are numeric
		EntityType:  a.options.DefaultEntityType,
		Owner:       a.options.Owner,
		Team:        a.options.Team,
		Tags:        unique(tags),
		Category:    "metric",
		Status:      "active",
		Version:     1,
		Metadata:    metadata,
		Source: FeatureSource{
			Type:         "dbt",
			DBTUniqueID:  metric.UniqueID,
			DBTModelName: metric.Name,
			DBTProject:   manifest.Metadata.ProjectName,
			SyncedAt:     time.Now().Format(time.RFC3339),
		},
	}, nil
}

// determineEntityType determines the entity type from tags.
func (a *Adapter) determineEntityType(tags []string) string {
	// Check explicit mapping
	if a.options.EntityTypeMapping != nil {
		for _, tag := range tags {
			if entityType, ok := a.options.EntityTypeMapping[tag]; ok {
				return entityType
			}
		}
	}

	// Check for entity:* tags
	for _, tag := range tags {
		if strings.HasPrefix(tag, "entity:") {
			return strings.TrimPrefix(tag, "entity:")
		}
	}

	return a.options.DefaultEntityType
}

// mapDataType maps dbt/SQL data types to Feather data types.
func (a *Adapter) mapDataType(dbtType string) string {
	// Normalize to lowercase
	t := strings.ToLower(strings.TrimSpace(dbtType))

	// Handle empty or unknown types
	if t == "" {
		return "string"
	}

	// Array/Vector types (check first to handle ARRAY<FLOAT> etc.)
	switch {
	case strings.Contains(t, "array"):
		return "vector"
	case strings.Contains(t, "vector"):
		return "vector"

	// Integer types
	case strings.Contains(t, "int"):
		return "int64"
	case strings.Contains(t, "bigint"):
		return "int64"
	case strings.Contains(t, "smallint"):
		return "int64"
	case strings.Contains(t, "tinyint"):
		return "int64"

	// Float types
	case strings.Contains(t, "float"):
		return "float64"
	case strings.Contains(t, "double"):
		return "float64"
	case strings.Contains(t, "decimal"):
		return "float64"
	case strings.Contains(t, "numeric"):
		return "float64"
	case strings.Contains(t, "real"):
		return "float64"

	// Boolean types
	case strings.Contains(t, "bool"):
		return "bool"

	// Timestamp/Date types
	case strings.Contains(t, "timestamp"):
		return "timestamp"
	case strings.Contains(t, "datetime"):
		return "timestamp"
	case strings.Contains(t, "date"):
		return "timestamp"

	// Binary types
	case strings.Contains(t, "binary"):
		return "bytes"
	case strings.Contains(t, "blob"):
		return "bytes"

	// Default to string
	default:
		return "string"
	}
}

// unique returns unique strings from a slice.
func unique(s []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// ValidateManifest validates a manifest without syncing.
func (a *Adapter) ValidateManifest(manifest *Manifest) (*SyncResult, error) {
	// Store original DryRun setting
	originalDryRun := a.options.DryRun
	a.options.DryRun = true
	defer func() { a.options.DryRun = originalDryRun }()

	return a.SyncManifest(manifest)
}
