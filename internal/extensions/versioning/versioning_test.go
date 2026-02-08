package versioning

import (
	"context"
	"testing"
)

func TestCreateBranch(t *testing.T) {
	vs := NewVersionStore()

	if err := vs.CreateBranch("feature-1", "main"); err != nil {
		t.Fatalf("CreateBranch() error = %v", err)
	}

	branches := vs.ListBranches()
	if len(branches) != 2 {
		t.Fatalf("expected 2 branches, got %d", len(branches))
	}

	// Duplicate branch name.
	if err := vs.CreateBranch("feature-1", "main"); err == nil {
		t.Fatal("expected error creating duplicate branch")
	}

	// Non-existent base branch.
	if err := vs.CreateBranch("feature-2", "nonexistent"); err == nil {
		t.Fatal("expected error with nonexistent base branch")
	}

	// Verify branch properties.
	b, err := vs.GetBranch("feature-1")
	if err != nil {
		t.Fatalf("GetBranch() error = %v", err)
	}
	if b.Base != "main" {
		t.Errorf("expected base 'main', got %q", b.Base)
	}
	if b.IsDefault {
		t.Error("expected feature branch to not be default")
	}
}

func TestCommit(t *testing.T) {
	ctx := context.Background()
	vs := NewVersionStore()

	changes := []*Change{
		{Type: ChangeTypeCreate, Path: "features/user_age", NewValue: "int"},
	}

	commit, err := vs.Commit(ctx, "add user_age feature", "alice", changes)
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if commit.ID == "" {
		t.Fatal("expected non-empty commit ID")
	}
	if commit.Author != "alice" {
		t.Errorf("expected author 'alice', got %q", commit.Author)
	}
	if commit.Branch != "main" {
		t.Errorf("expected branch 'main', got %q", commit.Branch)
	}
	if len(commit.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(commit.Changes))
	}

	// Verify head was updated.
	branch, _ := vs.GetBranch("main")
	if branch.Head != commit.ID {
		t.Errorf("expected branch head %q, got %q", commit.ID, branch.Head)
	}

	// Commit with no changes should fail.
	_, err = vs.Commit(ctx, "empty", "alice", nil)
	if err == nil {
		t.Fatal("expected error committing with no changes")
	}

	// GetCommit.
	got, err := vs.GetCommit(commit.ID)
	if err != nil {
		t.Fatalf("GetCommit() error = %v", err)
	}
	if got.Message != "add user_age feature" {
		t.Errorf("expected message 'add user_age feature', got %q", got.Message)
	}

	// GetCommit not found.
	_, err = vs.GetCommit("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent commit")
	}
}

func TestGetHistory(t *testing.T) {
	ctx := context.Background()
	vs := NewVersionStore()

	// Create 3 commits.
	for i := 0; i < 3; i++ {
		changes := []*Change{
			{Type: ChangeTypeCreate, Path: "features/f", NewValue: i},
		}
		_, err := vs.Commit(ctx, "commit", "alice", changes)
		if err != nil {
			t.Fatalf("Commit() error = %v", err)
		}
	}

	history := vs.GetHistory("main", 0)
	if len(history) != 3 {
		t.Fatalf("expected 3 commits in history, got %d", len(history))
	}

	// History should be newest first.
	if history[0].CreatedAt.Before(history[2].CreatedAt) {
		t.Error("expected history ordered newest first")
	}

	// With limit.
	limited := vs.GetHistory("main", 2)
	if len(limited) != 2 {
		t.Fatalf("expected 2 commits with limit, got %d", len(limited))
	}

	// Nonexistent branch.
	if h := vs.GetHistory("nonexistent", 0); h != nil {
		t.Errorf("expected nil for nonexistent branch, got %v", h)
	}
}

func TestDiff(t *testing.T) {
	ctx := context.Background()
	vs := NewVersionStore()

	changes1 := []*Change{
		{Type: ChangeTypeCreate, Path: "features/a", NewValue: "int"},
	}
	c1, _ := vs.Commit(ctx, "first", "alice", changes1)

	changes2 := []*Change{
		{Type: ChangeTypeUpdate, Path: "features/a", OldValue: "int", NewValue: "float"},
	}
	c2, _ := vs.Commit(ctx, "second", "alice", changes2)

	diff, err := vs.Diff(c1.ID, c2.ID)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if len(diff) != 1 {
		t.Fatalf("expected 1 change in diff, got %d", len(diff))
	}
	if diff[0].Type != ChangeTypeUpdate {
		t.Errorf("expected change type 'update', got %q", diff[0].Type)
	}

	// Nonexistent commit.
	_, err = vs.Diff("bad", c2.ID)
	if err == nil {
		t.Fatal("expected error for nonexistent from-commit")
	}
	_, err = vs.Diff(c1.ID, "bad")
	if err == nil {
		t.Fatal("expected error for nonexistent to-commit")
	}
}

func TestRollback(t *testing.T) {
	ctx := context.Background()
	vs := NewVersionStore()

	changes1 := []*Change{
		{Type: ChangeTypeCreate, Path: "features/a", NewValue: "v1"},
	}
	c1, _ := vs.Commit(ctx, "first", "alice", changes1)

	changes2 := []*Change{
		{Type: ChangeTypeUpdate, Path: "features/a", OldValue: "v1", NewValue: "v2"},
	}
	vs.Commit(ctx, "second", "alice", changes2)

	// Rollback to first commit.
	if err := vs.Rollback(ctx, c1.ID); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}

	branch, _ := vs.GetBranch("main")
	if branch.Head != c1.ID {
		t.Errorf("expected head %q after rollback, got %q", c1.ID, branch.Head)
	}

	// Rollback to nonexistent commit.
	if err := vs.Rollback(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error rolling back to nonexistent commit")
	}

	// Rollback to commit on different branch.
	vs.CreateBranch("other", "main")
	vs.SwitchBranch("other")
	otherChanges := []*Change{
		{Type: ChangeTypeCreate, Path: "features/b", NewValue: "v1"},
	}
	otherCommit, _ := vs.Commit(ctx, "other commit", "alice", otherChanges)

	vs.SwitchBranch("main")
	if err := vs.Rollback(ctx, otherCommit.ID); err == nil {
		t.Fatal("expected error rolling back to commit on different branch")
	}
}

func TestMerge(t *testing.T) {
	ctx := context.Background()
	vs := NewVersionStore()

	// Create initial commit on main.
	changes1 := []*Change{
		{Type: ChangeTypeCreate, Path: "features/a", NewValue: "v1"},
	}
	vs.Commit(ctx, "initial", "alice", changes1)

	// Create feature branch and add a commit.
	vs.CreateBranch("feature", "main")
	vs.SwitchBranch("feature")

	changes2 := []*Change{
		{Type: ChangeTypeCreate, Path: "features/b", NewValue: "new"},
	}
	vs.Commit(ctx, "feature work", "bob", changes2)

	// Merge feature into main.
	mergeCommit, err := vs.Merge(ctx, "feature", "main", "alice")
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}
	if mergeCommit.Metadata["merge_source"] != "feature" {
		t.Errorf("expected merge_source 'feature', got %q", mergeCommit.Metadata["merge_source"])
	}

	// Verify main branch head was updated.
	main, _ := vs.GetBranch("main")
	if main.Head != mergeCommit.ID {
		t.Errorf("expected main head %q, got %q", mergeCommit.ID, main.Head)
	}

	// Merge nonexistent source.
	_, err = vs.Merge(ctx, "nonexistent", "main", "alice")
	if err == nil {
		t.Fatal("expected error merging nonexistent source")
	}

	// Merge nonexistent target.
	_, err = vs.Merge(ctx, "feature", "nonexistent", "alice")
	if err == nil {
		t.Fatal("expected error merging into nonexistent target")
	}

	// Merge branch with no new changes.
	_, err = vs.Merge(ctx, "feature", "main", "alice")
	if err == nil {
		t.Fatal("expected error merging with no new changes")
	}
}

func TestTags(t *testing.T) {
	ctx := context.Background()
	vs := NewVersionStore()

	changes := []*Change{
		{Type: ChangeTypeCreate, Path: "features/a", NewValue: "v1"},
	}
	commit, _ := vs.Commit(ctx, "release", "alice", changes)

	// Create tag.
	if err := vs.CreateTag("v1.0", commit.ID, "first release"); err != nil {
		t.Fatalf("CreateTag() error = %v", err)
	}

	// Get tag.
	tag, err := vs.GetTag("v1.0")
	if err != nil {
		t.Fatalf("GetTag() error = %v", err)
	}
	if tag.CommitID != commit.ID {
		t.Errorf("expected commit ID %q, got %q", commit.ID, tag.CommitID)
	}
	if tag.Message != "first release" {
		t.Errorf("expected message 'first release', got %q", tag.Message)
	}

	// Duplicate tag.
	if err := vs.CreateTag("v1.0", commit.ID, ""); err == nil {
		t.Fatal("expected error creating duplicate tag")
	}

	// Tag for nonexistent commit.
	if err := vs.CreateTag("v2.0", "nonexistent", ""); err == nil {
		t.Fatal("expected error tagging nonexistent commit")
	}

	// List tags.
	tags := vs.ListTags()
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}

	// Get nonexistent tag.
	_, err = vs.GetTag("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent tag")
	}
}

func TestSnapshots(t *testing.T) {
	ctx := context.Background()
	vs := NewVersionStore()
	sm := NewSnapshotManager(vs)

	changes := []*Change{
		{Type: ChangeTypeCreate, Path: "features/a", NewValue: "v1"},
	}
	vs.Commit(ctx, "initial", "alice", changes)

	state := map[string]interface{}{
		"features/a": "v1",
	}

	// Create snapshot.
	snap, err := sm.CreateSnapshot(ctx, "snap-1", "first snapshot", state)
	if err != nil {
		t.Fatalf("CreateSnapshot() error = %v", err)
	}
	if snap.Name != "snap-1" {
		t.Errorf("expected name 'snap-1', got %q", snap.Name)
	}
	if snap.SizeBytes == 0 {
		t.Error("expected non-zero size")
	}

	// Get snapshot.
	got, err := sm.GetSnapshot(snap.ID)
	if err != nil {
		t.Fatalf("GetSnapshot() error = %v", err)
	}
	if got.ID != snap.ID {
		t.Errorf("expected ID %q, got %q", snap.ID, got.ID)
	}

	// Duplicate name.
	_, err = sm.CreateSnapshot(ctx, "snap-1", "", state)
	if err == nil {
		t.Fatal("expected error creating snapshot with duplicate name")
	}

	// List snapshots.
	snaps := sm.ListSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}

	// Compare snapshots.
	state2 := map[string]interface{}{
		"features/a": "v2",
		"features/b": "new",
	}
	snap2, _ := sm.CreateSnapshot(ctx, "snap-2", "", state2)

	diff, err := sm.CompareSnapshots(snap.ID, snap2.ID)
	if err != nil {
		t.Fatalf("CompareSnapshots() error = %v", err)
	}
	if len(diff.Added) != 1 || diff.Added[0] != "features/b" {
		t.Errorf("expected 1 added key 'features/b', got %v", diff.Added)
	}
	if len(diff.Changed) != 1 || diff.Changed[0] != "features/a" {
		t.Errorf("expected 1 changed key 'features/a', got %v", diff.Changed)
	}

	// Restore snapshot.
	if err := sm.RestoreSnapshot(ctx, snap.ID); err != nil {
		t.Fatalf("RestoreSnapshot() error = %v", err)
	}

	// Delete snapshot.
	if err := sm.DeleteSnapshot(snap.ID); err != nil {
		t.Fatalf("DeleteSnapshot() error = %v", err)
	}
	if _, err := sm.GetSnapshot(snap.ID); err == nil {
		t.Fatal("expected error getting deleted snapshot")
	}

	// Delete nonexistent.
	if err := sm.DeleteSnapshot("nonexistent"); err == nil {
		t.Fatal("expected error deleting nonexistent snapshot")
	}
}

func TestProtectedBranch(t *testing.T) {
	vs := NewVersionStore()

	// Cannot delete default/protected branch.
	if err := vs.DeleteBranch("main"); err == nil {
		t.Fatal("expected error deleting protected branch")
	}

	// Cannot delete active branch.
	vs.CreateBranch("feature", "main")
	vs.SwitchBranch("feature")
	if err := vs.DeleteBranch("feature"); err == nil {
		t.Fatal("expected error deleting active branch")
	}

	// Can delete non-protected, non-active branch.
	vs.SwitchBranch("main")
	if err := vs.DeleteBranch("feature"); err != nil {
		t.Fatalf("DeleteBranch() error = %v", err)
	}

	// Delete nonexistent branch.
	if err := vs.DeleteBranch("nonexistent"); err == nil {
		t.Fatal("expected error deleting nonexistent branch")
	}

	// Switch to nonexistent branch.
	if err := vs.SwitchBranch("nonexistent"); err == nil {
		t.Fatal("expected error switching to nonexistent branch")
	}

	// GetBranch nonexistent.
	_, err := vs.GetBranch("nonexistent")
	if err == nil {
		t.Fatal("expected error getting nonexistent branch")
	}

	// Stats.
	stats := vs.Stats()
	if stats.BranchCount != 1 {
		t.Errorf("expected 1 branch, got %d", stats.BranchCount)
	}
	if stats.ActiveBranch != "main" {
		t.Errorf("expected active branch 'main', got %q", stats.ActiveBranch)
	}

	// GetCurrentBranch.
	if vs.GetCurrentBranch() != "main" {
		t.Errorf("expected current branch 'main', got %q", vs.GetCurrentBranch())
	}
}
