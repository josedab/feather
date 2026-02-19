package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/incrmat"
)

// IncrMatHandler handles incremental materialization API requests.
type IncrMatHandler struct {
	engine *incrmat.Engine
}

// NewIncrMatHandler creates a new incremental materialization handler.
func NewIncrMatHandler(engine *incrmat.Engine) *IncrMatHandler {
	return &IncrMatHandler{
		engine: engine,
	}
}

// RegisterRoutes registers incremental materialization API routes.
func (h *IncrMatHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/materialization/nodes", h.handleListNodes)
	mux.HandleFunc("POST /v1/materialization/nodes", h.handleRegisterNode)
	mux.HandleFunc("DELETE /v1/materialization/nodes/{id}", h.handleRemoveNode)
	mux.HandleFunc("POST /v1/materialization/changes", h.handleRecordChange)
	mux.HandleFunc("GET /v1/materialization/dirty", h.handleGetDirtyNodes)
	mux.HandleFunc("POST /v1/materialization/run", h.handleMaterialize)
	mux.HandleFunc("GET /v1/materialization/results", h.handleGetResults)
	mux.HandleFunc("GET /v1/materialization/incr/stats", h.handleGetStats)
}

// handleListNodes handles GET /v1/materialization/nodes
func (h *IncrMatHandler) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes := h.engine.ListNodes()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"nodes": nodes,
	})
}

// handleRegisterNode handles POST /v1/materialization/nodes
func (h *IncrMatHandler) handleRegisterNode(w http.ResponseWriter, r *http.Request) {
	var node incrmat.MaterializationNode
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.engine.RegisterNode(node); err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "node registered"})
}

// handleRemoveNode handles DELETE /v1/materialization/nodes/{id}
func (h *IncrMatHandler) handleRemoveNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "node id required")
		return
	}

	if err := h.engine.RemoveNode(id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "node removed"})
}

// handleRecordChange handles POST /v1/materialization/changes
func (h *IncrMatHandler) handleRecordChange(w http.ResponseWriter, r *http.Request) {
	var event incrmat.ChangeEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.engine.RecordChange(event)

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "change recorded"})
}

// handleGetDirtyNodes handles GET /v1/materialization/dirty
func (h *IncrMatHandler) handleGetDirtyNodes(w http.ResponseWriter, r *http.Request) {
	dirty := h.engine.GetDirtyNodes()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"dirty_nodes": dirty,
	})
}

// handleMaterialize handles POST /v1/materialization/run
func (h *IncrMatHandler) handleMaterialize(w http.ResponseWriter, r *http.Request) {
	results := h.engine.Materialize()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
	})
}

// handleGetResults handles GET /v1/materialization/results
func (h *IncrMatHandler) handleGetResults(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	results := h.engine.GetResults(limit)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
	})
}

// handleGetStats handles GET /v1/materialization/incr/stats
func (h *IncrMatHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.engine.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *IncrMatHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *IncrMatHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
