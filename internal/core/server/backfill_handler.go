package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
	"github.com/feather-store/feather/internal/tools/backfill"
)

// BackfillHandler handles backfill API requests.
type BackfillHandler struct {
	manager *backfill.Manager
}

// storeWriter adapts storage.Store to backfill.FeatureWriter
type storeWriter struct {
	store *storage.Store
}

func (w *storeWriter) WriteFeature(ctx context.Context, entityID string, feature string, value interface{}, timestamp time.Time) error {
	fv := &domain.FeatureValue{
		Value:     value,
		Timestamp: timestamp.UnixNano(),
	}
	return w.store.Put(ctx, entityID, map[string]*domain.FeatureValue{feature: fv})
}

func (w *storeWriter) WriteBatch(ctx context.Context, records []backfill.FeatureRecord) error {
	// Group by entity
	byEntity := make(map[string]map[string]*domain.FeatureValue)
	for _, r := range records {
		if byEntity[r.EntityID] == nil {
			byEntity[r.EntityID] = make(map[string]*domain.FeatureValue)
		}
		byEntity[r.EntityID][r.Feature] = &domain.FeatureValue{
			Value:     r.Value,
			Timestamp: r.Timestamp.UnixNano(),
		}
	}

	// Write each entity
	for entityID, features := range byEntity {
		if err := w.store.Put(ctx, entityID, features); err != nil {
			return err
		}
	}

	return nil
}

// NewBackfillHandler creates a new backfill handler.
func NewBackfillHandler(store *storage.Store) *BackfillHandler {
	writer := &storeWriter{store: store}
	return &BackfillHandler{
		manager: backfill.NewManager(writer),
	}
}

// RegisterRoutes registers backfill API routes.
func (h *BackfillHandler) RegisterRoutes(mux *http.ServeMux) {
	// Job management
	mux.HandleFunc("POST /v1/backfill/jobs", h.handleCreateJob)
	mux.HandleFunc("GET /v1/backfill/jobs", h.handleListJobs)
	mux.HandleFunc("GET /v1/backfill/jobs/{id}", h.handleGetJob)
	mux.HandleFunc("DELETE /v1/backfill/jobs/{id}", h.handleDeleteJob)

	// Job control
	mux.HandleFunc("POST /v1/backfill/jobs/{id}/start", h.handleStartJob)
	mux.HandleFunc("POST /v1/backfill/jobs/{id}/pause", h.handlePauseJob)
	mux.HandleFunc("POST /v1/backfill/jobs/{id}/resume", h.handleResumeJob)
	mux.HandleFunc("POST /v1/backfill/jobs/{id}/cancel", h.handleCancelJob)

	// Checkpoints
	mux.HandleFunc("GET /v1/backfill/jobs/{id}/checkpoint", h.handleGetCheckpoint)

	// Stats and export
	mux.HandleFunc("GET /v1/backfill/stats", h.handleGetStats)
	mux.HandleFunc("GET /v1/backfill/jobs/{id}/export", h.handleExportJob)
	mux.HandleFunc("POST /v1/backfill/import", h.handleImportJob)
}

// GetManager returns the backfill manager for integration.
func (h *BackfillHandler) GetManager() *backfill.Manager {
	return h.manager
}

// CreateJobRequest represents a request to create a backfill job.
type CreateJobRequest struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Source      backfill.DataSource    `json:"source"`
	Features    []string               `json:"features"`
	EntityType  string                 `json:"entity_type"`
	StartTime   string                 `json:"start_time"`
	EndTime     string                 `json:"end_time"`
	Config      *backfill.JobConfig    `json:"config,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// handleCreateJob handles POST /v1/backfill/jobs
func (h *BackfillHandler) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" || len(req.Features) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "id and features are required")
		return
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid start_time format")
		return
	}

	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid end_time format")
		return
	}

	createdBy := r.Header.Get("X-User-ID")
	if createdBy == "" {
		createdBy = "anonymous"
	}

	job := &backfill.Job{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		Source:      req.Source,
		Features:    req.Features,
		EntityType:  req.EntityType,
		StartTime:   startTime,
		EndTime:     endTime,
		Metadata:    req.Metadata,
		CreatedBy:   createdBy,
	}

	if req.Config != nil {
		job.Config = *req.Config
	}

	if err := h.manager.CreateJob(job); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"job":     job,
	})
}

// handleListJobs handles GET /v1/backfill/jobs
func (h *BackfillHandler) handleListJobs(w http.ResponseWriter, r *http.Request) {
	status := backfill.JobStatus(r.URL.Query().Get("status"))
	jobs := h.manager.ListJobs(status)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"jobs":  jobs,
		"count": len(jobs),
	})
}

// handleGetJob handles GET /v1/backfill/jobs/{id}
func (h *BackfillHandler) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "job id required")
		return
	}

	job := h.manager.GetJob(id)
	if job == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "job not found")
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, job)
}

// handleDeleteJob handles DELETE /v1/backfill/jobs/{id}
func (h *BackfillHandler) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "job id required")
		return
	}

	if err := h.manager.DeleteJob(id); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
	})
}

// handleStartJob handles POST /v1/backfill/jobs/{id}/start
func (h *BackfillHandler) handleStartJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "job id required")
		return
	}

	if err := h.manager.StartJob(r.Context(), id); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
		"status":  "running",
	})
}

// handlePauseJob handles POST /v1/backfill/jobs/{id}/pause
func (h *BackfillHandler) handlePauseJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "job id required")
		return
	}

	if err := h.manager.PauseJob(id); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
		"status":  "paused",
	})
}

// handleResumeJob handles POST /v1/backfill/jobs/{id}/resume
func (h *BackfillHandler) handleResumeJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "job id required")
		return
	}

	if err := h.manager.ResumeJob(r.Context(), id); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
		"status":  "running",
	})
}

// handleCancelJob handles POST /v1/backfill/jobs/{id}/cancel
func (h *BackfillHandler) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "job id required")
		return
	}

	if err := h.manager.CancelJob(id); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
		"status":  "canceled",
	})
}

// handleGetCheckpoint handles GET /v1/backfill/jobs/{id}/checkpoint
func (h *BackfillHandler) handleGetCheckpoint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "job id required")
		return
	}

	checkpoint := h.manager.GetCheckpoint(id)
	if checkpoint == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "checkpoint not found")
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, checkpoint)
}

// handleGetStats handles GET /v1/backfill/stats
func (h *BackfillHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

// handleExportJob handles GET /v1/backfill/jobs/{id}/export
func (h *BackfillHandler) handleExportJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "job id required")
		return
	}

	data, err := h.manager.ExportJob(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=backfill_job_"+id+".json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
	}
}

// handleImportJob handles POST /v1/backfill/import
func (h *BackfillHandler) handleImportJob(w http.ResponseWriter, r *http.Request) {
	var job backfill.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.manager.CreateJob(&job); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"job":     &job,
	})
}

func (h *BackfillHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *BackfillHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
