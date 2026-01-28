package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/platform/autoscaler"
)

// AutoscalerHandler provides HTTP endpoints for the K8s-aware autoscaler.
type AutoscalerHandler struct {
	scaler *autoscaler.Autoscaler
}

// NewAutoscalerHandler creates a new autoscaler handler.
func NewAutoscalerHandler(scaler *autoscaler.Autoscaler) *AutoscalerHandler {
	return &AutoscalerHandler{scaler: scaler}
}

// RegisterRoutes registers autoscaler API routes.
func (h *AutoscalerHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/autoscaler/metrics", h.handleRecordMetric)
	mux.HandleFunc("GET /v1/autoscaler/metrics", h.handleGetMetrics)
	mux.HandleFunc("GET /v1/autoscaler/evaluate", h.handleEvaluate)
	mux.HandleFunc("POST /v1/autoscaler/apply", h.handleApply)
	mux.HandleFunc("GET /v1/autoscaler/shards", h.handleGetShards)
	mux.HandleFunc("POST /v1/autoscaler/shards/rebalance", h.handleRebalance)
	mux.HandleFunc("GET /v1/autoscaler/history", h.handleHistory)
	mux.HandleFunc("GET /v1/autoscaler/stats", h.handleStats)
}

func (h *AutoscalerHandler) handleRecordMetric(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Metric string  `json:"metric"`
		Value  float64 `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Metric == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "metric is required")
		return
	}

	h.scaler.RecordMetric(autoscaler.MetricType(req.Metric), req.Value)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "recorded"})
}

func (h *AutoscalerHandler) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := h.scaler.GetMetrics()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"metrics": metrics,
		"total":   len(metrics),
	})
}

func (h *AutoscalerHandler) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	rec := h.scaler.Evaluate()
	writeJSONResponse(r.Context(), w, http.StatusOK, rec)
}

func (h *AutoscalerHandler) handleApply(w http.ResponseWriter, r *http.Request) {
	var rec autoscaler.ScaleRecommendation
	if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.scaler.Apply(&rec); err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"status":           "applied",
		"current_replicas": h.scaler.CurrentReplicas(),
	})
}

func (h *AutoscalerHandler) handleGetShards(w http.ResponseWriter, r *http.Request) {
	assignments := h.scaler.GetShardAssignments()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"shards": assignments,
		"total":  len(assignments),
	})
}

func (h *AutoscalerHandler) handleRebalance(w http.ResponseWriter, r *http.Request) {
	assignments := h.scaler.RebalanceShards()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"shards":     assignments,
		"total":      len(assignments),
		"rebalanced": true,
	})
}

func (h *AutoscalerHandler) handleHistory(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	events := h.scaler.GetScaleHistory(limit)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"events": events,
		"total":  len(events),
	})
}

func (h *AutoscalerHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.scaler.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
