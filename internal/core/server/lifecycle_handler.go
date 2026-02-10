package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/lifecycle"
)

// LifecycleManagerHandler provides HTTP endpoints for feature lifecycle management.
type LifecycleManagerHandler struct {
	manager *lifecycle.Manager
}

// NewLifecycleManagerHandler creates a new lifecycle manager handler.
func NewLifecycleManagerHandler(manager *lifecycle.Manager) *LifecycleManagerHandler {
	return &LifecycleManagerHandler{manager: manager}
}

// RegisterRoutes registers lifecycle management API routes.
func (h *LifecycleManagerHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/lifecycle/features", h.handleListFeatures)
	mux.HandleFunc("GET /v1/lifecycle/features/{name}", h.handleGetFeature)
	mux.HandleFunc("POST /v1/lifecycle/features/track", h.handleTrackFeature)
	mux.HandleFunc("POST /v1/lifecycle/features/access", h.handleRecordAccess)
	mux.HandleFunc("POST /v1/lifecycle/features/metrics", h.handleUpdateMetrics)
	mux.HandleFunc("GET /v1/lifecycle/rules", h.handleListRules)
	mux.HandleFunc("POST /v1/lifecycle/rules", h.handleAddRule)
	mux.HandleFunc("DELETE /v1/lifecycle/rules/{id}", h.handleRemoveRule)
	mux.HandleFunc("POST /v1/lifecycle/evaluate", h.handleEvaluate)
	mux.HandleFunc("GET /v1/lifecycle/events", h.handleGetEvents)
	mux.HandleFunc("GET /v1/lifecycle/cost-report", h.handleCostReport)
	mux.HandleFunc("GET /v1/lifecycle/stats", h.handleStats)
}

func (h *LifecycleManagerHandler) handleListFeatures(w http.ResponseWriter, r *http.Request) {
	features := h.manager.ListFeatures()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": features,
		"total":    len(features),
	})
}

func (h *LifecycleManagerHandler) handleGetFeature(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	fu, err := h.manager.GetFeature(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, fu)
}

func (h *LifecycleManagerHandler) handleTrackFeature(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Name == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "feature name is required")
		return
	}
	h.manager.TrackFeature(req.Name)
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"feature": req.Name,
	})
}

func (h *LifecycleManagerHandler) handleRecordAccess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Feature  string `json:"feature"`
		Consumer string `json:"consumer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	h.manager.RecordAccess(req.Feature, req.Consumer)
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *LifecycleManagerHandler) handleUpdateMetrics(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Feature        string  `json:"feature"`
		DriftScore     float64 `json:"drift_score"`
		FreshnessScore float64 `json:"freshness_score"`
		StorageBytes   int64   `json:"storage_bytes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.manager.UpdateMetrics(req.Feature, req.DriftScore, req.FreshnessScore, req.StorageBytes); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *LifecycleManagerHandler) handleListRules(w http.ResponseWriter, r *http.Request) {
	rules := h.manager.ListRules()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"rules": rules,
		"total": len(rules),
	})
}

func (h *LifecycleManagerHandler) handleAddRule(w http.ResponseWriter, r *http.Request) {
	var rule lifecycle.LifecycleRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.manager.AddRule(rule); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"rule_id": rule.ID,
	})
}

func (h *LifecycleManagerHandler) handleRemoveRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.manager.RemoveRule(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"deleted": id})
}

func (h *LifecycleManagerHandler) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	events := h.manager.Evaluate()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"events": events,
		"total":  len(events),
	})
}

func (h *LifecycleManagerHandler) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	events := h.manager.GetEvents(limit)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"events": events,
		"total":  len(events),
	})
}

func (h *LifecycleManagerHandler) handleCostReport(w http.ResponseWriter, r *http.Request) {
	topN := 10
	if v := r.URL.Query().Get("top"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			topN = parsed
		}
	}
	report := h.manager.CostReport(topN)
	writeJSONResponse(r.Context(), w, http.StatusOK, report)
}

func (h *LifecycleManagerHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
