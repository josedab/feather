// Package versioning provides git-like versioning for feature definitions and values.
//
// It implements a branching, commit, and rollback model inspired by Git,
// enabling teams to safely evolve feature definitions with full audit trails
// and the ability to roll back to any previous state.
//
// Key components:
//   - VersionStore: Manages branches, commits, tags, and history
//   - SnapshotManager: Creates and restores point-in-time snapshots
//
// Example usage:
//
//	store := versioning.NewVersionStore()
//	commit, err := store.Commit(ctx, "add user features", "alice", changes)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	err = store.Rollback(ctx, commit.ID)
package versioning
