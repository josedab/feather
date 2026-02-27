package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/extensions/computegraph"
)

// ---------------------------------------------------------------------------
// DeclarativeGraphHandler
// ---------------------------------------------------------------------------

// DeclarativeGraphHandler exposes declarative feature computation graph endpoints.
type DeclarativeGraphHandler struct {
	graph *computegraph.DeclarativeGraph
}

// NewDeclarativeGraphHandler creates a new DeclarativeGraphHandler.
func NewDeclarativeGraphHandler(graph *computegraph.DeclarativeGraph) *DeclarativeGraphHandler {
	return &DeclarativeGraphHandler{graph: graph}
}

// RegisterRoutes registers declarative graph API routes.
func (h *DeclarativeGraphHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/graph/declarative/apply", h.handleApply)
	mux.HandleFunc("POST /v1/graph/declarative/compute/{name}", h.handleCompute)
	mux.HandleFunc("POST /v1/graph/declarative/compute-all", h.handleComputeAll)
	mux.HandleFunc("GET /v1/graph/declarative/plan", h.handleGetPlan)
	mux.HandleFunc("GET /v1/graph/declarative/lineage/{name}", h.handleGetLineage)
	mux.HandleFunc("POST /v1/graph/declarative/invalidate/{name}", h.handleInvalidate)
	mux.HandleFunc("GET /v1/graph/declarative/stats", h.handleStats)
}

func (h *DeclarativeGraphHandler) handleApply(w http.ResponseWriter, r *http.Request) {
	var def computegraph.GraphSpec
	if err := strictDecode(r.Body, &def); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result := h.graph.ApplyDefinition(def)
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *DeclarativeGraphHandler) handleCompute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Inputs map[string]interface{} `json:"inputs"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result, err := h.graph.Compute(r.Context(), name, req.Inputs)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *DeclarativeGraphHandler) handleComputeAll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Inputs map[string]interface{} `json:"inputs"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	results, err := h.graph.ComputeAll(r.Context(), req.Inputs)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, results)
}

func (h *DeclarativeGraphHandler) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	plan, err := h.graph.GetExecutionPlan()
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, plan)
}

func (h *DeclarativeGraphHandler) handleGetLineage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	lineage := h.graph.GetLineage(name)
	writeJSONResponse(r.Context(), w, http.StatusOK, lineage)
}

func (h *DeclarativeGraphHandler) handleInvalidate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Inputs map[string]interface{} `json:"inputs,omitempty"`
	}
	_ = strictDecode(r.Body, &req)
	affected, err := h.graph.InvalidateAndRecompute(name, req.Inputs)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"invalidated": name,
		"affected":    affected,
	})
}

func (h *DeclarativeGraphHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.graph.Stats())
}
