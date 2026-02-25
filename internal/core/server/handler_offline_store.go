package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/offlinestore"
)

// ---------------------------------------------------------------------------
// OfflineStoreSyncHandler
// ---------------------------------------------------------------------------

// OfflineStoreSyncHandler exposes offline store with warehouse sync endpoints.
type OfflineStoreSyncHandler struct {
	syncer *offlinestore.WarehouseSyncer
	store  *offlinestore.Store
}

// NewOfflineStoreSyncHandler creates a new OfflineStoreSyncHandler.
func NewOfflineStoreSyncHandler(store *offlinestore.Store, syncer *offlinestore.WarehouseSyncer) *OfflineStoreSyncHandler {
	return &OfflineStoreSyncHandler{syncer: syncer, store: store}
}

// RegisterRoutes registers offline store sync API routes.
func (h *OfflineStoreSyncHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/offline-sync/datasets", h.handleCreateDataset)
	mux.HandleFunc("GET /v1/offline-sync/datasets", h.handleListDatasets)
	mux.HandleFunc("GET /v1/offline-sync/datasets/{name}", h.handleGetDataset)
	mux.HandleFunc("DELETE /v1/offline-sync/datasets/{name}", h.handleDeleteDataset)
	mux.HandleFunc("POST /v1/offline-sync/datasets/{name}/rows", h.handleAppendRows)
	mux.HandleFunc("GET /v1/offline-sync/datasets/{name}/rows", h.handleGetRows)
	mux.HandleFunc("POST /v1/offline-sync/export/{name}", h.handleExport)
	mux.HandleFunc("POST /v1/offline-sync/import/{name}", h.handleImport)
	mux.HandleFunc("POST /v1/offline-sync/pit-join", h.handlePITJoin)
	mux.HandleFunc("GET /v1/offline-sync/jobs", h.handleListJobs)
	mux.HandleFunc("GET /v1/offline-sync/jobs/{id}", h.handleGetJob)
	mux.HandleFunc("GET /v1/offline-sync/sync-stats", h.handleSyncStats)
	mux.HandleFunc("GET /v1/offline-sync/stats", h.handleStoreStats)
}

func (h *OfflineStoreSyncHandler) handleCreateDataset(w http.ResponseWriter, r *http.Request) {
	var cfg offlinestore.DatasetConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if _, err := h.store.CreateDataset(cfg); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]string{"created": cfg.Name})
}

func (h *OfflineStoreSyncHandler) handleListDatasets(w http.ResponseWriter, r *http.Request) {
	datasets := h.store.ListDatasets()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"datasets": datasets,
		"total":    len(datasets),
	})
}

func (h *OfflineStoreSyncHandler) handleGetDataset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	ds, err := h.store.GetDataset(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, ds)
}

func (h *OfflineStoreSyncHandler) handleDeleteDataset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.store.DeleteDataset(name); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"deleted": name})
}

func (h *OfflineStoreSyncHandler) handleAppendRows(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Rows []offlinestore.FeatureRow `json:"rows"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.store.AppendRows(name, req.Rows); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"appended": len(req.Rows)})
}

func (h *OfflineStoreSyncHandler) handleGetRows(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	rows, err := h.store.GetRows(name, 0, 1000)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"rows":  rows,
		"total": len(rows),
	})
}

func (h *OfflineStoreSyncHandler) handleExport(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	job, err := h.syncer.ExportToWarehouse(r.Context(), name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, job)
}

func (h *OfflineStoreSyncHandler) handleImport(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Rows []offlinestore.FeatureRow `json:"rows"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	job, err := h.syncer.ImportFromWarehouse(r.Context(), name, req.Rows)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, job)
}

func (h *OfflineStoreSyncHandler) handlePITJoin(w http.ResponseWriter, r *http.Request) {
	var req offlinestore.PointInTimeJoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result, err := h.syncer.PointInTimeJoin(r.Context(), req)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *OfflineStoreSyncHandler) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs := h.syncer.ListJobs()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"jobs":  jobs,
		"total": len(jobs),
	})
}

func (h *OfflineStoreSyncHandler) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, err := h.syncer.GetJob(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, job)
}

func (h *OfflineStoreSyncHandler) handleSyncStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.syncer.Stats())
}

func (h *OfflineStoreSyncHandler) handleStoreStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.store.Stats())
}
