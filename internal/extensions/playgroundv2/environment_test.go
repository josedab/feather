package playgroundv2

import (
	"strings"
	"testing"
)

func TestNewEnvironment(t *testing.T) {
	env := NewEnvironment(DefaultConfig(), nil)
	if env == nil {
		t.Fatal("NewEnvironment returned nil")
	}
	stats := env.Stats()
	if stats.QueriesExecuted != 0 {
		t.Errorf("QueriesExecuted = %d, want 0", stats.QueriesExecuted)
	}
}

func TestExecuteQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   Query
		wantErr bool
	}{
		{
			name:  "simple query",
			query: Query{Text: "SELECT * FROM features"},
		},
		{
			name: "with entity filters",
			query: Query{
				Text:          "get features",
				EntityFilters: []string{"user:1", "user:2"},
			},
		},
		{
			name: "with feature filters",
			query: Query{
				Text:           "get specific",
				FeatureFilters: []string{"clicks", "views"},
			},
		},
		{
			name:    "empty query text",
			query:   Query{Text: ""},
			wantErr: true,
		},
		{
			name:    "whitespace-only query",
			query:   Query{Text: "   "},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewEnvironment(DefaultConfig(), nil)
			result, err := env.ExecuteQuery(tt.query)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Columns) == 0 {
				t.Error("expected columns in result")
			}
			if result.RowCount != len(result.Rows) {
				t.Errorf("RowCount = %d, but len(Rows) = %d", result.RowCount, len(result.Rows))
			}
			if len(result.Schema) == 0 {
				t.Error("expected schema in result")
			}
		})
	}
}

func TestExecuteQuery_MaxResultSize(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxResultSize = 2
	env := NewEnvironment(cfg, nil)

	result, err := env.ExecuteQuery(Query{
		Text:          "get all",
		EntityFilters: []string{"u1", "u2", "u3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount > 2 {
		t.Errorf("RowCount = %d, want <= 2", result.RowCount)
	}
}

func TestBrowseSchemas(t *testing.T) {
	env := NewEnvironment(DefaultConfig(), nil)
	schemas := env.BrowseSchemas()
	if len(schemas) == 0 {
		t.Error("expected demo schemas")
	}
	for _, s := range schemas {
		if s.GroupName == "" {
			t.Error("GroupName should not be empty")
		}
		if s.EntityType == "" {
			t.Error("EntityType should not be empty")
		}
	}
}

func TestBrowseSchemas_WithProvider(t *testing.T) {
	provider := &mockSchemaProvider{
		groups: []SchemaInfo{
			{GroupName: "custom", EntityType: "item", FeatureCount: 2},
		},
	}
	env := NewEnvironment(DefaultConfig(), provider)
	schemas := env.BrowseSchemas()
	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}
	if schemas[0].GroupName != "custom" {
		t.Errorf("GroupName = %q, want %q", schemas[0].GroupName, "custom")
	}
}

func TestGetSchemaDetails(t *testing.T) {
	env := NewEnvironment(DefaultConfig(), nil)

	details, err := env.GetSchemaDetails("user_features")
	if err != nil {
		t.Fatalf("GetSchemaDetails: %v", err)
	}
	if details.GroupName != "user_features" {
		t.Errorf("GroupName = %q, want %q", details.GroupName, "user_features")
	}
	if len(details.Features) == 0 {
		t.Error("expected features in details")
	}

	// Not found
	if _, err := env.GetSchemaDetails("nonexistent"); err == nil {
		t.Error("expected error for unknown group")
	}
}

func TestPreviewRegistration(t *testing.T) {
	tests := []struct {
		name      string
		spec      RegistrationSpec
		wantValid bool
	}{
		{
			name: "valid spec",
			spec: RegistrationSpec{
				GroupName:  "new_group",
				EntityType: "user",
				Features:   []FeatureSpec{{Name: "f1", DataType: "int64"}},
			},
			wantValid: true,
		},
		{
			name: "missing group name",
			spec: RegistrationSpec{
				EntityType: "user",
				Features:   []FeatureSpec{{Name: "f1"}},
			},
			wantValid: false,
		},
		{
			name: "no features",
			spec: RegistrationSpec{
				GroupName:  "g",
				EntityType: "user",
			},
			wantValid: false,
		},
		{
			name: "duplicate feature names",
			spec: RegistrationSpec{
				GroupName:  "g",
				EntityType: "user",
				Features: []FeatureSpec{
					{Name: "f1", DataType: "int64"},
					{Name: "f1", DataType: "float64"},
				},
			},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewEnvironment(DefaultConfig(), nil)
			preview, err := env.PreviewRegistration(tt.spec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if preview.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v (errors: %v)", preview.Valid, tt.wantValid, preview.Errors)
			}
			if preview.Impact == "" {
				t.Error("Impact should not be empty")
			}
		})
	}
}

func TestConfirmRegistration(t *testing.T) {
	env := NewEnvironment(DefaultConfig(), nil)
	spec := RegistrationSpec{
		GroupName:  "test_group",
		EntityType: "user",
		Features:   []FeatureSpec{{Name: "f1", DataType: "int64"}},
	}

	result, err := env.ConfirmRegistration(spec)
	if err != nil {
		t.Fatalf("ConfirmRegistration: %v", err)
	}
	if result.GroupName != "test_group" {
		t.Errorf("GroupName = %q, want %q", result.GroupName, "test_group")
	}
	if result.FeaturesCreated != 1 {
		t.Errorf("FeaturesCreated = %d, want 1", result.FeaturesCreated)
	}
	if result.Status != "created" {
		t.Errorf("Status = %q, want %q", result.Status, "created")
	}

	// Second registration should update
	result2, err := env.ConfirmRegistration(spec)
	if err != nil {
		t.Fatal(err)
	}
	if result2.Status != "updated" {
		t.Errorf("Status = %q, want %q", result2.Status, "updated")
	}
}

func TestConfirmRegistration_Invalid(t *testing.T) {
	env := NewEnvironment(DefaultConfig(), nil)
	_, err := env.ConfirmRegistration(RegistrationSpec{})
	if err == nil {
		t.Error("expected error for invalid spec")
	}
}

func TestFormatResponse(t *testing.T) {
	env := NewEnvironment(DefaultConfig(), nil)
	result := &QueryResult{
		Columns:  []string{"entity", "feature"},
		Rows:     [][]interface{}{{"u1", "clicks"}},
		RowCount: 1,
	}

	tests := []struct {
		name    string
		format  string
		wantErr bool
		check   func([]byte)
	}{
		{
			name:   "json",
			format: "json",
			check: func(b []byte) {
				if !strings.Contains(string(b), "entity") {
					t.Error("JSON should contain column names")
				}
			},
		},
		{
			name:   "csv",
			format: "csv",
			check: func(b []byte) {
				if !strings.Contains(string(b), "entity,feature") {
					t.Error("CSV should contain header")
				}
			},
		},
		{
			name:   "table",
			format: "table",
		},
		{
			name:   "chart",
			format: "chart",
		},
		{
			name:    "unsupported",
			format:  "xml",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := env.FormatResponse(result, tt.format)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out) == 0 {
				t.Error("expected non-empty output")
			}
			if tt.check != nil {
				tt.check(out)
			}
		})
	}
}

func TestStats(t *testing.T) {
	env := NewEnvironment(DefaultConfig(), nil)
	env.ExecuteQuery(Query{Text: "q1"})
	env.ExecuteQuery(Query{Text: "q2"})

	stats := env.Stats()
	if stats.QueriesExecuted != 2 {
		t.Errorf("QueriesExecuted = %d, want 2", stats.QueriesExecuted)
	}
}

// mockSchemaProvider implements SchemaProvider for testing.
type mockSchemaProvider struct {
	groups []SchemaInfo
}

func (m *mockSchemaProvider) ListGroups() []SchemaInfo {
	return m.groups
}

func (m *mockSchemaProvider) GetGroupDetails(groupName string) (*SchemaDetails, error) {
	for _, g := range m.groups {
		if g.GroupName == groupName {
			return &SchemaDetails{SchemaInfo: g}, nil
		}
	}
	return nil, nil
}
