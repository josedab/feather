package server

import (
	"context"
	"net/http"

	"github.com/feather-store/feather/internal/platform/registry"
)

// ModelRegistryHandler handles multi-model feature registry API requests.
type ModelRegistryHandler struct {
	registry *registry.ModelRegistry
}

// NewModelRegistryHandler creates a new model registry handler.
func NewModelRegistryHandler(reg *registry.ModelRegistry) *ModelRegistryHandler {
	return &ModelRegistryHandler{registry: reg}
}

// RegisterRoutes registers model registry API routes.
func (h *ModelRegistryHandler) RegisterRoutes(mux *http.ServeMux) {
	// Model bindings
	mux.HandleFunc("GET /v1/registry/models", h.handleListModels)
	mux.HandleFunc("POST /v1/registry/models", h.handleRegisterModel)
	mux.HandleFunc("GET /v1/registry/models/{id}", h.handleGetModel)
	mux.HandleFunc("DELETE /v1/registry/models/{id}", h.handleRemoveModel)

	// Feature-to-model queries
	mux.HandleFunc("GET /v1/registry/features/{name}/usage", h.handleFeatureUsage)
	mux.HandleFunc("GET /v1/registry/features/{name}/blast-radius", h.handleBlastRadius)

	// Deprecation management
	mux.HandleFunc("GET /v1/registry/deprecations", h.handleListDeprecations)
	mux.HandleFunc("POST /v1/registry/deprecations", h.handleDeprecateFeature)
	mux.HandleFunc("POST /v1/registry/deprecations/{feature}/ack", h.handleAckDeprecation)

	// Stats
	mux.HandleFunc("GET /v1/registry/stats", h.handleStats)
}

func (h *ModelRegistryHandler) handleListModels(w http.ResponseWriter, r *http.Request) {
	models := h.registry.ListModels()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"models": models,
		"count":  len(models),
	})
}

func (h *ModelRegistryHandler) handleRegisterModel(w http.ResponseWriter, r *http.Request) {
	var binding registry.ModelBinding
	if err := strictDecode(r.Body, &binding); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.registry.RegisterModel(binding); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "model registered"})
}

func (h *ModelRegistryHandler) handleGetModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	model, err := h.registry.GetModel(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, model)
}

func (h *ModelRegistryHandler) handleRemoveModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.registry.RemoveModel(id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "model removed"})
}

func (h *ModelRegistryHandler) handleFeatureUsage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	usage := h.registry.GetFeatureUsage(name)
	h.writeJSON(r.Context(), w, http.StatusOK, usage)
}

func (h *ModelRegistryHandler) handleBlastRadius(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	br := h.registry.AnalyzeBlastRadius(name)
	h.writeJSON(r.Context(), w, http.StatusOK, br)
}

func (h *ModelRegistryHandler) handleListDeprecations(w http.ResponseWriter, r *http.Request) {
	deprecations := h.registry.GetDeprecations()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"deprecations": deprecations,
		"count":        len(deprecations),
	})
}

func (h *ModelRegistryHandler) handleDeprecateFeature(w http.ResponseWriter, r *http.Request) {
	var notice registry.DeprecationNotice
	if err := strictDecode(r.Body, &notice); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.registry.DeprecateFeature(notice); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "deprecation notice created"})
}

func (h *ModelRegistryHandler) handleAckDeprecation(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	var req struct {
		Owner string `json:"owner"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.registry.AcknowledgeDeprecation(feature, req.Owner); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "deprecation acknowledged"})
}

func (h *ModelRegistryHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.registry.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *ModelRegistryHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *ModelRegistryHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
