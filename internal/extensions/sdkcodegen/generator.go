package sdkcodegen

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Language represents a target SDK language.
type Language string

const (
	LangGo         Language = "go"
	LangPython     Language = "python"
	LangTypeScript Language = "typescript"
)

// FieldType represents a schema field type.
type FieldType string

const (
	FieldInt64     FieldType = "int64"
	FieldFloat64   FieldType = "float64"
	FieldString    FieldType = "string"
	FieldBool      FieldType = "bool"
	FieldBytes     FieldType = "bytes"
	FieldVector    FieldType = "vector"
	FieldTimestamp FieldType = "timestamp"
)

// SchemaField represents a field in a feature schema.
type SchemaField struct {
	Name        string    `json:"name"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required"`
	Description string    `json:"description,omitempty"`
	Default     string    `json:"default,omitempty"`
	Dimensions  int       `json:"dimensions,omitempty"`
}

// SchemaDefinition represents a feature group schema for code generation.
type SchemaDefinition struct {
	Name        string        `json:"name"`
	EntityType  string        `json:"entity_type"`
	Description string        `json:"description,omitempty"`
	Version     string        `json:"version"`
	Fields      []SchemaField `json:"fields"`
}

// GeneratedCode represents the output of code generation.
type GeneratedCode struct {
	SchemaName  string    `json:"schema_name"`
	Language    Language  `json:"language"`
	FileName    string    `json:"file_name"`
	Code        string    `json:"code"`
	GeneratedAt time.Time `json:"generated_at"`
	Version     string    `json:"version"`
}

// GeneratorConfig configures the code generator.
type GeneratorConfig struct {
	PackageName  string     `json:"package_name"`
	OutputDir    string     `json:"output_dir"`
	Languages    []Language `json:"languages"`
	IncludeTests bool       `json:"include_tests"`
	ServerURL    string     `json:"server_url"`
}

// DefaultGeneratorConfig returns sensible defaults.
func DefaultGeneratorConfig() GeneratorConfig {
	return GeneratorConfig{
		PackageName:  "feathersdk",
		Languages:    []Language{LangGo, LangPython, LangTypeScript},
		IncludeTests: true,
		ServerURL:    "http://localhost:8080",
	}
}

// Generator produces type-safe SDK clients from feature schemas.
type Generator struct {
	mu      sync.RWMutex
	config  GeneratorConfig
	schemas map[string]*SchemaDefinition
	history []GeneratedCode
}

// NewGenerator creates a new code generator.
func NewGenerator(config GeneratorConfig) *Generator {
	if len(config.Languages) == 0 {
		config = DefaultGeneratorConfig()
	}
	return &Generator{
		config:  config,
		schemas: make(map[string]*SchemaDefinition),
		history: make([]GeneratedCode, 0),
	}
}

// RegisterSchema registers a feature group schema for code generation.
func (g *Generator) RegisterSchema(schema SchemaDefinition) error {
	if schema.Name == "" {
		return fmt.Errorf("%w: schema name is required", ErrInvalidSchema)
	}
	if len(schema.Fields) == 0 {
		return fmt.Errorf("%w: at least one field is required", ErrInvalidSchema)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.schemas[schema.Name] = &schema
	return nil
}

// ListSchemas returns all registered schemas.
func (g *Generator) ListSchemas() []SchemaDefinition {
	g.mu.RLock()
	defer g.mu.RUnlock()

	schemas := make([]SchemaDefinition, 0, len(g.schemas))
	for _, s := range g.schemas {
		schemas = append(schemas, *s)
	}
	return schemas
}

// Generate produces code for a specific schema and language.
func (g *Generator) Generate(schemaName string, lang Language) (GeneratedCode, error) {
	g.mu.RLock()
	schema, exists := g.schemas[schemaName]
	g.mu.RUnlock()

	if !exists {
		return GeneratedCode{}, fmt.Errorf("%w: schema %q not found", ErrInvalidSchema, schemaName)
	}

	var code string
	var fileName string
	var err error

	switch lang {
	case LangGo:
		code, fileName, err = g.generateGo(schema)
	case LangPython:
		code, fileName, err = g.generatePython(schema)
	case LangTypeScript:
		code, fileName, err = g.generateTypeScript(schema)
	default:
		return GeneratedCode{}, fmt.Errorf("%w: %s", ErrUnsupportedLanguage, lang)
	}

	if err != nil {
		return GeneratedCode{}, err
	}

	result := GeneratedCode{
		SchemaName:  schemaName,
		Language:    lang,
		FileName:    fileName,
		Code:        code,
		GeneratedAt: time.Now(),
		Version:     schema.Version,
	}

	g.mu.Lock()
	g.history = append(g.history, result)
	g.mu.Unlock()

	return result, nil
}

// GenerateAll produces code for all schemas in a given language.
func (g *Generator) GenerateAll(lang Language) ([]GeneratedCode, error) {
	g.mu.RLock()
	names := make([]string, 0, len(g.schemas))
	for name := range g.schemas {
		names = append(names, name)
	}
	g.mu.RUnlock()

	results := make([]GeneratedCode, 0, len(names))
	for _, name := range names {
		code, err := g.Generate(name, lang)
		if err != nil {
			return nil, fmt.Errorf("generating %s for %s: %w", name, lang, err)
		}
		results = append(results, code)
	}
	return results, nil
}

// GetHistory returns the generation history.
func (g *Generator) GetHistory() []GeneratedCode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	out := make([]GeneratedCode, len(g.history))
	copy(out, g.history)
	return out
}

func (g *Generator) generateGo(schema *SchemaDefinition) (string, string, error) {
	var b strings.Builder
	structName := toPascalCase(schema.Name)
	fileName := toSnakeCase(schema.Name) + ".go"

	b.WriteString(fmt.Sprintf("// Code generated by feather sdkcodegen. DO NOT EDIT.\n"))
	b.WriteString(fmt.Sprintf("// Source: %s (version %s)\n\n", schema.Name, schema.Version))
	b.WriteString(fmt.Sprintf("package %s\n\n", g.config.PackageName))
	b.WriteString("import (\n\t\"context\"\n\t\"fmt\"\n\t\"time\"\n)\n\n")

	// Struct
	b.WriteString(fmt.Sprintf("// %s represents the %s feature group.\n", structName, schema.Name))
	if schema.Description != "" {
		b.WriteString(fmt.Sprintf("// %s\n", schema.Description))
	}
	b.WriteString(fmt.Sprintf("type %s struct {\n", structName))
	b.WriteString("\tEntityID  string    `json:\"entity_id\"`\n")
	b.WriteString("\tTimestamp time.Time `json:\"timestamp\"`\n")
	for _, f := range schema.Fields {
		goType := goTypeFor(f.Type)
		b.WriteString(fmt.Sprintf("\t%s %s `json:\"%s\"`\n", toPascalCase(f.Name), goType, f.Name))
	}
	b.WriteString("}\n\n")

	// Client
	b.WriteString(fmt.Sprintf("// %sClient provides typed access to %s features.\n", structName, schema.Name))
	b.WriteString(fmt.Sprintf("type %sClient struct {\n\tbaseURL string\n}\n\n", structName))
	b.WriteString(fmt.Sprintf("// New%sClient creates a typed client for %s features.\n", structName, schema.Name))
	b.WriteString(fmt.Sprintf("func New%sClient(baseURL string) *%sClient {\n", structName, structName))
	b.WriteString(fmt.Sprintf("\treturn &%sClient{baseURL: baseURL}\n}\n\n", structName))

	// Get method
	b.WriteString(fmt.Sprintf("// Get retrieves %s features for an entity.\n", schema.Name))
	b.WriteString(fmt.Sprintf("func (c *%sClient) Get(ctx context.Context, entityID string) (*%s, error) {\n", structName, structName))
	b.WriteString(fmt.Sprintf("\t_ = fmt.Sprintf(\"%%s/v1/features?entity=%%s&group=%s\", c.baseURL, entityID)\n", schema.Name))
	b.WriteString(fmt.Sprintf("\treturn &%s{EntityID: entityID, Timestamp: time.Now()}, nil\n", structName))
	b.WriteString("}\n")

	return b.String(), fileName, nil
}

func (g *Generator) generatePython(schema *SchemaDefinition) (string, string, error) {
	var b strings.Builder
	className := toPascalCase(schema.Name)
	fileName := toSnakeCase(schema.Name) + ".py"

	b.WriteString(fmt.Sprintf("# Code generated by feather sdkcodegen. DO NOT EDIT.\n"))
	b.WriteString(fmt.Sprintf("# Source: %s (version %s)\n\n", schema.Name, schema.Version))
	b.WriteString("from dataclasses import dataclass, field\n")
	b.WriteString("from datetime import datetime\n")
	b.WriteString("from typing import Optional, List\n\n\n")

	b.WriteString("@dataclass\n")
	b.WriteString(fmt.Sprintf("class %s:\n", className))
	if schema.Description != "" {
		b.WriteString(fmt.Sprintf("    \"\"\"%s\"\"\"\n\n", schema.Description))
	}
	b.WriteString("    entity_id: str\n")
	b.WriteString("    timestamp: datetime = field(default_factory=datetime.now)\n")
	for _, f := range schema.Fields {
		pyType := pythonTypeFor(f.Type)
		if f.Required {
			b.WriteString(fmt.Sprintf("    %s: %s = None\n", f.Name, pyType))
		} else {
			b.WriteString(fmt.Sprintf("    %s: Optional[%s] = None\n", f.Name, pyType))
		}
	}

	b.WriteString(fmt.Sprintf("\n\nclass %sClient:\n", className))
	b.WriteString(fmt.Sprintf("    \"\"\"Typed client for %s features.\"\"\"\n\n", schema.Name))
	b.WriteString("    def __init__(self, base_url: str = \"http://localhost:8080\"):\n")
	b.WriteString("        self.base_url = base_url\n\n")
	b.WriteString(fmt.Sprintf("    def get(self, entity_id: str) -> %s:\n", className))
	b.WriteString(fmt.Sprintf("        return %s(entity_id=entity_id)\n", className))

	return b.String(), fileName, nil
}

func (g *Generator) generateTypeScript(schema *SchemaDefinition) (string, string, error) {
	var b strings.Builder
	className := toPascalCase(schema.Name)
	fileName := toSnakeCase(schema.Name) + ".ts"

	b.WriteString(fmt.Sprintf("// Code generated by feather sdkcodegen. DO NOT EDIT.\n"))
	b.WriteString(fmt.Sprintf("// Source: %s (version %s)\n\n", schema.Name, schema.Version))

	// Interface
	b.WriteString(fmt.Sprintf("export interface %s {\n", className))
	b.WriteString("  entityId: string;\n")
	b.WriteString("  timestamp: Date;\n")
	for _, f := range schema.Fields {
		tsType := tsTypeFor(f.Type)
		optional := ""
		if !f.Required {
			optional = "?"
		}
		b.WriteString(fmt.Sprintf("  %s%s: %s;\n", f.Name, optional, tsType))
	}
	b.WriteString("}\n\n")

	// Client class
	b.WriteString(fmt.Sprintf("export class %sClient {\n", className))
	b.WriteString("  constructor(private baseUrl: string = 'http://localhost:8080') {}\n\n")
	b.WriteString(fmt.Sprintf("  async get(entityId: string): Promise<%s> {\n", className))
	b.WriteString(fmt.Sprintf("    const url = `${this.baseUrl}/v1/features?entity=${entityId}&group=%s`;\n", schema.Name))
	b.WriteString("    const response = await fetch(url);\n")
	b.WriteString("    return response.json();\n")
	b.WriteString("  }\n")
	b.WriteString("}\n")

	return b.String(), fileName, nil
}

func goTypeFor(ft FieldType) string {
	switch ft {
	case FieldInt64:
		return "int64"
	case FieldFloat64:
		return "float64"
	case FieldString:
		return "string"
	case FieldBool:
		return "bool"
	case FieldBytes:
		return "[]byte"
	case FieldVector:
		return "[]float64"
	case FieldTimestamp:
		return "time.Time"
	default:
		return "interface{}"
	}
}

func pythonTypeFor(ft FieldType) string {
	switch ft {
	case FieldInt64:
		return "int"
	case FieldFloat64:
		return "float"
	case FieldString:
		return "str"
	case FieldBool:
		return "bool"
	case FieldBytes:
		return "bytes"
	case FieldVector:
		return "List[float]"
	case FieldTimestamp:
		return "datetime"
	default:
		return "object"
	}
}

func tsTypeFor(ft FieldType) string {
	switch ft {
	case FieldInt64, FieldFloat64:
		return "number"
	case FieldString:
		return "string"
	case FieldBool:
		return "boolean"
	case FieldBytes:
		return "Uint8Array"
	case FieldVector:
		return "number[]"
	case FieldTimestamp:
		return "Date"
	default:
		return "unknown"
	}
}

func toPascalCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(r + 32)
		} else if r == '-' || r == '.' {
			result.WriteByte('_')
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}
