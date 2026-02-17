package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/lineagegraph"
)

// LineageGraphHandler handles feature lineage graph API requests.
type LineageGraphHandler struct {
	graph *lineagegraph.Graph
}

// NewLineageGraphHandler creates a new lineage graph handler.
func NewLineageGraphHandler(graph *lineagegraph.Graph) *LineageGraphHandler {
	return &LineageGraphHandler{graph: graph}
}

// RegisterRoutes registers lineage graph API routes.
func (h *LineageGraphHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/lineage/graph", h.handleGetGraph)
	mux.HandleFunc("POST /v1/lineage/nodes", h.handleAddNode)
	mux.HandleFunc("GET /v1/lineage/nodes/{id}", h.handleGetNode)
	mux.HandleFunc("PUT /v1/lineage/nodes/{id}", h.handleUpdateNode)
	mux.HandleFunc("DELETE /v1/lineage/nodes/{id}", h.handleRemoveNode)
	mux.HandleFunc("POST /v1/lineage/edges", h.handleAddEdge)
	mux.HandleFunc("GET /v1/lineage/nodes/{id}/upstream", h.handleGetUpstream)
	mux.HandleFunc("GET /v1/lineage/nodes/{id}/downstream", h.handleGetDownstream)
	mux.HandleFunc("GET /v1/lineage/nodes/{id}/impact", h.handleGetImpact)
	mux.HandleFunc("GET /v1/lineage/stats", h.handleGetStats)
}

func (h *LineageGraphHandler) handleGetGraph(w http.ResponseWriter, r *http.Request) {
	view := h.graph.GetView()
	h.writeJSON(r.Context(), w, http.StatusOK, view)
}

func (h *LineageGraphHandler) handleAddNode(w http.ResponseWriter, r *http.Request) {
	var node lineagegraph.Node
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if node.ID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "node id required")
		return
	}
	if err := h.graph.AddNode(node); err != nil {
		if errors.Is(err, lineagegraph.ErrNodeExists) {
			h.writeError(r.Context(), w, http.StatusConflict, "node already exists")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "id": node.ID})
}

func (h *LineageGraphHandler) handleGetNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	node, err := h.graph.GetNode(id)
	if err != nil {
		if errors.Is(err, lineagegraph.ErrNodeNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "node not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, node)
}

func (h *LineageGraphHandler) handleUpdateNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Freshness string            `json:"freshness"`
		Metadata  map[string]string `json:"metadata,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.graph.UpdateNode(id, lineagegraph.FreshnessStatus(req.Freshness), req.Metadata); err != nil {
		if errors.Is(err, lineagegraph.ErrNodeNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "node not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *LineageGraphHandler) handleRemoveNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.graph.RemoveNode(id); err != nil {
		if errors.Is(err, lineagegraph.ErrNodeNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "node not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *LineageGraphHandler) handleAddEdge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From  string `json:"from"`
		To    string `json:"to"`
		Label string `json:"label,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.graph.AddEdge(req.From, req.To, req.Label); err != nil {
		if errors.Is(err, lineagegraph.ErrCyclicDependency) {
			h.writeError(r.Context(), w, http.StatusConflict, "would create cycle")
			return
		}
		if errors.Is(err, lineagegraph.ErrNodeNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true})
}

func (h *LineageGraphHandler) handleGetUpstream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	nodes, err := h.graph.GetUpstream(id)
	if err != nil {
		if errors.Is(err, lineagegraph.ErrNodeNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "node not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"nodes": nodes, "count": len(nodes)})
}

func (h *LineageGraphHandler) handleGetDownstream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	nodes, err := h.graph.GetDownstream(id)
	if err != nil {
		if errors.Is(err, lineagegraph.ErrNodeNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "node not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"nodes": nodes, "count": len(nodes)})
}

func (h *LineageGraphHandler) handleGetImpact(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	impact, err := h.graph.GetImpact(id)
	if err != nil {
		if errors.Is(err, lineagegraph.ErrNodeNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "node not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, impact)
}

func (h *LineageGraphHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(r.Context(), w, http.StatusOK, h.graph.Stats())
}

func (h *LineageGraphHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *LineageGraphHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, msg string) {
	writeJSONError(ctx, w, status, msg)
}
