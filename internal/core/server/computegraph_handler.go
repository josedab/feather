package server

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/computegraph"
)

// ComputeGraphHandler provides HTTP endpoints for the feature compute graph.
type ComputeGraphHandler struct {
	engine      *computegraph.Engine
	incremental *computegraph.IncrementalEngine
}

// NewComputeGraphHandler creates a new compute graph handler.
func NewComputeGraphHandler(engine *computegraph.Engine) *ComputeGraphHandler {
	return &ComputeGraphHandler{
		engine:      engine,
		incremental: computegraph.NewIncrementalEngine(engine, computegraph.DefaultIncrementalConfig()),
	}
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
	mux.HandleFunc("POST /v1/graph/compute-parallel", h.handleComputeParallel)
	mux.HandleFunc("POST /v1/graph/invalidate/{name}", h.handleInvalidate)
	mux.HandleFunc("GET /v1/graph/topology", h.handleTopologicalSort)
	mux.HandleFunc("GET /v1/graph/dag", h.handleGetDAG)
	mux.HandleFunc("GET /v1/graph/validate", h.handleValidate)
	mux.HandleFunc("GET /v1/graph/stats", h.handleStats)
	mux.HandleFunc("POST /v1/graph/dsl/parse", h.handleParseDSL)
	mux.HandleFunc("POST /v1/graph/dsl/apply", h.handleApplyDSL)
	mux.HandleFunc("POST /v1/graph/definition/apply", h.handleApplyDefinition)
	mux.HandleFunc("POST /v1/graph/propagate/{name}", h.handlePropagate)
	mux.HandleFunc("GET /v1/graph/changelog", h.handleGetChangelog)
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
	stats := h.incremental.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}

func (h *ComputeGraphHandler) handleComputeParallel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Nodes  []string               `json:"nodes"`
		Inputs map[string]interface{} `json:"inputs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if len(req.Nodes) == 0 {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "nodes list must not be empty")
		return
	}

	results, err := h.engine.ComputeParallel(req.Nodes, req.Inputs)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, results)
}

func (h *ComputeGraphHandler) handleParseDSL(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "failed to read body: "+err.Error())
		return
	}

	def, err := computegraph.ParseDSL(string(body))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, def)
}

func (h *ComputeGraphHandler) handleApplyDSL(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "failed to read body: "+err.Error())
		return
	}

	def, err := computegraph.ParseDSL(string(body))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.engine.ApplyDefinition(def)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	status := http.StatusOK
	if !result.Success {
		status = http.StatusMultiStatus
	}
	writeJSONResponse(r.Context(), w, status, result)
}

func (h *ComputeGraphHandler) handleApplyDefinition(w http.ResponseWriter, r *http.Request) {
	var def computegraph.GraphDefinition
	if err := json.NewDecoder(r.Body).Decode(&def); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	result, err := h.engine.ApplyDefinition(&def)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	status := http.StatusOK
	if !result.Success {
		status = http.StatusMultiStatus
	}
	writeJSONResponse(r.Context(), w, status, result)
}

func (h *ComputeGraphHandler) handlePropagate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Value  interface{}            `json:"value"`
		Inputs map[string]interface{} `json:"inputs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Inputs == nil {
		req.Inputs = make(map[string]interface{})
	}

	recomputed, err := h.incremental.PropagateChange(name, req.Value, req.Inputs)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"source":     name,
		"recomputed": recomputed,
	})
}

func (h *ComputeGraphHandler) handleGetChangelog(w http.ResponseWriter, r *http.Request) {
	nodeName := r.URL.Query().Get("node")
	log := h.incremental.GetChangeLog(nodeName, 100)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"changes": log,
		"total":   len(log),
	})
}
