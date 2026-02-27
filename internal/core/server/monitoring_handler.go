package server

import (
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/platform/monitoring"
)

// MonitoringHandler provides HTTP endpoints for unified feature monitoring.
type MonitoringHandler struct {
	manager *monitoring.Manager
}

// NewMonitoringHandler creates a new monitoring handler.
func NewMonitoringHandler(manager *monitoring.Manager) *MonitoringHandler {
	return &MonitoringHandler{manager: manager}
}

// RegisterRoutes registers monitoring API routes.
func (h *MonitoringHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/monitoring/monitors", h.handleListMonitors)
	mux.HandleFunc("POST /v1/monitoring/monitors", h.handleRegisterMonitor)
	mux.HandleFunc("GET /v1/monitoring/monitors/{id}", h.handleGetMonitor)
	mux.HandleFunc("DELETE /v1/monitoring/monitors/{id}", h.handleRemoveMonitor)
	mux.HandleFunc("POST /v1/monitoring/monitors/{id}/value", h.handleRecordValue)
	mux.HandleFunc("GET /v1/monitoring/rules", h.handleListRules)
	mux.HandleFunc("POST /v1/monitoring/rules", h.handleAddRule)
	mux.HandleFunc("DELETE /v1/monitoring/rules/{id}", h.handleRemoveRule)
	mux.HandleFunc("GET /v1/monitoring/alerts", h.handleGetAlerts)
	mux.HandleFunc("POST /v1/monitoring/alerts/{id}/ack", h.handleAcknowledgeAlert)
	mux.HandleFunc("GET /v1/monitoring/summary", h.handleSummary)
}

func (h *MonitoringHandler) handleListMonitors(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "monitoring not configured")
		return
	}
	monitors := h.manager.ListMonitors()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "monitors": monitors, "count": len(monitors),
	})
}

func (h *MonitoringHandler) handleRegisterMonitor(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "monitoring not configured")
		return
	}
	var monitor monitoring.FeatureMonitor
	if err := strictDecode(r.Body, &monitor); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.manager.RegisterMonitor(&monitor); err != nil {
		writeJSONError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "monitor": monitor})
}

func (h *MonitoringHandler) handleGetMonitor(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "monitoring not configured")
		return
	}
	monitor, err := h.manager.GetMonitor(r.PathValue("id"))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "monitor": monitor})
}

func (h *MonitoringHandler) handleRemoveMonitor(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "monitoring not configured")
		return
	}
	if err := h.manager.RemoveMonitor(r.PathValue("id")); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "monitor removed"})
}

func (h *MonitoringHandler) handleRecordValue(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "monitoring not configured")
		return
	}
	var req struct {
		Value float64 `json:"value"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.manager.RecordValue(r.PathValue("id"), req.Value); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "value recorded"})
}

func (h *MonitoringHandler) handleListRules(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "monitoring not configured")
		return
	}
	rules := h.manager.ListRules()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "rules": rules, "count": len(rules),
	})
}

func (h *MonitoringHandler) handleAddRule(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "monitoring not configured")
		return
	}
	var rule monitoring.AlertRule
	if err := strictDecode(r.Body, &rule); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.manager.AddRule(&rule); err != nil {
		writeJSONError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "rule": rule})
}

func (h *MonitoringHandler) handleRemoveRule(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "monitoring not configured")
		return
	}
	if err := h.manager.RemoveRule(r.PathValue("id")); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "rule removed"})
}

func (h *MonitoringHandler) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "monitoring not configured")
		return
	}
	since := time.Now().Add(-24 * time.Hour)
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		}
	}
	alerts := h.manager.GetAlerts(since)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "alerts": alerts, "count": len(alerts),
	})
}

func (h *MonitoringHandler) handleAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "monitoring not configured")
		return
	}
	if err := h.manager.AcknowledgeAlert(r.PathValue("id")); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "alert acknowledged"})
}

func (h *MonitoringHandler) handleSummary(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "monitoring not configured")
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "summary": h.manager.Summary(),
	})
}
