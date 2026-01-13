package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/feather-store/feather/internal/impact"
)

// ImpactHandler handles feature impact API requests.
type ImpactHandler struct {
	tracker *impact.ImpactTracker
}

// NewImpactHandler creates a new impact handler.
func NewImpactHandler() *ImpactHandler {
	return &ImpactHandler{
		tracker: impact.NewImpactTracker(),
	}
}

// RegisterRoutes registers impact API routes.
func (h *ImpactHandler) RegisterRoutes(mux *http.ServeMux) {
	// Feature usage
	mux.HandleFunc("POST /v1/impact/access", h.handleRecordAccess)
	mux.HandleFunc("GET /v1/impact/features", h.handleGetFeatures)
	mux.HandleFunc("GET /v1/impact/features/{feature}", h.handleGetFeatureUsage)
	mux.HandleFunc("POST /v1/impact/features/{feature}/dependencies", h.handleSetDependencies)

	// Model usage
	mux.HandleFunc("POST /v1/impact/models", h.handleRegisterModel)
	mux.HandleFunc("POST /v1/impact/models/{model}/inference", h.handleRecordInference)
	mux.HandleFunc("GET /v1/impact/models", h.handleGetModels)
	mux.HandleFunc("GET /v1/impact/models/{model}", h.handleGetModelUsage)

	// Impact scores
	mux.HandleFunc("GET /v1/impact/scores/{feature}", h.handleGetImpactScore)
	mux.HandleFunc("GET /v1/impact/scores", h.handleGetTopFeatures)
	mux.HandleFunc("GET /v1/impact/low-impact", h.handleGetLowImpact)
	mux.HandleFunc("GET /v1/impact/unused", h.handleGetUnused)

	// Lineage
	mux.HandleFunc("GET /v1/impact/lineage/{feature}", h.handleGetLineage)
	mux.HandleFunc("GET /v1/impact/graph", h.handleGetGraph)

	// Deprecation
	mux.HandleFunc("POST /v1/impact/deprecations", h.handleRequestDeprecation)
	mux.HandleFunc("GET /v1/impact/deprecations", h.handleGetDeprecations)
	mux.HandleFunc("GET /v1/impact/deprecations/{feature}", h.handleGetDeprecation)
	mux.HandleFunc("POST /v1/impact/deprecations/{feature}/approve", h.handleApproveDeprecation)

	// Reports
	mux.HandleFunc("GET /v1/impact/report", h.handleGetReport)
}

// GetTracker returns the impact tracker for integration.
func (h *ImpactHandler) GetTracker() *impact.ImpactTracker {
	return h.tracker
}

// ImpactAccessRequest represents a feature access record for impact tracking.
type ImpactAccessRequest struct {
	Feature   string  `json:"feature"`
	LatencyMs float64 `json:"latency_ms"`
	IsError   bool    `json:"is_error"`
	IsNull    bool    `json:"is_null"`
}

// handleRecordAccess handles POST /v1/impact/access
func (h *ImpactHandler) handleRecordAccess(w http.ResponseWriter, r *http.Request) {
	var req ImpactAccessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature is required")
		return
	}

	h.tracker.RecordAccess(req.Feature, req.LatencyMs, req.IsError, req.IsNull)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"feature": req.Feature,
	})
}

// handleGetFeatures handles GET /v1/impact/features
func (h *ImpactHandler) handleGetFeatures(w http.ResponseWriter, r *http.Request) {
	features := h.tracker.GetAllFeatureUsage()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": features,
		"count":    len(features),
	})
}

// handleGetFeatureUsage handles GET /v1/impact/features/{feature}
func (h *ImpactHandler) handleGetFeatureUsage(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	usage := h.tracker.GetFeatureUsage(feature)
	if usage == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "feature not found")
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, usage)
}

// SetDependenciesRequest represents a request to set feature dependencies.
type SetDependenciesRequest struct {
	Dependencies []string `json:"dependencies"`
}

// handleSetDependencies handles POST /v1/impact/features/{feature}/dependencies
func (h *ImpactHandler) handleSetDependencies(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	var req SetDependenciesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.tracker.SetDependencies(feature, req.Dependencies)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"feature":      feature,
		"dependencies": req.Dependencies,
	})
}

// RegisterModelRequest represents a request to register a model.
type RegisterModelRequest struct {
	ModelID      string            `json:"model_id"`
	ModelVersion string            `json:"model_version"`
	Features     []string          `json:"features"`
	Environment  string            `json:"environment"`
	Endpoint     string            `json:"endpoint"`
	Metadata     map[string]string `json:"metadata"`
}

// handleRegisterModel handles POST /v1/impact/models
func (h *ImpactHandler) handleRegisterModel(w http.ResponseWriter, r *http.Request) {
	var req RegisterModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ModelID == "" || len(req.Features) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model_id and features are required")
		return
	}

	model := &impact.ModelUsage{
		ModelID:      req.ModelID,
		ModelVersion: req.ModelVersion,
		Features:     req.Features,
		Environment:  req.Environment,
		Endpoint:     req.Endpoint,
		Metadata:     req.Metadata,
	}

	h.tracker.RegisterModel(model)

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"model":   model,
	})
}

// RecordInferenceRequest represents a request to record a model inference.
type RecordInferenceRequest struct {
	LatencyMs float64 `json:"latency_ms"`
	IsError   bool    `json:"is_error"`
}

// handleRecordInference handles POST /v1/impact/models/{model}/inference
func (h *ImpactHandler) handleRecordInference(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("model")
	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model id required")
		return
	}

	var req RecordInferenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.tracker.RecordInference(modelID, req.LatencyMs, req.IsError)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"model_id": modelID,
	})
}

// handleGetModels handles GET /v1/impact/models
func (h *ImpactHandler) handleGetModels(w http.ResponseWriter, r *http.Request) {
	models := h.tracker.GetAllModelUsage()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"models": models,
		"count":  len(models),
	})
}

// handleGetModelUsage handles GET /v1/impact/models/{model}
func (h *ImpactHandler) handleGetModelUsage(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("model")
	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model id required")
		return
	}

	model := h.tracker.GetModelUsage(modelID)
	if model == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "model not found")
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, model)
}

// handleGetImpactScore handles GET /v1/impact/scores/{feature}
func (h *ImpactHandler) handleGetImpactScore(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	score := h.tracker.CalculateImpactScore(feature)
	if score == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "feature not found")
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, score)
}

// handleGetTopFeatures handles GET /v1/impact/scores
func (h *ImpactHandler) handleGetTopFeatures(w http.ResponseWriter, r *http.Request) {
	n := 20
	if nStr := r.URL.Query().Get("n"); nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 {
			n = parsed
		}
	}

	scores := h.tracker.GetTopFeaturesByImpact(n)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"scores": scores,
		"count":  len(scores),
	})
}

// handleGetLowImpact handles GET /v1/impact/low-impact
func (h *ImpactHandler) handleGetLowImpact(w http.ResponseWriter, r *http.Request) {
	threshold := 20.0
	if threshStr := r.URL.Query().Get("threshold"); threshStr != "" {
		if parsed, err := strconv.ParseFloat(threshStr, 64); err == nil {
			threshold = parsed
		}
	}

	features := h.tracker.GetLowImpactFeatures(threshold)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features":  features,
		"count":     len(features),
		"threshold": threshold,
	})
}

// handleGetUnused handles GET /v1/impact/unused
func (h *ImpactHandler) handleGetUnused(w http.ResponseWriter, r *http.Request) {
	days := 30
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if parsed, err := strconv.Atoi(daysStr); err == nil && parsed > 0 {
			days = parsed
		}
	}

	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	features := h.tracker.GetUnusedFeatures(since)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": features,
		"count":    len(features),
		"since":    since.Format(time.RFC3339),
	})
}

// handleGetLineage handles GET /v1/impact/lineage/{feature}
func (h *ImpactHandler) handleGetLineage(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	lineage := h.tracker.GetFeatureLineage(feature)
	if lineage == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "feature not found")
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, lineage)
}

// handleGetGraph handles GET /v1/impact/graph
func (h *ImpactHandler) handleGetGraph(w http.ResponseWriter, r *http.Request) {
	graph := h.tracker.GetDependencyGraph()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"graph":         graph,
		"feature_count": len(graph),
	})
}

// DeprecationRequestBody represents a deprecation request body.
type DeprecationRequestBody struct {
	Feature     string `json:"feature"`
	Reason      string `json:"reason"`
	RequestedBy string `json:"requested_by"`
	TargetDate  string `json:"target_date"`
	Replacement string `json:"replacement"`
}

// handleRequestDeprecation handles POST /v1/impact/deprecations
func (h *ImpactHandler) handleRequestDeprecation(w http.ResponseWriter, r *http.Request) {
	var req DeprecationRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Feature == "" || req.Reason == "" || req.RequestedBy == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature, reason, and requested_by are required")
		return
	}

	targetDate := time.Now().Add(30 * 24 * time.Hour) // Default 30 days
	if req.TargetDate != "" {
		if parsed, err := time.Parse(time.RFC3339, req.TargetDate); err == nil {
			targetDate = parsed
		}
	}

	deprecation := &impact.DeprecationRequest{
		Feature:     req.Feature,
		Reason:      req.Reason,
		RequestedBy: req.RequestedBy,
		TargetDate:  targetDate,
		Replacement: req.Replacement,
	}

	if err := h.tracker.RequestDeprecation(deprecation); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":     true,
		"deprecation": deprecation,
	})
}

// handleGetDeprecations handles GET /v1/impact/deprecations
func (h *ImpactHandler) handleGetDeprecations(w http.ResponseWriter, r *http.Request) {
	deprecations := h.tracker.GetAllDeprecations()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"deprecations": deprecations,
		"count":        len(deprecations),
	})
}

// handleGetDeprecation handles GET /v1/impact/deprecations/{feature}
func (h *ImpactHandler) handleGetDeprecation(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	deprecation := h.tracker.GetDeprecationRequest(feature)
	if deprecation == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "deprecation request not found")
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, deprecation)
}

// handleApproveDeprecation handles POST /v1/impact/deprecations/{feature}/approve
func (h *ImpactHandler) handleApproveDeprecation(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	if err := h.tracker.ApproveDeprecation(feature); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"feature": feature,
		"status":  "approved",
	})
}

// handleGetReport handles GET /v1/impact/report
func (h *ImpactHandler) handleGetReport(w http.ResponseWriter, r *http.Request) {
	report := h.tracker.GenerateReport()
	h.writeJSON(r.Context(), w, http.StatusOK, report)
}

func (h *ImpactHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *ImpactHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
