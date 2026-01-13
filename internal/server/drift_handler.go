package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/drift"
)

// DriftHandler handles drift detection API requests.
type DriftHandler struct {
	detector *drift.Detector
}

// NewDriftHandler creates a new drift handler.
func NewDriftHandler(detector *drift.Detector) *DriftHandler {
	return &DriftHandler{
		detector: detector,
	}
}

// RegisterRoutes registers drift API routes.
func (h *DriftHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/drift/status", h.handleGetStatus)
	mux.HandleFunc("GET /v1/drift/alerts", h.handleGetAlerts)
	mux.HandleFunc("POST /v1/drift/reset/{feature}", h.handleResetReference)
	mux.HandleFunc("POST /v1/drift/register", h.handleRegisterFeature)
}

// DriftStatusResponse represents the drift monitoring status.
type DriftStatusResponse struct {
	Monitors []MonitorStatusJSON `json:"monitors"`
}

// MonitorStatusJSON is the JSON representation of a monitor status.
type MonitorStatusJSON struct {
	Feature         string  `json:"feature"`
	Type            string  `json:"type"`
	SampleCount     int64   `json:"sample_count"`
	DriftDetected   bool    `json:"drift_detected"`
	DriftType       string  `json:"drift_type,omitempty"`
	DriftScore      float64 `json:"drift_score"`
	CurrentMean     float64 `json:"current_mean,omitempty"`
	CurrentStdDev   float64 `json:"current_std_dev,omitempty"`
	ReferenceMean   float64 `json:"reference_mean,omitempty"`
	ReferenceStdDev float64 `json:"reference_std_dev,omitempty"`
}

// AlertJSON is the JSON representation of a drift alert.
type AlertJSON struct {
	Feature   string    `json:"feature"`
	Type      string    `json:"type"`
	Score     float64   `json:"score"`
	Threshold float64   `json:"threshold"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

// handleGetStatus handles GET /v1/drift/status
func (h *DriftHandler) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	if h.detector == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "drift detector not configured")
		return
	}

	statuses := h.detector.GetMonitorStatus()
	response := DriftStatusResponse{
		Monitors: make([]MonitorStatusJSON, len(statuses)),
	}

	for i, s := range statuses {
		featureType := "numeric"
		if s.Type == drift.TypeCategorical {
			featureType = "categorical"
		}

		response.Monitors[i] = MonitorStatusJSON{
			Feature:         s.Feature,
			Type:            featureType,
			SampleCount:     s.SampleCount,
			DriftDetected:   s.DriftType != drift.DriftNone,
			DriftType:       s.DriftType.String(),
			DriftScore:      s.DriftScore,
			CurrentMean:     s.CurrentMean,
			CurrentStdDev:   s.CurrentStdDev,
			ReferenceMean:   s.ReferenceMean,
			ReferenceStdDev: s.ReferenceStdDev,
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, response)
}

// handleGetAlerts handles GET /v1/drift/alerts
func (h *DriftHandler) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	if h.detector == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "drift detector not configured")
		return
	}

	// Parse since parameter (default: last hour)
	since := time.Now().Add(-1 * time.Hour)
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	alerts := h.detector.GetAlerts(since)
	response := make([]AlertJSON, len(alerts))
	for i, a := range alerts {
		response[i] = AlertJSON{
			Feature:   a.Feature,
			Type:      a.Type.String(),
			Score:     a.Score,
			Threshold: a.Threshold,
			Timestamp: a.Timestamp,
			Message:   a.Message,
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"alerts": response,
		"since":  since,
	})
}

// handleResetReference handles POST /v1/drift/reset/{feature}
func (h *DriftHandler) handleResetReference(w http.ResponseWriter, r *http.Request) {
	if h.detector == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "drift detector not configured")
		return
	}

	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	if err := h.detector.ResetReference(feature); err != nil {
		if errors.Is(err, drift.ErrFeatureNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "feature not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Reference distribution reset for feature: " + feature,
	})
}

// RegisterFeatureRequest represents a request to register a feature for monitoring.
type RegisterFeatureRequest struct {
	Name string `json:"name"`
	Type string `json:"type"` // "numeric" or "categorical"
}

// handleRegisterFeature handles POST /v1/drift/register
func (h *DriftHandler) handleRegisterFeature(w http.ResponseWriter, r *http.Request) {
	if h.detector == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "drift detector not configured")
		return
	}

	var req RegisterFeatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	featureType := drift.TypeNumeric
	if req.Type == "categorical" {
		featureType = drift.TypeCategorical
	}

	h.detector.RegisterFeature(req.Name, featureType)

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"feature": req.Name,
		"type":    req.Type,
	})
}

func (h *DriftHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *DriftHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
