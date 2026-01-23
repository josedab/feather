package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/feathercli"
)

// FeatherCLIHandler handles Feather CLI API requests.
type FeatherCLIHandler struct {
	client *feathercli.Client
}

// NewFeatherCLIHandler creates a new Feather CLI handler.
func NewFeatherCLIHandler(client *feathercli.Client) *FeatherCLIHandler {
	return &FeatherCLIHandler{
		client: client,
	}
}

// RegisterRoutes registers Feather CLI API routes.
func (h *FeatherCLIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/cli/query", h.handleQuery)
	mux.HandleFunc("GET /v1/cli/groups", h.handleListGroups)
	mux.HandleFunc("GET /v1/cli/schema/{group}", h.handleGetSchema)
	mux.HandleFunc("GET /v1/cli/health", h.handleGetHealth)
	mux.HandleFunc("GET /v1/cli/stats", h.handleGetStats)
}

// handleQuery handles POST /v1/cli/query
func (h *FeatherCLIHandler) handleQuery(w http.ResponseWriter, r *http.Request) {
	var query feathercli.FeatureQuery
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.client.GetFeatures(query)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleListGroups handles GET /v1/cli/groups
func (h *FeatherCLIHandler) handleListGroups(w http.ResponseWriter, r *http.Request) {
	result, err := h.client.ListGroups()
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleGetSchema handles GET /v1/cli/schema/{group}
func (h *FeatherCLIHandler) handleGetSchema(w http.ResponseWriter, r *http.Request) {
	group := r.PathValue("group")
	if group == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "group name required")
		return
	}

	result, err := h.client.GetSchema(group)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleGetHealth handles GET /v1/cli/health
func (h *FeatherCLIHandler) handleGetHealth(w http.ResponseWriter, r *http.Request) {
	result, err := h.client.GetHealth()
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleGetStats handles GET /v1/cli/stats
func (h *FeatherCLIHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.client.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *FeatherCLIHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *FeatherCLIHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
