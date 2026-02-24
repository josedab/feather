// Package contractcicd implements schema-as-code contracts for Feather feature
// definitions with CI/CD integration.
//
// It provides a declarative contract format (YAML) that can be checked into
// version control and enforced in CI pipelines. The plan/apply workflow
// detects breaking changes, generates migrations, and integrates with
// GitHub Actions and other CI systems.
//
// # Architecture
//
//   - Schema Contract: YAML-based feature contract definitions with
//     versioning, ownership, and validation rules.
//   - Plan/Apply Engine: Compares desired state (contracts) against current
//     state (registry) and generates a migration plan with breaking-change
//     detection.
//   - CI/CD Integration: GitHub Action templates and CLI wrappers for
//     automated contract enforcement in pull requests.
//
// # Usage
//
//	engine := contractcicd.NewEngine(contractcicd.DefaultEngineConfig())
//	plan, _ := engine.Plan("contracts/")
//	if plan.HasBreakingChanges() {
//	    // Reject PR
//	}
//	result, _ := engine.Apply(plan)
package contractcicd
