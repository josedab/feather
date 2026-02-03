package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/feastcompat"
)

// FeastGatewayHandler handles the full Feast-compatible gateway API.
type FeastGatewayHandler struct {
	gateway *feastcompat.Gateway
}

// NewFeastGatewayHandler creates a new Feast gateway handler.
func NewFeastGatewayHandler(gateway *feastcompat.Gateway) *FeastGatewayHandler {
	return &FeastGatewayHandler{gateway: gateway}
}

// RegisterRoutes registers Feast gateway API routes.
func (h *FeastGatewayHandler) RegisterRoutes(mux *http.ServeMux) {
	// Push endpoint (Feast push API)
	mux.HandleFunc("POST /v1/feast/push", h.handlePush)

	// Apply/Plan (Feast apply semantics)
	mux.HandleFunc("POST /v1/feast/apply", h.handleApply)

	// Feature services
	mux.HandleFunc("GET /v1/feast/services", h.handleListServices)
	mux.HandleFunc("GET /v1/feast/services/{name}", h.handleGetService)

	// Saved datasets
	mux.HandleFunc("GET /v1/feast/datasets", h.handleListDatasets)
	mux.HandleFunc("POST /v1/feast/datasets", h.handleSaveDataset)
	mux.HandleFunc("GET /v1/feast/datasets/{name}", h.handleGetDataset)

	// Migration tooling
	mux.HandleFunc("POST /v1/feast/migrate", h.handleMigrate)

	// Gateway stats
	mux.HandleFunc("GET /v1/feast/gateway/stats", h.handleGatewayStats)
}

func (h *FeastGatewayHandler) handlePush(w http.ResponseWriter, r *http.Request) {
	var req feastcompat.PushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.gateway.Push(req)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *FeastGatewayHandler) handleApply(w http.ResponseWriter, r *http.Request) {
	var req feastcompat.ApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.gateway.Apply(req)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *FeastGatewayHandler) handleListServices(w http.ResponseWriter, r *http.Request) {
	services := h.gateway.ListFeatureServices()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"feature_services": services,
		"count":            len(services),
	})
}

func (h *FeastGatewayHandler) handleGetService(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	svc, err := h.gateway.GetFeatureService(name)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, svc)
}

func (h *FeastGatewayHandler) handleListDatasets(w http.ResponseWriter, r *http.Request) {
	datasets := h.gateway.ListSavedDatasets()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"datasets": datasets,
		"count":    len(datasets),
	})
}

func (h *FeastGatewayHandler) handleSaveDataset(w http.ResponseWriter, r *http.Request) {
	var ds feastcompat.SavedDataset
	if err := json.NewDecoder(r.Body).Decode(&ds); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	saved, err := h.gateway.SaveDataset(ds)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, saved)
}

func (h *FeastGatewayHandler) handleGetDataset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	ds, err := h.gateway.GetSavedDataset(name)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, ds)
}

func (h *FeastGatewayHandler) handleGatewayStats(w http.ResponseWriter, r *http.Request) {
	stats := h.gateway.GatewayStats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *FeastGatewayHandler) handleMigrate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FeastYAML string `json:"feast_yaml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body; provide feast_yaml field")
		return
	}
	if body.FeastYAML == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feast_yaml field is required")
		return
	}

	result, err := feastcompat.MigrateFromFeastConfig([]byte(body.FeastYAML))
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

func (h *FeastGatewayHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *FeastGatewayHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
