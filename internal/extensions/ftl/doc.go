// Package ftl provides a Feature Transformation Language (FTL) compiler
// for in-memory feature transformations using a SQL-like DSL.
//
// FTL supports SELECT, FROM, WHERE, JOIN, GROUP BY, ORDER BY, and LIMIT
// clauses for declarative feature pipeline definitions. Compiled pipelines
// can be cached and reused for efficient repeated execution.
//
// Usage:
//
//	compiler := ftl.NewCompiler()
//	pipeline, err := compiler.Compile("SELECT click_count FROM user_features WHERE entity = 'user:123'")
package ftl
