package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/feather-store/feather/internal/extensions/offlinestore"
)

// OfflineStoreHandler handles offline store API requests.
type OfflineStoreHandler struct {
	store *offlinestore.Store
}

// NewOfflineStoreHandler creates a new offline store handler.
func NewOfflineStoreHandler(store *offlinestore.Store) *OfflineStoreHandler {
	return &OfflineStoreHandler{store: store}
}

// RegisterRoutes registers offline store API routes.
func (h *OfflineStoreHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/offline/datasets", h.handleListDatasets)
	mux.HandleFunc("POST /v1/offline/datasets", h.handleCreateDataset)
	mux.HandleFunc("GET /v1/offline/datasets/{name}", h.handleGetDataset)
	mux.HandleFunc("DELETE /v1/offline/datasets/{name}", h.handleDeleteDataset)
	mux.HandleFunc("POST /v1/offline/datasets/{name}/rows", h.handleAppendRows)
	mux.HandleFunc("GET /v1/offline/datasets/{name}/rows", h.handleGetRows)
	mux.HandleFunc("GET /v1/offline/datasets/{name}/pit", h.handlePointInTime)
	mux.HandleFunc("POST /v1/offline/datasets/{name}/export", h.handleExportDataset)
	mux.HandleFunc("GET /v1/offline/stats", h.handleGetStats)
}

// handleListDatasets handles GET /v1/offline/datasets
func (h *OfflineStoreHandler) handleListDatasets(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "offline store not configured")
		return
	}

	datasets := h.store.ListDatasets()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"datasets": datasets,
	})
}

// handleCreateDataset handles POST /v1/offline/datasets
func (h *OfflineStoreHandler) handleCreateDataset(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "offline store not configured")
		return
	}

	var cfg offlinestore.DatasetConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	info, err := h.store.CreateDataset(cfg)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, info)
}

// handleGetDataset handles GET /v1/offline/datasets/{name}
func (h *OfflineStoreHandler) handleGetDataset(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "offline store not configured")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "dataset name is required")
		return
	}

	info, err := h.store.GetDataset(name)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, info)
}

// handleDeleteDataset handles DELETE /v1/offline/datasets/{name}
func (h *OfflineStoreHandler) handleDeleteDataset(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "offline store not configured")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "dataset name is required")
		return
	}

	if err := h.store.DeleteDataset(name); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "dataset deleted"})
}

// handleAppendRows handles POST /v1/offline/datasets/{name}/rows
func (h *OfflineStoreHandler) handleAppendRows(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "offline store not configured")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "dataset name is required")
		return
	}

	var req struct {
		Rows []offlinestore.FeatureRow `json:"rows"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.store.AppendRows(name, req.Rows); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"count":   len(req.Rows),
	})
}

// handleGetRows handles GET /v1/offline/datasets/{name}/rows
func (h *OfflineStoreHandler) handleGetRows(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "offline store not configured")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "dataset name is required")
		return
	}

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	rows, err := h.store.GetRows(name, limit, offset)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"rows":   rows,
		"limit":  limit,
		"offset": offset,
	})
}

// handlePointInTime handles GET /v1/offline/datasets/{name}/pit
func (h *OfflineStoreHandler) handlePointInTime(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "offline store not configured")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "dataset name is required")
		return
	}

	entityID := r.URL.Query().Get("entity_id")
	if entityID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entity_id is required")
		return
	}

	asOf := time.Now()
	if v := r.URL.Query().Get("as_of"); v != "" {
		if parsed, err := time.Parse(time.RFC3339, v); err == nil {
			asOf = parsed
		}
	}

	rows, err := h.store.GetPointInTime(name, entityID, asOf)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"rows":      rows,
		"entity_id": entityID,
		"as_of":     asOf,
	})
}

// handleExportDataset handles POST /v1/offline/datasets/{name}/export
func (h *OfflineStoreHandler) handleExportDataset(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "offline store not configured")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "dataset name is required")
		return
	}

	var cfg offlinestore.ExportConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.store.ExportDataset(name, cfg)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleGetStats handles GET /v1/offline/stats
func (h *OfflineStoreHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "offline store not configured")
		return
	}

	stats := h.store.Stats()

	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *OfflineStoreHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *OfflineStoreHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
