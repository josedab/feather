package server

import (
	"errors"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/extensions/freshness"
)

// FreshnessHandler handles HTTP requests for adaptive freshness APIs.
type FreshnessHandler struct {
	manager *freshness.Manager
}

// NewFreshnessHandler creates a new freshness handler.
func NewFreshnessHandler(manager *freshness.Manager) *FreshnessHandler {
	return &FreshnessHandler{
		manager: manager,
	}
}

// RegisterRoutes registers freshness routes with the HTTP mux.
func (h *FreshnessHandler) RegisterRoutes(mux *http.ServeMux) {
	// Metrics endpoints
	mux.HandleFunc("GET /v1/freshness/metrics", h.handleGetAllMetrics)
	mux.HandleFunc("GET /v1/freshness/metrics/{feature}", h.handleGetFeatureMetrics)

	// TTL recommendation endpoints
	mux.HandleFunc("GET /v1/freshness/ttl/{feature}", h.handleGetTTL)
	mux.HandleFunc("GET /v1/freshness/predictions", h.handleGetAllPredictions)
	mux.HandleFunc("GET /v1/freshness/predictions/{feature}", h.handleGetPrediction)

	// Policy management endpoints
	mux.HandleFunc("GET /v1/freshness/policies", h.handleListPolicies)
	mux.HandleFunc("POST /v1/freshness/policies", h.handleCreatePolicy)
	mux.HandleFunc("GET /v1/freshness/policies/{id}", h.handleGetPolicy)
	mux.HandleFunc("PUT /v1/freshness/policies/{id}", h.handleUpdatePolicy)
	mux.HandleFunc("DELETE /v1/freshness/policies/{id}", h.handleDeletePolicy)

	// Recording endpoints
	mux.HandleFunc("POST /v1/freshness/access", h.handleRecordAccess)
	mux.HandleFunc("POST /v1/freshness/change", h.handleRecordChange)
	mux.HandleFunc("POST /v1/freshness/drift", h.handleRecordDrift)
	mux.HandleFunc("POST /v1/freshness/stale", h.handleRecordStale)

	// Stats endpoint
	mux.HandleFunc("GET /v1/freshness/stats", h.handleGetStats)

	// Evaluate all endpoint
	mux.HandleFunc("GET /v1/freshness/evaluate", h.handleEvaluateAll)
}

// handleGetAllMetrics returns metrics for all tracked features.
func (h *FreshnessHandler) handleGetAllMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := h.manager.GetAllMetrics()
	writeJSONResponse(r.Context(), w, http.StatusOK, metrics)
}

// handleGetFeatureMetrics returns metrics for a specific feature.
func (h *FreshnessHandler) handleGetFeatureMetrics(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "feature is required")
		return
	}

	accessMetrics, hasAccess := h.manager.GetAccessMetrics(feature)
	changeMetrics, hasChange := h.manager.GetChangeMetrics(feature)

	if !hasAccess && !hasChange {
		writeJSONError(r.Context(), w, http.StatusNotFound, "feature not found")
		return
	}

	response := freshness.FeatureMetrics{
		FeatureName: feature,
	}
	if hasAccess {
		response.Access = *accessMetrics
	}
	if hasChange {
		response.Change = changeMetrics
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, response)
}

// handleGetTTL returns the recommended TTL for a feature.
func (h *FreshnessHandler) handleGetTTL(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "feature is required")
		return
	}

	result := h.manager.GetTTLWithReason(feature)
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

// handleGetAllPredictions returns predictions for all tracked features.
func (h *FreshnessHandler) handleGetAllPredictions(w http.ResponseWriter, r *http.Request) {
	predictions := h.manager.GetAllPredictions()
	writeJSONResponse(r.Context(), w, http.StatusOK, predictions)
}

// handleGetPrediction returns the prediction for a specific feature.
func (h *FreshnessHandler) handleGetPrediction(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "feature is required")
		return
	}

	prediction := h.manager.GetPrediction(feature)
	writeJSONResponse(r.Context(), w, http.StatusOK, prediction)
}

// handleListPolicies returns all policies.
func (h *FreshnessHandler) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	policies := h.manager.ListPolicies()
	writeJSONResponse(r.Context(), w, http.StatusOK, policies)
}

// PolicyRequest represents a policy create/update request.
type PolicyRequest struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Type           freshness.PolicyType   `json:"type"`
	FeaturePattern string                 `json:"feature_pattern"`
	Priority       int                    `json:"priority"`
	Enabled        bool                   `json:"enabled"`
	Config         freshness.PolicyConfig `json:"config"`
}

// handleCreatePolicy creates a new policy.
func (h *FreshnessHandler) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	var req PolicyRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	policy := &freshness.Policy{
		ID:             req.ID,
		Name:           req.Name,
		Type:           req.Type,
		FeaturePattern: req.FeaturePattern,
		Priority:       req.Priority,
		Enabled:        req.Enabled,
		Config:         req.Config,
	}

	if err := h.manager.RegisterPolicy(policy); err != nil {
		if errors.Is(err, freshness.ErrPolicyExists) {
			writeJSONError(r.Context(), w, http.StatusConflict, err.Error())
		} else if errors.Is(err, freshness.ErrInvalidPolicy) {
			writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		} else {
			writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusCreated, policy)
}

// handleGetPolicy retrieves a policy by ID.
func (h *FreshnessHandler) handleGetPolicy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "policy id is required")
		return
	}

	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		if errors.Is(err, freshness.ErrPolicyNotFound) {
			writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		} else {
			writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, policy)
}

// handleUpdatePolicy updates an existing policy.
func (h *FreshnessHandler) handleUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "policy id is required")
		return
	}

	var req PolicyRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	policy := &freshness.Policy{
		ID:             id,
		Name:           req.Name,
		Type:           req.Type,
		FeaturePattern: req.FeaturePattern,
		Priority:       req.Priority,
		Enabled:        req.Enabled,
		Config:         req.Config,
	}

	if err := h.manager.UpdatePolicy(policy); err != nil {
		if errors.Is(err, freshness.ErrPolicyNotFound) {
			writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		} else if errors.Is(err, freshness.ErrInvalidPolicy) {
			writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		} else {
			writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, policy)
}

// handleDeletePolicy deletes a policy.
func (h *FreshnessHandler) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "policy id is required")
		return
	}

	if err := h.manager.DeletePolicy(id); err != nil {
		if errors.Is(err, freshness.ErrPolicyNotFound) {
			writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		} else {
			writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AccessRecordRequest represents an access record request.
type AccessRecordRequest struct {
	Feature  string        `json:"feature"`
	Latency  time.Duration `json:"latency"`
	CacheHit bool          `json:"cache_hit"`
}

// handleRecordAccess records a feature access.
func (h *FreshnessHandler) handleRecordAccess(w http.ResponseWriter, r *http.Request) {
	var req AccessRecordRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Feature == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "feature is required")
		return
	}

	h.manager.RecordAccess(req.Feature, req.Latency, req.CacheHit)
	w.WriteHeader(http.StatusAccepted)
}

// ChangeRecordRequest represents a change record request.
type ChangeRecordRequest struct {
	Feature  string  `json:"feature"`
	OldValue float64 `json:"old_value"`
	NewValue float64 `json:"new_value"`
}

// handleRecordChange records a feature value change.
func (h *FreshnessHandler) handleRecordChange(w http.ResponseWriter, r *http.Request) {
	var req ChangeRecordRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Feature == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "feature is required")
		return
	}

	h.manager.RecordChange(req.Feature, req.OldValue, req.NewValue)
	w.WriteHeader(http.StatusAccepted)
}

// DriftRecordRequest represents a drift score record request.
type DriftRecordRequest struct {
	Feature    string  `json:"feature"`
	DriftScore float64 `json:"drift_score"`
}

// handleRecordDrift records a drift score.
func (h *FreshnessHandler) handleRecordDrift(w http.ResponseWriter, r *http.Request) {
	var req DriftRecordRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Feature == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "feature is required")
		return
	}

	h.manager.RecordDriftScore(req.Feature, req.DriftScore)
	w.WriteHeader(http.StatusAccepted)
}

// StaleRecordRequest represents a stale serve record request.
type StaleRecordRequest struct {
	Feature string `json:"feature"`
}

// handleRecordStale records a stale serve event.
func (h *FreshnessHandler) handleRecordStale(w http.ResponseWriter, r *http.Request) {
	var req StaleRecordRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Feature == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "feature is required")
		return
	}

	h.manager.RecordStaleServe(req.Feature)
	w.WriteHeader(http.StatusAccepted)
}

// handleGetStats returns freshness manager statistics.
func (h *FreshnessHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}

// handleEvaluateAll evaluates freshness for all tracked features.
func (h *FreshnessHandler) handleEvaluateAll(w http.ResponseWriter, r *http.Request) {
	results := h.manager.EvaluateAll()
	writeJSONResponse(r.Context(), w, http.StatusOK, results)
}
