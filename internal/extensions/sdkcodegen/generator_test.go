package sdkcodegen

import (
	"strings"
	"testing"
)

func TestNewGenerator(t *testing.T) {
	g := NewGenerator(DefaultGeneratorConfig())
	if g == nil {
		t.Fatal("expected non-nil generator")
	}
}

func TestRegisterSchema(t *testing.T) {
	g := NewGenerator(DefaultGeneratorConfig())

	schema := SchemaDefinition{
		Name:       "user_features",
		EntityType: "user",
		Version:    "1.0.0",
		Fields: []SchemaField{
			{Name: "age", Type: FieldInt64, Required: true},
			{Name: "score", Type: FieldFloat64},
			{Name: "name", Type: FieldString, Required: true},
		},
	}

	if err := g.RegisterSchema(schema); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	schemas := g.ListSchemas()
	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}
}

func TestRegisterSchemaValidation(t *testing.T) {
	g := NewGenerator(DefaultGeneratorConfig())

	if err := g.RegisterSchema(SchemaDefinition{}); err == nil {
		t.Fatal("expected error for empty schema name")
	}

	if err := g.RegisterSchema(SchemaDefinition{Name: "test"}); err == nil {
		t.Fatal("expected error for schema with no fields")
	}
}

func TestGenerateGo(t *testing.T) {
	g := NewGenerator(DefaultGeneratorConfig())
	_ = g.RegisterSchema(SchemaDefinition{
		Name:       "user_features",
		EntityType: "user",
		Version:    "1.0.0",
		Fields: []SchemaField{
			{Name: "age", Type: FieldInt64, Required: true},
			{Name: "score", Type: FieldFloat64},
		},
	})

	code, err := g.Generate("user_features", LangGo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(code.Code, "type UserFeatures struct") {
		t.Error("expected Go struct definition")
	}
	if !strings.Contains(code.Code, "DO NOT EDIT") {
		t.Error("expected generation header")
	}
	if code.FileName != "user_features.go" {
		t.Errorf("expected user_features.go, got %s", code.FileName)
	}
}

func TestGeneratePython(t *testing.T) {
	g := NewGenerator(DefaultGeneratorConfig())
	_ = g.RegisterSchema(SchemaDefinition{
		Name:    "user_features",
		Version: "1.0.0",
		Fields: []SchemaField{
			{Name: "age", Type: FieldInt64},
		},
	})

	code, err := g.Generate("user_features", LangPython)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(code.Code, "@dataclass") {
		t.Error("expected Python dataclass")
	}
	if !strings.Contains(code.Code, "class UserFeatures:") {
		t.Error("expected Python class definition")
	}
}

func TestGenerateTypeScript(t *testing.T) {
	g := NewGenerator(DefaultGeneratorConfig())
	_ = g.RegisterSchema(SchemaDefinition{
		Name:    "user_features",
		Version: "1.0.0",
		Fields: []SchemaField{
			{Name: "score", Type: FieldFloat64},
		},
	})

	code, err := g.Generate("user_features", LangTypeScript)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(code.Code, "export interface UserFeatures") {
		t.Error("expected TypeScript interface")
	}
}

func TestGenerateUnsupportedLanguage(t *testing.T) {
	g := NewGenerator(DefaultGeneratorConfig())
	_ = g.RegisterSchema(SchemaDefinition{
		Name:    "test",
		Version: "1.0.0",
		Fields:  []SchemaField{{Name: "x", Type: FieldInt64}},
	})

	_, err := g.Generate("test", Language("ruby"))
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
}

func TestGenerateSchemaNotFound(t *testing.T) {
	g := NewGenerator(DefaultGeneratorConfig())
	_, err := g.Generate("nonexistent", LangGo)
	if err == nil {
		t.Fatal("expected error for missing schema")
	}
}
