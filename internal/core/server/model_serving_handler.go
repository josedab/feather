package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/tools/ml"
	"github.com/feather-store/feather/internal/core/storage"
)

// ModelServingHandler handles model-aware feature serving API requests.
type ModelServingHandler struct {
	orchestrator *ml.ModelServingOrchestrator
	store        *storage.Store
}

// NewModelServingHandler creates a new model serving handler.
func NewModelServingHandler(store *storage.Store) *ModelServingHandler {
	registry := ml.NewModelRegistry()
	snapshotStore := ml.NewSnapshotStore()
	orchestrator := ml.NewModelServingOrchestrator(
		registry,
		snapshotStore,
		ml.DefaultValidatorConfig(),
	)

	return &ModelServingHandler{
		orchestrator: orchestrator,
		store:        store,
	}
}

// RegisterRoutes registers model serving API routes.
func (h *ModelServingHandler) RegisterRoutes(mux *http.ServeMux) {
	// Model registry routes
	mux.HandleFunc("GET /v1/models", h.handleListModels)
	mux.HandleFunc("POST /v1/models", h.handleRegisterModel)
	mux.HandleFunc("GET /v1/models/{id}", h.handleGetModel)
	mux.HandleFunc("DELETE /v1/models/{id}", h.handleDeleteModel)

	// Model version routes
	mux.HandleFunc("GET /v1/models/{id}/versions", h.handleListVersions)
	mux.HandleFunc("POST /v1/models/{id}/versions", h.handleRegisterVersion)
	mux.HandleFunc("GET /v1/models/{id}/versions/{version}", h.handleGetVersion)
	mux.HandleFunc("POST /v1/models/{id}/versions/{version}/activate", h.handleActivateVersion)

	// Training snapshot routes
	mux.HandleFunc("GET /v1/models/{id}/snapshots", h.handleListSnapshots)
	mux.HandleFunc("POST /v1/models/{id}/snapshots", h.handleCreateSnapshot)
	mux.HandleFunc("GET /v1/models/{id}/snapshots/{snapshotId}", h.handleGetSnapshot)

	// Validation routes
	mux.HandleFunc("POST /v1/models/{id}/validate", h.handleValidateFeatures)
	mux.HandleFunc("GET /v1/models/{id}/validation/stats", h.handleValidationStats)

	// Model drift alerts routes
	mux.HandleFunc("GET /v1/models/{id}/drift/alerts", h.handleGetModelAlerts)
	mux.HandleFunc("GET /v1/models/drift/alerts", h.handleGetAllAlerts)
	mux.HandleFunc("POST /v1/models/drift/alerts/{alertId}/acknowledge", h.handleAcknowledgeAlert)

	// Model-aware feature serving
	mux.HandleFunc("POST /v1/models/{id}/serve", h.handleServeFeatures)

	// Stats
	mux.HandleFunc("GET /v1/models/stats", h.handleStats)
}

// Model API request/response types

// MLRegisterModelRequest represents a model registration request for model-aware serving.
type MLRegisterModelRequest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Team        string            `json:"team,omitempty"`
	Owner       string            `json:"owner,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// RegisterVersionRequest represents a version registration request.
type RegisterVersionRequest struct {
	Version         string            `json:"version"`
	Features        []string          `json:"features"`
	Description     string            `json:"description,omitempty"`
	ServingEndpoint string            `json:"serving_endpoint,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// CreateSnapshotRequest represents a snapshot creation request.
type CreateSnapshotRequest struct {
	Version          string                   `json:"version"`
	Description      string                   `json:"description,omitempty"`
	Features         map[string][]interface{} `json:"features"`
	TrainingMetadata ml.TrainingMetadata      `json:"training_metadata,omitempty"`
}

// ValidateFeaturesRequest represents a validation request.
type ValidateFeaturesRequest struct {
	Features map[string]interface{} `json:"features"`
	Version  string                 `json:"version,omitempty"`
}

// ServeFeaturesRequest represents a model-aware feature serving request.
type ServeFeaturesRequest struct {
	EntityID        string   `json:"entity_id"`
	FeatureNames    []string `json:"feature_names,omitempty"`
	Version         string   `json:"version,omitempty"`
	ValidateServing bool     `json:"validate_serving"`
}

// handleListModels handles GET /v1/models
func (h *ModelServingHandler) handleListModels(w http.ResponseWriter, r *http.Request) {
	models := h.orchestrator.Registry().ListModels()

	result := make([]map[string]interface{}, len(models))
	for i, m := range models {
		result[i] = map[string]interface{}{
			"id":             m.ID,
			"name":           m.Name,
			"description":    m.Description,
			"team":           m.Team,
			"owner":          m.Owner,
			"tags":           m.Tags,
			"active_version": m.ActiveVersion,
			"version_count":  len(m.Versions),
			"created_at":     m.CreatedAt,
			"updated_at":     m.UpdatedAt,
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"models": result,
		"count":  len(result),
	})
}

// handleRegisterModel handles POST /v1/models
func (h *ModelServingHandler) handleRegisterModel(w http.ResponseWriter, r *http.Request) {
	var req MLRegisterModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "id is required")
		return
	}
	if req.Name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "name is required")
		return
	}

	model := &ml.Model{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		Team:        req.Team,
		Owner:       req.Owner,
		Tags:        req.Tags,
		Metadata:    req.Metadata,
	}

	if err := h.orchestrator.Registry().RegisterModel(model); err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"model":   model,
	})
}

// handleGetModel handles GET /v1/models/{id}
func (h *ModelServingHandler) handleGetModel(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model id required")
		return
	}

	model, err := h.orchestrator.Registry().GetModel(modelID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, model)
}

// handleDeleteModel handles DELETE /v1/models/{id}
func (h *ModelServingHandler) handleDeleteModel(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model id required")
		return
	}

	if err := h.orchestrator.Registry().DeleteModel(modelID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleListVersions handles GET /v1/models/{id}/versions
func (h *ModelServingHandler) handleListVersions(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model id required")
		return
	}

	model, err := h.orchestrator.Registry().GetModel(modelID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	versions := make([]map[string]interface{}, 0, len(model.Versions))
	for _, v := range model.Versions {
		versions = append(versions, map[string]interface{}{
			"version":     v.Version,
			"status":      v.Status,
			"features":    v.Features,
			"created_at":  v.CreatedAt,
			"description": v.Description,
		})
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"versions":       versions,
		"active_version": model.ActiveVersion,
		"count":          len(versions),
	})
}

// handleRegisterVersion handles POST /v1/models/{id}/versions
func (h *ModelServingHandler) handleRegisterVersion(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model id required")
		return
	}

	var req RegisterVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Version == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "version is required")
		return
	}
	if len(req.Features) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "features are required")
		return
	}

	version := &ml.ModelVersion{
		Version:         req.Version,
		Features:        req.Features,
		Description:     req.Description,
		ServingEndpoint: req.ServingEndpoint,
		Metadata:        req.Metadata,
	}

	if err := h.orchestrator.Registry().RegisterVersion(modelID, version); err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"version": version,
	})
}

// handleGetVersion handles GET /v1/models/{id}/versions/{version}
func (h *ModelServingHandler) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	versionStr := r.PathValue("version")

	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model id required")
		return
	}
	if versionStr == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "version required")
		return
	}

	version, err := h.orchestrator.Registry().GetVersion(modelID, versionStr)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, version)
}

// handleActivateVersion handles POST /v1/models/{id}/versions/{version}/activate
func (h *ModelServingHandler) handleActivateVersion(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	versionStr := r.PathValue("version")

	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model id required")
		return
	}
	if versionStr == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "version required")
		return
	}

	if err := h.orchestrator.Registry().ActivateVersion(modelID, versionStr); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"active_version": versionStr,
	})
}

// handleListSnapshots handles GET /v1/models/{id}/snapshots
func (h *ModelServingHandler) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model id required")
		return
	}

	snapshots := h.orchestrator.SnapshotStore().ListSnapshotsForModel(modelID)

	result := make([]map[string]interface{}, len(snapshots))
	for i, s := range snapshots {
		result[i] = map[string]interface{}{
			"id":            s.ID,
			"model_version": s.ModelVersion,
			"description":   s.Description,
			"feature_count": len(s.Features),
			"created_at":    s.CreatedAt,
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"snapshots": result,
		"count":     len(result),
	})
}

// handleCreateSnapshot handles POST /v1/models/{id}/snapshots
func (h *ModelServingHandler) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model id required")
		return
	}

	var req CreateSnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Version == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "version is required")
		return
	}
	if len(req.Features) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "features are required")
		return
	}

	// Build snapshot from provided data
	builder := ml.NewSnapshotBuilder(modelID, req.Version, req.Description)
	builder.SetTrainingMetadata(req.TrainingMetadata)

	for featureName, samples := range req.Features {
		builder.AddSamples(featureName, samples)
	}

	snapshot := builder.Build()

	if err := h.orchestrator.SnapshotStore().CreateSnapshot(snapshot); err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	// Link snapshot to model version
	version, err := h.orchestrator.Registry().GetVersion(modelID, req.Version)
	if err == nil {
		version.TrainingSnapshotID = snapshot.ID
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":  true,
		"snapshot": snapshot,
	})
}

// handleGetSnapshot handles GET /v1/models/{id}/snapshots/{snapshotId}
func (h *ModelServingHandler) handleGetSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshotID := r.PathValue("snapshotId")
	if snapshotID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "snapshot id required")
		return
	}

	snapshot, err := h.orchestrator.SnapshotStore().GetSnapshot(snapshotID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, snapshot)
}

// handleValidateFeatures handles POST /v1/models/{id}/validate
func (h *ModelServingHandler) handleValidateFeatures(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model id required")
		return
	}

	var req ValidateFeaturesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Features) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "features are required")
		return
	}

	result, err := h.orchestrator.Validator().Validate(r.Context(), modelID, req.Features)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleValidationStats handles GET /v1/models/{id}/validation/stats
func (h *ModelServingHandler) handleValidationStats(w http.ResponseWriter, r *http.Request) {
	stats := h.orchestrator.Validator().Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

// handleGetModelAlerts handles GET /v1/models/{id}/drift/alerts
func (h *ModelServingHandler) handleGetModelAlerts(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model id required")
		return
	}

	// Get version from query param or use active version
	version := r.URL.Query().Get("version")
	if version == "" {
		activeVersion, err := h.orchestrator.Registry().GetActiveVersion(modelID)
		if err == nil {
			version = activeVersion.Version
		}
	}

	alerts := h.orchestrator.DriftMonitor().GetAlertsForModel(modelID, version)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

// handleGetAllAlerts handles GET /v1/models/drift/alerts
func (h *ModelServingHandler) handleGetAllAlerts(w http.ResponseWriter, r *http.Request) {
	sinceStr := r.URL.Query().Get("since")
	since := time.Now().Add(-24 * time.Hour) // Default to last 24 hours

	if sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	alerts := h.orchestrator.DriftMonitor().GetRecentAlerts(since)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
		"since":  since,
	})
}

// handleAcknowledgeAlert handles POST /v1/models/drift/alerts/{alertId}/acknowledge
func (h *ModelServingHandler) handleAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	alertID := r.PathValue("alertId")
	if alertID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "alert id required")
		return
	}

	var req struct {
		AcknowledgedBy string `json:"acknowledged_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.orchestrator.DriftMonitor().AcknowledgeAlert(alertID, req.AcknowledgedBy); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleServeFeatures handles POST /v1/models/{id}/serve
func (h *ModelServingHandler) handleServeFeatures(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model id required")
		return
	}

	var req ServeFeaturesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.EntityID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entity_id is required")
		return
	}

	// Get required features from model
	featureNames := req.FeatureNames
	if len(featureNames) == 0 {
		modelFeatures, err := h.orchestrator.Registry().GetFeaturesForModel(modelID)
		if err != nil {
			h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
			return
		}
		featureNames = modelFeatures
	}

	// Fetch features from store
	featureValues, err := h.store.Get(req.EntityID, featureNames)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, "failed to get features: "+err.Error())
		return
	}

	// Convert to map[string]interface{} for validation
	features := make(map[string]interface{}, len(featureValues))
	for name, fv := range featureValues {
		features[name] = fv.Value
	}

	response := map[string]interface{}{
		"entity_id": req.EntityID,
		"features":  features,
		"model_id":  modelID,
	}

	// Validate if requested
	if req.ValidateServing {
		result, err := h.orchestrator.Validator().Validate(r.Context(), modelID, features)
		if err == nil {
			response["validation"] = result
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, response)
}

// handleStats handles GET /v1/models/stats
func (h *ModelServingHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.orchestrator.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *ModelServingHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *ModelServingHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
