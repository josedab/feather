package versioning

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Snapshot represents a point-in-time capture of feature state.
type Snapshot struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	CommitID    string                 `json:"commit_id"`
	Branch      string                 `json:"branch"`
	State       map[string]interface{} `json:"state"`
	CreatedAt   time.Time              `json:"created_at"`
	SizeBytes   int64                  `json:"size_bytes"`
}

// SnapshotDiff represents the differences between two snapshots.
type SnapshotDiff struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Changed []string `json:"changed"`
}

// SnapshotManager manages point-in-time snapshots of the version store.
type SnapshotManager struct {
	snapshots map[string]*Snapshot
	store     *VersionStore
	mu        sync.RWMutex
}

// NewSnapshotManager creates a new SnapshotManager linked to a VersionStore.
func NewSnapshotManager(store *VersionStore) *SnapshotManager {
	return &SnapshotManager{
		snapshots: make(map[string]*Snapshot),
		store:     store,
	}
}

// CreateSnapshot creates a snapshot of the current state on the active branch.
func (sm *SnapshotManager) CreateSnapshot(ctx context.Context, name, description string, state map[string]interface{}) (*Snapshot, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("creating snapshot: %w", ctx.Err())
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, s := range sm.snapshots {
		if s.Name == name {
			return nil, fmt.Errorf("creating snapshot: snapshot with name %q already exists", name)
		}
	}

	sm.store.mu.RLock()
	branchName := sm.store.activeBranch
	branch := sm.store.branches[branchName]
	commitID := branch.Head
	sm.store.mu.RUnlock()

	// Estimate size from state keys and values.
	var sizeBytes int64
	for k, v := range state {
		sizeBytes += int64(len(k))
		sizeBytes += int64(len(fmt.Sprintf("%v", v)))
	}

	snap := &Snapshot{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		CommitID:    commitID,
		Branch:      branchName,
		State:       state,
		CreatedAt:   time.Now(),
		SizeBytes:   sizeBytes,
	}

	sm.snapshots[snap.ID] = snap
	return snap, nil
}

// RestoreSnapshot rolls back the version store to the snapshot's commit.
func (sm *SnapshotManager) RestoreSnapshot(ctx context.Context, id string) error {
	if ctx.Err() != nil {
		return fmt.Errorf("restoring snapshot: %w", ctx.Err())
	}

	sm.mu.RLock()
	snap, ok := sm.snapshots[id]
	sm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("restoring snapshot: snapshot %q not found", id)
	}

	// Switch to the snapshot's branch and rollback to its commit.
	if err := sm.store.SwitchBranch(snap.Branch); err != nil {
		return fmt.Errorf("restoring snapshot: %w", err)
	}
	if snap.CommitID == "" {
		return nil
	}
	if err := sm.store.Rollback(ctx, snap.CommitID); err != nil {
		return fmt.Errorf("restoring snapshot: %w", err)
	}
	return nil
}

// DeleteSnapshot removes a snapshot by ID.
func (sm *SnapshotManager) DeleteSnapshot(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.snapshots[id]; !ok {
		return fmt.Errorf("deleting snapshot: snapshot %q not found", id)
	}
	delete(sm.snapshots, id)
	return nil
}

// ListSnapshots returns all snapshots sorted by creation time (newest first).
func (sm *SnapshotManager) ListSnapshots() []*Snapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	snapshots := make([]*Snapshot, 0, len(sm.snapshots))
	for _, s := range sm.snapshots {
		snapshots = append(snapshots, s)
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})
	return snapshots
}

// GetSnapshot returns a snapshot by ID.
func (sm *SnapshotManager) GetSnapshot(id string) (*Snapshot, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	snap, ok := sm.snapshots[id]
	if !ok {
		return nil, fmt.Errorf("getting snapshot: snapshot %q not found", id)
	}
	return snap, nil
}

// CompareSnapshots returns the differences between two snapshots based on their state keys.
func (sm *SnapshotManager) CompareSnapshots(id1, id2 string) (*SnapshotDiff, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	snap1, ok := sm.snapshots[id1]
	if !ok {
		return nil, fmt.Errorf("comparing snapshots: snapshot %q not found", id1)
	}
	snap2, ok := sm.snapshots[id2]
	if !ok {
		return nil, fmt.Errorf("comparing snapshots: snapshot %q not found", id2)
	}

	diff := &SnapshotDiff{}
	for key, val1 := range snap1.State {
		val2, exists := snap2.State[key]
		if !exists {
			diff.Removed = append(diff.Removed, key)
		} else if fmt.Sprintf("%v", val1) != fmt.Sprintf("%v", val2) {
			diff.Changed = append(diff.Changed, key)
		}
	}
	for key := range snap2.State {
		if _, exists := snap1.State[key]; !exists {
			diff.Added = append(diff.Added, key)
		}
	}

	sort.Strings(diff.Added)
	sort.Strings(diff.Removed)
	sort.Strings(diff.Changed)
	return diff, nil
}
