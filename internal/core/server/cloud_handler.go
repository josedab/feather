package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/platform/cloud"
)

// CloudHandler provides HTTP endpoints for the cloud control plane.
type CloudHandler struct {
	cp *cloud.ControlPlane
}

// NewCloudHandler creates a new cloud handler.
func NewCloudHandler(cp *cloud.ControlPlane) *CloudHandler {
	return &CloudHandler{cp: cp}
}

// RegisterRoutes registers cloud API routes.
func (h *CloudHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/cloud/instances", h.handleProvision)
	mux.HandleFunc("GET /v1/cloud/instances", h.handleListInstances)
	mux.HandleFunc("GET /v1/cloud/instances/{id}", h.handleGetInstance)
	mux.HandleFunc("POST /v1/cloud/instances/{id}/scale", h.handleScale)
	mux.HandleFunc("DELETE /v1/cloud/instances/{id}", h.handleTerminate)
	mux.HandleFunc("GET /v1/cloud/usage/{tenantID}", h.handleGetUsage)
	mux.HandleFunc("POST /v1/cloud/usage/{tenantID}", h.handleRecordUsage)
	mux.HandleFunc("GET /v1/cloud/autoscale/evaluate", h.handleEvaluateAutoscale)
	mux.HandleFunc("GET /v1/cloud/stats", h.handleStats)
}

func (h *CloudHandler) handleProvision(w http.ResponseWriter, r *http.Request) {
	var req cloud.ProvisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	instance, err := h.cp.Provision(req)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, instance)
}

func (h *CloudHandler) handleListInstances(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	instances := h.cp.ListInstances(tenantID)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"instances": instances,
		"total":     len(instances),
	})
}

func (h *CloudHandler) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	instance, err := h.cp.GetInstance(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, instance)
}

func (h *CloudHandler) handleScale(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req cloud.ScaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	instance, err := h.cp.Scale(id, req)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, instance)
}

func (h *CloudHandler) handleTerminate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.cp.Terminate(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "terminating", "id": id})
}

func (h *CloudHandler) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenantID")
	usage, err := h.cp.GetUsage(tenantID)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, usage)
}

func (h *CloudHandler) handleRecordUsage(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenantID")

	var req struct {
		Reads  int64 `json:"reads"`
		Writes int64 `json:"writes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	h.cp.RecordUsage(tenantID, req.Reads, req.Writes)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "recorded"})
}

func (h *CloudHandler) handleEvaluateAutoscale(w http.ResponseWriter, r *http.Request) {
	actions := h.cp.EvaluateAutoscale()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"actions": actions,
		"total":   len(actions),
	})
}

func (h *CloudHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.cp.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
