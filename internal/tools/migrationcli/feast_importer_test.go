package migrationcli

import (
	"strings"
	"testing"
)

func TestMapValueType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"INT64", "INT64"},
		{"INT32", "INT64"},
		{"FLOAT", "FLOAT64"},
		{"DOUBLE", "FLOAT64"},
		{"STRING", "STRING"},
		{"BYTES", "BYTES"},
		{"BOOL", "BOOL"},
		{"UNKNOWN", "STRING"},
		{"float", "FLOAT64"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := MapValueType(tt.input)
			if got != tt.want {
				t.Errorf("MapValueType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseFeatureStoreYAML(t *testing.T) {
	yamlContent := `
project: my_project
registry: /path/to/registry.db
provider: local
online_store:
  type: sqlite
  path: /tmp/online_store.db
offline_store:
  type: file
`
	imp := NewImporter()
	cfg, err := imp.ParseFeatureStoreYAML(yamlContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "my_project" {
		t.Errorf("got project %q, want %q", cfg.Project, "my_project")
	}
	if cfg.Provider != "local" {
		t.Errorf("got provider %q, want %q", cfg.Provider, "local")
	}

	// missing project should error
	_, err = imp.ParseFeatureStoreYAML("registry: foo")
	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestImportFeatureView(t *testing.T) {
	imp := NewImporter()
	view := FeastFeatureView{
		Name:     "user_features",
		Entities: []string{"user_id"},
		Features: []FeastFeature{
			{Name: "login_count", ValueType: "INT64"},
			{Name: "avg_session", ValueType: "FLOAT"},
		},
		TTL: "3600s",
	}

	result, err := imp.ImportFeatureView(view)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ViewsImported != 1 {
		t.Errorf("got %d views, want 1", result.ViewsImported)
	}
	if result.FeaturesImported != 2 {
		t.Errorf("got %d features, want 2", result.FeaturesImported)
	}

	// empty name should error
	_, err = imp.ImportFeatureView(FeastFeatureView{})
	if err == nil {
		t.Error("expected error for empty view name")
	}
}

func TestValidateImport(t *testing.T) {
	warnings := ValidateImport(FeastFeatureView{})
	if len(warnings) == 0 {
		t.Error("expected warnings for empty view")
	}

	warnings = ValidateImport(FeastFeatureView{
		Name:     "ok",
		Entities: []string{"e"},
		Features: []FeastFeature{{Name: "f", ValueType: "INT64"}},
		TTL:      "60s",
	})
	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0", len(warnings))
	}
}

func TestGenerateReport(t *testing.T) {
	results := []ImportResult{
		{ViewsImported: 1, FeaturesImported: 3, Warnings: []string{"w1"}},
		{ViewsImported: 1, FeaturesImported: 2},
	}
	report := GenerateMigrationReport(results)
	if !strings.Contains(report, "Views imported:    2") {
		t.Errorf("report missing view count:\n%s", report)
	}
	if !strings.Contains(report, "Features imported: 5") {
		t.Errorf("report missing feature count:\n%s", report)
	}
}
