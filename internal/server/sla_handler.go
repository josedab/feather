package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/sla"
)

// SLAHandler handles SLA management API requests.
type SLAHandler struct {
	manager *sla.Manager
}

// NewSLAHandler creates a new SLA handler.
func NewSLAHandler(manager *sla.Manager) *SLAHandler {
	return &SLAHandler{
		manager: manager,
	}
}

// RegisterRoutes registers SLA API routes.
func (h *SLAHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/sla", h.handleListSLAs)
	mux.HandleFunc("GET /v1/sla/{name}", h.handleGetSLA)
	mux.HandleFunc("POST /v1/sla", h.handleCreateSLA)
	mux.HandleFunc("DELETE /v1/sla/{name}", h.handleDeleteSLA)
	mux.HandleFunc("GET /v1/sla/{name}/status", h.handleGetStatus)
	mux.HandleFunc("GET /v1/sla/status", h.handleGetAllStatuses)
	mux.HandleFunc("GET /v1/sla/breaches", h.handleGetBreaches)
	mux.HandleFunc("GET /v1/sla/summary", h.handleGetSummary)
	mux.HandleFunc("POST /v1/sla/evaluate", h.handleEvaluateNow)
}

// SLASpecJSON represents an SLA specification in JSON format.
type SLASpecJSON struct {
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	Type           string   `json:"type"`
	Target         float64  `json:"target"`
	Priority       string   `json:"priority"`
	Features       []string `json:"features,omitempty"`
	Groups         []string `json:"groups,omitempty"`
	Window         string   `json:"window"`
	AlertThreshold float64  `json:"alert_threshold,omitempty"`
	Owner          string   `json:"owner,omitempty"`
	Enabled        bool     `json:"enabled"`
}

// SLAStatusJSON represents an SLA status in JSON format.
type SLAStatusJSON struct {
	Name                 string  `json:"name"`
	Type                 string  `json:"type"`
	CurrentValue         float64 `json:"current_value"`
	TargetValue          float64 `json:"target_value"`
	IsBreached           bool    `json:"is_breached"`
	IsWarning            bool    `json:"is_warning"`
	BreachCount          int     `json:"breach_count"`
	LastBreachTime       string  `json:"last_breach_time,omitempty"`
	CompliancePercentage float64 `json:"compliance_percentage"`
	LastUpdated          string  `json:"last_updated"`
}

// SLABreachJSON represents an SLA breach in JSON format.
type SLABreachJSON struct {
	SLAName     string  `json:"sla_name"`
	Type        string  `json:"type"`
	Feature     string  `json:"feature,omitempty"`
	TargetValue float64 `json:"target_value"`
	ActualValue float64 `json:"actual_value"`
	Timestamp   string  `json:"timestamp"`
	Duration    string  `json:"duration,omitempty"`
	Priority    string  `json:"priority"`
	Severity    float64 `json:"severity"`
}

// CreateSLARequest represents a request to create an SLA.
type CreateSLARequest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Type           string   `json:"type"`
	Target         float64  `json:"target"`
	Priority       string   `json:"priority"`
	Features       []string `json:"features"`
	Groups         []string `json:"groups"`
	Window         string   `json:"window"`
	AlertThreshold float64  `json:"alert_threshold"`
	Owner          string   `json:"owner"`
	Enabled        bool     `json:"enabled"`
}

// handleListSLAs handles GET /v1/sla
func (h *SLAHandler) handleListSLAs(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "SLA manager not configured")
		return
	}

	specs := h.manager.ListSLAs()
	result := make([]SLASpecJSON, len(specs))

	for i, spec := range specs {
		result[i] = h.specToJSON(spec)
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"slas":  result,
		"count": len(result),
	})
}

// handleGetSLA handles GET /v1/sla/{name}
func (h *SLAHandler) handleGetSLA(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "SLA manager not configured")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "SLA name required")
		return
	}

	spec, err := h.manager.GetSLA(name)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, h.specToJSON(spec))
}

// handleCreateSLA handles POST /v1/sla
func (h *SLAHandler) handleCreateSLA(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "SLA manager not configured")
		return
	}

	var req CreateSLARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "name required")
		return
	}
	if req.Type == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "type required")
		return
	}
	if req.Target <= 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "target must be positive")
		return
	}
	if req.Window == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "window required")
		return
	}

	window, err := time.ParseDuration(req.Window)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid window duration")
		return
	}

	spec := &sla.Spec{
		Name:           req.Name,
		Description:    req.Description,
		Type:           sla.Type(req.Type),
		Target:         req.Target,
		Priority:       parsePriority(req.Priority),
		Features:       req.Features,
		Groups:         req.Groups,
		Window:         window,
		AlertThreshold: req.AlertThreshold,
		Owner:          req.Owner,
		Enabled:        req.Enabled,
	}

	if err := h.manager.RegisterSLA(spec); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"sla":     h.specToJSON(spec),
	})
}

// handleDeleteSLA handles DELETE /v1/sla/{name}
func (h *SLAHandler) handleDeleteSLA(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "SLA manager not configured")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "SLA name required")
		return
	}

	err := h.manager.UnregisterSLA(name)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleGetStatus handles GET /v1/sla/{name}/status
func (h *SLAHandler) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "SLA manager not configured")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "SLA name required")
		return
	}

	status, err := h.manager.GetStatus(name)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, h.statusToJSON(status))
}

// handleGetAllStatuses handles GET /v1/sla/status
func (h *SLAHandler) handleGetAllStatuses(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "SLA manager not configured")
		return
	}

	statuses := h.manager.GetAllStatuses()
	result := make([]SLAStatusJSON, len(statuses))

	for i, status := range statuses {
		result[i] = h.statusToJSON(status)
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"statuses": result,
		"count":    len(result),
	})
}

// handleGetBreaches handles GET /v1/sla/breaches
func (h *SLAHandler) handleGetBreaches(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "SLA manager not configured")
		return
	}

	// Parse since parameter
	sinceStr := r.URL.Query().Get("since")
	since := time.Now().Add(-24 * time.Hour) // Default to last 24 hours
	if sinceStr != "" {
		parsed, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			h.writeError(r.Context(), w, http.StatusBadRequest, "invalid since parameter, use RFC3339 format")
			return
		}
		since = parsed
	}

	breaches := h.manager.GetBreaches(since)
	result := make([]SLABreachJSON, len(breaches))

	for i, breach := range breaches {
		result[i] = h.breachToJSON(breach)
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"breaches": result,
		"count":    len(result),
		"since":    since.Format(time.RFC3339),
	})
}

// handleGetSummary handles GET /v1/sla/summary
func (h *SLAHandler) handleGetSummary(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "SLA manager not configured")
		return
	}

	summary := h.manager.GetComplianceSummary()
	h.writeJSON(r.Context(), w, http.StatusOK, summary)
}

// handleEvaluateNow handles POST /v1/sla/evaluate
func (h *SLAHandler) handleEvaluateNow(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "SLA manager not configured")
		return
	}

	h.manager.EvaluateNow(r.Context())

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"message":   "SLA evaluation triggered",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (h *SLAHandler) specToJSON(spec *sla.Spec) SLASpecJSON {
	return SLASpecJSON{
		Name:           spec.Name,
		Description:    spec.Description,
		Type:           string(spec.Type),
		Target:         spec.Target,
		Priority:       spec.Priority.String(),
		Features:       spec.Features,
		Groups:         spec.Groups,
		Window:         spec.Window.String(),
		AlertThreshold: spec.AlertThreshold,
		Owner:          spec.Owner,
		Enabled:        spec.Enabled,
	}
}

func (h *SLAHandler) statusToJSON(status *sla.Status) SLAStatusJSON {
	result := SLAStatusJSON{
		Name:                 status.Spec.Name,
		Type:                 string(status.Spec.Type),
		CurrentValue:         status.CurrentValue,
		TargetValue:          status.Spec.Target,
		IsBreached:           status.IsBreached,
		IsWarning:            status.IsWarning,
		BreachCount:          status.BreachCount,
		CompliancePercentage: status.CompliancePercentage,
		LastUpdated:          status.LastUpdated.Format(time.RFC3339),
	}

	if status.LastBreachTime != nil {
		result.LastBreachTime = status.LastBreachTime.Format(time.RFC3339)
	}

	return result
}

func (h *SLAHandler) breachToJSON(breach *sla.Breach) SLABreachJSON {
	result := SLABreachJSON{
		SLAName:     breach.SLAName,
		Type:        string(breach.Type),
		Feature:     breach.Feature,
		TargetValue: breach.TargetValue,
		ActualValue: breach.ActualValue,
		Timestamp:   breach.Timestamp.Format(time.RFC3339),
		Priority:    breach.Priority.String(),
		Severity:    breach.Severity,
	}

	if breach.Duration > 0 {
		result.Duration = breach.Duration.String()
	}

	return result
}

func parsePriority(s string) sla.Priority {
	switch s {
	case "low":
		return sla.PriorityLow
	case "medium":
		return sla.PriorityMedium
	case "high":
		return sla.PriorityHigh
	case "critical":
		return sla.PriorityCritical
	default:
		return sla.PriorityMedium
	}
}

func (h *SLAHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *SLAHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	h.writeJSON(ctx, w, status, map[string]interface{}{
		"error": message,
	})
}
