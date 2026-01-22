// Package sdkcodegen provides automatic type-safe SDK client generation from
// feature schema definitions. It generates Go, Python, and TypeScript clients
// with compile-time validation, IDE autocomplete, and schema evolution support.
//
// Key components:
//   - Generator: Produces typed SDK code from feature group schemas
//   - Template: Language-specific code templates
//   - SchemaIR: Intermediate representation for cross-language generation
package sdkcodegen
