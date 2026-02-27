package server

import (
	"errors"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/extensions/materialization"
)

// MaterializationHandler provides HTTP endpoints for materialization pipelines.
type MaterializationHandler struct {
	engine *materialization.Engine
}

// NewMaterializationHandler creates a new materialization handler.
func NewMaterializationHandler(engine *materialization.Engine) *MaterializationHandler {
	return &MaterializationHandler{engine: engine}
}

// RegisterRoutes registers materialization API routes.
func (h *MaterializationHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/pipelines", h.handleListPipelines)
	mux.HandleFunc("POST /v1/pipelines", h.handleCreatePipeline)
	mux.HandleFunc("GET /v1/pipelines/{name}", h.handleGetPipeline)
	mux.HandleFunc("PUT /v1/pipelines/{name}", h.handleUpdatePipeline)
	mux.HandleFunc("DELETE /v1/pipelines/{name}", h.handleDeletePipeline)
	mux.HandleFunc("POST /v1/pipelines/{name}/execute", h.handleExecutePipeline)
	mux.HandleFunc("GET /v1/pipelines/{name}/runs", h.handleGetRuns)
	mux.HandleFunc("POST /v1/pipelines/{name}/backfill", h.handleBackfill)
}

func (h *MaterializationHandler) handleListPipelines(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "materialization engine not configured")
		return
	}
	pipelines := h.engine.ListPipelines()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"pipelines": pipelines,
		"count":     len(pipelines),
	})
}

func (h *MaterializationHandler) handleCreatePipeline(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "materialization engine not configured")
		return
	}

	var pipeline materialization.Pipeline
	if err := strictDecode(r.Body, &pipeline); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.engine.RegisterPipeline(&pipeline); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, materialization.ErrPipelineExists) {
			status = http.StatusConflict
		}
		writeJSONError(r.Context(), w, status, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":  true,
		"pipeline": pipeline,
	})
}

func (h *MaterializationHandler) handleGetPipeline(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "materialization engine not configured")
		return
	}

	name := r.PathValue("name")
	pipeline, err := h.engine.GetPipeline(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"pipeline": pipeline,
	})
}

func (h *MaterializationHandler) handleUpdatePipeline(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "materialization engine not configured")
		return
	}

	name := r.PathValue("name")
	var pipeline materialization.Pipeline
	if err := strictDecode(r.Body, &pipeline); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	pipeline.Name = name

	if err := h.engine.UpdatePipeline(&pipeline); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"pipeline": pipeline,
	})
}

func (h *MaterializationHandler) handleDeletePipeline(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "materialization engine not configured")
		return
	}

	name := r.PathValue("name")
	if err := h.engine.DeletePipeline(name); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "pipeline deleted",
	})
}

func (h *MaterializationHandler) handleExecutePipeline(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "materialization engine not configured")
		return
	}

	name := r.PathValue("name")
	run, err := h.engine.ExecutePipeline(r.Context(), name, materialization.TriggerManual)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, materialization.ErrPipelineNotFound) {
			status = http.StatusNotFound
		}
		writeJSONError(r.Context(), w, status, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"run":     run,
	})
}

func (h *MaterializationHandler) handleGetRuns(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "materialization engine not configured")
		return
	}

	name := r.PathValue("name")
	runs := h.engine.GetRuns(name)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"runs":    runs,
		"count":   len(runs),
	})
}

func (h *MaterializationHandler) handleBackfill(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "materialization engine not configured")
		return
	}

	name := r.PathValue("name")
	var req struct {
		Start    string `json:"start"`
		End      string `json:"end"`
		Interval string `json:"interval"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	start, err := time.Parse(time.RFC3339, req.Start)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid start time")
		return
	}
	end, err := time.Parse(time.RFC3339, req.End)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid end time")
		return
	}
	interval, err := time.ParseDuration(req.Interval)
	if err != nil {
		interval = time.Hour
	}

	runs, err := h.engine.Backfill(r.Context(), name, start, end, interval)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"runs":    runs,
		"count":   len(runs),
	})
}
