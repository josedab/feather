// Package feastcompat provides a drop-in REST API compatibility layer with
// the Feast feature store SDK. It allows existing Feast clients to connect
// to Feather with zero code changes, providing seamless migration.
//
// Key components:
//   - Adapter: Translates Feast API calls to Feather operations
//   - Registry: Maps Feast feature views to Feather feature groups
package feastcompat
