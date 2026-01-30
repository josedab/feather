package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/platform/realtimemonitor"
)

// RealtimeMonitorHandler handles the real-time monitoring dashboard API.
type RealtimeMonitorHandler struct {
	dashboard *realtimemonitor.Dashboard
}

// NewRealtimeMonitorHandler creates a new monitoring dashboard handler.
func NewRealtimeMonitorHandler(dashboard *realtimemonitor.Dashboard) *RealtimeMonitorHandler {
	return &RealtimeMonitorHandler{dashboard: dashboard}
}

// RegisterRoutes registers monitoring dashboard API routes.
func (h *RealtimeMonitorHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/monitor/dashboard", h.handleSnapshot)
	mux.HandleFunc("GET /v1/monitor/freshness", h.handleFreshness)
	mux.HandleFunc("POST /v1/monitor/freshness", h.handleRecordFreshness)
	mux.HandleFunc("GET /v1/monitor/latency", h.handleLatency)
	mux.HandleFunc("POST /v1/monitor/latency", h.handleRecordLatency)
	mux.HandleFunc("GET /v1/monitor/alerts", h.handleGetAlerts)
	mux.HandleFunc("POST /v1/monitor/alerts", h.handleFireAlert)
	mux.HandleFunc("POST /v1/monitor/alerts/{id}/resolve", h.handleResolveAlert)
	mux.HandleFunc("GET /v1/monitor/pipelines", h.handlePipelineHealth)
	mux.HandleFunc("POST /v1/monitor/pipelines", h.handleUpdatePipelineHealth)

	// Alerting integrations
	mux.HandleFunc("GET /v1/monitor/notifiers", h.handleListNotifiers)
	mux.HandleFunc("POST /v1/monitor/notifiers", h.handleAddNotifier)
	mux.HandleFunc("DELETE /v1/monitor/notifiers/{id}", h.handleRemoveNotifier)
	mux.HandleFunc("POST /v1/monitor/notifiers/test", h.handleTestNotifier)
	mux.HandleFunc("GET /v1/monitor/notifiers/history", h.handleNotificationHistory)
}

func (h *RealtimeMonitorHandler) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	snap := h.dashboard.Snapshot()
	h.writeJSON(r.Context(), w, http.StatusOK, snap)
}

func (h *RealtimeMonitorHandler) handleFreshness(w http.ResponseWriter, r *http.Request) {
	snap := h.dashboard.Snapshot()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"freshness": snap.Freshness,
		"count":     len(snap.Freshness),
	})
}

type recordFreshnessReq struct {
	Feature     string `json:"feature"`
	Group       string `json:"group,omitempty"`
	LastUpdated string `json:"last_updated,omitempty"`
}

func (h *RealtimeMonitorHandler) handleRecordFreshness(w http.ResponseWriter, r *http.Request) {
	var req recordFreshnessReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature is required")
		return
	}

	lastUpdated := time.Now()
	if req.LastUpdated != "" {
		if parsed, err := time.Parse(time.RFC3339, req.LastUpdated); err == nil {
			lastUpdated = parsed
		}
	}
	h.dashboard.RecordFreshness(req.Feature, req.Group, lastUpdated)
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *RealtimeMonitorHandler) handleLatency(w http.ResponseWriter, r *http.Request) {
	snap := h.dashboard.Snapshot()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"latency": snap.Latency,
		"count":   len(snap.Latency),
	})
}

type recordLatencyReq struct {
	Endpoint  string  `json:"endpoint"`
	LatencyMs float64 `json:"latency_ms"`
	IsError   bool    `json:"is_error"`
}

func (h *RealtimeMonitorHandler) handleRecordLatency(w http.ResponseWriter, r *http.Request) {
	var req recordLatencyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Endpoint == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "endpoint is required")
		return
	}

	latency := time.Duration(req.LatencyMs * float64(time.Millisecond))
	h.dashboard.RecordLatency(req.Endpoint, latency, req.IsError)
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *RealtimeMonitorHandler) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	alerts := h.dashboard.GetAlerts(status)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

type fireAlertReq struct {
	Name     string                        `json:"name"`
	Severity realtimemonitor.AlertSeverity `json:"severity"`
	Message  string                        `json:"message"`
	Source   string                        `json:"source"`
	Labels   map[string]string             `json:"labels,omitempty"`
}

func (h *RealtimeMonitorHandler) handleFireAlert(w http.ResponseWriter, r *http.Request) {
	var req fireAlertReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "name is required")
		return
	}

	alert := h.dashboard.FireAlert(req.Name, req.Severity, req.Message, req.Source, req.Labels)
	h.writeJSON(r.Context(), w, http.StatusCreated, alert)
}

func (h *RealtimeMonitorHandler) handleResolveAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.dashboard.ResolveAlert(id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "alert resolved"})
}

func (h *RealtimeMonitorHandler) handlePipelineHealth(w http.ResponseWriter, r *http.Request) {
	snap := h.dashboard.Snapshot()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"pipelines": snap.PipelineHealth,
		"count":     len(snap.PipelineHealth),
	})
}

func (h *RealtimeMonitorHandler) handleUpdatePipelineHealth(w http.ResponseWriter, r *http.Request) {
	var health realtimemonitor.PipelineHealth
	if err := json.NewDecoder(r.Body).Decode(&health); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if health.PipelineID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "pipeline_id is required")
		return
	}

	h.dashboard.UpdatePipelineHealth(health)
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *RealtimeMonitorHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *RealtimeMonitorHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}

func (h *RealtimeMonitorHandler) handleListNotifiers(w http.ResponseWriter, r *http.Request) {
	notifier := h.dashboard.GetNotifier()
	if notifier == nil {
		h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"notifiers": []interface{}{}, "count": 0})
		return
	}
	notifiers := notifier.ListNotifiers()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"notifiers": notifiers,
		"count":     len(notifiers),
	})
}

func (h *RealtimeMonitorHandler) handleAddNotifier(w http.ResponseWriter, r *http.Request) {
	var config realtimemonitor.NotifierConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.dashboard.AddNotifier(config); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "notifier added"})
}

func (h *RealtimeMonitorHandler) handleRemoveNotifier(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	notifier := h.dashboard.GetNotifier()
	if notifier == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "no notifiers configured")
		return
	}
	if err := notifier.RemoveNotifier(id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "notifier removed"})
}

func (h *RealtimeMonitorHandler) handleTestNotifier(w http.ResponseWriter, r *http.Request) {
	notifier := h.dashboard.GetNotifier()
	if notifier == nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "no notifiers configured")
		return
	}

	testAlert := realtimemonitor.Alert{
		ID:       "test-alert",
		Name:     "Test Alert",
		Severity: realtimemonitor.SeverityInfo,
		Status:   realtimemonitor.AlertActive,
		Message:  "This is a test alert from Feather monitoring",
		Source:   "test",
		FiredAt:  time.Now(),
	}

	results := notifier.Notify(testAlert)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
		"count":   len(results),
	})
}

func (h *RealtimeMonitorHandler) handleNotificationHistory(w http.ResponseWriter, r *http.Request) {
	notifier := h.dashboard.GetNotifier()
	if notifier == nil {
		h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"history": []interface{}{}, "count": 0})
		return
	}

	history := notifier.GetHistory(100)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"history": history,
		"count":   len(history),
	})
}
