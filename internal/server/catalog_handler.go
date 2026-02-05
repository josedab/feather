package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/registry"
)

// CatalogHandler handles feature catalog API requests.
type CatalogHandler struct {
	catalog *registry.Catalog
}

// NewCatalogHandler creates a new catalog handler.
func NewCatalogHandler() *CatalogHandler {
	return &CatalogHandler{
		catalog: registry.NewCatalog(),
	}
}

// RegisterRoutes registers catalog API routes.
func (h *CatalogHandler) RegisterRoutes(mux *http.ServeMux) {
	// Feature CRUD
	mux.HandleFunc("POST /v1/catalog/features", h.handleRegisterFeature)
	mux.HandleFunc("GET /v1/catalog/features", h.handleListFeatures)
	mux.HandleFunc("GET /v1/catalog/features/{name}", h.handleGetFeature)
	mux.HandleFunc("DELETE /v1/catalog/features/{name}", h.handleDeleteFeature)
	mux.HandleFunc("PUT /v1/catalog/features/{name}/status", h.handleSetStatus)

	// Versioning
	mux.HandleFunc("GET /v1/catalog/features/{name}/versions", h.handleGetVersions)
	mux.HandleFunc("GET /v1/catalog/features/{name}/versions/{version}", h.handleGetVersion)

	// Search and discovery
	mux.HandleFunc("GET /v1/catalog/search", h.handleSearch)
	mux.HandleFunc("GET /v1/catalog/tags/{tag}", h.handleGetByTag)
	mux.HandleFunc("GET /v1/catalog/owners/{owner}", h.handleGetByOwner)
	mux.HandleFunc("GET /v1/catalog/teams/{team}", h.handleGetByTeam)
	mux.HandleFunc("GET /v1/catalog/categories/{category}", h.handleGetByCategory)
	mux.HandleFunc("GET /v1/catalog/entities/{entity}", h.handleGetByEntity)

	// Lineage
	mux.HandleFunc("GET /v1/catalog/features/{name}/lineage", h.handleGetLineage)

	// Stats and export
	mux.HandleFunc("GET /v1/catalog/stats", h.handleGetStats)
	mux.HandleFunc("GET /v1/catalog/export", h.handleExport)
	mux.HandleFunc("POST /v1/catalog/import", h.handleImport)
}

// GetCatalog returns the catalog for integration.
func (h *CatalogHandler) GetCatalog() *registry.Catalog {
	return h.catalog
}

// handleRegisterFeature handles POST /v1/catalog/features
func (h *CatalogHandler) handleRegisterFeature(w http.ResponseWriter, r *http.Request) {
	var def registry.FeatureDefinition
	if err := json.NewDecoder(r.Body).Decode(&def); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	registeredBy := r.Header.Get("X-User-ID")
	if registeredBy == "" {
		registeredBy = "anonymous"
	}

	if err := h.catalog.Register(&def, registeredBy); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"feature": def,
	})
}

// handleListFeatures handles GET /v1/catalog/features
func (h *CatalogHandler) handleListFeatures(w http.ResponseWriter, r *http.Request) {
	filter := &registry.ListFilter{
		Owner:      r.URL.Query().Get("owner"),
		Team:       r.URL.Query().Get("team"),
		Category:   r.URL.Query().Get("category"),
		EntityType: r.URL.Query().Get("entity_type"),
		Search:     r.URL.Query().Get("search"),
	}

	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = registry.FeatureStatus(status)
	}

	if tags := r.URL.Query().Get("tags"); tags != "" {
		filter.Tags = splitTags(tags)
	}

	features := h.catalog.List(filter)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": features,
		"count":    len(features),
	})
}

func splitTags(tags string) []string {
	if tags == "" {
		return nil
	}
	result := make([]string, 0)
	for _, t := range splitString(tags, ",") {
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

func splitString(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

// handleGetFeature handles GET /v1/catalog/features/{name}
func (h *CatalogHandler) handleGetFeature(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	def := h.catalog.Get(name)
	if def == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "feature not found")
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, def)
}

// handleDeleteFeature handles DELETE /v1/catalog/features/{name}
func (h *CatalogHandler) handleDeleteFeature(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	if err := h.catalog.Delete(name); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"name":    name,
	})
}

// SetStatusRequest represents a status update request.
type SetStatusRequest struct {
	Status string `json:"status"`
}

// handleSetStatus handles PUT /v1/catalog/features/{name}/status
func (h *CatalogHandler) handleSetStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	var req SetStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	updatedBy := r.Header.Get("X-User-ID")
	if updatedBy == "" {
		updatedBy = "anonymous"
	}

	if err := h.catalog.SetStatus(name, registry.FeatureStatus(req.Status), updatedBy); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"name":    name,
		"status":  req.Status,
	})
}

// handleGetVersions handles GET /v1/catalog/features/{name}/versions
func (h *CatalogHandler) handleGetVersions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	versions := h.catalog.GetVersionHistory(name)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"versions": versions,
		"count":    len(versions),
	})
}

// handleGetVersion handles GET /v1/catalog/features/{name}/versions/{version}
func (h *CatalogHandler) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	versionStr := r.PathValue("version")

	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid version number")
		return
	}

	def := h.catalog.GetVersion(name, version)
	if def == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "version not found")
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, def)
}

// handleSearch handles GET /v1/catalog/search
func (h *CatalogHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "search query required")
		return
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	results := h.catalog.Search(query, limit)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
		"count":   len(results),
		"query":   query,
	})
}

// handleGetByTag handles GET /v1/catalog/tags/{tag}
func (h *CatalogHandler) handleGetByTag(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	if tag == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tag required")
		return
	}

	features := h.catalog.GetByTag(tag)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": features,
		"count":    len(features),
		"tag":      tag,
	})
}

// handleGetByOwner handles GET /v1/catalog/owners/{owner}
func (h *CatalogHandler) handleGetByOwner(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	if owner == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "owner required")
		return
	}

	features := h.catalog.GetByOwner(owner)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": features,
		"count":    len(features),
		"owner":    owner,
	})
}

// handleGetByTeam handles GET /v1/catalog/teams/{team}
func (h *CatalogHandler) handleGetByTeam(w http.ResponseWriter, r *http.Request) {
	team := r.PathValue("team")
	if team == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "team required")
		return
	}

	features := h.catalog.GetByTeam(team)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": features,
		"count":    len(features),
		"team":     team,
	})
}

// handleGetByCategory handles GET /v1/catalog/categories/{category}
func (h *CatalogHandler) handleGetByCategory(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	if category == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "category required")
		return
	}

	features := h.catalog.GetByCategory(category)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": features,
		"count":    len(features),
		"category": category,
	})
}

// handleGetByEntity handles GET /v1/catalog/entities/{entity}
func (h *CatalogHandler) handleGetByEntity(w http.ResponseWriter, r *http.Request) {
	entity := r.PathValue("entity")
	if entity == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entity type required")
		return
	}

	features := h.catalog.GetByEntityType(entity)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features":    features,
		"count":       len(features),
		"entity_type": entity,
	})
}

// handleGetLineage handles GET /v1/catalog/features/{name}/lineage
func (h *CatalogHandler) handleGetLineage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	lineage := h.catalog.GetLineage(name)
	if lineage == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "feature not found")
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, lineage)
}

// handleGetStats handles GET /v1/catalog/stats
func (h *CatalogHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.catalog.GetStats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

// handleExport handles GET /v1/catalog/export
func (h *CatalogHandler) handleExport(w http.ResponseWriter, r *http.Request) {
	data, err := h.catalog.Export()
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=feature_catalog.json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
	}
}

// handleImport handles POST /v1/catalog/import
func (h *CatalogHandler) handleImport(w http.ResponseWriter, r *http.Request) {
	var features []registry.FeatureDefinition
	if err := json.NewDecoder(r.Body).Decode(&features); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	importedBy := r.Header.Get("X-User-ID")
	if importedBy == "" {
		importedBy = "anonymous"
	}

	imported := 0
	for _, def := range features {
		if err := h.catalog.Register(&def, importedBy); err == nil {
			imported++
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"imported": imported,
		"total":    len(features),
	})
}

func (h *CatalogHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *CatalogHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
