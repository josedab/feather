package federation

import "errors"

var (
	// ErrInvalidNodeID is returned when a node ID is empty or invalid.
	ErrInvalidNodeID = errors.New("invalid node ID")

	// ErrNodeNotFound is returned when a node is not found in the federation.
	ErrNodeNotFound = errors.New("node not found")

	// ErrNodeAlreadyExists is returned when trying to add a node that already exists.
	ErrNodeAlreadyExists = errors.New("node already exists")

	// ErrInvalidFeatureID is returned when a feature ID is empty or invalid.
	ErrInvalidFeatureID = errors.New("invalid feature ID")

	// ErrFeatureNotFound is returned when a feature is not found in the catalog.
	ErrFeatureNotFound = errors.New("feature not found")

	// ErrNotFeatureOwner is returned when trying to modify a feature not owned by this node.
	ErrNotFeatureOwner = errors.New("not the owner of this feature")

	// ErrReplicationDenied is returned when a node doesn't have replication permissions.
	ErrReplicationDenied = errors.New("replication denied")

	// ErrAccessDenied is returned when access to a feature is denied.
	ErrAccessDenied = errors.New("access denied")

	// ErrNodeUnreachable is returned when a node cannot be reached.
	ErrNodeUnreachable = errors.New("node unreachable")

	// ErrSyncFailed is returned when catalog synchronization fails.
	ErrSyncFailed = errors.New("sync failed")
)
