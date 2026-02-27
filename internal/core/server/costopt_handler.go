package server

import (
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/platform/costopt"
)

// CostOptHandler provides HTTP endpoints for AI-driven cost optimization.
type CostOptHandler struct {
	analyzer    *costopt.Analyzer
	recommender *costopt.Recommender
	forecaster  *costopt.Forecaster
}

// NewCostOptHandler creates a new cost optimization handler.
func NewCostOptHandler(analyzer *costopt.Analyzer, recommender *costopt.Recommender, forecaster *costopt.Forecaster) *CostOptHandler {
	return &CostOptHandler{
		analyzer:    analyzer,
		recommender: recommender,
		forecaster:  forecaster,
	}
}

// RegisterRoutes registers cost optimization API routes.
func (h *CostOptHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/costopt/access", h.handleRecordAccess)
	mux.HandleFunc("GET /v1/costopt/patterns", h.handleListPatterns)
	mux.HandleFunc("GET /v1/costopt/patterns/{group}", h.handleGetPattern)
	mux.HandleFunc("GET /v1/costopt/recommendations", h.handleListRecommendations)
	mux.HandleFunc("GET /v1/costopt/recommendations/{id}", h.handleGetRecommendation)
	mux.HandleFunc("POST /v1/costopt/recommendations/{id}/apply", h.handleApplyRecommendation)
	mux.HandleFunc("POST /v1/costopt/recommendations/{id}/dismiss", h.handleDismissRecommendation)
	mux.HandleFunc("POST /v1/costopt/forecast/data", h.handleAddCostData)
	mux.HandleFunc("GET /v1/costopt/forecast", h.handleGetForecast)
	mux.HandleFunc("GET /v1/costopt/anomalies", h.handleDetectAnomalies)
	mux.HandleFunc("GET /v1/costopt/stats", h.handleStats)
}

func (h *CostOptHandler) handleRecordAccess(w http.ResponseWriter, r *http.Request) {
	if h.analyzer == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "costopt not configured")
		return
	}
	var req struct {
		FeatureGroup string `json:"feature_group"`
		Entity       string `json:"entity"`
		Tier         string `json:"tier"`
		LatencyMs    int    `json:"latency_ms"`
		IsWrite      bool   `json:"is_write"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.FeatureGroup == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "feature_group is required")
		return
	}
	h.analyzer.RecordAccess(req.FeatureGroup, req.Entity, req.Tier,
		time.Duration(req.LatencyMs)*time.Millisecond, req.IsWrite)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "message": "access recorded",
	})
}

func (h *CostOptHandler) handleListPatterns(w http.ResponseWriter, r *http.Request) {
	if h.analyzer == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "costopt not configured")
		return
	}
	patterns := h.analyzer.ListPatterns()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "patterns": patterns, "count": len(patterns),
	})
}

func (h *CostOptHandler) handleGetPattern(w http.ResponseWriter, r *http.Request) {
	if h.analyzer == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "costopt not configured")
		return
	}
	group := r.PathValue("group")
	pattern := h.analyzer.GetPattern(group)
	if pattern == nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, "pattern not found")
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "pattern": pattern,
	})
}

func (h *CostOptHandler) handleListRecommendations(w http.ResponseWriter, r *http.Request) {
	if h.recommender == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "costopt not configured")
		return
	}
	recs := h.recommender.GenerateRecommendations()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "recommendations": recs, "count": len(recs),
	})
}

func (h *CostOptHandler) handleGetRecommendation(w http.ResponseWriter, r *http.Request) {
	if h.recommender == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "costopt not configured")
		return
	}
	rec, err := h.recommender.GetRecommendation(r.PathValue("id"))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "recommendation": rec,
	})
}

func (h *CostOptHandler) handleApplyRecommendation(w http.ResponseWriter, r *http.Request) {
	if h.recommender == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "costopt not configured")
		return
	}
	if err := h.recommender.ApplyRecommendation(r.PathValue("id")); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "message": "recommendation applied",
	})
}

func (h *CostOptHandler) handleDismissRecommendation(w http.ResponseWriter, r *http.Request) {
	if h.recommender == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "costopt not configured")
		return
	}
	if err := h.recommender.DismissRecommendation(r.PathValue("id")); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "message": "recommendation dismissed",
	})
}

func (h *CostOptHandler) handleAddCostData(w http.ResponseWriter, r *http.Request) {
	if h.forecaster == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "costopt not configured")
		return
	}
	var point costopt.CostDataPoint
	if err := strictDecode(r.Body, &point); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	h.forecaster.AddDataPoint(point)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "message": "data point added",
	})
}

func (h *CostOptHandler) handleGetForecast(w http.ResponseWriter, r *http.Request) {
	if h.forecaster == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "costopt not configured")
		return
	}
	forecast := h.forecaster.Predict()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "forecast": forecast,
	})
}

func (h *CostOptHandler) handleDetectAnomalies(w http.ResponseWriter, r *http.Request) {
	if h.forecaster == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "costopt not configured")
		return
	}
	anomalies := h.forecaster.DetectAnomalies()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "anomalies": anomalies, "count": len(anomalies),
	})
}

func (h *CostOptHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{"success": true}
	if h.analyzer != nil {
		patterns := h.analyzer.ListPatterns()
		resp["patterns_count"] = len(patterns)
	}
	if h.recommender != nil {
		resp["recommender"] = h.recommender.Stats()
	}
	if h.forecaster != nil {
		resp["forecast"] = h.forecaster.Predict()
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, resp)
}
