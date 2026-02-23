package sdkcodegen

import (
	"strings"
	"testing"
)

func TestGenerateSwift(t *testing.T) {
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

	codeStr, fileName, err := g.generateSwift(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fileName != "UserFeatures.swift" {
		t.Errorf("expected UserFeatures.swift, got %s", fileName)
	}

	code := struct{ Code string }{Code: codeStr}

	checks := []struct {
		desc string
		want string
	}{
		{"generation header", "DO NOT EDIT"},
		{"foundation import", "import Foundation"},
		{"codable conformance", "public struct UserFeatures: Codable"},
		{"int64 type", "public let age: Int64"},
		{"float64 type", "public let score: Double?"},
		{"string type", "public let name: String"},
		{"bool type", "public let active: Bool?"},
		{"vector type", "public let embedding: [Double]?"},
		{"timestamp type", "public let createdAt: Date?"},
		{"bytes type", "public let payload: Data?"},
		{"coding keys", "enum CodingKeys: String, CodingKey"},
		{"entity_id coding key", "case entityId = \"entity_id\""},
		{"created_at coding key", "case createdAt = \"created_at\""},
		{"init method", "public init("},
		{"description", "/// User feature group"},
	}

	for _, c := range checks {
		if !strings.Contains(code.Code, c.want) {
			t.Errorf("expected %s: %q not found in generated code", c.desc, c.want)
		}
	}
}

func TestSwiftTypeFor(t *testing.T) {
	tests := []struct {
		ft   FieldType
		want string
	}{
		{FieldInt64, "Int64"},
		{FieldFloat64, "Double"},
		{FieldString, "String"},
		{FieldBool, "Bool"},
		{FieldBytes, "Data"},
		{FieldVector, "[Double]"},
		{FieldTimestamp, "Date"},
		{FieldType("unknown"), "Any"},
	}

	for _, tt := range tests {
		got := swiftTypeFor(tt.ft)
		if got != tt.want {
			t.Errorf("swiftTypeFor(%s) = %s, want %s", tt.ft, got, tt.want)
		}
	}
}
