package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/extensions/obsconsole"
)

// ---------------------------------------------------------------------------
// ObsConsoleHandler
// ---------------------------------------------------------------------------

// ObsConsoleHandler exposes unified observability console endpoints.
type ObsConsoleHandler struct {
	console *obsconsole.Console
}

// NewObsConsoleHandler creates a new ObsConsoleHandler.
func NewObsConsoleHandler(console *obsconsole.Console) *ObsConsoleHandler {
	return &ObsConsoleHandler{console: console}
}

// RegisterRoutes registers observability console API routes.
func (h *ObsConsoleHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/observability/dashboard", h.handleDashboard)
	mux.HandleFunc("POST /v1/observability/features/register", h.handleRegister)
	mux.HandleFunc("POST /v1/observability/features/update", h.handleRecordUpdate)
	mux.HandleFunc("POST /v1/observability/quality", h.handleUpdateQuality)
	mux.HandleFunc("POST /v1/observability/alerts", h.handleAddAlert)
	mux.HandleFunc("POST /v1/observability/alerts/{id}/resolve", h.handleResolve)
	mux.HandleFunc("GET /v1/observability/alerts", h.handleGetAlerts)
	mux.HandleFunc("POST /v1/observability/costs", h.handleSetCost)
	mux.HandleFunc("GET /v1/observability/grafana", h.handleGrafana)
}

func (h *ObsConsoleHandler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	snap := h.console.GetSnapshot()
	writeJSONResponse(r.Context(), w, http.StatusOK, snap)
}

func (h *ObsConsoleHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Feature    string `json:"feature"`
		SLASeconds int    `json:"sla_seconds"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	h.console.RegisterFeature(req.Feature, time.Duration(req.SLASeconds)*time.Second)
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]string{"feature": req.Feature})
}

func (h *ObsConsoleHandler) handleRecordUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Feature string `json:"feature"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	h.console.RecordUpdate(req.Feature)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"updated": req.Feature})
}

func (h *ObsConsoleHandler) handleUpdateQuality(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Feature      string  `json:"feature"`
		Completeness float64 `json:"completeness"`
		Consistency  float64 `json:"consistency"`
		Timeliness   float64 `json:"timeliness"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	h.console.UpdateQuality(req.Feature, req.Completeness, req.Consistency, req.Timeliness)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"updated": req.Feature})
}

func (h *ObsConsoleHandler) handleAddAlert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type     string                   `json:"type"`
		Severity obsconsole.AlertSeverity `json:"severity"`
		Feature  string                   `json:"feature"`
		Message  string                   `json:"message"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	alert := h.console.AddAlert(req.Type, req.Severity, req.Feature, req.Message)
	writeJSONResponse(r.Context(), w, http.StatusCreated, alert)
}

func (h *ObsConsoleHandler) handleResolve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.console.ResolveAlert(id) {
		writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"resolved": id})
	} else {
		writeJSONError(r.Context(), w, http.StatusNotFound, "alert not found: "+id)
	}
}

func (h *ObsConsoleHandler) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active") == "true"
	alerts := h.console.GetAlerts(activeOnly)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

func (h *ObsConsoleHandler) handleSetCost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Feature string  `json:"feature"`
		Cost    float64 `json:"cost"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	h.console.SetCost(req.Feature, req.Cost)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"recorded": req.Feature})
}

func (h *ObsConsoleHandler) handleGrafana(w http.ResponseWriter, r *http.Request) {
	dashboard := h.console.GenerateGrafanaDashboard()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(dashboard)); err != nil {
		slog.Debug("failed to write grafana dashboard response", "error", err)
	}
}
