package server

import (
	"context"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/apigateway"
)

// APIGatewayHandler handles API gateway requests.
type APIGatewayHandler struct {
	gateway *apigateway.Gateway
}

// NewAPIGatewayHandler creates a new API gateway handler.
func NewAPIGatewayHandler(gateway *apigateway.Gateway) *APIGatewayHandler {
	return &APIGatewayHandler{
		gateway: gateway,
	}
}

// RegisterRoutes registers API gateway routes.
func (h *APIGatewayHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/gateway/backends", h.handleListBackends)
	mux.HandleFunc("POST /v1/gateway/backends", h.handleAddBackend)
	mux.HandleFunc("DELETE /v1/gateway/backends/{id}", h.handleRemoveBackend)
	mux.HandleFunc("PUT /v1/gateway/backends/{id}/status", h.handleUpdateStatus)
	mux.HandleFunc("POST /v1/gateway/route", h.handleRoute)
	mux.HandleFunc("GET /v1/gateway/backends/stats", h.handleGetBackendStats)
	mux.HandleFunc("GET /v1/gateway/stats", h.handleGetStats)
}

// handleListBackends handles GET /v1/gateway/backends
func (h *APIGatewayHandler) handleListBackends(w http.ResponseWriter, r *http.Request) {
	backends := h.gateway.ListBackends()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"backends": backends,
	})
}

// handleAddBackend handles POST /v1/gateway/backends
func (h *APIGatewayHandler) handleAddBackend(w http.ResponseWriter, r *http.Request) {
	var backend apigateway.Backend
	if err := strictDecode(r.Body, &backend); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.gateway.AddBackend(backend); err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "backend added"})
}

// handleRemoveBackend handles DELETE /v1/gateway/backends/{id}
func (h *APIGatewayHandler) handleRemoveBackend(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "backend id required")
		return
	}

	if err := h.gateway.RemoveBackend(id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "backend removed"})
}

// handleUpdateStatus handles PUT /v1/gateway/backends/{id}/status
func (h *APIGatewayHandler) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "backend id required")
		return
	}

	var req struct {
		Status  string  `json:"status"`
		Latency float64 `json:"latency"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.gateway.UpdateBackendStatus(id, apigateway.BackendStatus(req.Status), req.Latency); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "backend status updated"})
}

// handleRoute handles POST /v1/gateway/route
func (h *APIGatewayHandler) handleRoute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID  string `json:"tenant_id"`
		EntityKey string `json:"entity_key"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.gateway.Route(req.TenantID, req.EntityKey)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleGetBackendStats handles GET /v1/gateway/backends/stats
func (h *APIGatewayHandler) handleGetBackendStats(w http.ResponseWriter, r *http.Request) {
	stats := h.gateway.GetBackendStats()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"backends": stats,
	})
}

// handleGetStats handles GET /v1/gateway/stats
func (h *APIGatewayHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.gateway.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *APIGatewayHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *APIGatewayHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
