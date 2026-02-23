package sdkcodegen

import (
	"strings"
	"testing"
)

func TestGenerateKotlin(t *testing.T) {
	g := NewGenerator(DefaultGeneratorConfig())
	schema := &SchemaDefinition{
		Name:        "user_features",
		EntityType:  "user",
		Description: "User feature group",
		Version:     "1.0.0",
		Fields: []SchemaField{
			{Name: "age", Type: FieldInt64, Required: true},
			{Name: "score", Type: FieldFloat64},
			{Name: "name", Type: FieldString, Required: true},
			{Name: "active", Type: FieldBool},
			{Name: "embedding", Type: FieldVector},
			{Name: "created_at", Type: FieldTimestamp},
			{Name: "payload", Type: FieldBytes},
		},
	}

	codeStr, fileName, err := g.generateKotlin(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fileName != "UserFeatures.kt" {
		t.Errorf("expected UserFeatures.kt, got %s", fileName)
	}

	code := struct{ Code string }{Code: codeStr}

	checks := []struct {
		desc string
		want string
	}{
		{"generation header", "DO NOT EDIT"},
		{"package declaration", "package feathersdk"},
		{"serializable annotation", "@Serializable"},
		{"serial name import", "import kotlinx.serialization.SerialName"},
		{"data class declaration", "data class UserFeatures("},
		{"int64 type", "val age: Long"},
		{"float64 type", "val score: Double?"},
		{"string type", "val name: String"},
		{"bool type", "val active: Boolean?"},
		{"vector type", "val embedding: List<Double>?"},
		{"timestamp type for created_at", "val createdAt: String?"},
		{"bytes type", "val payload: ByteArray?"},
		{"serial name for entity_id", "@SerialName(\"entity_id\")"},
		{"serial name for created_at", "@SerialName(\"created_at\")"},
		{"description", "User feature group"},
	}

	for _, c := range checks {
		if !strings.Contains(code.Code, c.want) {
			t.Errorf("expected %s: %q not found in generated code", c.desc, c.want)
		}
	}
}

func TestKotlinTypeFor(t *testing.T) {
	tests := []struct {
		ft   FieldType
		want string
	}{
		{FieldInt64, "Long"},
		{FieldFloat64, "Double"},
		{FieldString, "String"},
		{FieldBool, "Boolean"},
		{FieldBytes, "ByteArray"},
		{FieldVector, "List<Double>"},
		{FieldTimestamp, "String"},
		{FieldType("unknown"), "Any"},
	}

	for _, tt := range tests {
		got := kotlinTypeFor(tt.ft)
		if got != tt.want {
			t.Errorf("kotlinTypeFor(%s) = %s, want %s", tt.ft, got, tt.want)
		}
	}
}
