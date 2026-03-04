package cmd

import (
	"testing"
)

func TestParseValue(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"true", true},
		{"false", false},
		{"123", int64(123)},
		{"0", int64(0)},
		{"-42", int64(-42)},
		{"3.14", 3.14},
		{"0.0", 0.0},
		{"hello", "hello"},
		{"", ""},
		{"not-a-number", "not-a-number"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseValue(tt.input)
			switch expected := tt.expected.(type) {
			case bool:
				if v, ok := result.(bool); !ok || v != expected {
					t.Errorf("parseValue(%q) = %v (%T), want %v (%T)", tt.input, result, result, expected, expected)
				}
			case int64:
				if v, ok := result.(int64); !ok || v != expected {
					t.Errorf("parseValue(%q) = %v (%T), want %v (%T)", tt.input, result, result, expected, expected)
				}
			case float64:
				if v, ok := result.(float64); !ok || v != expected {
					t.Errorf("parseValue(%q) = %v (%T), want %v (%T)", tt.input, result, result, expected, expected)
				}
			case string:
				if v, ok := result.(string); !ok || v != expected {
					t.Errorf("parseValue(%q) = %v (%T), want %v (%T)", tt.input, result, result, expected, expected)
				}
			}
		})
	}
}

func TestParseVector(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []float64
		wantErr bool
	}{
		{"single", "1.0", []float64{1.0}, false},
		{"multiple", "0.1,0.2,0.3", []float64{0.1, 0.2, 0.3}, false},
		{"with spaces", "0.1, 0.2, 0.3", []float64{0.1, 0.2, 0.3}, false},
		{"integers", "1,2,3", []float64{1, 2, 3}, false},
		{"negative", "-0.5,0.5", []float64{-0.5, 0.5}, false},
		{"invalid", "a,b,c", nil, true},
		{"partial invalid", "0.1,abc,0.3", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVector(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseVector(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("parseVector(%q) len = %d, want %d", tt.input, len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("parseVector(%q)[%d] = %f, want %f", tt.input, i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestRootCmdHasSubcommands(t *testing.T) {
	subcommands := []string{"features", "health", "version", "ingest", "schema", "vectors"}
	for _, name := range subcommands {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected root command to have subcommand %q", name)
		}
	}
}

func TestFeaturesHasSubcommands(t *testing.T) {
	subcommands := []string{"get", "put", "batch", "history"}
	for _, name := range subcommands {
		found := false
		for _, cmd := range featuresCmd.Commands() {
			if cmd.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected features command to have subcommand %q", name)
		}
	}
}
