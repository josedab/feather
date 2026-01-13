// Package gitops provides declarative feature definitions and policy-as-code enforcement.
package gitops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// FeatureDefinition represents a declarative feature definition in Git.
type FeatureDefinition struct {
	APIVersion string            `json:"apiVersion" yaml:"apiVersion"`
	Kind       string            `json:"kind" yaml:"kind"`
	Metadata   DefinitionMeta    `json:"metadata" yaml:"metadata"`
	Spec       FeatureSpec       `json:"spec" yaml:"spec"`
	Status     *DefinitionStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// DefinitionMeta contains metadata about a feature definition.
type DefinitionMeta struct {
	Name        string            `json:"name" yaml:"name"`
	Namespace   string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Owner       string            `json:"owner,omitempty" yaml:"owner,omitempty"`
	Team        string            `json:"team,omitempty" yaml:"team,omitempty"`
}

// FeatureSpec defines the specification for a feature or feature group.
type FeatureSpec struct {
	EntityType  string           `json:"entityType" yaml:"entityType"`
	Description string           `json:"description,omitempty" yaml:"description,omitempty"`
	Features    []FeatureField   `json:"features" yaml:"features"`
	TTL         *Duration        `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Tags        []string         `json:"tags,omitempty" yaml:"tags,omitempty"`
	Sources     []SourceConfig   `json:"sources,omitempty" yaml:"sources,omitempty"`
	Freshness   *FreshnessSpec   `json:"freshness,omitempty" yaml:"freshness,omitempty"`
	Validation  *ValidationSpec  `json:"validation,omitempty" yaml:"validation,omitempty"`
	Deprecation *DeprecationSpec `json:"deprecation,omitempty" yaml:"deprecation,omitempty"`
}

// FeatureField represents a single feature in a feature group.
type FeatureField struct {
	Name         string       `json:"name" yaml:"name"`
	DataType     string       `json:"dataType" yaml:"dataType"`
	Description  string       `json:"description,omitempty" yaml:"description,omitempty"`
	DefaultValue interface{}  `json:"defaultValue,omitempty" yaml:"defaultValue,omitempty"`
	Constraints  *Constraints `json:"constraints,omitempty" yaml:"constraints,omitempty"`
	Sensitivity  string       `json:"sensitivity,omitempty" yaml:"sensitivity,omitempty"` // public, internal, confidential, restricted
}

// Constraints defines validation constraints for a feature.
type Constraints struct {
	Required  bool          `json:"required,omitempty" yaml:"required,omitempty"`
	MinValue  *float64      `json:"minValue,omitempty" yaml:"minValue,omitempty"`
	MaxValue  *float64      `json:"maxValue,omitempty" yaml:"maxValue,omitempty"`
	MinLength *int          `json:"minLength,omitempty" yaml:"minLength,omitempty"`
	MaxLength *int          `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
	Pattern   string        `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	Enum      []interface{} `json:"enum,omitempty" yaml:"enum,omitempty"`
}

// Duration wraps time.Duration for YAML/JSON serialization.
type Duration struct {
	time.Duration
}

// MarshalJSON implements json.Marshaler.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case string:
		dur, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		d.Duration = dur
	case float64:
		d.Duration = time.Duration(value)
	default:
		return fmt.Errorf("invalid duration type: %T", v)
	}
	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (d Duration) MarshalYAML() (interface{}, error) {
	return d.String(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	return nil
}

// SourceConfig defines where feature data comes from.
type SourceConfig struct {
	Type       string            `json:"type" yaml:"type"` // kafka, http, batch, warehouse
	Connection string            `json:"connection,omitempty" yaml:"connection,omitempty"`
	Topic      string            `json:"topic,omitempty" yaml:"topic,omitempty"`
	Query      string            `json:"query,omitempty" yaml:"query,omitempty"`
	Schedule   string            `json:"schedule,omitempty" yaml:"schedule,omitempty"` // cron expression
	Config     map[string]string `json:"config,omitempty" yaml:"config,omitempty"`
}

// FreshnessSpec defines feature freshness requirements.
type FreshnessSpec struct {
	MaxAge  Duration    `json:"maxAge" yaml:"maxAge"`
	OnStale string      `json:"onStale,omitempty" yaml:"onStale,omitempty"` // error, warn, default
	Default interface{} `json:"default,omitempty" yaml:"default,omitempty"`
}

// ValidationSpec defines validation rules for features.
type ValidationSpec struct {
	OnFailure string           `json:"onFailure,omitempty" yaml:"onFailure,omitempty"` // reject, warn, allow
	Rules     []ValidationRule `json:"rules,omitempty" yaml:"rules,omitempty"`
}

// ValidationRule defines a single validation rule.
type ValidationRule struct {
	Name       string `json:"name" yaml:"name"`
	Expression string `json:"expression" yaml:"expression"` // CEL expression
	Message    string `json:"message,omitempty" yaml:"message,omitempty"`
}

// DeprecationSpec marks a feature as deprecated.
type DeprecationSpec struct {
	Deprecated  bool   `json:"deprecated" yaml:"deprecated"`
	Message     string `json:"message,omitempty" yaml:"message,omitempty"`
	Replacement string `json:"replacement,omitempty" yaml:"replacement,omitempty"`
	SunsetDate  string `json:"sunsetDate,omitempty" yaml:"sunsetDate,omitempty"` // RFC3339
}

// DefinitionStatus tracks the applied status of a definition.
type DefinitionStatus struct {
	Applied      bool      `json:"applied" yaml:"applied"`
	LastApplied  time.Time `json:"lastApplied,omitempty" yaml:"lastApplied,omitempty"`
	LastModified time.Time `json:"lastModified,omitempty" yaml:"lastModified,omitempty"`
	ResourceHash string    `json:"resourceHash,omitempty" yaml:"resourceHash,omitempty"`
	Errors       []string  `json:"errors,omitempty" yaml:"errors,omitempty"`
	Warnings     []string  `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

// SchemaLoader loads feature definitions from files.
type SchemaLoader struct {
	basePath string
}

// NewSchemaLoader creates a new schema loader.
func NewSchemaLoader(basePath string) *SchemaLoader {
	return &SchemaLoader{basePath: basePath}
}

// LoadDefinition loads a feature definition from a file.
func (l *SchemaLoader) LoadDefinition(filePath string) (*FeatureDefinition, error) {
	fullPath := filepath.Join(l.basePath, filePath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var def FeatureDefinition
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &def); err != nil {
			return nil, fmt.Errorf("parsing YAML: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &def); err != nil {
			return nil, fmt.Errorf("parsing JSON: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}

	if err := l.validateDefinition(&def); err != nil {
		return nil, fmt.Errorf("validating definition: %w", err)
	}

	return &def, nil
}

// LoadAllDefinitions loads all feature definitions from a directory.
func (l *SchemaLoader) LoadAllDefinitions(pattern string) ([]*FeatureDefinition, error) {
	fullPattern := filepath.Join(l.basePath, pattern)
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil, fmt.Errorf("glob pattern: %w", err)
	}

	definitions := make([]*FeatureDefinition, 0, len(matches))
	for _, match := range matches {
		relPath, err := filepath.Rel(l.basePath, match)
		if err != nil {
			relPath = match
		}
		def, err := l.LoadDefinition(relPath)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", relPath, err)
		}
		definitions = append(definitions, def)
	}

	return definitions, nil
}

// validateDefinition validates a feature definition.
func (l *SchemaLoader) validateDefinition(def *FeatureDefinition) error {
	if def.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}
	if def.Kind == "" {
		return fmt.Errorf("kind is required")
	}
	if def.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if def.Spec.EntityType == "" {
		return fmt.Errorf("spec.entityType is required")
	}
	if len(def.Spec.Features) == 0 {
		return fmt.Errorf("spec.features must not be empty")
	}

	for i, f := range def.Spec.Features {
		if f.Name == "" {
			return fmt.Errorf("feature[%d].name is required", i)
		}
		if f.DataType == "" {
			return fmt.Errorf("feature[%d].dataType is required", i)
		}
		if !isValidDataType(f.DataType) {
			return fmt.Errorf("feature[%d].dataType '%s' is invalid", i, f.DataType)
		}
	}

	return nil
}

// isValidDataType checks if a data type is valid.
func isValidDataType(dt string) bool {
	validTypes := map[string]bool{
		"string":       true,
		"int64":        true,
		"float64":      true,
		"bool":         true,
		"bytes":        true,
		"timestamp":    true,
		"string_list":  true,
		"int64_list":   true,
		"float64_list": true,
		"map":          true,
	}
	return validTypes[dt]
}

// SaveDefinition saves a feature definition to a file.
func (l *SchemaLoader) SaveDefinition(def *FeatureDefinition, filePath string) error {
	fullPath := filepath.Join(l.basePath, filePath)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	var data []byte
	var err error

	switch ext {
	case ".yaml", ".yml":
		data, err = yaml.Marshal(def)
	case ".json":
		data, err = json.MarshalIndent(def, "", "  ")
	default:
		return fmt.Errorf("unsupported file format: %s", ext)
	}

	if err != nil {
		return fmt.Errorf("marshaling: %w", err)
	}

	if err := os.WriteFile(fullPath, data, 0600); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}
