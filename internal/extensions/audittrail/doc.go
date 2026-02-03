// Package audittrail provides an immutable, event-sourced audit log with
// Merkle tree cryptographic hash chaining for feature mutations.
//
// # Usage
//
//	trail := audittrail.NewTrail(audittrail.DefaultConfig())
//	trail.Record(audittrail.Event{Action: "write", Entity: "user:123"})
//	proof, _ := trail.GetProof("user:123", timeRange)
package audittrail
