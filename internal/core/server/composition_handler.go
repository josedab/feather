package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/feather-store/feather/internal/extensions/composition"
)

// CompositionHandler handles feature composition API requests.
type CompositionHandler struct {
	engine *composition.Engine
}

// NewCompositionHandler creates a new composition handler.
func NewCompositionHandler(engine *composition.Engine) *CompositionHandler {
	return &CompositionHandler{
		engine: engine,
	}
}

// RegisterRoutes registers the composition routes.
func (h *CompositionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/composition/dags", h.handleListDAGs)
	mux.HandleFunc("POST /v1/composition/dags", h.handleCreateDAG)
	mux.HandleFunc("GET /v1/composition/dags/{id}", h.handleGetDAG)
	mux.HandleFunc("DELETE /v1/composition/dags/{id}", h.handleDeleteDAG)
	mux.HandleFunc("POST /v1/composition/dags/{id}/compose", h.handleCompose)
	mux.HandleFunc("POST /v1/composition/dags/{id}/compose/batch", h.handleComposeBatch)
	mux.HandleFunc("GET /v1/composition/stats", h.handleStats)
	mux.HandleFunc("POST /v1/composition/cache/clear", h.handleClearCache)
}

// DAGRequest represents a request to create a DAG.
type DAGRequest struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Nodes       []NodeRequest `json:"nodes"`
	Outputs     []string      `json:"outputs"`
}

// NodeRequest represents a node in a DAG creation request.
type NodeRequest struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Inputs       []string               `json:"inputs,omitempty"`
	Expression   string                 `json:"expression,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	CacheEnabled bool                   `json:"cache_enabled,omitempty"`
	CacheTTL     string                 `json:"cache_ttl,omitempty"`
	Timeout      string                 `json:"timeout,omitempty"`
}

// ComposeRequest represents a composition request.
type ComposeRequest struct {
	EntityID string `json:"entity_id"`
}

// ComposeBatchRequest represents a batch composition request.
type ComposeBatchRequest struct {
	EntityIDs []string `json:"entity_ids"`
}

func (h *CompositionHandler) handleListDAGs(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "composition engine not configured")
		return
	}

	dags := h.engine.ListDAGs()

	dagResponses := make([]map[string]interface{}, 0, len(dags))
	for _, dag := range dags {
		dagResponses = append(dagResponses, map[string]interface{}{
			"id":          dag.ID,
			"name":        dag.Name,
			"description": dag.Description,
			"node_count":  len(dag.Nodes),
			"outputs":     dag.Outputs,
			"version":     dag.Version,
			"created_at":  dag.CreatedAt,
			"updated_at":  dag.UpdatedAt,
		})
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"dags":  dagResponses,
		"count": len(dagResponses),
	})
}

func (h *CompositionHandler) handleCreateDAG(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "composition engine not configured")
		return
	}

	var req DAGRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "dag id is required")
		return
	}

	dag := composition.NewDAG(req.ID, req.Name)
	dag.Description = req.Description

	// Add nodes
	for _, nodeReq := range req.Nodes {
		node := &composition.Node{
			ID:           nodeReq.ID,
			Name:         nodeReq.Name,
			Type:         parseNodeType(nodeReq.Type),
			Inputs:       nodeReq.Inputs,
			Expression:   nodeReq.Expression,
			Config:       nodeReq.Config,
			CacheEnabled: nodeReq.CacheEnabled,
		}

		if nodeReq.CacheTTL != "" {
			if ttl, err := time.ParseDuration(nodeReq.CacheTTL); err == nil {
				node.CacheTTL = ttl
			}
		}

		if nodeReq.Timeout != "" {
			if timeout, err := time.ParseDuration(nodeReq.Timeout); err == nil {
				node.Timeout = timeout
			}
		}

		if err := dag.AddNode(node); err != nil {
			writeJSONError(r.Context(), w, http.StatusBadRequest, "failed to add node "+nodeReq.ID+": "+err.Error())
			return
		}
	}

	// Set outputs
	if err := dag.SetOutputs(req.Outputs); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "failed to set outputs: "+err.Error())
		return
	}

	// Register DAG
	if err := h.engine.RegisterDAG(dag); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "failed to register DAG: "+err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"id":      dag.ID,
		"name":    dag.Name,
		"message": "DAG created successfully",
	})
}

func (h *CompositionHandler) handleGetDAG(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "composition engine not configured")
		return
	}

	dagID := r.PathValue("id")
	if dagID == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "dag id is required")
		return
	}

	dag, err := h.engine.GetDAG(dagID)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, "DAG not found")
		return
	}

	// Build nodes response
	nodes := make([]map[string]interface{}, 0, len(dag.Nodes))
	for _, node := range dag.Nodes {
		nodes = append(nodes, map[string]interface{}{
			"id":            node.ID,
			"name":          node.Name,
			"type":          string(node.Type),
			"inputs":        node.Inputs,
			"expression":    node.Expression,
			"config":        node.Config,
			"cache_enabled": node.CacheEnabled,
			"cache_ttl":     node.CacheTTL.String(),
			"timeout":       node.Timeout.String(),
		})
	}

	stats := dag.Stats()

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"id":          dag.ID,
		"name":        dag.Name,
		"description": dag.Description,
		"nodes":       nodes,
		"outputs":     dag.Outputs,
		"version":     dag.Version,
		"created_at":  dag.CreatedAt,
		"updated_at":  dag.UpdatedAt,
		"stats":       stats,
	})
}

func (h *CompositionHandler) handleDeleteDAG(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "composition engine not configured")
		return
	}

	dagID := r.PathValue("id")
	if dagID == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "dag id is required")
		return
	}

	if err := h.engine.UnregisterDAG(dagID); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, "DAG not found")
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"message": "DAG deleted successfully",
	})
}

func (h *CompositionHandler) handleCompose(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "composition engine not configured")
		return
	}

	dagID := r.PathValue("id")
	if dagID == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "dag id is required")
		return
	}

	var req ComposeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.EntityID == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "entity_id is required")
		return
	}

	results, err := h.engine.Compose(r.Context(), dagID, req.EntityID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		} else {
			writeJSONError(r.Context(), w, http.StatusInternalServerError, "composition failed: "+err.Error())
		}
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"dag_id":    dagID,
		"entity_id": req.EntityID,
		"results":   results,
	})
}

func (h *CompositionHandler) handleComposeBatch(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "composition engine not configured")
		return
	}

	dagID := r.PathValue("id")
	if dagID == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "dag id is required")
		return
	}

	var req ComposeBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.EntityIDs) == 0 {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "entity_ids is required")
		return
	}

	results, err := h.engine.ComposeBatch(r.Context(), dagID, req.EntityIDs)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		} else {
			// Partial failure - return results with error info
			writeJSONResponse(r.Context(), w, http.StatusPartialContent, map[string]interface{}{
				"dag_id":  dagID,
				"results": results,
				"error":   err.Error(),
			})
			return
		}
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"dag_id":       dagID,
		"entity_count": len(req.EntityIDs),
		"results":      results,
	})
}

func (h *CompositionHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "composition engine not configured")
		return
	}

	stats := h.engine.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}

func (h *CompositionHandler) handleClearCache(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "composition engine not configured")
		return
	}

	h.engine.ClearCache()

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"message": "cache cleared successfully",
	})
}

func parseNodeType(t string) composition.NodeType {
	switch strings.ToLower(t) {
	case "source":
		return composition.NodeTypeSource
	case "transform":
		return composition.NodeTypeTransform
	case "aggregate":
		return composition.NodeTypeAggregate
	case "join":
		return composition.NodeTypeJoin
	case "filter":
		return composition.NodeTypeFilter
	case "custom":
		return composition.NodeTypeCustom
	default:
		return composition.NodeTypeCustom
	}
}
