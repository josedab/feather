package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/tools/backfill"
)

// OrchestratorHandler handles DAG orchestration API requests.
type OrchestratorHandler struct {
	orchestrator *backfill.Orchestrator
}

// NewOrchestratorHandler creates a new orchestrator handler.
func NewOrchestratorHandler(orchestrator *backfill.Orchestrator) *OrchestratorHandler {
	if orchestrator == nil {
		orchestrator = backfill.NewOrchestrator(backfill.DefaultOrchestratorConfig())
	}
	return &OrchestratorHandler{orchestrator: orchestrator}
}

// RegisterRoutes registers orchestrator API routes.
func (h *OrchestratorHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/orchestrator/dags", h.handleCreateDAG)
	mux.HandleFunc("GET /v1/orchestrator/dags", h.handleListDAGs)
	mux.HandleFunc("GET /v1/orchestrator/dags/{id}", h.handleGetDAG)
	mux.HandleFunc("DELETE /v1/orchestrator/dags/{id}", h.handleDeleteDAG)
	mux.HandleFunc("POST /v1/orchestrator/dags/{id}/execute", h.handleExecuteDAG)
	mux.HandleFunc("GET /v1/orchestrator/dags/{id}/ready", h.handleGetReadyNodes)
	mux.HandleFunc("POST /v1/orchestrator/dags/{id}/nodes/{nodeId}/complete", h.handleCompleteNode)
	mux.HandleFunc("POST /v1/orchestrator/dags/{id}/nodes/{nodeId}/fail", h.handleFailNode)
	mux.HandleFunc("GET /v1/orchestrator/dags/{id}/cost", h.handleEstimateCost)
	mux.HandleFunc("GET /v1/orchestrator/stats", h.handleStats)
}

// createDAGRequest is the request body for creating a DAG.
type createDAGRequest struct {
	Name  string             `json:"name"`
	Nodes []*backfill.DAGNode `json:"nodes"`
}

func (h *OrchestratorHandler) handleCreateDAG(w http.ResponseWriter, r *http.Request) {
	var req createDAGRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || len(req.Nodes) == 0 {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "name and nodes are required")
		return
	}

	dag, err := h.orchestrator.CreateDAG(req.Name, req.Nodes)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusCreated, dag)
}

func (h *OrchestratorHandler) handleListDAGs(w http.ResponseWriter, r *http.Request) {
	dags := h.orchestrator.ListDAGs()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"dags":  dags,
		"count": len(dags),
	})
}

func (h *OrchestratorHandler) handleGetDAG(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dag, err := h.orchestrator.GetDAG(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, dag)
}

func (h *OrchestratorHandler) handleDeleteDAG(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.orchestrator.DeleteDAG(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
	})
}

func (h *OrchestratorHandler) handleExecuteDAG(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.orchestrator.ExecuteDAG(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
		"status":  "running",
	})
}

func (h *OrchestratorHandler) handleGetReadyNodes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	nodes := h.orchestrator.GetReadyNodes(id)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"nodes": nodes,
		"count": len(nodes),
	})
}

func (h *OrchestratorHandler) handleCompleteNode(w http.ResponseWriter, r *http.Request) {
	dagID := r.PathValue("id")
	nodeID := r.PathValue("nodeId")
	if err := h.orchestrator.CompleteNode(dagID, nodeID); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"node_id": nodeID,
		"status":  "completed",
	})
}

// failNodeRequest is the request body for failing a node.
type failNodeRequest struct {
	Error string `json:"error"`
}

func (h *OrchestratorHandler) handleFailNode(w http.ResponseWriter, r *http.Request) {
	dagID := r.PathValue("id")
	nodeID := r.PathValue("nodeId")

	var req failNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.orchestrator.FailNode(dagID, nodeID, req.Error); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"node_id": nodeID,
	})
}

func (h *OrchestratorHandler) handleEstimateCost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	estimate, err := h.orchestrator.EstimateCost(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, estimate)
}

func (h *OrchestratorHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.orchestrator.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
