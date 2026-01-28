package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/feather-store/feather/internal/core/storage"
	"github.com/feather-store/feather/internal/platform/consistency"
)

// ConsistencyHandler handles consistency checking API requests.
type ConsistencyHandler struct {
	checker *consistency.Checker
}

// NewConsistencyHandler creates a new consistency handler.
func NewConsistencyHandler(store *storage.Store) *ConsistencyHandler {
	return &ConsistencyHandler{
		checker: consistency.NewChecker(store, nil, consistency.DefaultConfig()),
	}
}

// RegisterRoutes registers consistency API routes.
func (h *ConsistencyHandler) RegisterRoutes(mux *http.ServeMux) {
	// Source management
	mux.HandleFunc("POST /v1/consistency/sources/http", h.handleAddHTTPSource)

	// Checking
	mux.HandleFunc("POST /v1/consistency/check", h.handleCheckFeature)
	mux.HandleFunc("POST /v1/consistency/check/batch", h.handleCheckBatch)

	// Results
	mux.HandleFunc("GET /v1/consistency/results", h.handleGetResults)
	mux.HandleFunc("GET /v1/consistency/inconsistencies", h.handleGetInconsistencies)
	mux.HandleFunc("GET /v1/consistency/report", h.handleGetReport)
}

// GetChecker returns the checker for integration.
func (h *ConsistencyHandler) GetChecker() *consistency.Checker {
	return h.checker
}

// HTTPSourceRequest represents a request to add an HTTP offline source.
type HTTPSourceRequest struct {
	Name     string            `json:"name"`
	Endpoint string            `json:"endpoint"`
	Headers  map[string]string `json:"headers,omitempty"`
}

// handleAddHTTPSource handles POST /v1/consistency/sources/http
func (h *ConsistencyHandler) handleAddHTTPSource(w http.ResponseWriter, r *http.Request) {
	var req HTTPSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Endpoint == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "name and endpoint are required")
		return
	}

	source := consistency.NewHTTPOfflineSource(req.Name, req.Endpoint, req.Headers)
	h.checker.SetOfflineSource(source)

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":  true,
		"name":     req.Name,
		"endpoint": req.Endpoint,
	})
}

// CheckFeatureRequest represents a request to check a single feature.
type CheckFeatureRequest struct {
	EntityID string `json:"entity_id"`
	Feature  string `json:"feature"`
}

// handleCheckFeature handles POST /v1/consistency/check
func (h *ConsistencyHandler) handleCheckFeature(w http.ResponseWriter, r *http.Request) {
	var req CheckFeatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.EntityID == "" || req.Feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entity_id and feature are required")
		return
	}

	result, err := h.checker.CheckFeature(r.Context(), req.EntityID, req.Feature)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// CheckBatchRequest represents a request to check multiple features.
type CheckBatchRequest struct {
	EntityIDs []string `json:"entity_ids"`
	Features  []string `json:"features"`
}

// handleCheckBatch handles POST /v1/consistency/check/batch
func (h *ConsistencyHandler) handleCheckBatch(w http.ResponseWriter, r *http.Request) {
	var req CheckBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.EntityIDs) == 0 || len(req.Features) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entity_ids and features are required")
		return
	}

	results, err := h.checker.CheckBatch(r.Context(), req.EntityIDs, req.Features)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	report := h.checker.GenerateReport(results)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
		"report":  report,
	})
}

// handleGetResults handles GET /v1/consistency/results
func (h *ConsistencyHandler) handleGetResults(w http.ResponseWriter, r *http.Request) {
	feature := r.URL.Query().Get("feature")
	limit := 100
	since := time.Now().Add(-24 * time.Hour)

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	results := h.checker.GetResults(feature, since, limit)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
		"count":   len(results),
	})
}

// handleGetInconsistencies handles GET /v1/consistency/inconsistencies
func (h *ConsistencyHandler) handleGetInconsistencies(w http.ResponseWriter, r *http.Request) {
	feature := r.URL.Query().Get("feature")
	limit := 100
	since := time.Now().Add(-24 * time.Hour)

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	results := h.checker.GetInconsistencies(feature, since, limit)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"inconsistencies": results,
		"count":           len(results),
	})
}

// handleGetReport handles GET /v1/consistency/report
func (h *ConsistencyHandler) handleGetReport(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-24 * time.Hour)

	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	results := h.checker.GetResults("", since, 10000)
	report := h.checker.GenerateReport(results)

	h.writeJSON(r.Context(), w, http.StatusOK, report)
}

func (h *ConsistencyHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *ConsistencyHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
