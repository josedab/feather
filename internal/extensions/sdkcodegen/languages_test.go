package sdkcodegen

import (
	"strings"
	"testing"
)

func TestNewLanguageRegistry(t *testing.T) {
	r := NewLanguageRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}

	langs := r.SupportedLanguages()
	if len(langs) != 7 {
		t.Fatalf("expected 7 languages, got %d: %v", len(langs), langs)
	}
}

func TestLanguageRegistrySupportedLanguages(t *testing.T) {
	r := NewLanguageRegistry()
	langs := r.SupportedLanguages()

	expected := []Language{LangGo, LangJava, LangKotlin, LangPython, LangRust, LangSwift, LangTypeScript}
	if len(langs) != len(expected) {
		t.Fatalf("expected %d languages, got %d", len(expected), len(langs))
	}
	for i, lang := range expected {
		if langs[i] != lang {
			t.Errorf("expected langs[%d] = %s, got %s", i, lang, langs[i])
		}
	}
}

func TestLanguageRegistryGenerate(t *testing.T) {
	r := NewLanguageRegistry()
	schema := &SchemaDefinition{
		Name:       "test_features",
		EntityType: "user",
		Version:    "1.0.0",
		Fields: []SchemaField{
			{Name: "score", Type: FieldFloat64, Required: true},
		},
	}

	tests := []struct {
		lang Language
		want string
	}{
		{LangGo, "type TestFeatures struct"},
		{LangPython, "class TestFeatures:"},
		{LangTypeScript, "export interface TestFeatures"},
		{LangJava, "public class TestFeatures"},
		{LangRust, "pub struct TestFeatures"},
		{LangSwift, "public struct TestFeatures: Codable"},
		{LangKotlin, "data class TestFeatures("},
	}

	for _, tt := range tests {
		code, err := r.Generate(tt.lang, schema)
		if err != nil {
			t.Fatalf("Generate(%s) error: %v", tt.lang, err)
		}
		if !strings.Contains(code, tt.want) {
			t.Errorf("Generate(%s): expected %q in output", tt.lang, tt.want)
		}
	}
}

func TestLanguageRegistryUnsupportedLanguage(t *testing.T) {
	r := NewLanguageRegistry()
	schema := &SchemaDefinition{
		Name:    "test",
		Version: "1.0.0",
		Fields:  []SchemaField{{Name: "x", Type: FieldInt64}},
	}

	_, err := r.Generate(Language("ruby"), schema)
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
	if !strings.Contains(err.Error(), "unsupported language") {
		t.Errorf("expected unsupported language error, got: %v", err)
	}
}

func TestLanguageRegistryCustomLanguage(t *testing.T) {
	r := NewLanguageRegistry()
	r.Register(Language("custom"), func(s *SchemaDefinition) string {
		return "custom: " + s.Name
	})

	code, err := r.Generate(Language("custom"), &SchemaDefinition{
		Name:    "test",
		Version: "1.0.0",
		Fields:  []SchemaField{{Name: "x", Type: FieldInt64}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "custom: test" {
		t.Errorf("expected 'custom: test', got %q", code)
	}

	langs := r.SupportedLanguages()
	if len(langs) != 8 {
		t.Fatalf("expected 8 languages after adding custom, got %d", len(langs))
	}
}
