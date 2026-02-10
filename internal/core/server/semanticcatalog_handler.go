package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/semantic"
)

// SemanticCatalogHandler provides semantic feature catalog search and duplicate detection.
type SemanticCatalogHandler struct {
	catalog *semantic.Catalog
}

// NewSemanticCatalogHandler creates a new semantic catalog handler.
func NewSemanticCatalogHandler(catalog *semantic.Catalog) *SemanticCatalogHandler {
	return &SemanticCatalogHandler{catalog: catalog}
}

// RegisterRoutes registers semantic catalog API routes.
func (h *SemanticCatalogHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/catalog/search", h.handleSearch)
	mux.HandleFunc("POST /v1/catalog/index", h.handleIndex)
	mux.HandleFunc("GET /v1/catalog/entries", h.handleList)
	mux.HandleFunc("GET /v1/catalog/entries/{name}", h.handleGet)
	mux.HandleFunc("GET /v1/catalog/duplicates", h.handleDetectDuplicates)
	mux.HandleFunc("GET /v1/catalog/stats", h.handleStats)
}

func (h *SemanticCatalogHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	results := h.catalog.Search(query, limit)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
		"total":   len(results),
		"query":   query,
	})
}

func (h *SemanticCatalogHandler) handleIndex(w http.ResponseWriter, r *http.Request) {
	var entry semantic.CatalogEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.catalog.Index(entry); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"feature": entry.Name,
	})
}

func (h *SemanticCatalogHandler) handleList(w http.ResponseWriter, r *http.Request) {
	entries := h.catalog.List()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   len(entries),
	})
}

func (h *SemanticCatalogHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	entry, err := h.catalog.Get(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, entry)
}

func (h *SemanticCatalogHandler) handleDetectDuplicates(w http.ResponseWriter, r *http.Request) {
	dupes := h.catalog.DetectDuplicates()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"duplicates": dupes,
		"total":      len(dupes),
	})
}

func (h *SemanticCatalogHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.catalog.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
