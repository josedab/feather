package server

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/feather-store/feather/internal/extensions/qualityscore"
)

// QualityScoreHandler handles quality scoring API requests.
type QualityScoreHandler struct {
	scorer *qualityscore.Scorer
}

// NewQualityScoreHandler creates a new quality score handler.
func NewQualityScoreHandler(scorer *qualityscore.Scorer) *QualityScoreHandler {
	return &QualityScoreHandler{scorer: scorer}
}

// RegisterRoutes registers quality score API routes.
func (h *QualityScoreHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/quality-score/signal", h.handleRecordSignal)
	mux.HandleFunc("POST /v1/quality-score/compute/{featureId}", h.handleComputeScore)
	mux.HandleFunc("POST /v1/quality-score/compute-all", h.handleComputeAll)
	mux.HandleFunc("GET /v1/quality-score/top", h.handleTopN)
	mux.HandleFunc("GET /v1/quality-score/bottom", h.handleBottomN)
	mux.HandleFunc("GET /v1/quality-score/deprecation-candidates", h.handleDeprecationCandidates)
	mux.HandleFunc("GET /v1/quality-score/stats", h.handleStats)
	mux.HandleFunc("GET /v1/quality-score/{featureId}", h.handleGetScore)
}

// recordSignalRequest is the JSON body for recording a signal.
type recordSignalRequest struct {
	FeatureID string  `json:"feature_id"`
	Type      string  `json:"type"`
	Value     float64 `json:"value"`
	Weight    float64 `json:"weight"`
}

// handleRecordSignal handles POST /v1/quality-score/signal
func (h *QualityScoreHandler) handleRecordSignal(w http.ResponseWriter, r *http.Request) {
	var req recordSignalRequest
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FeatureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature_id is required")
		return
	}
	if req.Type == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "type is required")
		return
	}

	h.scorer.RecordSignal(req.FeatureID, &qualityscore.Signal{
		Type:      qualityscore.SignalType(req.Type),
		Value:     req.Value,
		Weight:    req.Weight,
		Timestamp: time.Now(),
	})

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":    true,
		"feature_id": req.FeatureID,
		"signal":     req.Type,
	})
}

// handleComputeScore handles POST /v1/quality-score/compute/{featureId}
func (h *QualityScoreHandler) handleComputeScore(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("featureId")
	if featureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature ID is required")
		return
	}

	score, err := h.scorer.Score(featureID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, score)
}

// handleComputeAll handles POST /v1/quality-score/compute-all
func (h *QualityScoreHandler) handleComputeAll(w http.ResponseWriter, r *http.Request) {
	scores := h.scorer.ScoreAll()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"scores": scores,
		"count":  len(scores),
	})
}

// handleGetScore handles GET /v1/quality-score/{featureId}
func (h *QualityScoreHandler) handleGetScore(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("featureId")
	if featureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature ID is required")
		return
	}

	score, err := h.scorer.GetScore(featureID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, score)
}

// handleTopN handles GET /v1/quality-score/top
func (h *QualityScoreHandler) handleTopN(w http.ResponseWriter, r *http.Request) {
	n := 10
	if nStr := r.URL.Query().Get("n"); nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 {
			n = parsed
		}
	}

	scores := h.scorer.TopN(n)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"scores": scores,
		"count":  len(scores),
	})
}

// handleBottomN handles GET /v1/quality-score/bottom
func (h *QualityScoreHandler) handleBottomN(w http.ResponseWriter, r *http.Request) {
	n := 10
	if nStr := r.URL.Query().Get("n"); nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 {
			n = parsed
		}
	}

	threshold := 0.4
	if tStr := r.URL.Query().Get("threshold"); tStr != "" {
		if parsed, err := strconv.ParseFloat(tStr, 64); err == nil {
			threshold = parsed
		}
	}

	scores := h.scorer.BottomN(n)

	// Filter by threshold if provided
	var filtered []*qualityscore.FeatureScore
	for _, s := range scores {
		if s.OverallScore <= threshold {
			filtered = append(filtered, s)
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"scores": filtered,
		"count":  len(filtered),
	})
}

// handleDeprecationCandidates handles GET /v1/quality-score/deprecation-candidates
func (h *QualityScoreHandler) handleDeprecationCandidates(w http.ResponseWriter, r *http.Request) {
	threshold := 0.4
	if tStr := r.URL.Query().Get("threshold"); tStr != "" {
		if parsed, err := strconv.ParseFloat(tStr, 64); err == nil {
			threshold = parsed
		}
	}

	candidates := h.scorer.GetDeprecationCandidates(threshold)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"candidates": candidates,
		"count":      len(candidates),
		"threshold":  threshold,
	})
}

// handleStats handles GET /v1/quality-score/stats
func (h *QualityScoreHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.scorer.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *QualityScoreHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *QualityScoreHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
