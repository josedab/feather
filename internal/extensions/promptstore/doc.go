// Package promptstore provides first-class support for LLM prompt templates
// as managed features. It supports versioning, A/B rollout, performance tracking,
// token usage attribution, and semantic similarity deduplication across teams.
//
// Key components:
//   - Store: Manages prompt templates with versioning and rollout
//   - Template: Represents a versioned prompt with metadata
//   - Tracker: Monitors prompt performance and token usage
package promptstore
