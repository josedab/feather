package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/fedlearning"
)

// FedLearningHandler handles federated learning API requests.
type FedLearningHandler struct {
	adapter *fedlearning.Adapter
}

// NewFedLearningHandler creates a new federated learning handler.
func NewFedLearningHandler(adapter *fedlearning.Adapter) *FedLearningHandler {
	return &FedLearningHandler{adapter: adapter}
}

// RegisterRoutes registers federated learning API routes.
func (h *FedLearningHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/fedlearn/orgs", h.handleRegisterOrg)
	mux.HandleFunc("GET /v1/fedlearn/orgs", h.handleListOrgs)
	mux.HandleFunc("DELETE /v1/fedlearn/orgs/{id}", h.handleDeregisterOrg)
	mux.HandleFunc("POST /v1/fedlearn/aggregate", h.handleSecureAggregate)
	mux.HandleFunc("POST /v1/fedlearn/policy", h.handleSetPolicy)
	mux.HandleFunc("GET /v1/fedlearn/policy/{feature}/{org}", h.handleCheckPolicy)
	mux.HandleFunc("POST /v1/fedlearn/gradient", h.handleSubmitGradient)
	mux.HandleFunc("GET /v1/fedlearn/gradient/{feature}", h.handleGetAggregatedGradient)
	mux.HandleFunc("GET /v1/fedlearn/stats", h.handleGetStats)
}

// handleRegisterOrg handles POST /v1/fedlearn/orgs
func (h *FedLearningHandler) handleRegisterOrg(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string               `json:"id"`
		Config fedlearning.OrgConfig `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "org id required")
		return
	}

	if err := h.adapter.RegisterOrg(req.ID, req.Config); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "org registered"})
}

// handleListOrgs handles GET /v1/fedlearn/orgs
func (h *FedLearningHandler) handleListOrgs(w http.ResponseWriter, r *http.Request) {
	orgs := h.adapter.ListOrgs()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"orgs": orgs,
	})
}

// handleDeregisterOrg handles DELETE /v1/fedlearn/orgs/{id}
func (h *FedLearningHandler) handleDeregisterOrg(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "org id required")
		return
	}

	if err := h.adapter.DeregisterOrg(id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "org deregistered"})
}

// handleSecureAggregate handles POST /v1/fedlearn/aggregate
func (h *FedLearningHandler) handleSecureAggregate(w http.ResponseWriter, r *http.Request) {
	var req fedlearning.AggregationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.adapter.SecureAggregate(r.Context(), req)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleSetPolicy handles POST /v1/fedlearn/policy
func (h *FedLearningHandler) handleSetPolicy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Feature string                   `json:"feature"`
		Policy  fedlearning.FeaturePolicy `json:"policy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	if err := h.adapter.SetFeaturePolicy(req.Feature, req.Policy); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "policy set"})
}

// handleCheckPolicy handles GET /v1/fedlearn/policy/{feature}/{org}
func (h *FedLearningHandler) handleCheckPolicy(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	org := r.PathValue("org")
	if feature == "" || org == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature and org required")
		return
	}

	allowed, reason := h.adapter.CheckPolicy(org, feature)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"feature": feature,
		"org":     org,
		"allowed": allowed,
		"reason":  reason,
	})
}

// handleSubmitGradient handles POST /v1/fedlearn/gradient
func (h *FedLearningHandler) handleSubmitGradient(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrgID    string    `json:"org_id"`
		Feature  string    `json:"feature"`
		Gradient []float64 `json:"gradient"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OrgID == "" || req.Feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "org_id and feature required")
		return
	}

	if err := h.adapter.SubmitGradient(req.OrgID, req.Feature, req.Gradient); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "gradient submitted"})
}

// handleGetAggregatedGradient handles GET /v1/fedlearn/gradient/{feature}
func (h *FedLearningHandler) handleGetAggregatedGradient(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	gradient, err := h.adapter.GetAggregatedGradient(feature)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"feature":  feature,
		"gradient": gradient,
	})
}

// handleGetStats handles GET /v1/fedlearn/stats
func (h *FedLearningHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.adapter.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *FedLearningHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *FedLearningHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
