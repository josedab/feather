package server

import (
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/semantic"
)

// handleExplain handles POST /v1/semantic/explain
func (h *SemanticHandler) handleExplain(w http.ResponseWriter, r *http.Request) {
	if h.explainer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "explainer not configured")
		return
	}

	var req ExplainRequestBody
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FeatureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature_id is required")
		return
	}
	if req.Query == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "query is required")
		return
	}

	if req.Score == 0 {
		req.Score = 0.5
	}

	explanation, err := h.explainer.Explain(r.Context(), req.FeatureID, req.Query, req.Score)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, explanation)
}

// handleExplainBatch handles POST /v1/semantic/explain/batch
func (h *SemanticHandler) handleExplainBatch(w http.ResponseWriter, r *http.Request) {
	if h.explainer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "explainer not configured")
		return
	}

	var req struct {
		Results []struct {
			FeatureID string  `json:"feature_id"`
			Score     float64 `json:"score"`
		} `json:"results"`
		Query string `json:"query"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Results) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "results is required")
		return
	}
	if req.Query == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "query is required")
		return
	}

	// Convert to RankedResult format
	rankedResults := make([]semantic.RankedResult, 0, len(req.Results))
	for _, res := range req.Results {
		enriched, err := h.indexer.GetEnrichedFeature(res.FeatureID)
		if err != nil {
			continue
		}
		rankedResults = append(rankedResults, semantic.RankedResult{
			Feature:    enriched,
			TotalScore: res.Score,
		})
	}

	explanations, err := h.explainer.ExplainResults(r.Context(), rankedResults, req.Query)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"explanations": explanations,
		"count":        len(explanations),
		"query":        req.Query,
	})
}

// handleDiscoverPopular handles GET /v1/semantic/discover/popular
func (h *SemanticHandler) handleDiscoverPopular(w http.ResponseWriter, r *http.Request) {
	if h.indexer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "enhanced indexer not configured")
		return
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	features := h.indexer.GetMostPopular(limit)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": features,
		"count":    len(features),
	})
}

// handleDiscoverHighQuality handles GET /v1/semantic/discover/quality
func (h *SemanticHandler) handleDiscoverHighQuality(w http.ResponseWriter, r *http.Request) {
	if h.indexer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "enhanced indexer not configured")
		return
	}

	minScore := float32(0.8)
	if ms := r.URL.Query().Get("min_score"); ms != "" {
		if parsed, err := strconv.ParseFloat(ms, 32); err == nil {
			minScore = float32(parsed)
		}
	}

	features := h.indexer.GetHighQuality(minScore)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features":  features,
		"count":     len(features),
		"min_score": minScore,
	})
}

// handleDiscoverByDomain handles GET /v1/semantic/discover/domain/{domain}
func (h *SemanticHandler) handleDiscoverByDomain(w http.ResponseWriter, r *http.Request) {
	if h.indexer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "enhanced indexer not configured")
		return
	}

	domain := r.PathValue("domain")
	if domain == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "domain required")
		return
	}

	features := h.indexer.FindByDomain(domain)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": features,
		"count":    len(features),
		"domain":   domain,
	})
}

// handleDiscoverByEntity handles GET /v1/semantic/discover/entity/{entityType}
func (h *SemanticHandler) handleDiscoverByEntity(w http.ResponseWriter, r *http.Request) {
	if h.indexer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "enhanced indexer not configured")
		return
	}

	entityType := r.PathValue("entityType")
	if entityType == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entity type required")
		return
	}

	features := h.indexer.FindByEntityType(entityType)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features":    features,
		"count":       len(features),
		"entity_type": entityType,
	})
}

// handleDiscoverByUseCase handles GET /v1/semantic/discover/usecase/{usecase}
func (h *SemanticHandler) handleDiscoverByUseCase(w http.ResponseWriter, r *http.Request) {
	if h.indexer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "enhanced indexer not configured")
		return
	}

	useCase := r.PathValue("usecase")
	if useCase == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "use case required")
		return
	}

	features := h.indexer.FindByUseCase(useCase)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": features,
		"count":    len(features),
		"use_case": useCase,
	})
}

// handleGetStats handles GET /v1/semantic/stats
func (h *SemanticHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	stats := h.search.GetStats()

	if h.indexer != nil {
		indexerStats := h.indexer.GetStats()
		stats["indexer"] = indexerStats
	}

	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}
