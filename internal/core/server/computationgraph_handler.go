package server

import (
	"io"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/computegraph"
)

// ComputationGraphHandler handles declarative computation graph API requests.
type ComputationGraphHandler struct {
	engine   *computegraph.Engine
	memoizer *computegraph.Memoizer
}

// NewComputationGraphHandler creates a new computation graph handler.
func NewComputationGraphHandler(engine *computegraph.Engine, memoizer *computegraph.Memoizer) *ComputationGraphHandler {
	return &ComputationGraphHandler{engine: engine, memoizer: memoizer}
}

// RegisterRoutes registers computation graph API routes.
func (h *ComputationGraphHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/computation/nodes", h.handleAddNode)
	mux.HandleFunc("GET /v1/computation/nodes", h.handleListNodes)
	mux.HandleFunc("GET /v1/computation/nodes/{name}", h.handleGetNode)
	mux.HandleFunc("DELETE /v1/computation/nodes/{name}", h.handleRemoveNode)
	mux.HandleFunc("POST /v1/computation/execute/{name}", h.handleExecute)
	mux.HandleFunc("POST /v1/computation/execute-batch", h.handleExecuteBatch)
	mux.HandleFunc("GET /v1/computation/dag", h.handleGetDAG)
	mux.HandleFunc("GET /v1/computation/topology", h.handleTopology)
	mux.HandleFunc("POST /v1/computation/dsl/parse", h.handleParseDSL)
	mux.HandleFunc("POST /v1/computation/dsl/apply", h.handleApplyDSL)
	mux.HandleFunc("GET /v1/computation/validate", h.handleValidate)
	mux.HandleFunc("GET /v1/computation/stats", h.handleStats)
}

func (h *ComputationGraphHandler) handleAddNode(w http.ResponseWriter, r *http.Request) {
	var node computegraph.FeatureNode
	if err := strictDecode(r.Body, &node); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.engine.AddNode(node); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, node)
}

func (h *ComputationGraphHandler) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes := h.engine.ListNodes()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"nodes": nodes,
		"total": len(nodes),
	})
}

func (h *ComputationGraphHandler) handleGetNode(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	node, err := h.engine.GetNode(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, node)
}

func (h *ComputationGraphHandler) handleRemoveNode(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.engine.RemoveNode(name); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"deleted": name})
}

func (h *ComputationGraphHandler) handleExecute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Inputs map[string]interface{} `json:"inputs"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
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

func (h *ComputationGraphHandler) handleExecuteBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Nodes  []string               `json:"nodes"`
		Inputs map[string]interface{} `json:"inputs"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
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

func (h *ComputationGraphHandler) handleGetDAG(w http.ResponseWriter, r *http.Request) {
	dag := h.engine.GetDAG()
	writeJSONResponse(r.Context(), w, http.StatusOK, dag)
}

func (h *ComputationGraphHandler) handleTopology(w http.ResponseWriter, r *http.Request) {
	order, err := h.engine.TopologicalSort()
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"order": order.Order,
		"levels": order.Levels,
		"total": len(order.Order),
	})
}

func (h *ComputationGraphHandler) handleParseDSL(w http.ResponseWriter, r *http.Request) {
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

func (h *ComputationGraphHandler) handleApplyDSL(w http.ResponseWriter, r *http.Request) {
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
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *ComputationGraphHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	errors := h.engine.Validate()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"errors": errors,
		"valid":  len(errors) == 0,
	})
}

func (h *ComputationGraphHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	memoStats := h.memoizer.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, memoStats)
}
