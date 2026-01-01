package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/observability"
	"github.com/feather-store/feather/internal/storage"
)

// ObservabilityHandler handles observability API requests.
type ObservabilityHandler struct {
	stack *observability.ObservabilityStack
	store *storage.Store
}

// NewObservabilityHandler creates a new observability handler.
func NewObservabilityHandler(store *storage.Store) *ObservabilityHandler {
	return &ObservabilityHandler{
		stack: observability.NewObservabilityStack(store),
		store: store,
	}
}

// RegisterRoutes registers observability API routes.
func (h *ObservabilityHandler) RegisterRoutes(mux *http.ServeMux) {
	// Metrics
	mux.HandleFunc("GET /v1/observability/metrics", h.handleGetMetrics)
	mux.HandleFunc("GET /v1/observability/metrics/{feature}", h.handleGetFeatureMetrics)
	mux.HandleFunc("GET /v1/observability/metrics/top", h.handleGetTopFeatures)

	// Freshness
	mux.HandleFunc("GET /v1/observability/freshness", h.handleCheckFreshness)
	mux.HandleFunc("POST /v1/observability/freshness/threshold", h.handleSetFreshnessThreshold)

	// Usage patterns
	mux.HandleFunc("GET /v1/observability/usage/{feature}", h.handleGetUsagePattern)

	// Quality
	mux.HandleFunc("GET /v1/observability/quality/{feature}", h.handleGetQualityScore)
	mux.HandleFunc("POST /v1/observability/quality/rules", h.handleAddQualityRule)
	mux.HandleFunc("GET /v1/observability/quality/violations", h.handleGetViolations)

	// Alerts
	mux.HandleFunc("GET /v1/observability/alerts", h.handleGetAlerts)
	mux.HandleFunc("POST /v1/observability/alerts/rules", h.handleAddAlertRule)
	mux.HandleFunc("POST /v1/observability/alerts/{id}/ack", h.handleAckAlert)
	mux.HandleFunc("POST /v1/observability/alerts/{id}/resolve", h.handleResolveAlert)
}

// GetStack returns the observability stack for integration.
func (h *ObservabilityHandler) GetStack() *observability.ObservabilityStack {
	return h.stack
}

// handleGetMetrics handles GET /v1/observability/metrics
func (h *ObservabilityHandler) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := h.stack.Metrics.GetAllMetrics()

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"metrics": metrics,
		"count":   len(metrics),
	})
}

// handleGetFeatureMetrics handles GET /v1/observability/metrics/{feature}
func (h *ObservabilityHandler) handleGetFeatureMetrics(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(w, http.StatusBadRequest, "feature name required")
		return
	}

	metrics := h.stack.Metrics.GetMetrics(feature)
	if metrics == nil {
		h.writeError(w, http.StatusNotFound, "no metrics for feature")
		return
	}

	h.writeJSON(w, http.StatusOK, metrics)
}

// handleGetTopFeatures handles GET /v1/observability/metrics/top
func (h *ObservabilityHandler) handleGetTopFeatures(w http.ResponseWriter, r *http.Request) {
	n := 10
	if nStr := r.URL.Query().Get("n"); nStr != "" {
		if parsed, err := json.Number(nStr).Int64(); err == nil {
			n = int(parsed)
		}
	}

	metrics := h.stack.Metrics.GetTopFeatures(n)

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"features": metrics,
		"count":    len(metrics),
	})
}

// handleCheckFreshness handles GET /v1/observability/freshness
func (h *ObservabilityHandler) handleCheckFreshness(w http.ResponseWriter, r *http.Request) {
	alerts := h.stack.Freshness.Check(r.Context())

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

// FreshnessThresholdRequest represents a freshness threshold request.
type FreshnessThresholdRequest struct {
	Feature   string `json:"feature"`
	Threshold string `json:"threshold"` // Duration string like "1h", "30m"
}

// handleSetFreshnessThreshold handles POST /v1/observability/freshness/threshold
func (h *ObservabilityHandler) handleSetFreshnessThreshold(w http.ResponseWriter, r *http.Request) {
	var req FreshnessThresholdRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Feature == "" {
		h.writeError(w, http.StatusBadRequest, "feature is required")
		return
	}

	threshold, err := time.ParseDuration(req.Threshold)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid threshold duration")
		return
	}

	h.stack.Freshness.SetThreshold(req.Feature, threshold)

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"feature":   req.Feature,
		"threshold": threshold.String(),
	})
}

// handleGetUsagePattern handles GET /v1/observability/usage/{feature}
func (h *ObservabilityHandler) handleGetUsagePattern(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(w, http.StatusBadRequest, "feature name required")
		return
	}

	pattern := h.stack.Usage.GetPattern(feature)
	if pattern == nil {
		h.writeError(w, http.StatusNotFound, "no usage data for feature")
		return
	}

	h.writeJSON(w, http.StatusOK, pattern)
}

// handleGetQualityScore handles GET /v1/observability/quality/{feature}
func (h *ObservabilityHandler) handleGetQualityScore(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(w, http.StatusBadRequest, "feature name required")
		return
	}

	score := h.stack.Quality.GetScore(feature)
	if score == nil {
		h.writeError(w, http.StatusNotFound, "no quality score for feature")
		return
	}

	h.writeJSON(w, http.StatusOK, score)
}

// QualityRuleRequest represents a quality rule request.
type QualityRuleRequest struct {
	Name     string                 `json:"name"`
	Feature  string                 `json:"feature"`
	RuleType string                 `json:"rule_type"`
	Config   map[string]interface{} `json:"config"`
	Severity string                 `json:"severity"`
}

// handleAddQualityRule handles POST /v1/observability/quality/rules
func (h *ObservabilityHandler) handleAddQualityRule(w http.ResponseWriter, r *http.Request) {
	var req QualityRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Feature == "" || req.RuleType == "" {
		h.writeError(w, http.StatusBadRequest, "name, feature, and rule_type are required")
		return
	}

	rule := &observability.QualityRule{
		Name:     req.Name,
		Feature:  req.Feature,
		RuleType: req.RuleType,
		Config:   req.Config,
		Severity: req.Severity,
		Enabled:  true,
	}

	if rule.Severity == "" {
		rule.Severity = "warning"
	}

	h.stack.Quality.AddRule(rule)

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"rule":    rule,
	})
}

// handleGetViolations handles GET /v1/observability/quality/violations
func (h *ObservabilityHandler) handleGetViolations(w http.ResponseWriter, r *http.Request) {
	feature := r.URL.Query().Get("feature")
	since := time.Now().Add(-24 * time.Hour)

	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	violations := h.stack.Quality.GetViolations(feature, since)

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"violations": violations,
		"count":      len(violations),
	})
}

// handleGetAlerts handles GET /v1/observability/alerts
func (h *ObservabilityHandler) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	alertType := observability.AlertType(r.URL.Query().Get("type"))
	feature := r.URL.Query().Get("feature")
	since := time.Now().Add(-24 * time.Hour)

	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	alerts := h.stack.Alerts.GetAlerts(alertType, feature, since)

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

// AlertRuleRequest represents an alert rule request.
type AlertRuleRequest struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Feature   string            `json:"feature"`
	Condition string            `json:"condition"`
	Threshold float64           `json:"threshold"`
	Duration  string            `json:"duration,omitempty"`
	Severity  string            `json:"severity"`
	Cooldown  string            `json:"cooldown,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// handleAddAlertRule handles POST /v1/observability/alerts/rules
func (h *ObservabilityHandler) handleAddAlertRule(w http.ResponseWriter, r *http.Request) {
	var req AlertRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Type == "" || req.Feature == "" || req.Condition == "" {
		h.writeError(w, http.StatusBadRequest, "name, type, feature, and condition are required")
		return
	}

	var duration, cooldown time.Duration
	if req.Duration != "" {
		var err error
		duration, err = time.ParseDuration(req.Duration)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid duration")
			return
		}
	}

	if req.Cooldown != "" {
		var err error
		cooldown, err = time.ParseDuration(req.Cooldown)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid cooldown")
			return
		}
	} else {
		cooldown = 5 * time.Minute
	}

	rule := &observability.AlertRule{
		Name:      req.Name,
		Type:      observability.AlertType(req.Type),
		Feature:   req.Feature,
		Condition: req.Condition,
		Threshold: req.Threshold,
		Duration:  duration,
		Severity:  observability.AlertSeverity(req.Severity),
		Cooldown:  cooldown,
		Labels:    req.Labels,
		Enabled:   true,
	}

	if rule.Severity == "" {
		rule.Severity = observability.SeverityWarning
	}

	h.stack.Alerts.AddRule(rule)

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"rule":    rule,
	})
}

// handleAckAlert handles POST /v1/observability/alerts/{id}/ack
func (h *ObservabilityHandler) handleAckAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "alert id required")
		return
	}

	var req struct {
		By string `json:"by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.By = "anonymous"
	}

	if h.stack.Alerts.Acknowledge(id, req.By) {
		h.writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"id":      id,
		})
	} else {
		h.writeError(w, http.StatusNotFound, "alert not found")
	}
}

// handleResolveAlert handles POST /v1/observability/alerts/{id}/resolve
func (h *ObservabilityHandler) handleResolveAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "alert id required")
		return
	}

	if h.stack.Alerts.Resolve(id) {
		h.writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"id":      id,
		})
	} else {
		h.writeError(w, http.StatusNotFound, "alert not found")
	}
}

func (h *ObservabilityHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *ObservabilityHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}
