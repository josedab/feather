package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/openapisync"
)

// OpenAPISyncHandler handles OpenAPI spec generation API requests.
type OpenAPISyncHandler struct {
	generator *openapisync.Generator
}

// NewOpenAPISyncHandler creates a new OpenAPI sync handler.
func NewOpenAPISyncHandler(generator *openapisync.Generator) *OpenAPISyncHandler {
	return &OpenAPISyncHandler{
		generator: generator,
	}
}

// RegisterRoutes registers OpenAPI sync API routes.
func (h *OpenAPISyncHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/openapi/spec", h.handleGetSpec)
	mux.HandleFunc("GET /v1/openapi/routes", h.handleListRoutes)
	mux.HandleFunc("POST /v1/openapi/routes", h.handleAddRoute)
	mux.HandleFunc("GET /v1/openapi/stats", h.handleGetStats)
}

// handleGetSpec handles GET /v1/openapi/spec
func (h *OpenAPISyncHandler) handleGetSpec(w http.ResponseWriter, r *http.Request) {
	spec := h.generator.GenerateSpec()
	h.writeJSON(r.Context(), w, http.StatusOK, spec)
}

// handleListRoutes handles GET /v1/openapi/routes
func (h *OpenAPISyncHandler) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	routes := h.generator.ListRoutes()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"routes": routes,
	})
}

// handleAddRoute handles POST /v1/openapi/routes
func (h *OpenAPISyncHandler) handleAddRoute(w http.ResponseWriter, r *http.Request) {
	var route openapisync.RouteInfo
	if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.generator.AddRoute(route)

	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "route added"})
}

// handleGetStats handles GET /v1/openapi/stats
func (h *OpenAPISyncHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.generator.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *OpenAPISyncHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *OpenAPISyncHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
