package sdkcodegen

import (
	"strings"
	"testing"
)

func TestGenerateJava(t *testing.T) {
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

	codeStr, fileName, err := g.generateJava(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fileName != "UserFeatures.java" {
		t.Errorf("expected UserFeatures.java, got %s", fileName)
	}

	code := struct{ Code string }{Code: codeStr}

	checks := []struct {
		desc string
		want string
	}{
		{"generation header", "DO NOT EDIT"},
		{"package declaration", "package feathersdk;"},
		{"jackson import", "com.fasterxml.jackson.annotation.JsonProperty"},
		{"class declaration", "public class UserFeatures"},
		{"int64 type", "private Long age"},
		{"float64 type", "private Double score"},
		{"string type", "private String name"},
		{"bool type", "private Boolean active"},
		{"vector type", "private List<Double> embedding"},
		{"timestamp type", "private Instant createdAt"},
		{"bytes type", "private byte[] payload"},
		{"getter", "public Long getAge()"},
		{"setter", "public void setAge(Long age)"},
		{"builder class", "public static class Builder"},
		{"builder method", "public static Builder builder()"},
		{"builder field method", "public Builder age(Long age)"},
		{"builder build", "public UserFeatures build()"},
		{"json property", "@JsonProperty(\"age\")"},
		{"description", "User feature group"},
	}

	for _, c := range checks {
		if !strings.Contains(code.Code, c.want) {
			t.Errorf("expected %s: %q not found in generated code", c.desc, c.want)
		}
	}
}

func TestJavaTypeFor(t *testing.T) {
	tests := []struct {
		ft   FieldType
		want string
	}{
		{FieldInt64, "Long"},
		{FieldFloat64, "Double"},
		{FieldString, "String"},
		{FieldBool, "Boolean"},
		{FieldBytes, "byte[]"},
		{FieldVector, "List<Double>"},
		{FieldTimestamp, "Instant"},
		{FieldType("unknown"), "Object"},
	}

	for _, tt := range tests {
		got := javaTypeFor(tt.ft)
		if got != tt.want {
			t.Errorf("javaTypeFor(%s) = %s, want %s", tt.ft, got, tt.want)
		}
	}
}

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"user_name", "userName"},
		{"age", "age"},
		{"created_at", "createdAt"},
		{"", ""},
	}

	for _, tt := range tests {
		got := toCamelCase(tt.in)
		if got != tt.want {
			t.Errorf("toCamelCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
