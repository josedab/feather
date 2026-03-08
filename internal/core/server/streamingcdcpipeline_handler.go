package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/extensions/incrmat"
)

// StreamingCDCPipelineHandler provides enhanced CDC pipeline endpoints
// that integrate with the existing incrmat CDC infrastructure.
type StreamingCDCPipelineHandler struct {
	cdcManager *incrmat.CDCManager
	engine     *incrmat.Engine
	recovery   *incrmat.RecoveryManager
}

// NewStreamingCDCPipelineHandler creates a new handler.
func NewStreamingCDCPipelineHandler(cdcManager *incrmat.CDCManager, engine *incrmat.Engine, recovery *incrmat.RecoveryManager) *StreamingCDCPipelineHandler {
	return &StreamingCDCPipelineHandler{
		cdcManager: cdcManager,
		engine:     engine,
		recovery:   recovery,
	}
}

// RegisterRoutes registers streaming CDC pipeline routes.
func (h *StreamingCDCPipelineHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/streaming/cdc/sources", h.handleListSources)
	mux.HandleFunc("POST /v1/streaming/cdc/sources", h.handleRegisterSource)
	mux.HandleFunc("POST /v1/streaming/cdc/events", h.handleProcessEvent)
	mux.HandleFunc("POST /v1/streaming/cdc/events/batch", h.handleProcessBatch)
	mux.HandleFunc("POST /v1/streaming/cdc/materialize", h.handleMaterialize)
	mux.HandleFunc("POST /v1/streaming/cdc/checkpoint/{source}", h.handleCheckpoint)
	mux.HandleFunc("POST /v1/streaming/cdc/recover/{source}", h.handleRecover)
	mux.HandleFunc("GET /v1/streaming/cdc/checkpoints", h.handleListCheckpoints)
	mux.HandleFunc("GET /v1/streaming/cdc/dirty", h.handleDirtyNodes)
	mux.HandleFunc("GET /v1/streaming/cdc/stats", h.handleStats)
}

func (h *StreamingCDCPipelineHandler) handleListSources(w http.ResponseWriter, r *http.Request) {
	sources := h.cdcManager.ListSources()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"sources": sources,
		"total":   len(sources),
	})
}

func (h *StreamingCDCPipelineHandler) handleRegisterSource(w http.ResponseWriter, r *http.Request) {
	var src incrmat.CDCSourceConfig
	if err := strictDecode(r.Body, &src); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.cdcManager.RegisterSource(src); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "source registered"})
}

func (h *StreamingCDCPipelineHandler) handleProcessEvent(w http.ResponseWriter, r *http.Request) {
	var event incrmat.CDCEvent
	if err := strictDecode(r.Body, &event); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.cdcManager.ProcessCDCEvent(event); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "event processed"})
}

func (h *StreamingCDCPipelineHandler) handleProcessBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Events []incrmat.CDCEvent `json:"events"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	processed, errCount, _ := h.cdcManager.ProcessBatch(req.Events)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"processed": processed,
		"errors":    errCount,
		"total":     len(req.Events),
	})
}

func (h *StreamingCDCPipelineHandler) handleMaterialize(w http.ResponseWriter, r *http.Request) {
	results := h.engine.Materialize()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results":      results,
		"materialized": len(results),
	})
}

func (h *StreamingCDCPipelineHandler) handleCheckpoint(w http.ResponseWriter, r *http.Request) {
	source := r.PathValue("source")
	cp, err := h.recovery.Checkpoint(source)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, cp)
}

func (h *StreamingCDCPipelineHandler) handleRecover(w http.ResponseWriter, r *http.Request) {
	source := r.PathValue("source")
	cp, err := h.recovery.RecoverFrom(source)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"recovered":  true,
		"checkpoint": cp,
	})
}

func (h *StreamingCDCPipelineHandler) handleListCheckpoints(w http.ResponseWriter, r *http.Request) {
	checkpoints := h.recovery.ListAllCheckpoints()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"checkpoints": checkpoints,
		"total":       len(checkpoints),
	})
}

func (h *StreamingCDCPipelineHandler) handleDirtyNodes(w http.ResponseWriter, r *http.Request) {
	dirty := h.engine.GetDirtyNodes()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"dirty_nodes": dirty,
		"total":       len(dirty),
	})
}

func (h *StreamingCDCPipelineHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"cdc":    h.cdcManager.Stats(),
		"engine": h.engine.Stats(),
	})
}
