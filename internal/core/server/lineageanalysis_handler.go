package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/lineageanalysis"
)

// LineageAnalysisHandler handles feature lineage and impact analysis API requests.
type LineageAnalysisHandler struct {
	tracker *lineageanalysis.Tracker
}

// NewLineageAnalysisHandler creates a new lineage analysis handler.
func NewLineageAnalysisHandler(tracker *lineageanalysis.Tracker) *LineageAnalysisHandler {
	return &LineageAnalysisHandler{tracker: tracker}
}

// RegisterRoutes registers lineage analysis API routes.
func (h *LineageAnalysisHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/lineage/nodes", h.handleListNodes)
	mux.HandleFunc("POST /v1/lineage/nodes", h.handleAddNode)
	mux.HandleFunc("GET /v1/lineage/nodes/{id}", h.handleGetNode)
	mux.HandleFunc("POST /v1/lineage/edges", h.handleAddEdge)
	mux.HandleFunc("GET /v1/lineage/nodes/{id}/upstream", h.handleUpstream)
	mux.HandleFunc("GET /v1/lineage/nodes/{id}/downstream", h.handleDownstream)
	mux.HandleFunc("GET /v1/lineage/nodes/{id}/impact", h.handleImpact)
	mux.HandleFunc("GET /v1/lineage/stats", h.handleStats)
}

func (h *LineageAnalysisHandler) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodeType := r.URL.Query().Get("type")
	nodes := h.tracker.ListNodes(nodeType)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"nodes": nodes,
		"count": len(nodes),
	})
}

func (h *LineageAnalysisHandler) handleAddNode(w http.ResponseWriter, r *http.Request) {
	var node lineageanalysis.LineageNode
	if err := strictDecode(r.Body, &node); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.tracker.AddNode(node); err != nil {
		if errors.Is(err, lineageanalysis.ErrNodeExists) {
			h.writeError(r.Context(), w, http.StatusConflict, "node already exists")
			return
		}
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "node added"})
}

func (h *LineageAnalysisHandler) handleGetNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	node, err := h.tracker.GetNode(id)
	if err != nil {
		if errors.Is(err, lineageanalysis.ErrNodeNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "node not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, node)
}

func (h *LineageAnalysisHandler) handleAddEdge(w http.ResponseWriter, r *http.Request) {
	var edge lineageanalysis.LineageEdge
	if err := strictDecode(r.Body, &edge); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.tracker.AddEdge(edge); err != nil {
		if errors.Is(err, lineageanalysis.ErrCyclicLineage) {
			h.writeError(r.Context(), w, http.StatusBadRequest, "would create cycle")
			return
		}
		if errors.Is(err, lineageanalysis.ErrEdgeExists) {
			h.writeError(r.Context(), w, http.StatusConflict, "edge already exists")
			return
		}
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "edge added"})
}

func (h *LineageAnalysisHandler) handleUpstream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	upstream := h.tracker.GetUpstream(id)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"upstream": upstream,
		"count":    len(upstream),
	})
}

func (h *LineageAnalysisHandler) handleDownstream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	downstream := h.tracker.GetDownstream(id)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"downstream": downstream,
		"count":      len(downstream),
	})
}

func (h *LineageAnalysisHandler) handleImpact(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	report, err := h.tracker.AnalyzeImpact(id)
	if err != nil {
		if errors.Is(err, lineageanalysis.ErrNodeNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "node not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, report)
}

func (h *LineageAnalysisHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.tracker.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *LineageAnalysisHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *LineageAnalysisHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
