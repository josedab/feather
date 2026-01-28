package versioning

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ChangeType represents the type of change in a commit.
type ChangeType string

const (
	ChangeTypeCreate ChangeType = "create"
	ChangeTypeUpdate ChangeType = "update"
	ChangeTypeDelete ChangeType = "delete"
)

// Change represents a single change within a commit.
type Change struct {
	Type     ChangeType  `json:"type"`
	Path     string      `json:"path"`
	OldValue interface{} `json:"old_value,omitempty"`
	NewValue interface{} `json:"new_value,omitempty"`
}

// Commit represents a versioned set of changes.
type Commit struct {
	ID        string            `json:"id"`
	ParentID  string            `json:"parent_id,omitempty"`
	Branch    string            `json:"branch"`
	Message   string            `json:"message"`
	Author    string            `json:"author"`
	Changes   []*Change         `json:"changes"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// Branch represents a named line of development.
type Branch struct {
	Name      string    `json:"name"`
	Head      string    `json:"head"`
	Base      string    `json:"base"`
	IsDefault bool      `json:"is_default"`
	Protected bool      `json:"protected"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Tag represents a named reference to a specific commit.
type Tag struct {
	Name      string    `json:"name"`
	CommitID  string    `json:"commit_id"`
	Message   string    `json:"message,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// VersionStats holds summary statistics about the version store.
type VersionStats struct {
	BranchCount  int    `json:"branch_count"`
	CommitCount  int    `json:"commit_count"`
	TagCount     int    `json:"tag_count"`
	ActiveBranch string `json:"active_branch"`
}

// VersionStore manages branches, commits, and tags for feature versioning.
type VersionStore struct {
	branches     map[string]*Branch
	commits      map[string]*Commit
	tags         map[string]*Tag
	activeBranch string
	mu           sync.RWMutex
}

// NewVersionStore creates a new VersionStore with a default "main" branch.
func NewVersionStore() *VersionStore {
	now := time.Now()
	main := &Branch{
		Name:      "main",
		Head:      "",
		Base:      "",
		IsDefault: true,
		Protected: true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return &VersionStore{
		branches:     map[string]*Branch{"main": main},
		commits:      make(map[string]*Commit),
		tags:         make(map[string]*Tag),
		activeBranch: "main",
	}
}

// CreateBranch creates a new branch based on an existing branch.
func (vs *VersionStore) CreateBranch(name, baseBranch string) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if _, exists := vs.branches[name]; exists {
		return fmt.Errorf("creating branch: branch %q already exists", name)
	}
	base, ok := vs.branches[baseBranch]
	if !ok {
		return fmt.Errorf("creating branch: base branch %q not found", baseBranch)
	}

	now := time.Now()
	vs.branches[name] = &Branch{
		Name:      name,
		Head:      base.Head,
		Base:      baseBranch,
		IsDefault: false,
		Protected: false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return nil
}

// DeleteBranch removes a branch. Protected and default branches cannot be deleted.
func (vs *VersionStore) DeleteBranch(name string) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	branch, ok := vs.branches[name]
	if !ok {
		return fmt.Errorf("deleting branch: branch %q not found", name)
	}
	if branch.IsDefault {
		return fmt.Errorf("deleting branch: cannot delete default branch %q", name)
	}
	if branch.Protected {
		return fmt.Errorf("deleting branch: cannot delete protected branch %q", name)
	}
	if vs.activeBranch == name {
		return fmt.Errorf("deleting branch: cannot delete active branch %q", name)
	}
	delete(vs.branches, name)
	return nil
}

// ListBranches returns all branches sorted by name.
func (vs *VersionStore) ListBranches() []*Branch {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	branches := make([]*Branch, 0, len(vs.branches))
	for _, b := range vs.branches {
		branches = append(branches, b)
	}
	sort.Slice(branches, func(i, j int) bool {
		return branches[i].Name < branches[j].Name
	})
	return branches
}

// GetBranch returns a branch by name.
func (vs *VersionStore) GetBranch(name string) (*Branch, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	branch, ok := vs.branches[name]
	if !ok {
		return nil, fmt.Errorf("getting branch: branch %q not found", name)
	}
	return branch, nil
}

// SwitchBranch sets the active branch.
func (vs *VersionStore) SwitchBranch(name string) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if _, ok := vs.branches[name]; !ok {
		return fmt.Errorf("switching branch: branch %q not found", name)
	}
	vs.activeBranch = name
	return nil
}

// Commit creates a new commit on the active branch.
func (vs *VersionStore) Commit(ctx context.Context, message, author string, changes []*Change) (*Commit, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("committing changes: %w", ctx.Err())
	}

	vs.mu.Lock()
	defer vs.mu.Unlock()

	branch, ok := vs.branches[vs.activeBranch]
	if !ok {
		return nil, fmt.Errorf("committing changes: active branch %q not found", vs.activeBranch)
	}
	if len(changes) == 0 {
		return nil, fmt.Errorf("committing changes: no changes provided")
	}

	commit := &Commit{
		ID:        uuid.New().String(),
		ParentID:  branch.Head,
		Branch:    vs.activeBranch,
		Message:   message,
		Author:    author,
		Changes:   changes,
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
	}

	vs.commits[commit.ID] = commit
	branch.Head = commit.ID
	branch.UpdatedAt = commit.CreatedAt
	return commit, nil
}

// GetCommit returns a commit by ID.
func (vs *VersionStore) GetCommit(id string) (*Commit, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	commit, ok := vs.commits[id]
	if !ok {
		return nil, fmt.Errorf("getting commit: commit %q not found", id)
	}
	return commit, nil
}

// GetHistory returns the commit history for a branch, up to limit entries.
func (vs *VersionStore) GetHistory(branch string, limit int) []*Commit {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	b, ok := vs.branches[branch]
	if !ok {
		return nil
	}

	var history []*Commit
	current := b.Head
	for current != "" && (limit <= 0 || len(history) < limit) {
		commit, ok := vs.commits[current]
		if !ok {
			break
		}
		history = append(history, commit)
		current = commit.ParentID
	}
	return history
}

// Diff returns the changes between two commits by collecting all changes
// in the commit chain from fromCommit to toCommit.
func (vs *VersionStore) Diff(fromCommit, toCommit string) ([]*Change, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if _, ok := vs.commits[fromCommit]; !ok {
		return nil, fmt.Errorf("computing diff: commit %q not found", fromCommit)
	}
	if _, ok := vs.commits[toCommit]; !ok {
		return nil, fmt.Errorf("computing diff: commit %q not found", toCommit)
	}

	// Collect commits from toCommit back to (but not including) fromCommit.
	var changes []*Change
	current := toCommit
	for current != "" && current != fromCommit {
		commit, ok := vs.commits[current]
		if !ok {
			break
		}
		changes = append(changes, commit.Changes...)
		current = commit.ParentID
	}
	return changes, nil
}

// CreateTag creates a named reference to a commit.
func (vs *VersionStore) CreateTag(name, commitID, message string) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if _, exists := vs.tags[name]; exists {
		return fmt.Errorf("creating tag: tag %q already exists", name)
	}
	if _, ok := vs.commits[commitID]; !ok {
		return fmt.Errorf("creating tag: commit %q not found", commitID)
	}

	vs.tags[name] = &Tag{
		Name:      name,
		CommitID:  commitID,
		Message:   message,
		CreatedAt: time.Now(),
	}
	return nil
}

// GetTag returns a tag by name.
func (vs *VersionStore) GetTag(name string) (*Tag, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	tag, ok := vs.tags[name]
	if !ok {
		return nil, fmt.Errorf("getting tag: tag %q not found", name)
	}
	return tag, nil
}

// ListTags returns all tags sorted by name.
func (vs *VersionStore) ListTags() []*Tag {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	tags := make([]*Tag, 0, len(vs.tags))
	for _, t := range vs.tags {
		tags = append(tags, t)
	}
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})
	return tags
}

// Rollback resets the active branch head to the specified commit.
func (vs *VersionStore) Rollback(ctx context.Context, commitID string) error {
	if ctx.Err() != nil {
		return fmt.Errorf("rolling back: %w", ctx.Err())
	}

	vs.mu.Lock()
	defer vs.mu.Unlock()

	commit, ok := vs.commits[commitID]
	if !ok {
		return fmt.Errorf("rolling back: commit %q not found", commitID)
	}
	if commit.Branch != vs.activeBranch {
		return fmt.Errorf("rolling back: commit %q does not belong to active branch %q", commitID, vs.activeBranch)
	}

	branch := vs.branches[vs.activeBranch]
	branch.Head = commitID
	branch.UpdatedAt = time.Now()
	return nil
}

// Merge merges the source branch into the target branch by creating a merge commit.
func (vs *VersionStore) Merge(ctx context.Context, sourceBranch, targetBranch, author string) (*Commit, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("merging branches: %w", ctx.Err())
	}

	vs.mu.Lock()
	defer vs.mu.Unlock()

	source, ok := vs.branches[sourceBranch]
	if !ok {
		return nil, fmt.Errorf("merging branches: source branch %q not found", sourceBranch)
	}
	target, ok := vs.branches[targetBranch]
	if !ok {
		return nil, fmt.Errorf("merging branches: target branch %q not found", targetBranch)
	}
	if source.Head == "" {
		return nil, fmt.Errorf("merging branches: source branch %q has no commits", sourceBranch)
	}

	// Check if the source head has already been merged into the target.
	if vs.alreadyMerged(sourceBranch, source.Head, target.Head) {
		return nil, fmt.Errorf("merging branches: no changes to merge from %q into %q", sourceBranch, targetBranch)
	}

	// Collect all changes from source branch since divergence from target.
	var mergeChanges []*Change
	current := source.Head
	for current != "" && current != target.Head {
		c, ok := vs.commits[current]
		if !ok {
			break
		}
		if !vs.isAncestor(c.ID, target.Head) {
			mergeChanges = append(mergeChanges, c.Changes...)
		}
		current = c.ParentID
	}

	if len(mergeChanges) == 0 {
		return nil, fmt.Errorf("merging branches: no changes to merge from %q into %q", sourceBranch, targetBranch)
	}

	commit := &Commit{
		ID:       uuid.New().String(),
		ParentID: target.Head,
		Branch:   targetBranch,
		Message:  fmt.Sprintf("Merge %s into %s", sourceBranch, targetBranch),
		Author:   author,
		Changes:  mergeChanges,
		Metadata: map[string]string{
			"merge_source":      sourceBranch,
			"merge_target":      targetBranch,
			"merge_source_head": source.Head,
		},
		CreatedAt: time.Now(),
	}

	vs.commits[commit.ID] = commit
	target.Head = commit.ID
	target.UpdatedAt = commit.CreatedAt
	return commit, nil
}

// alreadyMerged checks if sourceHead from sourceBranch was already merged into the
// target branch by looking for a merge commit in the target's history. Must be called with vs.mu held.
func (vs *VersionStore) alreadyMerged(sourceBranch, sourceHead, targetHead string) bool {
	current := targetHead
	for current != "" {
		c, ok := vs.commits[current]
		if !ok {
			break
		}
		if c.Metadata["merge_source"] == sourceBranch && c.Metadata["merge_source_head"] == sourceHead {
			return true
		}
		current = c.ParentID
	}
	return false
}

// isAncestor checks if commitID is reachable from headID by walking the parent chain.
// Must be called with vs.mu held.
func (vs *VersionStore) isAncestor(commitID, headID string) bool {
	current := headID
	for current != "" {
		if current == commitID {
			return true
		}
		c, ok := vs.commits[current]
		if !ok {
			break
		}
		current = c.ParentID
	}
	return false
}

// GetCurrentBranch returns the name of the active branch.
func (vs *VersionStore) GetCurrentBranch() string {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.activeBranch
}

// Stats returns summary statistics about the version store.
func (vs *VersionStore) Stats() *VersionStats {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	return &VersionStats{
		BranchCount:  len(vs.branches),
		CommitCount:  len(vs.commits),
		TagCount:     len(vs.tags),
		ActiveBranch: vs.activeBranch,
	}
}
