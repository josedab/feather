package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/integrations/dbt"
	"github.com/feather-store/feather/internal/platform/registry"
)

// DBTHandler handles dbt integration HTTP endpoints.
type DBTHandler struct {
	adapter    *dbt.Adapter
	catalog    *registry.Catalog
	lastSync   *dbt.SyncResult
	lastSyncMu sync.RWMutex
}

// DBTHandlerConfig configures the DBT handler.
type DBTHandlerConfig struct {
	Options *dbt.SyncOptions
	Catalog *registry.Catalog
}

// NewDBTHandler creates a new dbt handler.
func NewDBTHandler(options *dbt.SyncOptions) *DBTHandler {
	return &DBTHandler{
		adapter: dbt.NewAdapter(options),
	}
}

// NewDBTHandlerWithCatalog creates a new dbt handler with catalog persistence.
func NewDBTHandlerWithCatalog(cfg DBTHandlerConfig) *DBTHandler {
	return &DBTHandler{
		adapter: dbt.NewAdapter(cfg.Options),
		catalog: cfg.Catalog,
	}
}

// RegisterRoutes registers dbt-related routes.
func (h *DBTHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/dbt/sync", h.handleSync)
	mux.HandleFunc("POST /v1/dbt/validate", h.handleValidate)
	mux.HandleFunc("GET /v1/dbt/status", h.handleStatus)
}

// DBTSyncRequest represents a dbt sync request.
type DBTSyncRequest struct {
	Manifest json.RawMessage  `json:"manifest"`
	Options  *dbt.SyncOptions `json:"options,omitempty"`
}

// handleSync handles POST /v1/dbt/sync
func (h *DBTHandler) handleSync(w http.ResponseWriter, r *http.Request) {
	var req DBTSyncRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Parse manifest
	manifest, err := dbt.ParseManifestFromBytes(req.Manifest)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid manifest: "+err.Error())
		return
	}

	// Create adapter with request options or defaults
	adapter := h.adapter
	if req.Options != nil {
		adapter = dbt.NewAdapter(req.Options)
	}

	// Sync manifest
	result, err := adapter.SyncManifest(manifest)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, "sync failed: "+err.Error())
		return
	}

	// Store last sync result
	h.lastSyncMu.Lock()
	h.lastSync = result
	h.lastSyncMu.Unlock()

	// Persist features to catalog if configured
	if h.catalog != nil {
		created, updated := 0, 0
		for _, feature := range result.Features {
			catalogDef := convertDBTFeatureToCatalog(feature)

			// Check if feature exists to determine created vs updated
			existing := h.catalog.Get(catalogDef.Name)
			if err := h.catalog.Register(catalogDef, "dbt-sync"); err != nil {
				result.Errors = append(result.Errors, dbt.SyncError{
					ModelName: feature.Source.DBTModelName,
					Message:   "failed to persist to catalog: " + err.Error(),
				})
				continue
			}
			if existing != nil {
				updated++
			} else {
				created++
			}
		}
		result.FeaturesCreated = created
		result.FeaturesUpdated = updated
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

// handleValidate handles POST /v1/dbt/validate
func (h *DBTHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	var req DBTSyncRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Parse manifest
	manifest, err := dbt.ParseManifestFromBytes(req.Manifest)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid manifest: "+err.Error())
		return
	}

	// Validate without syncing
	result, err := h.adapter.ValidateManifest(manifest)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, "validation failed: "+err.Error())
		return
	}

	// Mark as validation-only
	result.FeaturesCreated = 0
	result.FeaturesUpdated = 0

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"valid":        result.Success && len(result.Errors) == 0,
		"features":     len(result.Features),
		"errors":       result.Errors,
		"project_name": result.ProjectName,
	})
}

// DBTStatusResponse represents the status of dbt sync.
type DBTStatusResponse struct {
	LastSyncAt      *time.Time `json:"last_sync_at,omitempty"`
	LastSyncSuccess *bool      `json:"last_sync_success,omitempty"`
	FeaturesCreated int        `json:"features_created"`
	FeaturesUpdated int        `json:"features_updated"`
	FeaturesSkipped int        `json:"features_skipped"`
	ErrorCount      int        `json:"error_count"`
	ProjectName     string     `json:"project_name,omitempty"`
	ManifestVersion string     `json:"manifest_version,omitempty"`
}

// handleStatus handles GET /v1/dbt/status
func (h *DBTHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	h.lastSyncMu.RLock()
	lastSync := h.lastSync
	h.lastSyncMu.RUnlock()

	response := DBTStatusResponse{}

	if lastSync != nil {
		response.LastSyncAt = &lastSync.SyncedAt
		response.LastSyncSuccess = &lastSync.Success
		response.FeaturesCreated = lastSync.FeaturesCreated
		response.FeaturesUpdated = lastSync.FeaturesUpdated
		response.FeaturesSkipped = lastSync.FeaturesSkipped
		response.ErrorCount = len(lastSync.Errors)
		response.ProjectName = lastSync.ProjectName
		response.ManifestVersion = lastSync.ManifestVersion
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, response)
}

// convertDBTFeatureToCatalog converts a dbt feature definition to a catalog feature definition.
func convertDBTFeatureToCatalog(feature dbt.FeatureDefinition) *registry.FeatureDefinition {
	return &registry.FeatureDefinition{
		Name:        feature.Name,
		Description: feature.Description,
		DataType:    feature.DataType,
		EntityType:  feature.EntityType,
		Owner:       feature.Owner,
		Team:        feature.Team,
		Tags:        feature.Tags,
		Category:    feature.Category,
		Source: registry.FeatureSource{
			Type:   "dbt",
			System: feature.Source.DBTProject,
			Table:  feature.Source.DBTModelName,
		},
		Metadata: feature.Metadata,
		Status:   registry.StatusActive,
	}
}
