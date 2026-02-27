package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/extensions/consistencyvalidator"
)

// ConsistencyValidatorHandler handles online/offline consistency API requests.
type ConsistencyValidatorHandler struct {
	validator *consistencyvalidator.Validator
}

// NewConsistencyValidatorHandler creates a new consistency validator handler.
func NewConsistencyValidatorHandler(v *consistencyvalidator.Validator) *ConsistencyValidatorHandler {
	return &ConsistencyValidatorHandler{validator: v}
}

// RegisterRoutes registers consistency validator API routes.
func (h *ConsistencyValidatorHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/consistency/features", h.handleListFeatures)
	mux.HandleFunc("POST /v1/consistency/register", h.handleRegister)
	mux.HandleFunc("POST /v1/consistency/record/online", h.handleRecordOnline)
	mux.HandleFunc("POST /v1/consistency/record/offline", h.handleRecordOffline)
	mux.HandleFunc("POST /v1/consistency/check", h.handleCheckAll)
	mux.HandleFunc("GET /v1/consistency/check/{feature}", h.handleCheckFeature)
	mux.HandleFunc("GET /v1/consistency/check-extended/{feature}", h.handleCheckExtended)
	mux.HandleFunc("GET /v1/consistency/reports", h.handleGetReports)
	mux.HandleFunc("GET /v1/consistency/alerts", h.handleGetAlerts)
	mux.HandleFunc("GET /v1/consistency/stats", h.handleGetStats)
	mux.HandleFunc("GET /v1/consistency/snapshots", h.handleSnapshots)
	mux.HandleFunc("POST /v1/consistency/config/{feature}", h.handleSetFeatureConfig)
	mux.HandleFunc("GET /v1/consistency/config/{feature}", h.handleGetFeatureConfig)
}

func (h *ConsistencyValidatorHandler) handleListFeatures(w http.ResponseWriter, r *http.Request) {
	features := h.validator.ListFeatures()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": features,
	})
}

func (h *ConsistencyValidatorHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
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

	h.validator.RegisterFeature(req.Name)
	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"feature": req.Name,
	})
}

type recordValueRequest struct {
	Feature string  `json:"feature"`
	Value   float64 `json:"value"`
}

func (h *ConsistencyValidatorHandler) handleRecordOnline(w http.ResponseWriter, r *http.Request) {
	var req recordValueRequest
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	h.validator.RecordOnline(req.Feature, req.Value)
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *ConsistencyValidatorHandler) handleRecordOffline(w http.ResponseWriter, r *http.Request) {
	var req recordValueRequest
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	h.validator.RecordOffline(req.Feature, req.Value)
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *ConsistencyValidatorHandler) handleCheckAll(w http.ResponseWriter, r *http.Request) {
	reports := h.validator.CheckAll()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"reports": reports,
		"count":   len(reports),
	})
}

func (h *ConsistencyValidatorHandler) handleCheckFeature(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	report, err := h.validator.Check(feature)
	if err != nil {
		if errors.Is(err, consistencyvalidator.ErrFeatureNotRegistered) {
			h.writeError(r.Context(), w, http.StatusNotFound, "feature not registered")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, report)
}

func (h *ConsistencyValidatorHandler) handleGetReports(w http.ResponseWriter, r *http.Request) {
	feature := r.URL.Query().Get("feature")
	reports := h.validator.GetReports(feature, 50)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"reports": reports,
		"count":   len(reports),
	})
}

func (h *ConsistencyValidatorHandler) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-24 * time.Hour)
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	alerts := h.validator.GetAlerts(since)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

func (h *ConsistencyValidatorHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.validator.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *ConsistencyValidatorHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *ConsistencyValidatorHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}

func (h *ConsistencyValidatorHandler) handleCheckExtended(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	report, err := h.validator.CheckExtended(feature)
	if err != nil {
		if errors.Is(err, consistencyvalidator.ErrFeatureNotRegistered) {
			h.writeError(r.Context(), w, http.StatusNotFound, "feature not registered")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, report)
}

func (h *ConsistencyValidatorHandler) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	snapshots := h.validator.Snapshot()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"snapshots": snapshots,
		"count":     len(snapshots),
	})
}

func (h *ConsistencyValidatorHandler) handleSetFeatureConfig(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	var cfg consistencyvalidator.PerFeatureConfig
	if err := strictDecode(r.Body, &cfg); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	h.validator.SetFeatureConfig(feature, cfg)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"feature": feature,
	})
}

func (h *ConsistencyValidatorHandler) handleGetFeatureConfig(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	cfg := h.validator.GetFeatureConfig(feature)
	h.writeJSON(r.Context(), w, http.StatusOK, cfg)
}
