package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/promptstore"
)

// PromptStoreHandler handles LLM prompt store API requests.
type PromptStoreHandler struct {
	store *promptstore.Store
}

// NewPromptStoreHandler creates a new prompt store handler.
func NewPromptStoreHandler(store *promptstore.Store) *PromptStoreHandler {
	return &PromptStoreHandler{store: store}
}

// RegisterRoutes registers prompt store API routes.
func (h *PromptStoreHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/prompts", h.handleList)
	mux.HandleFunc("POST /v1/prompts", h.handleCreate)
	mux.HandleFunc("GET /v1/prompts/{id}", h.handleGet)
	mux.HandleFunc("PUT /v1/prompts/{id}", h.handleUpdate)
	mux.HandleFunc("DELETE /v1/prompts/{id}", h.handleDelete)
	mux.HandleFunc("GET /v1/prompts/{id}/versions", h.handleListVersions)
	mux.HandleFunc("POST /v1/prompts/{id}/render", h.handleRender)
	mux.HandleFunc("GET /v1/prompts/{id}/usage", h.handleGetUsage)
	mux.HandleFunc("POST /v1/prompts/{id}/score", h.handleRecordScore)
	mux.HandleFunc("GET /v1/prompts/stats", h.handleStats)
}

func (h *PromptStoreHandler) handleList(w http.ResponseWriter, r *http.Request) {
	prompts := h.store.List()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"prompts": prompts,
		"count":   len(prompts),
	})
}

func (h *PromptStoreHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var tmpl promptstore.PromptTemplate
	if err := json.NewDecoder(r.Body).Decode(&tmpl); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.store.Create(tmpl)
	if err != nil {
		if errors.Is(err, promptstore.ErrPromptExists) {
			h.writeError(r.Context(), w, http.StatusConflict, "prompt already exists")
			return
		}
		if errors.Is(err, promptstore.ErrInvalidTemplate) {
			h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, result)
}

func (h *PromptStoreHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tmpl, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, promptstore.ErrPromptNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "prompt not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, tmpl)
}

func (h *PromptStoreHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var tmpl promptstore.PromptTemplate
	if err := json.NewDecoder(r.Body).Decode(&tmpl); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.store.Update(id, tmpl)
	if err != nil {
		if errors.Is(err, promptstore.ErrPromptNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "prompt not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

func (h *PromptStoreHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.Delete(id); err != nil {
		if errors.Is(err, promptstore.ErrPromptNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "prompt not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "prompt deleted"})
}

func (h *PromptStoreHandler) handleListVersions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	versions, err := h.store.ListVersions(id)
	if err != nil {
		if errors.Is(err, promptstore.ErrPromptNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "prompt not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"versions": versions,
		"count":    len(versions),
	})
}

func (h *PromptStoreHandler) handleRender(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Variables map[string]string `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.store.Render(id, req.Variables)
	if err != nil {
		if errors.Is(err, promptstore.ErrPromptNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "prompt not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

func (h *PromptStoreHandler) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	usage := h.store.GetUsage(id)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"usage": usage,
	})
}

func (h *PromptStoreHandler) handleRecordScore(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Version int     `json:"version"`
		Score   float64 `json:"score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.store.RecordScore(id, req.Version, req.Score)
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *PromptStoreHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.store.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *PromptStoreHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *PromptStoreHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
