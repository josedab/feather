package sdkcodegen

import (
	"strings"
	"testing"
)

func TestGenerateRust(t *testing.T) {
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

	codeStr, fileName, err := g.generateRust(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fileName != "user_features.rs" {
		t.Errorf("expected user_features.rs, got %s", fileName)
	}

	code := struct{ Code string }{Code: codeStr}

	checks := []struct {
		desc string
		want string
	}{
		{"generation header", "DO NOT EDIT"},
		{"serde import", "use serde::{Deserialize, Serialize}"},
		{"chrono import", "use chrono::{DateTime, Utc}"},
		{"derive macros", "#[derive(Debug, Clone, Serialize, Deserialize)]"},
		{"struct declaration", "pub struct UserFeatures"},
		{"int64 type", "pub age: i64"},
		{"float64 type", "pub score: Option<f64>"},
		{"string type", "pub name: String"},
		{"bool type", "pub active: Option<bool>"},
		{"vector type", "pub embedding: Option<Vec<f64>>"},
		{"timestamp type", "pub created_at: Option<DateTime<Utc>>"},
		{"bytes type", "pub payload: Option<Vec<u8>>"},
		{"builder struct", "pub struct UserFeaturesBuilder"},
		{"builder constructor", "pub fn builder() -> UserFeaturesBuilder"},
		{"builder field method", "pub fn age(mut self, val: i64) -> Self"},
		{"builder build", "pub fn build(self) -> Result<UserFeatures"},
		{"required field check", "self.age.ok_or(\"age is required\")"},
		{"optional field", "self.score,"},
		{"description", "/// User feature group"},
	}

	for _, c := range checks {
		if !strings.Contains(code.Code, c.want) {
			t.Errorf("expected %s: %q not found in generated code", c.desc, c.want)
		}
	}
}

func TestRustTypeFor(t *testing.T) {
	tests := []struct {
		ft   FieldType
		want string
	}{
		{FieldInt64, "i64"},
		{FieldFloat64, "f64"},
		{FieldString, "String"},
		{FieldBool, "bool"},
		{FieldBytes, "Vec<u8>"},
		{FieldVector, "Vec<f64>"},
		{FieldTimestamp, "DateTime<Utc>"},
		{FieldType("unknown"), "serde_json::Value"},
	}

	for _, tt := range tests {
		got := rustTypeFor(tt.ft)
		if got != tt.want {
			t.Errorf("rustTypeFor(%s) = %s, want %s", tt.ft, got, tt.want)
		}
	}
}
