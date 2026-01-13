package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/llmfeature"
)

// LLMFeatureHandler provides HTTP endpoints for LLM feature types.
type LLMFeatureHandler struct {
	store *llmfeature.Store
}

// NewLLMFeatureHandler creates a new LLM feature handler.
func NewLLMFeatureHandler(store *llmfeature.Store) *LLMFeatureHandler {
	return &LLMFeatureHandler{store: store}
}

// RegisterRoutes registers LLM feature API routes.
func (h *LLMFeatureHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/llm/templates", h.handleListTemplates)
	mux.HandleFunc("POST /v1/llm/templates", h.handleCreateTemplate)
	mux.HandleFunc("GET /v1/llm/templates/{id}", h.handleGetTemplate)
	mux.HandleFunc("PUT /v1/llm/templates/{id}", h.handleUpdateTemplate)
	mux.HandleFunc("DELETE /v1/llm/templates/{id}", h.handleDeleteTemplate)
	mux.HandleFunc("POST /v1/llm/completions", h.handleStoreCompletion)
	mux.HandleFunc("GET /v1/llm/completions/{id}", h.handleGetCompletion)
	mux.HandleFunc("GET /v1/llm/usage", h.handleGetAllUsage)
	mux.HandleFunc("GET /v1/llm/usage/{model}", h.handleGetUsage)
	mux.HandleFunc("GET /v1/llm/stats", h.handleStats)
}

func (h *LLMFeatureHandler) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "llm features not configured")
		return
	}
	templates := h.store.ListTemplates()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "templates": templates, "count": len(templates),
	})
}

func (h *LLMFeatureHandler) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "llm features not configured")
		return
	}
	var tmpl llmfeature.PromptTemplate
	if err := json.NewDecoder(r.Body).Decode(&tmpl); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.store.CreateTemplate(&tmpl); err != nil {
		status := http.StatusBadRequest
		if err == llmfeature.ErrTemplateExists {
			status = http.StatusConflict
		}
		writeJSONError(r.Context(), w, status, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "template": tmpl})
}

func (h *LLMFeatureHandler) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "llm features not configured")
		return
	}
	tmpl, err := h.store.GetTemplate(r.PathValue("id"))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "template": tmpl})
}

func (h *LLMFeatureHandler) handleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "llm features not configured")
		return
	}
	var tmpl llmfeature.PromptTemplate
	if err := json.NewDecoder(r.Body).Decode(&tmpl); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	tmpl.ID = r.PathValue("id")
	if err := h.store.UpdateTemplate(&tmpl); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "template": tmpl})
}

func (h *LLMFeatureHandler) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "llm features not configured")
		return
	}
	if err := h.store.DeleteTemplate(r.PathValue("id")); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "message": "template deleted"})
}

func (h *LLMFeatureHandler) handleStoreCompletion(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "llm features not configured")
		return
	}
	var rec llmfeature.CompletionRecord
	if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	h.store.StoreCompletion(&rec)
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "completion": rec})
}

func (h *LLMFeatureHandler) handleGetCompletion(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "llm features not configured")
		return
	}
	rec, err := h.store.GetCompletion(r.PathValue("id"))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "completion": rec})
}

func (h *LLMFeatureHandler) handleGetAllUsage(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "llm features not configured")
		return
	}
	usage := h.store.GetAllUsage()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "usage": usage})
}

func (h *LLMFeatureHandler) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "llm features not configured")
		return
	}
	usage := h.store.GetUsage(r.PathValue("model"))
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "usage": usage})
}

func (h *LLMFeatureHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "llm features not configured")
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "stats": h.store.Stats(),
	})
}
