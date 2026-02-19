package migrationcli

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// FeastConfig represents a Feast feature_store.yaml configuration.
type FeastConfig struct {
	Project      string                 `yaml:"project"`
	Registry     string                 `yaml:"registry"`
	Provider     string                 `yaml:"provider"`
	OnlineStore  map[string]interface{} `yaml:"online_store"`
	OfflineStore map[string]interface{} `yaml:"offline_store"`
}

// FeastFeatureView represents a Feast feature view definition.
type FeastFeatureView struct {
	Name        string
	Description string
	Entities    []string
	Features    []FeastFeature
	TTL         string
}

// FeastFeature represents a single feature in a Feast feature view.
type FeastFeature struct {
	Name      string
	ValueType string
}

// ImportResult contains the outcome of importing a single feature view.
type ImportResult struct {
	ViewsImported    int
	FeaturesImported int
	Warnings         []string
	Errors           []string
	Duration         time.Duration
}

// Importer handles migration from Feast to Feather.
type Importer struct {
	config  FeastConfig
	results []ImportResult
}

// NewImporter creates a new Feast-to-Feather importer.
func NewImporter() *Importer {
	return &Importer{}
}

// ParseFeatureStoreYAML parses a Feast feature_store.yaml configuration.
func (imp *Importer) ParseFeatureStoreYAML(content string) (*FeastConfig, error) {
	var cfg FeastConfig
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("parsing feature_store.yaml: %w", err)
	}
	if cfg.Project == "" {
		return nil, fmt.Errorf("parsing feature_store.yaml: project name is required")
	}
	imp.config = cfg
	return &cfg, nil
}

// MapValueType maps a Feast value type to the corresponding Feather type.
func MapValueType(feastType string) string {
	switch strings.ToUpper(feastType) {
	case "INT64", "INT32":
		return "INT64"
	case "FLOAT", "DOUBLE", "FLOAT64":
		return "FLOAT64"
	case "STRING":
		return "STRING"
	case "BYTES":
		return "BYTES"
	case "BOOL":
		return "BOOL"
	default:
		return "STRING"
	}
}

// ValidateImport checks a feature view for potential migration issues.
func ValidateImport(view FeastFeatureView) []string {
	var warnings []string
	if view.Name == "" {
		warnings = append(warnings, "feature view name is empty")
	}
	if len(view.Entities) == 0 {
		warnings = append(warnings, "no entities defined")
	}
	if len(view.Features) == 0 {
		warnings = append(warnings, "no features defined")
	}
	for _, f := range view.Features {
		if f.Name == "" {
			warnings = append(warnings, "feature with empty name found")
		}
		if f.ValueType == "" {
			warnings = append(warnings, fmt.Sprintf("feature %q has no value type", f.Name))
		}
	}
	if view.TTL == "" {
		warnings = append(warnings, "no TTL specified, using default")
	}
	return warnings
}

// ImportFeatureView converts a Feast feature view into a Feather-compatible schema.
func (imp *Importer) ImportFeatureView(view FeastFeatureView) (*ImportResult, error) {
	start := time.Now()
	result := &ImportResult{}

	warnings := ValidateImport(view)
	result.Warnings = warnings

	if view.Name == "" {
		result.Errors = append(result.Errors, "feature view name is required")
		result.Duration = time.Since(start)
		return result, fmt.Errorf("feature view name is required")
	}

	for _, f := range view.Features {
		mappedType := MapValueType(f.ValueType)
		if mappedType != strings.ToUpper(f.ValueType) {
			slog.Warn("feast value type mapped to default",
				"feature", f.Name,
				"feast_type", f.ValueType,
				"mapped_type", mappedType,
			)
		}
		result.FeaturesImported++
	}

	result.ViewsImported = 1
	result.Duration = time.Since(start)
	imp.results = append(imp.results, *result)
	return result, nil
}

// GenerateMigrationReport produces a human-readable migration summary.
func GenerateMigrationReport(results []ImportResult) string {
	var b strings.Builder
	b.WriteString("=== Feather Migration Report ===\n\n")

	totalViews := 0
	totalFeatures := 0
	totalWarnings := 0
	totalErrors := 0
	var totalDuration time.Duration

	for i, r := range results {
		fmt.Fprintf(&b, "View %d: %d features imported", i+1, r.FeaturesImported)
		if len(r.Warnings) > 0 {
			fmt.Fprintf(&b, " (%d warnings)", len(r.Warnings))
		}
		if len(r.Errors) > 0 {
			fmt.Fprintf(&b, " (%d errors)", len(r.Errors))
		}
		b.WriteString("\n")

		totalViews += r.ViewsImported
		totalFeatures += r.FeaturesImported
		totalWarnings += len(r.Warnings)
		totalErrors += len(r.Errors)
		totalDuration += r.Duration
	}

	b.WriteString("\n--- Summary ---\n")
	fmt.Fprintf(&b, "Views imported:    %d\n", totalViews)
	fmt.Fprintf(&b, "Features imported: %d\n", totalFeatures)
	fmt.Fprintf(&b, "Warnings:          %d\n", totalWarnings)
	fmt.Fprintf(&b, "Errors:            %d\n", totalErrors)
	fmt.Fprintf(&b, "Total duration:    %s\n", totalDuration.Round(time.Millisecond))

	return b.String()
}
