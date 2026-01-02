package gitops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// mockResourceStore is a mock implementation of ResourceStore for testing.
type mockResourceStore struct {
	mu        sync.RWMutex
	resources map[string]*FeatureDefinition
	createErr error
	updateErr error
	deleteErr error
	getErr    error
}

func newMockResourceStore() *mockResourceStore {
	return &mockResourceStore{
		resources: make(map[string]*FeatureDefinition),
	}
}

func (m *mockResourceStore) key(namespace, name string) string {
	if namespace != "" {
		return namespace + "/" + name
	}
	return name
}

func (m *mockResourceStore) Create(ctx context.Context, def *FeatureDefinition) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resources[m.key(def.Metadata.Namespace, def.Metadata.Name)] = def
	return nil
}

func (m *mockResourceStore) Update(ctx context.Context, def *FeatureDefinition) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resources[m.key(def.Metadata.Namespace, def.Metadata.Name)] = def
	return nil
}

func (m *mockResourceStore) Delete(ctx context.Context, namespace, name string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.resources, m.key(namespace, name))
	return nil
}

func (m *mockResourceStore) Get(ctx context.Context, namespace, name string) (*FeatureDefinition, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	def, exists := m.resources[m.key(namespace, name)]
	if !exists {
		return nil, fmt.Errorf("not found")
	}
	return def, nil
}

func (m *mockResourceStore) List(ctx context.Context, namespace string) ([]*FeatureDefinition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*FeatureDefinition
	for _, def := range m.resources {
		if namespace == "" || def.Metadata.Namespace == namespace {
			result = append(result, def)
		}
	}
	return result, nil
}

func createTestDefinition(tmpDir, name string) error {
	content := fmt.Sprintf(`
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: %s
spec:
  entityType: user
  features:
    - name: age
      dataType: int64
`, name)
	return os.WriteFile(filepath.Join(tmpDir, name+".yaml"), []byte(content), 0644)
}

func TestSyncManager_Sync_Create(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, ".sync-state.json")

	loader := NewSchemaLoader(tmpDir)
	engine := NewPolicyEngine()
	store := newMockResourceStore()
	manager := NewSyncManager(loader, engine, store, stateFile)

	// Create test definitions
	if err := createTestDefinition(tmpDir, "feature1"); err != nil {
		t.Fatalf("Failed to create test definition: %v", err)
	}
	if err := createTestDefinition(tmpDir, "feature2"); err != nil {
		t.Fatalf("Failed to create test definition: %v", err)
	}

	config := &SyncConfig{
		Mode:        SyncModeApply,
		FilePattern: "*.yaml",
	}

	result, err := manager.Sync(context.Background(), config)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.State != SyncStateSuccess {
		t.Errorf("Expected state Success, got %s", result.State)
	}
	if len(result.Created) != 2 {
		t.Errorf("Expected 2 created, got %d", len(result.Created))
	}
	if len(store.resources) != 2 {
		t.Errorf("Expected 2 resources in store, got %d", len(store.resources))
	}
}

func TestSyncManager_Sync_Update(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)
	engine := NewPolicyEngine()
	store := newMockResourceStore()
	manager := NewSyncManager(loader, engine, store, "")

	// Pre-populate store with existing resource (different hash)
	existing := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "feature1"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "old", DataType: "string"}}},
		Status:     &DefinitionStatus{ResourceHash: "old-hash"},
	}
	store.Create(context.Background(), existing)

	// Create new definition
	if err := createTestDefinition(tmpDir, "feature1"); err != nil {
		t.Fatalf("Failed to create test definition: %v", err)
	}

	config := &SyncConfig{
		Mode:        SyncModeApply,
		FilePattern: "*.yaml",
	}

	result, err := manager.Sync(context.Background(), config)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.State != SyncStateSuccess {
		t.Errorf("Expected state Success, got %s", result.State)
	}
	if len(result.Updated) != 1 {
		t.Errorf("Expected 1 updated, got %d", len(result.Updated))
	}
}

func TestSyncManager_Sync_Unchanged(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)
	engine := NewPolicyEngine()
	store := newMockResourceStore()
	manager := NewSyncManager(loader, engine, store, "")

	// Create definition file
	if err := createTestDefinition(tmpDir, "feature1"); err != nil {
		t.Fatalf("Failed to create test definition: %v", err)
	}

	// First sync to create
	config := &SyncConfig{
		Mode:        SyncModeApply,
		FilePattern: "*.yaml",
	}

	_, err := manager.Sync(context.Background(), config)
	if err != nil {
		t.Fatalf("First sync failed: %v", err)
	}

	// Second sync should be unchanged
	result, err := manager.Sync(context.Background(), config)
	if err != nil {
		t.Fatalf("Second sync failed: %v", err)
	}

	if result.State != SyncStateSuccess {
		t.Errorf("Expected state Success, got %s", result.State)
	}
	if len(result.Unchanged) != 1 {
		t.Errorf("Expected 1 unchanged, got %d", len(result.Unchanged))
	}
	if len(result.Created) != 0 {
		t.Errorf("Expected 0 created, got %d", len(result.Created))
	}
	if len(result.Updated) != 0 {
		t.Errorf("Expected 0 updated, got %d", len(result.Updated))
	}
}

func TestSyncManager_Sync_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)
	engine := NewPolicyEngine()
	store := newMockResourceStore()
	manager := NewSyncManager(loader, engine, store, "")

	if err := createTestDefinition(tmpDir, "feature1"); err != nil {
		t.Fatalf("Failed to create test definition: %v", err)
	}

	config := &SyncConfig{
		Mode:        SyncModeDryRun,
		FilePattern: "*.yaml",
	}

	result, err := manager.Sync(context.Background(), config)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if !result.DryRun {
		t.Error("Expected DryRun to be true")
	}
	if len(result.Created) != 1 {
		t.Errorf("Expected 1 to be created, got %d", len(result.Created))
	}
	// Verify nothing was actually created
	if len(store.resources) != 0 {
		t.Error("Expected store to be empty in dry run")
	}
}

func TestSyncManager_Sync_PruneOrphans(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)
	engine := NewPolicyEngine()
	store := newMockResourceStore()
	manager := NewSyncManager(loader, engine, store, "")

	// Pre-populate store with orphan
	orphan := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "orphan"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "x", DataType: "int64"}}},
	}
	store.Create(context.Background(), orphan)

	// Create only one definition in Git
	if err := createTestDefinition(tmpDir, "feature1"); err != nil {
		t.Fatalf("Failed to create test definition: %v", err)
	}

	config := &SyncConfig{
		Mode:         SyncModeApply,
		FilePattern:  "*.yaml",
		PruneOrphans: true,
	}

	result, err := manager.Sync(context.Background(), config)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if len(result.Deleted) != 1 {
		t.Errorf("Expected 1 deleted, got %d", len(result.Deleted))
	}
	if len(store.resources) != 1 {
		t.Errorf("Expected 1 resource in store (orphan deleted), got %d", len(store.resources))
	}
}

func TestSyncManager_Sync_WithPolicies(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)
	engine := NewPolicyEngine()
	store := newMockResourceStore()
	manager := NewSyncManager(loader, engine, store, "")

	// Register a policy
	policy := &Policy{
		Metadata: PolicyMeta{Name: "require-owner", Severity: "error"},
		Spec: PolicySpec{
			Rules: []PolicyRule{
				{Name: "owner", Type: "require", Field: "metadata.owner"},
			},
		},
	}
	engine.RegisterPolicy(policy)

	// Create definition without owner (will violate policy)
	if err := createTestDefinition(tmpDir, "feature1"); err != nil {
		t.Fatalf("Failed to create test definition: %v", err)
	}

	config := &SyncConfig{
		Mode:            SyncModeApply,
		FilePattern:     "*.yaml",
		EnforcePolicies: true,
	}

	result, err := manager.Sync(context.Background(), config)
	if err == nil {
		t.Error("Expected sync to fail due to policy violation")
	}

	if result.State != SyncStateFailed {
		t.Errorf("Expected state Failed, got %s", result.State)
	}
	if len(result.Violations) != 1 {
		t.Errorf("Expected 1 violation, got %d", len(result.Violations))
	}
}

func TestSyncManager_Sync_ContinueOnError(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)
	engine := NewPolicyEngine()
	store := newMockResourceStore()
	manager := NewSyncManager(loader, engine, store, "")

	// Make first create fail
	store.createErr = fmt.Errorf("create failed")

	if err := createTestDefinition(tmpDir, "feature1"); err != nil {
		t.Fatalf("Failed to create test definition: %v", err)
	}

	config := &SyncConfig{
		Mode:            SyncModeApply,
		FilePattern:     "*.yaml",
		ContinueOnError: true,
	}

	result, err := manager.Sync(context.Background(), config)
	if err != nil {
		t.Fatalf("Expected no error with ContinueOnError, got: %v", err)
	}

	if result.State != SyncStateFailed {
		t.Errorf("Expected state Failed, got %s", result.State)
	}
	if len(result.Failed) != 1 {
		t.Errorf("Expected 1 failed, got %d", len(result.Failed))
	}
}

func TestSyncManager_Sync_NamespaceFilter(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)
	engine := NewPolicyEngine()
	store := newMockResourceStore()
	manager := NewSyncManager(loader, engine, store, "")

	// Create definitions with different namespaces
	content1 := `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: prod_feature
  namespace: production
spec:
  entityType: user
  features:
    - name: age
      dataType: int64
`
	content2 := `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: dev_feature
  namespace: development
spec:
  entityType: user
  features:
    - name: age
      dataType: int64
`

	os.WriteFile(filepath.Join(tmpDir, "prod.yaml"), []byte(content1), 0644)
	os.WriteFile(filepath.Join(tmpDir, "dev.yaml"), []byte(content2), 0644)

	config := &SyncConfig{
		Mode:        SyncModeApply,
		FilePattern: "*.yaml",
		Namespaces:  []string{"production"},
	}

	result, err := manager.Sync(context.Background(), config)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if len(result.Created) != 1 {
		t.Errorf("Expected 1 created (filtered by namespace), got %d", len(result.Created))
	}
}

func TestSyncManager_Sync_LabelFilter(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)
	engine := NewPolicyEngine()
	store := newMockResourceStore()
	manager := NewSyncManager(loader, engine, store, "")

	// Create definitions with different labels
	content1 := `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: ml_feature
  labels:
    team: ml
spec:
  entityType: user
  features:
    - name: age
      dataType: int64
`
	content2 := `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: analytics_feature
  labels:
    team: analytics
spec:
  entityType: user
  features:
    - name: age
      dataType: int64
`

	os.WriteFile(filepath.Join(tmpDir, "ml.yaml"), []byte(content1), 0644)
	os.WriteFile(filepath.Join(tmpDir, "analytics.yaml"), []byte(content2), 0644)

	config := &SyncConfig{
		Mode:        SyncModeApply,
		FilePattern: "*.yaml",
		Labels:      map[string]string{"team": "ml"},
	}

	result, err := manager.Sync(context.Background(), config)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if len(result.Created) != 1 {
		t.Errorf("Expected 1 created (filtered by label), got %d", len(result.Created))
	}
}

func TestSyncManager_Diff(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)
	engine := NewPolicyEngine()
	store := newMockResourceStore()
	manager := NewSyncManager(loader, engine, store, "")

	// Pre-populate store
	existing := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "existing"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "x", DataType: "int64"}}},
		Status:     &DefinitionStatus{ResourceHash: "old-hash"},
	}
	store.Create(context.Background(), existing)

	orphan := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "orphan"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "x", DataType: "int64"}}},
	}
	store.Create(context.Background(), orphan)

	// Create Git definitions
	if err := createTestDefinition(tmpDir, "existing"); err != nil {
		t.Fatalf("Failed to create test definition: %v", err)
	}
	if err := createTestDefinition(tmpDir, "new_feature"); err != nil {
		t.Fatalf("Failed to create test definition: %v", err)
	}

	config := &SyncConfig{
		FilePattern:  "*.yaml",
		PruneOrphans: true,
	}

	report, err := manager.Diff(context.Background(), config)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if len(report.ToCreate) != 1 {
		t.Errorf("Expected 1 to create, got %d", len(report.ToCreate))
	}
	if len(report.ToUpdate) != 1 {
		t.Errorf("Expected 1 to update, got %d", len(report.ToUpdate))
	}
	if len(report.ToDelete) != 1 {
		t.Errorf("Expected 1 to delete, got %d", len(report.ToDelete))
	}
}

func TestSyncManager_Validate(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)
	engine := NewPolicyEngine()
	store := newMockResourceStore()
	manager := NewSyncManager(loader, engine, store, "")

	// Register policies
	engine.RegisterPolicy(&Policy{
		Metadata: PolicyMeta{Name: "require-owner", Severity: "error"},
		Spec: PolicySpec{
			Rules: []PolicyRule{
				{Name: "owner", Type: "require", Field: "metadata.owner"},
			},
		},
	})

	if err := createTestDefinition(tmpDir, "feature1"); err != nil {
		t.Fatalf("Failed to create test definition: %v", err)
	}

	config := &SyncConfig{
		FilePattern: "*.yaml",
	}

	violations, err := manager.Validate(config)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if len(violations) != 1 {
		t.Errorf("Expected 1 violation, got %d", len(violations))
	}
}

func TestSyncManager_GetHistory(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)
	engine := NewPolicyEngine()
	store := newMockResourceStore()
	manager := NewSyncManager(loader, engine, store, "")

	if err := createTestDefinition(tmpDir, "feature1"); err != nil {
		t.Fatalf("Failed to create test definition: %v", err)
	}

	config := &SyncConfig{
		Mode:        SyncModeApply,
		FilePattern: "*.yaml",
	}

	// Perform multiple syncs
	manager.Sync(context.Background(), config)
	manager.Sync(context.Background(), config)
	manager.Sync(context.Background(), config)

	history := manager.GetHistory()
	if len(history) != 3 {
		t.Errorf("Expected 3 history entries, got %d", len(history))
	}
}

func TestSyncManager_GetLastResult(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSchemaLoader(tmpDir)
	engine := NewPolicyEngine()
	store := newMockResourceStore()
	manager := NewSyncManager(loader, engine, store, "")

	// No history yet
	lastResult := manager.GetLastResult()
	if lastResult != nil {
		t.Error("Expected nil result before any sync")
	}

	if err := createTestDefinition(tmpDir, "feature1"); err != nil {
		t.Fatalf("Failed to create test definition: %v", err)
	}

	config := &SyncConfig{
		Mode:        SyncModeApply,
		FilePattern: "*.yaml",
	}

	manager.Sync(context.Background(), config)

	lastResult = manager.GetLastResult()
	if lastResult == nil {
		t.Error("Expected non-nil result after sync")
	}
	if lastResult.State != SyncStateSuccess {
		t.Errorf("Expected state Success, got %s", lastResult.State)
	}
}

func TestSyncManager_SaveAndLoadState(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, ".sync-state.json")

	loader := NewSchemaLoader(tmpDir)
	engine := NewPolicyEngine()
	store := newMockResourceStore()
	manager := NewSyncManager(loader, engine, store, stateFile)

	if err := createTestDefinition(tmpDir, "feature1"); err != nil {
		t.Fatalf("Failed to create test definition: %v", err)
	}

	config := &SyncConfig{
		Mode:        SyncModeApply,
		FilePattern: "*.yaml",
	}

	manager.Sync(context.Background(), config)

	// Load state
	state, err := manager.LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if state == nil {
		t.Fatal("Expected non-nil state")
	}
	if state.State != SyncStateSuccess {
		t.Errorf("Expected state Success, got %s", state.State)
	}
}

func TestSyncManager_LoadState_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "nonexistent.json")

	loader := NewSchemaLoader(tmpDir)
	engine := NewPolicyEngine()
	store := newMockResourceStore()
	manager := NewSyncManager(loader, engine, store, stateFile)

	state, err := manager.LoadState()
	if err != nil {
		t.Fatalf("LoadState should not error on non-existent file: %v", err)
	}
	if state != nil {
		t.Error("Expected nil state for non-existent file")
	}
}

func TestSyncManager_CalculateHash(t *testing.T) {
	loader := NewSchemaLoader("")
	engine := NewPolicyEngine()
	store := newMockResourceStore()
	manager := NewSyncManager(loader, engine, store, "")

	def1 := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "test"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	def2 := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "test"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "age", DataType: "int64"}}},
	}

	def3 := &FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   DefinitionMeta{Name: "test"},
		Spec:       FeatureSpec{EntityType: "user", Features: []FeatureField{{Name: "name", DataType: "string"}}},
	}

	hash1 := manager.calculateHash(def1)
	hash2 := manager.calculateHash(def2)
	hash3 := manager.calculateHash(def3)

	if hash1 != hash2 {
		t.Error("Expected identical definitions to have same hash")
	}
	if hash1 == hash3 {
		t.Error("Expected different definitions to have different hash")
	}
}

func TestSyncResult_Fields(t *testing.T) {
	result := SyncResult{
		State:      SyncStateSuccess,
		StartTime:  time.Now().Add(-time.Minute),
		EndTime:    time.Now(),
		Created:    []string{"feature1"},
		Updated:    []string{"feature2"},
		Deleted:    []string{"feature3"},
		Unchanged:  []string{"feature4"},
		DryRun:     false,
		CommitHash: "abc123",
		Message:    "Sync completed",
	}

	if result.State != SyncStateSuccess {
		t.Errorf("Expected state Success, got %s", result.State)
	}
	if len(result.Created) != 1 {
		t.Errorf("Expected 1 created, got %d", len(result.Created))
	}
}

func TestDiffReport_Fields(t *testing.T) {
	report := DiffReport{
		ToCreate:  []string{"new"},
		ToUpdate:  []string{"existing"},
		ToDelete:  []string{"orphan"},
		Unchanged: []string{"same"},
		Timestamp: time.Now(),
	}

	if len(report.ToCreate) != 1 {
		t.Errorf("Expected 1 to create, got %d", len(report.ToCreate))
	}
	if len(report.ToUpdate) != 1 {
		t.Errorf("Expected 1 to update, got %d", len(report.ToUpdate))
	}
	if len(report.ToDelete) != 1 {
		t.Errorf("Expected 1 to delete, got %d", len(report.ToDelete))
	}
}

func TestSyncConfig_Fields(t *testing.T) {
	config := SyncConfig{
		Mode:            SyncModeApply,
		SourcePath:      "/path/to/source",
		FilePattern:     "**/*.yaml",
		Namespaces:      []string{"prod", "staging"},
		Labels:          map[string]string{"env": "prod"},
		PruneOrphans:    true,
		EnforcePolicies: true,
		ContinueOnError: false,
	}

	if config.Mode != SyncModeApply {
		t.Errorf("Expected mode Apply, got %s", config.Mode)
	}
	if !config.PruneOrphans {
		t.Error("Expected PruneOrphans to be true")
	}
}
