package gitops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SyncState represents the synchronization state.
type SyncState string

const (
	SyncStatePending    SyncState = "pending"
	SyncStateInProgress SyncState = "in_progress"
	SyncStateSuccess    SyncState = "success"
	SyncStateFailed     SyncState = "failed"
	SyncStateConflict   SyncState = "conflict"
)

// SyncMode determines how changes are applied.
type SyncMode string

const (
	SyncModeApply  SyncMode = "apply"   // Create or update resources
	SyncModeDelete SyncMode = "delete"  // Remove resources not in Git
	SyncModeDryRun SyncMode = "dry_run" // Report changes without applying
	SyncModeForce  SyncMode = "force"   // Apply even with conflicts
)

// SyncResult contains the result of a sync operation.
type SyncResult struct {
	State      SyncState         `json:"state"`
	StartTime  time.Time         `json:"startTime"`
	EndTime    time.Time         `json:"endTime"`
	Created    []string          `json:"created,omitempty"`
	Updated    []string          `json:"updated,omitempty"`
	Deleted    []string          `json:"deleted,omitempty"`
	Unchanged  []string          `json:"unchanged,omitempty"`
	Failed     []SyncError       `json:"failed,omitempty"`
	Violations []PolicyViolation `json:"violations,omitempty"`
	DryRun     bool              `json:"dryRun"`
	CommitHash string            `json:"commitHash,omitempty"`
	Message    string            `json:"message,omitempty"`
}

// SyncError represents a sync error for a specific resource.
type SyncError struct {
	Resource string `json:"resource"`
	Error    string `json:"error"`
}

// SyncConfig configures the sync behavior.
type SyncConfig struct {
	Mode            SyncMode          `json:"mode"`
	SourcePath      string            `json:"sourcePath"`
	FilePattern     string            `json:"filePattern"`
	Namespaces      []string          `json:"namespaces,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	PruneOrphans    bool              `json:"pruneOrphans"`
	EnforcePolicies bool              `json:"enforcePolicies"`
	ContinueOnError bool              `json:"continueOnError"`
}

// ResourceStore provides an interface for applying feature definitions.
type ResourceStore interface {
	Create(ctx context.Context, def *FeatureDefinition) error
	Update(ctx context.Context, def *FeatureDefinition) error
	Delete(ctx context.Context, namespace, name string) error
	Get(ctx context.Context, namespace, name string) (*FeatureDefinition, error)
	List(ctx context.Context, namespace string) ([]*FeatureDefinition, error)
}

// SyncManager manages synchronization between Git and the feature store.
type SyncManager struct {
	mu           sync.RWMutex
	loader       *SchemaLoader
	policyEngine *PolicyEngine
	store        ResourceStore
	stateFile    string
	syncHistory  []SyncResult
	maxHistory   int
}

// NewSyncManager creates a new sync manager.
func NewSyncManager(loader *SchemaLoader, policyEngine *PolicyEngine, store ResourceStore, stateFile string) *SyncManager {
	return &SyncManager{
		loader:       loader,
		policyEngine: policyEngine,
		store:        store,
		stateFile:    stateFile,
		maxHistory:   100,
	}
}

// Sync performs a synchronization operation.
func (m *SyncManager) Sync(ctx context.Context, config *SyncConfig) (*SyncResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &SyncResult{
		State:     SyncStateInProgress,
		StartTime: time.Now(),
		DryRun:    config.Mode == SyncModeDryRun,
	}

	// Load definitions from Git
	definitions, err := m.loader.LoadAllDefinitions(config.FilePattern)
	if err != nil {
		result.State = SyncStateFailed
		result.EndTime = time.Now()
		result.Message = fmt.Sprintf("Failed to load definitions: %v", err)
		m.recordResult(result)
		return result, err
	}

	// Filter by namespace and labels
	definitions = m.filterDefinitions(definitions, config)

	// Enforce policies if enabled
	if config.EnforcePolicies {
		for _, def := range definitions {
			policyResult := m.policyEngine.Evaluate(def)
			if !policyResult.Passed {
				result.Violations = append(result.Violations, policyResult.Violations...)
				if !config.ContinueOnError {
					result.State = SyncStateFailed
					result.EndTime = time.Now()
					result.Message = "Policy violations detected"
					m.recordResult(result)
					return result, fmt.Errorf("policy violations detected")
				}
			}
		}
	}

	// Apply changes
	for _, def := range definitions {
		// Calculate hash
		def.Status = &DefinitionStatus{
			ResourceHash: m.calculateHash(def),
		}

		existing, err := m.store.Get(ctx, def.Metadata.Namespace, def.Metadata.Name)
		if err != nil {
			// Assume not found, create
			if config.Mode != SyncModeDryRun {
				if err := m.store.Create(ctx, def); err != nil {
					result.Failed = append(result.Failed, SyncError{
						Resource: m.resourceKey(def),
						Error:    err.Error(),
					})
					if !config.ContinueOnError {
						result.State = SyncStateFailed
						result.EndTime = time.Now()
						m.recordResult(result)
						return result, err
					}
					continue
				}
			}
			result.Created = append(result.Created, m.resourceKey(def))
		} else {
			// Check if update needed
			if existing.Status != nil && existing.Status.ResourceHash == def.Status.ResourceHash {
				result.Unchanged = append(result.Unchanged, m.resourceKey(def))
				continue
			}

			if config.Mode != SyncModeDryRun {
				if err := m.store.Update(ctx, def); err != nil {
					result.Failed = append(result.Failed, SyncError{
						Resource: m.resourceKey(def),
						Error:    err.Error(),
					})
					if !config.ContinueOnError {
						result.State = SyncStateFailed
						result.EndTime = time.Now()
						m.recordResult(result)
						return result, err
					}
					continue
				}
			}
			result.Updated = append(result.Updated, m.resourceKey(def))
		}
	}

	// Prune orphans if enabled
	if config.PruneOrphans {
		orphans, err := m.findOrphans(ctx, definitions, config)
		if err != nil && !config.ContinueOnError {
			result.State = SyncStateFailed
			result.EndTime = time.Now()
			result.Message = fmt.Sprintf("Failed to find orphans: %v", err)
			m.recordResult(result)
			return result, err
		}

		for _, orphan := range orphans {
			if config.Mode != SyncModeDryRun {
				if err := m.store.Delete(ctx, orphan.Metadata.Namespace, orphan.Metadata.Name); err != nil {
					result.Failed = append(result.Failed, SyncError{
						Resource: m.resourceKey(orphan),
						Error:    err.Error(),
					})
					continue
				}
			}
			result.Deleted = append(result.Deleted, m.resourceKey(orphan))
		}
	}

	// Determine final state
	if len(result.Failed) > 0 {
		result.State = SyncStateFailed
	} else if len(result.Violations) > 0 {
		result.State = SyncStateConflict
	} else {
		result.State = SyncStateSuccess
	}

	result.EndTime = time.Now()
	m.recordResult(result)

	// Save state file
	if m.stateFile != "" && config.Mode != SyncModeDryRun {
		if err := m.saveState(result); err != nil {
			// Log but don't fail the sync
			result.Message = fmt.Sprintf("Sync completed but failed to save state: %v", err)
		}
	}

	return result, nil
}

// filterDefinitions filters definitions by namespace and labels.
func (m *SyncManager) filterDefinitions(defs []*FeatureDefinition, config *SyncConfig) []*FeatureDefinition {
	var filtered []*FeatureDefinition

	for _, def := range defs {
		// Filter by namespace
		if len(config.Namespaces) > 0 {
			found := false
			for _, ns := range config.Namespaces {
				if def.Metadata.Namespace == ns {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by labels
		if len(config.Labels) > 0 {
			match := true
			for key, value := range config.Labels {
				if def.Metadata.Labels[key] != value {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		filtered = append(filtered, def)
	}

	return filtered
}

// findOrphans finds resources in the store that are not in Git.
func (m *SyncManager) findOrphans(ctx context.Context, gitDefs []*FeatureDefinition, config *SyncConfig) ([]*FeatureDefinition, error) {
	var orphans []*FeatureDefinition

	// Build a set of Git resources
	gitSet := make(map[string]bool)
	for _, def := range gitDefs {
		gitSet[m.resourceKey(def)] = true
	}

	// Get resources from each namespace
	namespaces := config.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{""}
	}

	for _, ns := range namespaces {
		storeDefs, err := m.store.List(ctx, ns)
		if err != nil {
			return nil, err
		}

		for _, def := range storeDefs {
			// Skip if labels don't match
			if len(config.Labels) > 0 {
				match := true
				for key, value := range config.Labels {
					if def.Metadata.Labels[key] != value {
						match = false
						break
					}
				}
				if !match {
					continue
				}
			}

			// Check if in Git
			if !gitSet[m.resourceKey(def)] {
				orphans = append(orphans, def)
			}
		}
	}

	return orphans, nil
}

// resourceKey returns a unique key for a resource.
func (m *SyncManager) resourceKey(def *FeatureDefinition) string {
	if def.Metadata.Namespace != "" {
		return def.Metadata.Namespace + "/" + def.Metadata.Name
	}
	return def.Metadata.Name
}

// calculateHash calculates a hash of a definition for change detection.
func (m *SyncManager) calculateHash(def *FeatureDefinition) string {
	// Create a copy without status for hashing
	copy := *def
	copy.Status = nil

	data, _ := json.Marshal(copy)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// recordResult records a sync result in history.
func (m *SyncManager) recordResult(result *SyncResult) {
	m.syncHistory = append(m.syncHistory, *result)
	if len(m.syncHistory) > m.maxHistory {
		m.syncHistory = m.syncHistory[1:]
	}
}

// GetHistory returns the sync history.
func (m *SyncManager) GetHistory() []SyncResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]SyncResult, len(m.syncHistory))
	copy(history, m.syncHistory)
	return history
}

// GetLastResult returns the last sync result.
func (m *SyncManager) GetLastResult() *SyncResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.syncHistory) == 0 {
		return nil
	}
	return &m.syncHistory[len(m.syncHistory)-1]
}

// saveState saves the sync state to a file.
func (m *SyncManager) saveState(result *SyncResult) error {
	dir := filepath.Dir(m.stateFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.stateFile, data, 0644)
}

// LoadState loads the sync state from a file.
func (m *SyncManager) LoadState() (*SyncResult, error) {
	if m.stateFile == "" {
		return nil, nil
	}

	data, err := os.ReadFile(m.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var result SyncResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DiffReport represents differences between Git and the store.
type DiffReport struct {
	ToCreate  []string  `json:"toCreate"`
	ToUpdate  []string  `json:"toUpdate"`
	ToDelete  []string  `json:"toDelete"`
	Unchanged []string  `json:"unchanged"`
	Timestamp time.Time `json:"timestamp"`
}

// Diff computes differences between Git and the store without applying changes.
func (m *SyncManager) Diff(ctx context.Context, config *SyncConfig) (*DiffReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &DiffReport{
		Timestamp: time.Now(),
	}

	// Load definitions from Git
	definitions, err := m.loader.LoadAllDefinitions(config.FilePattern)
	if err != nil {
		return nil, err
	}

	definitions = m.filterDefinitions(definitions, config)

	// Compare with store
	for _, def := range definitions {
		hash := m.calculateHash(def)
		existing, err := m.store.Get(ctx, def.Metadata.Namespace, def.Metadata.Name)
		if err != nil {
			report.ToCreate = append(report.ToCreate, m.resourceKey(def))
		} else if existing.Status != nil && existing.Status.ResourceHash == hash {
			report.Unchanged = append(report.Unchanged, m.resourceKey(def))
		} else {
			report.ToUpdate = append(report.ToUpdate, m.resourceKey(def))
		}
	}

	// Find orphans
	if config.PruneOrphans {
		orphans, err := m.findOrphans(ctx, definitions, config)
		if err != nil {
			return nil, err
		}
		for _, orphan := range orphans {
			report.ToDelete = append(report.ToDelete, m.resourceKey(orphan))
		}
	}

	return report, nil
}

// Validate validates definitions without applying them.
func (m *SyncManager) Validate(config *SyncConfig) ([]PolicyViolation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Load definitions
	definitions, err := m.loader.LoadAllDefinitions(config.FilePattern)
	if err != nil {
		return nil, err
	}

	definitions = m.filterDefinitions(definitions, config)

	var violations []PolicyViolation
	for _, def := range definitions {
		result := m.policyEngine.Evaluate(def)
		violations = append(violations, result.Violations...)
		violations = append(violations, result.Warnings...)
	}

	return violations, nil
}
