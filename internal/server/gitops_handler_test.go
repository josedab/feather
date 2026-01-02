package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/feather-store/feather/internal/gitops"
)

// mockStore implements gitops.ResourceStore for testing.
type mockStore struct {
	mu        sync.RWMutex
	resources map[string]*gitops.FeatureDefinition
}

func newMockStore() *mockStore {
	return &mockStore{
		resources: make(map[string]*gitops.FeatureDefinition),
	}
}

func (m *mockStore) key(namespace, name string) string {
	if namespace != "" {
		return namespace + "/" + name
	}
	return name
}

func (m *mockStore) Create(ctx context.Context, def *gitops.FeatureDefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resources[m.key(def.Metadata.Namespace, def.Metadata.Name)] = def
	return nil
}

func (m *mockStore) Update(ctx context.Context, def *gitops.FeatureDefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resources[m.key(def.Metadata.Namespace, def.Metadata.Name)] = def
	return nil
}

func (m *mockStore) Delete(ctx context.Context, namespace, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.resources, m.key(namespace, name))
	return nil
}

func (m *mockStore) Get(ctx context.Context, namespace, name string) (*gitops.FeatureDefinition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	def, exists := m.resources[m.key(namespace, name)]
	if !exists {
		return nil, fmt.Errorf("not found")
	}
	return def, nil
}

func (m *mockStore) List(ctx context.Context, namespace string) ([]*gitops.FeatureDefinition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*gitops.FeatureDefinition
	for _, def := range m.resources {
		if namespace == "" || def.Metadata.Namespace == namespace {
			result = append(result, def)
		}
	}
	return result, nil
}

func setupGitOpsHandler(t *testing.T) (*GitOpsHandler, string) {
	tmpDir := t.TempDir()
	loader := gitops.NewSchemaLoader(tmpDir)
	engine := gitops.NewPolicyEngine()
	store := newMockStore()
	manager := gitops.NewSyncManager(loader, engine, store, "")
	handler := NewGitOpsHandler(loader, engine, manager)
	return handler, tmpDir
}

func TestGitOpsHandler_ListPolicies(t *testing.T) {
	handler, _ := setupGitOpsHandler(t)

	// Register some policies
	policies := gitops.CreateStandardPolicies()
	for _, p := range policies {
		handler.policyEngine.RegisterPolicy(p)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/gitops/policies", nil)
	w := httptest.NewRecorder()
	handler.handleListPolicies(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)

	count := int(result["count"].(float64))
	if count != len(policies) {
		t.Errorf("Expected count %d, got %d", len(policies), count)
	}
}

func TestGitOpsHandler_CreatePolicy(t *testing.T) {
	handler, _ := setupGitOpsHandler(t)

	policy := gitops.Policy{
		APIVersion: "feather.io/v1",
		Kind:       "Policy",
		Metadata: gitops.PolicyMeta{
			Name:     "test-policy",
			Severity: "error",
		},
		Spec: gitops.PolicySpec{
			Rules: []gitops.PolicyRule{
				{Name: "test-rule", Type: "require", Field: "metadata.owner"},
			},
		},
	}

	body, _ := json.Marshal(policy)
	req := httptest.NewRequest(http.MethodPost, "/v1/gitops/policies", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.handleCreatePolicy(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify policy was registered
	_, exists := handler.policyEngine.GetPolicy("test-policy")
	if !exists {
		t.Error("Expected policy to be registered")
	}
}

func TestGitOpsHandler_CreatePolicy_Invalid(t *testing.T) {
	handler, _ := setupGitOpsHandler(t)

	// Missing rules
	policy := gitops.Policy{
		Metadata: gitops.PolicyMeta{Name: "test"},
		Spec:     gitops.PolicySpec{Rules: []gitops.PolicyRule{}},
	}

	body, _ := json.Marshal(policy)
	req := httptest.NewRequest(http.MethodPost, "/v1/gitops/policies", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.handleCreatePolicy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGitOpsHandler_GetPolicy(t *testing.T) {
	handler, _ := setupGitOpsHandler(t)

	// Register a policy
	policy := &gitops.Policy{
		Metadata: gitops.PolicyMeta{Name: "test-policy"},
		Spec: gitops.PolicySpec{
			Rules: []gitops.PolicyRule{{Name: "rule", Type: "require"}},
		},
	}
	handler.policyEngine.RegisterPolicy(policy)

	req := httptest.NewRequest(http.MethodGet, "/v1/gitops/policies/test-policy", nil)
	req.SetPathValue("name", "test-policy")
	w := httptest.NewRecorder()
	handler.handleGetPolicy(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGitOpsHandler_GetPolicy_NotFound(t *testing.T) {
	handler, _ := setupGitOpsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/gitops/policies/nonexistent", nil)
	req.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()
	handler.handleGetPolicy(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGitOpsHandler_DeletePolicy(t *testing.T) {
	handler, _ := setupGitOpsHandler(t)

	// Register a policy
	policy := &gitops.Policy{
		Metadata: gitops.PolicyMeta{Name: "test-policy"},
		Spec: gitops.PolicySpec{
			Rules: []gitops.PolicyRule{{Name: "rule", Type: "require"}},
		},
	}
	handler.policyEngine.RegisterPolicy(policy)

	req := httptest.NewRequest(http.MethodDelete, "/v1/gitops/policies/test-policy", nil)
	req.SetPathValue("name", "test-policy")
	w := httptest.NewRecorder()
	handler.handleDeletePolicy(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}

	// Verify policy was removed
	_, exists := handler.policyEngine.GetPolicy("test-policy")
	if exists {
		t.Error("Expected policy to be removed")
	}
}

func TestGitOpsHandler_DeletePolicy_NotFound(t *testing.T) {
	handler, _ := setupGitOpsHandler(t)

	req := httptest.NewRequest(http.MethodDelete, "/v1/gitops/policies/nonexistent", nil)
	req.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()
	handler.handleDeletePolicy(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGitOpsHandler_Sync(t *testing.T) {
	handler, tmpDir := setupGitOpsHandler(t)

	// Create a definition file
	content := `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: test_features
spec:
  entityType: user
  features:
    - name: age
      dataType: int64
`
	os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(content), 0644)

	syncReq := GitOpsSyncRequest{
		FilePattern: "*.yaml",
		Mode:        "apply",
	}

	body, _ := json.Marshal(syncReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/gitops/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.handleSync(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var result gitops.SyncResult
	json.Unmarshal(w.Body.Bytes(), &result)

	if result.State != gitops.SyncStateSuccess {
		t.Errorf("Expected state Success, got %s", result.State)
	}
	if len(result.Created) != 1 {
		t.Errorf("Expected 1 created, got %d", len(result.Created))
	}
}

func TestGitOpsHandler_Sync_DryRun(t *testing.T) {
	handler, tmpDir := setupGitOpsHandler(t)

	// Create a definition file
	content := `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: test_features
spec:
  entityType: user
  features:
    - name: age
      dataType: int64
`
	os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(content), 0644)

	syncReq := GitOpsSyncRequest{
		FilePattern: "*.yaml",
		Mode:        "dry_run",
	}

	body, _ := json.Marshal(syncReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/gitops/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.handleSync(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result gitops.SyncResult
	json.Unmarshal(w.Body.Bytes(), &result)

	if !result.DryRun {
		t.Error("Expected DryRun to be true")
	}
}

func TestGitOpsHandler_Sync_MissingPattern(t *testing.T) {
	handler, _ := setupGitOpsHandler(t)

	syncReq := GitOpsSyncRequest{
		Mode: "apply",
	}

	body, _ := json.Marshal(syncReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/gitops/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.handleSync(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGitOpsHandler_Diff(t *testing.T) {
	handler, tmpDir := setupGitOpsHandler(t)

	// Create a definition file
	content := `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: test_features
spec:
  entityType: user
  features:
    - name: age
      dataType: int64
`
	os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(content), 0644)

	diffReq := GitOpsSyncRequest{
		FilePattern: "*.yaml",
	}

	body, _ := json.Marshal(diffReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/gitops/diff", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.handleDiff(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var report gitops.DiffReport
	json.Unmarshal(w.Body.Bytes(), &report)

	if len(report.ToCreate) != 1 {
		t.Errorf("Expected 1 to create, got %d", len(report.ToCreate))
	}
}

func TestGitOpsHandler_Validate(t *testing.T) {
	handler, tmpDir := setupGitOpsHandler(t)

	// Register a policy
	handler.policyEngine.RegisterPolicy(&gitops.Policy{
		Metadata: gitops.PolicyMeta{Name: "require-owner", Severity: "error"},
		Spec: gitops.PolicySpec{
			Rules: []gitops.PolicyRule{
				{Name: "owner", Type: "require", Field: "metadata.owner"},
			},
		},
	})

	// Create a definition file without owner
	content := `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: test_features
spec:
  entityType: user
  features:
    - name: age
      dataType: int64
`
	os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(content), 0644)

	validateReq := GitOpsSyncRequest{
		FilePattern: "*.yaml",
	}

	body, _ := json.Marshal(validateReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/gitops/validate", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.handleValidate(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)

	if result["valid"].(bool) {
		t.Error("Expected valid to be false")
	}
	if result["count"].(float64) != 1 {
		t.Errorf("Expected 1 violation, got %v", result["count"])
	}
}

func TestGitOpsHandler_GetHistory(t *testing.T) {
	handler, tmpDir := setupGitOpsHandler(t)

	// Create a definition file and sync
	content := `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: test_features
spec:
  entityType: user
  features:
    - name: age
      dataType: int64
`
	os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(content), 0644)

	syncReq := GitOpsSyncRequest{
		FilePattern: "*.yaml",
		Mode:        "apply",
	}
	body, _ := json.Marshal(syncReq)
	syncReqHTTP := httptest.NewRequest(http.MethodPost, "/v1/gitops/sync", bytes.NewReader(body))
	syncW := httptest.NewRecorder()
	handler.handleSync(syncW, syncReqHTTP)

	// Get history
	req := httptest.NewRequest(http.MethodGet, "/v1/gitops/history", nil)
	w := httptest.NewRecorder()
	handler.handleGetHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)

	if result["count"].(float64) < 1 {
		t.Error("Expected at least 1 history entry")
	}
}

func TestGitOpsHandler_GetLatestResult(t *testing.T) {
	handler, _ := setupGitOpsHandler(t)

	// No history yet
	req := httptest.NewRequest(http.MethodGet, "/v1/gitops/history/latest", nil)
	w := httptest.NewRecorder()
	handler.handleGetLatestResult(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGitOpsHandler_GetDefinition(t *testing.T) {
	handler, tmpDir := setupGitOpsHandler(t)

	// Create a definition file
	content := `
apiVersion: feather.io/v1
kind: FeatureGroup
metadata:
  name: test_features
spec:
  entityType: user
  features:
    - name: age
      dataType: int64
`
	os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(content), 0644)

	req := httptest.NewRequest(http.MethodGet, "/v1/gitops/definitions/test.yaml", nil)
	req.SetPathValue("path", "test.yaml")
	w := httptest.NewRecorder()
	handler.handleGetDefinition(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var def gitops.FeatureDefinition
	json.Unmarshal(w.Body.Bytes(), &def)

	if def.Metadata.Name != "test_features" {
		t.Errorf("Expected name 'test_features', got '%s'", def.Metadata.Name)
	}
}

func TestGitOpsHandler_GetDefinition_NotFound(t *testing.T) {
	handler, _ := setupGitOpsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/gitops/definitions/nonexistent.yaml", nil)
	req.SetPathValue("path", "nonexistent.yaml")
	w := httptest.NewRecorder()
	handler.handleGetDefinition(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGitOpsHandler_CreateDefinition(t *testing.T) {
	handler, tmpDir := setupGitOpsHandler(t)

	def := &gitops.FeatureDefinition{
		APIVersion: "feather.io/v1",
		Kind:       "FeatureGroup",
		Metadata:   gitops.DefinitionMeta{Name: "new_features"},
		Spec: gitops.FeatureSpec{
			EntityType: "user",
			Features:   []gitops.FeatureField{{Name: "age", DataType: "int64"}},
		},
	}

	createReq := CreateDefinitionRequest{
		Path:       "new.yaml",
		Definition: def,
	}

	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/gitops/definitions", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.handleCreateDefinition(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify file was created
	if _, err := os.Stat(filepath.Join(tmpDir, "new.yaml")); os.IsNotExist(err) {
		t.Error("Expected file to be created")
	}
}

func TestGitOpsHandler_CreateDefinition_MissingPath(t *testing.T) {
	handler, _ := setupGitOpsHandler(t)

	createReq := CreateDefinitionRequest{
		Definition: &gitops.FeatureDefinition{},
	}

	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/gitops/definitions", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.handleCreateDefinition(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGitOpsHandler_CreateDefinition_MissingDefinition(t *testing.T) {
	handler, _ := setupGitOpsHandler(t)

	createReq := CreateDefinitionRequest{
		Path: "test.yaml",
	}

	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/gitops/definitions", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.handleCreateDefinition(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGitOpsSyncRequest_ToConfig(t *testing.T) {
	tests := []struct {
		mode     string
		expected gitops.SyncMode
	}{
		{"", gitops.SyncModeApply},
		{"apply", gitops.SyncModeApply},
		{"dry_run", gitops.SyncModeDryRun},
		{"delete", gitops.SyncModeDelete},
		{"force", gitops.SyncModeForce},
	}

	for _, tt := range tests {
		req := GitOpsSyncRequest{Mode: tt.mode, FilePattern: "*.yaml"}
		config := req.toConfig()
		if config.Mode != tt.expected {
			t.Errorf("Mode %q: expected %s, got %s", tt.mode, tt.expected, config.Mode)
		}
	}
}
