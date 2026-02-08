package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/computegraph"
)

// ComputeGraphHandler provides HTTP endpoints for the feature compute graph.
type ComputeGraphHandler struct {
	engine *computegraph.Engine
}

// NewComputeGraphHandler creates a new compute graph handler.
func NewComputeGraphHandler(engine *computegraph.Engine) *ComputeGraphHandler {
	return &ComputeGraphHandler{engine: engine}
}

// RegisterRoutes registers compute graph API routes.
func (h *ComputeGraphHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/graph/nodes", h.handleAddNode)
	mux.HandleFunc("GET /v1/graph/nodes", h.handleListNodes)
	mux.HandleFunc("GET /v1/graph/nodes/{name}", h.handleGetNode)
	mux.HandleFunc("DELETE /v1/graph/nodes/{name}", h.handleRemoveNode)
	mux.HandleFunc("GET /v1/graph/nodes/{name}/upstream", h.handleGetUpstream)
	mux.HandleFunc("GET /v1/graph/nodes/{name}/downstream", h.handleGetDownstream)
	mux.HandleFunc("POST /v1/graph/compute/{name}", h.handleCompute)
	mux.HandleFunc("POST /v1/graph/invalidate/{name}", h.handleInvalidate)
	mux.HandleFunc("GET /v1/graph/topology", h.handleTopologicalSort)
	mux.HandleFunc("GET /v1/graph/dag", h.handleGetDAG)
	mux.HandleFunc("GET /v1/graph/validate", h.handleValidate)
	mux.HandleFunc("GET /v1/graph/stats", h.handleStats)
}

func (h *ComputeGraphHandler) handleAddNode(w http.ResponseWriter, r *http.Request) {
	var node computegraph.FeatureNode
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.engine.AddNode(node); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, node)
}

func (h *ComputeGraphHandler) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes := h.engine.ListNodes()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"nodes": nodes,
		"total": len(nodes),
	})
}

func (h *ComputeGraphHandler) handleGetNode(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	node, err := h.engine.GetNode(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, node)
}

func (h *ComputeGraphHandler) handleRemoveNode(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.engine.RemoveNode(name); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"deleted": name})
}

func (h *ComputeGraphHandler) handleGetUpstream(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	upstream, err := h.engine.GetUpstream(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"node":     name,
		"upstream": upstream,
	})
}

func (h *ComputeGraphHandler) handleGetDownstream(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	downstream, err := h.engine.GetDownstream(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"node":       name,
		"downstream": downstream,
	})
}

func (h *ComputeGraphHandler) handleCompute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var req struct {
		Inputs map[string]interface{} `json:"inputs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	result, err := h.engine.Compute(name, req.Inputs)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *ComputeGraphHandler) handleInvalidate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	count := h.engine.Invalidate(name)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"invalidated": count,
		"node":        name,
	})
}

func (h *ComputeGraphHandler) handleTopologicalSort(w http.ResponseWriter, r *http.Request) {
	order, err := h.engine.TopologicalSort()
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, order)
}

func (h *ComputeGraphHandler) handleGetDAG(w http.ResponseWriter, r *http.Request) {
	dag := h.engine.GetDAG()
	writeJSONResponse(r.Context(), w, http.StatusOK, dag)
}

func (h *ComputeGraphHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	errors := h.engine.Validate()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"errors": errors,
		"valid":  len(errors) == 0,
	})
}

func (h *ComputeGraphHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.engine.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
