package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/extensions/garbagecollect"
)

// GarbageCollectHandler handles smart feature garbage collection API requests.
type GarbageCollectHandler struct {
	collector *garbagecollect.Collector
}

// NewGarbageCollectHandler creates a new garbage collection handler.
func NewGarbageCollectHandler(collector *garbagecollect.Collector) *GarbageCollectHandler {
	return &GarbageCollectHandler{collector: collector}
}

// RegisterRoutes registers garbage collection API routes.
func (h *GarbageCollectHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/gc/policies", h.handleListPolicies)
	mux.HandleFunc("POST /v1/gc/policies", h.handleCreatePolicy)
	mux.HandleFunc("DELETE /v1/gc/policies/{name}", h.handleDeletePolicy)
	mux.HandleFunc("POST /v1/gc/analyze/{policy}", h.handleAnalyze)
	mux.HandleFunc("POST /v1/gc/run/{policy}", h.handleRun)
	mux.HandleFunc("POST /v1/gc/access", h.handleRecordAccess)
	mux.HandleFunc("GET /v1/gc/results", h.handleGetResults)
	mux.HandleFunc("GET /v1/gc/stats", h.handleStats)
}

func (h *GarbageCollectHandler) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	policies := h.collector.ListPolicies()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"policies": policies,
		"total":    len(policies),
	})
}

func (h *GarbageCollectHandler) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	var policy garbagecollect.GCPolicy
	if err := strictDecode(r.Body, &policy); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.collector.RegisterPolicy(policy); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "policy created"})
}

func (h *GarbageCollectHandler) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.collector.DeletePolicy(name); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "policy deleted"})
}

func (h *GarbageCollectHandler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	policy := r.PathValue("policy")
	candidates, err := h.collector.Analyze(policy)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"candidates": candidates,
		"total":      len(candidates),
	})
}

func (h *GarbageCollectHandler) handleRun(w http.ResponseWriter, r *http.Request) {
	policy := r.PathValue("policy")
	result, err := h.collector.Run(policy)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *GarbageCollectHandler) handleRecordAccess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FeatureName string `json:"feature_name"`
		Group       string `json:"group"`
		SizeBytes   int64  `json:"size_bytes"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.FeatureName == "" || req.Group == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "feature_name and group are required")
		return
	}
	h.collector.RecordAccess(req.FeatureName, req.Group, req.SizeBytes)
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "access recorded"})
}

func (h *GarbageCollectHandler) handleGetResults(w http.ResponseWriter, r *http.Request) {
	results := h.collector.GetResults(50)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
		"total":   len(results),
	})
}

func (h *GarbageCollectHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.collector.Stats())
}
