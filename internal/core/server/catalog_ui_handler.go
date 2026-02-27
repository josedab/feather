package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/tools/catalog"
)

// CatalogUIHandler provides HTTP endpoints for the feature catalog UI.
type CatalogUIHandler struct {
	service *catalog.Service
}

// NewCatalogUIHandler creates a new catalog UI handler.
func NewCatalogUIHandler(service *catalog.Service) *CatalogUIHandler {
	return &CatalogUIHandler{service: service}
}

// RegisterRoutes registers catalog UI API routes.
func (h *CatalogUIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/catalog/features", h.handleListFeatures)
	mux.HandleFunc("POST /v1/catalog/features", h.handleRegisterFeature)
	mux.HandleFunc("GET /v1/catalog/features/{name}", h.handleGetFeature)
	mux.HandleFunc("DELETE /v1/catalog/features/{name}", h.handleDeleteFeature)
	mux.HandleFunc("POST /v1/catalog/search", h.handleSearch)
	mux.HandleFunc("GET /v1/catalog/features/{name}/lineage", h.handleGetLineage)
	mux.HandleFunc("POST /v1/catalog/features/{name}/usage", h.handleRecordUsage)
	mux.HandleFunc("GET /v1/catalog/usage", h.handleGetUsageStats)
	mux.HandleFunc("GET /v1/catalog/stats", h.handleGetStats)
	mux.HandleFunc("GET /v1/catalog/owners/{owner}", h.handleGetByOwner)
	mux.HandleFunc("GET /v1/catalog/tags/{tag}", h.handleGetByTag)
}

func (h *CatalogUIHandler) handleListFeatures(w http.ResponseWriter, r *http.Request) {
	entries := h.service.ListAll()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": entries,
		"total":    len(entries),
	})
}

func (h *CatalogUIHandler) handleRegisterFeature(w http.ResponseWriter, r *http.Request) {
	var entry catalog.CatalogEntry
	if err := strictDecode(r.Body, &entry); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.service.Register(entry); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	registered, _ := h.service.Get(entry.Name)
	writeJSONResponse(r.Context(), w, http.StatusCreated, registered)
}

func (h *CatalogUIHandler) handleGetFeature(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	entry, err := h.service.Get(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, entry)
}

func (h *CatalogUIHandler) handleDeleteFeature(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.service.Delete(name); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"deleted": name})
}

func (h *CatalogUIHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	var query catalog.SearchQuery
	if err := strictDecode(r.Body, &query); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	result, err := h.service.Search(query)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *CatalogUIHandler) handleGetLineage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	result, err := h.service.GetLineage(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *CatalogUIHandler) handleRecordUsage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var req struct {
		Consumer string `json:"consumer"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Consumer == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "consumer is required")
		return
	}

	h.service.RecordUsage(name, req.Consumer)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "recorded"})
}

func (h *CatalogUIHandler) handleGetUsageStats(w http.ResponseWriter, r *http.Request) {
	stats := h.service.GetUsageStats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}

func (h *CatalogUIHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.service.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}

func (h *CatalogUIHandler) handleGetByOwner(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	entries := h.service.GetByOwner(owner)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": entries,
		"owner":    owner,
		"total":    len(entries),
	})
}

func (h *CatalogUIHandler) handleGetByTag(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	entries := h.service.GetByTag(tag)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": entries,
		"tag":      tag,
		"total":    len(entries),
	})
}
