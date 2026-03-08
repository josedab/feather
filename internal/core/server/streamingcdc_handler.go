package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/extensions/streamingcdc"
)

// StreamingCDCHandler handles streaming CDC pipeline API requests.
type StreamingCDCHandler struct {
	manager *streamingcdc.Manager
}

// NewStreamingCDCHandler creates a new streaming CDC handler.
func NewStreamingCDCHandler(manager *streamingcdc.Manager) *StreamingCDCHandler {
	return &StreamingCDCHandler{manager: manager}
}

// RegisterRoutes registers streaming CDC API routes.
func (h *StreamingCDCHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/streaming/pipelines", h.handleListPipelines)
	mux.HandleFunc("POST /v1/streaming/pipelines", h.handleCreatePipeline)
	mux.HandleFunc("GET /v1/streaming/pipelines/{id}", h.handleGetPipeline)
	mux.HandleFunc("DELETE /v1/streaming/pipelines/{id}", h.handleDeletePipeline)
	mux.HandleFunc("POST /v1/streaming/pipelines/{id}/start", h.handleStartPipeline)
	mux.HandleFunc("POST /v1/streaming/pipelines/{id}/stop", h.handleStopPipeline)
	mux.HandleFunc("POST /v1/streaming/pipelines/{id}/ingest", h.handleIngest)
	mux.HandleFunc("POST /v1/streaming/pipelines/{id}/ingest/batch", h.handleIngestBatch)
	mux.HandleFunc("GET /v1/streaming/pipelines/{id}/watermarks", h.handleGetWatermarks)
	mux.HandleFunc("GET /v1/streaming/stats", h.handleStats)
}

func (h *StreamingCDCHandler) handleListPipelines(w http.ResponseWriter, r *http.Request) {
	pipelines := h.manager.ListPipelines()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"pipelines": pipelines,
		"total":     len(pipelines),
	})
}

func (h *StreamingCDCHandler) handleCreatePipeline(w http.ResponseWriter, r *http.Request) {
	var config streamingcdc.PipelineConfig
	if err := strictDecode(r.Body, &config); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	info, err := h.manager.CreatePipeline(config)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, info)
}

func (h *StreamingCDCHandler) handleGetPipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	info, err := h.manager.GetPipeline(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, info)
}

func (h *StreamingCDCHandler) handleDeletePipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.manager.DeletePipeline(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "pipeline deleted"})
}

func (h *StreamingCDCHandler) handleStartPipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.manager.StartPipeline(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "pipeline started"})
}

func (h *StreamingCDCHandler) handleStopPipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.manager.StopPipeline(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "pipeline stopped"})
}

func (h *StreamingCDCHandler) handleIngest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var record streamingcdc.ChangeRecord
	if err := strictDecode(r.Body, &record); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.manager.IngestRecord(id, record); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "record ingested"})
}

func (h *StreamingCDCHandler) handleIngestBatch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Records []streamingcdc.ChangeRecord `json:"records"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	ingested, dropped, err := h.manager.IngestBatch(id, req.Records)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"ingested": ingested,
		"dropped":  dropped,
		"total":    len(req.Records),
	})
}

func (h *StreamingCDCHandler) handleGetWatermarks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	watermarks, err := h.manager.GetWatermarks(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"watermarks": watermarks,
	})
}

func (h *StreamingCDCHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.manager.Stats())
}
