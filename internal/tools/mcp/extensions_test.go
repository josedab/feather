package mcp

import (
	"testing"
)

func TestAdditionalTools(t *testing.T) {
	tools := AdditionalTools()
	if len(tools) < 5 {
		t.Errorf("expected at least 5 additional tools, got %d", len(tools))
	}
	for _, tool := range tools {
		if tool.Name == "" {
			t.Error("tool name should not be empty")
		}
		if tool.Description == "" {
			t.Errorf("tool %q should have description", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %q should have input schema", tool.Name)
		}
	}
}

func TestBuiltinResources(t *testing.T) {
	resources := BuiltinResources()
	if len(resources) < 3 {
		t.Errorf("expected at least 3 resources, got %d", len(resources))
	}
	for _, r := range resources {
		if r.URI == "" {
			t.Error("resource URI should not be empty")
		}
		if r.Name == "" {
			t.Error("resource name should not be empty")
		}
		if r.MimeType == "" {
			t.Errorf("resource %q should have MIME type", r.Name)
		}
	}
}

func TestBuiltinPrompts(t *testing.T) {
	prompts := BuiltinPrompts()
	if len(prompts) < 3 {
		t.Errorf("expected at least 3 prompts, got %d", len(prompts))
	}
	for _, p := range prompts {
		if p.Name == "" {
			t.Error("prompt name should not be empty")
		}
		if p.Template == "" {
			t.Errorf("prompt %q should have template", p.Name)
		}
	}
}

func TestGetServerInfo(t *testing.T) {
	info := GetServerInfo()
	if info.Name != "feather-mcp" {
		t.Errorf("expected name 'feather-mcp', got %q", info.Name)
	}
	if info.ToolCount < 10 {
		t.Errorf("expected at least 10 tools, got %d", info.ToolCount)
	}
	if info.ResourceCount < 3 {
		t.Errorf("expected at least 3 resources, got %d", info.ResourceCount)
	}
	if len(info.Capabilities) < 3 {
		t.Errorf("expected at least 3 capabilities, got %d", len(info.Capabilities))
	}
}

func TestFormatFeatureTable(t *testing.T) {
	features := map[string]interface{}{
		"click_count":    15,
		"purchase_total": 245.50,
	}
	table := FormatFeatureTable(features)
	if table == "" {
		t.Error("expected non-empty table")
	}

	empty := FormatFeatureTable(nil)
	if empty != "No features found." {
		t.Errorf("expected 'No features found.', got %q", empty)
	}
}
