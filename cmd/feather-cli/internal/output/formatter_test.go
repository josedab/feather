package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		format   string
		expected Format
	}{
		{"json", FormatJSON},
		{"yaml", FormatYAML},
		{"table", FormatTable},
		{"", FormatTable},
		{"unknown", FormatTable},
		{"JSON", FormatJSON},
		{"YAML", FormatYAML},
	}

	for _, tt := range tests {
		var buf bytes.Buffer
		f := NewFormatter(tt.format, &buf)
		if f.format != tt.expected {
			t.Errorf("NewFormatter(%q) format = %v, want %v", tt.format, f.format, tt.expected)
		}
	}
}

func TestPrintJSON(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter("json", &buf)

	data := map[string]string{"key": "value"}
	if err := f.Print(data); err != nil {
		t.Fatal(err)
	}

	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got key=%s", result["key"])
	}
}

func TestPrintYAML(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter("yaml", &buf)

	data := map[string]string{"key": "value"}
	if err := f.Print(data); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "key: value") {
		t.Errorf("expected YAML output to contain 'key: value', got %q", buf.String())
	}
}

func TestPrintTableData(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter("table", &buf)

	data := TableData{
		Headers: []string{"NAME", "VALUE"},
		Rows: []TableRow{
			{"foo", "bar"},
			{"baz", "qux"},
		},
	}

	if err := f.Print(data); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	if !strings.Contains(output, "NAME") {
		t.Error("expected table output to contain header NAME")
	}
	if !strings.Contains(output, "foo") {
		t.Error("expected table output to contain row value foo")
	}
	if !strings.Contains(output, "baz") {
		t.Error("expected table output to contain row value baz")
	}
}

func TestPrintMessage(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter("table", &buf)

	f.PrintMessage("hello %s", "world")
	if !strings.Contains(buf.String(), "hello world") {
		t.Errorf("expected 'hello world', got %q", buf.String())
	}
}

func TestPrintError(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter("table", &buf)

	f.PrintError(json.Unmarshal([]byte("invalid"), nil))
	if !strings.Contains(buf.String(), "Error:") {
		t.Errorf("expected error prefix, got %q", buf.String())
	}
}

func TestPrintSuccess(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter("table", &buf)

	f.PrintSuccess("it worked")
	if !strings.Contains(buf.String(), "Success: it worked") {
		t.Errorf("expected success message, got %q", buf.String())
	}
}

func TestPrintFeatureResult(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"json", "json"},
		{"yaml", "yaml"},
		{"table", "table"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			f := NewFormatter(tt.format, &buf)

			result := &FeatureResult{
				EntityID: "user:123",
				Features: map[string]FeatureDisplay{
					"score": {Value: 0.95, Timestamp: "2024-01-01T00:00:00Z"},
				},
			}

			if err := f.PrintFeatureResult(result); err != nil {
				t.Fatal(err)
			}

			if buf.Len() == 0 {
				t.Error("expected non-empty output")
			}
		})
	}
}

func TestPrintSchemaList(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"json", "json"},
		{"table", "table"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			f := NewFormatter(tt.format, &buf)

			schemas := []*SchemaResult{
				{
					Name:       "user_features",
					EntityType: "user",
					Features: []SchemaFeature{
						{Name: "score", DataType: "float"},
					},
				},
			}

			if err := f.PrintSchemaList(schemas); err != nil {
				t.Fatal(err)
			}

			if buf.Len() == 0 {
				t.Error("expected non-empty output")
			}
		})
	}
}

func TestPrintVectorSearchResults(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"json", "json"},
		{"table", "table"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			f := NewFormatter(tt.format, &buf)

			results := []*VectorSearchResult{
				{ID: "doc-1", Score: 0.95},
				{ID: "doc-2", Score: 0.87, Metadata: map[string]interface{}{"key": "val"}},
			}

			if err := f.PrintVectorSearchResults(results); err != nil {
				t.Fatal(err)
			}

			if buf.Len() == 0 {
				t.Error("expected non-empty output")
			}
		})
	}
}

func TestPrintHealthResult(t *testing.T) {
	tests := []struct {
		name       string
		format     string
		components bool
	}{
		{"json", "json", false},
		{"table-simple", "table", false},
		{"table-components", "table", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			f := NewFormatter(tt.format, &buf)

			result := &HealthResult{Status: "healthy"}
			if tt.components {
				result.Components = map[string]HealthCheck{
					"storage": {Status: "healthy", Message: "ok"},
				}
			}

			if err := f.PrintHealthResult(result); err != nil {
				t.Fatal(err)
			}

			if buf.Len() == 0 {
				t.Error("expected non-empty output")
			}
		})
	}
}
