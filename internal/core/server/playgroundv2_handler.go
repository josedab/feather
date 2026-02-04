package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/playgroundv2"
)

// PlaygroundV2Handler handles playground v2 API requests.
type PlaygroundV2Handler struct {
	env *playgroundv2.Environment
}

// NewPlaygroundV2Handler creates a new playground v2 handler.
func NewPlaygroundV2Handler(env *playgroundv2.Environment) *PlaygroundV2Handler {
	return &PlaygroundV2Handler{env: env}
}

// RegisterRoutes registers playground v2 API routes.
func (h *PlaygroundV2Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/playground/query", h.handleExecuteQuery)
	mux.HandleFunc("GET /v1/playground/schemas", h.handleBrowseSchemas)
	mux.HandleFunc("GET /v1/playground/schemas/{name}", h.handleGetSchemaDetails)
	mux.HandleFunc("POST /v1/playground/simulate", h.handleStartSimulation)
	mux.HandleFunc("DELETE /v1/playground/simulate/{id}", h.handleStopSimulation)
	mux.HandleFunc("POST /v1/playground/deploy/preview", h.handlePreviewRegistration)
	mux.HandleFunc("POST /v1/playground/deploy/confirm", h.handleConfirmRegistration)
	mux.HandleFunc("GET /v1/playground/v2/stats", h.handleGetStats)
}

// handleExecuteQuery handles POST /v1/playground/query
func (h *PlaygroundV2Handler) handleExecuteQuery(w http.ResponseWriter, r *http.Request) {
	var query playgroundv2.Query
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.env.ExecuteQuery(query)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleBrowseSchemas handles GET /v1/playground/schemas
func (h *PlaygroundV2Handler) handleBrowseSchemas(w http.ResponseWriter, r *http.Request) {
	schemas := h.env.BrowseSchemas()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"schemas": schemas,
	})
}

// handleGetSchemaDetails handles GET /v1/playground/schemas/{name}
func (h *PlaygroundV2Handler) handleGetSchemaDetails(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "schema name required")
		return
	}

	details, err := h.env.GetSchemaDetails(name)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, details)
}

// handleStartSimulation handles POST /v1/playground/simulate
func (h *PlaygroundV2Handler) handleStartSimulation(w http.ResponseWriter, r *http.Request) {
	var cfg playgroundv2.SimulationConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.env.StartSimulation(cfg)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, session)
}

// handleStopSimulation handles DELETE /v1/playground/simulate/{id}
func (h *PlaygroundV2Handler) handleStopSimulation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "simulation id required")
		return
	}

	if err := h.env.StopSimulation(id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "simulation stopped"})
}

// handlePreviewRegistration handles POST /v1/playground/deploy/preview
func (h *PlaygroundV2Handler) handlePreviewRegistration(w http.ResponseWriter, r *http.Request) {
	var spec playgroundv2.RegistrationSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	preview, err := h.env.PreviewRegistration(spec)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, preview)
}

// handleConfirmRegistration handles POST /v1/playground/deploy/confirm
func (h *PlaygroundV2Handler) handleConfirmRegistration(w http.ResponseWriter, r *http.Request) {
	var spec playgroundv2.RegistrationSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.env.ConfirmRegistration(spec)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleGetStats handles GET /v1/playground/v2/stats
func (h *PlaygroundV2Handler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.env.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *PlaygroundV2Handler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *PlaygroundV2Handler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
