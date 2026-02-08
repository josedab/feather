package transform

import (
	"testing"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

func TestCatalog_BuiltInTemplates(t *testing.T) {
	c := NewCatalog()

	if got := c.Count(); got != 15 {
		t.Fatalf("expected 15 built-in templates, got %d", got)
	}

	// Verify a few key templates exist.
	for _, id := range []string{"one_hot", "bucket", "min_max_scale", "z_score", "log_transform", "abs", "power", "lower", "upper", "trim", "day_of_week", "hour_of_day", "is_weekend", "count", "mean"} {
		tmpl, err := c.Get(id)
		if err != nil {
			t.Errorf("expected built-in template %q, got error: %v", id, err)
			continue
		}
		if !tmpl.BuiltIn {
			t.Errorf("template %q should be built-in", id)
		}
		if tmpl.Version != 1 {
			t.Errorf("template %q version = %d, want 1", id, tmpl.Version)
		}
	}
}

func TestCatalog_Register(t *testing.T) {
	c := NewCatalog()

	custom := &Template{
		ID:          "custom_transform",
		Name:        "Custom Transform",
		Description: "A user-defined transform",
		Category:    CategoryMath,
		Type:        TypeArithmetic,
		Expression:  "input * 2",
		InputTypes:  []domain.DataType{domain.DataTypeFloat64},
		OutputType:  domain.DataTypeFloat64,
		Version:     1,
		CreatedAt:   time.Now(),
		BuiltIn:     false,
	}

	if err := c.Register(custom); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if got := c.Count(); got != 16 {
		t.Fatalf("expected 16 templates after register, got %d", got)
	}

	tmpl, err := c.Get("custom_transform")
	if err != nil {
		t.Fatalf("Get custom template failed: %v", err)
	}
	if tmpl.Name != "Custom Transform" {
		t.Errorf("name = %q, want %q", tmpl.Name, "Custom Transform")
	}
}

func TestCatalog_ListByCategory(t *testing.T) {
	c := NewCatalog()

	mathTemplates := c.ListByCategory(CategoryMath)
	if len(mathTemplates) != 3 {
		t.Fatalf("expected 3 math templates, got %d", len(mathTemplates))
	}

	stringTemplates := c.ListByCategory(CategoryString)
	if len(stringTemplates) != 3 {
		t.Fatalf("expected 3 string templates, got %d", len(stringTemplates))
	}

	temporalTemplates := c.ListByCategory(CategoryTemporal)
	if len(temporalTemplates) != 3 {
		t.Fatalf("expected 3 temporal templates, got %d", len(temporalTemplates))
	}

	aggTemplates := c.ListByCategory(CategoryAggregation)
	if len(aggTemplates) != 2 {
		t.Fatalf("expected 2 aggregation templates, got %d", len(aggTemplates))
	}
}

func TestCatalog_Search(t *testing.T) {
	c := NewCatalog()

	results := c.Search("normalize")
	// "Min-Max Normalization" and "Z-Score Normalization" descriptions contain "normalization"
	// but search is on name/description - "normalize" appears in descriptions
	if len(results) < 1 {
		t.Fatalf("expected at least 1 search result for 'normalize', got %d", len(results))
	}

	results = c.Search("string")
	if len(results) < 1 {
		t.Fatalf("expected at least 1 search result for 'string', got %d", len(results))
	}

	results = c.Search("UPPERCASE")
	if len(results) != 1 {
		t.Fatalf("expected 1 search result for 'UPPERCASE', got %d", len(results))
	}

	results = c.Search("nonexistent_xyz_query")
	if len(results) != 0 {
		t.Fatalf("expected 0 search results for nonexistent query, got %d", len(results))
	}
}

func TestCatalog_Remove(t *testing.T) {
	c := NewCatalog()

	if err := c.Remove("abs"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if got := c.Count(); got != 14 {
		t.Fatalf("expected 14 templates after removal, got %d", got)
	}

	_, err := c.Get("abs")
	if err != ErrTemplateNotFound {
		t.Fatalf("expected ErrTemplateNotFound, got %v", err)
	}

	err = c.Remove("nonexistent")
	if err != ErrTemplateNotFound {
		t.Fatalf("expected ErrTemplateNotFound for nonexistent, got %v", err)
	}
}
