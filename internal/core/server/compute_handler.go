package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/tools/compute"
)

// ComputeHandler handles feature computation API requests.
type ComputeHandler struct {
	engine *compute.ComputeEngine
}

// NewComputeHandler creates a new compute handler.
func NewComputeHandler(engine *compute.ComputeEngine) *ComputeHandler {
	return &ComputeHandler{engine: engine}
}

// RegisterRoutes registers compute API routes.
func (h *ComputeHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/compute/definitions", h.handleListDefinitions)
	mux.HandleFunc("POST /v1/compute/definitions", h.handleDefineFeature)
	mux.HandleFunc("GET /v1/compute/definitions/{name}", h.handleGetDefinition)
	mux.HandleFunc("DELETE /v1/compute/definitions/{name}", h.handleDeleteDefinition)
	mux.HandleFunc("POST /v1/compute/execute", h.handleExecute)
	mux.HandleFunc("POST /v1/compute/execute/batch", h.handleBatchExecute)
	mux.HandleFunc("GET /v1/compute/stats", h.handleStats)
	mux.HandleFunc("GET /v1/compute/lineage/{name}", h.handleGetLineage)
}

func (h *ComputeHandler) handleListDefinitions(w http.ResponseWriter, r *http.Request) {
	defs := h.engine.List(r.Context())
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"definitions": defs})
}

func (h *ComputeHandler) handleDefineFeature(w http.ResponseWriter, r *http.Request) {
	var def compute.FeatureDefinition
	if err := json.NewDecoder(r.Body).Decode(&def); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.engine.Define(r.Context(), &def); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "name": def.Name})
}

func (h *ComputeHandler) handleGetDefinition(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "name required")
		return
	}
	def, err := h.engine.Get(r.Context(), name)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, def)
}

func (h *ComputeHandler) handleDeleteDefinition(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "name required")
		return
	}
	if err := h.engine.Undefine(r.Context(), name); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

type computeExecuteRequest struct {
	Name   string                 `json:"name"`
	Inputs map[string]interface{} `json:"inputs"`
}

func (h *ComputeHandler) handleExecute(w http.ResponseWriter, r *http.Request) {
	var req computeExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.engine.Compute(r.Context(), req.Name, req.Inputs)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

type computeBatchRequest struct {
	Name     string                   `json:"name"`
	Entities []map[string]interface{} `json:"entities"`
}

func (h *ComputeHandler) handleBatchExecute(w http.ResponseWriter, r *http.Request) {
	var req computeBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	results, err := h.engine.ComputeBatch(r.Context(), req.Name, req.Entities)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"results": results})
}

func (h *ComputeHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.engine.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *ComputeHandler) handleGetLineage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "name required")
		return
	}
	lineage, err := h.engine.GetLineage(r.Context(), name)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, lineage)
}

func (h *ComputeHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *ComputeHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
