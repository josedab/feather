package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/importancescoring"
)

// ImportanceScoringHandler handles feature importance scoring API requests.
type ImportanceScoringHandler struct {
	scorer *importancescoring.Scorer
}

// NewImportanceScoringHandler creates a new importance scoring handler.
func NewImportanceScoringHandler(scorer *importancescoring.Scorer) *ImportanceScoringHandler {
	return &ImportanceScoringHandler{
		scorer: scorer,
	}
}

// RegisterRoutes registers importance scoring API routes.
func (h *ImportanceScoringHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/importance/record", h.handleRecordAccess)
	mux.HandleFunc("POST /v1/importance/score", h.handleScoreAll)
	mux.HandleFunc("GET /v1/importance/scores/{name}", h.handleGetScore)
	mux.HandleFunc("GET /v1/importance/top", h.handleGetTopK)
	mux.HandleFunc("GET /v1/importance/bottom", h.handleGetBottomK)
	mux.HandleFunc("GET /v1/importance/deprecation", h.handleGetDeprecationCandidates)
	mux.HandleFunc("GET /v1/importance/stats", h.handleGetStats)
}

// handleRecordAccess handles POST /v1/importance/record
func (h *ImportanceScoringHandler) handleRecordAccess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string  `json:"name"`
		Value float64 `json:"value"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	h.scorer.RecordAccess(req.Name, req.Value)

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "access recorded"})
}

// handleScoreAll handles POST /v1/importance/score
func (h *ImportanceScoringHandler) handleScoreAll(w http.ResponseWriter, r *http.Request) {
	scores := h.scorer.ScoreAll()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"scores": scores,
	})
}

// handleGetScore handles GET /v1/importance/scores/{name}
func (h *ImportanceScoringHandler) handleGetScore(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	score, err := h.scorer.GetScore(name)
	if err != nil {
		if errors.Is(err, importancescoring.ErrFeatureNotScored) {
			h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, score)
}

// handleGetTopK handles GET /v1/importance/top
func (h *ImportanceScoringHandler) handleGetTopK(w http.ResponseWriter, r *http.Request) {
	k := 10
	if kStr := r.URL.Query().Get("k"); kStr != "" {
		if parsed, err := strconv.Atoi(kStr); err == nil && parsed > 0 {
			k = parsed
		}
	}

	scores := h.scorer.GetTopK(k)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": scores,
		"k":        k,
	})
}

// handleGetBottomK handles GET /v1/importance/bottom
func (h *ImportanceScoringHandler) handleGetBottomK(w http.ResponseWriter, r *http.Request) {
	k := 10
	if kStr := r.URL.Query().Get("k"); kStr != "" {
		if parsed, err := strconv.Atoi(kStr); err == nil && parsed > 0 {
			k = parsed
		}
	}

	scores := h.scorer.GetBottomK(k)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": scores,
		"k":        k,
	})
}

// handleGetDeprecationCandidates handles GET /v1/importance/deprecation
func (h *ImportanceScoringHandler) handleGetDeprecationCandidates(w http.ResponseWriter, r *http.Request) {
	candidates := h.scorer.GetDeprecationCandidates()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"candidates": candidates,
	})
}

// handleGetStats handles GET /v1/importance/stats
func (h *ImportanceScoringHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.scorer.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *ImportanceScoringHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *ImportanceScoringHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
