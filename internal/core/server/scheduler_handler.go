package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/integrations/warehouse"
)

// SchedulerHandler handles scheduler API requests.
type SchedulerHandler struct {
	scheduler *warehouse.CronScheduler
}

// NewSchedulerHandler creates a new scheduler handler.
func NewSchedulerHandler(scheduler *warehouse.CronScheduler) *SchedulerHandler {
	return &SchedulerHandler{
		scheduler: scheduler,
	}
}

// RegisterRoutes registers scheduler API routes.
func (h *SchedulerHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/scheduler/jobs", h.handleListJobs)
	mux.HandleFunc("GET /v1/scheduler/jobs/{id}", h.handleGetJob)
	mux.HandleFunc("POST /v1/scheduler/jobs", h.handleCreateJob)
	mux.HandleFunc("DELETE /v1/scheduler/jobs/{id}", h.handleDeleteJob)
	mux.HandleFunc("POST /v1/scheduler/jobs/{id}/enable", h.handleEnableJob)
	mux.HandleFunc("POST /v1/scheduler/jobs/{id}/disable", h.handleDisableJob)
	mux.HandleFunc("POST /v1/scheduler/jobs/{id}/trigger", h.handleTriggerJob)
	mux.HandleFunc("GET /v1/scheduler/status", h.handleGetStatus)
	mux.HandleFunc("POST /v1/scheduler/start", h.handleStart)
	mux.HandleFunc("POST /v1/scheduler/stop", h.handleStop)
}

// ScheduleJobJSON represents a scheduled job in JSON format.
type ScheduleJobJSON struct {
	JobID         string `json:"job_id"`
	ConnectorName string `json:"connector_name"`
	Schedule      string `json:"schedule"`
	NextRun       string `json:"next_run,omitempty"`
	LastRun       string `json:"last_run,omitempty"`
	Enabled       bool   `json:"enabled"`
	RetryCount    int    `json:"retry_count"`
	MaxRetries    int    `json:"max_retries"`
	LastError     string `json:"last_error,omitempty"`
}

// ScheduleJobRequest represents a request to create a scheduled job.
type ScheduleJobRequest struct {
	JobID         string `json:"job_id"`
	ConnectorName string `json:"connector_name"`
	Schedule      string `json:"schedule"`
	MaxRetries    int    `json:"max_retries"`
}

// handleListJobs handles GET /v1/scheduler/jobs
func (h *SchedulerHandler) handleListJobs(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "scheduler not configured")
		return
	}

	entries := h.scheduler.ListEntries()
	jobs := make([]ScheduleJobJSON, len(entries))

	for i, entry := range entries {
		jobs[i] = h.entryToJSON(entry)
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"jobs":  jobs,
		"count": len(jobs),
	})
}

// handleGetJob handles GET /v1/scheduler/jobs/{id}
func (h *SchedulerHandler) handleGetJob(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "scheduler not configured")
		return
	}

	jobID := r.PathValue("id")
	if jobID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "job ID required")
		return
	}

	entry, err := h.scheduler.GetEntry(jobID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, h.entryToJSON(entry))
}

// handleCreateJob handles POST /v1/scheduler/jobs
func (h *SchedulerHandler) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "scheduler not configured")
		return
	}

	var req ScheduleJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.JobID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "job_id required")
		return
	}
	if req.ConnectorName == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "connector_name required")
		return
	}
	if req.Schedule == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "schedule required")
		return
	}

	err := h.scheduler.Schedule(req.JobID, req.ConnectorName, req.Schedule, req.MaxRetries)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	entry, _ := h.scheduler.GetEntry(req.JobID)
	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"job":     h.entryToJSON(entry),
	})
}

// handleDeleteJob handles DELETE /v1/scheduler/jobs/{id}
func (h *SchedulerHandler) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "scheduler not configured")
		return
	}

	jobID := r.PathValue("id")
	if jobID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "job ID required")
		return
	}

	err := h.scheduler.Unschedule(jobID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleEnableJob handles POST /v1/scheduler/jobs/{id}/enable
func (h *SchedulerHandler) handleEnableJob(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "scheduler not configured")
		return
	}

	jobID := r.PathValue("id")
	if jobID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "job ID required")
		return
	}

	err := h.scheduler.Enable(jobID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	entry, _ := h.scheduler.GetEntry(jobID)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"job":     h.entryToJSON(entry),
	})
}

// handleDisableJob handles POST /v1/scheduler/jobs/{id}/disable
func (h *SchedulerHandler) handleDisableJob(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "scheduler not configured")
		return
	}

	jobID := r.PathValue("id")
	if jobID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "job ID required")
		return
	}

	err := h.scheduler.Disable(jobID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	entry, _ := h.scheduler.GetEntry(jobID)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"job":     h.entryToJSON(entry),
	})
}

// handleTriggerJob handles POST /v1/scheduler/jobs/{id}/trigger
func (h *SchedulerHandler) handleTriggerJob(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "scheduler not configured")
		return
	}

	jobID := r.PathValue("id")
	if jobID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "job ID required")
		return
	}

	err := h.scheduler.TriggerNow(r.Context(), jobID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusAccepted, map[string]interface{}{
		"success": true,
		"message": "job triggered",
	})
}

// handleGetStatus handles GET /v1/scheduler/status
func (h *SchedulerHandler) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "scheduler not configured")
		return
	}

	entries := h.scheduler.ListEntries()
	enabledCount := 0
	for _, entry := range entries {
		if entry.Enabled {
			enabledCount++
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"running":       h.scheduler.IsRunning(),
		"total_jobs":    len(entries),
		"enabled_jobs":  enabledCount,
		"disabled_jobs": len(entries) - enabledCount,
	})
}

// handleStart handles POST /v1/scheduler/start
func (h *SchedulerHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "scheduler not configured")
		return
	}

	err := h.scheduler.Start(r.Context())
	if err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"running": true,
	})
}

// handleStop handles POST /v1/scheduler/stop
func (h *SchedulerHandler) handleStop(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "scheduler not configured")
		return
	}

	err := h.scheduler.Stop()
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"running": false,
	})
}

func (h *SchedulerHandler) entryToJSON(entry *warehouse.ScheduleEntry) ScheduleJobJSON {
	job := ScheduleJobJSON{
		JobID:         entry.JobID,
		ConnectorName: entry.ConnectorName,
		Schedule:      entry.Schedule.String(),
		Enabled:       entry.Enabled,
		RetryCount:    entry.RetryCount,
		MaxRetries:    entry.MaxRetries,
		LastError:     entry.LastError,
	}

	if !entry.NextRun.IsZero() {
		job.NextRun = entry.NextRun.Format("2006-01-02T15:04:05Z07:00")
	}
	if !entry.LastRun.IsZero() {
		job.LastRun = entry.LastRun.Format("2006-01-02T15:04:05Z07:00")
	}

	return job
}

func (h *SchedulerHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *SchedulerHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	h.writeJSON(ctx, w, status, map[string]interface{}{
		"error": message,
	})
}
