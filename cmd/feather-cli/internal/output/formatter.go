// Package output provides output formatting for feather-cli.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Format represents output format type.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

// Formatter handles output formatting.
type Formatter struct {
	format Format
	writer io.Writer
}

// NewFormatter creates a new Formatter.
func NewFormatter(format string, writer io.Writer) *Formatter {
	f := FormatTable
	switch strings.ToLower(format) {
	case "json":
		f = FormatJSON
	case "yaml":
		f = FormatYAML
	case "table", "":
		f = FormatTable
	}
	return &Formatter{
		format: f,
		writer: writer,
	}
}

// Print prints data in the configured format.
func (f *Formatter) Print(data interface{}) error {
	switch f.format {
	case FormatJSON:
		return f.printJSON(data)
	case FormatYAML:
		return f.printYAML(data)
	default:
		return f.printTable(data)
	}
}

// PrintJSON forces JSON output regardless of configured format.
func (f *Formatter) PrintJSON(data interface{}) error {
	return f.printJSON(data)
}

// PrintMessage prints a simple message.
func (f *Formatter) PrintMessage(format string, args ...interface{}) {
	fmt.Fprintf(f.writer, format+"\n", args...)
}

// PrintError prints an error message.
func (f *Formatter) PrintError(err error) {
	fmt.Fprintf(f.writer, "Error: %v\n", err)
}

// PrintSuccess prints a success message.
func (f *Formatter) PrintSuccess(msg string) {
	fmt.Fprintf(f.writer, "Success: %s\n", msg)
}

func (f *Formatter) printJSON(data interface{}) error {
	encoder := json.NewEncoder(f.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func (f *Formatter) printYAML(data interface{}) error {
	encoder := yaml.NewEncoder(f.writer)
	defer encoder.Close()
	return encoder.Encode(data)
}

func (f *Formatter) printTable(data interface{}) error {
	switch v := data.(type) {
	case TableData:
		return f.printTableData(v)
	case []TableRow:
		return f.printTableRows(nil, v)
	default:
		// Fall back to JSON for non-table data
		return f.printJSON(data)
	}
}

// TableData represents tabular data for display.
type TableData struct {
	Headers []string
	Rows    []TableRow
}

// TableRow represents a single row of table data.
type TableRow []string

func (f *Formatter) printTableData(data TableData) error {
	return f.printTableRows(data.Headers, data.Rows)
}

func (f *Formatter) printTableRows(headers []string, rows []TableRow) error {
	w := tabwriter.NewWriter(f.writer, 0, 0, 2, ' ', 0)

	// Print headers
	if len(headers) > 0 {
		fmt.Fprintln(w, strings.Join(headers, "\t"))
		// Print separator
		sep := make([]string, len(headers))
		for i := range sep {
			sep[i] = strings.Repeat("-", len(headers[i]))
		}
		fmt.Fprintln(w, strings.Join(sep, "\t"))
	}

	// Print rows
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}

	return w.Flush()
}

// FeatureResult represents a feature get result for display.
type FeatureResult struct {
	EntityID string                    `json:"entity_id" yaml:"entity_id"`
	Features map[string]FeatureDisplay `json:"features" yaml:"features"`
}

// FeatureDisplay represents a feature value for display.
type FeatureDisplay struct {
	Value     interface{} `json:"value" yaml:"value"`
	Timestamp string      `json:"timestamp" yaml:"timestamp"`
	Version   int64       `json:"version,omitempty" yaml:"version,omitempty"`
}

// PrintFeatureResult prints feature results in the appropriate format.
func (f *Formatter) PrintFeatureResult(result *FeatureResult) error {
	switch f.format {
	case FormatJSON, FormatYAML:
		return f.Print(result)
	default:
		// Table format
		headers := []string{"FEATURE", "VALUE", "TIMESTAMP"}
		var rows []TableRow
		for name, feat := range result.Features {
			rows = append(rows, TableRow{
				name,
				fmt.Sprintf("%v", feat.Value),
				feat.Timestamp,
			})
		}
		return f.printTableRows(headers, rows)
	}
}

// SchemaResult represents a schema for display.
type SchemaResult struct {
	Name       string             `json:"name" yaml:"name"`
	EntityType string             `json:"entity_type" yaml:"entity_type"`
	Features   []SchemaFeature    `json:"features" yaml:"features"`
	Metadata   map[string]string  `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// SchemaFeature represents a feature in a schema.
type SchemaFeature struct {
	Name        string `json:"name" yaml:"name"`
	DataType    string `json:"data_type" yaml:"data_type"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// PrintSchemaList prints a list of schemas in the appropriate format.
func (f *Formatter) PrintSchemaList(schemas []*SchemaResult) error {
	switch f.format {
	case FormatJSON, FormatYAML:
		return f.Print(schemas)
	default:
		headers := []string{"NAME", "ENTITY TYPE", "FEATURES"}
		var rows []TableRow
		for _, s := range schemas {
			rows = append(rows, TableRow{
				s.Name,
				s.EntityType,
				fmt.Sprintf("%d", len(s.Features)),
			})
		}
		return f.printTableRows(headers, rows)
	}
}

// VectorSearchResult represents a vector search result for display.
type VectorSearchResult struct {
	ID       string                 `json:"id" yaml:"id"`
	Score    float64                `json:"score" yaml:"score"`
	Metadata map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// PrintVectorSearchResults prints vector search results.
func (f *Formatter) PrintVectorSearchResults(results []*VectorSearchResult) error {
	switch f.format {
	case FormatJSON, FormatYAML:
		return f.Print(results)
	default:
		headers := []string{"ID", "SCORE", "METADATA"}
		var rows []TableRow
		for _, r := range results {
			metadata := ""
			if len(r.Metadata) > 0 {
				b, _ := json.Marshal(r.Metadata)
				metadata = string(b)
			}
			rows = append(rows, TableRow{
				r.ID,
				fmt.Sprintf("%.4f", r.Score),
				metadata,
			})
		}
		return f.printTableRows(headers, rows)
	}
}

// HealthResult represents health check result for display.
type HealthResult struct {
	Status     string                 `json:"status" yaml:"status"`
	Components map[string]HealthCheck `json:"components,omitempty" yaml:"components,omitempty"`
}

// HealthCheck represents a component health check.
type HealthCheck struct {
	Status  string `json:"status" yaml:"status"`
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

// PrintHealthResult prints health check results.
func (f *Formatter) PrintHealthResult(result *HealthResult) error {
	switch f.format {
	case FormatJSON, FormatYAML:
		return f.Print(result)
	default:
		f.PrintMessage("Status: %s", result.Status)
		if len(result.Components) > 0 {
			f.PrintMessage("\nComponents:")
			headers := []string{"COMPONENT", "STATUS", "MESSAGE"}
			var rows []TableRow
			for name, check := range result.Components {
				rows = append(rows, TableRow{
					name,
					check.Status,
					check.Message,
				})
			}
			return f.printTableRows(headers, rows)
		}
		return nil
	}
}
