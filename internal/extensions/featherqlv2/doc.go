// Package featherqlv2 provides a SQL-like declarative DSL for defining
// end-to-end feature pipelines with window functions, joins, and query
// optimization. FeatherQL v2 compiles queries to optimized execution plans.
//
// Example query:
//
//	SELECT avg(amount) OVER (PARTITION BY user_id WINDOW '1h') AS avg_spend
//	FROM transactions
//
// Key components:
//   - Parser: Parses FeatherQL v2 syntax into AST
//   - Compiler: Compiles AST to execution plans
//   - Executor: Runs compiled pipelines against feature data
package featherqlv2
