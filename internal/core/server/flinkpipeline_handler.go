package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/feather-store/feather/internal/integrations/flinkpipeline"
)

// FlinkPipelineHandler handles streaming pipeline API requests.
type FlinkPipelineHandler struct {
	manager *flinkpipeline.Manager
}

// NewFlinkPipelineHandler creates a new streaming pipeline handler.
func NewFlinkPipelineHandler(manager *flinkpipeline.Manager) *FlinkPipelineHandler {
	return &FlinkPipelineHandler{manager: manager}
}

// RegisterRoutes registers streaming pipeline API routes.
func (h *FlinkPipelineHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/pipelines/streaming", h.handleList)
	mux.HandleFunc("POST /v1/pipelines/streaming", h.handleCreate)
	mux.HandleFunc("GET /v1/pipelines/streaming/{id}", h.handleGet)
	mux.HandleFunc("DELETE /v1/pipelines/streaming/{id}", h.handleDelete)
	mux.HandleFunc("POST /v1/pipelines/streaming/{id}/start", h.handleStart)
	mux.HandleFunc("POST /v1/pipelines/streaming/{id}/stop", h.handleStop)
	mux.HandleFunc("GET /v1/pipelines/streaming/{id}/stats", h.handleStats)
	mux.HandleFunc("POST /v1/pipelines/streaming/{id}/ingest", h.handleIngest)
	mux.HandleFunc("GET /v1/pipelines/streaming/stats", h.handleManagerStats)
}

func (h *FlinkPipelineHandler) handleList(w http.ResponseWriter, r *http.Request) {
	pipelines := h.manager.ListPipelines()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"pipelines": pipelines,
		"count":     len(pipelines),
	})
}

func (h *FlinkPipelineHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var p flinkpipeline.Pipeline
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.manager.CreatePipeline(p)
	if err != nil {
		if errors.Is(err, flinkpipeline.ErrPipelineExists) {
			h.writeError(r.Context(), w, http.StatusConflict, "pipeline already exists")
			return
		}
		if errors.Is(err, flinkpipeline.ErrInvalidPipeline) {
			h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, created)
}

func (h *FlinkPipelineHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := h.manager.GetPipeline(id)
	if err != nil {
		if errors.Is(err, flinkpipeline.ErrPipelineNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "pipeline not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, p)
}

func (h *FlinkPipelineHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.manager.DeletePipeline(id); err != nil {
		if errors.Is(err, flinkpipeline.ErrPipelineNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "pipeline not found")
			return
		}
		if errors.Is(err, flinkpipeline.ErrPipelineRunning) {
			h.writeError(r.Context(), w, http.StatusConflict, "pipeline is running; stop it first")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "pipeline deleted"})
}

func (h *FlinkPipelineHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.manager.StartPipeline(id); err != nil {
		if errors.Is(err, flinkpipeline.ErrPipelineNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "pipeline not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "pipeline started"})
}

func (h *FlinkPipelineHandler) handleStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.manager.StopPipeline(id); err != nil {
		if errors.Is(err, flinkpipeline.ErrPipelineNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "pipeline not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "pipeline stopped"})
}

func (h *FlinkPipelineHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stats, err := h.manager.GetPipelineStats(id)
	if err != nil {
		if errors.Is(err, flinkpipeline.ErrPipelineNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "pipeline not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *FlinkPipelineHandler) handleIngest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var event map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.manager.IngestEvent(id, event); err != nil {
		if errors.Is(err, flinkpipeline.ErrPipelineNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "pipeline not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "event ingested"})
}

func (h *FlinkPipelineHandler) handleManagerStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *FlinkPipelineHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *FlinkPipelineHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
