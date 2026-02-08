package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/streamdsl"
)

// StreamDSLHandler provides HTTP endpoints for the StreamDSL pipeline manager.
type StreamDSLHandler struct {
	manager *streamdsl.PipelineManager
}

// NewStreamDSLHandler creates a new StreamDSL handler.
func NewStreamDSLHandler(manager *streamdsl.PipelineManager) *StreamDSLHandler {
	return &StreamDSLHandler{manager: manager}
}

// RegisterRoutes registers StreamDSL API routes.
func (h *StreamDSLHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/streamdsl/compile", h.handleCompile)
	mux.HandleFunc("POST /v1/streamdsl/validate", h.handleValidate)
	mux.HandleFunc("GET /v1/streamdsl/pipelines", h.handleList)
	mux.HandleFunc("GET /v1/streamdsl/pipelines/{id}", h.handleGet)
	mux.HandleFunc("DELETE /v1/streamdsl/pipelines/{id}", h.handleDelete)
	mux.HandleFunc("GET /v1/streamdsl/stats", h.handleStats)
}

func (h *StreamDSLHandler) handleCompile(w http.ResponseWriter, r *http.Request) {
	var spec streamdsl.PipelineSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	plan, err := h.manager.Compile(spec)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusCreated, plan)
}

func (h *StreamDSLHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	var spec streamdsl.PipelineSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	errs := h.manager.Validate(spec)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"valid":  len(errs) == 0,
		"errors": errs,
	})
}

func (h *StreamDSLHandler) handleList(w http.ResponseWriter, r *http.Request) {
	pipelines := h.manager.List()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"pipelines": pipelines,
		"total":     len(pipelines),
	})
}

func (h *StreamDSLHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	plan, err := h.manager.Get(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, plan)
}

func (h *StreamDSLHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.manager.Delete(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"deleted": id})
}

func (h *StreamDSLHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
