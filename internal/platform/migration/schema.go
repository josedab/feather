package migration

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

// Common errors
var (
	ErrInvalidFeastSchema = errors.New("invalid Feast schema")
	ErrUnsupportedType    = errors.New("unsupported data type")
	ErrMigrationFailed    = errors.New("migration failed")
)

// FeastValueType represents Feast value types.
type FeastValueType string

// FeastValueType constants.
const (
	FeastTypeBool          FeastValueType = "BOOL"
	FeastTypeInt32         FeastValueType = "INT32"
	FeastTypeInt64         FeastValueType = "INT64"
	FeastTypeFloat         FeastValueType = "FLOAT"
	FeastTypeDouble        FeastValueType = "DOUBLE"
	FeastTypeString        FeastValueType = "STRING"
	FeastTypeBytes         FeastValueType = "BYTES"
	FeastTypeBoolList      FeastValueType = "BOOL_LIST"
	FeastTypeInt32List     FeastValueType = "INT32_LIST"
	FeastTypeInt64List     FeastValueType = "INT64_LIST"
	FeastTypeFloatList     FeastValueType = "FLOAT_LIST"
	FeastTypeStringList    FeastValueType = "STRING_LIST"
	FeastTypeUnixTimestamp FeastValueType = "UNIX_TIMESTAMP"
)

// FeastEntity represents a Feast entity definition.
type FeastEntity struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	ValueType   FeastValueType    `json:"value_type" yaml:"value_type"`
	JoinKey     string            `json:"join_key,omitempty" yaml:"join_key,omitempty"`
	Tags        map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// FeastFeature represents a Feast feature definition.
type FeastFeature struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	ValueType   FeastValueType    `json:"value_type" yaml:"value_type"`
	Tags        map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// FeastFeatureView represents a Feast feature view definition.
type FeastFeatureView struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Entities    []string          `json:"entities" yaml:"entities"`
	Features    []FeastFeature    `json:"features" yaml:"features"`
	TTL         *time.Duration    `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Online      bool              `json:"online" yaml:"online"`
	Offline     bool              `json:"offline,omitempty" yaml:"offline,omitempty"`
	Source      *FeastDataSource  `json:"source,omitempty" yaml:"source,omitempty"`
	Tags        map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// FeastOnDemandFeatureView represents a Feast on-demand feature view.
type FeastOnDemandFeatureView struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Features    []FeastFeature    `json:"features" yaml:"features"`
	Sources     map[string]string `json:"sources,omitempty" yaml:"sources,omitempty"`
	UDF         string            `json:"udf,omitempty" yaml:"udf,omitempty"`
	Tags        map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// FeastDataSource represents a Feast data source.
type FeastDataSource struct {
	Type             string            `json:"type" yaml:"type"`
	Name             string            `json:"name,omitempty" yaml:"name,omitempty"`
	Path             string            `json:"path,omitempty" yaml:"path,omitempty"`
	Query            string            `json:"query,omitempty" yaml:"query,omitempty"`
	Table            string            `json:"table,omitempty" yaml:"table,omitempty"`
	EventTimestamp   string            `json:"event_timestamp_column,omitempty" yaml:"event_timestamp_column,omitempty"`
	CreatedTimestamp string            `json:"created_timestamp_column,omitempty" yaml:"created_timestamp_column,omitempty"`
	FieldMapping     map[string]string `json:"field_mapping,omitempty" yaml:"field_mapping,omitempty"`
}

// FeastProject represents a complete Feast project.
type FeastProject struct {
	Name                 string                     `json:"project" yaml:"project"`
	Description          string                     `json:"description,omitempty" yaml:"description,omitempty"`
	Provider             string                     `json:"provider,omitempty" yaml:"provider,omitempty"`
	Registry             string                     `json:"registry,omitempty" yaml:"registry,omitempty"`
	OnlineStore          map[string]interface{}     `json:"online_store,omitempty" yaml:"online_store,omitempty"`
	OfflineStore         map[string]interface{}     `json:"offline_store,omitempty" yaml:"offline_store,omitempty"`
	Entities             []FeastEntity              `json:"entities,omitempty" yaml:"entities,omitempty"`
	FeatureViews         []FeastFeatureView         `json:"feature_views,omitempty" yaml:"feature_views,omitempty"`
	OnDemandFeatureViews []FeastOnDemandFeatureView `json:"on_demand_feature_views,omitempty" yaml:"on_demand_feature_views,omitempty"`
	FeatureServices      []FeastFeatureService      `json:"feature_services,omitempty" yaml:"feature_services,omitempty"`
}

// FeastFeatureService represents a Feast feature service.
type FeastFeatureService struct {
	Name        string                  `json:"name" yaml:"name"`
	Description string                  `json:"description,omitempty" yaml:"description,omitempty"`
	Features    []FeastFeatureReference `json:"features" yaml:"features"`
	Tags        map[string]string       `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// FeastFeatureReference represents a reference to a feature in a feature service.
type FeastFeatureReference struct {
	FeatureViewName string   `json:"feature_view_name" yaml:"feature_view_name"`
	FeatureNames    []string `json:"feature_names,omitempty" yaml:"feature_names,omitempty"`
}

// SchemaConverter converts Feast schemas to Feather format.
type SchemaConverter struct {
	config SchemaConverterConfig
}

// SchemaConverterConfig configures the schema converter.
type SchemaConverterConfig struct {
	// DefaultTTL is used when Feast feature view doesn't specify TTL
	DefaultTTL time.Duration
	// PreserveMetadata keeps Feast metadata in feature tags
	PreserveMetadata bool
	// NameMapping provides custom name transformations
	NameMapping map[string]string
}

// DefaultSchemaConverterConfig returns sensible defaults.
func DefaultSchemaConverterConfig() SchemaConverterConfig {
	return SchemaConverterConfig{
		DefaultTTL:       5 * time.Minute,
		PreserveMetadata: true,
		NameMapping:      make(map[string]string),
	}
}

// NewSchemaConverter creates a new schema converter.
func NewSchemaConverter(config SchemaConverterConfig) *SchemaConverter {
	return &SchemaConverter{
		config: config,
	}
}

// ConvertResult contains the results of a schema conversion.
type ConvertResult struct {
	FeatureGroups []domain.FeatureGroup `json:"feature_groups"`
	Warnings      []string              `json:"warnings,omitempty"`
	Errors        []string              `json:"errors,omitempty"`
}

// ConvertProject converts a complete Feast project to Feather format.
func (c *SchemaConverter) ConvertProject(project *FeastProject) (*ConvertResult, error) {
	if project == nil {
		return nil, ErrInvalidFeastSchema
	}

	result := &ConvertResult{
		FeatureGroups: make([]domain.FeatureGroup, 0),
		Warnings:      make([]string, 0),
		Errors:        make([]string, 0),
	}

	// Convert feature views to feature groups
	for _, fv := range project.FeatureViews {
		group, warnings, err := c.ConvertFeatureView(&fv, project.Entities)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("feature view '%s': %v", fv.Name, err))
			continue
		}
		result.FeatureGroups = append(result.FeatureGroups, *group)
		result.Warnings = append(result.Warnings, *warnings...)
	}

	// Convert on-demand feature views
	for _, odfv := range project.OnDemandFeatureViews {
		group, warnings, err := c.ConvertOnDemandFeatureView(&odfv)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("on-demand view '%s': %v", odfv.Name, err))
			continue
		}
		result.FeatureGroups = append(result.FeatureGroups, *group)
		result.Warnings = append(result.Warnings, *warnings...)
	}

	return result, nil
}

// ConvertFeatureView converts a Feast feature view to a Feather feature group.
func (c *SchemaConverter) ConvertFeatureView(fv *FeastFeatureView, entities []FeastEntity) (*domain.FeatureGroup, *[]string, error) {
	if fv == nil {
		return nil, nil, ErrInvalidFeastSchema
	}

	warnings := make([]string, 0)

	// Determine entity type from the first entity
	entityType := "entity"
	if len(fv.Entities) > 0 {
		entityType = fv.Entities[0]
	}

	// Convert features
	features := make([]domain.FeatureSpec, 0, len(fv.Features))
	for _, f := range fv.Features {
		spec, err := c.convertFeature(&f)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("feature '%s': %v", f.Name, err))
			continue
		}
		features = append(features, *spec)
	}

	// Determine TTL
	ttl := c.config.DefaultTTL
	if fv.TTL != nil && *fv.TTL > 0 {
		ttl = *fv.TTL
	}

	// Build feature group
	group := &domain.FeatureGroup{
		Name:        c.mapName(fv.Name),
		Description: fv.Description,
		EntityType:  entityType,
		Features:    features,
		TTL:         ttl,
	}

	// Preserve metadata in Tags if configured
	if c.config.PreserveMetadata && len(fv.Tags) > 0 {
		group.Tags = fv.Tags
	}

	return group, &warnings, nil
}

// ConvertOnDemandFeatureView converts a Feast on-demand feature view.
func (c *SchemaConverter) ConvertOnDemandFeatureView(odfv *FeastOnDemandFeatureView) (*domain.FeatureGroup, *[]string, error) {
	if odfv == nil {
		return nil, nil, ErrInvalidFeastSchema
	}

	warnings := make([]string, 0)

	// On-demand views are derived features - we mark them accordingly
	features := make([]domain.FeatureSpec, 0, len(odfv.Features))
	for _, f := range odfv.Features {
		spec, err := c.convertFeature(&f)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("feature '%s': %v", f.Name, err))
			continue
		}
		if odfv.UDF != "" {
			warnings = append(warnings, fmt.Sprintf("feature '%s' has UDF that requires manual migration", f.Name))
		}
		features = append(features, *spec)
	}

	// Mark as on_demand in group tags
	tags := make(map[string]string)
	tags["feast_type"] = "on_demand"
	if c.config.PreserveMetadata && len(odfv.Tags) > 0 {
		for k, v := range odfv.Tags {
			tags[k] = v
		}
	}

	group := &domain.FeatureGroup{
		Name:        c.mapName(odfv.Name),
		Description: odfv.Description,
		EntityType:  "derived",
		Features:    features,
		TTL:         c.config.DefaultTTL,
		Tags:        tags,
	}

	return group, &warnings, nil
}

func (c *SchemaConverter) convertFeature(f *FeastFeature) (*domain.FeatureSpec, error) {
	dataType, err := c.convertType(f.ValueType)
	if err != nil {
		// Default to float64 for unknown types
		dataType = domain.DataTypeFloat64
	}

	spec := &domain.FeatureSpec{
		Name:     c.mapName(f.Name),
		DataType: dataType,
	}

	return spec, nil
}

func (c *SchemaConverter) convertType(feastType FeastValueType) (domain.DataType, error) {
	switch feastType {
	case FeastTypeBool:
		return domain.DataTypeBool, nil
	case FeastTypeInt32, FeastTypeInt64:
		return domain.DataTypeInt64, nil
	case FeastTypeFloat, FeastTypeDouble:
		return domain.DataTypeFloat64, nil
	case FeastTypeString, FeastTypeBytes:
		return domain.DataTypeString, nil
	case FeastTypeBoolList, FeastTypeInt32List, FeastTypeInt64List, FeastTypeFloatList, FeastTypeStringList:
		return domain.DataTypeString, nil // Store lists as JSON strings
	case FeastTypeUnixTimestamp:
		return domain.DataTypeInt64, nil
	default:
		return domain.DataTypeFloat64, fmt.Errorf("%w: %s", ErrUnsupportedType, feastType)
	}
}

func (c *SchemaConverter) mapName(name string) string {
	if mapped, ok := c.config.NameMapping[name]; ok {
		return mapped
	}
	// Convert to snake_case if needed
	return strings.ToLower(strings.ReplaceAll(name, "-", "_"))
}

// ValidateFeastProject validates a Feast project definition.
func ValidateFeastProject(project *FeastProject) []string {
	errors := make([]string, 0)

	if project == nil {
		errors = append(errors, "project is nil")
		return errors
	}

	if project.Name == "" {
		errors = append(errors, "project name is required")
	}

	// Validate entities
	entityNames := make(map[string]bool)
	for i, entity := range project.Entities {
		if entity.Name == "" {
			errors = append(errors, fmt.Sprintf("entity[%d]: name is required", i))
		}
		if entityNames[entity.Name] {
			errors = append(errors, fmt.Sprintf("entity[%d]: duplicate name '%s'", i, entity.Name))
		}
		entityNames[entity.Name] = true
	}

	// Validate feature views
	viewNames := make(map[string]bool)
	for i, fv := range project.FeatureViews {
		if fv.Name == "" {
			errors = append(errors, fmt.Sprintf("feature_view[%d]: name is required", i))
		}
		if viewNames[fv.Name] {
			errors = append(errors, fmt.Sprintf("feature_view[%d]: duplicate name '%s'", i, fv.Name))
		}
		viewNames[fv.Name] = true

		// Validate entity references
		for _, entityRef := range fv.Entities {
			if !entityNames[entityRef] {
				errors = append(errors, fmt.Sprintf("feature_view[%d]: references unknown entity '%s'", i, entityRef))
			}
		}

		// Validate features
		featureNames := make(map[string]bool)
		for j, f := range fv.Features {
			if f.Name == "" {
				errors = append(errors, fmt.Sprintf("feature_view[%d].features[%d]: name is required", i, j))
			}
			if featureNames[f.Name] {
				errors = append(errors, fmt.Sprintf("feature_view[%d].features[%d]: duplicate name '%s'", i, j, f.Name))
			}
			featureNames[f.Name] = true
		}
	}

	return errors
}
