package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/gitopsdefs"
)

// GitOpsDefsHandler handles GitOps definitions API requests.
type GitOpsDefsHandler struct {
	reconciler *gitopsdefs.Reconciler
}

// NewGitOpsDefsHandler creates a new GitOps definitions handler.
func NewGitOpsDefsHandler(reconciler *gitopsdefs.Reconciler) *GitOpsDefsHandler {
	return &GitOpsDefsHandler{reconciler: reconciler}
}

// RegisterRoutes registers GitOps definitions API routes.
func (h *GitOpsDefsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/gitops/definitions", h.handleListDefinitions)
	mux.HandleFunc("POST /v1/gitops/definitions", h.handleLoadDefinition)
	mux.HandleFunc("GET /v1/gitops/definitions/{name}", h.handleGetDefinition)
	mux.HandleFunc("DELETE /v1/gitops/definitions/{name}", h.handleDeleteDefinition)
	mux.HandleFunc("POST /v1/gitops/reconcile", h.handleReconcile)
	mux.HandleFunc("GET /v1/gitops/diff", h.handleGetDiff)
	mux.HandleFunc("GET /v1/gitops/history", h.handleGetHistory)
	mux.HandleFunc("GET /v1/gitops/stats", h.handleGetStats)
	mux.HandleFunc("POST /v1/gitops/plan", h.handlePlan)
	mux.HandleFunc("POST /v1/gitops/apply", h.handleApply)
	mux.HandleFunc("POST /v1/gitops/validate", h.handleValidate)
	mux.HandleFunc("POST /v1/gitops/rollback/{name}", h.handleRollback)
	mux.HandleFunc("POST /v1/gitops/load-spec", h.handleLoadSpec)
}

// handleListDefinitions handles GET /v1/gitops/definitions
func (h *GitOpsDefsHandler) handleListDefinitions(w http.ResponseWriter, r *http.Request) {
	if h.reconciler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "gitops reconciler not configured")
		return
	}

	definitions := h.reconciler.ListDefinitions()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"definitions": definitions,
	})
}

// handleLoadDefinition handles POST /v1/gitops/definitions
func (h *GitOpsDefsHandler) handleLoadDefinition(w http.ResponseWriter, r *http.Request) {
	if h.reconciler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "gitops reconciler not configured")
		return
	}

	var def gitopsdefs.FeatureDefinition
	if err := strictDecode(r.Body, &def); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.reconciler.LoadDefinition(def); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"name":    def.Name,
	})
}

// handleGetDefinition handles GET /v1/gitops/definitions/{name}
func (h *GitOpsDefsHandler) handleGetDefinition(w http.ResponseWriter, r *http.Request) {
	if h.reconciler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "gitops reconciler not configured")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "definition name is required")
		return
	}

	def, err := h.reconciler.GetDefinition(name)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, def)
}

// handleDeleteDefinition handles DELETE /v1/gitops/definitions/{name}
func (h *GitOpsDefsHandler) handleDeleteDefinition(w http.ResponseWriter, r *http.Request) {
	if h.reconciler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "gitops reconciler not configured")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "definition name is required")
		return
	}

	if err := h.reconciler.DeleteDefinition(name); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "definition deleted"})
}

// handleReconcile handles POST /v1/gitops/reconcile
func (h *GitOpsDefsHandler) handleReconcile(w http.ResponseWriter, r *http.Request) {
	if h.reconciler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "gitops reconciler not configured")
		return
	}

	results := h.reconciler.Reconcile()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
	})
}

// handleGetDiff handles GET /v1/gitops/diff
func (h *GitOpsDefsHandler) handleGetDiff(w http.ResponseWriter, r *http.Request) {
	if h.reconciler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "gitops reconciler not configured")
		return
	}

	diff := h.reconciler.Diff()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"diff": diff,
	})
}

// handleGetHistory handles GET /v1/gitops/history
func (h *GitOpsDefsHandler) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	if h.reconciler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "gitops reconciler not configured")
		return
	}

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	history := h.reconciler.GetHistory(limit)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"history": history,
	})
}

// handleGetStats handles GET /v1/gitops/stats
func (h *GitOpsDefsHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.reconciler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "gitops reconciler not configured")
		return
	}

	stats := h.reconciler.Stats()

	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *GitOpsDefsHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *GitOpsDefsHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}

func (h *GitOpsDefsHandler) handlePlan(w http.ResponseWriter, r *http.Request) {
	if h.reconciler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "gitops reconciler not configured")
		return
	}
	plan := h.reconciler.Plan()
	h.writeJSON(r.Context(), w, http.StatusOK, plan)
}

func (h *GitOpsDefsHandler) handleApply(w http.ResponseWriter, r *http.Request) {
	if h.reconciler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "gitops reconciler not configured")
		return
	}
	result := h.reconciler.Apply()
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

func (h *GitOpsDefsHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	var spec gitopsdefs.DeclarativeSpec
	if err := strictDecode(r.Body, &spec); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result := gitopsdefs.ValidateSpec(&spec)
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

func (h *GitOpsDefsHandler) handleRollback(w http.ResponseWriter, r *http.Request) {
	if h.reconciler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "gitops reconciler not configured")
		return
	}
	name := r.PathValue("name")
	result, err := h.reconciler.Rollback(name)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

func (h *GitOpsDefsHandler) handleLoadSpec(w http.ResponseWriter, r *http.Request) {
	if h.reconciler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "gitops reconciler not configured")
		return
	}

	var body json.RawMessage
	if err := strictDecode(r.Body, &body); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	spec, validation, err := h.reconciler.LoadSpecJSON(body)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	status := http.StatusCreated
	if !validation.Valid {
		status = http.StatusBadRequest
	}
	h.writeJSON(r.Context(), w, status, map[string]interface{}{
		"spec":       spec,
		"validation": validation,
	})
}
