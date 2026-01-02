package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/gitops"
)

// GitOpsHandler handles GitOps API requests.
type GitOpsHandler struct {
	loader       *gitops.SchemaLoader
	policyEngine *gitops.PolicyEngine
	syncManager  *gitops.SyncManager
}

// NewGitOpsHandler creates a new GitOps handler.
func NewGitOpsHandler(loader *gitops.SchemaLoader, policyEngine *gitops.PolicyEngine, syncManager *gitops.SyncManager) *GitOpsHandler {
	return &GitOpsHandler{
		loader:       loader,
		policyEngine: policyEngine,
		syncManager:  syncManager,
	}
}

// RegisterRoutes registers GitOps routes.
func (h *GitOpsHandler) RegisterRoutes(mux *http.ServeMux) {
	// Policy endpoints
	mux.HandleFunc("GET /v1/gitops/policies", h.handleListPolicies)
	mux.HandleFunc("POST /v1/gitops/policies", h.handleCreatePolicy)
	mux.HandleFunc("GET /v1/gitops/policies/{name}", h.handleGetPolicy)
	mux.HandleFunc("DELETE /v1/gitops/policies/{name}", h.handleDeletePolicy)

	// Sync endpoints
	mux.HandleFunc("POST /v1/gitops/sync", h.handleSync)
	mux.HandleFunc("POST /v1/gitops/diff", h.handleDiff)
	mux.HandleFunc("POST /v1/gitops/validate", h.handleValidate)

	// History endpoints
	mux.HandleFunc("GET /v1/gitops/history", h.handleGetHistory)
	mux.HandleFunc("GET /v1/gitops/history/latest", h.handleGetLatestResult)

	// Definition endpoints
	mux.HandleFunc("GET /v1/gitops/definitions/{path...}", h.handleGetDefinition)
	mux.HandleFunc("POST /v1/gitops/definitions", h.handleCreateDefinition)
}

// handleListPolicies returns all registered policies.
func (h *GitOpsHandler) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	policies := h.policyEngine.ListPolicies()
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"policies": policies,
		"count":    len(policies),
	})
}

// handleCreatePolicy registers a new policy.
func (h *GitOpsHandler) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	var policy gitops.Policy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if err := h.policyEngine.RegisterPolicy(&policy); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(w, http.StatusCreated, policy)
}

// handleGetPolicy returns a specific policy.
func (h *GitOpsHandler) handleGetPolicy(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	policy, exists := h.policyEngine.GetPolicy(name)
	if !exists {
		h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "policy not found"})
		return
	}

	h.writeJSON(w, http.StatusOK, policy)
}

// handleDeletePolicy removes a policy.
func (h *GitOpsHandler) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	_, exists := h.policyEngine.GetPolicy(name)
	if !exists {
		h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "policy not found"})
		return
	}

	h.policyEngine.UnregisterPolicy(name)
	w.WriteHeader(http.StatusNoContent)
}

// GitOpsSyncRequest is the request body for GitOps sync operations.
type GitOpsSyncRequest struct {
	Mode            string            `json:"mode,omitempty"`
	FilePattern     string            `json:"filePattern"`
	Namespaces      []string          `json:"namespaces,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	PruneOrphans    bool              `json:"pruneOrphans,omitempty"`
	EnforcePolicies bool              `json:"enforcePolicies,omitempty"`
	ContinueOnError bool              `json:"continueOnError,omitempty"`
}

func (r *GitOpsSyncRequest) toConfig() *gitops.SyncConfig {
	mode := gitops.SyncModeApply
	switch r.Mode {
	case "dry_run":
		mode = gitops.SyncModeDryRun
	case "delete":
		mode = gitops.SyncModeDelete
	case "force":
		mode = gitops.SyncModeForce
	}

	return &gitops.SyncConfig{
		Mode:            mode,
		FilePattern:     r.FilePattern,
		Namespaces:      r.Namespaces,
		Labels:          r.Labels,
		PruneOrphans:    r.PruneOrphans,
		EnforcePolicies: r.EnforcePolicies,
		ContinueOnError: r.ContinueOnError,
	}
}

// handleSync performs a sync operation.
func (h *GitOpsHandler) handleSync(w http.ResponseWriter, r *http.Request) {
	var req GitOpsSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.FilePattern == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "filePattern is required"})
		return
	}

	config := req.toConfig()
	result, err := h.syncManager.Sync(r.Context(), config)
	if err != nil {
		// Still return the result even on error
		h.writeJSON(w, http.StatusOK, result)
		return
	}

	status := http.StatusOK
	if result.State == gitops.SyncStateFailed {
		status = http.StatusConflict
	}

	h.writeJSON(w, status, result)
}

// handleDiff computes differences without applying.
func (h *GitOpsHandler) handleDiff(w http.ResponseWriter, r *http.Request) {
	var req GitOpsSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.FilePattern == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "filePattern is required"})
		return
	}

	config := req.toConfig()
	report, err := h.syncManager.Diff(r.Context(), config)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, report)
}

// handleValidate validates definitions against policies.
func (h *GitOpsHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	var req GitOpsSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.FilePattern == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "filePattern is required"})
		return
	}

	config := req.toConfig()
	violations, err := h.syncManager.Validate(config)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid":      len(violations) == 0,
		"violations": violations,
		"count":      len(violations),
	})
}

// handleGetHistory returns sync history.
func (h *GitOpsHandler) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	history := h.syncManager.GetHistory()
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"history": history,
		"count":   len(history),
	})
}

// handleGetLatestResult returns the latest sync result.
func (h *GitOpsHandler) handleGetLatestResult(w http.ResponseWriter, r *http.Request) {
	result := h.syncManager.GetLastResult()
	if result == nil {
		h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "no sync history"})
		return
	}
	h.writeJSON(w, http.StatusOK, result)
}

// handleGetDefinition returns a feature definition.
func (h *GitOpsHandler) handleGetDefinition(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")

	def, err := h.loader.LoadDefinition(path)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, def)
}

// CreateDefinitionRequest is the request to create a definition.
type CreateDefinitionRequest struct {
	Path       string                    `json:"path"`
	Definition *gitops.FeatureDefinition `json:"definition"`
}

// handleCreateDefinition creates or updates a feature definition.
func (h *GitOpsHandler) handleCreateDefinition(w http.ResponseWriter, r *http.Request) {
	var req CreateDefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Path == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path is required"})
		return
	}
	if req.Definition == nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "definition is required"})
		return
	}

	if err := h.loader.SaveDefinition(req.Definition, req.Path); err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(w, http.StatusCreated, req.Definition)
}

// writeJSON writes a JSON response.
func (h *GitOpsHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}
