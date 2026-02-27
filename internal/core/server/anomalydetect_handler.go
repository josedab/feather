package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/feather-store/feather/internal/extensions/anomalydetect"
)

// AnomalyDetectHandler handles anomaly detection API requests.
type AnomalyDetectHandler struct {
	detector *anomalydetect.Detector
}

// NewAnomalyDetectHandler creates a new anomaly detection handler.
func NewAnomalyDetectHandler(detector *anomalydetect.Detector) *AnomalyDetectHandler {
	return &AnomalyDetectHandler{
		detector: detector,
	}
}

// RegisterRoutes registers anomaly detection API routes.
func (h *AnomalyDetectHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/anomaly/register", h.handleRegisterFeature)
	mux.HandleFunc("POST /v1/anomaly/check", h.handleCheck)
	mux.HandleFunc("GET /v1/anomaly/alerts", h.handleGetAlerts)
	mux.HandleFunc("GET /v1/anomaly/quarantine/{feature}", h.handleIsQuarantined)
	mux.HandleFunc("POST /v1/anomaly/quarantine/{feature}/clear", h.handleClearQuarantine)
	mux.HandleFunc("GET /v1/anomaly/features/{feature}/stats", h.handleGetFeatureStats)
	mux.HandleFunc("GET /v1/anomaly/stats", h.handleGetStats)
}

// handleRegisterFeature handles POST /v1/anomaly/register
func (h *AnomalyDetectHandler) handleRegisterFeature(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	h.detector.RegisterFeature(req.Name)

	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "feature registered"})
}

// handleCheck handles POST /v1/anomaly/check
func (h *AnomalyDetectHandler) handleCheck(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Feature string  `json:"feature"`
		Value   float64 `json:"value"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	result := h.detector.Check(req.Feature, req.Value)
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleGetAlerts handles GET /v1/anomaly/alerts
func (h *AnomalyDetectHandler) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-1 * time.Hour)
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	alerts := h.detector.GetAlerts(since, limit)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"since":  since,
	})
}

// handleIsQuarantined handles GET /v1/anomaly/quarantine/{feature}
func (h *AnomalyDetectHandler) handleIsQuarantined(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	quarantined := h.detector.IsQuarantined(feature)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"feature":     feature,
		"quarantined": quarantined,
	})
}

// handleClearQuarantine handles POST /v1/anomaly/quarantine/{feature}/clear
func (h *AnomalyDetectHandler) handleClearQuarantine(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	if err := h.detector.ClearQuarantine(feature); err != nil {
		if errors.Is(err, anomalydetect.ErrFeatureNotMonitored) {
			h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "quarantine cleared"})
}

// handleGetFeatureStats handles GET /v1/anomaly/features/{feature}/stats
func (h *AnomalyDetectHandler) handleGetFeatureStats(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	stats, err := h.detector.GetFeatureStats(feature)
	if err != nil {
		if errors.Is(err, anomalydetect.ErrFeatureNotMonitored) {
			h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

// handleGetStats handles GET /v1/anomaly/stats
func (h *AnomalyDetectHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.detector.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *AnomalyDetectHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *AnomalyDetectHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
