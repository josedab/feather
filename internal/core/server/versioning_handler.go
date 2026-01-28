package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/versioning"
)

// VersioningHandler handles feature versioning API requests.
type VersioningHandler struct {
	store *versioning.VersionStore
}

// NewVersioningHandler creates a new versioning handler.
func NewVersioningHandler(store *versioning.VersionStore) *VersioningHandler {
	return &VersioningHandler{store: store}
}

// RegisterRoutes registers versioning API routes.
func (h *VersioningHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/versioning/branches", h.handleListBranches)
	mux.HandleFunc("POST /v1/versioning/branches", h.handleCreateBranch)
	mux.HandleFunc("GET /v1/versioning/branches/{name}", h.handleGetBranch)
	mux.HandleFunc("DELETE /v1/versioning/branches/{name}", h.handleDeleteBranch)
	mux.HandleFunc("POST /v1/versioning/commits", h.handleCreateCommit)
	mux.HandleFunc("GET /v1/versioning/commits/{id}", h.handleGetCommit)
	mux.HandleFunc("GET /v1/versioning/history/{branch}", h.handleGetHistory)
	mux.HandleFunc("GET /v1/versioning/tags", h.handleListTags)
	mux.HandleFunc("POST /v1/versioning/tags", h.handleCreateTag)
	mux.HandleFunc("POST /v1/versioning/rollback", h.handleRollback)
}

func (h *VersioningHandler) handleListBranches(w http.ResponseWriter, r *http.Request) {
	branches := h.store.ListBranches()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"branches": branches})
}

type createBranchRequest struct {
	Name       string `json:"name"`
	BaseBranch string `json:"base_branch"`
}

func (h *VersioningHandler) handleCreateBranch(w http.ResponseWriter, r *http.Request) {
	var req createBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "branch name required")
		return
	}
	if err := h.store.CreateBranch(req.Name, req.BaseBranch); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "name": req.Name})
}

func (h *VersioningHandler) handleGetBranch(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "branch name required")
		return
	}
	branch, err := h.store.GetBranch(name)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, branch)
}

func (h *VersioningHandler) handleDeleteBranch(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "branch name required")
		return
	}
	if err := h.store.DeleteBranch(name); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

type createCommitRequest struct {
	Message string               `json:"message"`
	Author  string               `json:"author"`
	Changes []*versioning.Change `json:"changes"`
}

func (h *VersioningHandler) handleCreateCommit(w http.ResponseWriter, r *http.Request) {
	var req createCommitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Message == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "commit message required")
		return
	}
	commit, err := h.store.Commit(r.Context(), req.Message, req.Author, req.Changes)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, commit)
}

func (h *VersioningHandler) handleGetCommit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "commit id required")
		return
	}
	commit, err := h.store.GetCommit(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, commit)
}

func (h *VersioningHandler) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	branch := r.PathValue("branch")
	if branch == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "branch name required")
		return
	}
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	commits := h.store.GetHistory(branch, limit)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"commits": commits})
}

func (h *VersioningHandler) handleListTags(w http.ResponseWriter, r *http.Request) {
	tags := h.store.ListTags()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"tags": tags})
}

type createTagRequest struct {
	Name     string `json:"name"`
	CommitID string `json:"commit_id"`
	Message  string `json:"message"`
}

func (h *VersioningHandler) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	var req createTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tag name required")
		return
	}
	if err := h.store.CreateTag(req.Name, req.CommitID, req.Message); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "name": req.Name})
}

type rollbackRequest struct {
	CommitID string `json:"commit_id"`
}

func (h *VersioningHandler) handleRollback(w http.ResponseWriter, r *http.Request) {
	var req rollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CommitID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "commit_id required")
		return
	}
	if err := h.store.Rollback(r.Context(), req.CommitID); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "rolled_back_to": req.CommitID})
}

func (h *VersioningHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *VersioningHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
